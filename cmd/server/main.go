package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/database"
	internalGRPC "github.com/irvanmalik48/realm-api/internal/grpc"
	"github.com/irvanmalik48/realm-api/internal/router"
	"github.com/irvanmalik48/realm-api/internal/security"
	"github.com/irvanmalik48/realm-api/internal/storage"
	"github.com/irvanmalik48/realm-api/internal/telemetry"
	"golang.org/x/sys/unix"
)

func main() {
	cfg := config.Load()

	// Initialize structured logging with OpenTelemetry trace correlation
	logger := telemetry.InitLogger(cfg.Environment)
	logger.Info("Realm API bootstrapping", "env", cfg.Environment)

	// Initialize OpenTelemetry tracing and metrics
	ctx := context.Background()
	otelShutdown, err := telemetry.InitTracer(ctx, "realm-api", cfg.Environment)
	if err != nil {
		slog.Warn("Failed to initialize OpenTelemetry subsystem", "error", err)
	} else if otelShutdown != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = otelShutdown(shutdownCtx)
		}()
	}

	// Initialize Pyroscope continuous profiling (if configured)
	profiler, err := telemetry.InitProfiler("realm-api", cfg.Environment)
	if err != nil {
		slog.Warn("Failed to initialize Pyroscope continuous profiler", "error", err)
	} else if profiler != nil {
		defer profiler.Stop() // #nosec G104
		slog.Info("Pyroscope continuous profiler active")
	}

	// Initialize Database connection if configured
	var db *database.DB
	if cfg.DatabaseURL != "" {
		dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		db, err = database.Connect(dbCtx, cfg.DatabaseURL)
		if err != nil {
			slog.Warn("Failed to connect to database", "error", err)
		} else {
			defer db.Close()
			// Re-initialize logger with asynchronous DB log sink
			telemetry.InitLogger(cfg.Environment, db)

			// Bootstrap Superadmins from env configuration
			if len(cfg.SuperadminEmails) > 0 {
				if err := db.BootstrapSuperadmins(context.Background(), cfg.SuperadminEmails); err != nil {
					slog.Warn("Failed to bootstrap superadmins", "error", err)
				} else {
					slog.Info("Superadmin accounts bootstrapped", "count", len(cfg.SuperadminEmails))
				}
			}
		}
	}

	// Initialize shared storage and rate limiting / auth cache
	var storageEngine storage.Engine
	var storageErr error
	if cfg.StorageBackend == "s3" {
		storageEngine, storageErr = storage.NewS3Engine(cfg)
		if storageErr != nil {
			slog.Error("Failed to initialize S3 storage engine", "error", storageErr)
			os.Exit(1)
		}
		slog.Info("Using S3 storage engine", "bucket", cfg.S3Bucket, "endpoint", cfg.S3Endpoint)
	} else {
		storageEngine, storageErr = storage.NewZstdEngine(cfg.StorageDir)
		if storageErr != nil {
			slog.Error("Failed to initialize local storage engine", "error", storageErr)
			os.Exit(1)
		}
		slog.Info("Using local Zstd storage engine", "dir", cfg.StorageDir)
	}
	tokenCache := auth.NewTokenCache(5 * time.Minute)
	tokenLimiter := auth.NewTokenRateLimiter()

	// Apply Linux Landlock filesystem containment sandbox (local storage only)
	if cfg.StorageBackend != "s3" {
		_ = security.ApplyLandlockSandbox(cfg.StorageDir)
	}

	// 1. Initialize & Start gRPC Server with SO_REUSEPORT
	grpcServer := internalGRPC.NewServer(cfg, db, &internalGRPC.ServerDeps{
		TokenCache:    tokenCache,
		TokenLimiter:  tokenLimiter,
		StorageEngine: storageEngine,
	})
	grpcAddr := fmt.Sprintf(":%s", cfg.GRPCPort)

	listenConfig := &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			err := c.Control(func(fd uintptr) {
				opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
			if err != nil {
				return err
			}
			return opErr
		},
	}

	grpcListener, err := listenConfig.Listen(ctx, "tcp", grpcAddr)
	if err != nil {
		slog.Error("Failed to listen on gRPC port", "port", cfg.GRPCPort, "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("Realm gRPC Server active", "addr", grpcAddr, "env", cfg.Environment)
		if err := grpcServer.Serve(grpcListener); err != nil {
			slog.Warn("gRPC server exited", "error", err)
		}
	}()

	// 2. Initialize & Start HTTP Gateway / API
	app := router.New(cfg, db, &router.ServerDeps{
		TokenCache:    tokenCache,
		TokenLimiter:  tokenLimiter,
		StorageEngine: storageEngine,
	})
	httpAddr := fmt.Sprintf(":%s", cfg.Port)

	// Channel for idle connections / graceful shutdown
	idleConnsClosed := make(chan struct{})

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		slog.Info("Received shutdown signal, gracefully shutting down servers...")
		grpcServer.GracefulStop()
		if err := app.Shutdown(); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
		tokenCache.Close()
		tokenLimiter.Close()
		if db != nil {
			db.Close()
		}
		close(idleConnsClosed)
	}()

	slog.Info("Realm HTTP API active", "addr", httpAddr, "env", cfg.Environment)
	if err := app.Listen(httpAddr); err != nil {
		slog.Warn("HTTP server exited", "error", err)
	}

	<-idleConnsClosed
	slog.Info("Servers stopped cleanly")
}


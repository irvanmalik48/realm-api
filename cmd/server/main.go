package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/database"
	internalGRPC "github.com/irvanmalik48/realm-api/internal/grpc"
	"github.com/irvanmalik48/realm-api/internal/router"
	"github.com/irvanmalik48/realm-api/internal/telemetry"
)

func main() {
	cfg := config.Load()

	// Initialize OpenTelemetry tracer
	ctx := context.Background()
	otelShutdown, err := telemetry.InitTracer(ctx, "realm-api", cfg.Environment)
	if err != nil {
		log.Printf("Warning: Failed to initialize OpenTelemetry tracer: %v\n", err)
	} else if otelShutdown != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = otelShutdown(shutdownCtx)
		}()
	}

	// Initialize Database connection if configured
	var db *database.DB
	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		db, err = database.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v\n", err)
		} else {
			defer db.Close()
		}
	}

	// 1. Initialize & Start gRPC Server
	grpcServer := internalGRPC.NewServer(cfg, db)
	grpcAddr := fmt.Sprintf(":%s", cfg.GRPCPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port %s: %v\n", cfg.GRPCPort, err)
	}

	go func() {
		log.Printf("Realm gRPC Server starting on %s (%s mode)\n", grpcAddr, cfg.Environment)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Printf("gRPC server exited: %v\n", err)
		}
	}()

	// 2. Initialize & Start HTTP Gateway / API
	app := router.New(cfg, db)
	httpAddr := fmt.Sprintf(":%s", cfg.Port)

	// Channel for idle connections / graceful shutdown
	idleConnsClosed := make(chan struct{})

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Received shutdown signal, gracefully shutting down servers...")
		grpcServer.GracefulStop()
		if err := app.Shutdown(); err != nil {
			log.Printf("HTTP server shutdown error: %v\n", err)
		}
		if db != nil {
			db.Close()
		}
		close(idleConnsClosed)
	}()

	log.Printf("Realm HTTP API starting on %s (%s mode)\n", httpAddr, cfg.Environment)
	if err := app.Listen(httpAddr); err != nil {
		log.Printf("HTTP server exited: %v\n", err)
	}

	<-idleConnsClosed
	log.Println("Servers stopped.")
}


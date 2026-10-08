package grpc

import (
	"time"

	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/grpc/interceptors"
	grpcServer "github.com/irvanmalik48/realm-api/internal/grpc/server"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
	"github.com/irvanmalik48/realm-api/internal/storage"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type ServerDeps struct {
	TokenCache    *auth.TokenCache
	TokenLimiter  *auth.TokenRateLimiter
	StorageEngine storage.Engine
	TokenRepo     repository.TokenRepository
	AdminRepo     repository.AdminRepository
}

// NewServer creates and configures a new gRPC server with all services and interceptors registered.
func NewServer(cfg *config.Config, db *database.DB, deps ...*ServerDeps) *grpc.Server {
	// Initialize repositories
	var contactRepo repository.ContactRepository
	var storageRepo repository.StorageRepository
	var tokenRepo repository.TokenRepository
	var userRepo repository.UserRepository
	var reactionRepo repository.ReactionRepository
	var commentRepo repository.CommentRepository
	var adminRepo repository.AdminRepository
	var logRepo repository.LogRepository

	if db != nil {
		contactRepo = repository.NewContactRepository(db)
		storageRepo = repository.NewStorageRepository(db)
		tokenRepo = repository.NewTokenRepository(db)
		userRepo = repository.NewUserRepository(db)
		reactionRepo = repository.NewReactionRepository(db)
		commentRepo = repository.NewCommentRepository(db)
		adminRepo = repository.NewAdminRepository(db)
		logRepo = repository.NewLogRepository(db)
	}

	var storageEngine storage.Engine
	var tokenCache *auth.TokenCache
	var tokenLimiter *auth.TokenRateLimiter

	if len(deps) > 0 && deps[0] != nil {
		storageEngine = deps[0].StorageEngine
		tokenCache = deps[0].TokenCache
		tokenLimiter = deps[0].TokenLimiter
		if deps[0].TokenRepo != nil {
			tokenRepo = deps[0].TokenRepo
		}
		if deps[0].AdminRepo != nil {
			adminRepo = deps[0].AdminRepo
		}
	}

	if storageEngine == nil {
		var err error
		storageEngine, err = storage.NewZstdEngine(cfg.StorageDir)
		if err != nil {
			panic(err)
		}
	}

	if tokenCache == nil {
		tokenCache = auth.NewTokenCache(5 * time.Minute)
	}
	if tokenLimiter == nil {
		tokenLimiter = auth.NewTokenRateLimiter()
	}

	pasetoSvc, err := auth.NewPasetoService(cfg.PASETOSymmetricKey)
	if err != nil {
		panic(err)
	}

	lastFMSvc := service.NewLastFMService(cfg.LastFMAPIKey, cfg.LastFMAPISecret)
	contactSvc := service.NewContactService(cfg, contactRepo)
	storageSvc := service.NewStorageService(cfg, storageRepo, storageEngine)
	tokenSvc := service.NewTokenService(tokenRepo, tokenCache, tokenLimiter)
	authSvc := service.NewAuthService(userRepo, pasetoSvc)
	reactionSvc := service.NewReactionService(reactionRepo)
	commentSvc := service.NewCommentService(commentRepo)

	// Create gRPC Server with OpenTelemetry tracing and interceptors
	maxMsgSize := (cfg.MaxUploadSizeMB + 2) * 1024 * 1024
	if maxMsgSize <= 0 {
		maxMsgSize = 12 * 1024 * 1024
	}

	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize),
		grpc.ChainUnaryInterceptor(
			interceptors.ErrorUnaryInterceptor(),
			interceptors.AuthUnaryInterceptor(pasetoSvc, tokenSvc, tokenLimiter),
		),
		grpc.ChainStreamInterceptor(
			interceptors.ErrorStreamInterceptor(),
		),
	)

	// Register services
	realmv1.RegisterHealthServiceServer(server, grpcServer.NewHealthServer(db))
	realmv1.RegisterAuthServiceServer(server, grpcServer.NewAuthServer(authSvc))
	realmv1.RegisterContactServiceServer(server, grpcServer.NewContactServer(contactSvc))
	realmv1.RegisterLastFMServiceServer(server, grpcServer.NewLastFMServer(cfg, lastFMSvc))
	realmv1.RegisterStorageServiceServer(server, grpcServer.NewStorageServer(cfg, storageSvc, adminRepo))
	realmv1.RegisterReactionServiceServer(server, grpcServer.NewReactionServer(reactionSvc))
	realmv1.RegisterCommentServiceServer(server, grpcServer.NewCommentServer(commentSvc, adminRepo))
	realmv1.RegisterAdminRBACServiceServer(server, grpcServer.NewAdminServer(adminRepo))
	realmv1.RegisterLogServiceServer(server, grpcServer.NewLogServer(logRepo, adminRepo))
	realmv1.RegisterTokenServiceServer(server, grpcServer.NewTokenServer(tokenSvc, adminRepo))

	// Register standard gRPC Health Checking Protocol (grpc.health.v1)
	standardHealthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, standardHealthServer)
	standardHealthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.HealthService", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.AuthService", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.ContactService", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.LastFMService", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.StorageService", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.ReactionService", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.CommentService", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.AdminRBACService", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.LogService", healthpb.HealthCheckResponse_SERVING)
	standardHealthServer.SetServingStatus("realm.v1.TokenService", healthpb.HealthCheckResponse_SERVING)

	// Enable gRPC Server Reflection for debugging and developer tooling
	reflection.Register(server)

	return server
}

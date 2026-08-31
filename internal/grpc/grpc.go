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
	"google.golang.org/grpc/reflection"
)

// NewServer creates and configures a new gRPC server with all services and interceptors registered.
func NewServer(cfg *config.Config, db *database.DB) *grpc.Server {
	// Initialize repositories
	var contactRepo repository.ContactRepository
	var storageRepo repository.StorageRepository
	var tokenRepo repository.TokenRepository
	var userRepo repository.UserRepository
	var reactionRepo repository.ReactionRepository
	var commentRepo repository.CommentRepository

	if db != nil {
		contactRepo = repository.NewContactRepository(db)
		storageRepo = repository.NewStorageRepository(db)
		tokenRepo = repository.NewTokenRepository(db)
		userRepo = repository.NewUserRepository(db)
		reactionRepo = repository.NewReactionRepository(db)
		commentRepo = repository.NewCommentRepository(db)
	}

	storageEngine, err := storage.NewZstdEngine(cfg.StorageDir)
	if err != nil {
		panic(err)
	}

	tokenCache := auth.NewTokenCache(5 * time.Minute)
	tokenLimiter := auth.NewTokenRateLimiter()

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
	realmv1.RegisterStorageServiceServer(server, grpcServer.NewStorageServer(cfg, storageSvc))
	realmv1.RegisterReactionServiceServer(server, grpcServer.NewReactionServer(reactionSvc))
	realmv1.RegisterCommentServiceServer(server, grpcServer.NewCommentServer(commentSvc))

	// Enable gRPC Server Reflection for debugging and developer tooling
	reflection.Register(server)

	return server
}

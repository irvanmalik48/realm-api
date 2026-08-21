package router

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/middleware"
	"github.com/irvanmalik48/realm-api/internal/openapi"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
	"github.com/irvanmalik48/realm-api/internal/storage"
)

func New(cfg *config.Config, db *database.DB) *fiber.App {
	bodyLimit := cfg.MaxUploadSizeMB * 1024 * 1024
	if bodyLimit <= 0 {
		bodyLimit = 10 * 1024 * 1024
	}

	app := fiber.New(fiber.Config{
		AppName:               "Realm API",
		DisableStartupMessage: cfg.Environment == "production",
		BodyLimit:             bodyLimit,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			var e *fiber.Error
			if errorsAs(err, &e) {
				code = e.Code
			}
			return handler.ErrorResponse(c, err.Error(), code)
		},
	})

	// Middlewares
	app.Use(middleware.OpenTelemetryTracing())
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault,
	}))

	allowedOrigins := cfg.AllowedOrigins
	if allowedOrigins == "" {
		allowedOrigins = "*"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: "GET,POST,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Realm-Request, X-Requested-With, X-API-Key, X-API-Token",
	}))

	// Repositories & Services
	var contactRepo repository.ContactRepository
	var storageRepo repository.StorageRepository
	var tokenRepo repository.TokenRepository
	var userRepo repository.UserRepository

	if db != nil {
		contactRepo = repository.NewContactRepository(db)
		storageRepo = repository.NewStorageRepository(db)
		tokenRepo = repository.NewTokenRepository(db)
		userRepo = repository.NewUserRepository(db)
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
	oauthSvc := service.NewOAuthService(cfg)
	authSvc := service.NewAuthService(userRepo, pasetoSvc)

	rootHdlr := handler.NewRootHandler()
	healthHdlr := handler.NewHealthHandler(db)
	lastFMHdlr := handler.NewLastFMHandler(cfg, lastFMSvc)
	contactHdlr := handler.NewContactHandler(cfg, contactSvc)
	storageHdlr := handler.NewStorageHandler(cfg, storageSvc)
	authHdlr := handler.NewAuthHandler(cfg, authSvc, oauthSvc)

	// Root and Health routes
	app.Get("/", rootHdlr.Handle)
	app.Get("/health", healthHdlr.Handle)

	// OpenAPI Specification and Interactive Docs
	app.Get("/openapi.yaml", openapi.ServeYAML)
	app.Get("/openapi.json", openapi.ServeJSON)
	app.Get("/docs", openapi.ServeDocs)

	// v1 routes (/v1/health, /v1/lastfm/track, /v1/lastfm/user, /v1/contact, /v1/storage, /v1/auth)
	v1 := app.Group("/v1")
	v1.Get("/health", healthHdlr.Handle)
	v1.Get("/openapi.yaml", openapi.ServeYAML)
	v1.Get("/openapi.json", openapi.ServeJSON)
	v1.Get("/docs", openapi.ServeDocs)

	// User Auth endpoints (Traditional & OIDC/OAuth2 with PASETO tokens)
	authGroup := v1.Group("/auth")
	authGroup.Get("/check", authHdlr.CheckAvailability)
	authGroup.Post("/register", authHdlr.Register)
	authGroup.Post("/login", authHdlr.Login)
	authGroup.Get("/me", middleware.RequireUserAuth(pasetoSvc), authHdlr.GetMe)
	authGroup.Patch("/profile", middleware.RequireUserAuth(pasetoSvc), authHdlr.UpdateProfile)
	authGroup.Get("/google", authHdlr.GoogleLogin)
	authGroup.Get("/google/callback", authHdlr.GoogleCallback)
	authGroup.Get("/github", authHdlr.GitHubLogin)
	authGroup.Get("/github/callback", authHdlr.GitHubCallback)

	// LastFM endpoints with optional token auth and dynamic rate limiting
	lastfm := v1.Group("/lastfm", middleware.OptionalToken(tokenSvc, tokenLimiter))
	lastfm.Get("/track", lastFMHdlr.GetRecentTracks)
	lastfm.Get("/user", lastFMHdlr.GetUserInfo)

	// Contact endpoint with CSRF protection and rate limiting (5 requests per 10 minutes)
	contactLimiter := limiter.New(limiter.Config{
		Max:        5,
		Expiration: 10 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return handler.ErrorResponse(c, "Too many requests. Please try again later.", http.StatusTooManyRequests)
		},
	})
	v1.Post("/contact", middleware.CSRFProtection(cfg), contactLimiter, contactHdlr.Handle)

	// Storage endpoints (Zstd-compressed, Blurhash, on-the-fly WebP)
	storageGroup := v1.Group("/storage")
	storageGroup.Post("/upload", middleware.RequireToken(tokenSvc, tokenLimiter, "storage:write"), storageHdlr.Upload)
	storageGroup.Get("/:id", middleware.OptionalToken(tokenSvc, tokenLimiter), storageHdlr.GetFile)
	storageGroup.Get("/:id/info", middleware.OptionalToken(tokenSvc, tokenLimiter), storageHdlr.GetFileInfo)
	storageGroup.Delete("/:id", middleware.RequireToken(tokenSvc, tokenLimiter, "storage:write"), storageHdlr.DeleteFile)

	// 404 Not Found fallback handler
	app.Use(func(c *fiber.Ctx) error {
		return handler.ErrorResponse(c, "Endpoint not found", http.StatusNotFound)
	})

	return app
}

func errorsAs(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*fiber.Error); ok {
		if t, ok := target.(**fiber.Error); ok {
			*t = e
			return true
		}
	}
	return false
}

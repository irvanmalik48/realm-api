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
	app := fiber.New(fiber.Config{
		AppName:               "Realm API",
		DisableStartupMessage: cfg.Environment == "production",
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
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Realm-Request, X-Requested-With, X-API-Key",
	}))

	// Repositories & Services
	var contactRepo repository.ContactRepository
	var storageRepo repository.StorageRepository
	if db != nil {
		contactRepo = repository.NewContactRepository(db)
		storageRepo = repository.NewStorageRepository(db)
	}

	storageEngine, err := storage.NewZstdEngine(cfg.StorageDir)
	if err != nil {
		panic(err)
	}

	lastFMSvc := service.NewLastFMService(cfg.LastFMAPIKey, cfg.LastFMAPISecret)
	contactSvc := service.NewContactService(cfg, contactRepo)
	storageSvc := service.NewStorageService(cfg, storageRepo, storageEngine)

	rootHdlr := handler.NewRootHandler()
	lastFMHdlr := handler.NewLastFMHandler(cfg, lastFMSvc)
	contactHdlr := handler.NewContactHandler(cfg, contactSvc)
	storageHdlr := handler.NewStorageHandler(cfg, storageSvc)

	// Root route
	app.Get("/", rootHdlr.Handle)

	// OpenAPI Specification and Interactive Docs
	app.Get("/openapi.yaml", openapi.ServeYAML)
	app.Get("/openapi.json", openapi.ServeJSON)
	app.Get("/docs", openapi.ServeDocs)

	// v1 routes (/v1/lastfm/track, /v1/lastfm/user, /v1/contact, /v1/storage)
	v1 := app.Group("/v1")
	v1.Get("/openapi.yaml", openapi.ServeYAML)
	v1.Get("/openapi.json", openapi.ServeJSON)
	v1.Get("/docs", openapi.ServeDocs)

	// LastFM endpoints
	lastfm := v1.Group("/lastfm")
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
	storageGroup.Post("/upload", middleware.StorageAuth(cfg), storageHdlr.Upload)
	storageGroup.Get("/:id", storageHdlr.GetFile)
	storageGroup.Get("/:id/info", storageHdlr.GetFileInfo)
	storageGroup.Delete("/:id", middleware.StorageAuth(cfg), storageHdlr.DeleteFile)

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

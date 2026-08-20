package middleware

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
)

// StorageAuth ensures write/upload operations to storage are authenticated if an API key is configured.
func StorageAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if cfg.StorageAPIKey == "" {
			// If no key is set in configuration, allow in development/testing
			return c.Next()
		}

		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			authHeader := c.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" || apiKey != cfg.StorageAPIKey {
			return handler.ErrorResponse(c, "Unauthorized: invalid or missing storage API key", http.StatusUnauthorized)
		}

		return c.Next()
	}
}

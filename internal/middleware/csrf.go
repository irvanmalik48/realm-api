package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
)

// CSRFProtection returns a middleware that blocks Cross-Site Request Forgery attacks.
func CSRFProtection(cfg *config.Config) fiber.Handler {
	allowedOrigins := parseAllowedOrigins(cfg.AllowedOrigins)

	return func(c *fiber.Ctx) error {
		// Only check state-changing methods
		method := c.Method()
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
			return c.Next()
		}

		// 1. Fetch metadata validation: Sec-Fetch-Site
		secFetchSite := c.Get("Sec-Fetch-Site")
		if secFetchSite == "cross-site" && len(allowedOrigins) > 0 {
			origin := c.Get("Origin")
			if !isAllowed(origin, allowedOrigins) {
				return handler.ErrorResponse(c, "Cross-site request forgery blocked", http.StatusForbidden)
			}
		}

		// 2. Validate Origin header if present
		origin := c.Get("Origin")
		if origin != "" && len(allowedOrigins) > 0 {
			if !isAllowed(origin, allowedOrigins) {
				return handler.ErrorResponse(c, "Invalid request origin", http.StatusForbidden)
			}
		}

		// 3. Validate Referer header if Origin is absent
		referer := c.Get("Referer")
		if origin == "" && referer != "" && len(allowedOrigins) > 0 {
			refURL, err := url.Parse(referer)
			if err != nil || !isAllowed(refURL.Scheme+"://"+refURL.Host, allowedOrigins) {
				return handler.ErrorResponse(c, "Invalid request referer", http.StatusForbidden)
			}
		}

		// 4. Custom header check to prevent simple HTML form POST forgery
		customHeader := c.Get("X-Realm-Request")
		requestedWith := c.Get("X-Requested-With")
		if customHeader == "" && requestedWith == "" {
			// Require at least one custom header on state-changing API endpoints
			return handler.ErrorResponse(c, "Missing required request security header", http.StatusBadRequest)
		}

		return c.Next()
	}
}

func parseAllowedOrigins(origins string) []string {
	if origins == "" || origins == "*" {
		return nil
	}
	raw := strings.Split(origins, ",")
	var cleaned []string
	for _, o := range raw {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			cleaned = append(cleaned, strings.ToLower(trimmed))
		}
	}
	return cleaned
}

func isAllowed(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	normOrigin := strings.TrimRight(strings.ToLower(strings.TrimSpace(origin)), "/")
	for _, a := range allowed {
		normAllowed := strings.TrimRight(strings.ToLower(a), "/")
		if normOrigin == normAllowed {
			return true
		}
	}
	return false
}

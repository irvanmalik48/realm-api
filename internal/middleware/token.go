package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
)

// ExtractRawToken reads token from Authorization Bearer header, X-API-Token, or X-API-Key
func ExtractRawToken(c *fiber.Ctx) string {
	authHeader := c.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}

	if tok := c.Get("X-API-Token"); tok != "" {
		return strings.TrimSpace(tok)
	}

	if tok := c.Get("X-API-Key"); tok != "" {
		return strings.TrimSpace(tok)
	}

	return ""
}

// RequireToken validates API token and checks for required scopes with per-token rate limiting
func RequireToken(svc service.TokenService, limiter *auth.TokenRateLimiter, requiredScopes ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawToken := ExtractRawToken(c)

		if rawToken == "" {
			return handler.ErrorResponse(c, "Unauthorized: missing API token", http.StatusUnauthorized)
		}

		if svc == nil {
			return handler.ErrorResponse(c, "Authentication service unavailable", http.StatusServiceUnavailable)
		}

		token, err := svc.Verify(c.Context(), rawToken)
		if err != nil {
			if errors.Is(err, service.ErrTokenExpired) {
				return handler.ErrorResponse(c, "Unauthorized: API token has expired", http.StatusUnauthorized)
			}
			if errors.Is(err, service.ErrTokenRevoked) {
				return handler.ErrorResponse(c, "Unauthorized: API token has been revoked", http.StatusUnauthorized)
			}
			return handler.ErrorResponse(c, "Unauthorized: invalid API token", http.StatusUnauthorized)
		}

		// Apply rate limit for authenticated token
		if limiter != nil {
			allowed, remaining, resetEpoch := limiter.Allow(token.ID.String(), token.RateLimitRPM)
			c.Set("X-RateLimit-Limit", strconv.Itoa(token.RateLimitRPM))
			c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			c.Set("X-RateLimit-Reset", strconv.FormatInt(resetEpoch, 10))

			if !allowed {
				return handler.ErrorResponse(c, "Too many requests. Rate limit quota exceeded.", http.StatusTooManyRequests)
			}
		}

		// Verify scopes if required
		for _, requiredScope := range requiredScopes {
			if !svc.HasScope(token, requiredScope) {
				return handler.ErrorResponse(c, "Forbidden: token lacks required scope '"+requiredScope+"'", http.StatusForbidden)
			}
		}

		c.Locals("token", token)
		return c.Next()
	}
}

// OptionalToken extracts and verifies token if provided, attaching rate limit headers without blocking unauthenticated requests
func OptionalToken(svc service.TokenService, limiter *auth.TokenRateLimiter) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawToken := ExtractRawToken(c)
		if rawToken == "" || svc == nil {
			return c.Next()
		}

		token, err := svc.Verify(c.Context(), rawToken)
		if err == nil && token != nil {
			c.Locals("token", token)
			if limiter != nil {
				allowed, remaining, resetEpoch := limiter.Allow(token.ID.String(), token.RateLimitRPM)
				c.Set("X-RateLimit-Limit", strconv.Itoa(token.RateLimitRPM))
				c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
				c.Set("X-RateLimit-Reset", strconv.FormatInt(resetEpoch, 10))

				if !allowed {
					return handler.ErrorResponse(c, "Too many requests. Rate limit quota exceeded.", http.StatusTooManyRequests)
				}
			}
		}

		return c.Next()
	}
}

// GetAuthenticatedToken helper retrieves token from fiber context
func GetAuthenticatedToken(c *fiber.Ctx) *model.APIToken {
	val := c.Locals("token")
	if tok, ok := val.(*model.APIToken); ok {
		return tok
	}
	return nil
}

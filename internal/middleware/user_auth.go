package middleware

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/model"
)

// ExtractUserRawToken extracts token from Bearer auth, X-Auth-Token header, or realm_auth_token cookie
func ExtractUserRawToken(c *fiber.Ctx) string {
	authHeader := c.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}

	if tok := c.Get("X-Auth-Token"); tok != "" {
		return strings.TrimSpace(tok)
	}

	if tok := c.Cookies("realm_auth_token"); tok != "" {
		return strings.TrimSpace(tok)
	}

	return ""
}

// RequireUserAuth validates PASETO bearer token for protected user endpoints
func RequireUserAuth(pasetoSvc auth.PasetoService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawToken := ExtractUserRawToken(c)
		if rawToken == "" {
			return handler.ErrorResponse(c, "Unauthorized: missing authentication token", http.StatusUnauthorized)
		}

		if pasetoSvc == nil {
			return handler.ErrorResponse(c, "Authentication service unavailable", http.StatusServiceUnavailable)
		}

		claims, err := pasetoSvc.VerifyToken(rawToken)
		if err != nil {
			return handler.ErrorResponse(c, "Unauthorized: invalid or expired session", http.StatusUnauthorized)
		}

		userID, err := uuid.Parse(claims.ID)
		if err != nil {
			return handler.ErrorResponse(c, "Unauthorized: invalid user identifier in token", http.StatusUnauthorized)
		}

		c.Locals("user_claims", claims)
		c.Locals("user_id", userID)
		return c.Next()
	}
}

// GetAuthenticatedUser helper retrieves claims from fiber context
func GetAuthenticatedUser(c *fiber.Ctx) *model.UserClaims {
	val := c.Locals("user_claims")
	if claims, ok := val.(*model.UserClaims); ok {
		return claims
	}
	return nil
}

// GetAuthenticatedUserID helper retrieves user UUID from fiber context
func GetAuthenticatedUserID(c *fiber.Ctx) (uuid.UUID, bool) {
	val := c.Locals("user_id")
	if id, ok := val.(uuid.UUID); ok {
		return id, true
	}
	return uuid.Nil, false
}

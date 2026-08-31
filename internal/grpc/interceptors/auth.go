package interceptors

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	userIDKey    contextKey = "user_id"
	userEmailKey contextKey = "user_email"
	usernameKey  contextKey = "username"
	apiTokenKey  contextKey = "api_token"
)

// GetUserID retrieves the authenticated user's UUID from the context, if present.
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	val := ctx.Value(userIDKey)
	if id, ok := val.(uuid.UUID); ok && id != uuid.Nil {
		return id, true
	}
	return uuid.Nil, false
}

// RequireUserID returns the authenticated user's UUID or returns an Unauthenticated error.
func RequireUserID(ctx context.Context) (uuid.UUID, error) {
	id, ok := GetUserID(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "Authentication required")
	}
	return id, nil
}

// AuthUnaryInterceptor parses authorization credentials (PASETO or API Token) from gRPC metadata.
func AuthUnaryInterceptor(pasetoSvc auth.PasetoService, tokenSvc service.TokenService, tokenLimiter *auth.TokenRateLimiter) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return handler(ctx, req)
		}

		var rawToken string
		if vals := md.Get("authorization"); len(vals) > 0 {
			authHeader := vals[0]
			if strings.HasPrefix(authHeader, "Bearer ") {
				rawToken = strings.TrimPrefix(authHeader, "Bearer ")
			} else {
				rawToken = authHeader
			}
		} else if vals := md.Get("x-api-token"); len(vals) > 0 {
			rawToken = vals[0]
		}

		rawToken = strings.TrimSpace(rawToken)
		if rawToken == "" {
			return handler(ctx, req)
		}

		// Check if it's an API token (starts with realm_tok_)
		if strings.HasPrefix(rawToken, "realm_tok_") {
			if tokenSvc != nil {
				tokenRecord, err := tokenSvc.Verify(ctx, rawToken)
				if err == nil && tokenRecord != nil {
					if tokenLimiter != nil {
						if allowed, _, _ := tokenLimiter.Allow(tokenRecord.ID.String(), tokenRecord.RateLimitRPM); !allowed {
							return nil, status.Error(codes.ResourceExhausted, "Rate limit exceeded for API token")
						}
					}
					ctx = context.WithValue(ctx, apiTokenKey, tokenRecord)
				}
			}
			return handler(ctx, req)
		}

		// PASETO user authentication token
		if pasetoSvc != nil {
			claims, err := pasetoSvc.VerifyToken(rawToken)
			if err == nil && claims != nil && claims.ID != "" {
				if uid, parseErr := uuid.Parse(claims.ID); parseErr == nil {
					ctx = context.WithValue(ctx, userIDKey, uid)
					ctx = context.WithValue(ctx, userEmailKey, claims.Email)
					ctx = context.WithValue(ctx, usernameKey, claims.Username)
				}
			}
		}

		return handler(ctx, req)
	}
}

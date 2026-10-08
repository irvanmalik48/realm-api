package server

import (
	"context"
	"strings"

	"github.com/irvanmalik48/realm-api/internal/grpc/interceptors"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Authorize verifies that the gRPC caller has a valid API token or Admin User session
// satisfying at least one of the required permission scopes.
func Authorize(ctx context.Context, adminRepo repository.AdminRepository, requiredScopes ...string) error {
	// 1. API Token Check
	if tok, ok := interceptors.GetAPIToken(ctx); ok && tok != nil {
		if len(requiredScopes) == 0 {
			return nil
		}
		for _, reqScope := range requiredScopes {
			for _, s := range tok.Scopes {
				if s == "*" || s == "all" || s == "admin" || s == reqScope ||
					(strings.HasSuffix(s, ":*") && strings.HasPrefix(reqScope, strings.TrimSuffix(s, ":*")+":")) {
					return nil
				}
			}
		}
		return status.Errorf(codes.PermissionDenied, "API token lacks required scope %v", requiredScopes)
	}

	// 2. User Authentication Check
	userID, ok := interceptors.GetUserID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "Authentication credentials required")
	}

	if adminRepo == nil {
		return status.Error(codes.Unavailable, "Admin repository unavailable")
	}

	admin, err := adminRepo.GetByUserID(ctx, userID)
	if err != nil || admin == nil {
		return status.Error(codes.PermissionDenied, "Administrator privileges required")
	}

	if admin.IsSuperadmin {
		return nil
	}

	if len(requiredScopes) == 0 {
		return nil
	}

	for _, reqScope := range requiredScopes {
		for _, p := range admin.Permissions {
			if p == "*" || p == reqScope ||
				(strings.HasSuffix(p, ":*") && strings.HasPrefix(reqScope, strings.TrimSuffix(p, ":*")+":")) {
				return nil
			}
		}
	}

	return status.Errorf(codes.PermissionDenied, "Administrator lacks required permission %v", requiredScopes)
}

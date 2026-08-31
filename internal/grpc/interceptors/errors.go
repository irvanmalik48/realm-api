package interceptors

import (
	"context"
	"errors"
	"log"

	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
	"github.com/irvanmalik48/realm-api/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MapError converts internal domain errors into appropriate gRPC status errors.
func MapError(err error) error {
	if err == nil {
		return nil
	}

	// If already a gRPC status error, return directly
	if _, ok := status.FromError(err); ok {
		return err
	}

	switch {
	// Authentication / Credentials
	case errors.Is(err, service.ErrInvalidCredentials),
		errors.Is(err, auth.ErrInvalidToken),
		errors.Is(err, auth.ErrExpiredToken):
		return status.Error(codes.Unauthenticated, err.Error())

	// Resource Not Found
	case errors.Is(err, repository.ErrUserNotFound),
		errors.Is(err, repository.ErrRecordNotFound),
		errors.Is(err, storage.ErrFileNotFound),
		errors.Is(err, service.ErrCommentNotFound):
		return status.Error(codes.NotFound, err.Error())

	// Conflicts / Already Exists
	case errors.Is(err, repository.ErrUserAlreadyExists),
		errors.Is(err, service.ErrUsernameTaken):
		return status.Error(codes.AlreadyExists, err.Error())

	// Validation / Bad Request
	case errors.Is(err, service.ErrInvalidEmail),
		errors.Is(err, service.ErrInvalidUsername),
		errors.Is(err, service.ErrPasswordTooShort),
		errors.Is(err, service.ErrFullNameRequired),
		errors.Is(err, service.ErrEmptySlug),
		errors.Is(err, service.ErrInvalidReaction),
		errors.Is(err, service.ErrEmptyComment),
		errors.Is(err, service.ErrCommentTooLong),
		errors.Is(err, service.ErrCurrentPasswordReq),
		errors.Is(err, service.ErrCurrentPasswordBad):
		return status.Error(codes.InvalidArgument, err.Error())

	// Resource Limits
	case errors.Is(err, service.ErrFileTooLarge):
		return status.Error(codes.ResourceExhausted, err.Error())

	// Permission Denied / Prohibited
	case errors.Is(err, repository.ErrCannotUnlinkLastAuth),
		errors.Is(err, service.ErrUnauthorizedCommentAction):
		return status.Error(codes.PermissionDenied, err.Error())

	// Database Unavailable
	case errors.Is(err, service.ErrDatabaseUnavailable):
		return status.Error(codes.Unavailable, err.Error())

	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// ErrorUnaryInterceptor intercepts unary RPC calls, recovers from panics, and maps internal errors.
func ErrorUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[gRPC Panic] %s: %v\n", info.FullMethod, r)
				err = status.Errorf(codes.Internal, "Internal server error: %v", r)
			}
		}()
		resp, err = handler(ctx, req)
		return resp, MapError(err)
	}
}

// ErrorStreamInterceptor intercepts stream RPC calls, recovers from panics, and maps internal errors.
func ErrorStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[gRPC Stream Panic] %s: %v\n", info.FullMethod, r)
				err = status.Errorf(codes.Internal, "Internal server error: %v", r)
			}
		}()
		err = handler(srv, ss)
		return MapError(err)
	}
}

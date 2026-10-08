package server

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TokenServer struct {
	realmv1.UnimplementedTokenServiceServer
	tokenSvc  service.TokenService
	adminRepo repository.AdminRepository
}

func NewTokenServer(tokenSvc service.TokenService, adminRepo repository.AdminRepository) *TokenServer {
	return &TokenServer{
		tokenSvc:  tokenSvc,
		adminRepo: adminRepo,
	}
}

func mapTokenDTOToProto(dto *model.TokenDTO) *realmv1.APIToken {
	if dto == nil {
		return nil
	}

	var lastUsedStr, expiresStr *string
	if dto.LastUsedAt != nil {
		s := dto.LastUsedAt.Format(time.RFC3339)
		lastUsedStr = &s
	}
	if dto.ExpiresAt != nil {
		s := dto.ExpiresAt.Format(time.RFC3339)
		expiresStr = &s
	}

	return &realmv1.APIToken{
		Id:           dto.ID.String(),
		Name:         dto.Name,
		TokenPrefix:  dto.TokenPrefix,
		Scopes:       dto.Scopes,
		RateLimitRpm: int32(dto.RateLimitRPM),
		LastUsedAt:   lastUsedStr,
		ExpiresAt:    expiresStr,
		IsRevoked:    dto.IsRevoked,
		CreatedAt:    dto.CreatedAt.Format(time.RFC3339),
	}
}

func (s *TokenServer) ListTokens(ctx context.Context, req *realmv1.ListTokensRequest) (*realmv1.ListTokensResponse, error) {
	if err := Authorize(ctx, s.adminRepo, "tokens:manage"); err != nil {
		return nil, err
	}

	tokens, err := s.tokenSvc.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to list tokens: %v", err)
	}

	protoTokens := make([]*realmv1.APIToken, len(tokens))
	for i := range tokens {
		protoTokens[i] = mapTokenDTOToProto(&tokens[i])
	}

	return &realmv1.ListTokensResponse{
		Tokens: protoTokens,
	}, nil
}

func (s *TokenServer) CreateToken(ctx context.Context, req *realmv1.CreateTokenRequest) (*realmv1.CreateTokenResponse, error) {
	if err := Authorize(ctx, s.adminRepo, "tokens:manage"); err != nil {
		return nil, err
	}

	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "Token name cannot be empty")
	}

	var expiresIn time.Duration
	if req.ExpiresInSeconds != nil && *req.ExpiresInSeconds > 0 {
		expiresIn = time.Duration(*req.ExpiresInSeconds) * time.Second
	}

	input := model.TokenCreateInput{
		Name:         name,
		Scopes:       req.GetScopes(),
		RateLimitRPM: int(req.GetRateLimitRpm()),
		ExpiresIn:    expiresIn,
	}

	result, err := s.tokenSvc.Create(ctx, input)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create token: %v", err)
	}

	return &realmv1.CreateTokenResponse{
		Token:    mapTokenDTOToProto(&result.Token),
		RawToken: result.Raw,
	}, nil
}

func (s *TokenServer) RevokeToken(ctx context.Context, req *realmv1.RevokeTokenRequest) (*realmv1.RevokeTokenResponse, error) {
	if err := Authorize(ctx, s.adminRepo, "tokens:manage"); err != nil {
		return nil, err
	}

	tokenID, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid token UUID")
	}

	if err := s.tokenSvc.Revoke(ctx, tokenID); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to revoke token: %v", err)
	}

	return &realmv1.RevokeTokenResponse{
		Status:  "success",
		Message: "Token revoked successfully",
	}, nil
}

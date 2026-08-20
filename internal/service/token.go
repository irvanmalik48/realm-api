package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
)

var (
	ErrInvalidToken   = errors.New("invalid or malformed api token")
	ErrTokenExpired   = errors.New("api token has expired")
	ErrTokenRevoked   = errors.New("api token has been revoked")
	ErrTokenForbidden = errors.New("api token lacks required permissions")
)

type TokenService interface {
	Create(ctx context.Context, input model.TokenCreateInput) (*model.TokenCreateResult, error)
	Verify(ctx context.Context, rawToken string) (*model.APIToken, error)
	List(ctx context.Context) ([]model.TokenDTO, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	HasScope(token *model.APIToken, requiredScope string) bool
	HashToken(rawToken string) string
}

type tokenService struct {
	repo    repository.TokenRepository
	cache   *auth.TokenCache
	limiter *auth.TokenRateLimiter
}

func NewTokenService(repo repository.TokenRepository, cache *auth.TokenCache, limiter *auth.TokenRateLimiter) TokenService {
	return &tokenService{
		repo:    repo,
		cache:   cache,
		limiter: limiter,
	}
}

func (s *tokenService) HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func (s *tokenService) Create(ctx context.Context, input model.TokenCreateInput) (*model.TokenCreateResult, error) {
	if s.repo == nil {
		return nil, errors.New("database repository required to create tokens")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "default-token"
	}

	rpm := input.RateLimitRPM
	if rpm <= 0 {
		rpm = 60
	}

	scopes := input.Scopes
	if len(scopes) == 0 {
		scopes = []string{"*"}
	}

	// Generate 32 bytes cryptographically secure random secret
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random entropy for token: %w", err)
	}

	rawToken := "realm_tok_" + base64.RawURLEncoding.EncodeToString(randBytes)
	tokenPrefix := rawToken[:14] + "..."
	tokenHash := s.HashToken(rawToken)

	var expiresAt *time.Time
	if input.ExpiresIn > 0 {
		exp := time.Now().UTC().Add(input.ExpiresIn)
		expiresAt = &exp
	}

	tokenID := uuid.New()
	record := &model.APIToken{
		ID:           tokenID,
		Name:         name,
		TokenPrefix:  tokenPrefix,
		TokenHash:    tokenHash,
		Scopes:       scopes,
		RateLimitRPM: rpm,
		ExpiresAt:    expiresAt,
		IsRevoked:    false,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to save api token to database: %w", err)
	}

	dto := model.TokenDTO{
		ID:           record.ID,
		Name:         record.Name,
		TokenPrefix:  record.TokenPrefix,
		Scopes:       record.Scopes,
		RateLimitRPM: record.RateLimitRPM,
		ExpiresAt:    record.ExpiresAt,
		IsRevoked:    record.IsRevoked,
		CreatedAt:    record.CreatedAt,
	}

	return &model.TokenCreateResult{
		Token: dto,
		Raw:   rawToken,
	}, nil
}

func (s *tokenService) Verify(ctx context.Context, rawToken string) (*model.APIToken, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || !strings.HasPrefix(rawToken, "realm_tok_") {
		return nil, ErrInvalidToken
	}

	tokenHash := s.HashToken(rawToken)

	// Check in-memory cache
	if s.cache != nil {
		if cached, ok := s.cache.Get(tokenHash); ok {
			if cached.IsRevoked {
				return nil, ErrTokenRevoked
			}
			if cached.ExpiresAt != nil && time.Now().After(*cached.ExpiresAt) {
				return nil, ErrTokenExpired
			}
			return cached, nil
		}
	}

	if s.repo == nil {
		return nil, errors.New("database repository unavailable for token verification")
	}

	token, err := s.repo.GetByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, repository.ErrTokenNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	if token.IsRevoked {
		return nil, ErrTokenRevoked
	}

	if token.ExpiresAt != nil && time.Now().After(*token.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Update cache
	if s.cache != nil {
		s.cache.Set(tokenHash, token)
	}

	// Asynchronously update last used timestamp
	go func(id uuid.UUID) {
		_ = s.repo.UpdateLastUsed(context.Background(), id)
	}(token.ID)

	return token, nil
}

func (s *tokenService) List(ctx context.Context) ([]model.TokenDTO, error) {
	if s.repo == nil {
		return nil, errors.New("database repository unavailable")
	}

	tokens, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	dtos := make([]model.TokenDTO, 0, len(tokens))
	for _, t := range tokens {
		dtos = append(dtos, model.TokenDTO{
			ID:           t.ID,
			Name:         t.Name,
			TokenPrefix:  t.TokenPrefix,
			Scopes:       t.Scopes,
			RateLimitRPM: t.RateLimitRPM,
			LastUsedAt:   t.LastUsedAt,
			ExpiresAt:    t.ExpiresAt,
			IsRevoked:    t.IsRevoked,
			CreatedAt:    t.CreatedAt,
		})
	}
	return dtos, nil
}

func (s *tokenService) Revoke(ctx context.Context, id uuid.UUID) error {
	if s.repo == nil {
		return errors.New("database repository unavailable")
	}

	if err := s.repo.Revoke(ctx, id); err != nil {
		return err
	}

	if s.cache != nil {
		s.cache.InvalidateByID(id)
	}

	return nil
}

func (s *tokenService) HasScope(token *model.APIToken, requiredScope string) bool {
	if token == nil {
		return false
	}
	for _, s := range token.Scopes {
		if s == "*" || s == "all" || s == "admin" || s == requiredScope {
			return true
		}
		// Prefix matching (e.g. storage:* matches storage:write)
		if strings.HasSuffix(s, ":*") && strings.HasPrefix(requiredScope, strings.TrimSuffix(s, ":*")+":") {
			return true
		}
	}
	return false
}

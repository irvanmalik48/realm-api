package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/middleware"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
)

type mockTokenRepo struct {
	tokens map[string]*model.APIToken
	byID   map[uuid.UUID]*model.APIToken
}

func newMockTokenRepo() *mockTokenRepo {
	return &mockTokenRepo{
		tokens: make(map[string]*model.APIToken),
		byID:   make(map[uuid.UUID]*model.APIToken),
	}
}

func (m *mockTokenRepo) Create(ctx context.Context, token *model.APIToken) error {
	m.tokens[token.TokenHash] = token
	m.byID[token.ID] = token
	return nil
}

func (m *mockTokenRepo) GetByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	t, ok := m.tokens[hash]
	if !ok {
		return nil, repository.ErrTokenNotFound
	}
	return t, nil
}

func (m *mockTokenRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.APIToken, error) {
	t, ok := m.byID[id]
	if !ok {
		return nil, repository.ErrTokenNotFound
	}
	return t, nil
}

func (m *mockTokenRepo) List(ctx context.Context) ([]model.APIToken, error) {
	var list []model.APIToken
	for _, t := range m.tokens {
		list = append(list, *t)
	}
	return list, nil
}

func (m *mockTokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	t, ok := m.byID[id]
	if !ok {
		return repository.ErrTokenNotFound
	}
	t.IsRevoked = true
	return nil
}

func (m *mockTokenRepo) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	if t, ok := m.byID[id]; ok {
		now := time.Now().UTC()
		t.LastUsedAt = &now
	}
	return nil
}

func TestToken_GenerationAndVerification(t *testing.T) {
	repo := newMockTokenRepo()
	cache := auth.NewTokenCache(1 * time.Minute)
	limiter := auth.NewTokenRateLimiter()
	svc := service.NewTokenService(repo, cache, limiter)

	ctx := context.Background()

	// 1. Create Token
	result, err := svc.Create(ctx, model.TokenCreateInput{
		Name:         "test-app",
		Scopes:       []string{"storage:write", "contact:read"},
		RateLimitRPM: 30,
		ExpiresIn:    24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	if !strings.HasPrefix(result.Raw, "realm_tok_") {
		t.Errorf("expected token prefix 'realm_tok_', got %s", result.Raw)
	}
	if result.Token.Name != "test-app" {
		t.Errorf("expected name 'test-app', got %s", result.Token.Name)
	}
	if result.Token.RateLimitRPM != 30 {
		t.Errorf("expected RPM 30, got %d", result.Token.RateLimitRPM)
	}

	// 2. Verify Valid Token
	verified, err := svc.Verify(ctx, result.Raw)
	if err != nil {
		t.Fatalf("failed to verify valid token: %v", err)
	}
	if verified.ID != result.Token.ID {
		t.Errorf("expected token ID %s, got %s", result.Token.ID, verified.ID)
	}

	// 3. Check Scope Authorization
	if !svc.HasScope(verified, "storage:write") {
		t.Errorf("expected token to have scope 'storage:write'")
	}
	if svc.HasScope(verified, "admin:delete") {
		t.Errorf("token should not have scope 'admin:delete'")
	}

	// 4. Verify Malformed Token Rejection
	_, err = svc.Verify(ctx, "invalid_token_format")
	if err == nil {
		t.Errorf("expected error for malformed token")
	}

	// 5. Revoke Token
	err = svc.Revoke(ctx, result.Token.ID)
	if err != nil {
		t.Fatalf("failed to revoke token: %v", err)
	}

	_, err = svc.Verify(ctx, result.Raw)
	if err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Errorf("expected revoked token error, got %v", err)
	}
}

func TestToken_RateLimitingMiddleware(t *testing.T) {
	repo := newMockTokenRepo()
	cache := auth.NewTokenCache(1 * time.Minute)
	limiter := auth.NewTokenRateLimiter()
	svc := service.NewTokenService(repo, cache, limiter)
	cfg := &config.Config{Environment: "test"}

	ctx := context.Background()

	// Create token with strict 3 requests per minute limit
	result, err := svc.Create(ctx, model.TokenCreateInput{
		Name:         "rate-limited-app",
		Scopes:       []string{"*"},
		RateLimitRPM: 3,
	})
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	app := fiber.New()
	app.Get("/protected", middleware.RequireToken(cfg, svc, limiter, "storage:write"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// 1. Request without token -> 401
	req0 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp0, _ := app.Test(req0, -1)
	if resp0.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp0.StatusCode)
	}

	// 2. Perform 3 successful requests
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+result.Raw)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 for request %d, got %d", i, resp.StatusCode)
		}
		if resp.Header.Get("X-RateLimit-Limit") != "3" {
			t.Errorf("expected X-RateLimit-Limit 3, got %s", resp.Header.Get("X-RateLimit-Limit"))
		}
	}

	// 3. 4th request must be rate limited (429 Too Many Requests)
	req4 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req4.Header.Set("Authorization", "Bearer "+result.Raw)
	resp4, _ := app.Test(req4, -1)
	if resp4.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests on 4th request, got %d", resp4.StatusCode)
	}
	if resp4.Header.Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("expected X-RateLimit-Remaining 0, got %s", resp4.Header.Get("X-RateLimit-Remaining"))
	}
}

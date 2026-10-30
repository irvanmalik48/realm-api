package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/router"
	"github.com/irvanmalik48/realm-api/internal/service"
)

// MockPasteRepository for testing service logic without postgres
type mockPasteRepository struct {
	pastes map[string]*model.Paste
}

func newMockPasteRepository() *mockPasteRepository {
	return &mockPasteRepository{
		pastes: make(map[string]*model.Paste),
	}
}

func (m *mockPasteRepository) CreatePaste(ctx context.Context, paste *model.Paste) (*model.Paste, error) {
	if paste.Slug == "" {
		slug, err := repository.Generate7LetterSlug()
		if err != nil {
			return nil, err
		}
		paste.Slug = slug
	}
	paste.ID = uuid.New()
	paste.CreatedAt = time.Now()
	paste.UpdatedAt = time.Now()
	paste.Views = 0
	saved := *paste
	m.pastes[paste.Slug] = &saved
	ret := *paste
	return &ret, nil
}

func (m *mockPasteRepository) GetPasteBySlug(ctx context.Context, slug string) (*model.Paste, error) {
	p, ok := m.pastes[slug]
	if !ok {
		return nil, repository.ErrPasteNotFound
	}
	if p.ExpiresAt != nil && p.ExpiresAt.Before(time.Now()) {
		return nil, repository.ErrPasteExpired
	}
	ret := *p
	if p.BurnAfterRead {
		delete(m.pastes, slug)
	}
	return &ret, nil
}

func (m *mockPasteRepository) IncrementViews(ctx context.Context, slug string) error {
	if p, ok := m.pastes[slug]; ok {
		p.Views++
	}
	return nil
}

func (m *mockPasteRepository) ListPastes(ctx context.Context, search, syntax string, isPublicOnly *bool, userID *uuid.UUID, limit, offset int) (*model.PastesListResponse, error) {
	var list []model.PasteSummaryDTO
	for _, p := range m.pastes {
		if p.ExpiresAt != nil && p.ExpiresAt.Before(time.Now()) {
			continue
		}
		if p.BurnAfterRead {
			continue
		}
		if isPublicOnly != nil && *isPublicOnly && !p.IsPublic {
			continue
		}
		list = append(list, model.PasteSummaryDTO{
			ID:            p.ID,
			Slug:          p.Slug,
			Title:         p.Title,
			Syntax:        p.Syntax,
			UserID:        p.UserID,
			IsPublic:      p.IsPublic,
			IsEncrypted:   p.IsEncrypted,
			BurnAfterRead: p.BurnAfterRead,
			Views:         p.Views,
			Size:          len(p.Content),
			ExpiresAt:     p.ExpiresAt,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}
	return &model.PastesListResponse{
		Total:  len(list),
		Pastes: list,
	}, nil
}

func (m *mockPasteRepository) DeletePaste(ctx context.Context, slug string) error {
	if _, ok := m.pastes[slug]; !ok {
		return repository.ErrPasteNotFound
	}
	delete(m.pastes, slug)
	return nil
}

func TestGenerate7LetterSlug(t *testing.T) {
	letterRegex := regexp.MustCompile(`^[a-zA-Z]{7}$`)
	generated := make(map[string]bool)

	for i := 0; i < 500; i++ {
		slug, err := repository.Generate7LetterSlug()
		if err != nil {
			t.Fatalf("unexpected error generating slug: %v", err)
		}

		if len(slug) != 7 {
			t.Errorf("expected slug length 7, got %d (slug: %s)", len(slug), slug)
		}

		if !letterRegex.MatchString(slug) {
			t.Errorf("expected 7-letter slug matching [a-zA-Z]{7}, got %s", slug)
		}

		if generated[slug] {
			t.Errorf("duplicate slug generated within 500 iterations: %s", slug)
		}
		generated[slug] = true
	}
}

func TestPasteService_CreateAndGet(t *testing.T) {
	repo := newMockPasteRepository()
	svc := service.NewPasteService(repo)
	ctx := context.Background()

	// 1. Test empty content
	_, err := svc.CreatePaste(ctx, &model.CreatePasteRequest{
		Content: "   ",
	}, nil)
	if err != service.ErrEmptyPasteContent {
		t.Errorf("expected ErrEmptyPasteContent, got %v", err)
	}

	// 2. Test valid paste with 1h TTL
	paste, err := svc.CreatePaste(ctx, &model.CreatePasteRequest{
		Title:   "main.go",
		Content: "package main\n\nfunc main() {}",
		Syntax:  "go",
		TTL:     "1h",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error creating paste: %v", err)
	}

	if len(paste.Slug) != 7 {
		t.Errorf("expected 7-letter slug, got %q", paste.Slug)
	}
	if paste.Syntax != "go" {
		t.Errorf("expected syntax 'go', got %q", paste.Syntax)
	}
	if paste.ExpiresAt == nil {
		t.Errorf("expected expires_at to be set for 1h TTL")
	}

	// 3. Test get paste
	fetched, err := svc.GetPasteBySlug(ctx, paste.Slug, "")
	if err != nil {
		t.Fatalf("unexpected error getting paste: %v", err)
	}
	if fetched.Content != paste.Content {
		t.Errorf("content mismatch: expected %q, got %q", paste.Content, fetched.Content)
	}

	// 4. Test raw content
	raw, err := svc.GetRawContent(ctx, paste.Slug, "")
	if err != nil {
		t.Fatalf("unexpected error getting raw content: %v", err)
	}
	if raw != paste.Content {
		t.Errorf("raw content mismatch: expected %q, got %q", paste.Content, raw)
	}

	// 5. Test expired paste
	expiredTime := time.Now().Add(-1 * time.Minute)
	repo.pastes[paste.Slug].ExpiresAt = &expiredTime
	_, err = svc.GetPasteBySlug(ctx, paste.Slug, "")
	if err != repository.ErrPasteExpired {
		t.Errorf("expected ErrPasteExpired for past expires_at, got %v", err)
	}
}

func TestPasteService_PasswordEncryption(t *testing.T) {
	repo := newMockPasteRepository()
	svc := service.NewPasteService(repo)
	ctx := context.Background()

	originalContent := "Top secret server password = hunter2"
	correctPassword := "p@ssw0rd!42"

	// 1. Create encrypted paste
	paste, err := svc.CreatePaste(ctx, &model.CreatePasteRequest{
		Title:    "secret.txt",
		Content:  originalContent,
		Syntax:   "plaintext",
		Password: correctPassword,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error creating encrypted paste: %v", err)
	}

	if !paste.IsEncrypted {
		t.Errorf("expected paste.IsEncrypted to be true")
	}

	// Creator gets plaintext back on creation
	if paste.Content != originalContent {
		t.Errorf("expected creator response content to be plaintext %q, got %q", originalContent, paste.Content)
	}

	// In repository, content must be encrypted with $paste_v1$
	storedInRepo := repo.pastes[paste.Slug]
	if storedInRepo.Content == originalContent {
		t.Errorf("stored content in repository must not be plaintext!")
	}
	if !strings.HasPrefix(storedInRepo.Content, "$paste_v1$") {
		t.Errorf("stored content in repository should have $paste_v1$ prefix, got %q", storedInRepo.Content)
	}

	// 2. Get paste without password -> should return empty content and IsEncrypted = true
	locked, err := svc.GetPasteBySlug(ctx, paste.Slug, "")
	if err != nil {
		t.Fatalf("unexpected error getting paste without password: %v", err)
	}
	if locked.Content != "" {
		t.Errorf("expected empty content when password is omitted, got %q", locked.Content)
	}
	if !locked.IsEncrypted {
		t.Errorf("expected locked.IsEncrypted to be true")
	}

	// 3. Get raw paste without password -> should return ErrPasswordRequired
	_, err = svc.GetRawContent(ctx, paste.Slug, "")
	if err != service.ErrPasswordRequired {
		t.Errorf("expected ErrPasswordRequired, got %v", err)
	}

	// 4. Get paste with wrong password -> should return ErrInvalidPassword
	_, err = svc.GetPasteBySlug(ctx, paste.Slug, "wrong-password")
	if err != service.ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}

	// 5. Get raw paste with wrong password -> should return ErrInvalidPassword
	_, err = svc.GetRawContent(ctx, paste.Slug, "wrong-password")
	if err != service.ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}

	// 6. Get paste with correct password -> successfully decrypted!
	unlocked, err := svc.GetPasteBySlug(ctx, paste.Slug, correctPassword)
	if err != nil {
		t.Fatalf("failed to decrypt paste with correct password: %v", err)
	}
	if unlocked.Content != originalContent {
		t.Errorf("decrypted content mismatch: expected %q, got %q", originalContent, unlocked.Content)
	}

	// 7. Get raw paste with correct password -> successfully decrypted!
	rawUnlocked, err := svc.GetRawContent(ctx, paste.Slug, correctPassword)
	if err != nil {
		t.Fatalf("failed to decrypt raw paste with correct password: %v", err)
	}
	if rawUnlocked != originalContent {
		t.Errorf("raw decrypted content mismatch: expected %q, got %q", originalContent, rawUnlocked)
	}
}

func TestPasteEndpoints_Validation(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
		StorageDir:  t.TempDir(),
	}
	app := router.New(cfg, nil)

	// Test GET non-existent paste
	req := httptest.NewRequest(http.MethodGet, "/v1/pastes/abcdefg", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 (since db is nil in mock), got %d", resp.StatusCode)
	}

	// Test invalid payload on POST /v1/pastes
	badReq := httptest.NewRequest(http.MethodPost, "/v1/pastes", bytes.NewBufferString("invalid json"))
	badReq.Header.Set("Content-Type", "application/json")
	badResp, err := app.Test(badReq, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if badResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for bad json, got %d", badResp.StatusCode)
	}
	body, _ := io.ReadAll(badResp.Body)
	var errResp map[string]interface{}
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}
	if errResp["status"] != "error" {
		t.Errorf("expected status 'error', got %v", errResp["status"])
	}
}

func TestPasteService_BurnAfterRead(t *testing.T) {
	repo := newMockPasteRepository()
	svc := service.NewPasteService(repo)
	ctx := context.Background()

	// Create a paste with TTL "burn"
	created, err := svc.CreatePaste(ctx, &model.CreatePasteRequest{
		Content: "Confidential credentials",
		TTL:     "burn",
	}, nil)
	if err != nil {
		t.Fatalf("failed to create burn paste: %v", err)
	}
	if !created.BurnAfterRead {
		t.Errorf("expected BurnAfterRead to be true, got %v", created.BurnAfterRead)
	}

	// First fetch must succeed
	p, err := svc.GetPasteBySlug(ctx, created.Slug, "")
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if p.Content != "Confidential credentials" {
		t.Errorf("unexpected content: %s", p.Content)
	}

	// Second fetch must fail because paste was burned
	_, err = svc.GetPasteBySlug(ctx, created.Slug, "")
	if err == nil {
		t.Fatal("expected second fetch to fail with ErrPasteNotFound, but it succeeded")
	}
	if !strings.Contains(err.Error(), "paste not found") {
		t.Errorf("expected paste not found error, got: %v", err)
	}
}

func TestPasteHandler_CLICurl(t *testing.T) {
	repo := newMockPasteRepository()
	svc := service.NewPasteService(repo)
	cfg := &config.Config{
		Environment: "test",
		FrontendURL: "https://paste.realm.dev",
	}

	app := fiber.New()
	pasteHdlr := handler.NewPasteHandler(cfg, svc)
	app.Post("/test/pastes", pasteHdlr.CreatePaste)
	app.Get("/test/pastes/:slug", pasteHdlr.GetPaste)

	// Simulate curl POST with raw plain text
	body := bytes.NewBufferString("package main\n\nfunc main() {}\n")
	req := httptest.NewRequest(http.MethodPost, "/test/pastes", body)
	req.Header.Set("User-Agent", "curl/8.5.0")
	req.Header.Set("Content-Type", "text/plain")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("curl post error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for curl, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)
	if !strings.HasPrefix(respStr, "https://paste.realm.dev/") {
		t.Fatalf("expected response starting with frontend URL, got: %s", respStr)
	}
	if !strings.HasSuffix(respStr, "\n") {
		t.Fatalf("expected response to end with trailing newline for terminal friendly display, got: %q", respStr)
	}

	slug := strings.TrimSpace(strings.TrimPrefix(respStr, "https://paste.realm.dev/"))

	// Simulate curl GET for raw paste
	getReq := httptest.NewRequest(http.MethodGet, "/test/pastes/"+slug, nil)
	getReq.Header.Set("User-Agent", "curl/8.5.0")
	getResp, err := app.Test(getReq, -1)
	if err != nil {
		t.Fatalf("curl get error: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for curl get, got %d", getResp.StatusCode)
	}

	rawBody, _ := io.ReadAll(getResp.Body)
	if string(rawBody) != "package main\n\nfunc main() {}\n" {
		t.Fatalf("unexpected content returned to curl: %q", string(rawBody))
	}
}

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/middleware"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
	"github.com/irvanmalik48/realm-api/internal/storage"
)

type mockStorageRepo struct {
	records map[uuid.UUID]*model.FileRecord
}

func newMockStorageRepo() *mockStorageRepo {
	return &mockStorageRepo{
		records: make(map[uuid.UUID]*model.FileRecord),
	}
}

func (m *mockStorageRepo) Create(ctx context.Context, record *model.FileRecord) error {
	m.records[record.ID] = record
	return nil
}

func (m *mockStorageRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.FileRecord, error) {
	rec, ok := m.records[id]
	if !ok {
		return nil, repository.ErrRecordNotFound
	}
	return rec, nil
}

func (m *mockStorageRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.records, id)
	return nil
}

func createTestPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// Draw colorful pattern
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 2), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func setupStorageTestApp(t *testing.T, withAuth bool) (*fiber.App, string, service.StorageService, string) {
	tempDir, err := os.MkdirTemp("", "realm-storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		StorageDir:      tempDir,
		MaxUploadSizeMB: 10,
	}

	engine, err := storage.NewZstdEngine(tempDir)
	if err != nil {
		t.Fatalf("failed to create zstd engine: %v", err)
	}

	repo := newMockStorageRepo()
	svc := service.NewStorageService(cfg, repo, engine)
	hdlr := handler.NewStorageHandler(cfg, svc)

	tokenRepo := newMockTokenRepo()
	tokenCache := auth.NewTokenCache(1 * time.Minute)
	tokenLimiter := auth.NewTokenRateLimiter()
	tokenSvc := service.NewTokenService(tokenRepo, tokenCache, tokenLimiter)

	var validToken string
	if withAuth {
		tokResult, err := tokenSvc.Create(context.Background(), model.TokenCreateInput{
			Name:         "test-storage",
			Scopes:       []string{"storage:write"},
			RateLimitRPM: 100,
		})
		if err != nil {
			t.Fatalf("failed to create test token: %v", err)
		}
		validToken = tokResult.Raw
	}

	app := fiber.New()
	v1 := app.Group("/v1/storage")
	v1.Post("/upload", middleware.RequireToken(tokenSvc, tokenLimiter, "storage:write"), hdlr.Upload)
	v1.Get("/:id", hdlr.GetFile)
	v1.Get("/:id/info", hdlr.GetFileInfo)
	v1.Delete("/:id", middleware.RequireToken(tokenSvc, tokenLimiter, "storage:write"), hdlr.DeleteFile)

	return app, tempDir, svc, validToken
}

func TestStorage_UploadAndServe(t *testing.T) {
	app, tempDir, _, validToken := setupStorageTestApp(t, true)
	defer os.RemoveAll(tempDir)

	pngBytes := createTestPNG()

	// 1. Upload sample PNG
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test-image.png")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = part.Write(pngBytes)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/storage/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+validToken)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("upload request error: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var uploadResp model.FileUploadResponse
	respBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBytes, &uploadResp); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}

	if uploadResp.Status != "success" {
		t.Errorf("expected success status, got %s", uploadResp.Status)
	}

	fileID := uploadResp.File.ID
	if uploadResp.File.Width == nil || *uploadResp.File.Width != 100 {
		t.Errorf("expected width 100, got %v", uploadResp.File.Width)
	}
	if uploadResp.File.Height == nil || *uploadResp.File.Height != 100 {
		t.Errorf("expected height 100, got %v", uploadResp.File.Height)
	}
	if uploadResp.File.Blurhash == nil || *uploadResp.File.Blurhash == "" {
		t.Errorf("expected blurhash to be generated, got empty")
	}

	// 2. Fetch original image
	getReq := httptest.NewRequest(http.MethodGet, "/v1/storage/"+fileID.String(), nil)
	getResp, err := app.Test(getReq, -1)
	if err != nil {
		t.Fatalf("get request error: %v", err)
	}

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", getResp.StatusCode)
	}

	if getResp.Header.Get("X-Blurhash") == "" {
		t.Errorf("expected X-Blurhash header on image response")
	}
	if getResp.Header.Get("ETag") == "" {
		t.Errorf("expected ETag header on image response")
	}

	downloadedBytes, _ := io.ReadAll(getResp.Body)
	if len(downloadedBytes) != len(pngBytes) {
		t.Errorf("expected %d bytes, got %d", len(pngBytes), len(downloadedBytes))
	}

	// 3. Test ETag 304 Not Modified
	etag := getResp.Header.Get("ETag")
	cacheReq := httptest.NewRequest(http.MethodGet, "/v1/storage/"+fileID.String(), nil)
	cacheReq.Header.Set("If-None-Match", etag)
	cacheResp, _ := app.Test(cacheReq, -1)
	if cacheResp.StatusCode != http.StatusNotModified {
		t.Errorf("expected 304 Not Modified, got %d", cacheResp.StatusCode)
	}

	// 4. Test endpoint-level WebP conversion
	webpReq := httptest.NewRequest(http.MethodGet, "/v1/storage/"+fileID.String()+"?format=webp", nil)
	webpResp, err := app.Test(webpReq, -1)
	if err != nil {
		t.Fatalf("webp request error: %v", err)
	}

	if webpResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for webp, got %d", webpResp.StatusCode)
	}

	if webpResp.Header.Get("Content-Type") != "image/webp" {
		t.Errorf("expected Content-Type image/webp, got %s", webpResp.Header.Get("Content-Type"))
	}

	webpBytes, _ := io.ReadAll(webpResp.Body)
	if len(webpBytes) == 0 {
		t.Errorf("expected non-empty webp bytes")
	}

	// 5. Delete file
	delReq := httptest.NewRequest(http.MethodDelete, "/v1/storage/"+fileID.String(), nil)
	delReq.Header.Set("Authorization", "Bearer "+validToken)
	delResp, _ := app.Test(delReq, -1)
	if delResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for delete, got %d", delResp.StatusCode)
	}

	// 6. Verify file is gone
	goneReq := httptest.NewRequest(http.MethodGet, "/v1/storage/"+fileID.String(), nil)
	goneResp, _ := app.Test(goneReq, -1)
	if goneResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for deleted file, got %d", goneResp.StatusCode)
	}
}

func TestStorage_AuthProtection(t *testing.T) {
	app, tempDir, _, validToken := setupStorageTestApp(t, true)
	defer os.RemoveAll(tempDir)

	// Unauthorized upload
	req1 := httptest.NewRequest(http.MethodPost, "/v1/storage/upload", nil)
	resp1, _ := app.Test(req1, -1)
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing key, got %d", resp1.StatusCode)
	}

	// Authorized upload with Authorization Bearer token
	pngBytes := createTestPNG()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "auth-test.png")
	_, _ = part.Write(pngBytes)
	_ = writer.Close()

	req2 := httptest.NewRequest(http.MethodPost, "/v1/storage/upload", body)
	req2.Header.Set("Content-Type", writer.FormDataContentType())
	req2.Header.Set("Authorization", "Bearer "+validToken)

	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("auth upload error: %v", err)
	}

	if resp2.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 for valid API token, got %d", resp2.StatusCode)
	}
}

func TestStorage_UploadSizeLimit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "realm-storage-limit-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set tight 1KB upload limit (0.001MB not supported so we set MaxUploadSizeMB to 1 and mock a 2MB payload)
	cfg := &config.Config{
		StorageDir:      tempDir,
		MaxUploadSizeMB: 1, // 1 MB limit
	}

	engine, _ := storage.NewZstdEngine(tempDir)
	repo := newMockStorageRepo()
	svc := service.NewStorageService(cfg, repo, engine)

	// Upload payload of 2MB
	largePayload := bytes.Repeat([]byte("A"), 2*1024*1024)
	_, err = svc.Upload(context.Background(), "large.txt", bytes.NewReader(largePayload), "text/plain")
	if err == nil {
		t.Fatalf("expected ErrFileTooLarge for 2MB file with 1MB limit")
	}
}

func TestStorage_WebPCache(t *testing.T) {
	app, tempDir, _, validToken := setupStorageTestApp(t, true)
	defer os.RemoveAll(tempDir)

	pngBytes := createTestPNG()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "cache-test.png")
	_, _ = part.Write(pngBytes)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/storage/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+validToken)

	resp, err := app.Test(req, -1)
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload failed: %v", err)
	}

	var uploadResp model.FileUploadResponse
	respBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(respBytes, &uploadResp)
	fileID := uploadResp.File.ID

	// First WebP request converts and caches
	req1 := httptest.NewRequest(http.MethodGet, "/v1/storage/"+fileID.String()+"?format=webp", nil)
	resp1, err := app.Test(req1, -1)
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("first webp request failed: %v", err)
	}
	bytes1, _ := io.ReadAll(resp1.Body)

	// Second WebP request loads from disk cache
	req2 := httptest.NewRequest(http.MethodGet, "/v1/storage/"+fileID.String()+"?format=webp", nil)
	resp2, err := app.Test(req2, -1)
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("second webp request failed: %v", err)
	}
	bytes2, _ := io.ReadAll(resp2.Body)

	if len(bytes1) != len(bytes2) {
		t.Errorf("cached webp size mismatch: %d vs %d", len(bytes1), len(bytes2))
	}
}

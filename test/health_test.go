package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/router"
)

func TestHealthHandler(t *testing.T) {
	cfg := &config.Config{Environment: "test"}
	app := router.New(cfg, nil)

	// Test GET /health
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("health request error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var healthResp handler.HealthResponse
	if err := json.Unmarshal(body, &healthResp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if healthResp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", healthResp.Status)
	}
	if healthResp.Service != "realm-api" {
		t.Errorf("expected service 'realm-api', got '%s'", healthResp.Service)
	}

	// Test GET /v1/health
	reqV1 := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	respV1, err := app.Test(reqV1, -1)
	if err != nil {
		t.Fatalf("v1 health request error: %v", err)
	}
	if respV1.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK for /v1/health, got %d", respV1.StatusCode)
	}
}

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

func TestRootHandler(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	app := router.New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var payload handler.RootResponsePayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Message != "Nothing to see here" || payload.Status != "success" {
		t.Errorf("unexpected body: %+v", payload)
	}
}

func TestNotFoundHandler(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	app := router.New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/non-existent", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var payload handler.ErrorResponsePayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Error != "Endpoint not found" || payload.Status != "error" {
		t.Errorf("unexpected body: %+v", payload)
	}
}

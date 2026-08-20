package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
)

type mockLastFMService struct {
	trackResponse *model.LastFMTrackResponseBody
	userResponse  *model.LastFMUserResponseBody
	err           error
}

func (m *mockLastFMService) GetRecentTracks(ctx context.Context, username string, limit int) (*model.LastFMTrackResponseBody, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.trackResponse, nil
}

func (m *mockLastFMService) GetUserInfo(ctx context.Context, username string) (*model.LastFMUserResponseBody, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.userResponse, nil
}

func setupTestApp(cfg *config.Config, svc service.LastFMService) *fiber.App {
	app := fiber.New()
	hdlr := handler.NewLastFMHandler(cfg, svc)

	v1 := app.Group("/v1/lastfm")
	v1.Get("/track", hdlr.GetRecentTracks)
	v1.Get("/user", hdlr.GetUserInfo)

	return app
}

func TestGetRecentTracks_MissingAPIKey(t *testing.T) {
	cfg := &config.Config{
		LastFMAPIKey: "",
	}
	svc := &mockLastFMService{}
	app := setupTestApp(cfg, svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/lastfm/track?username=test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestGetRecentTracks_MissingUsername(t *testing.T) {
	cfg := &config.Config{
		LastFMAPIKey: "dummy-key",
	}
	svc := &mockLastFMService{}
	app := setupTestApp(cfg, svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/lastfm/track", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestGetRecentTracks_InvalidLimit(t *testing.T) {
	cfg := &config.Config{
		LastFMAPIKey: "dummy-key",
	}
	svc := &mockLastFMService{}
	app := setupTestApp(cfg, svc)

	testCases := []string{
		"/v1/lastfm/track?username=test&limit=0",
		"/v1/lastfm/track?username=test&limit=201",
		"/v1/lastfm/track?username=test&limit=abc",
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(http.MethodGet, tc, nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 for path %s, got %d", tc, resp.StatusCode)
		}
	}
}

func TestGetRecentTracks_Success(t *testing.T) {
	cfg := &config.Config{
		LastFMAPIKey:           "dummy-key",
		CacheRevalidateSeconds: 900,
	}
	svc := &mockLastFMService{
		trackResponse: &model.LastFMTrackResponseBody{
			RecentTracks: model.RecentTracks{
				Track: []model.Track{
					{
						Name: "Test Track",
						Artist: model.TrackArtist{
							Text: "Test Artist",
						},
					},
				},
			},
		},
	}
	app := setupTestApp(cfg, svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/lastfm/track?username=test&limit=5", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	cacheControl := resp.Header.Get("Cache-Control")
	if cacheControl != "public, s-maxage=900, stale-while-revalidate=1800" {
		t.Errorf("expected Cache-Control header, got %s", cacheControl)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var body model.LastFMTrackResponseBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if len(body.RecentTracks.Track) != 1 || body.RecentTracks.Track[0].Name != "Test Track" {
		t.Errorf("unexpected body content: %+v", body)
	}
}

func TestGetUserInfo_Success(t *testing.T) {
	cfg := &config.Config{
		LastFMAPIKey:           "dummy-key",
		CacheRevalidateSeconds: 900,
	}
	svc := &mockLastFMService{
		userResponse: &model.LastFMUserResponseBody{
			User: model.LastFMUser{
				Name:      "testuser",
				PlayCount: "12345",
			},
		},
	}
	app := setupTestApp(cfg, svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/lastfm/user?username=testuser", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var body model.LastFMUserResponseBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.User.Name != "testuser" || body.User.PlayCount != "12345" {
		t.Errorf("unexpected user body content: %+v", body)
	}
}

func TestGetUserInfo_UpstreamError(t *testing.T) {
	cfg := &config.Config{
		LastFMAPIKey: "dummy-key",
	}
	svc := &mockLastFMService{
		err: &service.UpstreamError{
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		},
	}
	app := setupTestApp(cfg, svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/lastfm/user?username=unknown", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestGetUserInfo_GenericError(t *testing.T) {
	cfg := &config.Config{
		LastFMAPIKey: "dummy-key",
	}
	svc := &mockLastFMService{
		err: errors.New("connection failed"),
	}
	app := setupTestApp(cfg, svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/lastfm/user?username=test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", resp.StatusCode)
	}
}

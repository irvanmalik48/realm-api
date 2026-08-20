package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/middleware"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/security"
	"github.com/irvanmalik48/realm-api/internal/service"
)

type mockContactService struct {
	err error
}

func (m *mockContactService) SendMessage(ctx context.Context, req *model.ContactRequest, ipAddress, userAgent string) (*model.ContactSubmission, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.ContactSubmission{
		ID:        uuid.New(),
		Name:      req.Name,
		Email:     req.Email,
		Subject:   req.Subject,
		Message:   req.Message,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		CreatedAt: time.Now(),
	}, nil
}

func setupContactTestApp(cfg *config.Config, svc service.ContactService) *fiber.App {
	app := fiber.New()
	hdlr := handler.NewContactHandler(cfg, svc)
	app.Post("/v1/contact", hdlr.Handle)
	return app
}

func TestContactHandler_Success(t *testing.T) {
	cfg := &config.Config{}
	svc := &mockContactService{}
	app := setupContactTestApp(cfg, svc)

	payload := model.ContactRequest{
		Name:    "Irvan Malik",
		Email:   "irvan@example.com",
		Subject: "Collaboration Inquiry",
		Message: "Hello, I would like to discuss a potential project collaboration.",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/contact", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var respPayload model.ContactResponse
	if err := json.Unmarshal(bodyBytes, &respPayload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respPayload.Status != "success" || respPayload.Message != "Your message has been sent successfully." {
		t.Errorf("unexpected response content: %+v", respPayload)
	}
}

func TestContactHandler_HoneypotTriggered(t *testing.T) {
	cfg := &config.Config{}
	svc := &mockContactService{}
	app := setupContactTestApp(cfg, svc)

	payload := model.ContactRequest{
		Name:     "Spam Bot",
		Email:    "bot@spam.com",
		Subject:  "Buy crypto now",
		Message:  "Click here to buy tokens cheap!",
		Honeypot: "http://spamlink.com",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/contact", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for honeypot drop, got %d", resp.StatusCode)
	}
}

func TestContactHandler_ValidationErrors(t *testing.T) {
	cfg := &config.Config{}
	svc := &mockContactService{}
	app := setupContactTestApp(cfg, svc)

	testCases := []struct {
		name    string
		payload model.ContactRequest
	}{
		{
			name: "missing name",
			payload: model.ContactRequest{
				Name:    "",
				Email:   "test@example.com",
				Subject: "Valid Subject",
				Message: "Valid message with more than 10 characters.",
			},
		},
		{
			name: "short name",
			payload: model.ContactRequest{
				Name:    "a",
				Email:   "test@example.com",
				Subject: "Valid Subject",
				Message: "Valid message with more than 10 characters.",
			},
		},
		{
			name: "missing email",
			payload: model.ContactRequest{
				Name:    "Valid Name",
				Email:   "",
				Subject: "Valid Subject",
				Message: "Valid message with more than 10 characters.",
			},
		},
		{
			name: "invalid email format",
			payload: model.ContactRequest{
				Name:    "Valid Name",
				Email:   "not-an-email",
				Subject: "Valid Subject",
				Message: "Valid message with more than 10 characters.",
			},
		},
		{
			name: "missing subject",
			payload: model.ContactRequest{
				Name:    "Valid Name",
				Email:   "test@example.com",
				Subject: "",
				Message: "Valid message with more than 10 characters.",
			},
		},
		{
			name: "short subject",
			payload: model.ContactRequest{
				Name:    "Valid Name",
				Email:   "test@example.com",
				Subject: "ab",
				Message: "Valid message with more than 10 characters.",
			},
		},
		{
			name: "missing message",
			payload: model.ContactRequest{
				Name:    "Valid Name",
				Email:   "test@example.com",
				Subject: "Valid Subject",
				Message: "",
			},
		},
		{
			name: "short message",
			payload: model.ContactRequest{
				Name:    "Valid Name",
				Email:   "test@example.com",
				Subject: "Valid Subject",
				Message: "Short",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/contact", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status 400 for %s, got %d", tc.name, resp.StatusCode)
			}
		})
	}
}

func TestContactHandler_ServiceError(t *testing.T) {
	cfg := &config.Config{}
	svc := &mockContactService{
		err: errors.New("storage error"),
	}
	app := setupContactTestApp(cfg, svc)

	payload := model.ContactRequest{
		Name:    "Irvan Malik",
		Email:   "irvan@example.com",
		Subject: "Collaboration Inquiry",
		Message: "Hello, I would like to discuss a potential project collaboration.",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/contact", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestCSRFMiddleware(t *testing.T) {
	cfg := &config.Config{
		AllowedOrigins: "https://irvanma.eu.org,http://localhost:3000",
	}

	app := fiber.New()
	app.Post("/test", middleware.CSRFProtection(cfg), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Missing security header
	req1 := httptest.NewRequest(http.MethodPost, "/test", nil)
	resp1, _ := app.Test(req1, -1)
	if resp1.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing custom header, got %d", resp1.StatusCode)
	}

	// Disallowed Origin
	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req2.Header.Set("X-Realm-Request", "1")
	req2.Header.Set("Origin", "https://malicious-site.com")
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed origin, got %d", resp2.StatusCode)
	}

	// Allowed Origin
	req3 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req3.Header.Set("X-Realm-Request", "1")
	req3.Header.Set("Origin", "https://irvanma.eu.org")
	resp3, _ := app.Test(req3, -1)
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for allowed origin, got %d", resp3.StatusCode)
	}
}

func TestSSRFSecurity(t *testing.T) {
	// Test private IP checking
	if !security.IsPrivateIP(net.ParseIP("127.0.0.1")) {
		t.Errorf("127.0.0.1 should be detected as private")
	}
	if !security.IsPrivateIP(net.ParseIP("10.0.0.1")) {
		t.Errorf("10.0.0.1 should be detected as private")
	}
	if !security.IsPrivateIP(net.ParseIP("192.168.1.1")) {
		t.Errorf("192.168.1.1 should be detected as private")
	}
	if !security.IsPrivateIP(net.ParseIP("169.254.169.254")) {
		t.Errorf("169.254.169.254 should be detected as private")
	}
	if security.IsPrivateIP(net.ParseIP("8.8.8.8")) {
		t.Errorf("8.8.8.8 should NOT be detected as private")
	}

	// Test Discord webhook validation
	if err := security.ValidateDiscordWebhookURL("https://discord.com/api/webhooks/123/abc"); err != nil {
		t.Errorf("valid discord webhook should pass: %v", err)
	}
	if err := security.ValidateDiscordWebhookURL("http://discord.com/api/webhooks/123/abc"); err == nil {
		t.Errorf("insecure http discord webhook should fail")
	}
	if err := security.ValidateDiscordWebhookURL("https://evil.com/api/webhooks/123/abc"); err == nil {
		t.Errorf("non-discord webhook should fail")
	}
}

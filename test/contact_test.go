package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/model"
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

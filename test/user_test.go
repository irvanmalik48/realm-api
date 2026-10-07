package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/model"
)

func TestUserHandler_ListAndUsers(t *testing.T) {
	repo := newMockUserRepo()
	u1 := &model.User{
		ID:        uuid.New(),
		Email:     "user1@example.com",
		Username:  "user1",
		FullName:  "User One",
		Provider:  "local",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.Create(context.Background(), u1)

	cfg := &config.Config{}
	hdlr := handler.NewUserHandler(cfg, repo)

	app := fiber.New()
	app.Get("/v1/users", hdlr.ListUsers)
	app.Delete("/v1/users/:id", hdlr.DeleteUser)

	// 1. List users
	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var listRes handler.ListUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&listRes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if listRes.Total != 1 {
		t.Fatalf("expected total 1, got %d", listRes.Total)
	}

	// 2. Delete user
	delReq := httptest.NewRequest(http.MethodDelete, "/v1/users/"+u1.ID.String(), nil)
	delResp, err := app.Test(delReq)
	if err != nil {
		t.Fatalf("failed to test delete request: %v", err)
	}
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", delResp.StatusCode)
	}

	// 3. Delete non-existent user returns 404
	delReq2 := httptest.NewRequest(http.MethodDelete, "/v1/users/"+uuid.New().String(), nil)
	delResp2, err := app.Test(delReq2)
	if err != nil {
		t.Fatalf("failed to test delete request: %v", err)
	}
	if delResp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", delResp2.StatusCode)
	}
}

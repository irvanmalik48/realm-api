package test

import (
	"bytes"
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

func TestUserHandler_UpdateAndDeactivate(t *testing.T) {
	repo := newMockUserRepo()
	u := &model.User{
		ID:        uuid.New(),
		Email:     "target@example.com",
		Username:  "targetuser",
		FullName:  "Target User",
		Provider:  "local",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.Create(context.Background(), u)

	cfg := &config.Config{}
	hdlr := handler.NewUserHandler(cfg, repo)

	app := fiber.New()
	app.Patch("/v1/users/:id", hdlr.UpdateUser)

	// 1. Edit Full Name and Username
	newFullName := "Updated Name"
	newUsername := "updatedtarget"
	updateBody, _ := json.Marshal(model.UpdateUserInput{
		FullName: &newFullName,
		Username: &newUsername,
	})
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/users/"+u.ID.String(), bytes.NewReader(updateBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := app.Test(patchReq)
	if err != nil {
		t.Fatalf("failed to execute patch request: %v", err)
	}
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", patchResp.StatusCode)
	}

	var updateRes struct {
		Status string        `json:"status"`
		User   model.UserDTO `json:"user"`
	}
	if err := json.NewDecoder(patchResp.Body).Decode(&updateRes); err != nil {
		t.Fatalf("failed to decode patch response: %v", err)
	}
	if updateRes.User.FullName != newFullName || updateRes.User.Username != newUsername {
		t.Fatalf("expected name %s and username %s, got %s and %s", newFullName, newUsername, updateRes.User.FullName, updateRes.User.Username)
	}
	if !updateRes.User.IsActive {
		t.Fatalf("expected user to remain active")
	}

	// 2. Deactivate Account
	deactivate := false
	deactivateBody, _ := json.Marshal(model.UpdateUserInput{
		IsActive: &deactivate,
	})
	deactReq := httptest.NewRequest(http.MethodPatch, "/v1/users/"+u.ID.String(), bytes.NewReader(deactivateBody))
	deactReq.Header.Set("Content-Type", "application/json")
	deactResp, err := app.Test(deactReq)
	if err != nil {
		t.Fatalf("failed to execute deactivation request: %v", err)
	}
	if deactResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", deactResp.StatusCode)
	}

	var deactRes struct {
		Status string        `json:"status"`
		User   model.UserDTO `json:"user"`
	}
	_ = json.NewDecoder(deactResp.Body).Decode(&deactRes)
	if deactRes.User.IsActive {
		t.Fatalf("expected user to be deactivated (is_active = false)")
	}

	// 3. Reactivate Account
	reactivate := true
	reactivateBody, _ := json.Marshal(model.UpdateUserInput{
		IsActive: &reactivate,
	})
	reactReq := httptest.NewRequest(http.MethodPatch, "/v1/users/"+u.ID.String(), bytes.NewReader(reactivateBody))
	reactReq.Header.Set("Content-Type", "application/json")
	reactResp, err := app.Test(reactReq)
	if err != nil {
		t.Fatalf("failed to execute reactivation request: %v", err)
	}
	if reactResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", reactResp.StatusCode)
	}

	var reactRes struct {
		Status string        `json:"status"`
		User   model.UserDTO `json:"user"`
	}
	_ = json.NewDecoder(reactResp.Body).Decode(&reactRes)
	if !reactRes.User.IsActive {
		t.Fatalf("expected user to be reactivated (is_active = true)")
	}
}


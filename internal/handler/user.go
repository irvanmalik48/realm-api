package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
)

type UserHandler struct {
	cfg      *config.Config
	userRepo repository.UserRepository
}

func NewUserHandler(cfg *config.Config, userRepo repository.UserRepository) *UserHandler {
	return &UserHandler{
		cfg:      cfg,
		userRepo: userRepo,
	}
}

type ListUsersResponse struct {
	Status string          `json:"status"`
	Users  []model.UserDTO `json:"users"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

func (h *UserHandler) ListUsers(c *fiber.Ctx) error {
	search := c.Query("search")
	provider := c.Query("provider")
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)

	users, total, err := h.userRepo.ListUsers(c.Context(), search, provider, limit, offset)
	if err != nil {
		log.Printf("[Users] ListUsers error: %v\n", err)
		return ErrorResponse(c, "Failed to retrieve users", http.StatusInternalServerError)
	}

	return JSONResponse(c, ListUsersResponse{
		Status: "success",
		Users:  users,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, http.StatusOK, 0)
}

func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	idParam := c.Params("id")
	if idParam == "" {
		return ErrorResponse(c, "User ID is required", http.StatusBadRequest)
	}

	id, err := uuid.Parse(idParam)
	if err != nil {
		return ErrorResponse(c, "Invalid user ID", http.StatusBadRequest)
	}

	if err := h.userRepo.DeleteUser(c.Context(), id); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrorResponse(c, "User not found", http.StatusNotFound)
		}
		if err.Error() == "cannot delete superadmin user" {
			return ErrorResponse(c, "Cannot delete a superadmin user", http.StatusForbidden)
		}
		log.Printf("[Users] DeleteUser error: %v\n", err)
		return ErrorResponse(c, "Failed to delete user", http.StatusInternalServerError)
	}

	return JSONResponse(c, fiber.Map{
		"status":  "success",
		"message": "User deleted successfully",
	}, http.StatusOK, 0)
}

func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	idParam := c.Params("id")
	if idParam == "" {
		return ErrorResponse(c, "User ID is required", http.StatusBadRequest)
	}

	id, err := uuid.Parse(idParam)
	if err != nil {
		return ErrorResponse(c, "Invalid user ID", http.StatusBadRequest)
	}

	var input model.UpdateUserInput
	if err := c.BodyParser(&input); err != nil {
		return ErrorResponse(c, "Invalid request body", http.StatusBadRequest)
	}

	updatedUser, err := h.userRepo.UpdateUser(c.Context(), id, input)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrorResponse(c, "User not found", http.StatusNotFound)
		}
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return ErrorResponse(c, "Username or email is already taken", http.StatusConflict)
		}
		if err.Error() == "cannot deactivate a superadmin account" {
			return ErrorResponse(c, "Cannot deactivate a superadmin account", http.StatusForbidden)
		}
		log.Printf("[Users] UpdateUser error: %v\n", err)
		return ErrorResponse(c, "Failed to update user", http.StatusInternalServerError)
	}

	accounts, _ := h.userRepo.GetOAuthAccounts(c.Context(), updatedUser.ID)
	dto := updatedUser.ToDTOWithAccounts(accounts)

	return JSONResponse(c, fiber.Map{
		"status": "success",
		"user":   dto,
	}, http.StatusOK, 0)
}


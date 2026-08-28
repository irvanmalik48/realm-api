package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
)

type CommentHandler struct {
	cfg *config.Config
	svc service.CommentService
}

func NewCommentHandler(cfg *config.Config, svc service.CommentService) *CommentHandler {
	return &CommentHandler{
		cfg: cfg,
		svc: svc,
	}
}

func (h *CommentHandler) GetComments(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return ErrorResponse(c, "Post slug is required", http.StatusBadRequest)
	}

	var userIDPtr *uuid.UUID
	if val := c.Locals("user_id"); val != nil {
		if uid, ok := val.(uuid.UUID); ok && uid != uuid.Nil {
			userIDPtr = &uid
		}
	}

	res, err := h.svc.GetComments(c.Context(), slug, userIDPtr)
	if err != nil {
		if errors.Is(err, service.ErrEmptySlug) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		log.Printf("[Comments] GetComments error for slug %q: %v\n", slug, err)
		return ErrorResponse(c, "Failed to retrieve comments", http.StatusInternalServerError)
	}

	return JSONResponse(c, res, http.StatusOK, 0)
}

func (h *CommentHandler) CreateComment(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return ErrorResponse(c, "Post slug is required", http.StatusBadRequest)
	}

	val := c.Locals("user_id")
	if val == nil {
		return ErrorResponse(c, "Unauthorized: login required to comment", http.StatusUnauthorized)
	}

	userID, ok := val.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return ErrorResponse(c, "Unauthorized: invalid user session", http.StatusUnauthorized)
	}

	var req model.CreateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorResponse(c, "Invalid request payload", http.StatusBadRequest)
	}

	res, err := h.svc.CreateComment(c.Context(), slug, userID, req.Content, req.ParentID)
	if err != nil {
		if errors.Is(err, service.ErrEmptyComment) || errors.Is(err, service.ErrCommentTooLong) || errors.Is(err, service.ErrEmptySlug) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		log.Printf("[Comments] CreateComment error for slug %q (user %s): %v\n", slug, userID, err)
		return ErrorResponse(c, "Failed to post comment", http.StatusInternalServerError)
	}

	return JSONResponse(c, res, http.StatusCreated, 0)
}

func (h *CommentHandler) UpdateComment(c *fiber.Ctx) error {
	commentIDStr := c.Params("id")
	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		return ErrorResponse(c, "Invalid comment identifier", http.StatusBadRequest)
	}

	val := c.Locals("user_id")
	if val == nil {
		return ErrorResponse(c, "Unauthorized: login required to edit comment", http.StatusUnauthorized)
	}

	userID, ok := val.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return ErrorResponse(c, "Unauthorized: invalid user session", http.StatusUnauthorized)
	}

	var req model.UpdateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorResponse(c, "Invalid request payload", http.StatusBadRequest)
	}

	res, err := h.svc.UpdateComment(c.Context(), commentID, userID, req.Content)
	if err != nil {
		if errors.Is(err, service.ErrEmptyComment) || errors.Is(err, service.ErrCommentTooLong) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		log.Printf("[Comments] UpdateComment error for comment %s (user %s): %v\n", commentID, userID, err)
		return ErrorResponse(c, "Failed to update comment", http.StatusInternalServerError)
	}

	return JSONResponse(c, res, http.StatusOK, 0)
}

func (h *CommentHandler) DeleteComment(c *fiber.Ctx) error {
	commentIDStr := c.Params("id")
	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		return ErrorResponse(c, "Invalid comment identifier", http.StatusBadRequest)
	}

	val := c.Locals("user_id")
	if val == nil {
		return ErrorResponse(c, "Unauthorized: login required to delete comment", http.StatusUnauthorized)
	}

	userID, ok := val.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return ErrorResponse(c, "Unauthorized: invalid user session", http.StatusUnauthorized)
	}

	if err := h.svc.DeleteComment(c.Context(), commentID, userID); err != nil {
		log.Printf("[Comments] DeleteComment error for comment %s (user %s): %v\n", commentID, userID, err)
		return ErrorResponse(c, "Failed to delete comment", http.StatusInternalServerError)
	}

	return JSONResponse(c, fiber.Map{"status": "success", "message": "Comment deleted"}, http.StatusOK, 0)
}

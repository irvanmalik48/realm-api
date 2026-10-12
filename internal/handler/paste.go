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
	"github.com/irvanmalik48/realm-api/internal/service"
)

type PasteHandler struct {
	cfg *config.Config
	svc service.PasteService
}

func NewPasteHandler(cfg *config.Config, svc service.PasteService) *PasteHandler {
	return &PasteHandler{
		cfg: cfg,
		svc: svc,
	}
}

func (h *PasteHandler) CreatePaste(c *fiber.Ctx) error {
	var req model.CreatePasteRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorResponse(c, "Invalid request payload", http.StatusBadRequest)
	}

	var userID *uuid.UUID
	if val := c.Locals("user_id"); val != nil {
		if id, ok := val.(uuid.UUID); ok && id != uuid.Nil {
			userID = &id
		}
	}

	paste, err := h.svc.CreatePaste(c.Context(), &req, userID)
	if err != nil {
		if errors.Is(err, service.ErrEmptyPasteContent) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		if errors.Is(err, service.ErrPasteTooLarge) {
			return ErrorResponse(c, err.Error(), http.StatusRequestEntityTooLarge)
		}
		log.Printf("[Pastebin] CreatePaste error: %v\n", err)
		return ErrorResponse(c, "Failed to create paste", http.StatusInternalServerError)
	}

	return JSONResponse(c, paste, http.StatusCreated, 0)
}

func (h *PasteHandler) GetPaste(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return ErrorResponse(c, "Paste slug is required", http.StatusBadRequest)
	}

	paste, err := h.svc.GetPasteBySlug(c.Context(), slug)
	if err != nil {
		if errors.Is(err, repository.ErrPasteNotFound) {
			return ErrorResponse(c, "Paste not found", http.StatusNotFound)
		}
		if errors.Is(err, repository.ErrPasteExpired) {
			return ErrorResponse(c, "Paste has expired", http.StatusGone)
		}
		log.Printf("[Pastebin] GetPaste error for slug %q: %v\n", slug, err)
		return ErrorResponse(c, "Failed to retrieve paste", http.StatusInternalServerError)
	}

	return JSONResponse(c, paste, http.StatusOK, 0)
}

func (h *PasteHandler) GetRawPaste(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(http.StatusBadRequest).SendString("Paste slug is required")
	}

	content, err := h.svc.GetRawContent(c.Context(), slug)
	if err != nil {
		if errors.Is(err, repository.ErrPasteNotFound) {
			return c.Status(http.StatusNotFound).SendString("Paste not found")
		}
		if errors.Is(err, repository.ErrPasteExpired) {
			return c.Status(http.StatusGone).SendString("Paste has expired")
		}
		log.Printf("[Pastebin] GetRawPaste error for slug %q: %v\n", slug, err)
		return c.Status(http.StatusInternalServerError).SendString("Failed to retrieve paste content")
	}

	c.Set("Content-Type", "text/plain; charset=utf-8")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.SendString(content)
}

func (h *PasteHandler) ListPastes(c *fiber.Ctx) error {
	search := c.Query("search")
	syntax := c.Query("syntax")
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	var isPublicOnly *bool
	if pub := c.Query("public"); pub != "" {
		val := pub == "true" || pub == "1"
		isPublicOnly = &val
	}

	var userID *uuid.UUID
	if userParam := c.Query("user_id"); userParam != "" {
		if uid, err := uuid.Parse(userParam); err == nil {
			userID = &uid
		}
	}

	res, err := h.svc.ListPastes(c.Context(), search, syntax, isPublicOnly, userID, limit, offset)
	if err != nil {
		log.Printf("[Pastebin] ListPastes error: %v\n", err)
		return ErrorResponse(c, "Failed to retrieve pastes", http.StatusInternalServerError)
	}

	return JSONResponse(c, res, http.StatusOK, 0)
}

func (h *PasteHandler) DeletePaste(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return ErrorResponse(c, "Paste slug is required", http.StatusBadRequest)
	}

	if err := h.svc.DeletePaste(c.Context(), slug); err != nil {
		if errors.Is(err, repository.ErrPasteNotFound) {
			return ErrorResponse(c, "Paste not found", http.StatusNotFound)
		}
		log.Printf("[Pastebin] DeletePaste error for slug %q: %v\n", slug, err)
		return ErrorResponse(c, "Failed to delete paste", http.StatusInternalServerError)
	}

	return JSONResponse(c, fiber.Map{"status": "success", "message": "Paste deleted"}, http.StatusOK, 0)
}

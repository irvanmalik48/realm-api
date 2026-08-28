package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
)

type ReactionHandler struct {
	cfg *config.Config
	svc service.ReactionService
}

func NewReactionHandler(cfg *config.Config, svc service.ReactionService) *ReactionHandler {
	return &ReactionHandler{
		cfg: cfg,
		svc: svc,
	}
}

func getClientIdentifier(c *fiber.Ctx) string {
	// If user is authenticated via token
	if val := c.Locals("user_id"); val != nil {
		if uid, ok := val.(uuid.UUID); ok && uid != uuid.Nil {
			return "user:" + uid.String()
		}
	}

	// If explicit client ID header provided by frontend
	if clientID := c.Get("X-Client-ID"); clientID != "" {
		h := sha256.Sum256([]byte(clientID))
		return "client:" + hex.EncodeToString(h[:16])
	}

	// Fallback to IP + User Agent hash
	ip := c.IP()
	ua := c.Get("User-Agent")
	h := sha256.Sum256([]byte(ip + ":" + ua))
	return "ip:" + hex.EncodeToString(h[:16])
}

func (h *ReactionHandler) GetReactions(c *fiber.Ctx) error {
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

	res, err := h.svc.GetReactions(c.Context(), slug, userIDPtr)
	if err != nil {
		if errors.Is(err, service.ErrEmptySlug) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		return ErrorResponse(c, "Failed to retrieve reactions", http.StatusInternalServerError)
	}

	return JSONResponse(c, res, http.StatusOK, 0)
}

func (h *ReactionHandler) ToggleReaction(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return ErrorResponse(c, "Post slug is required", http.StatusBadRequest)
	}

	val := c.Locals("user_id")
	if val == nil {
		return ErrorResponse(c, "Unauthorized: login required to react", http.StatusUnauthorized)
	}

	userID, ok := val.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return ErrorResponse(c, "Unauthorized: invalid user session", http.StatusUnauthorized)
	}

	var req model.ToggleReactionRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorResponse(c, "Invalid request payload", http.StatusBadRequest)
	}

	if req.Reaction == "" {
		return ErrorResponse(c, "Reaction type is required", http.StatusBadRequest)
	}

	res, err := h.svc.ToggleReaction(c.Context(), slug, req.Reaction, userID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidReaction) || errors.Is(err, service.ErrEmptySlug) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		return ErrorResponse(c, "Failed to update reaction", http.StatusInternalServerError)
	}

	return JSONResponse(c, res, http.StatusOK, 0)
}

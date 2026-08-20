package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/service"
)

type LastFMHandler struct {
	cfg     *config.Config
	service service.LastFMService
}

func NewLastFMHandler(cfg *config.Config, svc service.LastFMService) *LastFMHandler {
	return &LastFMHandler{
		cfg:     cfg,
		service: svc,
	}
}

func (h *LastFMHandler) GetRecentTracks(c *fiber.Ctx) error {
	if h.cfg.LastFMAPIKey == "" {
		log.Println("Missing LASTFM_API_KEY environment variable")
		return ErrorResponse(c, "Internal server error", http.StatusInternalServerError)
	}

	username := c.Query("username")
	if username == "" {
		return ErrorResponse(c, "Username is required.", http.StatusBadRequest)
	}

	limitStr := c.Query("limit")
	limit := 1
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 || parsedLimit > 200 {
			return ErrorResponse(c, "Limit must be between 1 and 200.", http.StatusBadRequest)
		}
		limit = parsedLimit
	}

	data, err := h.service.GetRecentTracks(c.Context(), username, limit)
	if err != nil {
		if errors.Is(err, service.ErrMissingAPIKey) {
			log.Println("Missing LASTFM_API_KEY environment variable")
			return ErrorResponse(c, "Internal server error", http.StatusInternalServerError)
		}

		var upstreamErr *service.UpstreamError
		if errors.As(err, &upstreamErr) {
			return ErrorResponse(c, "Failed to fetch LastFM data", upstreamErr.StatusCode)
		}

		log.Printf("Error fetching LastFM tracks: %v\n", err)
		return ErrorResponse(c, "Failed to fetch LastFM data", http.StatusBadGateway)
	}

	return JSONResponse(c, data, http.StatusOK, h.cfg.CacheRevalidateSeconds)
}

func (h *LastFMHandler) GetUserInfo(c *fiber.Ctx) error {
	if h.cfg.LastFMAPIKey == "" {
		log.Println("Missing LASTFM_API_KEY environment variable")
		return ErrorResponse(c, "Internal server error", http.StatusInternalServerError)
	}

	username := c.Query("username")
	if username == "" {
		return ErrorResponse(c, "Username is required.", http.StatusBadRequest)
	}

	data, err := h.service.GetUserInfo(c.Context(), username)
	if err != nil {
		if errors.Is(err, service.ErrMissingAPIKey) {
			log.Println("Missing LASTFM_API_KEY environment variable")
			return ErrorResponse(c, "Internal server error", http.StatusInternalServerError)
		}

		var upstreamErr *service.UpstreamError
		if errors.As(err, &upstreamErr) {
			return ErrorResponse(c, "Failed to fetch LastFM user data", upstreamErr.StatusCode)
		}

		log.Printf("Error fetching LastFM user: %v\n", err)
		return ErrorResponse(c, "Failed to fetch LastFM user data", http.StatusBadGateway)
	}

	return JSONResponse(c, data, http.StatusOK, h.cfg.CacheRevalidateSeconds)
}

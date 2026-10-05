package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
)

// AnalyticsHandler handles HTTP endpoints for pageview analytics ingestion and statistics.
type AnalyticsHandler struct {
	cfg *config.Config
	svc *service.AnalyticsService
}

// NewAnalyticsHandler constructs an AnalyticsHandler.
func NewAnalyticsHandler(cfg *config.Config, svc *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{cfg: cfg, svc: svc}
}

// TrackEvent handles POST /v1/analytics/events.
func (h *AnalyticsHandler) TrackEvent(c *fiber.Ctx) error {
	var req model.TrackEventRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorResponse(c, "Invalid request body", http.StatusBadRequest)
	}

	// Capture user agent from headers if not specified in body
	if req.UserAgent == "" {
		req.UserAgent = string(c.Request().Header.UserAgent())
	}
	if req.Referrer == "" {
		req.Referrer = string(c.Request().Header.Referer())
	}

	clientIP := c.IP()
	if err := h.svc.TrackEvent(c.Context(), &req, clientIP); err != nil {
		return ErrorResponse(c, "Failed to record event: "+err.Error(), http.StatusInternalServerError)
	}

	return JSONResponse(c, fiber.Map{"status": "recorded"}, http.StatusCreated, 0)
}

// GetStats handles GET /v1/analytics/stats.
func (h *AnalyticsHandler) GetStats(c *fiber.Ctx) error {
	period := c.Query("period", "30d")
	stats, err := h.svc.GetStats(c.Context(), period)
	if err != nil {
		return ErrorResponse(c, "Failed to retrieve analytics stats: "+err.Error(), http.StatusInternalServerError)
	}

	return JSONResponse(c, stats, http.StatusOK, 0)
}

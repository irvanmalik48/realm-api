package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/database"
)

var startTime = time.Now()

type HealthResponse struct {
	Status        string `json:"status"`
	Service       string `json:"service"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	Timestamp     string `json:"timestamp"`
	Database      string `json:"database"`
}

type HealthHandler struct {
	db *database.DB
}

func NewHealthHandler(db *database.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Handle(c *fiber.Ctx) error {
	dbStatus := "connected"
	overallStatus := "healthy"
	httpStatus := http.StatusOK

	if h.db != nil && h.db.Pool != nil {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := h.db.Pool.Ping(ctx); err != nil {
			dbStatus = "disconnected"
			overallStatus = "degraded"
		}
	} else {
		dbStatus = "not_configured"
	}

	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")

	return c.Status(httpStatus).JSON(HealthResponse{
		Status:        overallStatus,
		Service:       "realm-api",
		Version:       "1.0.0",
		UptimeSeconds: int64(time.Since(startTime).Seconds()),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Database:      dbStatus,
	})
}

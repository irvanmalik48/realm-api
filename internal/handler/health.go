package handler

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/telemetry"
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

type DBPoolStats struct {
	AcquiredConns int32 `json:"acquired_conns"`
	IdleConns     int32 `json:"idle_conns"`
	TotalConns    int32 `json:"total_conns"`
	MaxConns      int32 `json:"max_conns"`
}

type RuntimeStats struct {
	Goroutines      int32  `json:"goroutines"`
	AllocBytes      uint64 `json:"alloc_bytes"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	SysBytes        uint64 `json:"sys_bytes"`
	GCCycles        uint32 `json:"gc_cycles"`
}

type DetailedTelemetryResponse struct {
	Status        string             `json:"status"`
	Service       string             `json:"service"`
	Version       string             `json:"version"`
	UptimeSeconds int64              `json:"uptime_seconds"`
	Timestamp     string             `json:"timestamp"`
	Database      string             `json:"database"`
	DBPool        DBPoolStats        `json:"db_pool"`
	Runtime       RuntimeStats       `json:"runtime"`
	CPU           telemetry.CPUStats `json:"cpu"`
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

func (h *HealthHandler) Telemetry(c *fiber.Ctx) error {
	dbStatus := "connected"
	overallStatus := "healthy"

	var poolStats DBPoolStats
	if h.db != nil && h.db.Pool != nil {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := h.db.Pool.Ping(ctx); err != nil {
			dbStatus = "disconnected"
			overallStatus = "degraded"
		}
		stat := h.db.Pool.Stat()
		poolStats = DBPoolStats{
			AcquiredConns: stat.AcquiredConns(),
			IdleConns:     stat.IdleConns(),
			TotalConns:    stat.TotalConns(),
			MaxConns:      stat.MaxConns(),
		}
	} else {
		dbStatus = "not_configured"
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")

	return c.Status(http.StatusOK).JSON(DetailedTelemetryResponse{
		Status:        overallStatus,
		Service:       "realm-api",
		Version:       "1.0.0",
		UptimeSeconds: int64(time.Since(startTime).Seconds()),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Database:      dbStatus,
		DBPool:        poolStats,
		Runtime: RuntimeStats{
			Goroutines:      int32(runtime.NumGoroutine()),
			AllocBytes:      mem.Alloc,
			TotalAllocBytes: mem.TotalAlloc,
			SysBytes:        mem.Sys,
			GCCycles:        mem.NumGC,
		},
		CPU: telemetry.DefaultCPUMonitor().GetStats(),
	})
}

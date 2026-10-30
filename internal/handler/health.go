package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"runtime/metrics"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/telemetry"
	"github.com/valyala/fasthttp"
)

var startTime = time.Now()

type HealthResponse struct {
	Status        string `json:"status"`
	Service       string `json:"service"`
	Version       string `json:"version"`
	Timestamp     string `json:"timestamp"`
	Database      string `json:"database"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

type DBPoolStats struct {
	AcquiredConns int32 `json:"acquired_conns"`
	IdleConns     int32 `json:"idle_conns"`
	TotalConns    int32 `json:"total_conns"`
	MaxConns      int32 `json:"max_conns"`
}

type RuntimeStats struct {
	AllocBytes      uint64 `json:"alloc_bytes"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	SysBytes        uint64 `json:"sys_bytes"`
	Goroutines      int32  `json:"goroutines"`
	GCCycles        uint32 `json:"gc_cycles"`
}

type DetailedTelemetryResponse struct {
	Status        string             `json:"status"`
	Service       string             `json:"service"`
	Version       string             `json:"version"`
	Timestamp     string             `json:"timestamp"`
	Database      string             `json:"database"`
	CPU           telemetry.CPUStats `json:"cpu"`
	Runtime       RuntimeStats       `json:"runtime"`
	UptimeSeconds int64              `json:"uptime_seconds"`
	DBPool        DBPoolStats        `json:"db_pool"`
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

func (h *HealthHandler) collectTelemetry(ctx context.Context) DetailedTelemetryResponse {
	dbStatus := "connected"
	overallStatus := "healthy"

	var poolStats DBPoolStats
	if h.db != nil && h.db.Pool != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := h.db.Pool.Ping(pingCtx); err != nil {
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

	samples := []metrics.Sample{
		{Name: "/memory/classes/heap/objects:bytes"},
		{Name: "/memory/classes/total:bytes"},
		{Name: "/gc/cycles/total:gc-cycles"},
	}
	metrics.Read(samples)

	return DetailedTelemetryResponse{
		Status:        overallStatus,
		Service:       "realm-api",
		Version:       "1.0.0",
		UptimeSeconds: int64(time.Since(startTime).Seconds()),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Database:      dbStatus,
		DBPool:        poolStats,
		Runtime: RuntimeStats{
			Goroutines:      int32(runtime.NumGoroutine()),
			AllocBytes:      samples[0].Value.Uint64(),
			TotalAllocBytes: samples[1].Value.Uint64(),
			SysBytes:        samples[1].Value.Uint64(),
			GCCycles:        uint32(samples[2].Value.Uint64()),
		},
		CPU: telemetry.DefaultCPUMonitor().GetStats(),
	}
}

func (h *HealthHandler) Telemetry(c *fiber.Ctx) error {
	data := h.collectTelemetry(c.Context())
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	return c.Status(http.StatusOK).JSON(data)
}

func (h *HealthHandler) Stream(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache, no-transform")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Send initial telemetry payload immediately
		initial := h.collectTelemetry(context.Background())
		if data, err := json.Marshal(initial); err == nil {
			if _, err := fmt.Fprintf(w, "event: telemetry\ndata: %s\n\n", data); err != nil {
				return
			}
			if err := w.Flush(); err != nil {
				return
			}
		}

		for range ticker.C {
			current := h.collectTelemetry(context.Background())
			data, err := json.Marshal(current)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "event: telemetry\ndata: %s\n\n", data); err != nil {
				return
			}
			if err := w.Flush(); err != nil {
				return
			}
		}
	}))

	return nil
}

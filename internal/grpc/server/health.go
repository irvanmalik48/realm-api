package server

import (
	"context"
	"runtime"
	"time"

	"github.com/irvanmalik48/realm-api/internal/database"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
)

var startTime = time.Now()

type HealthServer struct {
	realmv1.UnimplementedHealthServiceServer
	db *database.DB
}

func NewHealthServer(db *database.DB) *HealthServer {
	return &HealthServer{db: db}
}

func (s *HealthServer) GetHealth(ctx context.Context, req *realmv1.HealthRequest) (*realmv1.HealthResponse, error) {
	dbStatus := "connected"
	overallStatus := "healthy"

	if s.db != nil && s.db.Pool != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := s.db.Pool.Ping(pingCtx); err != nil {
			dbStatus = "disconnected"
			overallStatus = "degraded"
		}
	} else {
		dbStatus = "not_configured"
	}

	uptime := int64(time.Since(startTime).Seconds())

	return &realmv1.HealthResponse{
		Status:        overallStatus,
		Service:       "realm-api",
		Version:       "1.0.0",
		UptimeSeconds: uptime,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Database:      dbStatus,
	}, nil
}

func (s *HealthServer) GetDetailedTelemetry(ctx context.Context, req *realmv1.HealthRequest) (*realmv1.DetailedTelemetryResponse, error) {
	healthResp, err := s.GetHealth(ctx, req)
	if err != nil {
		return nil, err
	}

	var poolStats *realmv1.DBPoolStats
	if s.db != nil && s.db.Pool != nil {
		stat := s.db.Pool.Stat()
		poolStats = &realmv1.DBPoolStats{
			AcquiredConns: stat.AcquiredConns(),
			IdleConns:     stat.IdleConns(),
			TotalConns:    stat.TotalConns(),
			MaxConns:      stat.MaxConns(),
		}
	} else {
		poolStats = &realmv1.DBPoolStats{}
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	runtimeStats := &realmv1.RuntimeStats{
		Goroutines:      int32(runtime.NumGoroutine()),
		AllocBytes:      mem.Alloc,
		TotalAllocBytes: mem.TotalAlloc,
		SysBytes:        mem.Sys,
		GcCycles:        mem.NumGC,
	}

	return &realmv1.DetailedTelemetryResponse{
		Status:        healthResp.Status,
		Service:       healthResp.Service,
		Version:       healthResp.Version,
		UptimeSeconds: healthResp.UptimeSeconds,
		Timestamp:     healthResp.Timestamp,
		Database:      healthResp.Database,
		DbPool:        poolStats,
		Runtime:       runtimeStats,
	}, nil
}

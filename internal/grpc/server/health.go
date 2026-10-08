package server

import (
	"context"
	"runtime"
	"time"

	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/telemetry"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc"
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

	cpuSnapshot := telemetry.DefaultCPUMonitor().GetStats()
	cpuStats := &realmv1.CPUStats{
		UsagePercent:     cpuSnapshot.UsagePercent,
		CoreUsagePercent: cpuSnapshot.CoreUsagePercent,
		AvgFrequencyMhz:  cpuSnapshot.AvgFrequencyMHz,
		CoreFrequencyMhz: cpuSnapshot.CoreFrequencyMHz,
		MinFrequencyMhz:  cpuSnapshot.MinFrequencyMHz,
		MaxFrequencyMhz:  cpuSnapshot.MaxFrequencyMHz,
		Load_1M:          cpuSnapshot.Load1m,
		Load_5M:          cpuSnapshot.Load5m,
		Load_15M:         cpuSnapshot.Load15m,
		ModelName:        cpuSnapshot.ModelName,
		CoreCount:        cpuSnapshot.CoreCount,
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
		Cpu:           cpuStats,
	}, nil
}

func (s *HealthServer) StreamTelemetry(req *realmv1.HealthRequest, stream grpc.ServerStreamingServer[realmv1.DetailedTelemetryResponse]) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Send initial response immediately
	initialResp, err := s.GetDetailedTelemetry(stream.Context(), req)
	if err != nil {
		return err
	}
	if err := stream.Send(initialResp); err != nil {
		return err
	}

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			resp, err := s.GetDetailedTelemetry(stream.Context(), req)
			if err != nil {
				return err
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
		}
	}
}

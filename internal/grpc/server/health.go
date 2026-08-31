package server

import (
	"context"
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
	dbStatus := "disconnected"
	if s.db != nil {
		if err := s.db.Ping(ctx); err == nil {
			dbStatus = "connected"
		}
	}

	uptime := int64(time.Since(startTime).Seconds())

	return &realmv1.HealthResponse{
		Status:        "healthy",
		Service:       "realm-api",
		Version:       "1.0.0",
		UptimeSeconds: uptime,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Database:      dbStatus,
	}, nil
}

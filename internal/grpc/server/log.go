package server

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LogServer struct {
	realmv1.UnimplementedLogServiceServer
	logRepo   repository.LogRepository
	adminRepo repository.AdminRepository
}

func NewLogServer(logRepo repository.LogRepository, adminRepo repository.AdminRepository) *LogServer {
	return &LogServer{
		logRepo:   logRepo,
		adminRepo: adminRepo,
	}
}

func mapLogEntryToProto(l *model.SystemLog) *realmv1.LogEntry {
	if l == nil {
		return nil
	}

	return &realmv1.LogEntry{
		Id:             l.ID.String(),
		Timestamp:      l.Timestamp.Format(time.RFC3339Nano),
		Level:          l.Level,
		Message:        l.Message,
		Component:      l.Component,
		TraceId:        l.TraceID,
		SpanId:         l.SpanID,
		AttributesJson: l.AttributesJSON,
	}
}

func (s *LogServer) GetLogs(ctx context.Context, req *realmv1.GetLogsRequest) (*realmv1.GetLogsResponse, error) {
	if err := Authorize(ctx, s.adminRepo, "analytics:read", "system:telemetry", "logs:delete", "admins:read", "*"); err != nil {
		return nil, err
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.GetOffset())
	if offset < 0 {
		offset = 0
	}

	level := req.GetLevel()
	search := req.GetSearch()
	traceID := req.GetTraceId()

	logs, total, err := s.logRepo.GetLogs(ctx, limit, offset, level, search, traceID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get logs: %v", err)
	}

	protoLogs := make([]*realmv1.LogEntry, len(logs))
	for i := range logs {
		protoLogs[i] = mapLogEntryToProto(&logs[i])
	}

	return &realmv1.GetLogsResponse{
		Total: int32(total),
		Logs:  protoLogs,
	}, nil
}

func (s *LogServer) DeleteLogs(ctx context.Context, req *realmv1.DeleteLogsRequest) (*realmv1.DeleteLogsResponse, error) {
	if err := Authorize(ctx, s.adminRepo, "logs:delete"); err != nil {
		return nil, err
	}

	var beforeTime *time.Time
	if req.BeforeTimestamp != nil && *req.BeforeTimestamp != "" {
		t, err := time.Parse(time.RFC3339, *req.BeforeTimestamp)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid before_timestamp format: %v", err)
		}
		beforeTime = &t
	}

	var logID *uuid.UUID
	if req.LogId != nil && *req.LogId != "" {
		id, err := uuid.Parse(*req.LogId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid log_id UUID: %v", err)
		}
		logID = &id
	}

	deleted, err := s.logRepo.DeleteLogs(ctx, beforeTime, logID, req.GetClearAll())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to delete logs: %v", err)
	}

	return &realmv1.DeleteLogsResponse{
		DeletedCount: deleted,
		Status:       "success",
		Message:      "Logs deleted successfully",
	}, nil
}

func (s *LogServer) StreamLogs(req *realmv1.StreamLogsRequest, stream realmv1.LogService_StreamLogsServer) error {
	ctx := stream.Context()
	if err := Authorize(ctx, s.adminRepo, "analytics:read", "system:telemetry", "logs:delete", "admins:read", "*"); err != nil {
		return err
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Initial fetch of recent 20 logs
	initialLogs, _, err := s.logRepo.GetLogs(ctx, 20, 0, req.GetLevel(), req.GetSearch(), "")
	if err == nil {
		// Send initial in reverse (oldest first)
		for i := len(initialLogs) - 1; i >= 0; i-- {
			if err := stream.Send(mapLogEntryToProto(&initialLogs[i])); err != nil {
				return err
			}
		}
	}

	lastSeen := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			logs, _, err := s.logRepo.GetLogs(ctx, 50, 0, req.GetLevel(), req.GetSearch(), "")
			if err != nil {
				continue
			}

			// Filter only logs newer than lastSeen
			var newLogs []model.SystemLog
			for _, l := range logs {
				if l.Timestamp.After(lastSeen) {
					newLogs = append(newLogs, l)
				}
			}

			if len(newLogs) > 0 {
				for i := len(newLogs) - 1; i >= 0; i-- {
					if err := stream.Send(mapLogEntryToProto(&newLogs[i])); err != nil {
						return err
					}
					if newLogs[i].Timestamp.After(lastSeen) {
						lastSeen = newLogs[i].Timestamp
					}
				}
			}
		}
	}
}

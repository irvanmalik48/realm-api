package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

type LogRepository interface {
	InsertLog(ctx context.Context, log *model.SystemLog) error
	GetLogs(ctx context.Context, limit, offset int, level, search, traceID string) ([]model.SystemLog, int, error)
	DeleteLogs(ctx context.Context, beforeTimestamp *time.Time, logID *uuid.UUID, clearAll bool) (int64, error)
}

type logRepository struct {
	db *database.DB
}

func NewLogRepository(db *database.DB) LogRepository {
	return &logRepository{db: db}
}

func (r *logRepository) InsertLog(ctx context.Context, log *model.SystemLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	if log.AttributesJSON == "" {
		log.AttributesJSON = "{}"
	}

	query := `
		INSERT INTO system_logs (id, timestamp, level, message, component, trace_id, span_id, attributes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		log.ID, log.Timestamp, log.Level, log.Message, log.Component, log.TraceID, log.SpanID, log.AttributesJSON,
	)
	return err
}

func (r *logRepository) GetLogs(ctx context.Context, limit, offset int, level, search, traceID string) ([]model.SystemLog, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if level != "" {
		whereClause += fmt.Sprintf(" AND level = $%d", argIdx)
		args = append(args, level)
		argIdx++
	}

	if search != "" {
		whereClause += fmt.Sprintf(" AND (message ILIKE $%d OR component ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	if traceID != "" {
		whereClause += fmt.Sprintf(" AND trace_id = $%d", argIdx)
		args = append(args, traceID)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM system_logs %s", whereClause)
	var total int
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, timestamp, level, message, COALESCE(component, ''), COALESCE(trace_id, ''), COALESCE(span_id, ''), attributes::text
		FROM system_logs
		%s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []model.SystemLog
	for rows.Next() {
		var l model.SystemLog
		if err := rows.Scan(
			&l.ID, &l.Timestamp, &l.Level, &l.Message, &l.Component, &l.TraceID, &l.SpanID, &l.AttributesJSON,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan log: %w", err)
		}
		logs = append(logs, l)
	}

	return logs, total, nil
}

func (r *logRepository) DeleteLogs(ctx context.Context, beforeTimestamp *time.Time, logID *uuid.UUID, clearAll bool) (int64, error) {
	if clearAll {
		cmdTag, err := r.db.Pool.Exec(ctx, "TRUNCATE TABLE system_logs")
		if err != nil {
			return 0, fmt.Errorf("failed to truncate system_logs: %w", err)
		}
		return cmdTag.RowsAffected(), nil
	}

	if logID != nil && *logID != uuid.Nil {
		cmdTag, err := r.db.Pool.Exec(ctx, "DELETE FROM system_logs WHERE id = $1", *logID)
		if err != nil {
			return 0, fmt.Errorf("failed to delete log: %w", err)
		}
		return cmdTag.RowsAffected(), nil
	}

	if beforeTimestamp != nil && !beforeTimestamp.IsZero() {
		cmdTag, err := r.db.Pool.Exec(ctx, "DELETE FROM system_logs WHERE timestamp < $1", *beforeTimestamp)
		if err != nil {
			return 0, fmt.Errorf("failed to delete logs before timestamp: %w", err)
		}
		return cmdTag.RowsAffected(), nil
	}

	return 0, nil
}

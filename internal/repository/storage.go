package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

var (
	ErrRecordNotFound = errors.New("file record not found")
)

type StorageRepository interface {
	Create(ctx context.Context, record *model.FileRecord) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.FileRecord, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int, search, backend string) ([]model.FileRecord, int, error)
	GetStats(ctx context.Context) (totalFiles int64, totalOrigBytes int64, totalCompBytes int64, avgSavings float64, err error)
}

type storageRepository struct {
	db *database.DB
}

func NewStorageRepository(db *database.DB) StorageRepository {
	return &storageRepository{db: db}
}

func (r *storageRepository) Create(ctx context.Context, record *model.FileRecord) error {
	if record.StorageBackend == "" {
		record.StorageBackend = "local"
	}

	query := `
	INSERT INTO files (
		id, filename, content_type, original_size, compressed_size,
		compression_algorithm, sha256, blurhash, width, height, is_public,
		storage_backend, s3_bucket, s3_key, s3_etag, created_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
	);
	`
	_, err := r.db.Pool.Exec(ctx, query,
		record.ID,
		record.Filename,
		record.ContentType,
		record.OriginalSize,
		record.CompressedSize,
		record.CompressionAlgorithm,
		record.SHA256,
		record.Blurhash,
		record.Width,
		record.Height,
		record.IsPublic,
		record.StorageBackend,
		record.S3Bucket,
		record.S3Key,
		record.S3ETag,
		record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert file record: %w", err)
	}
	return nil
}

func (r *storageRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.FileRecord, error) {
	query := `
	SELECT id, filename, content_type, original_size, compressed_size,
	       compression_algorithm, sha256, blurhash, width, height, is_public,
	       COALESCE(storage_backend, 'local'), s3_bucket, s3_key, s3_etag, created_at
	FROM files
	WHERE id = $1;
	`
	var record model.FileRecord
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&record.ID,
		&record.Filename,
		&record.ContentType,
		&record.OriginalSize,
		&record.CompressedSize,
		&record.CompressionAlgorithm,
		&record.SHA256,
		&record.Blurhash,
		&record.Width,
		&record.Height,
		&record.IsPublic,
		&record.StorageBackend,
		&record.S3Bucket,
		&record.S3Key,
		&record.S3ETag,
		&record.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, fmt.Errorf("failed to query file record: %w", err)
	}
	return &record, nil
}

func (r *storageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM files WHERE id = $1;`
	cmdTag, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete file record: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrRecordNotFound
	}
	return nil
}

func (r *storageRepository) List(ctx context.Context, limit, offset int, search, backend string) ([]model.FileRecord, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	whereClauses := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		whereClauses += fmt.Sprintf(" AND filename ILIKE $%d", argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	if backend != "" && backend != "all" {
		whereClauses += fmt.Sprintf(" AND COALESCE(storage_backend, 'local') = $%d", argIdx)
		args = append(args, backend)
		argIdx++
	}

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM files %s", whereClauses)
	var total int
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count files: %w", err)
	}

	// Data query
	dataQuery := fmt.Sprintf(`
		SELECT id, filename, content_type, original_size, compressed_size,
		       compression_algorithm, sha256, blurhash, width, height, is_public,
		       COALESCE(storage_backend, 'local'), s3_bucket, s3_key, s3_etag, created_at
		FROM files
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClauses, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list files: %w", err)
	}
	defer rows.Close()

	records := make([]model.FileRecord, 0)
	for rows.Next() {
		var record model.FileRecord
		if err := rows.Scan(
			&record.ID,
			&record.Filename,
			&record.ContentType,
			&record.OriginalSize,
			&record.CompressedSize,
			&record.CompressionAlgorithm,
			&record.SHA256,
			&record.Blurhash,
			&record.Width,
			&record.Height,
			&record.IsPublic,
			&record.StorageBackend,
			&record.S3Bucket,
			&record.S3Key,
			&record.S3ETag,
			&record.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan file record: %w", err)
		}
		records = append(records, record)
	}

	return records, total, nil
}

func (r *storageRepository) GetStats(ctx context.Context) (int64, int64, int64, float64, error) {
	query := `
		SELECT 
			COUNT(*),
			COALESCE(SUM(original_size), 0),
			COALESCE(SUM(compressed_size), 0)
		FROM files;
	`
	var totalFiles, totalOrigBytes, totalCompBytes int64
	err := r.db.Pool.QueryRow(ctx, query).Scan(&totalFiles, &totalOrigBytes, &totalCompBytes)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to query storage stats: %w", err)
	}

	var avgSavings float64
	if totalOrigBytes > 0 {
		avgSavings = float64(totalOrigBytes-totalCompBytes) / float64(totalOrigBytes) * 100.0
	}

	return totalFiles, totalOrigBytes, totalCompBytes, avgSavings, nil
}

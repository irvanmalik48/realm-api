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
}

type storageRepository struct {
	db *database.DB
}

func NewStorageRepository(db *database.DB) StorageRepository {
	return &storageRepository{db: db}
}

func (r *storageRepository) Create(ctx context.Context, record *model.FileRecord) error {
	query := `
	INSERT INTO files (
		id, filename, content_type, original_size, compressed_size,
		compression_algorithm, sha256, blurhash, width, height, is_public, created_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
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
	       compression_algorithm, sha256, blurhash, width, height, is_public, created_at
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

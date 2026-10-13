package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrPasteNotFound = errors.New("paste not found")
	ErrPasteExpired  = errors.New("paste has expired")
)

const (
	slugLetters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	slugLength  = 7
)

// Generate7LetterSlug generates a cryptographically secure random 7-letter slug
func Generate7LetterSlug() (string, error) {
	bytes := make([]byte, slugLength)
	charsetLen := big.NewInt(int64(len(slugLetters)))
	for i := 0; i < slugLength; i++ {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random slug: %w", err)
		}
		bytes[i] = slugLetters[num.Int64()]
	}
	return string(bytes), nil
}

type PasteRepository interface {
	CreatePaste(ctx context.Context, paste *model.Paste) (*model.Paste, error)
	GetPasteBySlug(ctx context.Context, slug string) (*model.Paste, error)
	ListPastes(ctx context.Context, search, syntax string, isPublicOnly *bool, userID *uuid.UUID, limit, offset int) (*model.PastesListResponse, error)
	DeletePaste(ctx context.Context, slug string) error
	IncrementViews(ctx context.Context, slug string) error
}

type pasteRepository struct {
	db *database.DB
}

func NewPasteRepository(db *database.DB) PasteRepository {
	return &pasteRepository{db: db}
}

func (r *pasteRepository) CreatePaste(ctx context.Context, paste *model.Paste) (*model.Paste, error) {
	const maxRetries = 5
	query := `
		INSERT INTO pastes (slug, title, content, syntax, user_id, is_public, is_encrypted, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, slug, title, content, syntax, user_id, is_public, is_encrypted, views, expires_at, created_at, updated_at
	`

	for attempt := 0; attempt < maxRetries; attempt++ {
		if paste.Slug == "" || attempt > 0 {
			slug, err := Generate7LetterSlug()
			if err != nil {
				return nil, err
			}
			paste.Slug = slug
		}

		var p model.Paste
		err := r.db.Pool.QueryRow(ctx, query,
			paste.Slug, paste.Title, paste.Content, paste.Syntax, paste.UserID, paste.IsPublic, paste.IsEncrypted, paste.ExpiresAt,
		).Scan(
			&p.ID, &p.Slug, &p.Title, &p.Content, &p.Syntax, &p.UserID, &p.IsPublic, &p.IsEncrypted, &p.Views, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
		)

		if err == nil {
			return &p, nil
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Slug collision, retry with a fresh 7-letter slug
			continue
		}

		return nil, fmt.Errorf("failed to create paste: %w", err)
	}

	return nil, errors.New("failed to generate unique 7-letter slug after multiple attempts")
}

func (r *pasteRepository) GetPasteBySlug(ctx context.Context, slug string) (*model.Paste, error) {
	query := `
		SELECT 
			id, slug, title, content, syntax, user_id, is_public, is_encrypted, views, expires_at, created_at, updated_at
		FROM pastes
		WHERE slug = $1
	`

	var p model.Paste
	err := r.db.Pool.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.Slug, &p.Title, &p.Content, &p.Syntax, &p.UserID, &p.IsPublic, &p.IsEncrypted, &p.Views, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPasteNotFound
		}
		return nil, fmt.Errorf("failed to get paste: %w", err)
	}

	// Check expiration
	if p.ExpiresAt != nil && p.ExpiresAt.Before(time.Now()) {
		return nil, ErrPasteExpired
	}

	return &p, nil
}

func (r *pasteRepository) IncrementViews(ctx context.Context, slug string) error {
	query := `UPDATE pastes SET views = views + 1 WHERE slug = $1`
	_, err := r.db.Pool.Exec(ctx, query, slug)
	return err
}

func (r *pasteRepository) ListPastes(ctx context.Context, search, syntax string, isPublicOnly *bool, userID *uuid.UUID, limit, offset int) (*model.PastesListResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	conditions := []string{
		"(expires_at IS NULL OR expires_at > NOW())",
	}
	var args []interface{}
	argIdx := 1

	if isPublicOnly != nil && *isPublicOnly {
		conditions = append(conditions, "is_public = true")
	}

	if syntax != "" {
		conditions = append(conditions, fmt.Sprintf("syntax = $%d", argIdx))
		args = append(args, strings.ToLower(strings.TrimSpace(syntax)))
		argIdx++
	}

	if userID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, *userID)
		argIdx++
	}

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR slug ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+search+"%")
		argIdx++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM pastes %s", whereClause)
	var total int
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count pastes: %w", err)
	}

	dataQuery := fmt.Sprintf(`
		SELECT 
			id, slug, title, syntax, user_id, is_public, is_encrypted, views, OCTET_LENGTH(content) AS size, expires_at, created_at, updated_at
		FROM pastes
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list pastes: %w", err)
	}
	defer rows.Close()

	pastes := make([]model.PasteSummaryDTO, 0)
	for rows.Next() {
		var p model.PasteSummaryDTO
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.Title, &p.Syntax, &p.UserID, &p.IsPublic, &p.IsEncrypted, &p.Views, &p.Size, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan paste summary: %w", err)
		}
		pastes = append(pastes, p)
	}

	return &model.PastesListResponse{
		Total:  total,
		Pastes: pastes,
	}, nil
}

func (r *pasteRepository) DeletePaste(ctx context.Context, slug string) error {
	query := `DELETE FROM pastes WHERE slug = $1`
	res, err := r.db.Pool.Exec(ctx, query, slug)
	if err != nil {
		return fmt.Errorf("failed to delete paste: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrPasteNotFound
	}
	return nil
}

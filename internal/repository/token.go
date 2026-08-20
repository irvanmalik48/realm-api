package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

var (
	ErrTokenNotFound = errors.New("api token not found")
)

type TokenRepository interface {
	Create(ctx context.Context, token *model.APIToken) error
	GetByHash(ctx context.Context, hash string) (*model.APIToken, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.APIToken, error)
	List(ctx context.Context) ([]model.APIToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}

type tokenRepository struct {
	db *database.DB
}

func NewTokenRepository(db *database.DB) TokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) Create(ctx context.Context, token *model.APIToken) error {
	query := `
	INSERT INTO api_tokens (
		id, name, token_prefix, token_hash, scopes, rate_limit_rpm,
		last_used_at, expires_at, is_revoked, created_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
	);
	`
	_, err := r.db.Pool.Exec(ctx, query,
		token.ID,
		token.Name,
		token.TokenPrefix,
		token.TokenHash,
		token.Scopes,
		token.RateLimitRPM,
		token.LastUsedAt,
		token.ExpiresAt,
		token.IsRevoked,
		token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert api token: %w", err)
	}
	return nil
}

func (r *tokenRepository) GetByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	query := `
	SELECT id, name, token_prefix, token_hash, scopes, rate_limit_rpm,
	       last_used_at, expires_at, is_revoked, created_at
	FROM api_tokens
	WHERE token_hash = $1;
	`
	var t model.APIToken
	err := r.db.Pool.QueryRow(ctx, query, hash).Scan(
		&t.ID,
		&t.Name,
		&t.TokenPrefix,
		&t.TokenHash,
		&t.Scopes,
		&t.RateLimitRPM,
		&t.LastUsedAt,
		&t.ExpiresAt,
		&t.IsRevoked,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("failed to query api token: %w", err)
	}
	return &t, nil
}

func (r *tokenRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.APIToken, error) {
	query := `
	SELECT id, name, token_prefix, token_hash, scopes, rate_limit_rpm,
	       last_used_at, expires_at, is_revoked, created_at
	FROM api_tokens
	WHERE id = $1;
	`
	var t model.APIToken
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&t.ID,
		&t.Name,
		&t.TokenPrefix,
		&t.TokenHash,
		&t.Scopes,
		&t.RateLimitRPM,
		&t.LastUsedAt,
		&t.ExpiresAt,
		&t.IsRevoked,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("failed to query api token by id: %w", err)
	}
	return &t, nil
}

func (r *tokenRepository) List(ctx context.Context) ([]model.APIToken, error) {
	query := `
	SELECT id, name, token_prefix, token_hash, scopes, rate_limit_rpm,
	       last_used_at, expires_at, is_revoked, created_at
	FROM api_tokens
	ORDER BY created_at DESC;
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []model.APIToken
	for rows.Next() {
		var t model.APIToken
		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.TokenPrefix,
			&t.TokenHash,
			&t.Scopes,
			&t.RateLimitRPM,
			&t.LastUsedAt,
			&t.ExpiresAt,
			&t.IsRevoked,
			&t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan api token row: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, nil
}

func (r *tokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE api_tokens SET is_revoked = true WHERE id = $1;`
	cmdTag, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to revoke api token: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrTokenNotFound
	}
	return nil
}

func (r *tokenRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE api_tokens SET last_used_at = $2 WHERE id = $1;`
	_, err := r.db.Pool.Exec(ctx, query, id, time.Now().UTC())
	return err
}

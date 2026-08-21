package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user with this email or username already exists")
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByIdentifier(ctx context.Context, identifier string) (*model.User, error)
	GetByProvider(ctx context.Context, provider, providerID string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
}

type userRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (id, email, username, full_name, password_hash, avatar_url, provider, provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		user.ID,
		strings.ToLower(strings.TrimSpace(user.Email)),
		strings.ToLower(strings.TrimSpace(user.Username)),
		user.FullName,
		user.PasswordHash,
		user.AvatarURL,
		user.Provider,
		user.ProviderID,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return ErrUserAlreadyExists
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	query := `
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u model.User
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.FullName,
		&u.PasswordHash,
		&u.AvatarURL,
		&u.Provider,
		&u.ProviderID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by id: %w", err)
	}
	return &u, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var u model.User
	err := r.db.Pool.QueryRow(ctx, query, strings.TrimSpace(email)).Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.FullName,
		&u.PasswordHash,
		&u.AvatarURL,
		&u.Provider,
		&u.ProviderID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}
	return &u, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	query := `
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, created_at, updated_at
		FROM users
		WHERE LOWER(username) = LOWER($1)
	`
	var u model.User
	err := r.db.Pool.QueryRow(ctx, query, strings.TrimSpace(username)).Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.FullName,
		&u.PasswordHash,
		&u.AvatarURL,
		&u.Provider,
		&u.ProviderID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by username: %w", err)
	}
	return &u, nil
}

func (r *userRepository) GetByIdentifier(ctx context.Context, identifier string) (*model.User, error) {
	query := `
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1) OR LOWER(username) = LOWER($1)
		LIMIT 1
	`
	var u model.User
	err := r.db.Pool.QueryRow(ctx, query, strings.TrimSpace(identifier)).Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.FullName,
		&u.PasswordHash,
		&u.AvatarURL,
		&u.Provider,
		&u.ProviderID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by identifier: %w", err)
	}
	return &u, nil
}

func (r *userRepository) GetByProvider(ctx context.Context, provider, providerID string) (*model.User, error) {
	query := `
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, created_at, updated_at
		FROM users
		WHERE provider = $1 AND provider_id = $2
		LIMIT 1
	`
	var u model.User
	err := r.db.Pool.QueryRow(ctx, query, provider, providerID).Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.FullName,
		&u.PasswordHash,
		&u.AvatarURL,
		&u.Provider,
		&u.ProviderID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by provider: %w", err)
	}
	return &u, nil
}

func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	query := `
		UPDATE users
		SET email = $1, username = $2, full_name = $3, avatar_url = $4, updated_at = $5
		WHERE id = $6
	`
	_, err := r.db.Pool.Exec(ctx, query,
		user.Email,
		user.Username,
		user.FullName,
		user.AvatarURL,
		user.UpdatedAt,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

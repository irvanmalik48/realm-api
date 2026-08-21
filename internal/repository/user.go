package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrUserAlreadyExists     = errors.New("user with this email or username already exists")
	ErrOAuthAccountLinked    = errors.New("this social account is already linked to another user")
	ErrCannotUnlinkLastAuth  = errors.New("cannot disconnect the only login method; set a password first")
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByIdentifier(ctx context.Context, identifier string) (*model.User, error)
	GetByProvider(ctx context.Context, provider, providerID string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	GetOAuthAccounts(ctx context.Context, userID uuid.UUID) ([]model.OAuthAccount, error)
	LinkOAuthAccount(ctx context.Context, account *model.OAuthAccount) error
	UnlinkOAuthAccount(ctx context.Context, userID uuid.UUID, provider string) error
	GetByOAuthAccount(ctx context.Context, provider, providerID string) (*model.User, error)
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

	// If provider is OAuth (not local), also record in user_oauth_accounts
	if user.Provider != "" && user.Provider != "local" && user.ProviderID != nil {
		oauthAcct := &model.OAuthAccount{
			ID:         uuid.New(),
			UserID:     user.ID,
			Provider:   user.Provider,
			ProviderID: *user.ProviderID,
			Email:      &user.Email,
			CreatedAt:  user.CreatedAt,
		}
		_ = r.LinkOAuthAccount(ctx, oauthAcct)
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
	return r.GetByOAuthAccount(ctx, provider, providerID)
}

func (r *userRepository) GetByOAuthAccount(ctx context.Context, provider, providerID string) (*model.User, error) {
	// First check user_oauth_accounts table
	query := `
		SELECT u.id, u.email, u.username, u.full_name, u.password_hash, u.avatar_url, u.provider, u.provider_id, u.created_at, u.updated_at
		FROM users u
		JOIN user_oauth_accounts oa ON u.id = oa.user_id
		WHERE oa.provider = $1 AND oa.provider_id = $2
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
	if err == nil {
		return &u, nil
	}

	// Fallback to users table direct columns for backwards compatibility
	fallbackQuery := `
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, created_at, updated_at
		FROM users
		WHERE provider = $1 AND provider_id = $2
		LIMIT 1
	`
	err = r.db.Pool.QueryRow(ctx, fallbackQuery, provider, providerID).Scan(
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
		return nil, fmt.Errorf("failed to query user by oauth account: %w", err)
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

func (r *userRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $1, updated_at = $2
		WHERE id = $3
	`
	res, err := r.db.Pool.Exec(ctx, query, passwordHash, time.Now().UTC(), userID)
	if err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *userRepository) GetOAuthAccounts(ctx context.Context, userID uuid.UUID) ([]model.OAuthAccount, error) {
	query := `
		SELECT id, user_id, provider, provider_id, email, created_at
		FROM user_oauth_accounts
		WHERE user_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query oauth accounts: %w", err)
	}
	defer rows.Close()

	var accounts []model.OAuthAccount
	for rows.Next() {
		var a model.OAuthAccount
		if err := rows.Scan(&a.ID, &a.UserID, &a.Provider, &a.ProviderID, &a.Email, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan oauth account: %w", err)
		}
		accounts = append(accounts, a)
	}

	return accounts, nil
}

func (r *userRepository) LinkOAuthAccount(ctx context.Context, account *model.OAuthAccount) error {
	if account.ID == uuid.Nil {
		account.ID = uuid.New()
	}
	if account.CreatedAt.IsZero() {
		account.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO user_oauth_accounts (id, user_id, provider, provider_id, email, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider, provider_id) DO UPDATE
		SET user_id = EXCLUDED.user_id, email = EXCLUDED.email
	`
	_, err := r.db.Pool.Exec(ctx, query,
		account.ID,
		account.UserID,
		account.Provider,
		account.ProviderID,
		account.Email,
		account.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return ErrOAuthAccountLinked
		}
		return fmt.Errorf("failed to link oauth account: %w", err)
	}
	return nil
}

func (r *userRepository) UnlinkOAuthAccount(ctx context.Context, userID uuid.UUID, provider string) error {
	// First check that user still has another login method (password or another OAuth provider)
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	accounts, err := r.GetOAuthAccounts(ctx, userID)
	if err != nil {
		return err
	}

	hasPassword := user.PasswordHash != nil && *user.PasswordHash != ""
	if !hasPassword && len(accounts) <= 1 {
		return ErrCannotUnlinkLastAuth
	}

	query := `
		DELETE FROM user_oauth_accounts
		WHERE user_id = $1 AND provider = $2
	`
	res, err := r.db.Pool.Exec(ctx, query, userID, provider)
	if err != nil {
		return fmt.Errorf("failed to unlink oauth account: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errors.New("oauth account not found")
	}

	return nil
}

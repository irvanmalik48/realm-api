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
	Update2FASecret(ctx context.Context, userID uuid.UUID, secret string) error
	Enable2FA(ctx context.Context, userID uuid.UUID, secret string, recoveryCodes []string) error
	Disable2FA(ctx context.Context, userID uuid.UUID) error
	ConsumeRecoveryCode(ctx context.Context, userID uuid.UUID, code string) (bool, error)
	ListUsers(ctx context.Context, search, provider string, limit, offset int) ([]model.UserDTO, int, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
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
			AvatarURL:  user.AvatarURL,
			CreatedAt:  user.CreatedAt,
		}
		_ = r.LinkOAuthAccount(ctx, oauthAcct)
	}

	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	query := `
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, two_factor_enabled, two_factor_secret, COALESCE(two_factor_recovery_codes, '{}'), created_at, updated_at
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
		&u.TwoFactorEnabled,
		&u.TwoFactorSecret,
		&u.TwoFactorRecoveryCodes,
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
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, two_factor_enabled, two_factor_secret, COALESCE(two_factor_recovery_codes, '{}'), created_at, updated_at
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
		&u.TwoFactorEnabled,
		&u.TwoFactorSecret,
		&u.TwoFactorRecoveryCodes,
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
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, two_factor_enabled, two_factor_secret, COALESCE(two_factor_recovery_codes, '{}'), created_at, updated_at
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
		&u.TwoFactorEnabled,
		&u.TwoFactorSecret,
		&u.TwoFactorRecoveryCodes,
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
	norm := strings.ToLower(strings.TrimSpace(identifier))
	query := `
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, two_factor_enabled, two_factor_secret, COALESCE(two_factor_recovery_codes, '{}'), created_at, updated_at
		FROM users
		WHERE email = $1 OR username = $1
		LIMIT 1
	`
	var u model.User
	err := r.db.Pool.QueryRow(ctx, query, norm).Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.FullName,
		&u.PasswordHash,
		&u.AvatarURL,
		&u.Provider,
		&u.ProviderID,
		&u.TwoFactorEnabled,
		&u.TwoFactorSecret,
		&u.TwoFactorRecoveryCodes,
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
		SELECT u.id, u.email, u.username, u.full_name, u.password_hash, u.avatar_url, u.provider, u.provider_id, u.two_factor_enabled, u.two_factor_secret, COALESCE(u.two_factor_recovery_codes, '{}'), u.created_at, u.updated_at
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
		&u.TwoFactorEnabled,
		&u.TwoFactorSecret,
		&u.TwoFactorRecoveryCodes,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err == nil {
		return &u, nil
	}

	// Fallback to users table direct columns for backwards compatibility
	fallbackQuery := `
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, two_factor_enabled, two_factor_secret, COALESCE(two_factor_recovery_codes, '{}'), created_at, updated_at
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
		&u.TwoFactorEnabled,
		&u.TwoFactorSecret,
		&u.TwoFactorRecoveryCodes,
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
		SELECT id, user_id, provider, provider_id, email, avatar_url, created_at
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
		if err := rows.Scan(&a.ID, &a.UserID, &a.Provider, &a.ProviderID, &a.Email, &a.AvatarURL, &a.CreatedAt); err != nil {
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

	// Verify the provider account is not already linked to a different user
	var existingUserID uuid.UUID
	checkQuery := `SELECT user_id FROM user_oauth_accounts WHERE provider = $1 AND provider_id = $2`
	err := r.db.Pool.QueryRow(ctx, checkQuery, account.Provider, account.ProviderID).Scan(&existingUserID)
	if err == nil && existingUserID != account.UserID {
		return ErrOAuthAccountLinked
	}

	query := `
		INSERT INTO user_oauth_accounts (id, user_id, provider, provider_id, email, avatar_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (provider, provider_id) DO UPDATE
		SET email = EXCLUDED.email, avatar_url = COALESCE(EXCLUDED.avatar_url, user_oauth_accounts.avatar_url)
		WHERE user_oauth_accounts.user_id = EXCLUDED.user_id
	`
	_, err = r.db.Pool.Exec(ctx, query,
		account.ID,
		account.UserID,
		account.Provider,
		account.ProviderID,
		account.Email,
		account.AvatarURL,
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

func (r *userRepository) Update2FASecret(ctx context.Context, userID uuid.UUID, secret string) error {
	query := `UPDATE users SET two_factor_secret = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, secret, userID)
	return err
}

func (r *userRepository) Enable2FA(ctx context.Context, userID uuid.UUID, secret string, recoveryCodes []string) error {
	query := `
		UPDATE users
		SET two_factor_enabled = true,
		    two_factor_secret = $1,
		    two_factor_recovery_codes = $2,
		    updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.db.Pool.Exec(ctx, query, secret, recoveryCodes, userID)
	return err
}

func (r *userRepository) Disable2FA(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET two_factor_enabled = false,
		    two_factor_secret = NULL,
		    two_factor_recovery_codes = '{}',
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	return err
}

func (r *userRepository) ConsumeRecoveryCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	normCode := strings.ToUpper(strings.TrimSpace(code))
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}

	foundIndex := -1
	for i, c := range user.TwoFactorRecoveryCodes {
		if strings.ToUpper(strings.TrimSpace(c)) == normCode {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		return false, nil
	}

	updatedCodes := append(user.TwoFactorRecoveryCodes[:foundIndex], user.TwoFactorRecoveryCodes[foundIndex+1:]...)
	query := `UPDATE users SET two_factor_recovery_codes = $1, updated_at = NOW() WHERE id = $2`
	_, err = r.db.Pool.Exec(ctx, query, updatedCodes, userID)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *userRepository) ListUsers(ctx context.Context, search, provider string, limit, offset int) ([]model.UserDTO, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if s := strings.TrimSpace(search); s != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("(LOWER(username) LIKE $%d OR LOWER(email) LIKE $%d OR LOWER(full_name) LIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, "%"+strings.ToLower(s)+"%")
		argIdx++
	}

	if p := strings.TrimSpace(provider); p != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("(provider = $%d OR EXISTS (SELECT 1 FROM user_oauth_accounts WHERE user_oauth_accounts.user_id = users.id AND user_oauth_accounts.provider = $%d))", argIdx, argIdx))
		args = append(args, strings.ToLower(p))
		argIdx++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users WHERE %s", whereSQL)
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, email, username, full_name, password_hash, avatar_url, provider, provider_id, two_factor_enabled, created_at, updated_at
		FROM users
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	var userIDs []uuid.UUID

	for rows.Next() {
		var u model.User
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Username, &u.FullName, &u.PasswordHash, &u.AvatarURL,
			&u.Provider, &u.ProviderID, &u.TwoFactorEnabled, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &u)
		userIDs = append(userIDs, u.ID)
	}

	if len(users) == 0 {
		return []model.UserDTO{}, total, nil
	}

	accountsQuery := `
		SELECT id, user_id, provider, provider_id, email, avatar_url, created_at
		FROM user_oauth_accounts
		WHERE user_id = ANY($1)
	`
	accRows, err := r.db.Pool.Query(ctx, accountsQuery, userIDs)
	oauthMap := make(map[uuid.UUID][]model.OAuthAccount)
	if err == nil {
		defer accRows.Close()
		for accRows.Next() {
			var a model.OAuthAccount
			if scanErr := accRows.Scan(&a.ID, &a.UserID, &a.Provider, &a.ProviderID, &a.Email, &a.AvatarURL, &a.CreatedAt); scanErr == nil {
				oauthMap[a.UserID] = append(oauthMap[a.UserID], a)
			}
		}
	}

	dtos := make([]model.UserDTO, 0, len(users))
	for _, u := range users {
		dto := u.ToDTOWithAccounts(oauthMap[u.ID])
		if dto != nil {
			dtos = append(dtos, *dto)
		}
	}

	return dtos, total, nil
}

func (r *userRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	var isSuperadmin bool
	err := r.db.Pool.QueryRow(ctx, `SELECT is_superadmin FROM admin_users WHERE user_id = $1`, id).Scan(&isSuperadmin)
	if err == nil && isSuperadmin {
		return errors.New("cannot delete superadmin user")
	}

	_, _ = r.db.Pool.Exec(ctx, `UPDATE admin_users SET created_by = NULL WHERE created_by = $1`, id)

	result, err := r.db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}



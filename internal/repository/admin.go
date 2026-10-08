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
	ErrAdminNotFound = errors.New("admin user not found")
	ErrCannotRemoveSuperadmin = errors.New("cannot remove superadmin")
)

type AdminRepository interface {
	ListAdmins(ctx context.Context) ([]model.AdminUser, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*model.AdminUser, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.AdminUser, error)
	AddAdmin(ctx context.Context, email string, permissions []string, createdBy *uuid.UUID) (*model.AdminUser, error)
	UpdatePermissions(ctx context.Context, adminID uuid.UUID, permissions []string) (*model.AdminUser, error)
	RemoveAdmin(ctx context.Context, adminID uuid.UUID) error
}

type adminRepository struct {
	db *database.DB
}

func NewAdminRepository(db *database.DB) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) ListAdmins(ctx context.Context) ([]model.AdminUser, error) {
	query := `
		SELECT 
			a.id, a.user_id, a.is_superadmin, a.permissions, a.created_by, a.created_at, a.updated_at,
			u.id, u.email, u.username, u.full_name, u.avatar_url, u.provider, u.created_at
		FROM admin_users a
		JOIN users u ON a.user_id = u.id
		ORDER BY a.is_superadmin DESC, a.created_at ASC
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query admin users: %w", err)
	}
	defer rows.Close()

	var admins []model.AdminUser
	for rows.Next() {
		var a model.AdminUser
		var u model.User
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.IsSuperadmin, &a.Permissions, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
			&u.ID, &u.Email, &u.Username, &u.FullName, &u.AvatarURL, &u.Provider, &u.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan admin user: %w", err)
		}
		a.User = u
		admins = append(admins, a)
	}
	return admins, nil
}

func (r *adminRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*model.AdminUser, error) {
	query := `
		SELECT 
			a.id, a.user_id, a.is_superadmin, a.permissions, a.created_by, a.created_at, a.updated_at,
			u.id, u.email, u.username, u.full_name, u.avatar_url, u.provider, u.created_at
		FROM admin_users a
		JOIN users u ON a.user_id = u.id
		WHERE a.user_id = $1
	`
	var a model.AdminUser
	var u model.User
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&a.ID, &a.UserID, &a.IsSuperadmin, &a.Permissions, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
		&u.ID, &u.Email, &u.Username, &u.FullName, &u.AvatarURL, &u.Provider, &u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get admin by user_id: %w", err)
	}
	a.User = u
	return &a, nil
}

func (r *adminRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AdminUser, error) {
	query := `
		SELECT 
			a.id, a.user_id, a.is_superadmin, a.permissions, a.created_by, a.created_at, a.updated_at,
			u.id, u.email, u.username, u.full_name, u.avatar_url, u.provider, u.created_at
		FROM admin_users a
		JOIN users u ON a.user_id = u.id
		WHERE a.id = $1
	`
	var a model.AdminUser
	var u model.User
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.UserID, &a.IsSuperadmin, &a.Permissions, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
		&u.ID, &u.Email, &u.Username, &u.FullName, &u.AvatarURL, &u.Provider, &u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get admin by id: %w", err)
	}
	a.User = u
	return &a, nil
}

func (r *adminRepository) AddAdmin(ctx context.Context, email string, permissions []string, createdBy *uuid.UUID) (*model.AdminUser, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))

	// Find user by email
	var u model.User
	userQuery := `SELECT id, email, username, full_name, avatar_url, provider, created_at FROM users WHERE LOWER(email) = $1`
	err := r.db.Pool.QueryRow(ctx, userQuery, cleanEmail).Scan(
		&u.ID, &u.Email, &u.Username, &u.FullName, &u.AvatarURL, &u.Provider, &u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user with email %s not found; they must register an account first", cleanEmail)
		}
		return nil, fmt.Errorf("failed to lookup user: %w", err)
	}

	insertQuery := `
		INSERT INTO admin_users (user_id, is_superadmin, permissions, created_by)
		VALUES ($1, false, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET permissions = EXCLUDED.permissions, updated_at = NOW()
		WHERE admin_users.is_superadmin = false
		RETURNING id, user_id, is_superadmin, permissions, created_by, created_at, updated_at
	`
	var a model.AdminUser
	err = r.db.Pool.QueryRow(ctx, insertQuery, u.ID, permissions, createdBy).Scan(
		&a.ID, &a.UserID, &a.IsSuperadmin, &a.Permissions, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert admin user: %w", err)
	}
	a.User = u
	return &a, nil
}

func (r *adminRepository) UpdatePermissions(ctx context.Context, adminID uuid.UUID, permissions []string) (*model.AdminUser, error) {
	query := `
		UPDATE admin_users
		SET permissions = $1, updated_at = NOW()
		WHERE id = $2 AND is_superadmin = false
		RETURNING id, user_id, is_superadmin, permissions, created_by, created_at, updated_at
	`
	var a model.AdminUser
	err := r.db.Pool.QueryRow(ctx, query, permissions, adminID).Scan(
		&a.ID, &a.UserID, &a.IsSuperadmin, &a.Permissions, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("admin not found or is superadmin")
		}
		return nil, fmt.Errorf("failed to update admin permissions: %w", err)
	}

	// Fetch user details
	admin, err := r.GetByID(ctx, adminID)
	if err != nil {
		return nil, err
	}
	return admin, nil
}

func (r *adminRepository) RemoveAdmin(ctx context.Context, adminID uuid.UUID) error {
	query := `DELETE FROM admin_users WHERE id = $1 AND is_superadmin = false`
	cmdTag, err := r.db.Pool.Exec(ctx, query, adminID)
	if err != nil {
		return fmt.Errorf("failed to delete admin: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("admin not found or is superadmin")
	}
	return nil
}

package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

type ContactRepository interface {
	Create(ctx context.Context, submission *model.ContactSubmission) error
	List(ctx context.Context, limit, offset int, search string) ([]model.ContactSubmission, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type contactRepository struct {
	db *database.DB
}

func NewContactRepository(db *database.DB) ContactRepository {
	return &contactRepository{db: db}
}

func (r *contactRepository) Create(ctx context.Context, submission *model.ContactSubmission) error {
	query := `
	INSERT INTO contact_submissions (name, email, subject, message, ip_address, user_agent)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, created_at
	`

	return r.db.Pool.QueryRow(
		ctx,
		query,
		submission.Name,
		submission.Email,
		submission.Subject,
		submission.Message,
		submission.IPAddress,
		submission.UserAgent,
	).Scan(&submission.ID, &submission.CreatedAt)
}

func (r *contactRepository) List(ctx context.Context, limit, offset int, search string) ([]model.ContactSubmission, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := ""
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		whereClause = fmt.Sprintf("WHERE name ILIKE $%d OR email ILIKE $%d OR subject ILIKE $%d OR message ILIKE $%d", argIdx, argIdx, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM contact_submissions %s", whereClause)
	var total int
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count submissions: %w", err)
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, name, email, subject, message, ip_address, user_agent, created_at
		FROM contact_submissions
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query contact submissions: %w", err)
	}
	defer rows.Close()

	var submissions []model.ContactSubmission
	for rows.Next() {
		var s model.ContactSubmission
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Email, &s.Subject, &s.Message, &s.IPAddress, &s.UserAgent, &s.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan submission: %w", err)
		}
		submissions = append(submissions, s)
	}

	return submissions, total, nil
}

func (r *contactRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM contact_submissions WHERE id = $1`
	cmdTag, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete submission: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("submission not found")
	}
	return nil
}


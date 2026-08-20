package repository

import (
	"context"

	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

type ContactRepository interface {
	Create(ctx context.Context, submission *model.ContactSubmission) error
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

package model

import (
	"time"

	"github.com/google/uuid"
)

// Paste represents the database row for a pastebin entry
type Paste struct {
	ID        uuid.UUID  `json:"id"`
	Slug      string     `json:"slug"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Syntax    string     `json:"syntax"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	IsPublic    bool       `json:"is_public"`
	IsEncrypted bool       `json:"is_encrypted"`
	Views       int        `json:"views"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// PasteSummaryDTO represents a lightweight representation of a paste for listing
type PasteSummaryDTO struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Syntax      string     `json:"syntax"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	IsPublic    bool       `json:"is_public"`
	IsEncrypted bool       `json:"is_encrypted"`
	Views       int        `json:"views"`
	Size        int        `json:"size"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreatePasteRequest is the payload to create a new paste
type CreatePasteRequest struct {
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Syntax    string     `json:"syntax"`
	Password  string     `json:"password,omitempty"`
	IsPublic  *bool      `json:"is_public,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	TTL       string     `json:"ttl,omitempty"` // e.g. "1h", "1d", "7d", "30d", "never"
}

// PastesListResponse is the response structure for listing pastes
type PastesListResponse struct {
	Total  int               `json:"total"`
	Pastes []PasteSummaryDTO `json:"pastes"`
}

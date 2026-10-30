package model

import (
	"time"

	"github.com/google/uuid"
)

// Paste represents the database row for a pastebin entry
type Paste struct {
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Syntax      string     `json:"syntax"`
	Views       int        `json:"views"`
	ID          uuid.UUID  `json:"id"`
	IsPublic    bool       `json:"is_public"`
	IsEncrypted bool       `json:"is_encrypted"`
	BurnAfterRead bool     `json:"burn_after_read"`
}

// PasteSummaryDTO represents a lightweight representation of a paste for listing
type PasteSummaryDTO struct {
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Syntax      string     `json:"syntax"`
	Views       int        `json:"views"`
	Size        int        `json:"size"`
	ID          uuid.UUID  `json:"id"`
	IsPublic    bool       `json:"is_public"`
	IsEncrypted bool       `json:"is_encrypted"`
	BurnAfterRead bool     `json:"burn_after_read"`
}

// CreatePasteRequest is the payload to create a new paste
type CreatePasteRequest struct {
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsPublic      *bool      `json:"is_public,omitempty"`
	BurnAfterRead *bool      `json:"burn_after_read,omitempty"`
	Title         string     `json:"title"`
	Content       string     `json:"content"`
	Syntax        string     `json:"syntax"`
	Password      string     `json:"password,omitempty"`
	TTL           string     `json:"ttl,omitempty"` // e.g. "burn", "10m", "1h", "1d", "7d", "30d", "never"
}

// PastesListResponse is the response structure for listing pastes
type PastesListResponse struct {
	Pastes []PasteSummaryDTO `json:"pastes"`
	Total  int               `json:"total"`
}

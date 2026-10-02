package model

import (
	"time"

	"github.com/google/uuid"
)

type AdminUser struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	User         User      `json:"user"`
	IsSuperadmin bool      `json:"is_superadmin"`
	Permissions  []string  `json:"permissions"`
	CreatedBy    *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SystemLog struct {
	ID             uuid.UUID `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	Level          string    `json:"level"`
	Message        string    `json:"message"`
	Component      string    `json:"component"`
	TraceID        string    `json:"trace_id"`
	SpanID         string    `json:"span_id"`
	AttributesJSON string    `json:"attributes_json"`
}

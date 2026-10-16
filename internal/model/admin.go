package model

import (
	"time"

	"github.com/google/uuid"
)

type AdminUser struct {
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CreatedBy    *uuid.UUID `json:"created_by,omitempty"`
	Permissions  []string   `json:"permissions"`
	User         User       `json:"user"`
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	IsSuperadmin bool       `json:"is_superadmin"`
}

type SystemLog struct {
	Timestamp      time.Time `json:"timestamp"`
	Level          string    `json:"level"`
	Message        string    `json:"message"`
	Component      string    `json:"component"`
	TraceID        string    `json:"trace_id"`
	SpanID         string    `json:"span_id"`
	AttributesJSON string    `json:"attributes_json"`
	ID             uuid.UUID `json:"id"`
}

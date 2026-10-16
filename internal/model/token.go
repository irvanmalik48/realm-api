package model

import (
	"time"

	"github.com/google/uuid"
)

type APIToken struct {
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Name         string     `json:"name"`
	TokenPrefix  string     `json:"token_prefix"`
	TokenHash    string     `json:"token_hash"`
	Scopes       []string   `json:"scopes"`
	RateLimitRPM int        `json:"rate_limit_rpm"`
	ID           uuid.UUID  `json:"id"`
	IsRevoked    bool       `json:"is_revoked"`
}

type TokenDTO struct {
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Name         string     `json:"name"`
	TokenPrefix  string     `json:"token_prefix"`
	Scopes       []string   `json:"scopes"`
	RateLimitRPM int        `json:"rate_limit_rpm"`
	ID           uuid.UUID  `json:"id"`
	IsRevoked    bool       `json:"is_revoked"`
}

type TokenCreateResult struct {
	Raw   string   `json:"raw_token"`
	Token TokenDTO `json:"token"`
}

type TokenCreateInput struct {
	Name         string
	Scopes       []string
	RateLimitRPM int
	ExpiresIn    time.Duration
}

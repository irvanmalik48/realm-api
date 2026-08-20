package model

import (
	"time"

	"github.com/google/uuid"
)

type APIToken struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	TokenPrefix  string     `json:"token_prefix"`
	TokenHash    string     `json:"token_hash"`
	Scopes       []string   `json:"scopes"`
	RateLimitRPM int        `json:"rate_limit_rpm"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	IsRevoked    bool       `json:"is_revoked"`
	CreatedAt    time.Time  `json:"created_at"`
}

type TokenDTO struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	TokenPrefix  string     `json:"token_prefix"`
	Scopes       []string   `json:"scopes"`
	RateLimitRPM int        `json:"rate_limit_rpm"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	IsRevoked    bool       `json:"is_revoked"`
	CreatedAt    time.Time  `json:"created_at"`
}

type TokenCreateResult struct {
	Token TokenDTO `json:"token"`
	Raw   string   `json:"raw_token"`
}

type TokenCreateInput struct {
	Name         string
	Scopes       []string
	RateLimitRPM int
	ExpiresIn    time.Duration
}

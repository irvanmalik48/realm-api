package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	PasswordHash *string   `json:"-"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	Provider     string    `json:"provider"`
	ProviderID   *string   `json:"provider_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type OAuthAccount struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Provider   string    `json:"provider"`
	ProviderID string    `json:"provider_id"`
	Email      *string   `json:"email,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserDTO struct {
	ID                 uuid.UUID `json:"id"`
	Email              string    `json:"email"`
	Username           string    `json:"username"`
	FullName           string    `json:"full_name"`
	AvatarURL          *string   `json:"avatar_url,omitempty"`
	Provider           string    `json:"provider"`
	HasPassword        bool      `json:"has_password"`
	ConnectedProviders []string  `json:"connected_providers"`
	CreatedAt          time.Time `json:"created_at"`
}

func (u *User) ToDTO() *UserDTO {
	if u == nil {
		return nil
	}
	hasPassword := u.PasswordHash != nil && *u.PasswordHash != ""
	providers := make([]string, 0)
	if u.Provider != "" && u.Provider != "local" {
		providers = append(providers, u.Provider)
	}

	return &UserDTO{
		ID:                 u.ID,
		Email:              u.Email,
		Username:           u.Username,
		FullName:           u.FullName,
		AvatarURL:          u.AvatarURL,
		Provider:           u.Provider,
		HasPassword:        hasPassword,
		ConnectedProviders: providers,
		CreatedAt:          u.CreatedAt,
	}
}

func (u *User) ToDTOWithProviders(connectedProviders []string) *UserDTO {
	dto := u.ToDTO()
	if dto == nil {
		return nil
	}

	// Merge unique providers
	seen := make(map[string]bool)
	merged := make([]string, 0)
	for _, p := range dto.ConnectedProviders {
		if !seen[p] {
			seen[p] = true
			merged = append(merged, p)
		}
	}
	for _, p := range connectedProviders {
		if !seen[p] {
			seen[p] = true
			merged = append(merged, p)
		}
	}
	dto.ConnectedProviders = merged
	return dto
}

type RegisterInput struct {
	Email     string  `json:"email"`
	Username  string  `json:"username"`
	Password  string  `json:"password"`
	FullName  string  `json:"full_name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type LoginInput struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type SetPasswordInput struct {
	CurrentPassword *string `json:"current_password,omitempty"`
	NewPassword     string  `json:"new_password"`
}

type UpdateProfileInput struct {
	FullName  *string `json:"full_name,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type AuthResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	Token   string   `json:"token,omitempty"`
	User    *UserDTO `json:"user,omitempty"`
}

type UserClaims struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Provider  string    `json:"provider"`
	Issuer    string    `json:"iss"`
	Subject   string    `json:"sub"`
	Audience  string    `json:"aud"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
}

type CheckAvailabilityResponse struct {
	UsernameAvailable *bool  `json:"username_available,omitempty"`
	EmailAvailable    *bool  `json:"email_available,omitempty"`
	UsernameReason    string `json:"username_reason,omitempty"`
	EmailReason       string `json:"email_reason,omitempty"`
}

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
	AvatarURL  *string   `json:"avatar_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type OAuthAccountDTO struct {
	Provider  string  `json:"provider"`
	Email     *string `json:"email,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type UserDTO struct {
	ID                 uuid.UUID         `json:"id"`
	Email              string            `json:"email"`
	Username           string            `json:"username"`
	FullName           string            `json:"full_name"`
	AvatarURL          *string           `json:"avatar_url,omitempty"`
	Provider           string            `json:"provider"`
	HasPassword        bool              `json:"has_password"`
	ConnectedProviders []string          `json:"connected_providers"`
	ConnectedAccounts  []OAuthAccountDTO `json:"connected_accounts"`
	CreatedAt          time.Time         `json:"created_at"`
}

func (u *User) ToDTO() *UserDTO {
	if u == nil {
		return nil
	}
	hasPassword := u.PasswordHash != nil && *u.PasswordHash != ""
	providers := make([]string, 0)
	accounts := make([]OAuthAccountDTO, 0)

	if u.Provider != "" && u.Provider != "local" {
		providers = append(providers, u.Provider)
		accounts = append(accounts, OAuthAccountDTO{
			Provider:  u.Provider,
			Email:     &u.Email,
			AvatarURL: u.AvatarURL,
		})
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
		ConnectedAccounts:  accounts,
		CreatedAt:          u.CreatedAt,
	}
}

func (u *User) ToDTOWithAccounts(oauthAccounts []OAuthAccount) *UserDTO {
	dto := u.ToDTO()
	if dto == nil {
		return nil
	}

	seenProviders := make(map[string]bool)
	providers := make([]string, 0)
	accounts := make([]OAuthAccountDTO, 0)

	for _, p := range dto.ConnectedProviders {
		if !seenProviders[p] {
			seenProviders[p] = true
			providers = append(providers, p)
		}
	}
	for _, a := range dto.ConnectedAccounts {
		accounts = append(accounts, a)
	}

	for _, a := range oauthAccounts {
		if !seenProviders[a.Provider] {
			seenProviders[a.Provider] = true
			providers = append(providers, a.Provider)
		}
		// Check if already in accounts list
		found := false
		for i, existing := range accounts {
			if existing.Provider == a.Provider {
				found = true
				if a.AvatarURL != nil && *a.AvatarURL != "" {
					accounts[i].AvatarURL = a.AvatarURL
				}
				if a.Email != nil && *a.Email != "" {
					accounts[i].Email = a.Email
				}
				break
			}
		}
		if !found {
			accounts = append(accounts, OAuthAccountDTO{
				Provider:  a.Provider,
				Email:     a.Email,
				AvatarURL: a.AvatarURL,
			})
		}
	}

	dto.ConnectedProviders = providers
	dto.ConnectedAccounts = accounts
	return dto
}

func (u *User) ToDTOWithProviders(connectedProviders []string) *UserDTO {
	dto := u.ToDTO()
	if dto == nil {
		return nil
	}

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
	Username  *string `json:"username,omitempty"`
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

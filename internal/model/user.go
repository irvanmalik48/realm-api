package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UpdatedAt              time.Time `json:"updated_at"`
	CreatedAt              time.Time `json:"created_at"`
	ProviderID             *string   `json:"provider_id,omitempty"`
	TwoFactorSecret        *string   `json:"-"`
	PasswordHash           *string   `json:"-"`
	AvatarURL              *string   `json:"avatar_url,omitempty"`
	FullName               string    `json:"full_name"`
	Provider               string    `json:"provider"`
	Username               string    `json:"username"`
	Email                  string    `json:"email"`
	TwoFactorRecoveryCodes []string  `json:"-"`
	ID                     uuid.UUID `json:"id"`
	TwoFactorEnabled       bool      `json:"two_factor_enabled"`
	IsActive               bool      `json:"is_active"`
}

type OAuthAccount struct {
	CreatedAt  time.Time `json:"created_at"`
	Email      *string   `json:"email,omitempty"`
	AvatarURL  *string   `json:"avatar_url,omitempty"`
	Provider   string    `json:"provider"`
	ProviderID string    `json:"provider_id"`
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
}

type OAuthAccountDTO struct {
	Email     *string `json:"email,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Provider  string  `json:"provider"`
}

type UserDTO struct {
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	AvatarURL          *string           `json:"avatar_url,omitempty"`
	Email              string            `json:"email"`
	Username           string            `json:"username"`
	FullName           string            `json:"full_name"`
	Provider           string            `json:"provider"`
	ConnectedProviders []string          `json:"connected_providers"`
	ConnectedAccounts  []OAuthAccountDTO `json:"connected_accounts"`
	ID                 uuid.UUID         `json:"id"`
	IsActive           bool              `json:"is_active"`
	HasPassword        bool              `json:"has_password"`
	TwoFactorEnabled   bool              `json:"two_factor_enabled"`
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
		TwoFactorEnabled:   u.TwoFactorEnabled,
		HasPassword:        hasPassword,
		IsActive:           u.IsActive,
		ConnectedProviders: providers,
		ConnectedAccounts:  accounts,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
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
	accounts = append(accounts, dto.ConnectedAccounts...)

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
	AvatarURL *string `json:"avatar_url,omitempty"`
	Email     string  `json:"email"`
	Username  string  `json:"username"`
	Password  string  `json:"password"`
	FullName  string  `json:"full_name"`
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
	User              *UserDTO `json:"user,omitempty"`
	Status            string   `json:"status"`
	Message           string   `json:"message,omitempty"`
	Token             string   `json:"token,omitempty"`
	TempToken         string   `json:"temp_token,omitempty"`
	TwoFactorRequired bool     `json:"two_factor_required,omitempty"`
}

type Setup2FAResponse struct {
	Secret     string `json:"secret"`
	QRCode     string `json:"qr_code"`
	OtpauthURL string `json:"otpauth_url"`
}

type Enable2FARequest struct {
	Code   string `json:"code"`
	Secret string `json:"secret,omitempty"`
}

type Enable2FAResponse struct {
	Status        string   `json:"status"`
	Message       string   `json:"message"`
	RecoveryCodes []string `json:"recovery_codes"`
}

type Verify2FARequest struct {
	TempToken string `json:"temp_token"`
	Code      string `json:"code"`
}

type Disable2FARequest struct {
	Code     string `json:"code,omitempty"`
	Password string `json:"password,omitempty"`
}

type Disable2FAResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type UserClaims struct {
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Provider  string    `json:"provider"`
	Purpose   string    `json:"purpose,omitempty"`
	Issuer    string    `json:"iss"`
	Subject   string    `json:"sub"`
	Audience  string    `json:"aud"`
}

type CheckAvailabilityResponse struct {
	UsernameAvailable *bool  `json:"username_available,omitempty"`
	EmailAvailable    *bool  `json:"email_available,omitempty"`
	UsernameReason    string `json:"username_reason,omitempty"`
	EmailReason       string `json:"email_reason,omitempty"`
}

type UpdateUserInput struct {
	Email    *string `json:"email"`
	Username *string `json:"username"`
	FullName *string `json:"full_name"`
	IsActive *bool   `json:"is_active"`
}

package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email/username or password")
	ErrInvalidEmail       = errors.New("invalid email address format")
	ErrInvalidUsername    = errors.New("username must be 3-30 characters (alphanumeric and underscores only)")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters long")
	ErrFullNameRequired   = errors.New("full name must be between 2 and 100 characters")
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrCurrentPasswordReq = errors.New("current password is required to change password")
	ErrCurrentPasswordBad = errors.New("incorrect current password")
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)

type AuthService interface {
	Register(ctx context.Context, input model.RegisterInput) (*model.AuthResponse, error)
	Login(ctx context.Context, input model.LoginInput) (*model.AuthResponse, error)
	HandleOAuthLogin(ctx context.Context, userInfo *OAuthUserInfo) (*model.AuthResponse, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserDTO, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, input model.UpdateProfileInput) (*model.UserDTO, error)
	SetPassword(ctx context.Context, userID uuid.UUID, input model.SetPasswordInput) error
	LinkOAuthAccount(ctx context.Context, userID uuid.UUID, userInfo *OAuthUserInfo) error
	UnlinkOAuthAccount(ctx context.Context, userID uuid.UUID, provider string) error
	CheckAvailability(ctx context.Context, username, email string) (*model.CheckAvailabilityResponse, error)
}

type authService struct {
	userRepo  repository.UserRepository
	pasetoSvc auth.PasetoService
	tokenTTL  time.Duration
}

func NewAuthService(userRepo repository.UserRepository, pasetoSvc auth.PasetoService) AuthService {
	return &authService{
		userRepo:  userRepo,
		pasetoSvc: pasetoSvc,
		tokenTTL:  7 * 24 * time.Hour,
	}
}

func (s *authService) getConnectedAccounts(ctx context.Context, userID uuid.UUID) []model.OAuthAccount {
	if s.userRepo == nil {
		return nil
	}
	accts, err := s.userRepo.GetOAuthAccounts(ctx, userID)
	if err != nil {
		return nil
	}
	return accts
}

func (s *authService) getConnectedProviders(ctx context.Context, userID uuid.UUID) []string {
	accts := s.getConnectedAccounts(ctx, userID)
	if accts == nil {
		return nil
	}
	providers := make([]string, 0, len(accts))
	for _, a := range accts {
		providers = append(providers, a.Provider)
	}
	return providers
}

func (s *authService) Register(ctx context.Context, input model.RegisterInput) (*model.AuthResponse, error) {
	if s.userRepo == nil {
		return nil, errors.New("database repository unavailable")
	}

	email := strings.TrimSpace(input.Email)
	username := strings.TrimSpace(input.Username)
	fullName := strings.TrimSpace(input.FullName)

	// 1. Validation
	if _, err := mail.ParseAddress(email); err != nil || !strings.Contains(email, "@") {
		return nil, ErrInvalidEmail
	}
	if !usernameRegex.MatchString(username) {
		return nil, ErrInvalidUsername
	}
	if len(input.Password) < 8 {
		return nil, ErrPasswordTooShort
	}
	if len(fullName) < 2 || len(fullName) > 100 {
		return nil, ErrFullNameRequired
	}

	// 2. Hash Password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	hashStr := string(hashedPassword)

	// 3. Create User in Repository
	now := time.Now().UTC()
	user := &model.User{
		ID:           uuid.New(),
		Email:        strings.ToLower(email),
		Username:     strings.ToLower(username),
		FullName:     fullName,
		PasswordHash: &hashStr,
		AvatarURL:    input.AvatarURL,
		Provider:     "local",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 4. Generate PASETO Bearer Token
	token, err := s.pasetoSvc.GenerateToken(user, s.tokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &model.AuthResponse{
		Status:  "success",
		Message: "Registration successful",
		Token:   token,
		User:    user.ToDTO(),
	}, nil
}

func (s *authService) Login(ctx context.Context, input model.LoginInput) (*model.AuthResponse, error) {
	if s.userRepo == nil {
		return nil, errors.New("database repository unavailable")
	}

	identifier := strings.TrimSpace(input.Identifier)
	if identifier == "" || input.Password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.userRepo.GetByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return nil, errors.New("account was created with social login. Please sign in with Google or GitHub")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.pasetoSvc.GenerateToken(user, s.tokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	accounts := s.getConnectedAccounts(ctx, user.ID)

	return &model.AuthResponse{
		Status:  "success",
		Message: "Login successful",
		Token:   token,
		User:    user.ToDTOWithAccounts(accounts),
	}, nil
}

func (s *authService) HandleOAuthLogin(ctx context.Context, userInfo *OAuthUserInfo) (*model.AuthResponse, error) {
	if s.userRepo == nil {
		return nil, errors.New("database user repository unavailable")
	}

	var avatarPtr *string
	if userInfo.AvatarURL != "" {
		avatarPtr = &userInfo.AvatarURL
	}

	// 1. Try finding by OAuth account
	user, err := s.userRepo.GetByOAuthAccount(ctx, userInfo.Provider, userInfo.ProviderID)
	if err == nil && user != nil {
		// Update avatar if provided
		if userInfo.AvatarURL != "" && (user.AvatarURL == nil || *user.AvatarURL == "") {
			user.AvatarURL = &userInfo.AvatarURL
			user.UpdatedAt = time.Now().UTC()
			_ = s.userRepo.Update(ctx, user)
		}

		// Also link/update avatar in oauth accounts table
		_ = s.userRepo.LinkOAuthAccount(ctx, &model.OAuthAccount{
			UserID:     user.ID,
			Provider:   userInfo.Provider,
			ProviderID: userInfo.ProviderID,
			Email:      &userInfo.Email,
			AvatarURL:  avatarPtr,
			CreatedAt:  time.Now().UTC(),
		})

		token, err := s.pasetoSvc.GenerateToken(user, s.tokenTTL)
		if err != nil {
			return nil, err
		}
		accounts := s.getConnectedAccounts(ctx, user.ID)
		return &model.AuthResponse{
			Status:  "success",
			Message: "OAuth login successful",
			Token:   token,
			User:    user.ToDTOWithAccounts(accounts),
		}, nil
	}

	// 2. Try finding by email to link account
	user, err = s.userRepo.GetByEmail(ctx, userInfo.Email)
	if err == nil && user != nil {
		oauthAcct := &model.OAuthAccount{
			ID:         uuid.New(),
			UserID:     user.ID,
			Provider:   userInfo.Provider,
			ProviderID: userInfo.ProviderID,
			Email:      &userInfo.Email,
			AvatarURL:  avatarPtr,
			CreatedAt:  time.Now().UTC(),
		}
		_ = s.userRepo.LinkOAuthAccount(ctx, oauthAcct)

		if user.AvatarURL == nil || *user.AvatarURL == "" {
			user.AvatarURL = &userInfo.AvatarURL
			user.UpdatedAt = time.Now().UTC()
			_ = s.userRepo.Update(ctx, user)
		}

		token, err := s.pasetoSvc.GenerateToken(user, s.tokenTTL)
		if err != nil {
			return nil, err
		}
		accounts := s.getConnectedAccounts(ctx, user.ID)
		return &model.AuthResponse{
			Status:  "success",
			Message: "OAuth login successful",
			Token:   token,
			User:    user.ToDTOWithAccounts(accounts),
		}, nil
	}

	// 3. Register new user from OAuth info
	now := time.Now().UTC()
	cleanUsername := strings.ToLower(strings.TrimSpace(userInfo.Username))
	if !usernameRegex.MatchString(cleanUsername) {
		cleanUsername = fmt.Sprintf("user_%s", uuid.New().String()[:8])
	}

	// Ensure username uniqueness
	originalUsername := cleanUsername
	counter := 1
	for {
		_, err := s.userRepo.GetByUsername(ctx, cleanUsername)
		if errors.Is(err, repository.ErrUserNotFound) {
			break
		}
		cleanUsername = fmt.Sprintf("%s_%d", originalUsername[:min(len(originalUsername), 20)], counter)
		counter++
		if counter > 50 {
			cleanUsername = fmt.Sprintf("user_%s", uuid.New().String()[:8])
			break
		}
	}

	fullName := strings.TrimSpace(userInfo.FullName)
	if fullName == "" {
		fullName = cleanUsername
	}

	newUser := &model.User{
		ID:           uuid.New(),
		Email:        strings.ToLower(strings.TrimSpace(userInfo.Email)),
		Username:     cleanUsername,
		FullName:     fullName,
		PasswordHash: nil,
		AvatarURL:    avatarPtr,
		Provider:     userInfo.Provider,
		ProviderID:   &userInfo.ProviderID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create oauth user: %w", err)
	}

	// Also link in oauth accounts table
	oauthAcct := &model.OAuthAccount{
		ID:         uuid.New(),
		UserID:     newUser.ID,
		Provider:   userInfo.Provider,
		ProviderID: userInfo.ProviderID,
		Email:      &userInfo.Email,
		AvatarURL:  avatarPtr,
		CreatedAt:  now,
	}
	_ = s.userRepo.LinkOAuthAccount(ctx, oauthAcct)

	token, err := s.pasetoSvc.GenerateToken(newUser, s.tokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	accounts := s.getConnectedAccounts(ctx, newUser.ID)

	return &model.AuthResponse{
		Status:  "success",
		Message: "OAuth registration successful",
		Token:   token,
		User:    newUser.ToDTOWithAccounts(accounts),
	}, nil
}

func (s *authService) GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserDTO, error) {
	if s.userRepo == nil {
		return nil, errors.New("database repository unavailable")
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	accounts := s.getConnectedAccounts(ctx, userID)
	return user.ToDTOWithAccounts(accounts), nil
}

func (s *authService) UpdateProfile(ctx context.Context, userID uuid.UUID, input model.UpdateProfileInput) (*model.UserDTO, error) {
	if s.userRepo == nil {
		return nil, errors.New("database repository unavailable")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if input.FullName != nil && strings.TrimSpace(*input.FullName) != "" {
		trimmed := strings.TrimSpace(*input.FullName)
		if len(trimmed) < 2 || len(trimmed) > 100 {
			return nil, ErrFullNameRequired
		}
		user.FullName = trimmed
	}

	if input.Username != nil && strings.TrimSpace(*input.Username) != "" {
		cleanUsername := strings.ToLower(strings.TrimSpace(*input.Username))
		if cleanUsername != user.Username {
			if !usernameRegex.MatchString(cleanUsername) {
				return nil, ErrInvalidUsername
			}
			existing, err := s.userRepo.GetByUsername(ctx, cleanUsername)
			if err == nil && existing != nil && existing.ID != user.ID {
				return nil, ErrUsernameTaken
			}
			user.Username = cleanUsername
		}
	}

	if input.AvatarURL != nil {
		if strings.TrimSpace(*input.AvatarURL) == "" {
			user.AvatarURL = nil
		} else {
			trimmedURL := strings.TrimSpace(*input.AvatarURL)
			user.AvatarURL = &trimmedURL
		}
	}

	user.UpdatedAt = time.Now().UTC()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	accounts := s.getConnectedAccounts(ctx, userID)
	return user.ToDTOWithAccounts(accounts), nil
}

func (s *authService) SetPassword(ctx context.Context, userID uuid.UUID, input model.SetPasswordInput) error {
	if s.userRepo == nil {
		return errors.New("database repository unavailable")
	}

	if len(input.NewPassword) < 8 {
		return ErrPasswordTooShort
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// If user already has a password, verify current password
	if user.PasswordHash != nil && *user.PasswordHash != "" {
		if input.CurrentPassword == nil || *input.CurrentPassword == "" {
			return ErrCurrentPasswordReq
		}
		if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(*input.CurrentPassword)); err != nil {
			return ErrCurrentPasswordBad
		}
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return s.userRepo.UpdatePassword(ctx, userID, string(newHash))
}

func (s *authService) LinkOAuthAccount(ctx context.Context, userID uuid.UUID, userInfo *OAuthUserInfo) error {
	if s.userRepo == nil {
		return errors.New("database repository unavailable")
	}

	var avatarPtr *string
	if userInfo.AvatarURL != "" {
		avatarPtr = &userInfo.AvatarURL
	}

	oauthAcct := &model.OAuthAccount{
		ID:         uuid.New(),
		UserID:     userID,
		Provider:   userInfo.Provider,
		ProviderID: userInfo.ProviderID,
		Email:      &userInfo.Email,
		AvatarURL:  avatarPtr,
		CreatedAt:  time.Now().UTC(),
	}

	return s.userRepo.LinkOAuthAccount(ctx, oauthAcct)
}

func (s *authService) UnlinkOAuthAccount(ctx context.Context, userID uuid.UUID, provider string) error {
	if s.userRepo == nil {
		return errors.New("database repository unavailable")
	}

	return s.userRepo.UnlinkOAuthAccount(ctx, userID, provider)
}

func (s *authService) CheckAvailability(ctx context.Context, username, email string) (*model.CheckAvailabilityResponse, error) {
	resp := &model.CheckAvailabilityResponse{}

	if s.userRepo == nil {
		avail := true
		if username != "" {
			trimmedUsername := strings.TrimSpace(username)
			if !usernameRegex.MatchString(trimmedUsername) {
				avail = false
				resp.UsernameAvailable = &avail
				resp.UsernameReason = "Username must be 3-30 characters (alphanumeric and underscores only)"
			} else {
				resp.UsernameAvailable = &avail
			}
		}
		if email != "" {
			trimmedEmail := strings.TrimSpace(email)
			if _, err := mail.ParseAddress(trimmedEmail); err != nil || !strings.Contains(trimmedEmail, "@") {
				avail = false
				resp.EmailAvailable = &avail
				resp.EmailReason = "Invalid email format"
			} else {
				resp.EmailAvailable = &avail
			}
		}
		return resp, nil
	}

	if username != "" {
		trimmedUsername := strings.TrimSpace(username)
		if !usernameRegex.MatchString(trimmedUsername) {
			avail := false
			resp.UsernameAvailable = &avail
			resp.UsernameReason = "Username must be 3-30 characters (alphanumeric and underscores only)"
		} else {
			_, err := s.userRepo.GetByUsername(ctx, trimmedUsername)
			if errors.Is(err, repository.ErrUserNotFound) {
				avail := true
				resp.UsernameAvailable = &avail
			} else if err == nil {
				avail := false
				resp.UsernameAvailable = &avail
				resp.UsernameReason = "Username is already taken"
			} else {
				return nil, err
			}
		}
	}

	if email != "" {
		trimmedEmail := strings.TrimSpace(email)
		if _, err := mail.ParseAddress(trimmedEmail); err != nil || !strings.Contains(trimmedEmail, "@") {
			avail := false
			resp.EmailAvailable = &avail
			resp.EmailReason = "Invalid email format"
		} else {
			_, err := s.userRepo.GetByEmail(ctx, trimmedEmail)
			if errors.Is(err, repository.ErrUserNotFound) {
				avail := true
				resp.EmailAvailable = &avail
			} else if err == nil {
				avail := false
				resp.EmailAvailable = &avail
				resp.EmailReason = "Email is already registered"
			} else {
				return nil, err
			}
		}
	}

	return resp, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

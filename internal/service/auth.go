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
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)

type AuthService interface {
	Register(ctx context.Context, input model.RegisterInput) (*model.AuthResponse, error)
	Login(ctx context.Context, input model.LoginInput) (*model.AuthResponse, error)
	HandleOAuthLogin(ctx context.Context, userInfo *OAuthUserInfo) (*model.AuthResponse, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserDTO, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, input model.UpdateProfileInput) (*model.UserDTO, error)
	CheckAvailability(ctx context.Context, username, email string) (*model.CheckAvailabilityResponse, error)
}

type authService struct {
	userRepo   repository.UserRepository
	pasetoSvc  auth.PasetoService
	tokenTTL   time.Duration
}

func NewAuthService(userRepo repository.UserRepository, pasetoSvc auth.PasetoService) AuthService {
	return &authService{
		userRepo:  userRepo,
		pasetoSvc: pasetoSvc,
		tokenTTL:  7 * 24 * time.Hour,
	}
}

func (s *authService) Register(ctx context.Context, input model.RegisterInput) (*model.AuthResponse, error) {
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

	if s.userRepo == nil {
		return nil, errors.New("database user repository unavailable")
	}

	// 2. Check duplicates
	if existing, _ := s.userRepo.GetByEmail(ctx, email); existing != nil {
		return nil, repository.ErrUserAlreadyExists
	}
	if existing, _ := s.userRepo.GetByUsername(ctx, username); existing != nil {
		return nil, repository.ErrUserAlreadyExists
	}

	// 3. Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	hashStr := string(hashed)

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

	// 4. Issue PASETO token
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
	identifier := strings.TrimSpace(input.Identifier)
	if identifier == "" || input.Password == "" {
		return nil, ErrInvalidCredentials
	}

	if s.userRepo == nil {
		return nil, errors.New("database user repository unavailable")
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

	return &model.AuthResponse{
		Status:  "success",
		Message: "Login successful",
		Token:   token,
		User:    user.ToDTO(),
	}, nil
}

func (s *authService) HandleOAuthLogin(ctx context.Context, userInfo *OAuthUserInfo) (*model.AuthResponse, error) {
	if s.userRepo == nil {
		return nil, errors.New("database user repository unavailable")
	}

	// 1. Try finding by provider and provider ID
	user, err := s.userRepo.GetByProvider(ctx, userInfo.Provider, userInfo.ProviderID)
	if err == nil && user != nil {
		// Update avatar if provided
		if userInfo.AvatarURL != "" && (user.AvatarURL == nil || *user.AvatarURL == "") {
			user.AvatarURL = &userInfo.AvatarURL
			user.UpdatedAt = time.Now().UTC()
			_ = s.userRepo.Update(ctx, user)
		}

		token, err := s.pasetoSvc.GenerateToken(user, s.tokenTTL)
		if err != nil {
			return nil, err
		}
		return &model.AuthResponse{
			Status:  "success",
			Message: "OAuth login successful",
			Token:   token,
			User:    user.ToDTO(),
		}, nil
	}

	// 2. Try finding by email to link account
	user, err = s.userRepo.GetByEmail(ctx, userInfo.Email)
	if err == nil && user != nil {
		user.Provider = userInfo.Provider
		user.ProviderID = &userInfo.ProviderID
		if user.AvatarURL == nil || *user.AvatarURL == "" {
			user.AvatarURL = &userInfo.AvatarURL
		}
		user.UpdatedAt = time.Now().UTC()
		if err := s.userRepo.Update(ctx, user); err != nil {
			return nil, err
		}

		token, err := s.pasetoSvc.GenerateToken(user, s.tokenTTL)
		if err != nil {
			return nil, err
		}
		return &model.AuthResponse{
			Status:  "success",
			Message: "OAuth login successful",
			Token:   token,
			User:    user.ToDTO(),
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
	for i := 1; i <= 10; i++ {
		existing, _ := s.userRepo.GetByUsername(ctx, cleanUsername)
		if existing == nil {
			break
		}
		cleanUsername = fmt.Sprintf("%s_%d", originalUsername[:min(len(originalUsername), 20)], i)
	}

	var avatarPtr *string
	if userInfo.AvatarURL != "" {
		avatarPtr = &userInfo.AvatarURL
	}

	newUser := &model.User{
		ID:         uuid.New(),
		Email:      strings.ToLower(strings.TrimSpace(userInfo.Email)),
		Username:   cleanUsername,
		FullName:   userInfo.FullName,
		AvatarURL:  avatarPtr,
		Provider:   userInfo.Provider,
		ProviderID: &userInfo.ProviderID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create oauth user: %w", err)
	}

	token, err := s.pasetoSvc.GenerateToken(newUser, s.tokenTTL)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Status:  "success",
		Message: "OAuth registration successful",
		Token:   token,
		User:    newUser.ToDTO(),
	}, nil
}

func (s *authService) GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserDTO, error) {
	if s.userRepo == nil {
		return nil, errors.New("database user repository unavailable")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return user.ToDTO(), nil
}

func (s *authService) UpdateProfile(ctx context.Context, userID uuid.UUID, input model.UpdateProfileInput) (*model.UserDTO, error) {
	if s.userRepo == nil {
		return nil, errors.New("database user repository unavailable")
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

	if input.AvatarURL != nil {
		user.AvatarURL = input.AvatarURL
	}

	user.UpdatedAt = time.Now().UTC()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user.ToDTO(), nil
}

func (s *authService) CheckAvailability(ctx context.Context, username, email string) (*model.CheckAvailabilityResponse, error) {
	resp := &model.CheckAvailabilityResponse{}

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


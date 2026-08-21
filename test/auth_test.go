package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/handler"
	"github.com/irvanmalik48/realm-api/internal/middleware"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
)

type mockUserRepo struct {
	usersByID       map[uuid.UUID]*model.User
	usersByEmail    map[string]*model.User
	usersByUsername map[string]*model.User
	oauthAccounts   map[uuid.UUID][]model.OAuthAccount
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByID:       make(map[uuid.UUID]*model.User),
		usersByEmail:    make(map[string]*model.User),
		usersByUsername: make(map[string]*model.User),
		oauthAccounts:   make(map[uuid.UUID][]model.OAuthAccount),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error {
	email := strings.ToLower(user.Email)
	username := strings.ToLower(user.Username)

	if _, exists := m.usersByEmail[email]; exists {
		return repository.ErrUserAlreadyExists
	}
	if _, exists := m.usersByUsername[username]; exists {
		return repository.ErrUserAlreadyExists
	}

	m.usersByID[user.ID] = user
	m.usersByEmail[email] = user
	m.usersByUsername[username] = user

	if user.Provider != "" && user.Provider != "local" && user.ProviderID != nil {
		_ = m.LinkOAuthAccount(ctx, &model.OAuthAccount{
			ID:         uuid.New(),
			UserID:     user.ID,
			Provider:   user.Provider,
			ProviderID: *user.ProviderID,
			Email:      &user.Email,
			CreatedAt:  user.CreatedAt,
		})
	}
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	u, ok := m.usersByEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	u, ok := m.usersByUsername[strings.ToLower(strings.TrimSpace(username))]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByIdentifier(ctx context.Context, identifier string) (*model.User, error) {
	idLower := strings.ToLower(strings.TrimSpace(identifier))
	if u, ok := m.usersByEmail[idLower]; ok {
		return u, nil
	}
	if u, ok := m.usersByUsername[idLower]; ok {
		return u, nil
	}
	return nil, repository.ErrUserNotFound
}

func (m *mockUserRepo) GetByProvider(ctx context.Context, provider, providerID string) (*model.User, error) {
	return m.GetByOAuthAccount(ctx, provider, providerID)
}

func (m *mockUserRepo) GetByOAuthAccount(ctx context.Context, provider, providerID string) (*model.User, error) {
	for uid, accounts := range m.oauthAccounts {
		for _, a := range accounts {
			if a.Provider == provider && a.ProviderID == providerID {
				return m.usersByID[uid], nil
			}
		}
	}
	for _, u := range m.usersByID {
		if u.Provider == provider && u.ProviderID != nil && *u.ProviderID == providerID {
			return u, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

func (m *mockUserRepo) Update(ctx context.Context, user *model.User) error {
	m.usersByID[user.ID] = user
	m.usersByEmail[strings.ToLower(user.Email)] = user
	m.usersByUsername[strings.ToLower(user.Username)] = user
	return nil
}

func (m *mockUserRepo) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	u, ok := m.usersByID[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	u.PasswordHash = &passwordHash
	return nil
}

func (m *mockUserRepo) GetOAuthAccounts(ctx context.Context, userID uuid.UUID) ([]model.OAuthAccount, error) {
	return m.oauthAccounts[userID], nil
}

func (m *mockUserRepo) LinkOAuthAccount(ctx context.Context, account *model.OAuthAccount) error {
	for _, acct := range m.oauthAccounts[account.UserID] {
		if acct.Provider == account.Provider {
			return nil
		}
	}
	m.oauthAccounts[account.UserID] = append(m.oauthAccounts[account.UserID], *account)
	return nil
}

func (m *mockUserRepo) UnlinkOAuthAccount(ctx context.Context, userID uuid.UUID, provider string) error {
	user, ok := m.usersByID[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	hasPassword := user.PasswordHash != nil && *user.PasswordHash != ""
	accounts := m.oauthAccounts[userID]
	if !hasPassword && len(accounts) <= 1 {
		return repository.ErrCannotUnlinkLastAuth
	}

	var filtered []model.OAuthAccount
	for _, a := range accounts {
		if a.Provider != provider {
			filtered = append(filtered, a)
		}
	}
	m.oauthAccounts[userID] = filtered
	return nil
}

func setupAuthTestApp(t *testing.T) (*fiber.App, service.AuthService, auth.PasetoService) {
	pasetoSvc, err := auth.NewPasetoService("707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f")
	if err != nil {
		t.Fatalf("failed to create paseto service: %v", err)
	}

	repo := newMockUserRepo()
	authSvc := service.NewAuthService(repo, pasetoSvc)
	oauthSvc := service.NewOAuthService(&config.Config{})
	hdlr := handler.NewAuthHandler(&config.Config{}, authSvc, oauthSvc)

	app := fiber.New()
	v1 := app.Group("/v1/auth")
	v1.Get("/check", hdlr.CheckAvailability)
	v1.Post("/register", hdlr.Register)
	v1.Post("/login", hdlr.Login)
	v1.Get("/me", middleware.RequireUserAuth(pasetoSvc), hdlr.GetMe)
	v1.Patch("/profile", middleware.RequireUserAuth(pasetoSvc), hdlr.UpdateProfile)
	v1.Post("/password", middleware.RequireUserAuth(pasetoSvc), hdlr.SetPassword)
	v1.Delete("/oauth/:provider", middleware.RequireUserAuth(pasetoSvc), hdlr.UnlinkOAuth)

	return app, authSvc, pasetoSvc
}

func TestAuth_RegisterAndLoginFlow(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	// 1. Register new user
	regPayload := model.RegisterInput{
		Email:    "testuser@example.com",
		Username: "testuser",
		Password: "SecurePassword123!",
		FullName: "Test User",
	}
	body, _ := json.Marshal(regPayload)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("register request error: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 201 Created, got %d: %s", resp.StatusCode, string(respBody))
	}

	var authResp model.AuthResponse
	respBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBytes, &authResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if authResp.Token == "" {
		t.Errorf("expected non-empty PASETO token in registration response")
	}
	if authResp.User == nil || authResp.User.Email != "testuser@example.com" {
		t.Errorf("expected user email 'testuser@example.com', got %+v", authResp.User)
	}
	if authResp.User.FullName != "Test User" {
		t.Errorf("expected FullName 'Test User', got %s", authResp.User.FullName)
	}

	token := authResp.Token

	// 2. Duplicate registration should fail (409 Conflict)
	reqDup := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	reqDup.Header.Set("Content-Type", "application/json")
	respDup, _ := app.Test(reqDup, -1)
	if respDup.StatusCode != http.StatusConflict {
		t.Errorf("expected status 409 Conflict on duplicate registration, got %d", respDup.StatusCode)
	}

	// 3. Login with email
	loginWithEmail := model.LoginInput{
		Identifier: "testuser@example.com",
		Password:   "SecurePassword123!",
	}
	bodyEmail, _ := json.Marshal(loginWithEmail)
	reqLogin1 := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(bodyEmail))
	reqLogin1.Header.Set("Content-Type", "application/json")

	respLogin1, _ := app.Test(reqLogin1, -1)
	if respLogin1.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for email login, got %d", respLogin1.StatusCode)
	}

	// 4. Login with username
	loginWithUsername := model.LoginInput{
		Identifier: "testuser",
		Password:   "SecurePassword123!",
	}
	bodyUsername, _ := json.Marshal(loginWithUsername)
	reqLogin2 := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(bodyUsername))
	reqLogin2.Header.Set("Content-Type", "application/json")

	respLogin2, _ := app.Test(reqLogin2, -1)
	if respLogin2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for username login, got %d", respLogin2.StatusCode)
	}

	// 5. Login with invalid password
	loginInvalid := model.LoginInput{
		Identifier: "testuser",
		Password:   "WrongPassword",
	}
	bodyInvalid, _ := json.Marshal(loginInvalid)
	reqLoginInvalid := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(bodyInvalid))
	reqLoginInvalid.Header.Set("Content-Type", "application/json")

	respLoginInvalid, _ := app.Test(reqLoginInvalid, -1)
	if respLoginInvalid.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for wrong password, got %d", respLoginInvalid.StatusCode)
	}

	// 6. Get Profile /v1/auth/me with Bearer token
	reqMe := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+token)

	respMe, _ := app.Test(reqMe, -1)
	if respMe.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /v1/auth/me, got %d", respMe.StatusCode)
	}

	// 7. Update profile /v1/auth/profile
	newFullName := "Updated Full Name"
	newAvatar := "https://example.com/avatar.png"
	updatePayload := model.UpdateProfileInput{
		FullName:  &newFullName,
		AvatarURL: &newAvatar,
	}
	bodyUpdate, _ := json.Marshal(updatePayload)
	reqUpdate := httptest.NewRequest(http.MethodPatch, "/v1/auth/profile", bytes.NewReader(bodyUpdate))
	reqUpdate.Header.Set("Authorization", "Bearer "+token)
	reqUpdate.Header.Set("Content-Type", "application/json")

	respUpdate, _ := app.Test(reqUpdate, -1)
	if respUpdate.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /v1/auth/profile, got %d", respUpdate.StatusCode)
	}

	var updateResp struct {
		User model.UserDTO `json:"user"`
	}
	respUpdateBytes, _ := io.ReadAll(respUpdate.Body)
	_ = json.Unmarshal(respUpdateBytes, &updateResp)
	if updateResp.User.FullName != "Updated Full Name" {
		t.Errorf("expected updated FullName, got %s", updateResp.User.FullName)
	}
	if updateResp.User.AvatarURL == nil || *updateResp.User.AvatarURL != newAvatar {
		t.Errorf("expected updated AvatarURL, got %v", updateResp.User.AvatarURL)
	}
}

func TestAuth_PASETOExpirationAndTamper(t *testing.T) {
	pasetoSvc, _ := auth.NewPasetoService("707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f")

	testUser := &model.User{
		ID:        uuid.New(),
		Email:     "paseto@example.com",
		Username:  "paseto_user",
		FullName:  "Paseto User",
		Provider:  "local",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// 1. Generate short-lived token (expired)
	expiredToken, err := pasetoSvc.GenerateToken(testUser, -1*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = pasetoSvc.VerifyToken(expiredToken)
	if err == nil {
		t.Errorf("expected error verifying expired token, got nil")
	}

	// 2. Tampered token
	tamperedToken := expiredToken[:len(expiredToken)-5] + "XXXXX"
	_, err = pasetoSvc.VerifyToken(tamperedToken)
	if err == nil {
		t.Errorf("expected error verifying tampered token, got nil")
	}
}

func TestAuth_OAuthFindOrCreate(t *testing.T) {
	repo := newMockUserRepo()
	pasetoSvc, _ := auth.NewPasetoService("707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f")
	authSvc := service.NewAuthService(repo, pasetoSvc)

	googleUser := &service.OAuthUserInfo{
		Provider:   "google",
		ProviderID: "109876543210987654321",
		Email:      "oauthuser@gmail.com",
		Username:   "oauthuser",
		FullName:   "Google User",
		AvatarURL:  "https://lh3.googleusercontent.com/a/photo",
	}

	// 1. First OAuth login creates user
	resp1, err := authSvc.HandleOAuthLogin(context.Background(), googleUser)
	if err != nil {
		t.Fatalf("failed first oauth login: %v", err)
	}

	if resp1.Token == "" {
		t.Errorf("expected token from oauth login")
	}
	if resp1.User.Email != "oauthuser@gmail.com" {
		t.Errorf("expected email 'oauthuser@gmail.com', got %s", resp1.User.Email)
	}
	if resp1.User.Provider != "google" {
		t.Errorf("expected provider 'google', got %s", resp1.User.Provider)
	}

	// 2. Second OAuth login finds existing user and issues fresh token
	resp2, err := authSvc.HandleOAuthLogin(context.Background(), googleUser)
	if err != nil {
		t.Fatalf("failed second oauth login: %v", err)
	}

	if resp2.User.ID != resp1.User.ID {
		t.Errorf("expected identical user ID for subsequent oauth login, got %s vs %s", resp2.User.ID, resp1.User.ID)
	}
}

func TestAuth_CheckAvailability(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	// 1. Initial check on free username and email -> available: true
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/check?username=freshuser&email=fresh@example.com", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("check request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var availResp model.CheckAvailabilityResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &availResp)
	if availResp.UsernameAvailable == nil || !*availResp.UsernameAvailable {
		t.Errorf("expected username freshuser to be available")
	}
	if availResp.EmailAvailable == nil || !*availResp.EmailAvailable {
		t.Errorf("expected email fresh@example.com to be available")
	}

	// 2. Register freshuser
	regBody, _ := json.Marshal(model.RegisterInput{
		Email:    "fresh@example.com",
		Username: "freshuser",
		Password: "Password123!",
		FullName: "Fresh User",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := app.Test(regReq)
	if err != nil || regResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to register user: %v, code: %d", err, regResp.StatusCode)
	}

	// 3. Re-check now taken username and email -> available: false
	req2 := httptest.NewRequest(http.MethodGet, "/v1/auth/check?username=freshuser&email=fresh@example.com", nil)
	resp2, _ := app.Test(req2)
	var availResp2 model.CheckAvailabilityResponse
	body2, _ := io.ReadAll(resp2.Body)
	_ = json.Unmarshal(body2, &availResp2)
	if availResp2.UsernameAvailable == nil || *availResp2.UsernameAvailable {
		t.Errorf("expected username freshuser to be unavailable after registration")
	}
	if availResp2.EmailAvailable == nil || *availResp2.EmailAvailable {
		t.Errorf("expected email fresh@example.com to be unavailable after registration")
	}

	// 4. Invalid username format (e.g. too short)
	req3 := httptest.NewRequest(http.MethodGet, "/v1/auth/check?username=ab", nil)
	resp3, _ := app.Test(req3)
	var availResp3 model.CheckAvailabilityResponse
	body3, _ := io.ReadAll(resp3.Body)
	_ = json.Unmarshal(body3, &availResp3)
	if availResp3.UsernameAvailable == nil || *availResp3.UsernameAvailable {
		t.Errorf("expected username 'ab' to be marked unavailable due to invalid format")
	}
}

func TestAuth_SetPassword(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	// 1. Register user
	regBody, _ := json.Marshal(model.RegisterInput{
		Email:    "oauthpass@example.com",
		Username: "oauthpass",
		Password: "InitialPassword123!",
		FullName: "OAuth User",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, _ := app.Test(regReq)
	var regAuthResp model.AuthResponse
	body, _ := io.ReadAll(regResp.Body)
	_ = json.Unmarshal(body, &regAuthResp)

	// 2. Change password with wrong current password -> 400
	wrongPwd := "WrongCurrentPass123!"
	pwdBody, _ := json.Marshal(model.SetPasswordInput{
		CurrentPassword: &wrongPwd,
		NewPassword:     "NewSecurePassword123!",
	})
	pwdReq := httptest.NewRequest(http.MethodPost, "/v1/auth/password", bytes.NewReader(pwdBody))
	pwdReq.Header.Set("Content-Type", "application/json")
	pwdReq.Header.Set("Authorization", "Bearer "+regAuthResp.Token)
	pwdResp, _ := app.Test(pwdReq)
	if pwdResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for incorrect current password, got %d", pwdResp.StatusCode)
	}

	// 3. Change password with correct current password -> 200
	correctPwd := "InitialPassword123!"
	pwdBody2, _ := json.Marshal(model.SetPasswordInput{
		CurrentPassword: &correctPwd,
		NewPassword:     "NewSecurePassword123!",
	})
	pwdReq2 := httptest.NewRequest(http.MethodPost, "/v1/auth/password", bytes.NewReader(pwdBody2))
	pwdReq2.Header.Set("Content-Type", "application/json")
	pwdReq2.Header.Set("Authorization", "Bearer "+regAuthResp.Token)
	pwdResp2, _ := app.Test(pwdReq2)
	if pwdResp2.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for valid password update, got %d", pwdResp2.StatusCode)
	}
}

func TestAuth_OAuthLinkingAndUnlinking(t *testing.T) {
	repo := newMockUserRepo()
	pasetoSvc, _ := auth.NewPasetoService("707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f")
	authSvc := service.NewAuthService(repo, pasetoSvc)
	ctx := context.Background()

	// 1. Create user via Google OAuth
	googleUser := &service.OAuthUserInfo{
		Provider:   "google",
		ProviderID: "g-1111",
		Email:      "multiauth@example.com",
		Username:   "multiauth",
		FullName:   "Multi Auth User",
	}
	resp, err := authSvc.HandleOAuthLogin(ctx, googleUser)
	if err != nil {
		t.Fatalf("oauth login failed: %v", err)
	}
	userID := resp.User.ID

	// 2. Link GitHub account to the same user
	githubUser := &service.OAuthUserInfo{
		Provider:   "github",
		ProviderID: "gh-2222",
		Email:      "multiauth@example.com",
		Username:   "multiauth",
	}
	err = authSvc.LinkOAuthAccount(ctx, userID, githubUser)
	if err != nil {
		t.Fatalf("failed to link github account: %v", err)
	}

	// 3. Verify user profile returns both connected providers
	profile, err := authSvc.GetProfile(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get profile: %v", err)
	}
	if len(profile.ConnectedProviders) < 2 {
		t.Errorf("expected at least 2 connected providers, got %v", profile.ConnectedProviders)
	}

	// 4. Set local password on the OAuth user
	err = authSvc.SetPassword(ctx, userID, model.SetPasswordInput{
		NewPassword: "BrandNewPassword123!",
	})
	if err != nil {
		t.Fatalf("failed to set password: %v", err)
	}

	// 5. Verify user can now log in using the newly set password
	loginResp, err := authSvc.Login(ctx, model.LoginInput{
		Identifier: "multiauth",
		Password:   "BrandNewPassword123!",
	})
	if err != nil {
		t.Fatalf("failed to login with newly set password: %v", err)
	}
	if loginResp.User.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, loginResp.User.ID)
	}
	if !loginResp.User.HasPassword {
		t.Errorf("expected HasPassword to be true")
	}

	// 6. Unlink Google account (allowed because user has password and GitHub)
	err = authSvc.UnlinkOAuthAccount(ctx, userID, "google")
	if err != nil {
		t.Fatalf("failed to unlink google account: %v", err)
	}
}



package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
	"strings"
)

func getUserIDFromLocals(c *fiber.Ctx) (uuid.UUID, bool) {
	val := c.Locals("user_id")
	if id, ok := val.(uuid.UUID); ok {
		return id, true
	}
	return uuid.Nil, false
}

type AuthHandler struct {
	cfg      *config.Config
	authSvc  service.AuthService
	oauthSvc service.OAuthService
}

func NewAuthHandler(cfg *config.Config, authSvc service.AuthService, oauthSvc service.OAuthService) *AuthHandler {
	return &AuthHandler{
		cfg:      cfg,
		authSvc:  authSvc,
		oauthSvc: oauthSvc,
	}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var input model.RegisterInput
	if err := c.BodyParser(&input); err != nil {
		return ErrorResponse(c, "Invalid request body. Expected JSON.", http.StatusBadRequest)
	}

	resp, err := h.authSvc.Register(c.Context(), input)
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return ErrorResponse(c, "An account with this email or username already exists.", http.StatusConflict)
		}
		if errors.Is(err, service.ErrInvalidEmail) || errors.Is(err, service.ErrInvalidUsername) ||
			errors.Is(err, service.ErrPasswordTooShort) || errors.Is(err, service.ErrFullNameRequired) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		return ErrorResponse(c, fmt.Sprintf("Registration failed: %v", err), http.StatusInternalServerError)
	}

	return c.Status(http.StatusCreated).JSON(resp)
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var input model.LoginInput
	if err := c.BodyParser(&input); err != nil {
		return ErrorResponse(c, "Invalid request body. Expected JSON.", http.StatusBadRequest)
	}

	resp, err := h.authSvc.Login(c.Context(), input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return ErrorResponse(c, "Invalid email/username or password.", http.StatusUnauthorized)
		}
		return ErrorResponse(c, err.Error(), http.StatusBadRequest)
	}

	return c.Status(http.StatusOK).JSON(resp)
}

func (h *AuthHandler) GetMe(c *fiber.Ctx) error {
	userID, ok := getUserIDFromLocals(c)
	if !ok {
		return ErrorResponse(c, "Unauthorized: missing user context", http.StatusUnauthorized)
	}

	userDTO, err := h.authSvc.GetProfile(c.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrorResponse(c, "User record not found", http.StatusNotFound)
		}
		return ErrorResponse(c, fmt.Sprintf("Failed to retrieve profile: %v", err), http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"status": "success",
		"user":   userDTO,
	})
}

func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	userID, ok := getUserIDFromLocals(c)
	if !ok {
		return ErrorResponse(c, "Unauthorized: missing user context", http.StatusUnauthorized)
	}

	var input model.UpdateProfileInput
	if err := c.BodyParser(&input); err != nil {
		return ErrorResponse(c, "Invalid request body. Expected JSON.", http.StatusBadRequest)
	}

	userDTO, err := h.authSvc.UpdateProfile(c.Context(), userID, input)
	if err != nil {
		if errors.Is(err, service.ErrFullNameRequired) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		return ErrorResponse(c, fmt.Sprintf("Failed to update profile: %v", err), http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Profile updated successfully",
		"user":    userDTO,
	})
}

func (h *AuthHandler) GoogleLogin(c *fiber.Ctx) error {
	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state_google",
		Value:    state,
		MaxAge:   300,
		HTTPOnly: true,
		Secure:   h.cfg.Environment == "production",
		SameSite: "Lax",
	})

	url := h.oauthSvc.GetGoogleAuthURL(state)
	if url == "" {
		return ErrorResponse(c, "Google OAuth is not configured on this server", http.StatusNotImplemented)
	}

	return c.Redirect(url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) GoogleCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")
	savedState := c.Cookies("oauth_state_google")

	if code == "" || state == "" || savedState == "" || state != savedState {
		return ErrorResponse(c, "Invalid OAuth state or authorization code", http.StatusBadRequest)
	}

	userInfo, err := h.oauthSvc.HandleGoogleCallback(c.Context(), code)
	if err != nil {
		return ErrorResponse(c, fmt.Sprintf("Google authentication failed: %v", err), http.StatusBadRequest)
	}

	authResp, err := h.authSvc.HandleOAuthLogin(c.Context(), userInfo)
	if err != nil {
		return ErrorResponse(c, fmt.Sprintf("Account creation/linking failed: %v", err), http.StatusInternalServerError)
	}

	frontendURL := h.cfg.FrontendURL
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	redirectURI := fmt.Sprintf("%s/api/auth/callback?token=%s", frontendURL, url.QueryEscape(authResp.Token))
	return c.Redirect(redirectURI, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) GitHubLogin(c *fiber.Ctx) error {
	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state_github",
		Value:    state,
		MaxAge:   300,
		HTTPOnly: true,
		Secure:   h.cfg.Environment == "production",
		SameSite: "Lax",
	})

	url := h.oauthSvc.GetGitHubAuthURL(state)
	if url == "" {
		return ErrorResponse(c, "GitHub OAuth is not configured on this server", http.StatusNotImplemented)
	}

	return c.Redirect(url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) GitHubCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")
	savedState := c.Cookies("oauth_state_github")

	if code == "" || state == "" || savedState == "" || state != savedState {
		return ErrorResponse(c, "Invalid OAuth state or authorization code", http.StatusBadRequest)
	}

	userInfo, err := h.oauthSvc.HandleGitHubCallback(c.Context(), code)
	if err != nil {
		return ErrorResponse(c, fmt.Sprintf("GitHub authentication failed: %v", err), http.StatusBadRequest)
	}

	authResp, err := h.authSvc.HandleOAuthLogin(c.Context(), userInfo)
	if err != nil {
		return ErrorResponse(c, fmt.Sprintf("Account creation/linking failed: %v", err), http.StatusInternalServerError)
	}

	frontendURL := h.cfg.FrontendURL
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	redirectURI := fmt.Sprintf("%s/api/auth/callback?token=%s", frontendURL, url.QueryEscape(authResp.Token))
	return c.Redirect(redirectURI, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) CheckAvailability(c *fiber.Ctx) error {
	username := c.Query("username")
	email := c.Query("email")

	if username == "" && email == "" {
		return ErrorResponse(c, "Please provide 'username' or 'email' query parameter", http.StatusBadRequest)
	}

	resp, err := h.authSvc.CheckAvailability(c.Context(), username, email)
	if err != nil {
		return ErrorResponse(c, fmt.Sprintf("Failed to check availability: %v", err), http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(resp)
}

func (h *AuthHandler) SetPassword(c *fiber.Ctx) error {
	userID, ok := getUserIDFromLocals(c)
	if !ok {
		return ErrorResponse(c, "Unauthorized", http.StatusUnauthorized)
	}

	var input model.SetPasswordInput
	if err := c.BodyParser(&input); err != nil {
		return ErrorResponse(c, "Invalid request body. Expected JSON.", http.StatusBadRequest)
	}

	if err := h.authSvc.SetPassword(c.Context(), userID, input); err != nil {
		if errors.Is(err, service.ErrPasswordTooShort) || errors.Is(err, service.ErrCurrentPasswordReq) || errors.Is(err, service.ErrCurrentPasswordBad) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		return ErrorResponse(c, fmt.Sprintf("Failed to set password: %v", err), http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Password updated successfully",
	})
}

func (h *AuthHandler) UnlinkOAuth(c *fiber.Ctx) error {
	userID, ok := getUserIDFromLocals(c)
	if !ok {
		return ErrorResponse(c, "Unauthorized", http.StatusUnauthorized)
	}

	provider := strings.ToLower(c.Params("provider"))
	if provider != "google" && provider != "github" {
		return ErrorResponse(c, "Unsupported OAuth provider. Allowed: google, github", http.StatusBadRequest)
	}

	if err := h.authSvc.UnlinkOAuthAccount(c.Context(), userID, provider); err != nil {
		if errors.Is(err, repository.ErrCannotUnlinkLastAuth) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		return ErrorResponse(c, fmt.Sprintf("Failed to unlink account: %v", err), http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("%s account unlinked successfully", provider),
	})
}



package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"github.com/irvanmalik48/realm-api/internal/config"
)

var (
	ErrOAuthExchangeFailed = errors.New("failed to exchange authorization code with provider")
	ErrOAuthUserInfoFailed = errors.New("failed to fetch user information from provider")
	ErrOAuthEmailNotFound  = errors.New("no verified email found in provider account")
)

type OAuthUserInfo struct {
	Provider   string
	ProviderID string
	Email      string
	Username   string
	FullName   string
	AvatarURL  string
}

type OAuthService interface {
	GetGoogleAuthURL(state string) string
	GetGitHubAuthURL(state string) string
	HandleGoogleCallback(ctx context.Context, code string) (*OAuthUserInfo, error)
	HandleGitHubCallback(ctx context.Context, code string) (*OAuthUserInfo, error)
}

type oauthService struct {
	cfg          *config.Config
	googleConfig *oauth2.Config
	githubConfig *oauth2.Config
	httpClient   *http.Client
}

func NewOAuthService(cfg *config.Config) OAuthService {
	var gConfig *oauth2.Config
	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		gConfig = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes: []string{
				"openid",
				"https://www.googleapis.com/auth/userinfo.profile",
				"https://www.googleapis.com/auth/userinfo.email",
			},
			Endpoint: google.Endpoint,
		}
	}

	var ghConfig *oauth2.Config
	if cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "" {
		ghConfig = &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			RedirectURL:  cfg.GitHubRedirectURL,
			Scopes: []string{
				"read:user",
				"user:email",
			},
			Endpoint: github.Endpoint,
		}
	}

	return &oauthService{
		cfg:          cfg,
		googleConfig: gConfig,
		githubConfig: ghConfig,
		httpClient:   &http.Client{},
	}
}

func (s *oauthService) GetGoogleAuthURL(state string) string {
	if s.googleConfig == nil {
		return ""
	}
	return s.googleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *oauthService) GetGitHubAuthURL(state string) string {
	if s.githubConfig == nil {
		return ""
	}
	return s.githubConfig.AuthCodeURL(state)
}

func (s *oauthService) HandleGoogleCallback(ctx context.Context, code string) (*OAuthUserInfo, error) {
	if s.googleConfig == nil {
		return nil, errors.New("google oauth not configured")
	}

	token, err := s.googleConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOAuthExchangeFailed, err)
	}

	client := s.googleConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOAuthUserInfoFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrOAuthUserInfoFailed, resp.StatusCode)
	}

	var gUser struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gUser); err != nil {
		return nil, fmt.Errorf("failed to decode google user info: %w", err)
	}

	if gUser.Email == "" {
		return nil, ErrOAuthEmailNotFound
	}

	// Derive default username from email prefix
	username := strings.Split(gUser.Email, "@")[0]
	username = strings.ToLower(username)
	if len(username) < 3 {
		username = "user_" + gUser.Sub[:6]
	}

	fullName := gUser.Name
	if fullName == "" {
		fullName = username
	}

	return &OAuthUserInfo{
		Provider:   "google",
		ProviderID: gUser.Sub,
		Email:      gUser.Email,
		Username:   username,
		FullName:   fullName,
		AvatarURL:  gUser.Picture,
	}, nil
}

func (s *oauthService) HandleGitHubCallback(ctx context.Context, code string) (*OAuthUserInfo, error) {
	if s.githubConfig == nil {
		return nil, errors.New("github oauth not configured")
	}

	token, err := s.githubConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOAuthExchangeFailed, err)
	}

	client := s.githubConfig.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOAuthUserInfoFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrOAuthUserInfoFailed, resp.StatusCode)
	}

	var ghUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("failed to decode github user: %w", err)
	}

	email := ghUser.Email
	if email == "" {
		// Fetch primary verified email from /user/emails
		emailsResp, err := client.Get("https://api.github.com/user/emails")
		if err == nil && emailsResp.StatusCode == http.StatusOK {
			defer emailsResp.Body.Close()
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			bodyBytes, _ := io.ReadAll(emailsResp.Body)
			if err := json.Unmarshal(bodyBytes, &emails); err == nil {
				for _, e := range emails {
					if e.Primary && e.Verified {
						email = e.Email
						break
					}
				}
				if email == "" && len(emails) > 0 {
					email = emails[0].Email
				}
			}
		}
	}

	if email == "" {
		return nil, ErrOAuthEmailNotFound
	}

	fullName := ghUser.Name
	if fullName == "" {
		fullName = ghUser.Login
	}

	return &OAuthUserInfo{
		Provider:   "github",
		ProviderID: strconv.FormatInt(ghUser.ID, 10),
		Email:      email,
		Username:   ghUser.Login,
		FullName:   fullName,
		AvatarURL:  ghUser.AvatarURL,
	}, nil
}

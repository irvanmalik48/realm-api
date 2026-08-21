package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                   string
	Environment            string
	AllowedOrigins         string
	LastFMAPIKey           string
	LastFMAPISecret        string
	CacheRevalidateSeconds int

	// Database Settings
	DatabaseURL string

	// Storage Settings
	StorageDir      string
	MaxUploadSizeMB int

	// Authentication & PASETO Settings
	PASETOSymmetricKey string
	FrontendURL        string

	// Google OIDC / OAuth2 Settings
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// GitHub OAuth2 Settings
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string

	// Contact Form Integration Settings
	DiscordWebhookURL    string
	TelegramBotToken     string
	TelegramChatID       string
	ContactReceiverEmail string
	SMTPHost             string
	SMTPPort             int
	SMTPUser             string
	SMTPPass             string
}

func Load() *Config {
	// Attempt to load .env file if present, ignoring error if not found
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	env := getEnv("ENVIRONMENT", "development")
	allowedOrigins := getEnv("ALLOWED_ORIGINS", "https://irvanma.eu.org")
	lastfmKey := getEnv("LASTFM_API_KEY", "")
	lastfmSecret := getEnv("LASTFM_API_SECRET", "")

	cacheRevalidateStr := getEnv("CACHE_REVALIDATE_SECONDS", "900")
	cacheRevalidate, err := strconv.Atoi(cacheRevalidateStr)
	if err != nil || cacheRevalidate <= 0 {
		cacheRevalidate = 900
	}

	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		dbHost := getEnv("DB_HOST", "localhost")
		dbPort := getEnv("DB_PORT", "5432")
		dbUser := getEnv("DB_USER", "postgres")
		dbPass := getEnv("DB_PASSWORD", "postgres")
		dbName := getEnv("DB_NAME", "realm")
		dbSSL := getEnv("DB_SSLMODE", "disable")
		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", dbUser, dbPass, dbHost, dbPort, dbName, dbSSL)
	}

	storageDir := getEnv("STORAGE_DIR", "./data/storage")
	maxUploadMBStr := getEnv("MAX_UPLOAD_SIZE_MB", "10")
	maxUploadMB, err := strconv.Atoi(maxUploadMBStr)
	if err != nil || maxUploadMB <= 0 {
		maxUploadMB = 10
	}

	smtpPortStr := getEnv("SMTP_PORT", "587")
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		smtpPort = 587
	}

	return &Config{
		Port:                   port,
		Environment:            env,
		AllowedOrigins:         allowedOrigins,
		LastFMAPIKey:           lastfmKey,
		LastFMAPISecret:        lastfmSecret,
		CacheRevalidateSeconds: cacheRevalidate,

		DatabaseURL: databaseURL,

		StorageDir:      storageDir,
		MaxUploadSizeMB: maxUploadMB,

		PASETOSymmetricKey: getEnv("PASETO_SYMMETRIC_KEY", "707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:3000"),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/v1/auth/google/callback"),

		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubRedirectURL:  getEnv("GITHUB_REDIRECT_URL", "http://localhost:8080/v1/auth/github/callback"),

		DiscordWebhookURL:    getEnv("DISCORD_WEBHOOK_URL", ""),
		TelegramBotToken:     getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:       getEnv("TELEGRAM_CHAT_ID", ""),
		ContactReceiverEmail: getEnv("CONTACT_RECEIVER_EMAIL", ""),
		SMTPHost:             getEnv("SMTP_HOST", ""),
		SMTPPort:             smtpPort,
		SMTPUser:             getEnv("SMTP_USER", ""),
		SMTPPass:             getEnv("SMTP_PASS", ""),
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	GitHubClientID         string
	GitHubClientSecret     string
	Environment            string
	AllowedOrigins         string
	LastFMAPIKey           string
	LastFMAPISecret        string
	SMTPPass               string
	DatabaseURL            string
	SMTPUser               string
	StorageBackend         string
	StorageDir             string
	SMTPHost               string
	S3Endpoint             string
	S3Region               string
	S3Bucket               string
	S3AccessKeyID          string
	GRPCPort               string
	ContactReceiverEmail   string
	S3PublicDomain         string
	PASETOSymmetricKey     string
	FrontendURL            string
	GoogleClientID         string
	GoogleClientSecret     string
	GoogleRedirectURL      string
	Port                   string
	S3SecretAccessKey      string
	GitHubRedirectURL      string
	DiscordWebhookURL      string
	TelegramBotToken       string
	TelegramChatID         string
	SuperadminEmails       []string
	MaxUploadSizeMB        int
	SMTPPort               int
	CacheRevalidateSeconds int
	S3UsePathStyle         bool
}

func Load() *Config {
	// Attempt to load .env file if present, ignoring error if not found
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	grpcPort := getEnv("GRPC_PORT", "50051")
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
	maxUploadMBStr := getEnv("MAX_UPLOAD_SIZE_MB", "50")
	maxUploadMB, err := strconv.Atoi(maxUploadMBStr)
	if err != nil || maxUploadMB <= 0 {
		maxUploadMB = 50
	}

	smtpPortStr := getEnv("SMTP_PORT", "587")
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		smtpPort = 587
	}

	const DefaultDevPASETOKey = "707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f"

	pasetoKey := getEnv("PASETO_SYMMETRIC_KEY", "")
	if env == "production" {
		if pasetoKey == "" || pasetoKey == DefaultDevPASETOKey {
			panic("FATAL: PASETO_SYMMETRIC_KEY must be explicitly set to a secure 32-byte hex secret in production environment")
		}
	} else if pasetoKey == "" {
		pasetoKey = DefaultDevPASETOKey
	}

	var superadminEmails []string
	if rawEmails := getEnv("SUPERADMIN_EMAILS", ""); rawEmails != "" {
		for _, e := range strings.Split(rawEmails, ",") {
			if trimmed := strings.ToLower(strings.TrimSpace(e)); trimmed != "" {
				superadminEmails = append(superadminEmails, trimmed)
			}
		}
	}

	storageBackend := getEnv("STORAGE_BACKEND", "local")
	s3UsePathStyle := getEnv("S3_USE_PATH_STYLE", "false") == "true"

	return &Config{
		Port:                   port,
		GRPCPort:               grpcPort,
		Environment:            env,
		AllowedOrigins:         allowedOrigins,
		LastFMAPIKey:           lastfmKey,
		LastFMAPISecret:        lastfmSecret,
		CacheRevalidateSeconds: cacheRevalidate,

		DatabaseURL: databaseURL,

		SuperadminEmails:  superadminEmails,
		StorageBackend:    storageBackend,
		StorageDir:        storageDir,
		MaxUploadSizeMB:   maxUploadMB,
		S3Endpoint:        getEnv("S3_ENDPOINT", ""),
		S3Region:          getEnv("S3_REGION", "auto"),
		S3Bucket:          getEnv("S3_BUCKET", ""),
		S3AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", ""),
		S3SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", ""),
		S3UsePathStyle:    s3UsePathStyle,
		S3PublicDomain:    getEnv("S3_PUBLIC_DOMAIN", ""),

		PASETOSymmetricKey: pasetoKey,
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

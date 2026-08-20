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

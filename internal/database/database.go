package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	// Verify connection with ping
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	db := &DB{Pool: pool}

	// Initialize tables
	if err := db.migrate(ctx); err != nil {
		return nil, fmt.Errorf("failed to run database migration: %w", err)
	}

	log.Println("Database connection pool established successfully.")
	return db, nil
}

func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

func (db *DB) migrate(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS contact_submissions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(100) NOT NULL,
		email VARCHAR(255) NOT NULL,
		subject VARCHAR(200) NOT NULL,
		message TEXT NOT NULL,
		ip_address VARCHAR(45),
		user_agent TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_contact_submissions_created_at ON contact_submissions(created_at DESC);

	CREATE TABLE IF NOT EXISTS files (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		filename VARCHAR(255) NOT NULL,
		content_type VARCHAR(100) NOT NULL,
		original_size BIGINT NOT NULL,
		compressed_size BIGINT NOT NULL,
		compression_algorithm VARCHAR(20) NOT NULL DEFAULT 'zstd',
		sha256 VARCHAR(64) NOT NULL,
		blurhash VARCHAR(100),
		width INT,
		height INT,
		is_public BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_files_sha256 ON files(sha256);

	CREATE TABLE IF NOT EXISTS api_tokens (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(100) NOT NULL,
		token_prefix VARCHAR(50) NOT NULL,
		token_hash VARCHAR(64) NOT NULL UNIQUE,
		scopes TEXT[] NOT NULL DEFAULT '{"*"}',
		rate_limit_rpm INT NOT NULL DEFAULT 60,
		last_used_at TIMESTAMP WITH TIME ZONE,
		expires_at TIMESTAMP WITH TIME ZONE,
		is_revoked BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	ALTER TABLE api_tokens ALTER COLUMN token_prefix TYPE VARCHAR(50);
	CREATE INDEX IF NOT EXISTS idx_api_tokens_token_hash ON api_tokens(token_hash);
	CREATE INDEX IF NOT EXISTS idx_api_tokens_is_revoked ON api_tokens(is_revoked);

	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email VARCHAR(255) NOT NULL UNIQUE,
		username VARCHAR(50) NOT NULL UNIQUE,
		full_name VARCHAR(100) NOT NULL,
		password_hash VARCHAR(255),
		avatar_url TEXT,
		provider VARCHAR(30) NOT NULL DEFAULT 'local',
		provider_id VARCHAR(255),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_provider ON users(provider, provider_id);

	CREATE TABLE IF NOT EXISTS user_oauth_accounts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		provider VARCHAR(30) NOT NULL,
		provider_id VARCHAR(255) NOT NULL,
		email VARCHAR(255),
		avatar_url TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(provider, provider_id),
		UNIQUE(user_id, provider)
	);
	ALTER TABLE user_oauth_accounts ADD COLUMN IF NOT EXISTS avatar_url TEXT;
	CREATE INDEX IF NOT EXISTS idx_user_oauth_accounts_user_id ON user_oauth_accounts(user_id);
	CREATE INDEX IF NOT EXISTS idx_user_oauth_accounts_provider ON user_oauth_accounts(provider, provider_id);

	CREATE TABLE IF NOT EXISTS post_reactions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		post_slug VARCHAR(200) NOT NULL,
		reaction_type VARCHAR(50) NOT NULL,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	ALTER TABLE post_reactions DROP COLUMN IF EXISTS client_identifier;
	DELETE FROM post_reactions WHERE user_id IS NULL;
	ALTER TABLE post_reactions ALTER COLUMN user_id SET NOT NULL;
	DELETE FROM post_reactions a USING post_reactions b 
	WHERE a.created_at < b.created_at AND a.post_slug = b.post_slug AND a.user_id = b.user_id;
	ALTER TABLE post_reactions DROP CONSTRAINT IF EXISTS post_reactions_post_slug_reaction_type_client_identifier_key;
	CREATE UNIQUE INDEX IF NOT EXISTS idx_post_reactions_post_slug_user_id ON post_reactions(post_slug, user_id);
	CREATE INDEX IF NOT EXISTS idx_post_reactions_slug ON post_reactions(post_slug);
	CREATE INDEX IF NOT EXISTS idx_post_reactions_user ON post_reactions(post_slug, user_id);

	CREATE TABLE IF NOT EXISTS post_comments (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		post_slug VARCHAR(200) NOT NULL,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		parent_id UUID REFERENCES post_comments(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		is_edited BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_post_comments_slug ON post_comments(post_slug, created_at ASC);
	CREATE INDEX IF NOT EXISTS idx_post_comments_parent ON post_comments(parent_id);
	CREATE INDEX IF NOT EXISTS idx_post_comments_user ON post_comments(user_id);
	`

	_, err := db.Pool.Exec(ctx, query)
	return err
}

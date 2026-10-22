# Realm API

The core backend service for the Realm platform. It handles user authentication, dual-engine file storage, article publishing, comments and reactions, encrypted pastes, telemetry, analytics, and contact notifications. Built with Go, PostgreSQL, and gRPC alongside a high-performance HTTP REST gateway.

---

## Features

- User Authentication & 2FA: Email and password accounts, Google and GitHub OAuth social login, PASETO v2 symmetric session tokens, and RFC 6238 TOTP two-factor authentication with backup recovery codes.
- Dual-Engine File Storage: Transparent Zstandard compression on local disk or AWS S3-compatible cloud storage, direct binary streaming and multipart uploads (up to 50 MB), image dimensions extraction, Blurhash computation, and on-the-fly WebP conversion.
- Posts & Publishing: Full CRUD publishing pipeline with markdown/MDX content, slug-based routing, tag filtering, reading time estimation, and revalidation hooks.
- Comments & Reactions: Threaded comments, reaction types, administrator moderation, and comment pinning.
- Encrypted Pastes: Secure paste storage with client and server AES-GCM encryption and Argon2id key derivation.
- Analytics & Telemetry: Privacy-focused page view tracking with site filtering, country code resolution via geo headers, and real-time hardware telemetry (CPU load, core clock frequencies, memory, and database connection pool health).
- Contact Notifications: Contact form processing with honeypot bot defense and optional notification dispatches to Discord, Telegram, or SMTP email.
- Programmatic API Tokens: Built-in command-line tool (`cmd/token`) to generate, inspect, and revoke access tokens with custom scopes and per-token rate limits.
- Interactive API Documentation: Interactive documentation available in the browser at `/docs`, with OpenAPI schemas in YAML and JSON formats.

---

## Requirements

- Go 1.24 or later
- PostgreSQL 16 or later
- Protobuf compiler (`protoc`) and `buf` (optional, for regenerating protobuf stubs)
- Docker and Docker Compose (optional, for containerized deployment)

---

## Getting Started

### 1. Clone the repository

```bash
git clone https://github.com/irvanmalik48/realm-api.git
cd realm-api
```

### 2. Configure environment variables

Copy the example environment file and update the settings to match your local setup:

```bash
cp .env.example .env
```

Key settings to review:

- `DATABASE_URL`: PostgreSQL connection string (for example, `postgres://postgres:postgres@localhost:5432/realm?sslmode=disable`).
- `PASETO_SYMMETRIC_KEY`: 32-byte hex string used to encrypt user session tokens.
- `STORAGE_DIR`: Directory on disk where uploaded files are stored (default `./data/storage`).
- `MAX_UPLOAD_SIZE_MB`: Maximum file upload size in megabytes (default `50`).
- `PORT` and `GRPC_PORT`: Network ports for the HTTP gateway (default `8080`) and gRPC server (default `50051`).

### 3. Run the server

```bash
# Using Make
make dev

# Or directly with Go
go run ./cmd/server
```

The HTTP service will listen on `http://localhost:8080` and the gRPC service on `localhost:50051`.

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port for the HTTP REST gateway |
| `GRPC_PORT` | `50051` | Port for the gRPC service |
| `ENVIRONMENT` | `development` | Runtime environment (`development`, `production`, `test`) |
| `ALLOWED_ORIGINS` | `https://irvanma.eu.org,https://hq.irvanma.eu.org` | Allowed origins for browser CORS requests |
| `DATABASE_URL` | `""` | PostgreSQL connection string |
| `STORAGE_BACKEND` | `local` | Storage engine (`local` for disk, `s3` for AWS S3 / R2 / MinIO) |
| `STORAGE_DIR` | `./data/storage` | Local folder path where compressed files are stored |
| `MAX_UPLOAD_SIZE_MB` | `50` | Maximum file upload size in megabytes |
| `S3_BUCKET` | `""` | AWS S3 bucket name (when `STORAGE_BACKEND=s3`) |
| `S3_REGION` | `us-east-1` | AWS S3 region |
| `S3_ENDPOINT` | `""` | Custom S3 endpoint URL (for MinIO or Cloudflare R2) |
| `S3_ACCESS_KEY_ID` | `""` | AWS S3 access key ID |
| `S3_SECRET_ACCESS_KEY` | `""` | AWS S3 secret access key |
| `PASETO_SYMMETRIC_KEY` | `""` | 32-byte hexadecimal key for PASETO session tokens |
| `FRONTEND_URL` | `http://localhost:3000` | Web application URL for OAuth redirects |
| `GOOGLE_CLIENT_ID` | `""` | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | `""` | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | `http://localhost:8080/v1/auth/google/callback` | Google OAuth callback address |
| `GITHUB_CLIENT_ID` | `""` | GitHub OAuth client ID |
| `GITHUB_CLIENT_SECRET` | `""` | GitHub OAuth client secret |
| `GITHUB_REDIRECT_URL` | `http://localhost:8080/v1/auth/github/callback` | GitHub OAuth callback address |
| `LASTFM_API_KEY` | `""` | Last.fm API key |
| `LASTFM_API_SECRET` | `""` | Last.fm API secret |
| `CACHE_REVALIDATE_SECONDS` | `900` | Cache duration in seconds for upstream responses |
| `LOG_LEVEL` | `info` | Logging detail level (`debug`, `info`, `warn`, `error`) |
| `LOG_FORMAT` | `json` | Log format output (`json` or `text`) |
| `DISCORD_WEBHOOK_URL` | `""` | Discord webhook URL for contact notifications |
| `TELEGRAM_BOT_TOKEN` | `""` | Telegram bot token for contact notifications |
| `TELEGRAM_CHAT_ID` | `""` | Telegram chat ID for contact notifications |
| `CONTACT_RECEIVER_EMAIL` | `""` | Recipient email address for contact notifications |
| `SMTP_HOST` | `""` | Outgoing SMTP mail server host |
| `SMTP_PORT` | `587` | Outgoing SMTP mail server port |
| `SMTP_USER` | `""` | SMTP username |
| `SMTP_PASS` | `""` | SMTP password |

---

## API Endpoints

### Health and Monitoring

- `GET /`: Basic service identification.
- `GET /health`: System health status, uptime, and database connectivity.
- `GET /v1/telemetry`: Real-time CPU usage, clock frequencies, memory, and database connection pool gauges.
- gRPC `grpc.health.v1`: Standard gRPC health checking on port `50051`.

### User Authentication & Two-Factor Authentication

- `POST /v1/auth/register`: Create a new user account.
- `POST /v1/auth/login`: Sign in with email or username and password.
- `GET /v1/auth/me`: Get profile details for the authenticated user.
- `PATCH /v1/auth/profile`: Update user profile (name, avatar, username).
- `GET /v1/auth/google`: Initiate Google OAuth flow.
- `GET /v1/auth/github`: Initiate GitHub OAuth flow.
- `POST /v1/auth/2fa/setup`: Generate a new TOTP 2FA secret and QR code.
- `POST /v1/auth/2fa/verify`: Confirm 2FA setup with a 6-digit passcode and receive backup recovery codes.
- `POST /v1/auth/2fa/disable`: Disable 2FA with current password or recovery code.

### Storage Service

- `POST /v1/storage/upload`: Upload a file via raw binary body or `multipart/form-data` (up to 50 MB). JSON payloads are strictly rejected.
- `GET /v1/storage/{id}`: Stream a stored file with on-the-fly Zstandard decompression.
- `GET /v1/storage/{id}?format=webp`: Stream an image converted to WebP format.
- `GET /v1/storage/{id}/info`: Retrieve file metadata (size, savings, Blurhash, dimensions, sha256).
- `DELETE /v1/storage/{id}`: Delete a stored file and its associated cache.
- `POST /v1/storage/image-metadata`: Compute image dimensions, Blurhash, and blurDataURL with SSRF protection.

### Posts and Articles

- `GET /v1/posts`: List published articles with optional tag filters and pagination.
- `GET /v1/posts/{slug}`: Fetch an article by slug.
- `POST /v1/posts`: Create a new article (requires `posts:manage` permission).
- `PUT /v1/posts/{id}`: Update an article by ID.
- `DELETE /v1/posts/{id}`: Delete an article.

### Comments and Reactions

- `GET /v1/comments?post_slug={slug}`: Fetch comments for an article.
- `POST /v1/comments`: Submit a comment (authenticated user or verified guest).
- `PATCH /v1/comments/{id}`: Update or pin a comment.
- `DELETE /v1/comments/{id}`: Remove a comment.
- `GET /v1/reactions?post_slug={slug}`: Get reaction counts for an article.
- `POST /v1/reactions`: Toggle an emoji reaction on an article.

### Pastes

- `GET /v1/pastes`: List public pastes.
- `POST /v1/pastes`: Create a new paste (plaintext or encrypted envelope).
- `GET /v1/pastes/{slug}`: Fetch paste content and expiration metadata.
- `GET /v1/pastes/{slug}/raw`: Fetch raw paste plaintext.
- `DELETE /v1/pastes/{slug}`: Delete a paste.

### Analytics

- `POST /v1/analytics/pageview`: Record a page view with site tagging, referrer, and geo country code.
- `GET /v1/analytics/stats`: Aggregated page view analytics and country distribution.

### Contact and Music

- `POST /v1/contact`: Submit a message through the contact form with honeypot defense.
- `GET /v1/lastfm/track?username={username}&limit={limit}`: Fetch recent music scrobbles.
- `GET /v1/lastfm/user?username={username}`: Get Last.fm user profile info.

### Documentation

- `GET /docs`: Interactive API documentation interface.
- `GET /openapi.yaml`: OpenAPI 3.0 specification in YAML format.
- `GET /openapi.json`: OpenAPI 3.0 specification in JSON format.

---

## Managing API Tokens (`cmd/token`)

You can create and manage API tokens using the built-in CLI tool:

```bash
# Create a new token with specific permissions
go run ./cmd/token create -name "my-app" -scopes "storage:write,contact:read" -rpm 120 -expires 365d

# List all active tokens
go run ./cmd/token list

# Inspect a token secret
go run ./cmd/token inspect -token realm_tok_...

# Revoke a token
go run ./cmd/token revoke -id <token-uuid>
```

---

## Running with Docker Compose

Start both the API service and PostgreSQL using Docker Compose:

```bash
# Start containers in the background
docker compose up -d

# View live container logs
docker compose logs -f

# Stop containers
docker compose down
```

---

## Testing and Verification

```bash
# Run unit and integration tests
go test ./...

# Run static analysis
go vet ./...

# Check for known vulnerabilities
govulncheck ./...
```

---

## License

Licensed under the [Realm Collectives Community License (RCCL) Version 1.0](https://github.com/irvanmalik48/realm-api/blob/main/LICENSE).

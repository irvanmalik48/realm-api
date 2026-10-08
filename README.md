# Realm API

The backend service for the Realm platform. It handles user authentication, contact messages, file storage, music activity, and system monitoring. Built with Go, PostgreSQL, and gRPC alongside an HTTP gateway.

---

## Features

- User Authentication: Email and password registration, plus Google and GitHub social login with token-based sessions.
- File Storage: Stores uploaded files with automatic compression, image dimension detection, and on-the-fly WebP conversion.
- Contact Messages: Receives contact form submissions and can optionally send alerts to Discord, Telegram, or email.
- Music Tracking: Integrates with Last.fm to display current listening activity and user profiles.
- System Monitoring: Tracks CPU utilization, core clock frequencies, memory usage, and database connection pool health in real time.
- API Tokens: Built-in command-line tool to generate, inspect, and revoke access tokens with specific permissions.
- API Documentation: Interactive documentation available in the browser at `/docs`, with OpenAPI schemas in JSON and YAML formats.

---

## Requirements

- Go 1.24 or later
- PostgreSQL 16 or later
- Protobuf compiler (`protoc`) and `buf` (optional, for regenerating protobuf files)
- Docker and Docker Compose (optional, for containerized setup)

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
- `STORAGE_DIR`: Directory on disk where uploaded files are stored.
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
| `PORT` | `8080` | Port for the HTTP gateway |
| `GRPC_PORT` | `50051` | Port for the gRPC service |
| `ENVIRONMENT` | `development` | Runtime environment (`development`, `production`, `test`) |
| `ALLOWED_ORIGINS` | `https://irvanma.eu.org,https://hq.irvanma.eu.org` | Allowed origins for browser CORS requests |
| `DATABASE_URL` | `""` | PostgreSQL connection string |
| `STORAGE_DIR` | `./data/storage` | Local folder path where uploaded files are stored |
| `MAX_UPLOAD_SIZE_MB` | `10` | Maximum file upload size in megabytes |
| `PASETO_SYMMETRIC_KEY` | `""` | 32-byte hexadecimal key for session tokens |
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
| `DISCORD_WEBHOOK_URL` | `""` | Discord webhook URL for new contact alerts |
| `TELEGRAM_BOT_TOKEN` | `""` | Telegram bot token for contact alerts |
| `TELEGRAM_CHAT_ID` | `""` | Telegram chat ID for contact alerts |
| `CONTACT_RECEIVER_EMAIL` | `""` | Recipient email address for contact form submissions |
| `SMTP_HOST` | `""` | Outgoing SMTP mail server host |
| `SMTP_PORT` | `587` | Outgoing SMTP mail server port |
| `SMTP_USER` | `""` | SMTP username |
| `SMTP_PASS` | `""` | SMTP password |

---

## API Endpoints

### Health and Status

- `GET /`: Basic welcome message.
- `GET /health`: System status, uptime, and database connectivity.
- gRPC `grpc.health.v1`: Standard gRPC health checking on port `50051`.

### User Authentication

- `POST /v1/auth/register`: Create a new user account.
- `POST /v1/auth/login`: Sign in with email or username and password.
- `GET /v1/auth/me`: Get profile information for the authenticated user.
- `GET /v1/auth/google`: Start Google social sign-in.
- `GET /v1/auth/github`: Start GitHub social sign-in.

### Contact

- `POST /v1/contact`: Submit a message through the contact form.

### Storage

- `POST /v1/storage/upload`: Upload a file (requires an API token or user login).
- `GET /v1/storage/{id}`: Download a stored file.
- `GET /v1/storage/{id}?format=webp`: Download an image converted to WebP format.

### Last.fm

- `GET /v1/lastfm/track?username={username}&limit={limit}`: Get recent music tracks.
- `GET /v1/lastfm/user?username={username}`: Get user profile details.

### Documentation

- `GET /docs`: Interactive API documentation interface.
- `GET /openapi.yaml`: OpenAPI schema in YAML format.
- `GET /openapi.json`: OpenAPI schema in JSON format.

---

## Managing API Tokens (`cmd/token`)

You can create and manage API tokens using the built-in command-line tool:

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

You can start both the API service and PostgreSQL using Docker Compose:

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

# Check for known vulnerabilities
govulncheck ./...
```

---

## License

Licensed under the [Realm Collectives Community License (RCCL) Version 1.0](https://github.com/irvanmalik48/realm-api/blob/main/LICENSE).

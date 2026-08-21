# Realm API

High-performance, observable backend API service for Realm built with **Go**, **Fiber v2**, and **PostgreSQL**.

---

## Features

- **Blazing Fast**: Powered by [Fiber v2](https://github.com/gofiber/fiber/v2) and fasthttp.
- **OpenTelemetry v1.45.0**: Native distributed tracing, W3C `TraceContext` / `Baggage` propagators, OTLP HTTP exporter, and `X-Trace-Id` correlation headers.
- **OpenAPI 3.2.0 Compliant**: Interactive API documentation powered by [Scalar](https://github.com/scalar/scalar) served live at `/docs`, `/openapi.yaml`, and `/openapi.json`.
- **Health Check & Uptime**: Real-time heartbeat endpoint (`/health` & `/v1/health`) checking database connectivity and server uptime.
- **Secure API Tokens**: Cryptographically secure token authentication (`realm_tok_...`) generated via CLI (`cmd/token`), hashed with SHA-256 in PostgreSQL, with in-memory TTL caching.
- **Per-Token Rate Limiting**: Dynamic 1-minute sliding window rate limiter with standard `X-RateLimit-*` response headers.
- **Zstandard (`zstd`) File Storage**: High-compression disk storage with automatic Blurhash calculation, dimension extraction, and on-the-fly WebP conversion (`?format=webp`).
- **PostgreSQL Persistence**: Contact submissions, file metadata, and API tokens stored via `pgxpool` with automatic schema migrations.
- **LastFM Integration**: AudioScrobbler recent tracks and user statistics with caching headers.
- **Multi-channel Alerts**: Optional instant notifications to Discord webhooks or Telegram bots upon new contact messages.
- **Container Ready**: Multi-stage lightweight `Dockerfile` containing `/app/server` and `/app/token` binaries, and `docker-compose.yml` with non-root security.

---

## API Specification & Interactive Docs

* **Interactive Docs**: `https://api.irvanma.eu.org/docs` (or `http://localhost:8080/docs`)
* **OpenAPI 3.2.0 Spec (YAML)**: `GET /openapi.yaml` (or `/v1/openapi.yaml`)
* **OpenAPI 3.2.0 Spec (JSON)**: `GET /openapi.json` (or `/v1/openapi.json`)
* Complete endpoint guide and schema references are documented in [`API.md`](./API.md).

---

## API Reference

### 1. Health & Status

#### `GET /`
Root greeting endpoint.
```json
{
  "message": "Nothing to see here",
  "status": "success"
}
```

#### `GET /health` or `GET /v1/health`
Detailed service health, uptime, and database connectivity.
```json
{
  "status": "healthy",
  "service": "realm-api",
  "version": "1.0.0",
  "uptime_seconds": 86400,
  "timestamp": "2026-08-20T13:18:31Z",
  "database": "connected"
}
```

---

### 2. Contact Form Submission
Submits a contact form message and persists it into PostgreSQL.

```http
POST /v1/contact
Content-Type: application/json
X-Realm-Request: 1
```

```json
{
  "name": "Jane Doe",
  "email": "jane@example.com",
  "subject": "Project Collaboration",
  "message": "Hello, I would like to discuss a project with you."
}
```

#### Validation Rules
| Field | Type | Required | Constraints |
|---|---|---|---|
| `name` | string | **Yes** | Min 2, max 100 characters |
| `email` | string | **Yes** | Valid email format, max 254 characters |
| `subject` | string | **Yes** | Min 3, max 200 characters |
| `message` | string | **Yes** | Min 10, max 5000 characters |

---

### 3. LastFM Integration

#### Get Recent Tracks
```http
GET /v1/lastfm/track?username={username}&limit={limit}
```

#### Get User Profile Info
```http
GET /v1/lastfm/user?username={username}
```

---

### 4. File Storage Subsystem (Zstd Compressed & WebP)

#### Upload File
Uploads any file, compresses it on disk using **Zstandard (`zstd`)**, and automatically calculates its **Blurhash** and dimensions.

```http
POST /v1/storage/upload
Authorization: Bearer realm_tok_...
Content-Type: multipart/form-data
```

##### Success Response (`201 Created`)
```json
{
  "status": "success",
  "message": "File uploaded and compressed successfully",
  "file": {
    "id": "7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b",
    "filename": "wallpaper.png",
    "content_type": "image/png",
    "original_size": 2450000,
    "compressed_size": 1120000,
    "savings_percent": 54.28,
    "sha256": "5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8",
    "blurhash": "LEHV6nWB2yk8pyo0adR*.7kCMdnj",
    "width": 1920,
    "height": 1080,
    "url": "/v1/storage/7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b",
    "webp_url": "/v1/storage/7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b?format=webp",
    "created_at": "2026-08-20T12:00:00Z"
  }
}
```

#### Get File (Original or On-the-Fly WebP)
Streams the decompressed file from disk. Adding `?format=webp` or header `Accept: image/webp` dynamically converts images to WebP on-the-fly.

```http
GET /v1/storage/{id}
GET /v1/storage/{id}?format=webp
```

#### Get File Info / Metadata
```http
GET /v1/storage/{id}/info
```

#### Delete File
```http
DELETE /v1/storage/{id}
Authorization: Bearer realm_tok_...
```

---

## Administrative API Token CLI (`cmd/token`)

Tokens are generated with direct database access via the administrative CLI tool.

### Local Usage:
```bash
# Create a new API token
go run ./cmd/token create -name "my-app" -scopes "storage:write,contact:read" -rpm 120 -expires 365d

# List all tokens
go run ./cmd/token list

# Inspect a raw token secret against database
go run ./cmd/token inspect -token realm_tok_...

# Revoke a token
go run ./cmd/token revoke -id <token-uuid>
```

### Docker Compose Usage:
```bash
# Create a full-access token inside Docker
sudo docker compose exec api /app/token create -name "production-app" -scopes "*" -rpm 300

# List tokens
sudo docker compose exec api /app/token list
```

---

## Database Schema

Migrations run automatically on server startup:

```sql
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
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server listening port |
| `ENVIRONMENT` | `development` | Environment (`development`, `production`, `test`) |
| `ALLOWED_ORIGINS` | `https://irvanma.eu.org` | Comma-separated CORS allowed origins |
| `DATABASE_URL` | `""` | PostgreSQL connection string |
| `STORAGE_DIR` | `./data/storage` | Directory path for Zstd compressed file storage |
| `MAX_UPLOAD_SIZE_MB` | `10` | Maximum allowed file upload size in megabytes |
| `POSTGRES_USER` | `postgres` | PostgreSQL user for Docker Compose |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password for Docker Compose |
| `POSTGRES_DB` | `realm` | PostgreSQL database name |
| `LASTFM_API_KEY` | `""` | LastFM AudioScrobbler API Key |
| `LASTFM_API_SECRET` | `""` | LastFM API Secret (optional) |
| `CACHE_REVALIDATE_SECONDS` | `900` | Caching TTL in seconds for response headers |
| `OTEL_EXPORTER_OTLP_ENDPOINT`| `""` | OpenTelemetry OTLP HTTP collector endpoint |
| `OTEL_STDOUT_TRACING` | `false` | Set to `true` to print traces to stdout |
| `DISCORD_WEBHOOK_URL` | `""` | Optional Discord webhook for instant notifications |
| `TELEGRAM_BOT_TOKEN` | `""` | Optional Telegram bot token for alerts |
| `TELEGRAM_CHAT_ID` | `""` | Optional Telegram chat ID for alerts |

---

## Getting Started

### Local Development
```bash
# 1. Copy environment template
cp .env.example .env

# 2. Configure DATABASE_URL and LASTFM_API_KEY in .env

# 3. Run development server
make dev
# or
go run ./cmd/server
```

### Running Tests
```bash
make test
# or
go test -v ./...
```

### Building Binaries
```bash
make build
# Outputs ./bin/server and ./bin/token
```

---

## Docker & Docker Compose

### Prerequisites
Ensure the external Caddy network exists before starting the stack:
```bash
docker network create caddy_net
```

### Starting Services
```bash
# Start API and PostgreSQL in background
docker compose up -d

# View logs
docker compose logs -f

# Stop containers
docker compose down
```

### Caddy Reverse Proxy Configuration
```caddy
api.irvanma.eu.org {
    reverse_proxy realm-api:8080
}
```
Reload Caddy:
```bash
docker exec -w /etc/caddy caddy caddy reload
```

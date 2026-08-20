# Realm API

High-performance backend API service for Realm built with **Go**, **Fiber v2**, and **PostgreSQL**.

## Features

- **Blazing Fast**: Powered by [Fiber v2](https://github.com/gofiber/fiber/v2) and fasthttp.
- **RESTful Endpoints**: Clean versioned routes (`/v1/...`) with zero boilerplate.
- **PostgreSQL Persistence**: Contact form submissions stored in PostgreSQL via `pgxpool` with automatic table migrations.
- **Rate Limiting**: Built-in rate limiting protection for contact form submissions.
- **LastFM Integration**: Recent scrobbles and user profile stats with caching and rate handling.
- **Multi-channel Alerts**: Optional instant notifications to Discord webhooks or Telegram bots upon new contact messages.
- **Security & Headers**: Configurable CORS origins, Cache-Control headers with `s-maxage` and `stale-while-revalidate`.
- **Container Ready**: Multi-stage lightweight `Dockerfile` and `docker-compose.yml` with non-root security and PostgreSQL service.

---

## API Specification

Complete OpenAPI 3.2.0 compliant documentation and schema specifications are available in [`API.md`](./API.md).

---

## API Reference

### 1. Root / Health Check
```http
GET /
```
**Response (`200 OK`)**:
```json
{
  "message": "Nothing to see here",
  "status": "success"
}
```

---

### 2. Contact Form Submission
Submits a contact form message and persists it into PostgreSQL.

```http
POST /v1/contact
```

#### Request Headers
```http
Content-Type: application/json
```

#### Request Body
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

#### Success Response (`200 OK`)
```json
{
  "message": "Your message has been sent successfully.",
  "status": "success"
}
```

#### Error Response (`400 Bad Request` / `429 Too Many Requests`)
```json
{
  "error": "Invalid email address",
  "status": "error"
}
```

---

### 3. LastFM Recent Tracks
Fetches recent scrobbles for a given LastFM user.

```http
GET /v1/lastfm/track?username={username}&limit={limit}
```

#### Query Parameters
| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `username` | string | **Yes** | — | LastFM username |
| `limit` | integer | No | `1` | Number of tracks to return (range `1`–`200`) |

#### Response Headers
```http
Cache-Control: public, s-maxage=900, stale-while-revalidate=1800
Content-Type: application/json
```

#### Success Response (`200 OK`)
```json
{
  "recenttracks": {
    "@attr": {
      "page": "1",
      "perPage": "1",
      "total": "12345",
      "totalPages": "12345",
      "user": "username"
    },
    "track": [
      {
        "name": "Track Name",
        "artist": {
          "mbid": "",
          "#text": "Artist Name"
        },
        "album": {
          "mbid": "",
          "#text": "Album Name"
        },
        "url": "https://www.last.fm/music/...",
        "image": [
          { "size": "small", "#text": "https://..." },
          { "size": "medium", "#text": "https://..." },
          { "size": "large", "#text": "https://..." },
          { "size": "extralarge", "#text": "https://..." }
        ]
      }
    ]
  }
}
```

---

### 4. LastFM User Info
Fetches profile statistics for a LastFM user.

```http
GET /v1/lastfm/user?username={username}
```

#### Query Parameters
| Parameter | Type | Required | Description |
|---|---|---|---|
| `username` | string | **Yes** | LastFM username |

#### Response Headers
```http
Cache-Control: public, s-maxage=900, stale-while-revalidate=1800
Content-Type: application/json
```

#### Success Response (`200 OK`)
```json
{
  "user": {
    "name": "username",
    "playcount": "54321",
    "artist_count": "1200",
    "track_count": "3400",
    "album_count": "800",
    "image": [
      { "size": "small", "#text": "https://..." },
      { "size": "medium", "#text": "https://..." },
      { "size": "large", "#text": "https://..." },
      { "size": "extralarge", "#text": "https://..." }
    ],
    "url": "https://www.last.fm/user/username"
  }
}
```

---

### 5. File Storage (Zstd Compressed & WebP)

#### Upload File
Uploads any file, compresses it on disk using **Zstandard (`zstd`)**, and automatically calculates its **Blurhash** and dimensions if it is an image.

```http
POST /v1/storage/upload
```

##### Headers
```http
Content-Type: multipart/form-data
X-API-Key: your_storage_api_key (Optional / if STORAGE_API_KEY configured)
```

##### Form Fields
| Field | Type | Description |
|---|---|---|
| `file` | Binary File | The file to upload |

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

---

#### Get File (Original or On-the-Fly WebP)
Streams the decompressed file from disk. If requested with `?format=webp` or `Accept: image/webp`, dynamically converts images to WebP on-the-fly without storing duplicates.

```http
GET /v1/storage/{id}
GET /v1/storage/{id}?format=webp
```

##### Query Parameters
| Parameter | Type | Default | Description |
|---|---|---|---|
| `format` | string | original | Set to `webp` to convert image on-the-fly |
| `download` | boolean | `false` | Set to `1` or `true` for `attachment` disposition |

##### Response Headers
```http
Content-Type: image/webp (or original MIME type)
ETag: "5e884898da280471..." (supports If-None-Match 304)
Cache-Control: public, max-age=31536000, immutable
X-Blurhash: LEHV6nWB2yk8pyo0adR*.7kCMdnj
X-Image-Width: 1920
X-Image-Height: 1080
```

---

#### Get File Info / Metadata
```http
GET /v1/storage/{id}/info
```

---

#### Delete File
```http
DELETE /v1/storage/{id}
```

---

## Database Schema

Automatic table migration creates the `contact_submissions` and `files` tables on startup:

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
| `STORAGE_API_KEY` | `""` | Optional API Key required for upload and delete |
| `MAX_UPLOAD_SIZE_MB` | `50` | Maximum allowed file upload size in megabytes |
| `POSTGRES_USER` | `postgres` | PostgreSQL user for Docker Compose |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password for Docker Compose |
| `POSTGRES_DB` | `realm` | PostgreSQL database name |
| `LASTFM_API_KEY` | `""` | LastFM AudioScrobbler API Key |
| `LASTFM_API_SECRET` | `""` | LastFM API Secret (optional) |
| `CACHE_REVALIDATE_SECONDS` | `900` | Caching TTL in seconds for response headers |
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
# Output in ./bin/server and ./bin/token
```

### Managing API Tokens (CLI)
Generate and manage secure API tokens via the administrative CLI tool:
```bash
# Create a new API token
go run ./cmd/token create -name "my-app" -scopes "storage:write,contact:read" -rpm 120 -expires 365d

# List all tokens
go run ./cmd/token list

# Inspect / verify a raw token
go run ./cmd/token inspect -token realm_tok_...

# Revoke a token
go run ./cmd/token revoke -id <token-uuid>
```

---

## Docker & Docker Compose

### Prerequisites
Ensure the external Caddy network exists before starting the stack:
```bash
docker network create caddy_net
```

### Using Docker Compose
```bash
# Start API and PostgreSQL in background
docker compose up -d

# View logs
docker compose logs -f

# Stop containers
docker compose down
```

### Caddy Reverse Proxy Configuration
Add the following to your server's `Caddyfile`:
```caddy
api.irvanma.eu.org {
    reverse_proxy realm-api:8080
}
```

Reload Caddy:
```bash
docker exec -w /etc/caddy caddy caddy reload
```

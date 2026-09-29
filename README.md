# Realm API

High-performance, observable, and technologically hardened backend service for Realm built with **Go**, **gRPC**, **Fiber v2**, and **PostgreSQL**.

---

## Technological Marvel & Overkill Features

- **Blazing Fast gRPC Backend**: Native high-performance gRPC services on port `:50051` with Protobuf schemas powering internal microservices, Server Actions, and BFF proxies.
- **Protobuf Governance & Buf Tooling**: Governed with **Buf v2** (`buf.yaml` and `buf.gen.yaml`), enforcing strict schema linting, breaking change detection, and type-safe code generation.
- **Standard gRPC Health Checking**: Fully compliant with `grpc.health.v1` (`google.golang.org/grpc/health`) serving statuses for every subsystem alongside existing reflection and HTTP `/health`.
- **Trace-Correlated Structured Logging**: Standard `log/slog` structured logger with custom OpenTelemetry context handler that automatically extracts `trace_id` and `span_id` into every log record.
- **Continuous Profiling (Pyroscope)**: Native Grafana Pyroscope profiler capturing CPU, heap allocations, goroutines, and block profiles (configured via `PYROSCOPE_SERVER_ADDRESS`).
- **Database Observability (`otelpgx`)**: PostgreSQL connection pool instrumented with `exaring/otelpgx` for automated SQL span tracing and latency diagnostics.
- **Circuit Breaking & Fault Tolerance**: Sony `gobreaker` circuit breaker wrapping external upstreams (LastFM) with automated stale-cache fallbacks during upstream degradation or outages.
- **Linux Kernel Landlock Sandboxing**: Process-level filesystem sandboxing via `shoenig/go-landlock` restricting file access strictly to allowed certificates, assets, and `/tmp`.
- **High-Throughput Socket Tuning**: Socket reuse via `SO_REUSEPORT` enabled on the gRPC listener for high-concurrency throughput and multi-process scaling.
- **HTTP/REST Hybrid Gateway**: Powered by [Fiber v2](https://github.com/gofiber/fiber/v2) on port `:8080` for browser media streaming (WebP/Blurhash) and OAuth2 consent redirects.
- **User Authentication & PASETO**: Traditional credentials (Email/Username + bcrypt) and **Google OIDC** / **GitHub OAuth2** social login with tamper-proof **PASETO v2.local** symmetric bearer tokens.
- **OpenTelemetry Distributed Tracing**: Native distributed tracing with `otelgrpc` interceptors and HTTP trace correlation headers.
- **Interactive OpenAPI 3.2.0 Docs**: Interactive documentation powered by [Scalar](https://github.com/scalar/scalar) served live at `/docs`, `/openapi.yaml`, and `/openapi.json`.
- **Secure API Tokens**: Cryptographically secure token authentication (`realm_tok_...`) generated via CLI (`cmd/token`), hashed with SHA-256 in PostgreSQL, with in-memory TTL caching.
- **Sliding-Window Rate Limiting**: Token-bucket sliding window rate limiter with gRPC interceptors and standard `X-RateLimit-*` response headers.
- **Zstandard (`zstd`) File Storage**: High-compression disk storage with automatic Blurhash calculation, dimension extraction, gRPC streaming, and on-the-fly WebP conversion (`?format=webp`).
- **PostgreSQL Persistence**: User accounts, contact submissions, file metadata, and API tokens stored via `pgxpool` with automatic schema migrations.
- **Multi-channel Alerts**: Optional instant notifications to Discord webhooks, Telegram bots, or SMTP upon new contact messages.
- **Production-Hardened Containers**: Multi-stage lightweight `Dockerfile` with Alpine 3.21, `tini` init process, container health checking, and Docker log rotation.

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

#### gRPC Health Check (`grpc.health.v1`)
Standard health probing via any standard gRPC health client (e.g. `grpc-health-probe`):
```bash
grpc-health-probe -addr=localhost:50051 -service=realm.v1.AuthService
```

---

### 2. User Authentication (PASETO & OIDC)

#### User Registration
```http
POST /v1/auth/register
Content-Type: application/json
```
```json
{
  "email": "jane@example.com",
  "username": "janedoe",
  "password": "SecurePassword123!",
  "full_name": "Jane Doe",
  "avatar_url": "https://example.com/avatar.png"
}
```

#### User Login
```http
POST /v1/auth/login
Content-Type: application/json
```
```json
{
  "identifier": "janedoe",
  "password": "SecurePassword123!"
}
```

#### Current User Profile
```http
GET /v1/auth/me
Authorization: Bearer v2.local...
```

#### Social OIDC Logins
- Google: `GET /v1/auth/google` (initiates consent) -> `/v1/auth/google/callback`
- GitHub: `GET /v1/auth/github` (initiates consent) -> `/v1/auth/github/callback`

---

### 3. Contact Form Submission
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

---

### 4. LastFM Integration
Wrapped with a **Sony gobreaker** circuit breaker and stale-while-revalidate caching.

#### Get Recent Tracks
```http
GET /v1/lastfm/track?username={username}&limit={limit}
```

#### Get User Profile Info
```http
GET /v1/lastfm/user?username={username}
```

---

### 5. File Storage Subsystem (Zstd Compressed & WebP)

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

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP gateway port |
| `GRPC_PORT` | `50051` | High-performance gRPC port |
| `ENVIRONMENT` | `development` | Environment (`development`, `production`, `test`) |
| `ALLOWED_ORIGINS` | `https://irvanma.eu.org` | Comma-separated CORS allowed origins |
| `DATABASE_URL` | `""` | PostgreSQL connection string |
| `STORAGE_DIR` | `./data/storage` | Directory path for Zstd compressed file storage |
| `MAX_UPLOAD_SIZE_MB` | `10` | Maximum allowed file upload size in megabytes |
| `PASETO_SYMMETRIC_KEY` | `""` | 32-byte hex key for PASETO token encryption *(required in production)* |
| `FRONTEND_URL` | `http://localhost:3000` | Frontend web application origin for OAuth redirects |
| `GOOGLE_CLIENT_ID` | `""` | Google OAuth2 client ID |
| `GOOGLE_CLIENT_SECRET` | `""` | Google OAuth2 client secret |
| `GOOGLE_REDIRECT_URL` | `http://localhost:8080/v1/auth/google/callback` | Google OAuth2 redirect callback URL |
| `GITHUB_CLIENT_ID` | `""` | GitHub OAuth2 client ID |
| `GITHUB_CLIENT_SECRET` | `""` | GitHub OAuth2 client secret |
| `GITHUB_REDIRECT_URL` | `http://localhost:8080/v1/auth/github/callback` | GitHub OAuth2 redirect callback URL |
| `POSTGRES_USER` | `postgres` | PostgreSQL user for Docker Compose |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password for Docker Compose |
| `POSTGRES_DB` | `realm` | PostgreSQL database name |
| `LASTFM_API_KEY` | `""` | LastFM AudioScrobbler API Key |
| `LASTFM_API_SECRET` | `""` | LastFM API Secret (optional) |
| `CACHE_REVALIDATE_SECONDS` | `900` | Caching TTL in seconds for response headers |
| `OTEL_EXPORTER_OTLP_ENDPOINT`| `""` | OpenTelemetry OTLP gRPC endpoint (e.g. `localhost:4317`) |
| `PYROSCOPE_SERVER_ADDRESS` | `""` | Grafana Pyroscope continuous profiling server address |
| `LOG_LEVEL` | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`) |
| `LOG_FORMAT` | `json` | Structured log output format (`json` or `text`) |
| `DISCORD_WEBHOOK_URL` | `""` | Optional Discord webhook for instant notifications |
| `TELEGRAM_BOT_TOKEN` | `""` | Optional Telegram bot token for alerts |
| `TELEGRAM_CHAT_ID` | `""` | Optional Telegram chat ID for alerts |
| `CONTACT_RECEIVER_EMAIL` | `""` | Email address to receive contact form notifications |
| `SMTP_HOST` | `""` | SMTP mail server host |
| `SMTP_PORT` | `587` | SMTP mail server port |
| `SMTP_USER` | `""` | SMTP username |
| `SMTP_PASS` | `""` | SMTP password |

---

## Development & Quality Verification

### Build & Dev Commands
```bash
# 1. Run development server
make dev
# or
go run ./cmd/server

# 2. Recompile Protobuf schemas with Buf
make proto

# 3. Lint Protobuf schemas
make lint-proto

# 4. Build release binaries
make build
```

### Security & Quality Verification Pipeline
```bash
# Run unit & integration tests
go test -v ./test/...

# Run Gosec AST security scanner (excluding generated Protobuf code)
make sec

# Run Go vulnerability database scanner
make vuln

# Run full audit (Tests + Gosec + Govulncheck + Buf Lint)
make check
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

The Docker stack includes built-in container health checking (`wget -qO- /health`), process init wrapping (`tini`), log rotation (`20m`/`5` files), and PostgreSQL shared memory optimization (`shm_size: 256mb`).

---

## License

Licensed under the [Realm Collectives Community License (RCCL) Version 1.0](https://github.com/irvanmalik48/realm-api/blob/main/LICENSE).

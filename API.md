# Realm API Specification

OpenAPI 3.2.0 compliant specification and route documentation for Realm API.

---

## Table of Contents

- [Overview](#overview)
- [Base URLs](#base-urls)
- [Authentication & Token Management](#authentication--token-management)
- [User Authentication & OIDC (PASETO)](#user-authentication--oidc-paseto)
- [Error Format](#error-format)
- [Endpoints](#endpoints)
  - [Health & Status](#health--status)
  - [Authentication & Users](#authentication--users)
  - [LastFM](#lastfm)
  - [Contact](#contact)
  - [Storage](#storage)
- [Live OpenAPI 3.2.0 Specification & Documentation](#live-openapi-320-specification--documentation)

---

## Overview

Realm API is a RESTful backend service built with Go and Fiber v2. It provides endpoints for User Authentication (traditional and Google/GitHub OIDC with PASETO tokens), LastFM profile and scrobble data, contact form submission with PostgreSQL persistence and notifications, and Zstandard-compressed file storage with automatic Blurhash calculation and on-the-fly WebP conversion.

---

## Base URLs

- **Production**: `https://api.irvanma.eu.org`
- **Development**: `http://localhost:8080`

---

## Authentication & Token Management

### 1. Administrative API Tokens
Realm API supports cryptographically secure administrative API tokens (`realm_tok_...`) generated exclusively via the administrative CLI tool (`cmd/token`) for programmatic access and machine-to-machine integrations.

#### Administrative Authentication Headers

| Scheme | Header | Description |
|---|---|---|
| **Bearer Token** | `Authorization: Bearer realm_tok_<secret>` | Standard Bearer token authentication |
| **API Token** | `X-API-Token: realm_tok_<secret>` | Alternative token header |
| **API Key Header** | `X-API-Key: realm_tok_<secret>` | Alternative token header |

---

## User Authentication & OIDC (PASETO)

User sessions utilize **PASETO v2.local** (Platform-Agnostic Security Tokens with ChaCha20-Poly1305 symmetric encryption). Tokens are issued upon successful registration, login, or OAuth callback, and are valid for 7 days.

### User Authentication Headers

| Scheme | Header | Description |
|---|---|---|
| **Bearer Token** | `Authorization: Bearer v2.local.<payload>` | Standard PASETO bearer token |
| **Auth Header** | `X-Auth-Token: v2.local.<payload>` | Custom auth header |
| **Cookie** | `Cookie: realm_auth_token=v2.local...` | Secure httpOnly cookie |

### Token Claims Structure

```json
{
  "id": "uuid-v4",
  "email": "user@example.com",
  "username": "johndoe",
  "full_name": "John Doe",
  "avatar_url": "https://...",
  "provider": "local",
  "iss": "realm-api",
  "sub": "uuid-v4",
  "aud": "realm-frontend",
  "iat": "2026-08-21T12:00:00Z",
  "exp": "2026-08-28T12:00:00Z"
}
```

### Generating & Managing Tokens (CLI)

Tokens can only be created with direct database access via the CLI tool:

```bash
# Generate a new token with specific scopes and rate limit
go run ./cmd/token create -name "my-app" -scopes "storage:write,contact:read" -rpm 120 -expires 365d

# List all active tokens
go run ./cmd/token list

# Inspect a raw token string
go run ./cmd/token inspect -token realm_tok_...

# Revoke a token by ID
go run ./cmd/token revoke -id <token-uuid>
```

### Rate Limiting & Dynamic Headers

Authenticated requests receive per-token rate limiting according to their configured RPM (requests per minute). The server includes standard rate limit headers on every response:

| Header | Description |
|---|---|
| `X-RateLimit-Limit` | Maximum allowed requests per 1-minute window |
| `X-RateLimit-Remaining` | Remaining requests allowed in the current window |
| `X-RateLimit-Reset` | Unix timestamp (seconds) when the rate limit window resets |


---

## Error Format

All error responses follow a standard JSON structure:

```json
{
  "error": "Detailed error message",
  "status": "error"
}
```

| HTTP Status | Meaning |
|---|---|
| `400 Bad Request` | Invalid parameters, validation failure, or malformed JSON payload |
| `401 Unauthorized` | Missing or invalid API key on protected endpoints |
| `403 Forbidden` | CSRF origin validation mismatch |
| `404 Not Found` | Requested route or file resource not found |
| `413 Payload Too Large` | Upload size exceeds configured `MAX_UPLOAD_SIZE_MB` limit |
| `429 Too Many Requests` | Rate limit exceeded (e.g. 5 requests per 10 minutes on `/v1/contact`) |
| `500 Internal Server Error` | Unexpected backend or database error |
| `502 Bad Gateway` | Upstream service (e.g. LastFM API) unreachable or erroring |

---

## Endpoints

### Health & Status

#### `GET /`
Root greeting endpoint.

- **Request**: No parameters or headers required.
- **Response (`200 OK`)**:
  ```json
  {
    "message": "Nothing to see here",
    "status": "success"
  }
  ```

#### `GET /health` / `GET /v1/health`
Detailed service health, uptime, and database connectivity check.

- **Request**: No parameters or headers required.
- **Response (`200 OK`)**:
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

### Authentication & Users

#### `POST /v1/auth/register`
Registers a new user account with traditional email, username, and password credentials, and issues a 7-day PASETO v2 token.

- **Request Body**:
  ```json
  {
    "email": "jane@example.com",
    "username": "janedoe",
    "password": "SecurePassword123!",
    "full_name": "Jane Doe",
    "avatar_url": "https://example.com/avatar.png"
  }
  ```
- **Validation Rules**:
  - `email`: string, required, valid email format, unique.
  - `username`: string, required, 3–30 alphanumeric/underscore characters, unique.
  - `password`: string, required, minimum 8 characters (hashed via bcrypt).
  - `full_name`: string, required, 2–100 characters.
  - `avatar_url`: string, optional URL.
- **Response (`201 Created`)**:
  ```json
  {
    "status": "success",
    "message": "Registration successful",
    "token": "v2.local.k7x...",
    "user": {
      "id": "7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b",
      "email": "jane@example.com",
      "username": "janedoe",
      "full_name": "Jane Doe",
      "avatar_url": "https://example.com/avatar.png",
      "provider": "local",
      "created_at": "2026-08-21T12:00:00Z"
    }
  }
  ```

---

#### `POST /v1/auth/login`
Authenticates a user with their username or email address and password.

- **Request Body**:
  ```json
  {
    "identifier": "janedoe",
    "password": "SecurePassword123!"
  }
  ```
- **Response (`200 OK`)**:
  ```json
  {
    "status": "success",
    "message": "Login successful",
    "token": "v2.local.k7x...",
    "user": {
      "id": "7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b",
      "email": "jane@example.com",
      "username": "janedoe",
      "full_name": "Jane Doe",
      "avatar_url": "https://example.com/avatar.png",
      "provider": "local",
      "created_at": "2026-08-21T12:00:00Z"
    }
  }
  ```

---

#### `GET /v1/auth/me`
Retrieves the profile of the currently authenticated user.

- **Authentication**: Required (`Authorization: Bearer <paseto-token>` or `X-Auth-Token` or `realm_auth_token` cookie).
- **Response (`200 OK`)**:
  ```json
  {
    "status": "success",
    "user": {
      "id": "7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b",
      "email": "jane@example.com",
      "username": "janedoe",
      "full_name": "Jane Doe",
      "avatar_url": "https://example.com/avatar.png",
      "provider": "local",
      "created_at": "2026-08-21T12:00:00Z"
    }
  }
  ```

---

#### `PATCH /v1/auth/profile`
Updates profile information (Full Name and/or Avatar URL) for the currently authenticated user.

- **Authentication**: Required.
- **Request Body**:
  ```json
  {
    "full_name": "Jane Smith",
    "avatar_url": "https://example.com/new-avatar.png"
  }
  ```
- **Response (`200 OK`)**:
  ```json
  {
    "status": "success",
    "message": "Profile updated successfully",
    "user": {
      "id": "7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b",
      "email": "jane@example.com",
      "username": "janedoe",
      "full_name": "Jane Smith",
      "avatar_url": "https://example.com/new-avatar.png",
      "provider": "local",
      "created_at": "2026-08-21T12:00:00Z"
    }
  }
  ```

---

#### `GET /v1/auth/google` & `GET /v1/auth/google/callback`
Initiates Google OIDC flow and handles authorization callback, automatically finding or provisioning user accounts and returning a PASETO session token to the frontend.

---

#### `GET /v1/auth/github` & `GET /v1/auth/github/callback`
Initiates GitHub OAuth2 flow and handles authorization callback, automatically finding or provisioning user accounts and returning a PASETO session token to the frontend.

---

### LastFM

#### `GET /v1/lastfm/track`
Retrieves recent scrobbles for a LastFM user.

- **Query Parameters**:
  - `username` (string, **required**): LastFM user identifier.
  - `limit` (integer, optional, default: `1`, range: `1`–`200`): Number of recent tracks to return.
- **Response Headers**:
  - `Cache-Control`: `public, s-maxage=900, stale-while-revalidate=1800`
  - `Content-Type`: `application/json`
- **Response (`200 OK`)**:
  ```json
  {
    "recenttracks": {
      "track": [
        {
          "name": "Song Title",
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
          ],
          "date": {
            "uts": "1700000000",
            "#text": "20 Nov 2023, 12:00"
          },
          "@attr": {
            "nowplaying": "true"
          }
        }
      ]
    }
  }
  ```

---

#### `GET /v1/lastfm/user`
Retrieves profile details and listening statistics for a LastFM user.

- **Query Parameters**:
  - `username` (string, **required**): LastFM user identifier.
- **Response Headers**:
  - `Cache-Control`: `public, s-maxage=900, stale-while-revalidate=1800`
  - `Content-Type`: `application/json`
- **Response (`200 OK`)**:
  ```json
  {
    "user": {
      "name": "username",
      "realname": "Display Name",
      "playcount": "12345",
      "artist_count": "560",
      "track_count": "2300",
      "album_count": "450",
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

### Contact

#### `POST /v1/contact`
Submits a contact form message, persists it to PostgreSQL, and triggers configured webhook notifications.

- **Rate Limit**: 5 requests per 10 minutes per IP.
- **Required Headers**:
  - `Content-Type`: `application/json`
  - `X-Realm-Request: 1` or `X-Requested-With: XMLHttpRequest` (CSRF defense)
- **Request Body**:
  ```json
  {
    "name": "Jane Doe",
    "email": "jane@example.com",
    "subject": "Project Inquiry",
    "message": "Hello, I would like to discuss a project collaboration.",
    "_gotcha": ""
  }
  ```
- **Validation Rules**:
  - `name`: string, required, length 2–100 characters.
  - `email`: string, required, valid email format, max 254 characters.
  - `subject`: string, required, length 3–200 characters.
  - `message`: string, required, length 10–5000 characters.
  - `_gotcha`: string, optional (honeypot field; if populated, message is dropped silently).
- **Response (`200 OK`)**:
  ```json
  {
    "message": "Your message has been sent successfully.",
    "status": "success"
  }
  ```

---

### Storage

#### `POST /v1/storage/upload`
Uploads a binary file, compresses it with Zstandard on disk, and computes Blurhash and dimensions if the file is an image.

- **Authentication**: Required if `STORAGE_API_KEY` is configured (`X-API-Key` or `Authorization: Bearer <key>`).
- **Headers**:
  - `Content-Type`: `multipart/form-data`
- **Form Data**:
  - `file`: binary file payload (required).
- **Response (`201 Created`)**:
  ```json
  {
    "status": "success",
    "message": "File uploaded and compressed successfully",
    "file": {
      "id": "7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b",
      "filename": "cover.png",
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

#### `GET /v1/storage/{id}`
Streams the stored file decompressed on-the-fly. Converts supported images to WebP on endpoint level when requested.

- **Path Parameters**:
  - `id` (UUID string, **required**): File identifier.
- **Query Parameters**:
  - `format` (string, optional): Set to `webp` to trigger on-the-fly WebP conversion.
  - `download` (boolean, optional): Set to `1` or `true` to serve with `Content-Disposition: attachment`.
- **Request Headers**:
  - `Accept`: `image/webp` (alternative to `?format=webp`)
  - `If-None-Match`: `"<etag>"` (enables HTTP 304 caching)
- **Response Headers**:
  - `Content-Type`: MIME type (`image/webp` if converted, otherwise original MIME)
  - `Content-Disposition`: `inline; filename="..."` (or `attachment` if `download=true`)
  - `ETag`: `"sha256_hash"` or `"webp-sha256_hash"`
  - `Cache-Control`: `public, max-age=31536000, immutable`
  - `X-Blurhash`: Blurhash string (images only)
  - `X-Image-Width`: Image width in pixels (images only)
  - `X-Image-Height`: Image height in pixels (images only)
- **Response (`200 OK`)**: Binary data stream.
- **Response (`304 Not Modified`)**: Returned when ETag matches `If-None-Match`.

---

#### `GET /v1/storage/{id}/info`
Retrieves metadata, compression metrics, and Blurhash for a stored file.

- **Path Parameters**:
  - `id` (UUID string, **required**): File identifier.
- **Response (`200 OK`)**:
  ```json
  {
    "status": "success",
    "file": {
      "id": "7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b",
      "filename": "cover.png",
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

#### `DELETE /v1/storage/{id}`
Deletes the compressed file from disk and its metadata from PostgreSQL.

- **Authentication**: Required if `STORAGE_API_KEY` is configured.
- **Path Parameters**:
  - `id` (UUID string, **required**): File identifier.
- **Response (`200 OK`)**:
  ```json
  {
    "message": "File deleted successfully",
    "status": "success"
  }
  ```

---

---

## Live OpenAPI 3.2.0 Specification & Documentation

The OpenAPI 3.2.0 specification is embedded and served directly by the API server:

| Format / View | Endpoint | Content-Type | Description |
|---|---|---|---|
| **Interactive Docs** | `GET /docs` | `text/html` | Modern interactive API documentation powered by Scalar |
| **OpenAPI YAML** | `GET /openapi.yaml` | `application/yaml` | Complete OpenAPI 3.2.0 specification in YAML format |
| **OpenAPI JSON** | `GET /openapi.json` | `application/json` | Complete OpenAPI 3.2.0 specification in JSON format |

The raw specification file is also tracked in the repository at [`openapi.yaml`](./openapi.yaml).


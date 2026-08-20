# Realm API Specification

OpenAPI 3.2.0 compliant specification and route documentation for Realm API.

---

## Table of Contents

- [Overview](#overview)
- [Base URLs](#base-urls)
- [Authentication](#authentication)
- [Error Format](#error-format)
- [Endpoints](#endpoints)
  - [Root](#root)
  - [LastFM](#lastfm)
  - [Contact](#contact)
  - [Storage](#storage)
- [OpenAPI 3.2.0 Specification](#openapi-320-specification)

---

## Overview

Realm API is a RESTful backend service built with Go and Fiber v2. It provides endpoints for LastFM profile and scrobble data, contact form submission with PostgreSQL persistence and notifications, and Zstandard-compressed file storage with automatic Blurhash calculation and on-the-fly WebP conversion.

---

## Base URLs

- **Production**: `https://api.irvanma.eu.org`
- **Development**: `http://localhost:8080`

---

## Authentication

| Scheme | Type | Header | Description |
|---|---|---|---|
| `ApiKeyAuth` | API Key | `X-API-Key: <key>` | Used for storage upload and delete operations |
| `BearerAuth` | HTTP Bearer | `Authorization: Bearer <key>` | Alternative API key format |

Read-only endpoints (`GET /`, `GET /v1/lastfm/*`, `GET /v1/storage/*`) do not require authentication.

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

### Root

#### `GET /`
Health check and service status.

- **Request**: No parameters or headers required.
- **Response (`200 OK`)**:
  ```json
  {
    "message": "Nothing to see here",
    "status": "success"
  }
  ```

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


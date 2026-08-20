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

## OpenAPI 3.2.0 Specification

The following is the complete, valid OpenAPI 3.2.0 definition in YAML format:

```yaml
openapi: 3.2.0
$self: https://api.irvanma.eu.org/openapi.yaml
info:
  title: Realm API
  description: High-performance backend API service for Realm built with Go, Fiber v2, and PostgreSQL.
  version: 1.0.0
  contact:
    name: Irvan Malik Azantha
    url: https://irvanma.eu.org
    email: irvanma@gnuweeb.org
  license:
    name: RCCL-1.0
    url: https://github.com/irvanmalik48/realm-api/blob/main/LICENSE

servers:
  - url: https://api.irvanma.eu.org
    description: Production Server
  - url: http://localhost:8080
    description: Local Development Server

tags:
  - name: Health
    summary: Health check and status
    description: Core service health and uptime endpoints.
  - name: LastFM
    summary: LastFM Scrobble and User Data
    description: Proxied, cached endpoints for LastFM track scrobbles and user statistics.
  - name: Contact
    summary: Contact Form Submissions
    description: Contact message ingestion with CSRF protection, rate limiting, and instant notifications.
  - name: Storage
    summary: Zstandard Compressed File Storage
    description: High-performance file storage with Zstd compression, Blurhash analysis, and endpoint-level WebP conversion.

paths:
  /:
    get:
      summary: Health check / Root endpoint
      operationId: getRoot
      tags:
        - Health
      responses:
        '200':
          description: Service is healthy
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/RootResponse'

  /v1/lastfm/track:
    get:
      summary: Get recent LastFM tracks
      operationId: getLastFMRecentTracks
      tags:
        - LastFM
      parameters:
        - name: username
          in: query
          required: true
          description: LastFM username
          schema:
            type: string
            example: irvanmalik48
        - name: limit
          in: query
          required: false
          description: Number of tracks to retrieve (1-200)
          schema:
            type: integer
            default: 1
            minimum: 1
            maximum: 200
            example: 8
      responses:
        '200':
          description: List of recent tracks
          headers:
            Cache-Control:
              schema:
                type: string
                example: public, s-maxage=900, stale-while-revalidate=1800
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LastFMTrackResponse'
        '400':
          description: Missing or invalid parameters
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Server configuration error or missing API key
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '502':
          description: Upstream LastFM gateway error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

  /v1/lastfm/user:
    get:
      summary: Get LastFM user profile
      operationId: getLastFMUserInfo
      tags:
        - LastFM
      parameters:
        - name: username
          in: query
          required: true
          description: LastFM username
          schema:
            type: string
            example: irvanmalik48
      responses:
        '200':
          description: User profile and statistics
          headers:
            Cache-Control:
              schema:
                type: string
                example: public, s-maxage=900, stale-while-revalidate=1800
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LastFMUserResponse'
        '400':
          description: Missing or invalid parameters
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: User not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '502':
          description: Upstream LastFM gateway error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

  /v1/contact:
    post:
      summary: Submit contact form message
      operationId: submitContactMessage
      tags:
        - Contact
      parameters:
        - name: X-Realm-Request
          in: header
          required: false
          description: Security header for CSRF protection
          schema:
            type: string
            example: '1'
        - name: X-Requested-With
          in: header
          required: false
          description: Alternative CSRF protection header
          schema:
            type: string
            example: XMLHttpRequest
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ContactRequest'
      responses:
        '200':
          description: Message accepted and persisted
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ContactResponse'
        '400':
          description: Validation error or missing security headers
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '403':
          description: CSRF origin validation forbidden
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '429':
          description: Rate limit exceeded (5 requests per 10 minutes)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Database persistence error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

  /v1/storage/upload:
    post:
      summary: Upload and compress file
      operationId: uploadFile
      tags:
        - Storage
      security:
        - ApiKeyAuth: []
        - BearerAuth: []
        - {}
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              required:
                - file
              properties:
                file:
                  type: string
                  format: binary
                  description: The file to upload and compress with Zstd
      responses:
        '201':
          description: File uploaded, compressed, and registered
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileUploadResponse'
        '400':
          description: No file uploaded or corrupted payload
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: Unauthorized - missing or invalid storage API key
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '413':
          description: File size exceeds MAX_UPLOAD_SIZE_MB
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Storage or compression processing error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

  /v1/storage/{id}:
    get:
      summary: Fetch stored file or convert to WebP
      operationId: getFile
      tags:
        - Storage
      parameters:
        - name: id
          in: path
          required: true
          description: File UUID identifier
          schema:
            type: string
            format: uuid
            example: 7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b
        - name: format
          in: query
          required: false
          description: Request on-the-fly image conversion to WebP
          schema:
            type: string
            enum: [webp]
        - name: download
          in: query
          required: false
          description: Force attachment download disposition
          schema:
            type: boolean
            default: false
        - name: If-None-Match
          in: header
          required: false
          description: ETag value for cache validation
          schema:
            type: string
      responses:
        '200':
          description: Decompressed file stream
          headers:
            Content-Type:
              schema:
                type: string
                example: image/webp
            Content-Disposition:
              schema:
                type: string
                example: inline; filename="wallpaper.webp"
            ETag:
              schema:
                type: string
                example: '"5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8"'
            Cache-Control:
              schema:
                type: string
                example: public, max-age=31536000, immutable
            X-Blurhash:
              schema:
                type: string
                example: LEHV6nWB2yk8pyo0adR*.7kCMdnj
            X-Image-Width:
              schema:
                type: integer
                example: 1920
            X-Image-Height:
              schema:
                type: integer
                example: 1080
          content:
            application/octet-stream:
              schema:
                type: string
                format: binary
            image/*:
              schema:
                type: string
                format: binary
        '304':
          description: Resource Not Modified (ETag cache hit)
        '400':
          description: Invalid UUID format
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: File not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

    delete:
      summary: Delete stored file
      operationId: deleteFile
      tags:
        - Storage
      security:
        - ApiKeyAuth: []
        - BearerAuth: []
        - {}
      parameters:
        - name: id
          in: path
          required: true
          description: File UUID identifier
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: File and metadata successfully deleted
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
                    example: File deleted successfully
                  status:
                    type: string
                    example: success
        '401':
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: File not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

  /v1/storage/{id}/info:
    get:
      summary: Get file metadata and Blurhash
      operationId: getFileInfo
      tags:
        - Storage
      parameters:
        - name: id
          in: path
          required: true
          description: File UUID identifier
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: File metadata details
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileInfoResponse'
        '400':
          description: Invalid UUID format
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: File record not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
      description: API Key passed via X-API-Key header
    BearerAuth:
      type: http
      scheme: bearer
      description: API Key passed via Bearer token

  schemas:
    RootResponse:
      type: object
      required:
        - message
        - status
      properties:
        message:
          type: string
          example: Nothing to see here
        status:
          type: string
          example: success

    ErrorResponse:
      type: object
      required:
        - error
        - status
      properties:
        error:
          type: string
          example: Invalid parameters
        status:
          type: string
          example: error

    ContactRequest:
      type: object
      required:
        - name
        - email
        - subject
        - message
      properties:
        name:
          type: string
          minLength: 2
          maxLength: 100
          example: Jane Doe
        email:
          type: string
          format: email
          maxLength: 254
          example: jane@example.com
        subject:
          type: string
          minLength: 3
          maxLength: 200
          example: Project Collaboration
        message:
          type: string
          minLength: 10
          maxLength: 5000
          example: Hello, I would like to discuss a project with you.
        _gotcha:
          type: string
          description: Honeypot field (must remain empty for human submissions)
          example: ""

    ContactResponse:
      type: object
      required:
        - message
        - status
      properties:
        message:
          type: string
          example: Your message has been sent successfully.
        status:
          type: string
          example: success

    FileDTO:
      type: object
      required:
        - id
        - filename
        - content_type
        - original_size
        - compressed_size
        - savings_percent
        - sha256
        - url
        - created_at
      properties:
        id:
          type: string
          format: uuid
          example: 7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b
        filename:
          type: string
          example: wallpaper.png
        content_type:
          type: string
          example: image/png
        original_size:
          type: integer
          format: int64
          example: 2450000
        compressed_size:
          type: integer
          format: int64
          example: 1120000
        savings_percent:
          type: number
          format: float
          example: 54.28
        sha256:
          type: string
          example: 5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8
        blurhash:
          type: string
          nullable: true
          example: LEHV6nWB2yk8pyo0adR*.7kCMdnj
        width:
          type: integer
          nullable: true
          example: 1920
        height:
          type: integer
          nullable: true
          example: 1080
        url:
          type: string
          example: /v1/storage/7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b
        webp_url:
          type: string
          nullable: true
          example: /v1/storage/7fa84e72-d7b1-4bb2-b6be-4b95d0ef923b?format=webp
        created_at:
          type: string
          format: date-time
          example: '2026-08-20T12:00:00Z'

    FileUploadResponse:
      type: object
      required:
        - status
        - message
        - file
      properties:
        status:
          type: string
          example: success
        message:
          type: string
          example: File uploaded and compressed successfully
        file:
          $ref: '#/components/schemas/FileDTO'

    FileInfoResponse:
      type: object
      required:
        - status
        - file
      properties:
        status:
          type: string
          example: success
        file:
          $ref: '#/components/schemas/FileDTO'

    LastFMTrackResponse:
      type: object
      properties:
        recenttracks:
          type: object
          properties:
            track:
              type: array
              items:
                type: object
                properties:
                  name:
                    type: string
                  artist:
                    type: object
                    properties:
                      '#text':
                        type: string
                  album:
                    type: object
                    properties:
                      '#text':
                        type: string
                  url:
                    type: string
                  image:
                    type: array
                    items:
                      type: object
                      properties:
                        size:
                          type: string
                        '#text':
                          type: string

    LastFMUserResponse:
      type: object
      properties:
        user:
          type: object
          properties:
            name:
              type: string
            playcount:
              type: string
            artist_count:
              type: string
            track_count:
              type: string
            album_count:
              type: string
            url:
              type: string
```

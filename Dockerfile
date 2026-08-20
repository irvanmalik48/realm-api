# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies & SSL certs
RUN apk add --no-cache ca-certificates git tzdata

WORKDIR /src

# Leverage caching for dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and compile
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /bin/server ./cmd/server

# Final runtime stage
FROM alpine:3.21

# Install certificates for HTTPS requests to external APIs (LastFM)
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy compiled binary from builder stage
COPY --from=builder /bin/server /app/server

# Use non-privileged user for security
USER appuser:appgroup

# Expose default HTTP port
EXPOSE 8080

# Run server
ENTRYPOINT ["/app/server"]

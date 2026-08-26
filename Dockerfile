# Build stage
FROM golang:alpine AS builder

# Install build dependencies & SSL certs
RUN apk add --no-cache ca-certificates git tzdata

WORKDIR /src

ENV GOTOOLCHAIN=auto

# Leverage caching for dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and compile
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /bin/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /bin/token ./cmd/token

# Final runtime stage
FROM alpine:3.21

# Install certificates for HTTPS requests, tzdata, and su-exec for privilege dropping
RUN apk add --no-cache ca-certificates tzdata su-exec && \
    addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup && \
    mkdir -p /data/storage /app && \
    chown -R appuser:appgroup /data/storage /app

WORKDIR /app

# Copy compiled binaries and entrypoint script
COPY --from=builder /bin/server /app/server
COPY --from=builder /bin/token /app/token
COPY docker-entrypoint.sh /app/docker-entrypoint.sh

RUN chmod +x /app/docker-entrypoint.sh

# Expose default HTTP port
EXPOSE 8080

# Run entrypoint script which fixes volume permissions and drops privileges to appuser
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/server"]

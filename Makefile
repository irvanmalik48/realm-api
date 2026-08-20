.PHONY: dev build test tidy clean docker-build docker-up docker-down

dev:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o bin/server ./cmd/server

test:
	go test -v ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

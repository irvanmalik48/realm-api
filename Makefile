.PHONY: dev build test tidy clean proto docker-build docker-up docker-down

proto:
	@export PATH="$$(go env GOPATH)/bin:$$PATH"; \
	protoc --proto_path=. --go_out=. --go_opt=module=github.com/irvanmalik48/realm-api --go-grpc_out=. --go-grpc_opt=module=github.com/irvanmalik48/realm-api proto/realm/v1/*.proto


dev:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o bin/server ./cmd/server
	CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o bin/token ./cmd/token

test:
	go test -v ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/

docker-build:
	docker compose build

docker-up:
	mkdir -p ./data/storage
	docker compose up -d

docker-down:
	docker compose down

.PHONY: run build tidy test test-short lint fmt-check verify migrate-up migrate-down openapi-check backup restore-check

DB_URL ?= postgres://nusantara:nusantara@localhost:5432/nusantara?sslmode=disable

run:
	go run ./cmd/api

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/api ./cmd/api

tidy:
	go mod tidy

test:
	go test -race ./...

test-short:
	go test -race -short ./...

lint:
	golangci-lint run

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files:" && gofmt -l . && exit 1)

verify: fmt-check
	go vet ./...
	go test -short ./...

migrate-up:
	go run github.com/pressly/goose/v3/cmd/goose@v3.24.1 -dir migrations postgres "$(DB_URL)" up

migrate-down:
	go run github.com/pressly/goose/v3/cmd/goose@v3.24.1 -dir migrations postgres "$(DB_URL)" down

# Every docs/api/openapi.yaml operation is routed, secured ones answer 401 anonymously, and every
# operation with a response body has a fixture. Needs Docker (Testcontainers).
openapi-check:
	go test -count=1 -run 'OpenAPI|Every' ./internal/adapters/primary/rest/router/

# See README.md "Backups" for the environment both scripts read.
backup:
	./scripts/backup.sh

restore-check:
	./scripts/restore_check.sh

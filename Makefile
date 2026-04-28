.PHONY: run build test lint docker-up docker-build scrape

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

run:
	go run ./cmd/watchdog/
build:
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/watchdog ./cmd/watchdog/
test:
	go test -race github.com/maxlesscode/watchdog/internal/...
lint:
	go vet github.com/maxlesscode/watchdog/internal/... && golangci-lint run ./internal/...
docker-up:
	docker compose up -d
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t watchdog .
scrape:
	curl -s -X POST http://localhost:9999/admin/scrape -H "X-API-Key: $$API_KEY"

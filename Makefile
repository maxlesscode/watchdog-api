.PHONY: run build test lint

run:
	go run ./cmd/watchdog/
build:
	go build -o bin/watchdog ./cmd/watchdog/
test:
	go test -race ./...
lint:
	golangci-lint run ./internal/...

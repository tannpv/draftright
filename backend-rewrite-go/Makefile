# Local-dev convenience targets. CI runs raw `go test` / `docker build`.

.PHONY: help dev build test fmt vet lint clean tidy

help:
	@echo 'Targets:'
	@echo '  make dev     — build + run service via docker compose'
	@echo '  make build   — go build ./...'
	@echo '  make test    — go test ./... -race -count=1'
	@echo '  make fmt     — gofmt -s -w + goimports'
	@echo '  make vet     — go vet ./...'
	@echo '  make tidy    — go mod tidy'
	@echo '  make clean   — docker compose down + remove build artifacts'

dev:
	docker compose -f docker-compose.dev.yml up --build

build:
	go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

test:
	# -race catches data races at test time. -count=1 disables the test
	# cache so flake-prone tests don't hide behind stale results.
	go test ./... -race -count=1

fmt:
	gofmt -s -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	docker compose -f docker-compose.dev.yml down -v --remove-orphans
	rm -rf bin/

.PHONY: dev lint fmt test build \
        up up-optimized down logs ps

init:
	go mod download
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

dev:
	air

lint:
	golangci-lint-v2 run

test:
	go test -v ./...

build:
	go build -o bin/app cmd/server/main.go

up:
	cp .env.baseline.example .env
	docker compose up -d --build

up-optimized:
	cp .env.optimized.example .env
	docker compose --profile optimized up -d --build

down:
	docker compose --profile optimized down

logs:
	docker compose logs -f

ps:
	docker compose ps

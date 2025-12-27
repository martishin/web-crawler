.PHONY: test tidy tools deps sqlc-generate db-up db-down migrate-up migrate-down run-api crawl start-all stop-all logs

GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

SQLC := $(GOBIN)/sqlc
MIGRATE := $(GOBIN)/migrate

test:
	go test ./...

tools:
	test -x $(SQLC) || go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
	test -x $(MIGRATE) || go install github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1

tidy:
	go mod tidy

deps: tidy
	go mod download

sqlc-generate: tools
	$(SQLC) generate

db-up:
	docker compose up -d db

db-down:
	docker compose down

# Uses PG_DSN from your environment or .env (godotenv/autoload doesn't apply to Make)
migrate-up: tools
	$(MIGRATE) -path migrations -database "$$PG_DSN" up

migrate-down: tools
	$(MIGRATE) -path migrations -database "$$PG_DSN" down 1

run-api: deps sqlc-generate
	go run ./cmd/api

crawl: deps sqlc-generate
	go run ./cmd/crawler --once

start-all:
	docker compose up -d

stop-all:
	docker compose down

logs:
	docker compose logs -f

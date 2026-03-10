.PHONY: migrate-up migrate-down migrate-create schema-dump verify-openai test lint run build

# Build date for run/build. Injects into /dev/token and admin login page.
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-X github.com/jpfortier/gym-app/internal/env.buildDate=$(BUILD_DATE)"

lint:
	$(shell go env GOPATH)/bin/golangci-lint run

# Run migrations. Start proxy first: fly proxy 15432:5432 -a gym-app-pg
# Requires pgvector: enable in Fly dashboard (PostgreSQL Extensions) for migration 000008.
# Uses .env if present. All gym env vars use GYM_ prefix to avoid collisions with other projects.
MIGRATE := $(shell which migrate 2>/dev/null || echo "$(shell go env GOPATH)/bin/migrate")
migrate-up:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	$(MIGRATE) -path migrations -database "$${GYM_DATABASE_URL:-$$DATABASE_URL}" up

migrate-down:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	$(MIGRATE) -path migrations -database "$${GYM_DATABASE_URL:-$$DATABASE_URL}" down

# Dump current schema to docs/schema.sql. Run after migrations. Uses .env for GYM_DATABASE_URL.
# Requires pg_dump version >= Postgres server (e.g. brew install postgresql@17 for Fly Postgres 17).
schema-dump:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	pg_dump -d "$${GYM_DATABASE_URL:-$$DATABASE_URL}" --schema-only --no-owner --no-privileges -f docs/schema.sql && \
	echo "Wrote docs/schema.sql" || (echo "pg_dump failed (version mismatch? need pg_dump >= server)"; exit 1)

# Run tests. Uses .env for GYM_* vars.
test:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	go test ./... -count=1

# Run API server. Uses .env. Injects build date for /dev/token and admin login.
run:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	go run $(LDFLAGS) ./cmd/api

# Build API binary with build date.
build:
	go build $(LDFLAGS) -o gym-api ./cmd/api

# Verify OpenAI API key. Create at https://platform.openai.com/api-keys
verify-openai:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	if [ -z "$$GYM_OPENAI_API_KEY" ]; then echo "GYM_OPENAI_API_KEY not set. Add to .env"; exit 1; fi; \
	curl -sS -H "Authorization: Bearer $$GYM_OPENAI_API_KEY" https://api.openai.com/v1/models | head -c 200; \
	echo ""; echo "If you see JSON above, key works."

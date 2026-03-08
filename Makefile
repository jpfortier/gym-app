.PHONY: migrate-up migrate-down migrate-create verify-openai test

# Run migrations. Start proxy: fly proxy 15432:5432 -a gym-app-pg
# Requires pgvector: enable in Fly dashboard (PostgreSQL Extensions) for migration 000008.
# Uses .env if present (so gym DATABASE_URL overrides shell env from other projects).
# Or: DATABASE_URL="postgres://postgres:PASSWORD@localhost:15432/postgres?sslmode=disable" make migrate-up
MIGRATE := $(shell which migrate 2>/dev/null || echo "$(shell go env GOPATH)/bin/migrate")
migrate-up:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	$(MIGRATE) -path migrations -database "$${DATABASE_URL}" up

migrate-down:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	$(MIGRATE) -path migrations -database "$${DATABASE_URL}" down

# Run tests. Uses .env for DATABASE_URL (overrides shell env).
test:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	go test ./... -count=1

# Verify OpenAI API key. Create at https://platform.openai.com/api-keys
verify-openai:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	if [ -z "$$OPENAI_API_KEY" ]; then echo "OPENAI_API_KEY not set. Add to .env"; exit 1; fi; \
	curl -sS -H "Authorization: Bearer $$OPENAI_API_KEY" https://api.openai.com/v1/models | head -c 200; \
	echo ""; echo "If you see JSON above, key works."

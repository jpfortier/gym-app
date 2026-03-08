.PHONY: migrate-up migrate-down migrate-create

# Run migrations. Start proxy: fly proxy 15432:5432 -a gym-app-pg
# Uses .env if present (so gym DATABASE_URL overrides shell env from other projects).
# Or: DATABASE_URL="postgres://postgres:PASSWORD@localhost:15432/postgres?sslmode=disable" make migrate-up
migrate-up:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	migrate -path migrations -database "$${DATABASE_URL}" up

migrate-down:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	migrate -path migrations -database "$${DATABASE_URL}" down

# DongDo_CS_V2 — Docker & migrations helper

.PHONY: up down logs ps restart sync-init rebuild up-fresh migrate-status migrate-create

# =============================================================================
# Asterisk-related
# =============================================================================
ASTERISK_SVC := asterisk
ASTERISK_CONTAINER := dongdo_asterisk
ASTERISK_DOCKER := docker/asterisk

# =============================================================================
# Migrations (goose — embedded in the server binary)
# =============================================================================
# goose CLI (from tools/bin/) for manual migration control.
# Usage: make migrate-create name=my_new_migration
GOOSE       := ./tools/bin/goose
GOOSE_DSN   := "postgres://postgres:postgrespassword@localhost:5433/dongdo_cs?sslmode=disable"

# Create a new migration skeleton.
migrate-create:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create name=add_new_column"; exit 1; fi
	$(GOOSE) create $(name) sql

# Run pending migrations (app does this automatically on startup).
migrate-up:
	$(GOOSE) -dir internal/repository/postgres/migrations postgres "$(GOOSE_DSN)" up

# Roll back the last migration (use with care in production).
migrate-down:
	$(GOOSE) -dir internal/repository/postgres/migrations postgres "$(GOOSE_DSN)" down

# Show migration status.
migrate-status:
	$(GOOSE) -dir internal/repository/postgres/migrations postgres "$(GOOSE_DSN)" status

# =============================================================================
# Docker / Compose helpers
# =============================================================================

# Sync migration files from db/migrations → db/init (consumed by Postgres initdb
# for fresh volumes). Goose migrations are self-contained — each file has both
# UP and DOWN blocks.
sync-init:
	@mkdir -p db/init
	@rm -f db/init/*.sql
	@cp db/migrations/*.sql db/init/
	@echo "✅ Synced $$(ls db/init/*.sql 2>/dev/null | wc -l | tr -d ' ') migrations to db/init/"

# Bring up the full stack (postgres, redis, qdrant, server, web, asterisk).
up:
	docker compose up -d --build

# Up WITHOUT rebuilding the image (faster iteration if only db/init changed).
up-fast:
	docker compose up -d

# Wipe Postgres volume and bring the stack back up — full re-init from
# db/init/*.sql. USE WITH CARE: drops all data.
up-fresh: down
	docker volume rm dongdo_cs_v2_pgdata 2>/dev/null || true
	$(MAKE) up

down:
	docker compose down

logs:
	docker compose logs -f

ps:
	docker compose ps

restart:
	docker compose restart

# Rebuild only the server image after Go code changes.
rebuild:
	docker compose build server
	docker compose up -d server

# =============================================================================
# Asterisk PBX targets (NEW)
# =============================================================================

# Build only the asterisk container
asterisk-build:
	docker compose build $(ASTERISK_SVC)

# (Re-)Start only asterisk
asterisk-up:
	docker compose up -d $(ASTERISK_SVC)

# Tail asterisk logs
asterisk-logs:
	docker compose logs -f $(ASTERISK_SVC)

# Open the asterisk CLI (`exit` to leave)
asterisk-cli:
	docker exec -it $(ASTERISK_CONTAINER) asterisk -rvvv

# Quick health check (without entering CLI)
asterisk-health:
	docker exec $(ASTERISK_CONTAINER) asterisk -rx "core show version"
	@docker exec $(ASTERISK_CONTAINER) asterisk -rx "pjsip show endpoints" 2>&1 | head -10

# Restart only asterisk (config change pickup)
asterisk-restart:
	docker compose restart $(ASTERISK_SVC)

# Reload chan_pjsip only (fast, no channel teardown)
asterisk-reload-pjsip:
	docker exec $(ASTERISK_CONTAINER) asterisk -rx "module reload res_pjsip.so"

# Force-rebuild asterisk image from scratch
asterisk-rebuild:
	docker compose build --no-cache $(ASTERISK_SVC)
	docker compose up -d $(ASTERISK_CONTAINER)

# Run E2E smoke tests against a running asterisk container
test-call:
	./scripts/test_call.sh

# Wait until asterisk becomes healthy (used in CI / deploys)
wait-asterisk:
	./scripts/wait-asterisk.sh

# Clean up asterisk recordings + logs (preserve config)
asterisk-clean:
	docker compose stop $(ASTERISK_SVC) || true
	docker volume rm dongdo-cskh_asterisk-spool 2>/dev/null || true
	@echo "Volumes removed. Bring asterisk back with: make asterisk-up"

# Add an extra agent extension on a running stack
#   make register-agent ext=1006
register-agent:
	@if [ -z "$(ext)" ]; then echo "Usage: make register-agent ext=1006"; exit 2; fi
	./scripts/register-agent.sh $(ext)

# =============================================================================
# Dev shortcuts
# =============================================================================

# Run the server binary locally (requires Postgres on port 5433).
run:
	go run ./cmd/server

# Generate sqlc code from db/queries/*.sql → internal/repository/sqlcdb/
sqlc-gen:
	./tools/bin/sqlc generate

# Build the server binary.
build:
	go build -o server_new ./cmd/server

# Full dev workflow: sqlc generate → build → run.
dev: sqlc-gen build
	./server_new
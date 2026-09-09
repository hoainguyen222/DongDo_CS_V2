# DongDo_CS_V2 — Docker & migrations helper

.PHONY: up down logs ps restart sync-init rebuild up-fresh migrate-status migrate-create \
        monitoring-up monitoring-down monitoring-logs monitoring-status monitoring-config \
        metrics pprof-cpu pprof-heap pprof-goroutine pprof-mutex pprof-block health

# =============================================================================
# Monitoring & Observability
# =============================================================================
# The monitoring stack (Prometheus, Grafana, exporters) lives behind the
# `monitoring` Docker Compose profile so `make up` does NOT start it.
#
# Recommended workflow:
#   1. `make up`              — brings up app stack (creates dongdo_cs_v2_default)
#   2. `make monitoring-up`   — adds monitoring stack on top
#   3. `make monitoring-down` — tears monitoring back down (app untouched)
#
# Standalone (no app stack):
#   `make monitoring-up STANDALONE=1`  — creates a fallback bridge `dongdo_app`
#                                         so exporters can still resolve
#                                         `postgres` and `redis` service names.
#
# All exporter ports are bound to the private `dongdo-monitoring` Docker
# network. Grafana is exposed on host port 3050 by default (not 3000, to
# avoid colliding with Next.js dev server); change via GRAFANA_PORT env.

MONITORING_PROJECT := dongdo_cs_v2

# Bring up the monitoring stack on top of the app stack. Auto-creates the
# app network if it doesn't exist yet (so you can `make monitoring-up`
# before `make up` without a hard error).
# Uses --env-file .env.monitoring so Grafana credentials and Prometheus port
# are read from the monitoring-specific env file (not the app's .env).
monitoring-up:
	@echo "🔌 Ensuring dongdo_cs_v2_default network exists..."
	@docker network inspect dongdo_cs_v2_default >/dev/null 2>&1 || \
	  (echo "  ⚠️  dongdo_cs_v2_default missing — creating empty bridge." && \
	   echo "     (Run \`make up\` first so postgres/redis containers resolve.)" && \
	   docker network create dongdo_cs_v2_default >/dev/null)
	@echo "🚀 Starting monitoring stack..."
	docker compose -p $(MONITORING_PROJECT) \
	  --env-file .env.monitoring \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring up -d
	@echo "✅ Monitoring stack up. Open: Grafana → http://localhost:$${GRAFANA_PORT:-3050}  Prometheus → http://localhost:$${PROMETHEUS_PORT:-9091}  Alertmanager → http://localhost:9093"

monitoring-down:
	docker compose -p $(MONITORING_PROJECT) \
	  --env-file .env.monitoring \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring down

monitoring-logs:
	docker compose -p $(MONITORING_PROJECT) \
	  --env-file .env.monitoring \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring logs -f

monitoring-status:
	docker compose -p $(MONITORING_PROJECT) \
	  --env-file .env.monitoring \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring ps

monitoring-config:
	docker compose -p $(MONITORING_PROJECT) \
	  --env-file .env.monitoring \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring config

# Hit the running server's /metrics endpoint directly. Default address
# matches the default METRICS_ADDR (127.0.0.1:9090). Override:
#   make metrics METRICS_URL=http://localhost:9090/metrics
METRICS_URL ?= http://127.0.0.1:9090/metrics
metrics:
	@curl -fsS "$(METRICS_URL)" | head -50

# Health probe (uses the public /health endpoint on port 8080).
HEALTH_URL ?= http://127.0.0.1:8080/health
health:
	@curl -fsS "$(HEALTH_URL)" && echo

# ----- pprof shortcuts -----
# Default pprof address matches PPROF_ADDR (127.0.0.1:6060). To profile a
# remote host, SSH-tunnel first:
#   ssh -L 6060:127.0.0.1:6060 user@prod-host
# Then run e.g.:
#   make pprof-cpu PPROF_URL=http://localhost:6060
#
# These targets shell out to `go tool pprof`, which must be on PATH.
PPROF_URL ?= http://127.0.0.1:6060

pprof-cpu:
	go tool pprof -seconds=30 "$(PPROF_URL)/debug/pprof/profile"

pprof-heap:
	go tool pprof "$(PPROF_URL)/debug/pprof/heap"

pprof-goroutine:
	go tool pprof "$(PPROF_URL)/debug/pprof/goroutine"

pprof-mutex:
	go tool pprof -seconds=10 "$(PPROF_URL)/debug/pprof/mutex"

pprof-block:
	go tool pprof -seconds=10 "$(PPROF_URL)/debug/pprof/block"

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

# Bring up the full stack. The server binary runs goose migrations on startup.
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
# Dev shortcuts
# =============================================================================

# Run the server binary locally (requires Postgres on port 5433).
run:
	go run ./cmd/server

# Generate sqlc code from db/queries/*.sql → internal/repository/sqlcdb/
sqlc-gen:
	./tools/bin/sqlc generate

# Build server & ingest binaries into ./bin/ (gitignored).
# Running alongside a previously-started process (which still holds the old
# binary's inode) is safe — `./bin/server` is replaced atomically by Go.
build:
	@mkdir -p ./bin
	go build -o ./bin/server ./cmd/server
	go build -o ./bin/ingest ./cmd/ingest

# Full dev workflow: sqlc generate → build → run.
dev: sqlc-gen build
	./bin/server

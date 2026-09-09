# DongDo_CS_V2 — Docker & migrations helper

.PHONY: help up down logs ps restart sync-init rebuild up-fresh up-fast migrate-status migrate-create \
        monitoring-up monitoring-down monitoring-logs monitoring-status monitoring-config \
        metrics pprof-cpu pprof-heap pprof-goroutine pprof-mutex pprof-block health \
        run sqlc-gen build dev \
        web-install web-dev web-build web-start web-lint web-clean web-docker-build web-docker-build-dev \
        web-docker-run web-docker-run-dev \
        web-env-dev web-env-prod web-env-local web-env-list \
        docker-nginx-build docker-nginx-up docker-nginx-down docker-nginx-logs docker-nginx-reload \
        docker-nginx-config-test docker-nginx-shell \
        mon-up mon-down mon-restart mon-logs mon-ps mon-status \
        mon-reload-prom mon-check-targets mon-check-exporters \
        mon-check-server-metrics mon-validate-prom

.DEFAULT_GOAL := help

# =============================================================================
# Help
# =============================================================================
help: ## Show this help message (auto-extracted from `##` doc comments)
	@echo ""
	@echo "🏠  DongDo_CS_V2 — root Makefile"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} \
	  /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}' \
	  $(MAKEFILE_LIST)
	@echo ""

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
monitoring-up: ## Add the monitoring stack on top of the running app stack
	@echo "🔌 Ensuring dongdo_cs_v2_default network exists..."
	@docker network inspect dongdo_cs_v2_default >/dev/null 2>&1 || \
	  (echo "  ⚠️  dongdo_cs_v2_default missing — creating empty bridge." && \
	   echo "     (Run \`make up\` first so postgres/redis containers resolve.)" && \
	   docker network create dongdo_cs_v2_default >/dev/null)
	@echo "🚀 Starting monitoring stack..."
	docker compose -p $(MONITORING_PROJECT) \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring up -d
	@echo "✅ Monitoring stack up. Open: Grafana → http://localhost:$${GRAFANA_PORT:-3050}  Prometheus → http://localhost:9090  Alertmanager → http://localhost:9093"

monitoring-down: ## Tear down the monitoring stack (app stack untouched)
	docker compose -p $(MONITORING_PROJECT) \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring down

monitoring-logs: ## Tail monitoring stack logs (follow)
	docker compose -p $(MONITORING_PROJECT) \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring logs -f

monitoring-status: ## List monitoring stack containers
	docker compose -p $(MONITORING_PROJECT) \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring ps

monitoring-config: ## Render the merged monitoring compose config
	docker compose -p $(MONITORING_PROJECT) \
	  -f docker-compose.yml \
	  -f monitoring/docker-compose.monitoring.yml \
	  --profile monitoring config

# Hit the running server's /metrics endpoint directly. Default address
# matches the default METRICS_ADDR (127.0.0.1:9090). Override:
#   make metrics METRICS_URL=http://localhost:9090/metrics
METRICS_URL ?= http://127.0.0.1:9090/metrics
metrics: ## Hit the server's /metrics endpoint (override METRICS_URL=...)
	@curl -fsS "$(METRICS_URL)" | head -50

# Health probe (uses the public /health endpoint on port 8080).
HEALTH_URL ?= http://127.0.0.1:8080/health
health: ## Probe the public /health endpoint on port 8080 (override HEALTH_URL=...)
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

pprof-cpu: ## 30s CPU profile via go tool pprof (override PPROF_URL=...)
	go tool pprof -seconds=30 "$(PPROF_URL)/debug/pprof/profile"

pprof-heap: ## Heap profile via go tool pprof
	go tool pprof "$(PPROF_URL)/debug/pprof/heap"

pprof-goroutine: ## Goroutine profile via go tool pprof
	go tool pprof "$(PPROF_URL)/debug/pprof/goroutine"

pprof-mutex: ## 10s mutex profile via go tool pprof
	go tool pprof -seconds=10 "$(PPROF_URL)/debug/pprof/mutex"

pprof-block: ## 10s block profile via go tool pprof
	go tool pprof -seconds=10 "$(PPROF_URL)/debug/pprof/block"

# =============================================================================
# Migrations (goose — embedded in the server binary)
# =============================================================================
# goose CLI (from tools/bin/) for manual migration control.
# Usage: make migrate-create name=my_new_migration
GOOSE       := ./tools/bin/goose
GOOSE_DSN   := "postgres://postgres:postgrespassword@localhost:5433/dongdo_cs?sslmode=disable"

# Create a new migration skeleton.
migrate-create: ## Create a new SQL migration (Usage: make migrate-create name=foo)
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create name=add_new_column"; exit 1; fi
	$(GOOSE) create $(name) sql

# Run pending migrations (app does this automatically on startup).
migrate-up: ## Apply all pending migrations
	$(GOOSE) -dir internal/repository/postgres/migrations postgres "$(GOOSE_DSN)" up

# Roll back the last migration (use with care in production).
migrate-down: ## Roll back the most recent migration
	$(GOOSE) -dir internal/repository/postgres/migrations postgres "$(GOOSE_DSN)" down

# Show migration status.
migrate-status: ## Show applied/pending migration status
	$(GOOSE) -dir internal/repository/postgres/migrations postgres "$(GOOSE_DSN)" status

# =============================================================================
# Docker / Compose helpers
# =============================================================================

# Sync migration files from db/migrations → db/init (consumed by Postgres initdb
# for fresh volumes). Goose migrations are self-contained — each file has both
# UP and DOWN blocks.
sync-init: ## Sync db/migrations/*.sql → db/init/*.sql (for fresh Postgres volumes)
	@mkdir -p db/init
	@rm -f db/init/*.sql
	@cp db/migrations/*.sql db/init/
	@echo "✅ Synced $$(ls db/init/*.sql 2>/dev/null | wc -l | tr -d ' ') migrations to db/init/"

# Bring up the full stack. The server binary runs goose migrations on startup.
up: ## Build and start the full app stack
	docker compose up -d --build

# Up WITHOUT rebuilding the image (faster iteration if only db/init changed).
up-fast: ## Start the stack without rebuilding images
	docker compose up -d

# Wipe Postgres volume and bring the stack back up — full re-init from
# db/init/*.sql. USE WITH CARE: drops all data.
up-fresh: down ## Wipe Postgres volume and re-init the stack from scratch
	docker volume rm dongdo_cs_v2_pgdata 2>/dev/null || true
	$(MAKE) up

down: ## Stop the stack and remove containers (volumes preserved)
	docker compose down

logs: ## Tail logs for every app container (follow)
	docker compose logs -f

ps: ## List app stack containers
	docker compose ps

restart: ## Restart every app container
	docker compose restart

# Rebuild only the server image after Go code changes.
rebuild: ## Rebuild the server image and roll it
	docker compose build server
	docker compose up -d server

# =============================================================================
# Dev shortcuts
# =============================================================================

# Run the server binary locally (requires Postgres on port 5433).
run: ## Run the Go server locally (needs Postgres on :5433)
	go run ./cmd/server

# Generate sqlc code from db/queries/*.sql → internal/repository/sqlcdb/
sqlc-gen: ## Generate sqlc code from db/queries/*.sql
	./tools/bin/sqlc generate

# Build the server binary.
build: ## Compile the server binary to ./server_new
	go build -o server_new ./cmd/server

# Full dev workflow: sqlc generate → build → run.
dev: sqlc-gen build ## sqlc-gen + build + run the server locally
	./server_new

# =============================================================================
# Web (Next.js) — delegated to web/Makefile
# =============================================================================
web-install: ## [web] npm ci
	$(MAKE) -C web install

web-dev: ## [web] run Next.js dev server (HOST=0.0.0.0:3000)
	$(MAKE) -C web dev

web-build: ## [web] next build → .next/
	$(MAKE) -C web build

web-start: ## [web] next start -p 3000
	$(MAKE) -C web start

web-lint: ## [web] next lint
	$(MAKE) -C web lint

web-clean: ## [web] remove .next and .tsbuildinfo
	$(MAKE) -C web clean

web-docker-build: ## [web] build prod Docker image (dongdo-cs-web:latest)
	$(MAKE) -C web docker-build

web-docker-build-dev: ## [web] build dev Docker image (dongdo-cs-web:dev)
	$(MAKE) -C web docker-build-dev

web-docker-run: ## [web] run prod image on host port 3000
	$(MAKE) -C web docker-run

web-docker-run-dev: ## [web] run dev image with src/ bind-mounted for hot reload
	$(MAKE) -C web docker-run-dev

web-env-dev: ## [web] cp .env.development.example → .env.development
	$(MAKE) -C web env-dev

web-env-prod: ## [web] cp .env.production.example → .env.production
	$(MAKE) -C web env-prod

web-env-local: ## [web] cp .env.local.example → .env.local
	$(MAKE) -C web env-local

web-env-list: ## [web] list env-helper targets
	$(MAKE) -C web env-list

# =============================================================================
# Docker subsystem helpers (nginx)
# =============================================================================
docker-nginx-build: ## [docker] build nginx image (dongdo-cs-nginx:latest)
	$(MAKE) -C docker nginx-build

docker-nginx-up: ## [docker] start nginx container
	$(MAKE) -C docker nginx-up

docker-nginx-down: ## [docker] stop nginx container
	$(MAKE) -C docker nginx-down

docker-nginx-logs: ## [docker] tail nginx logs
	$(MAKE) -C docker nginx-logs

docker-nginx-reload: ## [docker] hot-reload nginx (nginx -s reload)
	$(MAKE) -C docker nginx-reload

docker-nginx-config-test: ## [docker] validate nginx config (nginx -t)
	$(MAKE) -C docker nginx-config-test

docker-nginx-shell: ## [docker] open bash in nginx container
	$(MAKE) -C docker nginx-shell

# =============================================================================
# Monitoring subsystem (delegated to monitoring/Makefile)
# =============================================================================
mon-up: ## [mon] bring up monitoring stack (assumes app stack running)
	$(MAKE) -C monitoring up

mon-down: ## [mon] tear down monitoring stack
	$(MAKE) -C monitoring down

mon-restart: ## [mon] restart every monitoring container
	$(MAKE) -C monitoring restart

mon-logs: ## [mon] tail monitoring logs (follow)
	$(MAKE) -C monitoring logs

mon-ps: ## [mon] list monitoring containers
	$(MAKE) -C monitoring ps

mon-status: ## [mon] container status + Prometheus/Grafana health probes
	$(MAKE) -C monitoring status

mon-reload-prom: ## [mon] hot-reload Prometheus (POST /-/reload)
	$(MAKE) -C monitoring reload-prometheus

mon-check-targets: ## [mon] show every scrape target's UP/DOWN state
	$(MAKE) -C monitoring check-targets

mon-check-exporters: ## [mon] UP/DOWN summary for every exporter
	$(MAKE) -C monitoring check-exporters

mon-check-server-metrics: ## [mon] verify server /metrics reachable from Prometheus
	$(MAKE) -C monitoring check-server-metrics

mon-validate-prom: ## [mon] validate prometheus.yml with promtool
	$(MAKE) -C monitoring validate-prometheus

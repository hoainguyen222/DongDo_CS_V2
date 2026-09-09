# =============================================================================
# DongDo_CS_V2 — Monitoring subsystem Makefile
# =============================================================================
# Standalone helper for the Prometheus / Grafana / exporters stack defined in
# monitoring/docker-compose.monitoring.yml.
#
# Assumes the app stack is already up (`make up` from project root), which
# creates the `dongdo_cs_v2_default` bridge that exporters need to resolve
# `postgres` and `redis`.
#
# This Makefile is the operational entry-point: it brings services up/down,
# tails logs, opens shells, AND — crucially — gives you a "why is my
# dashboard empty?" diagnosis with `check-targets` / `check-exporters`.
# =============================================================================

.PHONY: help up down restart logs ps status \
        prometheus-logs grafana-logs alertmanager-logs \
        postgres-exporter-logs redis-exporter-logs nginx-exporter-logs \
        node-exporter-logs \
        prometheus-shell grafana-shell alertmanager-shell \
        reload-prometheus check-targets check-exporters check-server-metrics \
        dashboard-list validate-prometheus promtool

.DEFAULT_GOAL := help

# -----------------------------------------------------------------------------
# Settings
# -----------------------------------------------------------------------------
COMPOSE_FILE_ROOT := ../docker-compose.yml
COMPOSE_FILE_MON  := docker-compose.monitoring.yml
COMPOSE_PROJECT   := dongdo_cs_v2
PROFILE           := monitoring
NETWORK_FALLBACK  := dongdo_cs_v2_default

PROM_URL          := http://localhost:9090
GRAFANA_URL       := http://localhost:$${GRAFANA_PORT:-3050}
SERVER_METRICS    := http://host.docker.internal:9090/metrics

# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------
help: ## Show this help message
	@echo ""
	@echo "📊  DongDo_CS_V2 — Monitoring subsystem Makefile"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} \
	  /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-26s\033[0m %s\n", $$1, $$2}' \
	  $(MAKEFILE_LIST)
	@echo ""

# -----------------------------------------------------------------------------
# Stack lifecycle
# -----------------------------------------------------------------------------
up: ## Bring up the monitoring stack on top of the app stack
	@echo "🔌 Ensuring $(NETWORK_FALLBACK) network exists..."
	@docker network inspect $(NETWORK_FALLBACK) >/dev/null 2>&1 || \
	  (echo "  ⚠️  $(NETWORK_FALLBACK) missing — creating empty bridge." && \
	   echo "     (Run \`make up\` first so postgres/redis resolve.)" && \
	   docker network create $(NETWORK_FALLBACK) >/dev/null)
	@echo "🚀 Starting monitoring stack (profile=$(PROFILE))..."
	docker compose -p $(COMPOSE_PROJECT) \
	  -f $(COMPOSE_FILE_ROOT) \
	  -f $(COMPOSE_FILE_MON) \
	  --profile $(PROFILE) up -d
	@echo "✅ Monitoring up. Grafana: $(GRAFANA_URL)  Prometheus: $(PROM_URL)"

down: ## Tear down the monitoring stack (app stack untouched)
	docker compose -p $(COMPOSE_PROJECT) \
	  -f $(COMPOSE_FILE_ROOT) \
	  -f $(COMPOSE_FILE_MON) \
	  --profile $(PROFILE) down

restart: ## Restart every monitoring container
	docker compose -p $(COMPOSE_PROJECT) \
	  -f $(COMPOSE_FILE_ROOT) \
	  -f $(COMPOSE_FILE_MON) \
	  --profile $(PROFILE) restart

logs: ## Tail logs for every monitoring container (follow)
	docker compose -p $(COMPOSE_PROJECT) \
	  -f $(COMPOSE_FILE_ROOT) \
	  -f $(COMPOSE_FILE_MON) \
	  --profile $(PROFILE) logs -f

ps: ## List monitoring containers
	docker compose -p $(COMPOSE_PROJECT) \
	  -f $(COMPOSE_FILE_ROOT) \
	  -f $(COMPOSE_FILE_MON) \
	  --profile $(PROFILE) ps

# -----------------------------------------------------------------------------
# status — containers + a quick health probe
# -----------------------------------------------------------------------------
status: ## Show container status + a Prometheus + Grafana health probe
	@echo ""
	@echo "📊  Monitoring containers:"
	@echo "─────────────────────────────────────────────"
	@docker compose -p $(COMPOSE_PROJECT) \
	  -f $(COMPOSE_FILE_ROOT) \
	  -f $(COMPOSE_FILE_MON) \
	  --profile $(PROFILE) ps || true
	@echo ""
	@echo "❤️   Health probes:"
	@printf "  Prometheus    → "; \
	  curl -fsS -o /dev/null -w "HTTP %{http_code} (%{time_total}s)\n" $(PROM_URL)/-/ready \
	    || echo "❌ unreachable (is the container up?)"
	@printf "  Prometheus    → "; \
	  curl -fsS -o /dev/null -w "HTTP %{http_code} (%{time_total}s)\n" $(PROM_URL)/-/healthy \
	    || echo "❌ unreachable"
	@printf "  Grafana       → "; \
	  curl -fsS -o /dev/null -w "HTTP %{http_code} (%{time_total}s)\n" $(GRAFANA_URL)/api/health \
	    || echo "❌ unreachable (is the container up?)"
	@printf "  Alertmanager  → "; \
	  curl -fsS -o /dev/null -w "HTTP %{http_code} (%{time_total}s)\n" http://localhost:9093/-/healthy \
	    || echo "❌ unreachable"
	@echo ""

# -----------------------------------------------------------------------------
# Per-service logs
# -----------------------------------------------------------------------------
prometheus-logs:     ## Tail Prometheus logs
	docker logs -f dongdo_prometheus

grafana-logs:        ## Tail Grafana logs
	docker logs -f dongdo_grafana

alertmanager-logs:   ## Tail Alertmanager logs
	docker logs -f dongdo_alertmanager

postgres-exporter-logs: ## Tail Postgres exporter logs
	docker logs -f dongdo_postgres_exporter

redis-exporter-logs:    ## Tail Redis exporter logs
	docker logs -f dongdo_redis_exporter

nginx-exporter-logs:    ## Tail nginx exporter logs (only if container exists)
	@if docker ps --format '{{.Names}}' | grep -q '^dongdo_nginx_exporter$$'; then \
	  docker logs -f dongdo_nginx_exporter; \
	else \
	  echo "❌  dongdo_nginx_exporter is not running (not in compose? not started?)"; \
	fi

node-exporter-logs:     ## Tail node-exporter logs
	docker logs -f dongdo_node_exporter

# -----------------------------------------------------------------------------
# Debug shells
# -----------------------------------------------------------------------------
prometheus-shell:   ## Open a shell in the Prometheus container
	docker exec -it dongdo_prometheus /bin/sh

grafana-shell:      ## Open a shell in the Grafana container
	docker exec -it dongdo_grafana /bin/sh

alertmanager-shell: ## Open a shell in the Alertmanager container
	docker exec -it dongdo_alertmanager /bin/sh

# -----------------------------------------------------------------------------
# Hot-reload Prometheus (requires --web.enable-lifecycle)
# -----------------------------------------------------------------------------
reload-prometheus: ## Hot-reload Prometheus (POST /-/reload)
	@echo "🔄 Triggering Prometheus reload at $(PROM_URL)/-/reload ..."
	@curl -fsS -X POST $(PROM_URL)/-/ready >/dev/null 2>&1 || \
	  (echo "❌ Prometheus not reachable at $(PROM_URL)"; exit 1)
	@curl -fsS -X POST $(PROM_URL)/-/reload && echo "✅ reloaded"
	@echo ""

# -----------------------------------------------------------------------------
# Diagnostic — "why is my dashboard empty?"
# -----------------------------------------------------------------------------
check-targets: ## Query Prometheus for every scrape target and show UP/DOWN
	@echo ""
	@echo "🎯  Prometheus scrape targets"
	@echo "─────────────────────────────────────────────"
	@JSON=$$(curl -fsS $(PROM_URL)/api/v1/targets 2>/dev/null); \
	if [ -z "$$JSON" ]; then \
	  echo "❌ Could not reach $(PROM_URL)/api/v1/targets — is Prometheus up?"; \
	  echo "   Falling back to docker exec dongdo_promium wget ..."; \
	  JSON=$$(docker exec dongdo_prometheus wget -qO- http://localhost:9090/api/v1/targets 2>/dev/null); \
	  if [ -z "$$JSON" ]; then \
	    echo "❌ Fallback also failed."; exit 1; \
	  fi; \
	fi; \
	echo "$$JSON" | python3 -c "import json,sys; \
d=json.load(sys.stdin)['data']['activeTargets']; \
groups={}; \
[groups.setdefault(t['labels'].get('job','?'),[]).append(t) for t in d]; \
order=['prometheus','node','postgres','redis','nginx','server','cs-server','cskh-server']; \
jobs=sorted(groups.keys(), key=lambda j:(order.index(j) if j in order else 999, j)); \
for j in jobs: \
  print(f'\n  [{j}]'); \
  [print(f\"    {'UP ' if t['health']=='up' else 'DOWN'}  {t['labels'].get('instance','?'):30s}  lastErr: {t.get('lastError','')[:80]}\") for t in groups[j]]" \
	|| (echo "❌ Failed to parse JSON — raw response:"; echo "$$JSON" | head -40)
	@echo ""

# -----------------------------------------------------------------------------
# Diagnostic — exporter UP/DOWN summary
# -----------------------------------------------------------------------------
check-exporters: ## Quick UP/DOWN summary for every exporter (postgres/redis/node/nginx)
	@echo ""
	@echo "🧪  Exporter UP/DOWN summary"
	@echo "─────────────────────────────────────────────"
	@for pair in \
	  "postgres-exporter:9187" \
	  "redis-exporter:9121"    \
	  "node-exporter:9100"     \
	  "nginx-exporter:9113"; do \
	    name=$${pair%%:*}; port=$${pair##*:}; \
	    code=$$(curl -s -o /dev/null -w '%{http_code}' http://localhost:$${port}/metrics 2>/dev/null); \
	    if [ "$$code" = "200" ]; then \
	      printf "  ✅  %-18s http://localhost:%s/metrics  → HTTP 200\n" "$$name" "$$port"; \
	    else \
	      printf "  ❌  %-18s http://localhost:%s/metrics  → HTTP %s\n" "$$name" "$$port" "$$code"; \
	    fi; \
	  done
	@echo ""
	@echo "💡  If an exporter shows ❌, the container is either down or its port is not published."
	@echo "    Run \`make ps\` to check container state, \`make <name>-logs\` for the reason."
	@echo ""

# -----------------------------------------------------------------------------
# Diagnostic — server /metrics reachable?
# -----------------------------------------------------------------------------
check-server-metrics: ## Verify the Go server's /metrics is reachable from Prometheus
	@echo ""
	@echo "🩺  Server /metrics reachability"
	@echo "─────────────────────────────────────────────"
	@printf "  From host    → "; \
	  curl -fsS -o /dev/null -w "HTTP %{http_code}\n" http://127.0.0.1:9090/metrics \
	    || echo "❌ 127.0.0.1:9090/metrics unreachable (server up? METRICS_ADDR set?)"
	@printf "  From Prometheus container → "; \
	  docker exec dongdo_prometheus wget -qO- --timeout=3 http://host.docker.internal:9090/metrics >/dev/null 2>&1 \
	    && echo "✅ OK" \
	    || echo "❌ unreachable (host.docker.internal mapping? firewall?)"
	@echo ""

# -----------------------------------------------------------------------------
# Grafana — list provisioned dashboards
# -----------------------------------------------------------------------------
dashboard-list: ## List Grafana provisioned dashboards via the API (admin user)
	@echo ""
	@echo "📈  Grafana dashboards"
	@echo "─────────────────────────────────────────────"
	@USER=$${GRAFANA_ADMIN_USER:-admin}; \
	PASS=$${GRAFANA_ADMIN_PASSWORD:-admin}; \
	BASE=$${GRAFANA_URL:-$(GRAFANA_URL)}; \
	CODE=$$(curl -s -o /tmp/_dash.json -w '%{http_code}' -u $$USER:$$PASS $$BASE/api/search?type=dash-db); \
	if [ "$$CODE" != "200" ]; then \
	  echo "❌ Grafana API returned HTTP $$CODE (auth? container up?)"; \
	  echo "   Tip: GRAFANA_ADMIN_USER / GRAFANA_ADMIN_PASSWORD envs override defaults."; \
	  exit 1; \
	fi; \
	python3 -c "import json; \
ds=json.load(open('/tmp/_dash.json')); \
[print(f\"  {d.get('type','?'):6s}  uid={d.get('uid','?'):20s}  {d.get('title','?')}\") for d in ds]" \
	|| cat /tmp/_dash.json
	@echo ""

# -----------------------------------------------------------------------------
# Prometheus config validation (uses promtool if available)
# -----------------------------------------------------------------------------
validate-prometheus: ## Validate prometheus.yml with promtool (or warn if missing)
	@echo ""
	@echo "🧪  Validating prometheus/prometheus.yml"
	@echo "─────────────────────────────────────────────"
	@if command -v promtool >/dev/null 2>&1; then \
	  promtool check config prometheus/prometheus.yml && echo "✅ config OK"; \
	else \
	  echo "⚠️  promtool not on PATH — falling back to in-container check"; \
	  docker exec dongdo_prometheus promtool check config /etc/prometheus/prometheus.yml \
	    && echo "✅ config OK" \
	    || echo "❌ config invalid"; \
	fi
	@echo ""

promtool: ## Alias for validate-prometheus
	$(MAKE) -C $(dir $(abspath $(lastword $(MAKEFILE_LIST)))) validate-prometheus

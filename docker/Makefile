# =============================================================================
# DongDo_CS_V2 — Docker subsystem (nginx + orchestration) Makefile
# =============================================================================
# Manages the nginx reverse-proxy container that fronts the CSKH app stack.
# Assumes the main app stack has already been started via `make up` from
# the project root.
#
# The nginx image is built from docker/nginx/Dockerfile and the resulting
# container joins the `dongdo_cs_v2_default` network (created by the main
# compose) so it can resolve `web`, `server`, etc.
# =============================================================================

.PHONY: help nginx-build nginx-up nginx-down nginx-logs nginx-reload \
        nginx-config-test nginx-shell env-list

.DEFAULT_GOAL := help

# -----------------------------------------------------------------------------
# Settings
# -----------------------------------------------------------------------------
NGINX_IMAGE      := dongdo-cs-nginx:latest
NGINX_CONTEXT    := nginx
NGINX_CONTAINER  := dongdo_nginx
COMPOSE_FILE     := ../docker-compose.yml
COMPOSE_PROJECT  := dongdo_cs_v2

# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------
help: ## Show this help message
	@echo ""
	@echo "🐳  DongDo_CS_V2 — Docker subsystem Makefile"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} \
	  /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' \
	  $(MAKEFILE_LIST)
	@echo ""

# -----------------------------------------------------------------------------
# Build the nginx image
# -----------------------------------------------------------------------------
nginx-build: ## Build the nginx image from docker/nginx/ (tag: dongdo-cs-nginx:latest)
	@echo "🔨 Building $(NGINX_IMAGE) from docker/$(NGINX_CONTEXT)/..."
	docker build -t $(NGINX_IMAGE) $(NGINX_CONTEXT)
	@echo "✅ Built $(NGINX_IMAGE)"

# -----------------------------------------------------------------------------
# Bring nginx up / down / logs
# -----------------------------------------------------------------------------
nginx-up: ## Start the nginx container (assumes main stack is already up)
	@echo "🚀 Starting $(NGINX_CONTAINER)..."
	docker compose -p $(COMPOSE_PROJECT) -f $(COMPOSE_FILE) up -d nginx
	@echo "✅ nginx up."

nginx-down: ## Stop and remove the nginx container
	docker compose -p $(COMPOSE_PROJECT) -f $(COMPOSE_FILE) stop nginx
	docker compose -p $(COMPOSE_PROJECT) -f $(COMPOSE_FILE) rm -f nginx

nginx-logs: ## Tail nginx container logs (follow)
	docker logs -f $(NGINX_CONTAINER)

# -----------------------------------------------------------------------------
# Hot-reload / config validation
# -----------------------------------------------------------------------------
nginx-reload: ## Reload nginx in place (no downtime) — `nginx -s reload`
	@echo "🔄 Reloading $(NGINX_CONTAINER)..."
	docker exec $(NGINX_CONTAINER) nginx -s reload
	@echo "✅ nginx reloaded."

nginx-config-test: ## Validate nginx config inside the container — `nginx -t`
	@echo "🧪 Testing nginx config inside $(NGINX_CONTAINER)..."
	docker exec $(NGINX_CONTAINER) nginx -t

# -----------------------------------------------------------------------------
# Debug shell
# -----------------------------------------------------------------------------
nginx-shell: ## Open an interactive bash shell in the nginx container
	docker exec -it $(NGINX_CONTAINER) bash

# -----------------------------------------------------------------------------
# Env introspection
# -----------------------------------------------------------------------------
env-list: ## Show the key envs nginx consumes (from docker-compose.yml + .env)
	@echo ""
	@echo "📋  nginx-relevant envs"
	@echo "─────────────────────────────────────────────"
	@if [ -f ../.env ]; then \
	  echo "(sourced from ../.env)"; \
	  grep -E '^(NGINX_|DOMAIN_|SSL_|LETSENCRYPT_)' ../.env 2>/dev/null || \
	    echo "  (no NGINX_/DOMAIN_/SSL_/LETSENCRYPT_ keys in .env)"; \
	else \
	  echo "  ../.env not found — run \`make sync-init\` or copy .env.example."; \
	fi
	@echo ""
	@echo "🔍  nginx container envs (live):"
	@docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' $(NGINX_CONTAINER) 2>/dev/null \
	  | grep -E '^(NGINX_|DOMAIN_|SSL_|LETSENCRYPT_)' \
	  || echo "  (container $(NGINX_CONTAINER) not running, or no matching envs)"
	@echo ""

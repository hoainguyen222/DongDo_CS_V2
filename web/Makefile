# =============================================================================
# DongDo_CS_V2 — Web (Next.js) Makefile
# =============================================================================
# Professional Next.js subproject helper. Every target has a `##` doc comment
# so `make help` auto-extracts and prints them grouped by section.
#
# Conventions:
#   - Tabs for indentation (Makefile requirement).
#   - `.PHONY` for every target.
#   - All paths absolute.
# =============================================================================

.PHONY: help install deps dev build start lint typecheck clean \
        docker-build docker-build-dev docker-run docker-run-dev \
        env-dev env-prod env-local env-list

# Default goal.
.DEFAULT_GOAL := help

# -----------------------------------------------------------------------------
# Help (auto-extracts `## description` comments)
# -----------------------------------------------------------------------------
help: ## Show this help message
	@echo ""
	@echo "🌐  DongDo_CS_V2 — Web (Next.js) Makefile"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} \
	  /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' \
	  $(MAKEFILE_LIST)
	@echo ""

# -----------------------------------------------------------------------------
# Install / dependencies
# -----------------------------------------------------------------------------
install: ## Install dependencies via npm ci (CI-friendly, requires lockfile)
	npm ci

deps: install ## Alias for `install`

# -----------------------------------------------------------------------------
# Dev server
# -----------------------------------------------------------------------------
dev: ## Run Next.js dev server bound to all interfaces (HOST=0.0.0.0, port 3000)
	HOST=0.0.0.0 npm run dev

# -----------------------------------------------------------------------------
# Build
# -----------------------------------------------------------------------------
build: ## Build the production bundle (next build → .next/)
	npm run build

# -----------------------------------------------------------------------------
# Start (production)
# -----------------------------------------------------------------------------
start: ## Start the production server (next start -p 3000)
	npm start

# -----------------------------------------------------------------------------
# Lint / typecheck
# -----------------------------------------------------------------------------
lint: ## Run ESLint (next lint)
	npm run lint

typecheck: ## Run TypeScript type-check only (tsc --noEmit, no emit)
	npx tsc --noEmit

# -----------------------------------------------------------------------------
# Clean
# -----------------------------------------------------------------------------
clean: ## Remove build artifacts (.next/, .tsbuildinfo)
	rm -rf .next
	rm -rf .tsbuildinfo
	@echo "✅ Cleaned .next and .tsbuildinfo"

# -----------------------------------------------------------------------------
# Docker — production image
# -----------------------------------------------------------------------------
docker-build: ## Build the production Docker image (tag: dongdo-cs-web:latest)
	docker build -t dongdo-cs-web:latest -f Dockerfile .

# -----------------------------------------------------------------------------
# Docker — dev image (with bind-mount for hot reload)
# -----------------------------------------------------------------------------
docker-build-dev: ## Build the dev Docker image (tag: dongdo-cs-web:dev)
	docker build -t dongdo-cs-web:dev -f Dockerfile.dev .

docker-run: ## Run the production image on host port 3000
	docker run --rm -p 3000:3000 --name dongdo-cs-web dongdo-cs-web:latest

docker-run-dev: ## Run the dev image with src/ bind-mounted for hot reload
	docker run --rm -p 3000:3000 \
	  -v $(PWD)/src:/app/src \
	  -v $(PWD)/public:/app/public \
	  --name dongdo-cs-web-dev dongdo-cs-web:dev

# -----------------------------------------------------------------------------
# Env file helpers (only copies if the target doesn't already exist)
# -----------------------------------------------------------------------------
env-dev: ## Create .env.development from .env.development.example (no-clobber)
	@if [ ! -f .env.development.example ]; then \
	  echo "❌  .env.development.example not found"; exit 1; \
	fi
	cp -n .env.development.example .env.development
	@echo "✅  .env.development ready (edit as needed)"

env-prod: ## Create .env.production from .env.production.example (no-clobber)
	@if [ ! -f .env.production.example ]; then \
	  echo "❌  .env.production.example not found"; exit 1; \
	fi
	cp -n .env.production.example .env.production
	@echo "✅  .env.production ready (edit as needed)"

env-local: ## Create .env.local from .env.local.example (no-clobber)
	@if [ ! -f .env.local.example ]; then \
	  echo "❌  .env.local.example not found"; exit 1; \
	fi
	cp -n .env.local.example .env.local
	@echo "✅  .env.local ready (edit as needed)"

env-list: ## List the env helper targets
	@echo "Available env helpers:"
	@echo "  env-dev     — create .env.development from .env.development.example"
	@echo "  env-prod    — create .env.production    from .env.production.example"
	@echo "  env-local   — create .env.local          from .env.local.example"

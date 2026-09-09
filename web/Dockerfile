# =============================================================================
# Web (Next.js) — Production Dockerfile
# -----------------------------------------------------------------------------
# Multi-stage build for Next.js 14 (App Router).
#
# Uses the NON-standalone output path (we do NOT enable `output: 'standalone'`
# in next.config.js) so this Dockerfile copies `.next`, `public`, and the full
# `node_modules` tree from the builder. This is the simplest and most reliable
# approach across Next.js minor versions.
#
# If you switch to `output: 'standalone'` later, the runner stage must change
# to:
#   COPY --from=builder /app/public ./public
#   COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
#   COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
# A short comment noting both paths is kept inline below for reference.
#
# IMPORTANT: Next.js inlines `NEXT_PUBLIC_*` variables at BUILD TIME.
# We accept them as ARGs so docker-compose can pass them in from the root
# `.env` file. Non-NEXT_PUBLIC vars (e.g. `BACKEND_URL`) are passed at runtime
# via compose `environment:` and reach `next start` directly.
# =============================================================================

# ---------- 1. deps ----------------------------------------------------------
# Installs ALL deps (including devDependencies) because the builder needs
# `next`, `typescript`, `sass`, etc. to compile the project.
FROM node:20-alpine AS deps
RUN apk add --no-cache libc6-compat
WORKDIR /app

# Copy manifests first for maximum layer-cache reuse.
COPY package.json package-lock.json* ./
RUN npm ci --no-audit --no-fund

# ---------- 2. builder -------------------------------------------------------
FROM node:20-alpine AS builder
RUN apk add --no-cache libc6-compat
WORKDIR /app

ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production

# ---- Build-time args (inlined into JS bundle by Next.js) -------------------
# All `NEXT_PUBLIC_*` values MUST be present at build time, otherwise
# Next.js will inline `undefined` and they will never reach the browser.
# Strategy: copy .env into the builder and source it so ENV vars are available
# to `next build`. This works on any host (local, CI, AWS) without needing
# `--env-file` flags.
COPY .env ./
RUN set -a && . .env && set +a && \
    echo "NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL:-http://localhost:8080}" && \
    echo "NEXT_PUBLIC_APP_ENV=${NEXT_PUBLIC_APP_ENV:-production}" && \
    echo "NEXT_PUBLIC_APP_NAME=${NEXT_PUBLIC_APP_NAME:-DongDo CS}" && \
    echo "NEXT_PUBLIC_ADMIN_PATH=${NEXT_PUBLIC_ADMIN_PATH:-/admin}" && \
    echo "NEXT_PUBLIC_WS_URL=${NEXT_PUBLIC_WS_URL:-ws://localhost:8080}" && \
    echo "NEXT_PUBLIC_STUN_SERVERS=${NEXT_PUBLIC_STUN_SERVERS:-stun:stun.l.google.com:19302}" && \
    echo "NEXT_PUBLIC_LOG_LEVEL=${NEXT_PUBLIC_LOG_LEVEL:-warn}"

# Promote all NEXT_PUBLIC_* from the sourced .env into ENV so `next build` sees them.
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL:-http://localhost:8080} \
    NEXT_PUBLIC_APP_ENV=${NEXT_PUBLIC_APP_ENV:-production} \
    NEXT_PUBLIC_APP_NAME=${NEXT_PUBLIC_APP_NAME:-DongDo CS} \
    NEXT_PUBLIC_ADMIN_PATH=${NEXT_PUBLIC_ADMIN_PATH:-/admin} \
    NEXT_PUBLIC_WS_URL=${NEXT_PUBLIC_WS_URL:-ws://localhost:8080} \
    NEXT_PUBLIC_STUN_SERVERS=${NEXT_PUBLIC_STUN_SERVERS:-stun:stun.l.google.com:19302} \
    NEXT_PUBLIC_LOG_LEVEL=${NEXT_PUBLIC_LOG_LEVEL:-warn}

# Reuse the already-installed node_modules from the deps stage.
COPY --from=deps /app/node_modules ./node_modules
COPY . .

RUN npm run build

# ---------- 3. runner --------------------------------------------------------
FROM node:20-alpine AS runner
RUN apk add --no-cache libc6-compat wget
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

# Create a non-root user for runtime safety.
RUN addgroup --system --gid 1001 nodejs \
 && adduser  --system --uid 1001 nextjs

# --- Choose ONE of the two copy strategies below ----------------------------
#
# (A) NON-STANDALONE strategy (currently used) ------------------------------
# Copy the full build output + public assets + production node_modules.
COPY --from=builder --chown=nextjs:nodejs /app/.next ./.next
COPY --from=builder --chown=nextjs:nodejs /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/node_modules ./node_modules
COPY --from=builder --chown=nextjs:nodejs /app/package.json ./package.json
#
# (B) STANDALONE strategy (only if next.config.js sets `output: 'standalone'`)
# COPY --from=builder --chown=nextjs:nodejs /app/public ./public
# COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
# COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
# ----------------------------------------------------------------------------

USER nextjs

EXPOSE 3000

# Liveness probe — `wget` is already installed via apk add above. Next.js
# `next start` begins answering HTTP immediately on PORT, so a GET to / is a
# sufficient smoke test.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget --spider http://localhost:3000/ || exit 1

CMD ["npm", "start"]

# ---------- Metadata --------------------------------------------------------
LABEL org.opencontainers.image.title="dongdo-cs-v2-web" \
      org.opencontainers.image.source="https://github.com/dongdo/dongdo-cs-v2" \
      org.opencontainers.image.version="2.0.0" \
      org.opencontainers.image.description="Next.js 14 frontend for DongDo Customer Service"

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
# The builder stage copies `web/.env` and sources it directly, so the env vars
# are available to `next build` without needing `--env-file` or `build.args`.
# Non-NEXT_PUBLIC vars (e.g. `BACKEND_URL`) are passed at runtime via
# docker-compose `env_file:` and reach `next start` directly.
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
# Strategy: stage `.env` into /tmp in the builder and source it so the
# variables are visible to `next build`. We stage in /tmp (not /app) because
# the runtime compose service mounts the same `.env` via `env_file:` at
# /app/.env, and we don't want to overwrite or conflict with that mount.
# `set -a` auto-exports every var, so we don't have to explicitly enumerate
# each one — simpler and avoids drift if a new var is added to web/.env later.
COPY .env /tmp/.env.build
RUN set -a && . /tmp/.env.build && set +a && \
    echo "NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL:-http://localhost:8080}" && \
    echo "NEXT_PUBLIC_APP_ENV=${NEXT_PUBLIC_APP_ENV:-production}" && \
    echo "NEXT_PUBLIC_APP_NAME=${NEXT_PUBLIC_APP_NAME:-DongDo CS}" && \
    echo "NEXT_PUBLIC_ADMIN_PATH=${NEXT_PUBLIC_ADMIN_PATH:-/admin}" && \
    echo "NEXT_PUBLIC_WS_URL=${NEXT_PUBLIC_WS_URL:-ws://localhost:8080}" && \
    echo "NEXT_PUBLIC_STUN_SERVERS=${NEXT_PUBLIC_STUN_SERVERS:-stun:stun.l.google.com:19302}" && \
    echo "NEXT_PUBLIC_LOG_LEVEL=${NEXT_PUBLIC_LOG_LEVEL:-warn}" && \
    echo "BACKEND_URL=${BACKEND_URL:-http://server:8080}" && \
    rm -f /tmp/.env.build

# Promote the sourced .env into ENV so `next build` sees them. Values with
# spaces MUST be quoted so Docker parses them as a single value; otherwise
# Docker splits on whitespace and fails with:
#   `Syntax error - can't find = in "CS"`.
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL:-http://localhost:8080} \
    NEXT_PUBLIC_APP_ENV=${NEXT_PUBLIC_APP_ENV:-production} \
    NEXT_PUBLIC_APP_NAME="${NEXT_PUBLIC_APP_NAME:-DongDo CS}" \
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

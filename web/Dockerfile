# ---------- deps ----------
  FROM node:20-alpine AS deps

  RUN apk add --no-cache libc6-compat
  
  WORKDIR /app
  
  COPY package.json package-lock.json* ./
  
  RUN npm ci --no-audit --no-fund
  
  
  # ---------- builder ----------
  FROM node:20-alpine AS builder
  
  RUN apk add --no-cache libc6-compat
  
  WORKDIR /app
  
  ENV NEXT_TELEMETRY_DISABLED=1
  ENV NODE_ENV=production
  
  COPY --from=deps /app/node_modules ./node_modules
  
  # Copy toàn bộ source, bao gồm .env nếu không bị .dockerignore ignore
  COPY . .
  
  RUN npm run build
  
  
  # ---------- runner ----------
  FROM node:20-alpine AS runner
  
  RUN apk add --no-cache libc6-compat wget
  
  WORKDIR /app
  
  ENV NODE_ENV=production
  ENV NEXT_TELEMETRY_DISABLED=1
  ENV PORT=3000
  ENV HOSTNAME=0.0.0.0
  
  RUN addgroup --system --gid 1001 nodejs \
   && adduser --system --uid 1001 nextjs
  
  COPY --from=builder --chown=nextjs:nodejs /app/.next ./.next
  COPY --from=builder --chown=nextjs:nodejs /app/public ./public
  COPY --from=builder --chown=nextjs:nodejs /app/node_modules ./node_modules
  COPY --from=builder --chown=nextjs:nodejs /app/package.json ./package.json
  
  USER nextjs
  
  EXPOSE 3000
  
  HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget --spider http://localhost:3000/ || exit 1
  
  CMD ["npm", "start"]
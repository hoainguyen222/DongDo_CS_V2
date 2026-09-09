# DongDo CS — Reverse Proxy (nginx)

This directory contains the production nginx reverse-proxy layer for the
DongDo CS stack. It sits in front of the Go backend (`server`) and the
Next.js frontend (`web`), terminating TLS and routing traffic to the
correct upstream.

## Architecture

```
                              Internet
                                 │
                                 │  :80 / :443
                                 ▼
                         ┌─────────────────┐
                         │      nginx      │   ← this image
                         │  (TLS, gzip,    │
                         │   rate-limit)   │
                         └────────┬────────┘
                                  │
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
        ▼                         ▼                         ▼
  /api /auth /chat         /_next/*  /*             /metrics (internal)
  /static /ws /wss         (Next.js)                     /debug/pprof
        │                         │                         │
        ▼                         ▼                         ▼
   ┌─────────┐               ┌─────────┐               ┌─────────┐
   │ server  │               │   web   │               │ server  │
   │ :8080   │               │ :3000   │               │ :9090   │
   └─────────┘               └─────────┘               └─────────┘
```

### Routing rules (production server block)

| Path prefix                 | Upstream             | Notes                              |
| --------------------------- | -------------------- | ---------------------------------- |
| `/`                         | `web:3000`           | Next.js renders everything not listed below |
| `/_next/static/*`           | `web:3000`           | 1-year `Cache-Control: immutable`  |
| `/favicon.ico`, `/robots.txt`, `/manifest.json`, `/sitemap.xml` | `web:3000` | short cache |
| `/api/*`                    | `server:8080`        | rate-limit `30r/s`, burst 60       |
| `/auth/*`                   | `server:8080`        | rate-limit `10r/s`, burst 20       |
| `/guest/*`                  | `server:8080`        |                                    |
| `/chat`                     | `server:8080`        | buffering off, 300s read timeout   |
| `/history/*`                | `server:8080`        |                                    |
| `/static/*`                 | `server:8080`        | 7-day cache                        |
| `/ws`, `/wss`               | `server:8080`        | WebSocket upgrade, 1h timeout      |
| `/socket.io/*`              | `server:8080`        | WebSocket upgrade                  |
| `/call/*`                   | `call_service:8081`  | optional direct path               |
| `/metrics`, `/debug/pprof/*` | `server` (internal) | allow only private subnets        |
| `/nginx-health`             | local `return 200`   | used by Docker healthcheck         |
| `/health`                   | `server:8080/health` | passthrough                        |

## Environment variables

All variables are optional and have sensible defaults.

| Variable              | Default                              | Description                                              |
| --------------------- | ------------------------------------ | -------------------------------------------------------- |
| `NGINX_SERVER_NAME`   | `_` (catch-all)                      | Virtual host. Set to your FQDN, e.g. `cskh.dongdo.vn`.   |
| `UPLOAD_LIMIT`        | `50M`                                | `client_max_body_size`. Audio uploads need ~20–50MB.      |
| `NGINX_ENABLE_SSL`    | `false`                              | Set to `true` to enable the :443 server block.           |
| `SSL_CERT_PATH`       | `/etc/nginx/ssl/fullchain.pem`       | Path inside the container to the fullchain certificate.  |
| `SSL_KEY_PATH`        | `/etc/nginx/ssl/privkey.pem`         | Path inside the container to the private key.            |

## Enabling SSL (Let's Encrypt)

The recommended setup is to terminate TLS at nginx using certificates
issued by Let's Encrypt via the certbot container. Two viable
approaches:

### Option 1 — HTTP-01 challenge (simplest, single domain)

```yaml
# add to the nginx service in docker-compose.yml
volumes:
  - ./docker/nginx/certbot/conf:/etc/letsencrypt:ro
  - ./docker/nginx/certbot/www:/var/www/certbot:ro
environment:
  NGINX_ENABLE_SSL: "true"
  NGINX_SERVER_NAME: "cskh.dongdo.vn"
  SSL_CERT_PATH: "/etc/letsencrypt/live/cskh.dongdo.vn/fullchain.pem"
  SSL_KEY_PATH:  "/etc/letsencrypt/live/cskh.dongdo.vn/privkey.pem"
```

Then run certbot once against the running stack:

```bash
docker run --rm \
  -v $(pwd)/docker/nginx/certbot/conf:/etc/letsencrypt \
  -v $(pwd)/docker/nginx/certbot/www:/var/www/certbot \
  certbot/certbot certonly --webroot \
    --webroot-path=/var/www/certbot \
    -d cskh.dongdo.vn --email you@example.com --agree-tos --no-eff-email
```

### Option 2 — Wildcard / DNS-01 challenge (multiple subdomains)

Use a DNS plugin (Cloudflare, Route53, ...) and the `certbot-dns-*`
image. Mount the resulting `/etc/letsencrypt` directory into the
nginx container identically.

> The nginx template ships an HTTP-01 challenge handler at
> `/.well-known/acme-challenge/` (root: `/var/www/certbot`). If you
> serve that path from a different root, update the template.

## Adding a new domain

Set `NGINX_SERVER_NAME` to the new FQDN and redeploy. To serve multiple
domains from one nginx instance, copy the `server { ... }` block in
`conf.d/dongdo.conf.template`, change its `server_name`, and
`nginx -t && nginx -s reload`.

## Rate limiting

Two zones are defined in `nginx.conf`:

- `api_rl` — `30 req/s` per IP, burst 60. Applied to `/api/*`.
- `auth_rl` — `10 req/s` per IP, burst 20. Applied to `/auth/*`.

Plus a connection cap of 50 concurrent connections per IP
(`limit_conn conn_per_ip 50`). Tune these in `nginx.conf` if your
traffic profile changes.

## gzip

`gzip on` with `gzip_min_length 1024` is enabled globally for text,
JSON, CSS, JS, XML, SVG and fonts. The compression level is 5; bump
to 6 if you have CPU headroom.

## WebSocket

`/ws`, `/wss`, and `/socket.io/*` all use the nginx WebSocket upgrade
headers (`Upgrade $http_upgrade; Connection "upgrade";`). Read and
send timeouts are raised to 3600s so long-lived voice / chat sessions
are not cut off.

`/chat` uses 300s timeouts (sufficient for SSE / LLM streaming) with
`proxy_buffering off` so responses stream straight to the client.

## Prometheus metrics

`conf.d/metrics.conf` exposes `stub_status` on port **9091 inside the
container**. The `nginx-prometheus-exporter` (added to
`monitoring/docker-compose.monitoring.yml`) reads that endpoint and
publishes metrics to Prometheus.

To verify:

```bash
docker compose \
  -f docker-compose.yml \
  -f monitoring/docker-compose.monitoring.yml \
  --profile monitoring up -d

curl -s http://localhost:9090/targets | jq '.data.activeTargets[] | select(.labels.job=="nginx")'
```

## Files

| Path                                       | Purpose                                                    |
| ------------------------------------------ | ---------------------------------------------------------- |
| `Dockerfile`                               | Builds `nginx:1.27-alpine` + entrypoint.                   |
| `nginx.conf`                               | Main http block, upstreams, gzip, WS headers, rate limits. |
| `docker-entrypoint.sh`                     | Renders templates, strips SSL block if disabled, runs nginx. |
| `conf.d/dongdo.conf.template`              | Main public server block with `${VAR}` placeholders.       |
| `conf.d/metrics.conf`                      | Internal `:9091` stub_status for the prometheus exporter.  |
| `README.md`                                | This file.                                                 |

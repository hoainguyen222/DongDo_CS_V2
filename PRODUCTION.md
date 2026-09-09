# DongDo CS — Production Deployment Guide

This document covers deploying `cskh.dongdopartners.com` (or any custom
domain) with HTTPS, Let's Encrypt auto-renewal, and the full observability
stack.

It assumes you have:
- A Linux server (Ubuntu 22.04+ / Amazon Linux 2023) with Docker + Compose v2
  installed.
- A public IP address and DNS A record pointing `cskh.dongdopartners.com`
  to that IP. (DNS propagation may take up to 24h.)
- Ports 80 and 443 open on the host firewall / security group.

---

## 1. Configure `.env`

```bash
cp .env.production.example .env
$EDITOR .env
```

Fill in (at minimum):
- `NGINX_SERVER_NAME=cskh.dongdopartners.com`
- `NGINX_ENABLE_SSL=true`
- `CERTBOT_EMAIL=ops@dongdopartners.com`
- `JWT_SECRET=<32-char random>` — generate with `openssl rand -base64 48`
- `GRAFANA_ADMIN_PASSWORD=<strong>`
- `POSTGRES_PASSWORD=<strong>`
- `ANTHROPIC_API_KEY=sk-ant-...`
- `ASTERISK_PASS=<strong>`

`COOKIE_DOMAIN` and `COOKIE_SECURE=true` must be set for HTTPS-only cookies.

---

## 2. Generate DH params (one-off, ~minutes)

```bash
make nginx-dhparam
```

This writes `docker/nginx/dhparam.pem` (4096-bit). Without it the nginx
image falls back to a 2048-bit value baked at build time — secure, but
slower TLS handshakes.

---

## 3. Bring up the stack

```bash
make up
```

This starts every service behind the reverse proxy:
- **nginx** listens on `:80` and `:443` (HTTPS block is rendered but the
  cert doesn't exist yet — nginx will refuse to start).
- **certbot** is on the `ssl` profile. Start it explicitly:
  ```bash
  docker compose --profile ssl up -d certbot
  ```
  It will loop `certbot renew` every 12h; on first start it does nothing
  until a cert exists.

---

## 4. Issue the first Let's Encrypt cert

```bash
make nginx-ssl-init
```

The HTTP-01 challenge uses `/var/www/certbot` (bind-mounted into both nginx
and certbot). nginx serves the challenge files from
`location /.well-known/acme-challenge/` before redirecting everything else
to HTTPS — so port :80 must already be reachable from the public internet.

If you hit Let's Encrypt rate limits (5 duplicate certs / 7 days), retry
with `CERTBOT_STAGING=1` to test the flow against the staging server.

---

## 5. Reload nginx

```bash
make docker-nginx-reload
```

nginx now serves HTTPS with OCSP stapling, HSTS, and the full CSP/CORS
header set.

---

## 6. Verify

```bash
# 6.1 Cert details (issuer, expiry, SANs).
make nginx-cert-info

# 6.2 Days until expiry (alerts if <14).
make nginx-check-expiry

# 6.3 External SSL test (testssl.sh if installed, else openssl s_client).
make nginx-test-ssl

# 6.4 HTTP → HTTPS redirect.
curl -fsS -o /dev/null -w "%{http_code} %{redirect_url}\n" \
  http://cskh.dongdopartners.com/nginx-health
# → 301 https://cskh.dongdopartners.com/nginx-health

# 6.5 Health check.
curl -fsS https://cskh.dongdopartners.com/nginx-health
# → ok
```

---

## 7. Add the monitoring stack (optional but recommended)

```bash
make monitoring-up
```

Opens Grafana on `https://cskh.dongdopartners.com:3050` (override via
`GRAFANA_PORT` in `.env`). Grafana admin password is whatever you set in
`GRAFANA_ADMIN_PASSWORD`.

> ⚠️  Grafana listens on a non-standard port to avoid colliding with your
> main app. If you want it on `:443/grafana/`, front it with another nginx
> server block — or just keep the dedicated port and restrict it via SSH
> tunnel / VPN.

---

## 8. Auto-renewal

`certbot` sidecar loops every 12h:
```sh
certbot renew --webroot -w /var/www/certbot --quiet
```

New certs land in `/etc/letsencrypt/live/<domain>/`. nginx keeps serving
the OLD cert until you reload — combine with a hook to get zero downtime:

```bash
# /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh
#!/bin/sh
docker exec dongdo_nginx nginx -s reload
```

Then mount that script into the certbot container (see certbot docs).
Alternatively run `make docker-nginx-reload` from a systemd timer:

```ini
# /etc/systemd/system/dongdo-nginx-reload.timer
[Unit]
Description=Hourly nginx reload (picks up renewed certs)

[Timer]
OnCalendar=hourly
Persistent=true

[Service]
Type=oneshot
ExecStart=/usr/bin/docker exec dongdo_nginx nginx -s reload
```

---

## 9. Backups

- **Postgres**: `docker exec dongdo_postgres pg_dump -U postgres dongdo_cs | gzip > backup-$(date +%F).sql.gz`
- **Redis** (AOF): `docker cp dongdo_redis:/data/appendonly.aof ./redis-aof-$(date +%F)`
- **Recordings**: tar up `./recordings/` (out-of-band cron recommended)
- **Let's Encrypt**: tar up `./docker/nginx/certbot/conf/` — losing this
  means waiting for the next renewal cycle.

---

## 10. Hardening checklist

- [x] HTTPS only, HSTS preload-eligible.
- [x] TLS 1.2 / 1.3 only, ECDHE preferred.
- [x] OCSP stapling on.
- [x] CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy,
      Permissions-Policy all set.
- [x] Rate limiting on `/api/*` and `/auth/*`.
- [x] Connection cap per IP (50).
- [x] `/metrics` and `/debug/pprof` denied from public.
- [ ]  Replace all `CHANGEME` values in `.env`.
- [ ]  Rotate `JWT_SECRET` after bootstrap.
- [ ]  Front Grafana / Prometheus with auth if exposed publicly.
- [ ]  Enable fail2ban on SSH.
- [ ]  Configure off-host backups for Postgres + Redis.
- [ ]  Submit `cskh.dongdopartners.com` to https://hstspreload.org/

---

## Troubleshooting

| Symptom | Likely cause | Fix |
| ------- | ------------ | --- |
| `nginx -t` fails: "cannot load certificate" | certbot never ran | `make nginx-ssl-init` |
| `nginx -t` fails: "BIO_new_file("/etc/nginx/dhparam.pem") failed" | DH params file missing | `make nginx-dhparam` then `make up` |
| ACME challenge fails with `connection refused` | Port :80 blocked or DNS not propagated | Check security group / DNS, wait for TTL |
| Cert renewal loop returns 0 but nginx still serves old cert | nginx didn't reload | `make docker-nginx-reload` (or use a deploy hook) |
| `curl: (60) SSL certificate problem: unable to get local issuer certificate` | OCSP stapling off or cert chain incomplete | nginx mounts `fullchain.pem` (not `cert.pem`); check `nginx-cert-info` |
| HSTS not applied over HTTP | HSTS is HTTPS-only by spec | This is correct. First request will be HTTP→HTTPS, subsequent requests carry HSTS. |

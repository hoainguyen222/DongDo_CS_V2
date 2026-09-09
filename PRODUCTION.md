# DongDo CS — Production Operations Guide

> **Audience:** DevOps / SRE / on-call engineer chịu trách nhiệm vận hành
> `cskh.dongdopartners.com` (hoặc bất kỳ domain nào dùng stack này).
>
> **Scope:** Tài liệu này cover toàn bộ vòng đời production — từ bootstrap
> EC2 mới, deploy code mới, vận hành hàng ngày, backup/restore, đến xử lý
> sự cố. Đọc tuần tự section 1–4 cho lần đầu, sau đó dùng section 5+ như
> reference khi cần.

---

## Mục lục

1. [Kiến trúc production tổng quan](#1-kiến-trúc-production-tổng-quan)
2. [Bootstrap EC2 mới (chạy lần đầu)](#2-bootstrap-ec2-mới-chạy-lần-đầu)
3. [Vận hành hàng ngày](#3-vận-hành-hàng-ngày)
4. [Deploy / Update code](#4-deploy--update-code)
5. [Backup & Restore](#5-backup--restore)
6. [Scaling & Performance tuning](#6-scaling--performance-tuning)
7. [Monitoring & Alerting](#7-monitoring--alerting)
8. [Bảo mật vận hành](#8-bảo-mật-vận-hành)
9. [Troubleshooting](#9-troubleshooting)
10. [Disaster Recovery](#10-disaster-recovery)
11. [Phụ lục — Cheat sheet](#11-phụ-lục--cheat-sheet)

---

## 1. Kiến trúc production tổng quan

### 1.1 Topology

```
                              Internet
                                 │
                                 ▼
                  ┌──────────────────────────────┐
                  │  EC2 host (1 instance)       │
                  │                              │
                  │   ┌──────────────────────┐   │
                  │   │ External nginx        │   │   TLS termination (managed separately)
                  │   │ (not in this stack)    │   │   reverse proxy
                  │   └──────┬───────────────┘   │
                  │          │ :3000 (http)       │
                  │   ┌──────▼───────┐  ┌─────┐ │
                  │   │ dongdo_web   │  │ ... │ │   Next.js SSR
                  │   │ (Next.js 14) │  └─────┘ │
                  │   └──────┬───────┘           │
                  │          │                   │
                  │   ┌──────▼───────┐           │   Go API + WS
                  │   │ dongdo_server│           │
                  │   │ (Golang)     │           │
                  │   └──┬───┬───┬───┘           │
                  │      │   │   │               │
                  │  ┌───▼┐ ┌▼──┐ ┌▼─────┐      │
                  │  │ PG │ │RD │ │QDRNT │      │   postgres / redis / qdrant
                  │  └───┘ └───┘ └──────┘      │
                  │                              │
                  │   ┌──────────────────────┐   │   (opt-in profile: monitoring)
                  │   │  prometheus + grafana│   │
                  │   │  + 4 exporters       │   │
                  │   └──────────────────────┘   │
                  └──────────────────────────────┘

  Docker volumes (pgdata, redisdata, qdrantdata, prometheus_data, grafana_data)
  Docker networks (dongdo_cs_v2_default, dongdo-monitoring)
```

### 1.2 Service inventory

| Service | Container | Host port | Profile | Mục đích |
|---|---|---|---|---|
| web | `dongdo_web` | 3000 (internal) | default | Next.js frontend |
| server | `dongdo_server` | 8080 (host), 9091/metrics (loopback) | default | Go API + WebSocket |
| call_service | `dongdo_call_service` | 8081 (host) | default | WebRTC signaling |
| asterisk | `dongdo_asterisk` | 5060/udp, 10000-10200/udp, 8088 | default | PBX |
| postgres | `dongdo_postgres` | 5433 (host, dev only) | default | Source of truth |
| redis | `dongdo_redis` | 6379 (host, dev only) | default | Cache + streams |
| qdrant | `dongdo_qdrant` | 6333, 6334 (host, dev only) | default | Vector DB |
| prometheus | `dongdo_prometheus` | 9090 | **monitoring** | Metrics scrape |
| alertmanager | `dongdo_alertmanager` | 9093 | **monitoring** | Alerts |
| grafana | `dongdo_grafana` | 3050 | **monitoring** | Dashboards |
| node-exporter | `dongdo_node_exporter` | — | **monitoring** | Host metrics |
| postgres-exporter | `dongdo_postgres_exporter` | — | **monitoring** | DB metrics |
| redis-exporter | `dongdo_redis_exporter` | — | **monitoring** | Redis metrics |

> **Note:** Reverse proxy and SSL termination (nginx) is handled by an **external nginx
> instance** (not included in this Docker stack). Deploy and manage it separately.

### 1.3 Yêu cầu hạ tầng tối thiểu

**EC2 instance** (khuyến nghị):
- **Type:** `t3.medium` (2 vCPU / 4 GB RAM) trở lên — production thực sự dùng `t3.large`/`t3.xlarge`.
- **Storage:** 30 GB gp3 EBS (root) + 50 GB gp3 EBS riêng cho `/var/lib/docker` và recordings.
- **OS:** Ubuntu 22.04 LTS hoặc Amazon Linux 2023.
- **Docker:** Engine 24+ với Compose v2.
- **Network:** Public IP + DNS A record trỏ về IP. Security group mở 22 (My IP), 3000, 8080, 8081, 5433, 6379 (tùy dev access).

**DNS** (làm **trước khi** bắt đầu deploy):
- `cskh.dongdopartners.com` → A record → EC2 public IP.
- (Tùy chọn) `monitoring.cskh.dongdopartners.com` → cùng IP, dùng cho Grafana.
- Verify: `dig +short cskh.dongdopartners.com` trả về IP. Propagation tối đa 24h.

---

## 2. Bootstrap EC2 mới (chạy lần đầu)

Toàn bộ phase này giả định EC2 mới, đã cài Docker + Compose v2, đã pull git về.

### 2.0 Chuẩn bị (làm 1 lần trước khi deploy)

```bash
# Trên máy local — generate tất cả secrets một lúc
openssl rand -base64 48     # → JWT_SECRET (≥32 ký tự)
openssl rand -base64 32     # → POSTGRES_PASSWORD
openssl rand -base64 24     # → GRAFANA_ADMIN_PASSWORD
openssl rand -base64 24     # → ASTERISK_PASS

# Verify DNS đã propagate
dig +short cskh.dongdopartners.com
nslookup cskh.dongdopartners.com

# Verify security group
#  - Inbound: 22 (My IP), 80 (0.0.0.0/0), 443 (0.0.0.0/0)
#  - Outbound: all (để apt update + Let's Encrypt)
```

### 2.1 SSH vào EC2

```bash
ssh ubuntu@<EC2_PUBLIC_IP>
cd DongDo_CS_V2
```

### 2.2 Tạo file `.env`

```bash
cp .env.production.example .env
nano .env
```

**Bắt buộc thay** (line-by-line):

| Key | Giá trị | Ví dụ |
|---|---|---|
| `JWT_SECRET` | ≥32 ký tự random | `K8mP2x...` (48 bytes base64) |
| `POSTGRES_PASSWORD` | Mạnh, đổi cả trong `docker-compose.yml` | `S3cur3P@ss...` |
| `GRAFANA_ADMIN_PASSWORD` | Mạnh | `Gr@fanaP@ss!` |
| `ASTERISK_PASS` | Mạnh | `St@riskP@ss!` |
| `ANTHROPIC_API_KEY` | API key thật | `sk-ant-api03-...` |
| `COOKIE_DOMAIN` | Khớp domain của external nginx | `cskh.dongdopartners.com` |
| `COOKIE_SECURE` | Bắt buộc với HTTPS | `true` |
| `GRAFANA_ROOT_URL` | URL public của Grafana | `https://cskh.dongdopartners.com` |
| `APP_ENV` | Production | `production` |

> ⚠️ **`POSTGRES_PASSWORD` xuất hiện ở 2 chỗ** — `.env` và
> `docker-compose.yml` (line `POSTGRES_PASSWORD: postgrespassword`). Phải khớp
> 100%, nếu không postgres-exporter sẽ fail auth.

> ⚠️ **Đừng commit `.env`**. File đã nằm trong `.gitignore` (section
> `Environment & Secrets`).

> Mất ~2–5 phút tùy CPU. DH params không phải secret, có thể copy
> file lên EC2 nếu muốn nhanh hơn.

### 2.3 Build + start app stack

```bash
make up
# Lần đầu sẽ build 4 image: server, web, asterisk, call_service
# ~3-5 phút tùy network
```

Verify:
```bash
docker compose ps
# Kỳ vọng: 7/7 UP

make health
# → ok
```

> **Note:** SSL/TLS termination is handled by external nginx (not in this stack).
> Configure your external nginx to proxy `https://<domain>/` → `http://<EC2_IP>:3000`
> for the Next.js frontend and `https://<domain>/api/*` → `http://<EC2_IP>:8080`
> for the Go API.

### 2.6 Start monitoring stack (khuyến nghị)

```bash
make monitoring-up
# ~30 giây để Prometheus scrape lần đầu
```

Verify:
```bash
make mon-status            # HTTP 200 cho Prometheus, Grafana, Alertmanager
make mon-check-targets     # Tất cả scrape target UP
make mon-check-exporters   # postgres/redis/node exporters OK
```

**Đổi mật khẩu Grafana admin lần đầu:**
1. Mở `https://cskh.dongdopartners.com:3050` (hoặc SSH tunnel nếu port chưa mở public).
2. Login `admin` / `<GRAFANA_ADMIN_PASSWORD>`.
3. Vào **Administration → Users → admin → Password** → đổi.

### 2.7 Smoke test toàn diện

```bash
# 1. App health (direct to backend)
curl -fsS http://<EC2_IP>:8080/health
# → ok

# 2. Web frontend (direct to Next.js)
curl -fsS http://<EC2_IP>:3000/ | head -5
# → <!DOCTYPE html>

# 3. HTTPS via external nginx (replace with your actual domain)
curl -fsS https://cskh.dongdopartners.com/ | head -5
# → <!DOCTYPE html>

# 4. Security headers via external nginx
curl -fsS -D - -o /dev/null https://cskh.dongdopartners.com/ | grep -iE 'strict-transport|x-frame|content-security'

# 5. Prometheus targets
make mon-check-targets
# → tất cả UP

# 6. App health
make health
# → ok

# 7. Manual test trên browser qua external nginx
# Mở https://cskh.dongdopartners.com → đăng nhập → thử chat
```

---

## 3. Vận hành hàng ngày

### 3.1 Container states

```bash
# Trạng thái app stack
docker compose ps

# Trạng thái monitoring
make mon-status

# Restart 1 service (không mất data)
docker compose restart server
docker compose restart web

# Restart tất cả app stack
docker compose restart

# Dừng tất cả (giữ volume/data)
make down

# Xóa hoàn toàn (⚠️ MẤT DATA — dùng cẩn thận)
make up-fresh   # wipe pgdata + redisdata + qdrantdata
```

### 3.2 Logs

```bash
# Tail tất cả app containers
make logs

# Tail 1 service cụ thể
docker logs -f dongdo_server

# Tìm error trong logs
docker logs dongdo_server --since 1h 2>&1 | grep -iE 'error|panic|fatal'

# Tail monitoring stack
make mon-logs
```

### 3.3 Exec vào container (debug)

```bash
# Shell vào Go server
docker exec -it dongdo_server /bin/sh

# Postgres CLI
docker exec -it dongdo_postgres psql -U postgres -d dongdo_cs

# Redis CLI
docker exec -it dongdo_redis redis-cli

# Qdrant REST API
curl http://localhost:6333/collections
```

### 3.4 Check cert expiry

> Skip this section if using external nginx for SSL. Cert expiry monitoring
> should be configured on your external nginx/certbot setup.

### 3.5 Đọc nhanh metrics từ CLI

```bash
# Server /metrics (loopback only — phải ở trên host)
make metrics METRICS_URL=http://127.0.0.1:9091/metrics | head -50

# pprof goroutine dump (debug leak)
make pprof-goroutine
# Trong pprof prompt:
#   top 20      ← top stack traces
#   traces      ← full text
#   quit        ← thoát

# Query Prometheus từ CLI
curl -s 'http://localhost:9090/api/v1/query?query=up' | jq

# Xem 1 metric cụ thể
curl -s 'http://localhost:9090/api/v1/query?query=http_requests_total{job="server"}' | jq
```

### 3.6 WebRTC/Asterisk health check

```bash
# Asterisk status
docker exec dongdo_asterisk asterisk -rx "core show version"
docker exec dongdo_asterisk asterisk -rx "pjsip show endpoints"
docker exec dongdo_asterisk asterisk -rx "core show channels"

# Call service health
curl -fsS http://localhost:8081/health
```

### 3.7 Free up disk (nếu đầy)

```bash
# Xem dung lượng từng thư mục
docker system df
du -sh /var/lib/docker/volumes/dongdo_cs_v2_*

# Xóa docker cache (images cũ, build cache)
docker system prune -af
# ⚠️ KHÔNG xóa `docker volume prune` (mất data)

# Rotate recordings (nếu lưu quá nhiều)
find /home/ubuntu/DongDo_CS_V2/recordings -name "*.webm" -mtime +30 -delete
# Hoặc move sang S3:
# aws s3 sync ./recordings/ s3://dongdo-recordings/ --storage-class STANDARD_IA
```

---

## 4. Deploy / Update code

### 4.1 Pull code mới + rebuild image

```bash
cd DongDo_CS_V2
git pull origin main

# Rebuild image server (Go code đổi)
make rebuild

# Rebuild image web (Next.js code đổi)
make web-docker-build && docker compose up -d web

# Rebuild image asterisk / call_service
docker compose build asterisk call_service
docker compose up -d asterisk call_service

# Rebuild tất cả
docker compose build && docker compose up -d
```

### 4.2 Sau khi thêm migration mới

```bash
# File migration nằm ở db/migrations/ — copy sang initdb dir
make sync-init

# Server tự chạy migration khi start. Nếu muốn chạy thủ công:
make migrate-status
make migrate-up

# Verify
docker logs dongdo_server 2>&1 | grep -i migration
```

### 4.3 Rollback code (nhanh)

```bash
# Quay về commit trước
git log --oneline -10
git checkout <previous-commit-sha>
make rebuild
docker compose up -d server

# Rollback migration (CẨN THẬN)
make migrate-down
```

### 4.4 Zero-downtime deploy

Stack hiện tại chưa có blue/green. Cách giảm downtime:

```bash
# 1. Pull code + build image MỚI trước (container cũ vẫn chạy)
git pull
docker compose build server

# 2. Recreate container (downtime ~5-10s)
docker compose up -d server
```

Nếu muốn zero-downtime thực sự:
- Deploy lên ECS/EKS với rolling update.
- Hoặc tách Go server thành deployment riêng (Kubernetes).
- Hoặc tách Next.js thành deployment riêng.

### 4.5 Cập nhật `.env` (không rebuild)

```bash
# Sửa .env
nano .env

# Apply cho containers (không cần rebuild)
docker compose up -d

# Hoặc restart riêng service
docker compose restart server
```

---

## 5. Backup & Restore

### 5.1 Postgres

```bash
# --- BACKUP ---
docker exec dongdo_postgres pg_dump -U postgres dongdo_cs \
  | gzip > /backups/postgres-$(date +%F).sql.gz

# Verify backup hợp lệ
gunzip -c /backups/postgres-2026-09-09.sql.gz | head -20

# --- RESTORE ---
gunzip -c /backups/postgres-2026-09-09.sql.gz \
  | docker exec -i dongdo_postgres psql -U postgres -d dongdo_cs
```

### 5.2 Redis

Redis chỉ chứa ephemeral state (cache, WebSocket presence, stream pending
entries). Backup **không bắt buộc** nhưng nên làm:

```bash
# --- BACKUP ---
docker exec dongdo_redis redis-cli BGSAVE
docker cp dongdo_redis:/data/dump.rdb /backups/redis-$(date +%F).rdb
# (Hoặc copy AOF nếu bật appendonly)
docker cp dongdo_redis:/data/appendonly.aof /backups/

# --- RESTORE ---
docker cp /backups/redis-2026-09-09.rdb dongdo_redis:/data/dump.rdb
docker compose restart redis
```

### 5.3 Qdrant (vector DB)

```bash
# --- BACKUP ---
docker exec dongdo_qdrant tar czf - /qdrant/storage \
  | cat > /backups/qdrant-$(date +%F).tar.gz

# --- RESTORE ---
cat /backups/qdrant-2026-09-09.tar.gz \
  | docker exec -i dongdo_qdrant tar xzf - -C /
docker compose restart qdrant
```

### 5.4 Recordings (cuộc gọi)

```bash
# --- BACKUP ---
tar czf /backups/recordings-$(date +%F).tgz \
  -C /home/ubuntu/DongDo_CS_V2 recordings/

# Hoặc sync lên S3 (off-host, an toàn hơn)
aws s3 sync \
  /home/ubuntu/DongDo_CS_V2/recordings/ \
  s3://dongdo-recordings-prod/ \
  --storage-class STANDARD_IA \
  --exclude "*.tmp"
```

### 5.5 Let's Encrypt certs (external nginx)

SSL certificates are managed by your **external nginx** installation, not this
Docker stack. Backup your external nginx cert directory:

```bash
# --- BACKUP ---
# Adjust path to match your external nginx cert location
sudo tar czf /backups/letsencrypt-$(date +%F).tgz \
  /etc/letsencrypt/

# --- RESTORE ---
sudo tar xzf /backups/letsencrypt-2026-09-09.tgz -C /
# Reload nginx: sudo nginx -s reload
```

### 5.6 Toàn bộ project state

```bash
# Snapshot toàn bộ bind-mount + config
tar czf /backups/dongdo-state-$(date +%F).tgz \
  -C /home/ubuntu \
  --exclude='DongDo_CS_V2/recordings' \
  --exclude='DongDo_CS_V2/web/node_modules' \
  --exclude='DongDo_CS_V2/web/.next' \
  DongDo_CS_V2/.env
```

### 5.7 Cronjob tự động (khuyến nghị)

```bash
sudo crontab -e
# Thêm:
0 2 * * * /home/ubuntu/DongDo_CS_V2/scripts/backup.sh
```

`scripts/backup.sh`:
```bash
#!/bin/bash
set -euo pipefail
BACKUP_DIR=/backups
PROJECT_DIR=/home/ubuntu/DongDo_CS_V2
DATE=$(date +%F)
mkdir -p "$BACKUP_DIR"

# Postgres
docker exec dongdo_postgres pg_dump -U postgres dongdo_cs \
  | gzip > "$BACKUP_DIR/postgres-$DATE.sql.gz"

# Qdrant
docker exec dongdo_qdrant tar czf - /qdrant/storage \
  > "$BACKUP_DIR/qdrant-$DATE.tar.gz"

# Recordings (incremental — chỉ file mới)
rsync -a --delete "$PROJECT_DIR/recordings/" "$BACKUP_DIR/recordings/"

# Retention: giữ 30 ngày
find "$BACKUP_DIR" -name "*.sql.gz" -mtime +30 -delete
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +30 -delete
```

---

## 6. Scaling & Performance tuning

### 6.1 Vertical scaling (đơn giản nhất)

Đổi EC2 instance type khi CPU/RAM cao:

| Stage | Type | vCPU | RAM | Use case |
|---|---|---|---|---|
| Dev/test | t3.small | 2 | 2 GB | Chỉ test |
| Small prod | t3.medium | 2 | 4 GB | < 50 concurrent users |
| Mid prod | t3.large | 2 | 8 GB | 50–200 concurrent users |
| Large prod | t3.xlarge | 4 | 16 GB | 200–500 concurrent users |
| Enterprise | t3.2xlarge | 8 | 32 GB | > 500 concurrent users |

Stop instance → change type → start. Container restart tự động.

### 6.2 Horizontal scaling (multi-instance)

**Hiện tại chưa support tự động.** Để scale horizontal cần:

1. Tách Postgres ra RDS/managed service (không share volume).
2. Dùng Redis Cluster hoặc ElastiCache.
3. Qdrant cluster mode (thay single node).
4. nginx upstream với nhiều server instances:
   ```nginx
   upstream dongdo_backend {
       least_conn;
       server server_a:8080;
       server server_b:8080;
       keepalive 32;
   }
   ```
5. Move stateful services (postgres/redis/qdrant) sang managed:
   - **RDS Postgres** thay `dongdo_postgres`.
   - **ElastiCache Redis** thay `dongdo_redis`.
   - **Qdrant Cloud** hoặc self-hosted cluster.

### 6.3 Tuning Postgres cho traffic cao

```bash
# Sửa command trong docker-compose.yml:
command:
  - "postgres"
  - "-c"
  - "shared_preload_libraries=pg_stat_statements"
  - "-c"
  - "max_connections=200"                    # mặc định 100
  - "-c"
  - "shared_buffers=2GB"                     # ~25% RAM
  - "-c"
  - "effective_cache_size=6GB"               # ~75% RAM
  - "-c"
  - "work_mem=64MB"
  - "-c"
  - "maintenance_work_mem=512MB"
  - "-c"
  - "random_page_cost=1.1"                   # nếu dùng SSD
  - "-c"
  - "effective_io_concurrency=200"
  - "-c"
  - "wal_compression=on"
```

### 6.4 Tuning Go server

Env vars trong `.env`:

```bash
# Worker tuning (Redis Streams)
WORKER_DB_BATCH_SIZE=100            # tăng từ 50 → batch writes nhanh hơn
WORKER_DB_FLUSH_INTERVAL_MS=1000    # flush thường xuyên hơn
WORKER_AI_READ_COUNT=1              # giữ 1 để AI xử lý sequential (rate-limited bởi API)
REDIS_POOL_SIZE=50                  # tăng từ 20
HTTP_READ_TIMEOUT_SEC=60
HTTP_WRITE_TIMEOUT_SEC=60
```

Restart:
```bash
docker compose up -d server
```

### 6.5 Tuning external nginx

Configuration for the external nginx reverse proxy is **outside this Docker stack**.
Refer to your external nginx configuration for:
- Worker processes and connections
- Rate limiting zones (`limit_req`)
- Buffer sizes
- Gzip compression
- Caching headers

If your external nginx proxies to this stack, recommended upstream config:
```nginx
# Point to Docker host IP (where port 3000 is published)
upstream dongdo_web {
    server <EC2_PRIVATE_IP>:3000;
}

# Point to Docker host IP (where port 8080 is published)
upstream dongdo_backend {
    server <EC2_PRIVATE_IP>:8080;
}
```

### 6.6 CDN / CloudFront

Đặt CloudFront trước nginx để cache static assets:

```
User → CloudFront → external nginx (origin) → web/server
```

Pattern:
- Cache `/_next/static/*` TTL 1 năm.
- Cache `/static/*` TTL 7 ngày.
- Không cache `/api/*`, `/auth/*`, `/ws`, `/chat` (dynamic).

---

## 7. Monitoring & Alerting

### 7.1 Truy cập nhanh

| Tool | URL (trên host) | URL (qua nginx) | Credentials |
|---|---|---|---|
| Grafana | `http://localhost:3050` | (chưa expose qua 443) | admin / `$GRAFANA_ADMIN_PASSWORD` |
| Prometheus | `http://localhost:9090` | — | (no auth) |
| Alertmanager | `http://localhost:9093` | — | (no auth) |
| Server /metrics | `http://localhost:9091/metrics` | — | (loopback only) |
| Server /debug/pprof | `http://localhost:6060/debug/pprof/` | — | (loopback only) |

**Từ xa**, dùng SSH tunnel:
```bash
ssh -L 3050:127.0.0.1:3050 -L 9090:127.0.0.1:9090 user@ec2-host
# Mở http://localhost:3050 trên trình duyệt local
```

### 7.2 Dashboards có sẵn (Grafana)

5 dashboard tự động provision:
1. **Infrastructure** — host CPU/memory/disk/network/load.
2. **Golang API** — request rate, error rate, P50/P95/P99, goroutines, heap, GC.
3. **Chat / WebSocket** — active connections, messages/sec, agent states, customers waiting.
4. **Database** — connections, slow queries (`pg_stat_statements`), locks.
5. **Redis** — memory, commands, streams, pool stats.

### 7.3 Alert rules (mặc định)

File `monitoring/prometheus/alert.rules.yml` đã define sẵn:

| Alert | Trigger | Severity |
|---|---|---|
| `HostHighCpuLoad` | CPU > 80% for 10m | warning |
| `HostHighMemoryUsage` | Memory > 85% | warning |
| `HostDiskSpaceLow` | Disk < 15% free | critical |
| `GoServerHighErrorRate` | 5xx > 5% for 5m | critical |
| `GoServerHighP99Latency` | P99 > 2s for 5m | warning |
| `GoGoroutineLeak` | Goroutines > 5000 | warning |
| `PostgresConnectionSaturation` | Used > 80% max_connections | warning |
| `PostgresSlowQueries` | Mean exec > 1s | warning |
| `RedisHighMemoryUsage` | Memory > 80% maxmemory | warning |
| `RedisExporterDown` | exporter unreachable 5m | critical |
| `WebSocketDropRateHigh` | drops > 10/min | warning |
| `SSLCertExpiringSoon` | < 14 days | critical |

### 7.4 Thêm notification channel (Slack/Telegram)

Sửa `monitoring/alertmanager/alertmanager.yml`:

```yaml
receivers:
  - name: 'ops-team'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/XXX/YYY/ZZZ'
        channel: '#dongdo-alerts'
        send_resolved: true
        title: '{{ .CommonAnnotations.summary }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'

route:
  receiver: 'ops-team'
  group_by: ['alertname', 'severity']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
```

Apply:
```bash
docker compose -p dongdo_cs_v2 \
  -f docker-compose.yml \
  -f monitoring/docker-compose.monitoring.yml \
  --profile monitoring restart alertmanager
```

### 7.5 Health check tự động (uptime monitoring)

Dùng [UptimeRobot](https://uptimerobot.com) (free), [Better Stack](https://betterstack.com), hoặc tự host:

```bash
# Health checks cần monitor:
# 1. https://cskh.dongdopartners.com/                    → 200
# 2. https://cskh.dongdopartners.com/api/health          → 200
# 3. http://localhost:9090/-/healthy (qua SSH tunnel)    → 200 (Prometheus)
```

---

## 8. Bảo mật vận hành

### 8.1 Checklist bảo mật NGAY SAU DEPLOY

- [ ] **Đổi tất cả `CHANGEME`** trong `.env`.
- [ ] **Rotate `JWT_SECRET`** sau khi tạo owner đầu tiên (tất cả user phải login lại).
- [ ] **Đổi `POSTGRES_PASSWORD`** + update cả `docker-compose.yml`.
- [ ] **Đổi Grafana admin password** (login lần đầu).
- [ ] **Không expose `:3050`, `:9090`, `:9093` ra public** (chỉ SSH tunnel).
- [ ] **Bật UFW/iptables** (chỉ mở 22, 80, 443 cho public):
  ```bash
  sudo ufw default deny incoming
  sudo ufw default allow outgoing
  sudo ufw allow 22/tcp
  sudo ufw allow 80/tcp
  sudo ufw allow 443/tcp
  sudo ufw enable
  ```
- [ ] **Bật fail2ban**:
  ```bash
  sudo apt install -y fail2ban
  sudo systemctl enable --now fail2ban
  ```
- [ ] **Submit domain vào [hstspreload.org](https://hstspreload.org/)** (sau khi confirm HTTPS hoạt động ổn định 1-2 tuần).
- [ ] **Bật automatic security updates**:
  ```bash
  sudo apt install -y unattended-upgrades
  sudo dpkg-reconfigure -plow unattended-upgrades
  ```
- [ ] **Rotate SSH key** thay vì password login.
- [ ] **Tắt password authentication** trong `/etc/ssh/sshd_config`:
  ```
  PasswordAuthentication no
  PermitRootLogin no
  ```

### 8.2 Audit định kỳ

```bash
# 1. Check containers có update không
docker images --format "{{.Repository}}:{{.Tag}} {{.CreatedSince}}"

# 2. Scan image lỗ hổng
docker scan dongdo-cs-server:latest   # cần login Docker Hub
# Hoặc dùng trivy:
trivy image dongdo-cs-server:latest

# 3. Check ports đang mở public
sudo ss -tlnp | grep -E ':(80|443|3000|3050|8080|8081|8088|9090|9093|5433|6379|6333|6334)'

# 4. Audit external nginx access log cho pattern tấn công
sudo grep -iE '(union|select|script|%3Cscript|admin|wp-login|\.env)' /var/log/nginx/access.log

# 5. Kiểm tra JWT secret đã được rotate chưa
grep -c 'JWT_SECRET' .env
```

### 8.3 Secret rotation

```bash
# 1. JWT_SECRET — mạnh nhất là rotate theo quý
# Tạo secret mới
NEW_JWT=$(openssl rand -base64 48)
sed -i.bak "s|^JWT_SECRET=.*|JWT_SECRET=$NEW_JWT|" .env
docker compose restart server
# → tất cả user phải login lại

# 2. Postgres password — ít khi cần
NEW_PG=$(openssl rand -base64 32)
sed -i "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$NEW_PG|" .env
sed -i "s|postgres:postgrespassword|postgres:$NEW_PG|g" docker-compose.yml
# Cập nhật monitoring exporter DSN
# Restart toàn bộ stack:
make down && make up
make monitoring-up

# 3. Grafana password — đổi trong UI (Administration → Users → admin)
```

### 8.4 Nếu lộ secret

1. **Ngay lập tức** rotate secret bị lộ.
2. **Audit logs** 7 ngày qua xem có dấu hiệu truy cập bất thường.
3. **Nếu JWT_SECRET bị lộ** → tất cả token hiện tại bị coi là compromised → force logout tất cả user.
4. **Nếu ANTHROPIC_API_KEY bị lộ** → revoke key trên console.anthropic.com → tạo key mới.
5. **Nếu POSTGRES_PASSWORD bị lộ** → rotate → check pg_stat_activity xem connection lạ.
6. **Document incident** + thông báo team.

---

## 9. Troubleshooting

### 9.1 Lỗi triển khai / setup

| Triệu chứng | Nguyên nhân | Cách xử lý |
|---|---|---|
| `make monitoring-up` fails `network dongdo_cs_v2_default not found` | Chưa chạy `make up` trước | Chạy `make up` trước |
| `make up` fail ở build stage | Network chậm / Dockerfile lỗi | `docker compose build --no-cache server` |
| Postgres container restart loop | Volume bị corrupt | `make up-fresh` (mất data) |
| `Cannot connect to redis` trong logs | Redis container chưa healthy | `docker logs dongdo_redis`, chờ healthcheck pass |

### 9.2 Lỗi vận hành

| Triệu chứng | Nguyên nhân | Cách xử lý |
|---|---|---|
| AI response chậm (>10s) | LLM API rate-limited hoặc context quá lớn | Check Anthropic console; giảm `LLM_MAX_TOKENS`; check `RETRIEVER_K` |
| Chat message mất | Redis stream bị full / consumer không XACK | `redis-cli XINFO STREAM stream:ws`; check `STREAM_MAX_LEN` |
| Web load chậm | Cold cache / build cache miss | Check CDN caching for static assets; verify Next.js build cache |
| Server OOM killed | Memory leak hoặc traffic spike | `make pprof-heap`; check Grafana "Go API" dashboard |
| Calls không connect được (WebRTC) | STUN unreachable / SIP NAT | Test STUN từ client; check `STUN_SERVERS` env; Asterisk `pjsip show endpoints` |

### 9.3 Lỗi database

```bash
# Connection pool exhausted
docker exec dongdo_postgres psql -U postgres -c \
  "SELECT count(*) FROM pg_stat_activity;"
docker exec dongdo_postgres psql -U postgres -c \
  "SELECT pid, usename, application_name, state, query_start, query \
   FROM pg_stat_activity ORDER BY query_start NULLS LAST LIMIT 20;"

# Kill query treo
docker exec dongdo_postgres psql -U postgres -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity \
   WHERE state = 'active' AND query_start < NOW() - INTERVAL '5 minutes';"

# Disk đầy (WAL không được archive)
docker exec dongdo_postgres psql -U postgres -c \
  "SELECT pg_size_pretty(pg_database_size('dongdo_cs'));"
du -sh /var/lib/docker/volumes/dongdo_cs_v2_pgdata/_data

# Slow queries (xem top 10)
docker exec dongdo_postgres psql -U postgres -d dongdo_cs -c \
  "SELECT substring(query, 1, 100) AS q, calls, \
          round(mean_exec_time::numeric, 1) AS mean_ms, \
          round(total_exec_time::numeric, 1) AS total_ms \
   FROM pg_stat_statements \
   ORDER BY mean_exec_time DESC LIMIT 10;"
```

### 9.4 Lỗi Redis

```bash
# Stream length quá lớn
docker exec dongdo_redis redis-cli XLEN stream:ws
docker exec dongdo_redis redis-cli XINFO STREAM stream:ws

# Pending messages (chưa XACK)
docker exec dongdo_redis redis-cli XPENDING stream:ws ws_group

# Memory
docker exec dongdo_redis redis-cli INFO memory | grep used_memory_human

# Slow log
docker exec dongdo_redis redis-cli SLOWLOG GET 20
```

### 9.5 SSL/TLS (external nginx)

SSL termination is handled by **external nginx** outside this Docker stack.
For nginx/TLS issues, check your external nginx configuration and logs:

```bash
# On the host where external nginx runs:
sudo nginx -t          # validate config
sudo nginx -s reload   # reload after changes
sudo tail -50 /var/log/nginx/error.log
```

### 9.6 Lỗi WebRTC / Asterisk

```bash
# Asterisk status
docker exec dongdo_asterisk asterisk -rx "core show version"
docker exec dongdo_asterisk asterisk -rx "pjsip show endpoints"
docker exec dongdo_asterisk asterisk -rx "core show channels"
docker exec dongdo_asterisk asterisk -rx "module show like res_pjsip"

# Call service health
curl -fsS http://localhost:8081/health
docker logs dongdo_call_service --tail 100

# SIP traffic capture (debug NAT)
docker exec dongdo_asterisk asterisk -rx "pjsip set logger on"
# Xem log realtime
docker logs -f dongdo_asterisk
```

### 9.7 Lỗi monitoring

```bash
# Prometheus không scrape được target
curl -s http://localhost:9090/api/v1/targets | \
  python3 -c "import json,sys; \
  [print(f\"{t['labels'].get('job','?'):20s} {'UP' if t['health']=='up' else 'DOWN':5s} {t.get('lastError','')[:80]}\") \
   for t in json.load(sys.stdin)['data']['activeTargets']]"

# Reload Prometheus sau khi sửa alert rules
make mon-reload-prom

# Validate alert rules syntax
make mon-validate-prom

# Grafana datasource không kết nối
# Vào Grafana → Connections → Data sources → Prometheus → Test connection
```

---

## 10. Disaster Recovery

### 10.1 RPO / RTO targets

| Tier | Service | RPO (data loss tối đa) | RTO (downtime tối đa) |
|---|---|---|---|
| Critical | Postgres | 1 giờ (với hourly backup) | 30 phút |
| Critical | Recordings | 24 giờ | 4 giờ |
| Important | Redis | 0 (ephemeral, rebuild OK) | 5 phút |
| Important | Qdrant | 24 giờ (daily backup) | 1 giờ |
| Replaceable | Let's Encrypt | 0 (auto-renew, 90 ngày TTL) | 1 phút |

### 10.2 Kịch bản EC2 mất hoàn toàn

Giả định có backup toàn bộ `/backups/` lên S3 off-host.

```bash
# 1. Spin up EC2 mới, cài Docker + Compose v2
# 2. Pull code
git clone https://github.com/yourorg/DongDo_CS_V2.git
cd DongDo_CS_V2

# 3. Restore config
tar xzf /backups/dongdo-state-2026-09-09.tgz
# (Hoặc restore file .env riêng)

# 4. Restore Postgres
docker compose up -d postgres
# Đợi healthy (~30s)
gunzip -c /backups/postgres-2026-09-09.sql.gz \
  | docker exec -i dongdo_postgres psql -U postgres -d dongdo_cs

# 5. Restore Qdrant
cat /backups/qdrant-2026-09-09.tar.gz \
  | docker exec -i dongdo_qdrant tar xzf - -C /

# 6. Restore Redis (nếu cần)
docker cp /backups/redis-2026-09-09.rdb dongdo_redis:/data/dump.rdb

# 7. Khởi động toàn bộ
make up
make monitoring-up

# 8. Update DNS nếu IP đổi
# (DNS A record → IP mới của EC2)

# 9. Verify
make health
make mon-status
```

### 10.3 Kịch bản Postgres corrupt

```bash
# 1. Stop server (giảm write conflict)
docker compose stop server

# 2. Stop postgres, wipe volume, restart từ backup
docker compose stop postgres
docker volume rm dongdo_cs_v2_pgdata
docker compose up -d postgres
# Đợi healthy

# 3. Restore
gunzip -c /backups/postgres-LATEST.sql.gz \
  | docker exec -i dongdo_postgres psql -U postgres -d dongdo_cs

# 4. Restart server
docker compose up -d server
make health
```

### 10.4 Kịch bản recordings bị mất

Recordings là data ít quan trọng hơn (có thể replay cuộc gọi). Nếu mất:

1. Xác nhận với khách hàng nếu cần (không cam kết 100% giữ).
2. Restore từ S3 backup gần nhất.
3. Document incident trong `/backups/INCIDENTS.md`.

---

## 11. Phụ lục — Cheat sheet

### 11.1 Lệnh hàng ngày

```bash
# Khởi động
make up
make monitoring-up

# Dừng (giữ data)
make down
make monitoring-down

# Restart service
docker compose restart <service>

# Xem logs
make logs
make mon-logs

# Health check
make health
```

### 11.2 Lệnh debug

```bash
# CPU profile (30s)
make pprof-cpu
# Trong prompt: top 20, web, quit

# Heap snapshot
make pprof-heap

# Goroutine dump
make pprof-goroutine

# Slow SQL queries
# Mở Prometheus → query: pg_stat_statements_mean_exec_time_seconds

# WebSocket connections
# Mở Grafana → Chat / WebSocket dashboard
```

### 11.3 Lệnh backup / restore

```bash
# Backup
docker exec dongdo_postgres pg_dump -U postgres dongdo_cs \
  | gzip > postgres-$(date +%F).sql.gz
docker exec dongdo_qdrant tar czf - /qdrant/storage \
  > qdrant-$(date +%F).tar.gz

# Restore
gunzip -c postgres-2026-09-09.sql.gz \
  | docker exec -i dongdo_postgres psql -U postgres -d dongdo_cs
```

### 11.4 Lệnh maintenance

```bash
# Rebuild image sau khi đổi code
make rebuild                          # server
make web-docker-build                 # web

# Xem disk usage
docker system df
df -h

# Cleanup docker
docker system prune -af               # ⚠️ không prune volumes

# Restart toàn bộ
make down && make up && make monitoring-up
```

### 11.5 Biến môi trường quan trọng

| Biến | Mặc định | Mục đích |
|---|---|---|
| `JWT_SECRET` | — | **MUST rotate** |
| `POSTGRES_PASSWORD` | `postgrespassword` | **MUST change** (cả `.env` + `docker-compose.yml`) |
| `GRAFANA_ADMIN_PASSWORD` | `admin` | **MUST change** |
| `APP_ENV` | `development` | Set `production` |
| `COOKIE_SECURE` | `false` | Set `true` với HTTPS |
| `ANTHROPIC_API_KEY` | — | **MUST provide** |
| `NGINX_SERVER_NAME` | `cskh.dongdopartners.com` | Used by external nginx (not this stack) |
| `NGINX_ENABLE_SSL` | `true` | Used by external nginx (not this stack) |

### 11.6 URLs tham chiếu nhanh

| Service | URL |
|---|---|
| App (via external nginx → Next.js) | `https://cskh.dongdopartners.com` |
| API (via external nginx → Go server) | `https://cskh.dongdopartners.com/api/*` |
| Grafana | `https://<EC2_IP>:3050` (qua SSH tunnel nếu SG chặn) |
| Prometheus | `http://<EC2_IP>:9090` (qua SSH tunnel) |
| Alertmanager | `http://<EC2_IP>:9093` (qua SSH tunnel) |
| Server /metrics | `http://127.0.0.1:9091/metrics` (loopback only) |
| Server pprof | `http://127.0.0.1:6060/debug/pprof/` (loopback only) |
| Asterisk | `http://<EC2_IP>:8088` |
| Postgres | `localhost:5433` (loopback, dev only) |
| Redis | `localhost:6379` (loopback, dev only) |
| Qdrant | `localhost:6333` (loopback, dev only) |

### 11.7 Liên hệ / Escalation

| Mức độ | Người | Liên hệ |
|---|---|---|
| L1 — Service down | DevOps on-call | +84 xxx |
| L2 — Data loss / security | Tech Lead | +84 xxx |
| L3 — Multi-service outage | CTO | +84 xxx |
| Cloud provider (AWS) | Support plan | Case qua console |
| LLM provider outage | Anthropic status | status.anthropic.com |

---

## Tài liệu liên quan

- [`README.md`](README.md) — Tổng quan kiến trúc, dev workflow.
- [`monitoring/README.md`](monitoring/README.md) — Chi tiết về monitoring stack.
- [`PRODUCTION.md`](PRODUCTION.md) — File này (operations runbook).
- [`tests/automation/README.md`](tests/automation/README.md) — Load test scripts.

# 🚀 Deployment — Triển khai Production

> **Phiên bản:** v2.0
> **Đối tượng:** DevOps engineers, SREs
> **Cập nhật lần cuối:** Sep 2026

## Mục lục

1. [Yêu cầu hệ thống](#1-yêu-cầu-hệ-thống)
2. [Topology production](#2-topology-production)
3. [Cài đặt ban đầu](#3-cài-đặt-ban-đầu)
4. [SSL/TLS cho SIP](#4-ssltls-cho-sip)
5. [Firewall rules](#5-firewall-rules)
6. [DNS & SRV records](#6-dns--srv-records)
7. [Monitoring](#7-monitoring)
8. [Backup strategy](#8-backup-strategy)
9. [Update procedure](#9-update-procedure)
10. [Disaster recovery](#10-disaster-recovery)
11. [Cross-references](#11-cross-references)

---

## 1. Yêu cầu hệ thống

### 1.1 Minimum (single host, ≤50 concurrent users)

| Resource | Min | Recommended |
|---|---|---|
| **CPU** | 2 vCPU | 4 vCPU (Intel Xeon / AMD EPYC) |
| **RAM** | 4 GB | 8 GB |
| **Disk** | 50 GB SSD | 100 GB NVMe SSD |
| **Network** | 100 Mbps | 1 Gbps |
| **OS** | Ubuntu 22.04 LTS / Debian 12 | Ubuntu 24.04 LTS |

### 1.2 Production (multi-host, 200+ users)

| Service | Host | Spec |
|---|---|---|
| **App server** (Go backend) | x2 | 4 vCPU, 8 GB RAM mỗi host |
| **Web** (Next.js) | x2 | 2 vCPU, 4 GB RAM |
| **Postgres** | x1 primary + x1 replica | 4 vCPU, 16 GB RAM, 100 GB SSD |
| **Redis** | x1 master + x1 replica | 2 vCPU, 4 GB RAM |
| **Qdrant** | x1 | 4 vCPU, 16 GB RAM, vector capacity ~500k |
| **Asterisk** | x1 | 4 vCPU, 8 GB RAM (call capacity ~500/cpu core) |
| **Recording storage** | S3 / MinIO | object storage, 1 TB |

### 1.3 Software dependencies

```bash
# Trên host
docker --version    # ≥ 24.0
docker compose version  # ≥ v2.20

# Recommended
ufw            # firewall
fail2ban       # brute-force protection
nginx          # reverse proxy (optional, cho HTTPS frontend)
certbot        # Let's Encrypt
```

---

## 2. Topology production

```mermaid
flowchart TB
    subgraph Internet
        GUEST[Guest<br/>Browser]
        ADMIN[Admin<br/>Browser]
        SIP[Agent SIP<br/>Client]
        ITSP[ITSP<br/>SIP Trunk]
    end

    subgraph DMZ
        LB[Nginx<br/>:443 :80<br/>SSL termination]
        FW[Firewall<br/>UFW]
    end

    subgraph App_Net["App Network (10.10.0.0/24)"]
        APP1[App server 1<br/>Go backend :8080]
        APP2[App server 2<br/>Go backend :8080]
        WEB1[Web :3000]
        WEB2[Web :3000]
    end

    subgraph Data_Net["Data Network (10.20.0.0/24)"]
        PG[(Postgres<br/>primary)]
        PGR[(Postgres<br/>replica)]
        RD[(Redis<br/>master)]
        RDR[(Redis<br/>replica)]
        QD[(Qdrant)]
    end

    subgraph Tel_Net["Telephony Network (10.30.0.0/24)"]
        AST1[Asterisk 1<br/>PJSIP + AMI + ARI]
        AST2[Asterisk 2<br/>failover]
    end

    subgraph Storage
        S3[(S3 / MinIO<br/>recordings)]
    end

    GUEST --> LB
    ADMIN --> LB
    LB --> WEB1
    LB --> WEB2
    WEB1 --> APP1
    WEB2 --> APP2

    SIP -->|SIP TLS| FW
    ITSP -->|SIP TLS| FW
    FW --> AST1
    FW --> AST2

    APP1 --> PG
    APP2 --> PG
    APP1 --> RD
    APP2 --> RD
    APP1 --> QD
    APP2 --> QD

    APP1 -->|AMI TCP| AST1
    APP2 -->|AMI TCP| AST1

    AST1 --> S3
    AST2 --> S3
```

---

## 3. Cài đặt ban đầu

### 3.1 Chuẩn bị server

```bash
# Update OS
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# Install certbot
sudo apt install -y certbot

# Create project dir
sudo mkdir -p /opt/dongdo
sudo chown $USER:$USER /opt/dongdo
cd /opt/dongdo
```

### 3.2 Clone & configure

```bash
git clone <repo-url> .
cp .env.example .env
nano .env  # điền ANTHROPIC_API_KEY, JWT_SECRET, AMI_PASS, ...
```

**Generate secrets:**

```bash
# JWT secret (≥32 chars)
openssl rand -base64 48

# AMI password
openssl rand -base64 24

# Postgres password
openssl rand -base64 24
```

### 3.3 Pull & start

```bash
docker compose pull
docker compose up -d
```

### 3.4 Verify

```bash
# Health
curl https://your-domain.com/api/health

# Asterisk
docker compose exec asterisk asterisk -rx "pjsip show endpoints"

# Database
docker compose exec postgres psql -U postgres -d dongdo_cs -c "SELECT COUNT(*) FROM users;"

# Ingest knowledge base
docker compose exec server /app/ingest
```

---

## 4. SSL/TLS cho SIP

### 4.1 Generate certificate (Let's Encrypt)

```bash
sudo certbot certonly --standalone -d sip.your-domain.com
```

Cert sẽ ở `/etc/letsencrypt/live/sip.your-domain.com/fullchain.pem`.

### 4.2 Mount cert vào Asterisk container

Thêm vào `docker-compose.yml`:

```yaml
services:
  asterisk:
    volumes:
      - /etc/letsencrypt:/etc/letsencrypt:ro
      - ./docker/asterisk/etc/asterisk/keys:/etc/asterisk/keys:ro
```

### 4.3 Symlink certs

```bash
mkdir -p ./docker/asterisk/etc/asterisk/keys
ln -sf /etc/letsencrypt/live/sip.your-domain.com/fullchain.pem \
       ./docker/asterisk/etc/asterisk/keys/asterisk.pem
ln -sf /etc/letsencrypt/live/sip.your-domain.com/privkey.pem \
       ./docker/asterisk/etc/asterisk/keys/asterisk.key
```

### 4.4 Auto-renewal

```bash
# /etc/cron.d/certbot-renew
0 3 * * * certbot renew --quiet --deploy-hook "docker compose -f /opt/dongdo/docker-compose.yml restart asterisk"
```

### 4.5 PJSIP TLS transport

`pjsip.conf`:

```ini
[transport-tls]
type=transport
protocol=tls
bind=0.0.0.0:5061
cert_file=/etc/asterisk/keys/asterisk.pem
priv_key_file=/etc/asterisk/keys/asterisk.key
method=tlsv1_2
verify_client=no      ; cho phép self-signed client certs
cipher=ECDHE+AES128
```

---

## 5. Firewall rules

### 5.1 UFW

```bash
# Reset
sudo ufw --force reset

# Default
sudo ufw default deny incoming
sudo ufw default allow outgoing

# SSH
sudo ufw allow 22/tcp

# HTTP / HTTPS (frontend)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Backend API (nếu không qua nginx)
# sudo ufw allow 8080/tcp

# SIP signaling
sudo ufw allow 5060/udp    # SIP
sudo ufw allow 5061/tcp    # SIPS/TLS

# AMI (chỉ từ app network)
sudo ufw allow from 10.10.0.0/24 to any port 5038 proto tcp

# ARI (optional, chỉ từ app network)
sudo ufw allow from 10.10.0.0/24 to any port 8088 proto tcp

# RTP media range
sudo ufw allow 10000:20000/udp

# Postgres (chỉ từ data network)
sudo ufw allow from 10.10.0.0/24 to any port 5432 proto tcp

# Redis (chỉ từ app network)
sudo ufw allow from 10.10.0.0/24 to any port 6379 proto tcp

# Qdrant
sudo ufw allow from 10.10.0.0/24 to any port 6333 proto tcp

# Enable
sudo ufw enable
sudo ufw status verbose
```

### 5.2 Port reference

| Port | Protocol | Service | Public? |
|---|---|---|---|
| 22 | TCP | SSH | Yes |
| 80 | TCP | HTTP (redirect → 443) | Yes |
| 443 | TCP | HTTPS (nginx) | Yes |
| 3000 | TCP | Next.js dev (internal) | No |
| 5060 | UDP | SIP | Yes |
| 5061 | TCP | SIPS/TLS | Yes |
| 5038 | TCP | AMI (App → Asterisk) | Internal only |
| 8080 | TCP | Go backend (App) | Optional (via nginx) |
| 8088 | TCP | ARI | Internal only |
| 5432 | TCP | Postgres | Internal only |
| 5433 | TCP | Postgres (host mapping, dev) | Local only |
| 6333 | TCP | Qdrant HTTP | Internal only |
| 6334 | TCP | Qdrant gRPC | Internal only |
| 6379 | TCP | Redis | Internal only |
| 10000-20000 | UDP | RTP media | Yes |

---

## 6. DNS & SRV records

### 6.1 A / AAAA records

```dns
sip.your-domain.com.       A    <server_public_ip>
api.your-domain.com.       A    <server_public_ip>
admin.your-domain.com.     A    <server_public_ip>
```

### 6.2 SIP SRV records (recommended)

```dns
_sips._tcp.your-domain.com.    86400 IN SRV 10 10 5061 sip.your-domain.com.
_sip._udp.your-domain.com.     86400 IN SRV 10 10 5060 sip.your-domain.com.
```

> SRV giúp SIP clients auto-discover server qua DNS thay vì hardcode IP.

### 6.3 NAPTR (optional)

```dns
your-domain.com.    86400 IN NAPTR 10 10 "s" "SIPS+D2T" "" _sips._tcp.your-domain.com.
your-domain.com.    86400 IN NAPTR 20 10 "s" "SIP+D2U" "" _sip._udp.your-domain.com.
```

---

## 7. Monitoring

### 7.1 Prometheus exporters

| Service | Exporter | Port |
|---|---|---|
| Host (CPU/RAM/disk) | `node_exporter` | 9100 |
| Postgres | `postgres_exporter` | 9187 |
| Redis | `redis_exporter` | 9121 |
| Asterisk | `asterisk_exporter` | 9680 |
| Qdrant | built-in | 6333 /metrics |
| Go backend | custom `/metrics` | 8080 |

`docker-compose.monitoring.yml`:

```yaml
version: '3.8'
services:
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3001:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASS}
    volumes:
      - grafana_data:/var/lib/grafana

  node-exporter:
    image: prom/node-exporter:latest
    ports:
      - "9100:9100"

  postgres-exporter:
    image: prometheuscommunity/postgres-exporter:latest
    environment:
      DATA_SOURCE_NAME: "postgresql://postgres:${POSTGRES_PASSWORD}@postgres:5432/dongdo_cs?sslmode=disable"
    ports:
      - "9187:9187"

  redis-exporter:
    image: oliver006/redis_exporter:latest
    environment:
      REDIS_ADDR: redis:6379
    ports:
      - "9121:9121"

volumes:
  grafana_data:
```

### 7.2 Prometheus config

`monitoring/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'node'
    static_configs:
      - targets: ['node-exporter:9100']

  - job_name: 'postgres'
    static_configs:
      - targets: ['postgres-exporter:9187']

  - job_name: 'redis'
    static_configs:
      - targets: ['redis-exporter:9121']

  - job_name: 'asterisk'
    static_configs:
      - targets: ['host.docker.internal:9680']

  - job_name: 'dongdo-backend'
    static_configs:
      - targets: ['server:8080']
```

### 7.3 Grafana dashboards

Import các dashboard ID:

| Dashboard | ID | Source |
|---|---|---|
| Node Exporter Full | 1860 | grafana.com |
| Postgres Overview | 9628 | grafana.com |
| Redis Dashboard | 11835 | grafana.com |
| Asterisk | custom | repo (see below) |

**Custom Asterisk dashboard** (xem `monitoring/grafana/asterisk-dashboard.json`):

Panels:
- Active calls (from AMI `CoreShowChannels`)
- Queue members & status
- Call duration histogram
- Recording disk usage

### 7.4 Alerts

`monitoring/alerts.yml`:

```yaml
groups:
  - name: dongdo-critical
    rules:
      - alert: HighCPUUsage
        expr: 100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 85
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "CPU > 85% on {{ $labels.instance }}"

      - alert: PostgresConnectionsExhausted
        expr: pg_stat_activity_count > 200
        for: 2m
        labels:
          severity: critical

      - alert: RedisDown
        expr: redis_up == 0
        for: 1m
        labels:
          severity: critical

      - alert: AMIDisconnected
        expr: asterisk_ami_connected == 0
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "Backend mất kết nối AMI"

      - alert: QueueLengthHigh
        expr: asterisk_queue_calls_waiting > 5
        for: 2m
        labels:
          severity: warning

      - alert: RecordingDiskLow
        expr: (node_filesystem_avail_bytes{mountpoint="/var/spool/asterisk/monitor"} / node_filesystem_size_bytes) < 0.1
        for: 5m
        labels:
          severity: warning
```

---

## 8. Backup strategy

### 8.1 Postgres dump (daily)

`/opt/dongdo/scripts/backup-postgres.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR=/var/backups/dongdo/postgres
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RETAIN_DAYS=30

mkdir -p "$BACKUP_DIR"

# Custom format dump (compressed, supports PITR)
docker compose exec -T postgres pg_dump -U postgres -Fc dongdo_cs \
    > "$BACKUP_DIR/dongdo_$TIMESTAMP.dump"

# Cleanup old
find "$BACKUP_DIR" -name "*.dump" -mtime +$RETAIN_DAYS -delete

echo "[$(date)] Backup complete: dongdo_$TIMESTAMP.dump"
```

```bash
chmod +x /opt/dongdo/scripts/backup-postgres.sh

# Cron daily at 2 AM
echo "0 2 * * * /opt/dongdo/scripts/backup-postgres.sh" | sudo crontab -
```

### 8.2 Restore Postgres

```bash
# Stop server (để tránh write conflicts)
docker compose stop server

# Restore
cat /var/backups/dongdo/postgres/dongdo_20240904.dump | \
    docker compose exec -T postgres pg_restore -U postgres -d dongdo_cs --clean

# Restart
docker compose start server
```

### 8.3 Recording → S3

`/opt/dongdo/scripts/sync-recordings-s3.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

LOCAL_DIR=/opt/dongdo/recordings
S3_BUCKET=s3://dongdo-recordings/

# Sync (delete remote files older than 90 days)
aws s3 sync "$LOCAL_DIR" "$S3_BUCKET" \
    --storage-class STANDARD_IA \
    --exclude "*.tmp"

# Delete local files older than 7 days
find "$LOCAL_DIR" -type f -mtime +7 -delete

echo "[$(date)] Sync complete"
```

```bash
# Cron mỗi 6h
echo "0 */6 * * * /opt/dongdo/scripts/sync-recordings-s3.sh" | sudo crontab -
```

### 8.4 Redis backup

`/opt/dongdo/scripts/backup-redis.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR=/var/backups/dongdo/redis
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

docker compose exec -T redis redis-cli BGSAVE
sleep 5
docker compose cp redis:/data/dump.rdb "$BACKUP_DIR/dump_$TIMESTAMP.rdb"

echo "[$(date)] Redis backup complete"
```

### 8.5 Offsite replication

```bash
# Postgres
rclone sync /var/backups/dongdo remote:backups/dongdo

# Recordings (đã sync S3 ở §8.3)
```

---

## 9. Update procedure

### 9.1 Zero-downtime blue-green

```bash
# 1. Pull new images
cd /opt/dongdo
git pull
docker compose pull

# 2. Build new app image
docker compose build server

# 3. Start new instance on different port (blue)
SERVER_PORT=8081 docker compose up -d server-blue

# 4. Health check
sleep 30
curl -f http://localhost:8081/health || {
    echo "Blue failed health check"
    docker compose logs server-blue
    exit 1
}

# 5. Switch nginx upstream
sudo sed -i 's/server localhost:8080/server localhost:8081/' /etc/nginx/sites-enabled/dongdo.conf
sudo nginx -s reload

# 6. Stop old instance
docker compose stop server

# 7. Cleanup
docker compose rm server-blue
```

### 9.2 Rolling restart (for Asterisk)

> ⚠️ **Asterisk restart gây mất tất cả active calls**. Plan trong maintenance window.

```bash
# Drain traffic
docker compose exec server sh -c 'curl -X POST localhost:8080/api/admin/system-maintenance -d "enabled=true"'

# Wait for calls to finish (or force disconnect)
sleep 300  # 5 minutes

# Restart
docker compose restart asterisk

# Verify
docker compose exec asterisk asterisk -rx "pjsip show endpoints"
docker compose exec asterisk asterisk -rx "core show version"
```

### 9.3 Migration upgrade

```bash
# Pull new code
git pull

# New migrations tự chạy khi server start (goose embedded)
docker compose up -d --build server

# Check migration status
docker compose exec server /app/server --migrate-status
```

Rollback migration (CHỈ dev):

```bash
make migrate-down
```

### 9.4 Frontend update

```bash
docker compose build web
docker compose up -d web
```

---

## 10. Disaster recovery

### 10.1 RTO / RPO targets

| Tier | RTO (Recovery Time Objective) | RPO (Recovery Point Objective) |
|---|---|---|
| Critical (Postgres, Redis) | 1 hour | 15 minutes |
| Important (recordings) | 24 hours | 6 hours |
| Standard (logs, metrics) | 72 hours | 24 hours |

### 10.2 Scenarios

#### A. Single host crash

1. Provision replacement server
2. Restore Postgres from S3 (≤15min old backup)
3. Restore Redis dump
4. Pull Docker images, run `docker compose up -d`
5. Mount latest recordings from S3

#### B. Postgres data corruption

```bash
# Promote replica
docker compose exec postgres-replica pg_ctl promote

# Update app to point to replica
# Edit .env: DATABASE_URL=postgres://postgres-replica:5432/dongdo_cs
docker compose up -d server

# Rebuild primary from replica later
```

#### C. Asterisk full loss

1. Recordings persist on S3 — restore by re-running `sync-recordings-s3.sh`
2. Call history in Postgres — backed up via daily dump
3. Config — version-controlled in repo, just `git pull && docker compose restart asterisk`

### 10.3 Backup verification

```bash
# Monthly restore test
# 1. Spin up separate host
# 2. Restore latest dump
# 3. Run smoke test
./scripts/smoke-test.sh --target https://restore-test.internal

# 4. Compare data integrity
docker compose exec postgres psql -U postgres -d dongdo_cs -c "
SELECT 
  (SELECT COUNT(*) FROM chat_messages) AS messages,
  (SELECT COUNT(*) FROM chat_cases) AS cases,
  (SELECT COUNT(*) FROM voice_calls) AS calls;
"
```

---

## 11. Cross-references

- [ARCHITECTURE.md §12 Scaling](./ARCHITECTURE.md#12-scaling-considerations)
- [TELEPHONY.md §11 Troubleshooting](./TELEPHONY.md#11-troubleshooting)
- [CONFIGURATION.md](./CONFIGURATION.md) — env vars
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
- [CHANGELOG.md](../CHANGELOG.md) — version notes

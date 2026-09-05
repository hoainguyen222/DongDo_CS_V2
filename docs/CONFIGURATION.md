# 🔧 Configuration — Tất cả tuỳ chọn cấu hình

> **Phiên bản:** v2.0
> **Đối tượng:** DevOps, Backend devs
> **Cập nhật lần cuối:** Sep 2026

## Mục lục

1. [Environment variables](#1-environment-variables)
2. [Asterisk configuration](#2-asterisk-configuration)
3. [PJSIP endpoint options](#3-pjsip-endpoint-options)
4. [Redis Stream configuration](#4-redis-stream-configuration)
5. [Worker tuning](#5-worker-tuning)
6. [Database configuration](#6-database-configuration)
7. [Frontend (Next.js) configuration](#7-frontend-nextjs-configuration)
8. [Cross-references](#8-cross-references)

---

## 1. Environment variables

Load từ file `.env` (qua `godotenv`) hoặc OS environment. Xem `internal/config/config.go`.

### 1.1 Server

| Biến | Default | Required | Mô tả |
|---|---|---|---|
| `PORT` | `8080` | ❌ | HTTP port backend |
| `SERVER_HOST` | `0.0.0.0` | ❌ | Bind address |
| `APP_ENV` | `development` | ❌ | `development` / `production` |
| `ADMIN_PATH` | `/admin` | ❌ | Server-side admin path prefix |
| `ENABLE_BOOTSTRAP` | `false` | ❌ | Cho phép tạo owner account qua `/admin/setup` |

### 1.2 Database

| Biến | Default | Required | Mô tả |
|---|---|---|---|
| `DATABASE_URL` | `postgres://localhost:5432/dongdo_cs?sslmode=disable` | ✅ | PostgreSQL DSN |

DSN format: `postgres://user:password@host:port/dbname?sslmode=disable`

Production: `postgres://user:pass@db.internal:5432/dongdo_cs?sslmode=require`.

### 1.3 Redis

| Biến | Default | Required | Mô tả |
|---|---|---|---|
| `REDIS_URL` | _(empty)_ | ❌ | Nếu trống → NoOp event bus fallback |

Examples:
- Local: `redis://localhost:6379`
- Upstash: `rediss://default:xxxxx@xxxxx.upstash.io:6379`
- Cluster: dùng sentinel/cluster URL

### 1.4 Qdrant

| Biến | Default | Required | Mô tả |
|---|---|---|---|
| `QDRANT_HOST` | `localhost` | ✅ | Qdrant gRPC host |
| `QDRANT_PORT` | `6334` | ❌ | Qdrant gRPC port (HTTP port = port - 1) |

### 1.5 LLM (Claude)

| Biến | Default | Required | Mô tả |
|---|---|---|---|
| `ANTHROPIC_API_KEY` | _(empty)_ | ✅ | Claude API key |
| `ANTHROPIC_WORKSPACE_ID` | _(empty)_ | ❌ | Workspace ID cho Claude |
| `OPENAI_API_KEY` | _(empty)_ | ❌ | Fallback LLM |
| `GEMINI_API_KEY` | _(empty)_ | ❌ | Fallback LLM + STT cho voice |
| `LLM_MODEL` | `claude-haiku-4-5-20251001` | ❌ | Model Claude |
| `LLM_TEMPERATURE` | `0.1` | ❌ | Sampling temperature |
| `LLM_MAX_TOKENS` | `4096` | ❌ | Max output tokens |

### 1.6 Embedding

| Biến | Default | Required | Mô tả |
|---|---|---|---|
| `EMBEDDING_MODEL` | `sentence-transformers/all-MiniLM-L6-v2` | ✅ | Embedding model (384 dim) |

> ⚠️ Vector dim trong Qdrant phải match `EMBEDDING_MODEL` output dim.

### 1.7 RAG parameters

| Biến | Default | Mô tả |
|---|---|---|
| `CHUNK_SIZE` | `800` | Số ký tự mỗi chunk tài liệu |
| `CHUNK_OVERLAP` | `200` | Overlap giữa các chunks |
| `RETRIEVER_K` | `5` | Số chunks retrieve mỗi query |
| `MEMORY_WINDOW_SIZE` | `10` | Số message gần nhất trong conversation history |

### 1.8 Paths

| Biến | Default | Mô tả |
|---|---|---|
| `DOCUMENTS_DIR` | `./tailieu` | Thư mục chứa .docx tài liệu |

### 1.9 WebSocket

| Biến | Default | Mô tả |
|---|---|---|
| `WS_PING_INTERVAL` | `30` (s) | Heartbeat interval từ server |
| `WS_WRITE_TIMEOUT` | `10` (s) | Timeout khi ghi WS frame |

### 1.10 Workers

| Biến | Default | Mô tả |
|---|---|---|
| `DB_BATCH_SIZE` | `50` | Số message batch insert |
| `DB_BATCH_INTERVAL_MS` | `2000` | Flush interval (ms) |
| `RETRY_MAX_COUNT` | `3` | Số lần retry trước khi vào DLQ |
| `RETRY_CLAIM_AFTER_SEC` | `60` | Stale claim timeout |

### 1.11 Voice (WebRTC fallback)

| Biến | Default | Mô tả |
|---|---|---|
| `STUN_SERVERS` | `stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302` | Comma-separated STUN URLs |

> ⚠️ STUN chỉ dùng cho **WebRTC fallback**. Production dùng Asterisk (PJSIP).

### 1.12 Authentication

| Biến | Default | Required | Mô tả |
|---|---|---|---|
| `JWT_SECRET` | _(default dev)_ | ✅ (prod) | ≥32 chars, dùng `openssl rand -base64 48` |
| `JWT_EXPIRY_HOURS` | `168` | ❌ | Session TTL (7 days default) |
| `COOKIE_DOMAIN` | _(empty)_ | ❌ | Cookie domain |
| `COOKIE_SECURE` | `false` | ❌ | `true` nếu HTTPS |
| `COOKIE_SAME_SITE` | `Strict` | ❌ | `Strict` / `Lax` / `None` |

### 1.13 System prompt

| Biến | Default | Mô tả |
|---|---|---|
| `SYSTEM_PROMPT` | (xem `internal/config/config.go` constant `defaultSystemPrompt`) | Prompt gốc cho AI |

> Override qua Admin UI: `POST /api/admin/config` sẽ ghi đè giá trị này.

---

## 2. Asterisk configuration

Config files tại `docker/asterisk/etc/asterisk/`. Được embed vào Docker image, override khi run qua volume mount.

### 2.1 Environment variables (Asterisk container)

| Biến | Default | Mô tả |
|---|---|---|
| `ASTERISK_AMI_PASS` | `dongdoami` | Password cho AMI user `dongdo` |
| `ASTERISK_MONITOR_PASS` | `dongdomonitor` | Password cho AMI user `monitor` |
| `ASTERISK_ARI_PASS` | `dongdoari` | Password cho ARI |
| `ASTERISK_GUEST_PASS` | `dongdoguest` | SIP password cho `guest` endpoint |
| `ASTERISK_AGENT_PASS_PREFIX` | `dongdoagent` | Prefix cho SIP password của agents (full = prefix + exten) |
| `ASTERISK_TRUNK_USER` | `mocktrunk` | SIP trunk username |
| `ASTERISK_TRUNK_PASS` | `mocktrunkpass` | SIP trunk password |
| `ASTERISK_VM_PASS` | `1234` | Voicemail PIN |
| `POSTGRES_HOST` | `postgres` | (để entrypoint wait) |
| `POSTGRES_PORT` | `5432` | |
| `REDIS_HOST` | `redis` | |
| `REDIS_PORT` | `6379` | |
| `NO_WAIT` | `0` | Set `1` để skip wait-for-services |

### 2.2 Templates

#### 2.2.1 Endpoint template

```ini
[template-dongdo](!)
type=endpoint
context=from-internal
allow=!all,opus,alaw,ulaw,g729
disallow=all
codec_prefs=opus,alaw,ulaw,g729
use_avpf=yes
media_use_received_transport=yes
ice_support=yes
turn_port=3478
rtp_symmetric=yes
rtp_keepalive=5
force_rport=yes
rewrite_contact=yes
moh_suggest=default
one_touch_recording=yes
record_on_feature=automixmon
trust_id_inbound=yes
send_picture=yes
```

#### 2.2.2 Auth template

```ini
[auth-dongdo](!)
type=auth
auth_type=userpass
realm=dongdo.local
```

#### 2.2.3 AOR template

```ini
[aor-dongdo](!)
type=aor
max_contacts=1
remove_existing=yes
qualify_frequency=60
support_path=yes
default_expiration=3600
media_address=dynamic
```

---

## 3. PJSIP endpoint options

### 3.1 Common endpoint options

| Option | Default | Mô tả |
|---|---|---|
| `type` | `endpoint` | Bắt buộc |
| `context` | `from-internal` | Dialplan context |
| `allow` | `!all,opus,alaw,ulaw,g729` | Allowed codecs |
| `disallow` | `all` | Disallowed codecs |
| `codec_prefs` | `opus,alaw,ulaw,g729` | Preference order |
| `use_avpf` | `yes` | RTP feedback cho WebRTC |
| `ice_support` | `yes` | ICE negotiation |
| `media_use_received_transport` | `yes` | Re-use transport address |
| `rtp_symmetric` | `yes` | Send RTP từ port đã nhận |
| `force_rport` | `yes` | Force rport cho NAT |
| `rewrite_contact` | `yes` | Rewrite Contact header |
| `one_touch_recording` | `yes` | Cho phép record bằng `*1` |
| `record_on_feature` | `automixmon` | DTMF feature code để start MixMonitor |
| `moh_suggest` | `default` | Music on hold class |

### 3.2 Codec priority

```ini
codec_prefs=opus,alaw,ulaw,g729
```

Quy ước:
1. **opus** — best for WebRTC
2. **alaw (PCMA)** — PSTN Europe, Vietnam
3. **ulaw (PCMU)** — PSTN US
4. **g729** — low bandwidth, cần license

### 3.3 NAT traversal

```ini
; For endpoints behind NAT
rtp_symmetric=yes
force_rport=yes
rewrite_contact=yes
ice_support=yes

; For servers behind NAT (public IP different from container IP)
[global]
external_media_address=<your_public_ip>
external_signaling_address=<your_public_ip>
```

### 3.4 WebRTC-specific

```ini
use_avpf=yes                ; AVPF feedback
ice_support=yes             ; ICE
turn_port=3478              ; TURN server port
media_encryption=dtls       ; SRTP/DTLS
dtls_verify=fingerprint
dtls_cert_file=/etc/asterisk/keys/asterisk.pem
dtls_priv_key_file=/etc/asterisk/keys/asterisk.key
```

---

## 4. Redis Stream configuration

Code: `internal/infra/redis/streams.go`.

```go
const (
    StreamWS   = "stream:ws"
    StreamAI   = "stream:ai"
    StreamDB   = "stream:db"
    StreamDLQ  = "stream:dlq"

    GroupWS = "ws_group"
    GroupAI = "ai_group"
    GroupDB = "db_group"

    StreamMaxLen = 5000  // approximate trim
)
```

### 4.1 Stream names

| Stream | Purpose | Publisher | Consumer group | Worker |
|---|---|---|---|---|
| `stream:ws` | Realtime WS events | Backend handlers | `ws_group` | `ws_worker_*` |
| `stream:ai` | RAG jobs | `ChatUseCase.SendGuestMessage` | `ai_group` | `ai_worker_*` |
| `stream:db` | DB writes | `ChatUseCase.SendCSReply` | `db_group` | `db_worker_*` |
| `stream:dlq` | Dead-letter queue | `retry_worker` | — | `retry_worker_*` |

### 4.2 Retention

Mỗi stream trim ở `MAXLEN ~ 5000` (approximate). Override qua env (TODO):

```bash
REDIS_STREAM_MAXLEN=10000
```

### 4.3 Monitor streams

```bash
docker compose exec redis redis-cli

> XLEN stream:ws
> XLEN stream:ai
> XLEN stream:db
> XLEN stream:dlq

> XRANGE stream:dlq - + COUNT 5
```

---

## 5. Worker tuning

### 5.1 DBWorker (batch insert)

| Env | Default | Recommendation |
|---|---|---|
| `DB_BATCH_SIZE` | `50` | 50-200 tuỳ traffic |
| `DB_BATCH_INTERVAL_MS` | `2000` | 500-2000ms |

Cân bằng latency vs throughput:
- `BATCH_SIZE=200, INTERVAL=1000ms` → throughput cao, latency 1s
- `BATCH_SIZE=10, INTERVAL=200ms` → latency thấp, throughput thấp hơn

### 5.2 AIWorker (RAG)

Single-threaded per consumer. Scale bằng cách chạy nhiều instances với cùng `ai_group` (Redis XREADGROUP sẽ phân phối messages).

### 5.3 WSWorker (realtime)

Broadcast qua in-memory Hub. Multi-instance cần Redis pub-sub adapter.

### 5.4 RetryWorker

| Env | Default | Mô tả |
|---|---|---|
| `RETRY_MAX_COUNT` | `3` | Max retries trước khi vào DLQ |
| `RETRY_CLAIM_AFTER_SEC` | `60` | Stale claim timeout |

Logic:
- Worker đọc message, xử lý
- Nếu fail → `XCLAIM` với `RETRY_CLAIM_AFTER_SEC`
- Sau `RETRY_MAX_COUNT` lần → push vào `stream:dlq`

---

## 6. Database configuration

### 6.1 Connection pool

`internal/repository/postgres/db.go`:

```go
config.MaxConns = 25
config.MinConns = 5
config.MaxConnLifetime = 1 * time.Hour
config.MaxConnIdleTime = 30 * time.Minute
config.HealthCheckPeriod = 1 * time.Minute
```

### 6.2 Tuning cho high traffic

```bash
# Postgres
max_connections = 200
shared_buffers = 4GB             # ~25% RAM
effective_cache_size = 12GB      # ~75% RAM
work_mem = 64MB
maintenance_work_mem = 1GB
wal_buffers = 16MB

# pgxpool override (qua env, chưa support — TODO):
# DB_MAX_CONNS=50
# DB_MIN_CONNS=10
```

### 6.3 Migrations

Migrations ở `db/migrations/` (source of truth) và `internal/repository/postgres/migrations/` (embedded copy).

**Sync:**
```bash
make sync-init
```

---

## 7. Frontend (Next.js) configuration

`web/` Next.js 14 (App Router). Env tại `web/.env.local`.

### 7.1 Frontend env vars

| Biến | Default | Mô tả |
|---|---|---|
| `NEXT_PUBLIC_API_BASE` | `http://localhost:8080` | Backend URL |
| `NEXT_PUBLIC_WS_URL` | `ws://localhost:8080/ws` | WebSocket URL |
| `NEXT_PUBLIC_ADMIN_PATH` | `/admin` | Admin path prefix |

### 7.2 Build-time config

`web/next.config.js`:

```js
module.exports = {
  reactStrictMode: true,
  output: 'standalone',  // cho Docker
  experimental: {
    serverActions: true,
  },
};
```

### 7.3 Frontend proxy (dev)

`web/src/lib/api/client.ts`:

```ts
const API_BASE = process.env.NEXT_PUBLIC_API_BASE || 'http://localhost:8080';
```

Trong dev, Next.js dev server chạy `:3000`, backend `:8080`. Browser fetch trực tiếp tới backend.

---

## 8. Cross-references

- [DEPLOYMENT.md](./DEPLOYMENT.md) — production deployment
- [DEVELOPMENT.md](./DEVELOPMENT.md) — local dev
- [TELEPHONY.md](./TELEPHONY.md) — Asterisk details
- [API.md](./API.md) — REST/WS reference

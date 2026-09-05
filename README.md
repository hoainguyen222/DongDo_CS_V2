# 📞 Đông Đô CS Core V2 — Hệ thống CSKH Doanh nghiệp với Asterisk Telephony

> **Customer Support & Knowledge Management Platform** cho **Đông Đô Partners** — phiên bản 2.0
> Tái cấu trúc từ Python/JS sang **Go (Gin + SQLC + Goose)** + **Next.js (TypeScript)** theo **Clean Architecture**,
> tích hợp **Asterisk 20 (PJSIP + AMI)** cho thoại doanh nghiệp, **Redis Streams Event Bus**, **Qdrant Vector DB**.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-14-000000?logo=next.js)](https://nextjs.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql)](https://postgresql.org)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis)](https://redis.io)
[![Asterisk](https://img.shields.io/badge/Asterisk-20-LTE-FF6B00)](https://asterisk.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

---

## 📖 Mục lục

- [Tổng quan](#-tổng-quan)
- [Kiến trúc](#-kiến-trúc)
- [Quick Start](#-quick-start)
- [Tech Stack](#-tech-stack)
- [Tính năng chính](#-tính-năng-chính)
- [Cấu trúc thư mục](#-cấu-trúc-thư-mục)
- [Tài khoản mặc định](#-tài-khoản-mặc-định)
- [Tài liệu chi tiết](#-tài-liệu-chi-tiết)
- [Đóng góp](#-đóng-góp)
- [License](#-license)

---

## 🎯 Tổng quan

**Đông Đô CS Core V2** là nền tảng chăm sóc khách hàng (CSKH) đa kênh, hỗ trợ:

- 💬 **Chat đa vai** — AI tự phục vụ → chuyển tiếp cho CSKH khi cần
- 📞 **Cuộc gọi thoại** — qua **Asterisk 20 PBX** với ghi âm tự động, queue, IVR
- 🧠 **Knowledge Base + RAG** — tra cứu tài liệu qua Qdrant + Claude LLM (tiếng Việt)
- 📊 **Dashboard CSKH** — phân tích hiệu suất, CSAT, audit log
- 🔁 **Continuous Learning** — CS duyệt tri thức mới → nạp vào RAG

> **Điểm khác biệt v2.0:** Tích hợp **Asterisk 20 (PJSIP/AMI)** thay thế WebRTC P2P — hỗ trợ PSTN, SIP hardphone, queue, recording chuyên nghiệp.

---

## 🏗 Kiến trúc

```text
┌──────────────────┐    HTTP/WS    ┌──────────────────────┐
│  Guest Browser   │◄────────────►│   Next.js Frontend   │
└──────────────────┘              │   (web/src/app)      │
                                  └──────────┬───────────┘
                                             │ REST + WS
                                             ▼
┌──────────────────┐              ┌──────────────────────┐
│  Admin Browser   │◄────────────►│   Go Backend (Gin)   │
└──────────────────┘              │   cmd/server         │
                                  └─────┬──────────┬─────┘
                                        │          │
                            ┌───────────▼─┐    ┌───▼────────────┐
                            │ PostgreSQL  │    │ Redis Streams │
                            │ (source of  │    │ (event bus +   │
                            │  truth)     │    │  state mgmt)   │
                            └─────────────┘    └────────┬──────┘
                                                        │
                                  ┌─────────────────────▼────────────┐
                                  │     Asterisk 20 (PJSIP + AMI)    │
                                  │   docker/asterisk (chan_pjsip)   │
                                  └─────┬───────────────┬─────────────┘
                                        │               │
                          ┌─────────────▼─┐    ┌────────▼────────┐
                          │ Agent SIP     │    │ Recording (.wav)│
                          │ phones 1001+  │    │ MixMonitor      │
                          └───────────────┘    └─────────────────┘
                                        ▲
                                        │ WebSocket signaling (fallback)
                            ┌───────────┴──────────┐
                            │  WebRTC (in-browser) │
                            │  for softphone dev   │
                            └──────────────────────┘
```

**Workers chạy nền trong Backend:**

| Worker | Trách nhiệm |
|---|---|
| `ws_worker` | Phát sự kiện từ `stream:ws` → WebSocket clients |
| `ai_worker` | Đọc `stream:ai` → gọi RAG → publish reply |
| `db_worker` | Batch insert messages vào Postgres từ `stream:db` |
| `retry_worker` | XAUTOCLAIM stale messages → `stream:dlq` khi quá retry |

---

## 🚀 Quick Start

### Yêu cầu

- Docker 24+ và Docker Compose v2
- 4 GB RAM, 2 vCPU trống
- Cổng mở: **8080** (backend), **3000** (web dev), **5060/udp** (SIP), **5038** (AMI), **10000-20000/udp** (RTP)

### Khởi chạy production stack

```bash
# 1. Clone và cấu hình
git clone <repo> DongDo_CS_V2
cd DongDo_CS_V2
cp .env.example .env

# 2. Cấu hình secrets trong .env
#    ANTHROPIC_API_KEY, JWT_SECRET, ASTERISK_AMI_PASS, ...

# 3. Khởi động toàn bộ stack (Postgres, Redis, Qdrant, Backend, Asterisk)
make up

# 4. Nạp tài liệu vào Qdrant (Knowledge Base)
docker compose exec server /app/ingest

# 5. Mở Admin UI
open http://localhost:8080/admin/login
# Đăng nhập: admin / DongDo@2026
```

### Khởi chạy nhanh (chỉ backend + DB, không cần Asterisk cho dev chat)

```bash
docker compose up -d postgres redis qdrant server
docker compose exec server /app/ingest
```

Chi tiết xem [docs/DEVELOPMENT.md](./docs/DEVELOPMENT.md).

---

## 🛠 Tech Stack

| Layer | Công nghệ | Chi tiết |
|---|---|---|
| **Backend** | Go 1.21+ / Gin | Clean Architecture, type-safe SQL, embedded migrations |
| **Migrations** | Goose v3 | Embedded trong binary qua `//go:embed` |
| **SQL Generation** | SQLC | Zero allocations, type-safe queries từ `db/queries/*.sql` |
| **Database** | PostgreSQL 16 | pgxpool, batch upsert, idempotent insert qua `client_msg_id` |
| **Cache / Event Bus** | Redis 7 | Streams + Consumer Groups, XAUTOCLAIM, DLQ |
| **Vector DB** | Qdrant | gRPC, cosine similarity, continuous learning |
| **LLM** | Claude (Haiku 4.5 / Sonnet / Opus) | RAG tiếng Việt, no hallucination |
| **Telephony** | Asterisk 20 LTS | PJSIP, AMI, MixMonitor recording, queue |
| **SIP Container** | `asterisk:20-current-bookworm` | Multi-stage build, custom config + entrypoint |
| **Frontend** | Next.js 14 / TS / Tailwind | Dark glassmorphism, micro-animations |
| **Realtime** | WebSocket Hub | Auto-reconnect, heartbeat, typing indicator |
| **Voice (Browser)** | WebRTC (fallback) | Dùng khi không qua Asterisk (dev mode) |
| **STUN** | Google free STUN | Cho WebRTC ICE negotiation |

---

## ✨ Tính năng chính

### 💬 Chat & AI
- ✅ Guest không cần đăng nhập — chỉ cần nhập tên (+ SĐT tùy chọn)
- ✅ AI auto-reply với RAG, fallback về CSKH khi không có dữ liệu
- ✅ Typing indicator real-time (AI ↔ Guest ↔ CSKH)
- ✅ CSKH tiếp nhận case qua REST + WS update
- ✅ Multi-modal recording: voice call tự động → transcript → đưa vào Continuous Learning queue

### 📞 Telephony (Asterisk 20)
- ✅ **PJSIP endpoints** cho agent (1001-1010) và guest browser SIP
- ✅ **AMI** điều khiển originate, transfer, hold, mute, hangup
- ✅ **MixMonitor** ghi âm cuộc gọi (WAV, 16kHz)
- ✅ **Queue** với ringall strategy, holdtime announce, supervisor escalation
- ✅ **CDR** lưu CSV hoặc push về Postgres qua ODBC
- ✅ **One-touch recording** (`*1`) cho agent

### 🧠 Knowledge Base & RAG
- ✅ Upload `.docx` qua Admin UI → tự động chunk + embed vào Qdrant
- ✅ RAG với 5 retrieval, 10-message memory window
- ✅ Continuous Learning queue: CSKH duyệt Q&A mới → nạp vào Qdrant
- ✅ Auto-learning toggle (bật/tắt trong Settings)

### 📊 Dashboard & Reports
- ✅ Real-time Inbox (live cases, queue metrics)
- ✅ 7 sub-reports: Overview, AI Performance, Staff Performance, CX, CSAT, Operational, Issue Analysis
- ✅ AI Learning stats
- ✅ Audit logs cho actions quan trọng (Owner-only)

### 🔒 Security & RBAC
- ✅ PBKDF2 password hashing (`pkg/security`)
- ✅ Bearer token auth + X-Auth-Token header
- ✅ Role-based: `Owner > Admin > Leader > CSKH`
- ✅ Per-feature permission matrix (`config.partner_config.permissions`)
- ✅ Session expiry (default 168h)

### 📦 DevOps
- ✅ Docker Compose multi-service
- ✅ Embedded migrations (binary self-contained)
- ✅ Graceful shutdown với 15s timeout
- ✅ Health check endpoint `/health`
- ✅ Structured logging (zerolog)

---

## 📂 Cấu trúc thư mục

```text
DongDo_CS_V2/
├── cmd/
│   ├── server/main.go          # Go backend entry point
│   └── ingest/main.go          # CLI nạp tailieu vào Qdrant
├── internal/
│   ├── config/                 # Env loader
│   ├── domain/                 # Pure Go entities & interfaces
│   ├── usecase/                # Business logic (Auth, Chat, Case, RAG, Voice, Learning)
│   ├── repository/
│   │   ├── postgres/           # Repo adapter (wraps sqlc queries)
│   │   │   └── migrations/     # Embedded goose migrations
│   │   └── sqlcdb/             # ⚠️ GENERATED — KHÔNG EDIT
│   ├── delivery/
│   │   ├── http/               # REST handlers + router + middleware
│   │   └── ws/                 # WebSocket Hub
│   ├── worker/                 # Background workers (ws, ai, db, retry)
│   └── infra/                  # Redis, Qdrant, Claude, Embedding
├── pkg/
│   ├── graceful/               # Graceful shutdown
│   └── security/               # PBKDF2
├── db/
│   ├── migrations/             # Goose migrations (source of truth)
│   ├── init/                   # First-boot SQL (synced via `make sync-init`)
│   └── queries/                # SQLC input
│       ├── auth/ chat/ learning/ voice/ settings/ analytics/ partner/
├── docker/
│   └── asterisk/               # 📞 Asterisk 20 container + config
│       ├── Dockerfile          # Multi-stage build
│       ├── entrypoint.sh       # Wait-for services + template secrets
│       └── etc/asterisk/       # pjsip.conf, extensions.conf, queues.conf, manager.conf, ...
├── web/                        # Next.js frontend
│   └── src/
│       ├── app/                # Routes (guest/, admin/, api/)
│       ├── components/         # UI components
│       ├── features/           # Feature modules
│       └── lib/                # api/, ws, webrtc, stores
├── docs/                       # 📚 Documentation (this directory)
│   ├── ARCHITECTURE.md
│   ├── TELEPHONY.md
│   ├── DEPLOYMENT.md
│   ├── DEVELOPMENT.md
│   ├── API.md
│   ├── CONFIGURATION.md
│   ├── TROUBLESHOOTING.md
│   └── diagrams/
├── scripts/                    # smoke-test.sh, maintenance scripts
├── sqlc.yaml
├── docker-compose.yml
├── Dockerfile                  # Backend Go container
├── Makefile
├── .env.example
├── CHANGELOG.md
├── CONTRIBUTING.md
├── LICENSE
└── README.md
```

---

## 🔑 Tài khoản mặc định

> ⚠️ **CHỈ DÙNG CHO DEV.** Đổi password ngay khi deploy production.

| Username | Password | Role | Quyền |
|---|---|---|---|
| `admin` | `DongDo@2026` | `owner` | Toàn quyền (Owner) |
| `cskh01` - `cskh05` | `DongDo@123` | `cskh` | Chat, reply case, voice |
| **Guest** | _(không cần)_ | — | Chỉ cần tên + SĐT (tùy chọn) |

---

## 📚 Tài liệu chi tiết

| File | Mô tả |
|---|---|
| [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) | Kiến trúc chi tiết, layers, data flow, sequence diagrams |
| [docs/TELEPHONY.md](./docs/TELEPHONY.md) | Asterisk integration, AMI vs ARI, dialplan, troubleshooting |
| [docs/DEPLOYMENT.md](./docs/DEPLOYMENT.md) | Production deployment, SSL/TLS, firewall, monitoring |
| [docs/DEVELOPMENT.md](./docs/DEVELOPMENT.md) | Local dev workflow, hot reload, testing |
| [docs/API.md](./docs/API.md) | REST API + WebSocket reference |
| [docs/CONFIGURATION.md](./docs/CONFIGURATION.md) | Toàn bộ env vars + Asterisk config |
| [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) | Common issues + solutions |
| [CHANGELOG.md](./CHANGELOG.md) | Version history, migration notes |
| [CONTRIBUTING.md](./CONTRIBUTING.md) | Code style, commit convention, PR process |

### Sơ đồ Mermaid
- [docs/diagrams/architecture.mmd](./docs/diagrams/architecture.mmd) — System architecture
- [docs/diagrams/sequence-call.mmd](./docs/diagrams/sequence-call.mmd) — Call flow sequence
- [docs/diagrams/deployment.mmd](./docs/diagrams/deployment.mmd) — Deployment topology

---

## 🧪 Smoke test

Kiểm tra end-to-end sau khi `make up`:

```bash
./scripts/smoke-test.sh
```

Script này sẽ:
1. Ping health endpoints (server, postgres, redis, asterisk)
2. Đăng ký guest
3. Gửi message AI → kiểm tra RAG response
4. Initiate voice call qua AMI (Asterisk)
5. Verify DB records + Redis stream events

---

## 🔄 Workflow Dev: Thêm / sửa query

```bash
# 1. Viết SQL trong db/queries/<domain>/*.sql
# 2. Generate code
make sqlc-gen

# 3. Viết / cập nhật repository adapter
# 4. Verify build
go build ./...
```

Chi tiết xem [docs/DEVELOPMENT.md §6](./docs/DEVELOPMENT.md#6-workflow-sqlc).

---

## 🤝 Đóng góp

Xem [CONTRIBUTING.md](./CONTRIBUTING.md) — Conventional Commits, code style, PR template.

---

## 📜 License

[MIT](./LICENSE) — © 2025 Đông Đô Partners.

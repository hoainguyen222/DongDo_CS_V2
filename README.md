# 🚀 Đông Đô CS Core V2 — Enterprise Architecture

Hệ thống Chăm Sóc Khách Hàng Thông Minh & Quản Trị Tri Thức Đông Đô Partners phiên bản 2.0.

Được tái cấu trúc hoàn toàn từ Python/JS sang **Golang (Gin + SQLC) + Next.js (TypeScript)** theo chuẩn **Clean Architecture**, tích hợp **Redis Streams Event Bus**, **Qdrant Vector DB** và **WebRTC Voice Calling**.

---

## 🛠️ Công Nghệ Sử Dụng

| Thành phần | Công nghệ | Chi tiết |
|---|---|---|
| **Backend API** | Golang 1.22+ / Gin / pgx / SQLC | Clean Architecture, 0 Reflection, Zero Allocations |
| **Frontend** | Next.js 14 / TypeScript / TailwindCSS | Dark Glassmorphism, Micro-animations |
| **Realtime** | WebSocket Hub + Goroutines | Auto-reconnect, Typing Indicator, Heartbeat Ping/Pong |
| **Event Bus** | Redis Streams | Consumer Groups (`ws_group`, `ai_group`, `db_group`), XACK, XAUTOCLAIM, DLQ |
| **Redis State** | Redis Key-Value & Sets | Distributed Lock, Unread Counter, Typing State (TTL 3s) |
| **Vector DB** | Qdrant (gRPC) | Vector search cosine, metadata filtering, continuous learning |
| **LLM Engine** | Claude Haiku 4.5 / Sonnet / Opus | RAG Pipeline tiếng Việt, Zero hallucination |
| **Voice Calling** | WebRTC (P2P Audio) | Gọi điện thoại 2 chiều qua trình duyệt, STUN server, ghi âm |
| **Database** | PostgreSQL 16 | Source of Truth, Batch Upsert (pgx.Batch), Idempotent |

---

## 📂 Cấu Trúc Thư Mục

```text
DongDo_CS_V2/
├── cmd/
│   ├── server/main.go          # Entry point Golang server
│   └── ingest/main.go          # CLI tool nạp tài liệu vào Qdrant
├── internal/
│   ├── config/config.go        # Configuration loader
│   ├── domain/                 # Entity & Interface (Pure Go)
│   ├── usecase/                # Business Logic (Auth, Chat, Case, RAG, Voice, Learning)
│   ├── repository/postgres/    # PostgreSQL repo implementation (pgx/v5)
│   ├── delivery/
│   │   ├── http/               # REST API Handlers & Middleware (Gin)
│   │   └── ws/                 # WebSocket Hub, Client & Signaling
│   ├── worker/                 # Background Workers (WS, AI, DB Batch, Retry/DLQ)
│   └── infra/                  # Redis, Qdrant, Claude API, Embedding
├── pkg/
│   ├── graceful/               # Graceful shutdown manager
│   └── security/               # PBKDF2 Password hashing
├── db/
│   ├── migrations/001_init.sql # PostgreSQL schema & seed data
│   └── queries/*.sql           # SQLC queries
├── web/                        # Next.js Frontend
│   ├── src/app/page.tsx        # Client Chat (Pre-chat form + Chat + Call)
│   ├── src/app/admin/page.tsx  # CS Studio Dashboard (5 tabs)
│   └── src/lib/                # API client, WS client, WebRTC manager
├── tailieu/                    # File tài liệu Word nguồn
├── docker-compose.yml          # Postgres + Redis + Qdrant + Server
├── Dockerfile                  # Multi-stage container build
└── README.md
```

---

## 🚀 Hướng Dẫn Khởi Chạy Nhanh

### Cách 1: Chạy bằng Docker Compose (Khuyên dùng)

```bash
# 1. Điền API Key Claude vào .env
cp .env.example .env
# Chỉnh sửa file .env và điền ANTHROPIC_API_KEY

# 2. Khởi chạy toàn bộ hạ tầng
docker-compose up -d --build

# 3. Nạp tài liệu nguồn vào Qdrant
docker-compose exec server /app/ingest
```

### Cách 2: Chạy trực tiếp trên máy chủ / Local

```bash
# 1. Chạy Backend Golang
go run cmd/server/main.go

# 2. Nạp tài liệu vào Qdrant
go run cmd/ingest/main.go

# 3. Chạy Frontend Next.js
cd web
npm install
npm run dev
```

---

## 🔑 Tài Khoản Mặc Định

| Tài khoản | Mật khẩu | Quyền hạn |
|---|---|---|
| `admin` | `DongDo@2026` | Quản trị viên hệ thống |
| `cskh01` - `cskh05` | `DongDo@123` | Chuyên viên CSKH |
| **Khách hàng** | *Không cần mật khẩu* | Chỉ cần nhập Tên + SĐT (tùy chọn) |

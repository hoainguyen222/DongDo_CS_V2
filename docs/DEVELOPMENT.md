# 💻 Development — Hướng dẫn Dev

> **Phiên bản:** v2.0
> **Đối tượng:** Backend devs, Frontend devs
> **Cập nhật lần cuối:** Sep 2026

## Mục lục

1. [Prerequisites](#1-prerequisites)
2. [Setup lần đầu](#2-setup-lần-đầu)
3. [Chạy với Docker Compose](#3-chạy-với-docker-compose)
4. [Chạy KHÔNG Docker (local Asterisk)](#4-chạy-không-docker-local-asterisk)
5. [Hot reload](#5-hot-reload)
6. [Workflow SQLC](#6-workflow-sqlc)
7. [Workflow Goose migrations](#7-workflow-goose-migrations)
8. [Testing](#8-testing)
9. [Debugging](#9-debugging)
10. [Code style & lint](#10-code-style--lint)
11. [Cross-references](#11-cross-references)

---

## 1. Prerequisites

### 1.1 Bắt buộc

| Tool | Version | Check |
|---|---|---|
| **Go** | 1.21+ (1.26 khuyến nghị) | `go version` |
| **Node.js** | 20 LTS+ | `node --version` |
| **pnpm** | 8+ | `pnpm --version` |
| **Docker** | 24+ | `docker --version` |
| **Docker Compose** | v2.20+ | `docker compose version` |
| **Git** | 2.30+ | `git --version` |
| **Make** | any | `make --version` |

### 1.2 Tuỳ chọn (cho chạy local Asterisk)

```bash
# Ubuntu/Debian
sudo apt install -y asterisk asterisk-dev

# macOS (qua brew)
brew install asterisk

# Asterisk headers cho Go libs (libami, libpri, ...)
sudo apt install -y asterisk-dev libpri-dev
```

### 1.3 Tools CLI

```bash
# sqlc (Go SQL → typed Go)
curl -sSL https://github.com/sqlc-dev/sqlc/releases/download/v1.27.0/sqlc_1.27.0_darwin_arm64.zip -o /tmp/sqlc.zip
unzip -o /tmp/sqlc.zip sqlc -d tools/bin/
chmod +x tools/bin/sqlc

# goose (migrations)
curl -sSL https://github.com/pressly/goose/releases/download/v3.22.1/goose_darwin_arm64 -o tools/bin/goose
chmod +x tools/bin/goose

# Verify
./tools/bin/sqlc version   # → v1.27.0
./tools/bin/goose --help
```

---

## 2. Setup lần đầu

### 2.1 Clone

```bash
git clone <repo-url> DongDo_CS_V2
cd DongDo_CS_V2
```

### 2.2 Environment

```bash
cp .env.example .env
nano .env
```

Điền tối thiểu:

```bash
ANTHROPIC_API_KEY=sk-ant-api03-xxxxx  # bắt buộc cho RAG
JWT_SECRET=<openssl rand -base64 48>   # bắt buộc
POSTGRES_PASSWORD=postgrespassword    # mặc định dev
```

### 2.3 Install dependencies

```bash
# Go modules
go mod download

# Frontend
cd web && pnpm install && cd ..
```

---

## 3. Chạy với Docker Compose

### 3.1 Khởi động toàn stack

```bash
make up
```

Services:

| Service | Port | URL |
|---|---|---|
| Postgres | 5433 → 5432 | `localhost:5433` |
| Redis | 6379 | `localhost:6379` |
| Qdrant | 6333 / 6334 | `localhost:6333` |
| Backend (Go) | 8080 | `http://localhost:8080` |
| **Asterisk** | 5060, 5061, 5038 | `localhost:5060` (SIP), `localhost:5038` (AMI) |

### 3.2 Logs

```bash
# All
make logs

# Just backend
docker compose logs -f server

# Just asterisk
docker compose logs -f asterisk
```

### 3.3 Restart một service

```bash
docker compose restart server
docker compose restart asterisk
```

### 3.4 Wipe và khởi động lại

```bash
make down
docker volume rm dongdo_cs_v2_pgdata dongdo_cs_v2_redisdata dongdo_cs_v2_qdrantdata
make up
```

---

## 4. Chạy KHÔNG Docker (local Asterisk)

> Khi cần debug Asterisk sâu hoặc không muốn container overhead.

### 4.1 Chạy Postgres + Redis + Qdrant qua Docker

```bash
docker compose up -d postgres redis qdrant
```

### 4.2 Cài Asterisk local

```bash
# macOS
brew install asterisk

# Ubuntu
sudo apt install -y asterisk
```

### 4.3 Mount config local

```bash
# Copy config
sudo cp -r docker/asterisk/etc/asterisk /etc/asterisk

# Restart local asterisk
sudo systemctl restart asterisk
# hoặc
sudo asterisk -f
```

### 4.4 Build & run backend

```bash
make sqlc-gen
make build
./server_new
```

### 4.5 Truy cập Asterisk CLI

```bash
sudo asterisk -rvvv
# hoặc từ container
docker compose exec asterisk asterisk -rvvv
```

Commands hữu ích:

```text
> pjsip show endpoints
> pjsip show registrations
> core show channels
> queue show dongdo-queue
> manager show connected
> cdr show
```

---

## 5. Hot reload

### 5.1 Backend (Go) với `air`

Cài đặt:

```bash
go install github.com/cosmtrek/air@latest
```

Tạo `.air.toml`:

```toml
[build]
cmd = "go build -o ./tmp/server ./cmd/server"
bin = "./tmp/server"
include_ext = ["go"]
exclude_dir = ["tmp", "web", "node_modules"]
delay = 1000

[log]
time = true

[color]
main = "magenta"
worker = "gray"
```

Chạy:

```bash
air
```

### 5.2 Frontend với `next dev`

```bash
cd web
pnpm dev
```

Hot reload tự động cho mọi file `.tsx`, `.ts`, `.scss`.

---

## 6. Workflow SQLC

### 6.1 Thêm/sửa query

**Bước 1:** Viết SQL trong `db/queries/<domain>/<file>.sql`:

```sql
-- db/queries/auth/users.sql
-- name: GetUserByEmail :one
SELECT id, username, password_hash, salt, full_name, role, is_active, created_at
FROM users
WHERE LOWER(username) = LOWER($1);
```

**Bước 2:** Generate:

```bash
make sqlc-gen
# hoặc
./tools/bin/sqlc generate
```

Output: `internal/repository/sqlcdb/auth/users.sql.go`

**Bước 3:** Viết repo adapter:

```go
// internal/repository/postgres/user_repo.go
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
    row, err := r.db.Auth.GetUserByEmail(ctx, email)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, nil
        }
        return nil, err
    }
    return &domain.User{
        ID:       row.ID,
        Username: row.Username,
        // ...
    }, nil
}
```

**Bước 4:** Verify build:

```bash
go build ./...
```

### 6.2 ⚠️ KHÔNG edit `internal/repository/sqlcdb/`

Đây là generated code — mọi thay đổi sẽ bị mất khi chạy `make sqlc-gen` lần sau.

---

## 7. Workflow Goose migrations

### 7.1 Tạo migration mới

```bash
make migrate-create name=add_customer_tags
```

Output: `db/migrations/00004_add_customer_tags.sql`

### 7.2 Viết Up/Down

```sql
-- db/migrations/00004_add_customer_tags.sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE guests ADD COLUMN tags TEXT[] DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE guests DROP COLUMN tags;
-- +goose StatementEnd
```

### 7.3 Copy vào embedded folder

```bash
cp db/migrations/00004_add_customer_tags.sql internal/repository/postgres/migrations/
```

### 7.4 Run

Migrations tự chạy khi server start. Manual:

```bash
make migrate-up
make migrate-status
make migrate-down
```

### 7.5 Sync init (cho Docker first-boot)

```bash
make sync-init
```

---

## 8. Testing

### 8.1 Unit tests

```bash
go test ./...
```

Run specific package:

```bash
go test ./internal/usecase/... -v
go test ./internal/repository/... -v
```

### 8.2 Integration tests

```bash
make test-integration
```

(TODO: setup testcontainers cho Postgres/Redis/Qdrant)

### 8.3 End-to-end smoke test

```bash
./scripts/smoke-test.sh
```

Steps:

1. Health check `/health`
2. Register guest qua `POST /guest/register`
3. Gửi message qua `POST /chat`
4. Verify AI reply qua WS
5. Initiate voice call qua `POST /api/voice/initiate`
6. Verify database record
7. Cleanup

### 8.4 Test coverage

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 8.5 Frontend tests

```bash
cd web
pnpm test        # unit
pnpm test:e2e    # playwright/cypress (nếu setup)
pnpm type-check
```

---

## 9. Debugging

### 9.1 Backend với Delve

```bash
# Install
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/server

# Set breakpoint
(dlv) break main.main
(dlv) continue
```

Attach vào running process:

```bash
dlv attach <pid>
```

VSCode config `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch server",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/server",
      "env": {
        "DATABASE_URL": "postgres://postgres:postgrespassword@localhost:5433/dongdo_cs?sslmode=disable",
        "REDIS_URL": "redis://localhost:6379",
        "QDRANT_HOST": "localhost"
      }
    }
  ]
}
```

### 9.2 Frontend với Chrome DevTools

```bash
cd web
pnpm dev
```

Mở `http://localhost:3000`, F12 → Sources tab, set breakpoints trong TS code.

### 9.3 Asterisk debug

```bash
sudo asterisk -rvvv
# hoặc với maximum verbosity
sudo asterisk -rvvvvvvvvvvv
```

Useful commands:

```text
core set verbose 9          # bật verbose level 9
pjsip set logger on         # bật SIP packet log
sip set debug on            # alias
core set debug 9
```

Log file: `/var/log/asterisk/messages` (hoặc trong container: `docker compose logs asterisk`).

### 9.4 Redis debug

```bash
docker compose exec redis redis-cli
> MONITOR                    # tail mọi command
> XINFO STREAM stream:ws
> XRANGE stream:ws - + COUNT 10
> XLEN stream:ws
> XLEN stream:ai
> XLEN stream:db
> XLEN stream:dlq
```

### 9.5 Postgres debug

```bash
docker compose exec postgres psql -U postgres -d dongdo_cs

-- Active queries
SELECT pid, query, state FROM pg_stat_activity WHERE state != 'idle';

-- Slow queries
SELECT query, mean_exec_time, calls FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;

-- Index usage
SELECT schemaname, tablename, indexname, idx_scan FROM pg_stat_user_indexes;
```

---

## 10. Code style & lint

### 10.1 Go

```bash
# Format
gofmt -w .

# Lint
golangci-lint run

# Vet
go vet ./...
```

`.golangci.yml`:

```yaml
linters:
  enable:
    - gofmt
    - govet
    - errcheck
    - staticcheck
    - unused
    - ineffassign
```

### 10.2 TypeScript / React

```bash
cd web
pnpm lint
pnpm type-check
pnpm format
```

ESLint + Prettier configured trong `web/`.

### 10.3 Editor setup

#### VSCode

`.vscode/settings.json`:

```json
{
  "go.formatTool": "gofmt",
  "go.lintTool": "golangci-lint",
  "editor.formatOnSave": true,
  "[go]": { "editor.defaultFormatter": "golang.go" },
  "[typescript]": { "editor.defaultFormatter": "esbenp.prettier-vscode" },
  "[typescriptreact]": { "editor.defaultFormatter": "esbenp.prettier-vscode" },
  "tailwindCSS.experimental.classRegex": [
    ["csl\\(([^)]*)\\)", "[\"'\`]([^\"`' ]+).*?[\"'`]"]
  ]
}
```

#### GoLand / JetBrains

- Enable "Go Modules" 
- Set GOROOT to Go 1.21+
- Run configurations: `main.go` for server, `main.go` for ingest

---

## 11. Cross-references

- [ARCHITECTURE.md](./ARCHITECTURE.md) — codebase structure
- [CONFIGURATION.md](./CONFIGURATION.md) — env vars
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) — debugging common issues
- [CONTRIBUTING.md](../CONTRIBUTING.md) — code review, commits

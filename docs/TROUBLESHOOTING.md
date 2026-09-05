# 🔍 Troubleshooting — Các vấn đề thường gặp

> **Phiên bản:** v2.0
> **Đối tượng:** Devs, DevOps
> **Cập nhật lần cuối:** Sep 2026

## Mục lục

1. [Build issues](#1-build-issues)
2. [Database](#2-database)
3. [Redis](#3-redis)
4. [Qdrant / RAG](#4-qdrant--rag)
5. [WebSocket](#5-websocket)
6. [Asterisk / Telephony](#6-asterisk--telephony)
7. [Frontend (Next.js)](#7-frontend-nextjs)
8. [Performance](#8-performance)
9. [Authentication](#9-authentication)
10. [Recording / Files](#10-recording--files)
11. [Cross-references](#11-cross-references)

---

## 1. Build issues

### 1.1 `go build`: Go version mismatch

**Triệu chứng:**
```
go.mod requires go 1.21 (running go 1.20)
```

**Fix:**
```bash
# Install Go 1.21+
brew install go@1.21    # macOS
# hoặc download từ https://go.dev/dl/

go version   # verify
```

### 1.2 `pnpm install` fails

**Triệu chứng:**
```
ERR_PNPM_PEER_DEP_ISSUES
```

**Fix:**
```bash
cd web
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

### 1.3 Docker build fails — `apt-get` cannot reach

**Fix:** Check DNS / network:
```bash
docker network inspect dongdo_default
```

Hoặc dùng DNS:
```yaml
# docker-compose.yml
services:
  server:
    dns:
      - 8.8.8.8
      - 1.1.1.1
```

### 1.4 `docker compose up` — port already in use

**Triệu chứng:**
```
Error: bind: address already in use 0.0.0.0:8080
```

**Fix:**
```bash
# Tìm process
lsof -i :8080
# Kill
kill -9 <pid>

# Hoặc đổi port
# docker-compose.yml → ports: "8081:8080"
```

### 1.5 SQLC generate fails

**Triệu chứng:**
```
sqlc: error parsing sqlc.json
```

**Fix:**
```bash
./tools/bin/sqlc version
# Update nếu < 1.27.0
```

---

## 2. Database

### 2.1 Postgres connection fails

**Triệu chứng:**
```
Failed to ping PostgreSQL: connection refused
```

**Check list:**

1. **Postgres container chạy?**
   ```bash
   docker compose ps postgres
   docker compose logs postgres
   ```

2. **Port mapping đúng?**
   ```bash
   # .env
   DATABASE_URL=postgres://postgres:postgrespassword@localhost:5433/dongdo_cs?sslmode=disable
   ```
   Postgres expose 5433 (host) → 5432 (container).

3. **Wrong credentials:**
   ```bash
   docker compose exec postgres psql -U postgres -c "SELECT 1"
   ```

### 2.2 Migration fails on startup

**Triệu chứng:**
```
failed to run migrations: ERROR: relation "x" already exists
```

**Fix (chỉ dev):**
```bash
# 1. Stop server
docker compose stop server

# 2. Drop everything và re-create
docker compose exec postgres psql -U postgres -c "DROP DATABASE dongdo_cs;"
docker compose exec postgres psql -U postgres -c "CREATE DATABASE dongdo_cs;"

# 3. Wipe volume (full reset)
docker volume rm dongdo_cs_v2_pgdata
docker compose up -d
```

### 2.3 Connection pool exhausted

**Triệu chứng:**
```
FATAL: remaining connection slots are reserved
```

**Fix:**

Tăng `max_connections` ở Postgres:
```sql
ALTER SYSTEM SET max_connections = 200;
```

Hoặc giảm `MaxConns` ở pgxpool (sửa code tạm thời tại `internal/repository/postgres/db.go`).

### 2.4 Slow queries

**Diagnose:**
```sql
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

**Fix:**
- Thêm index (xem migration mới)
- Dùng EXPLAIN ANALYZE

---

## 3. Redis

### 3.1 Connection fails

**Triệu chứng:**
```
Redis connection failed after all retries
```

**Fix:**

1. **Redis container running:**
   ```bash
   docker compose ps redis
   docker compose logs redis
   ```

2. **Wrong URL:**
   ```bash
   # .env
   REDIS_URL=redis://redis:6379    # tên service trong docker-compose
   # hoặc
   REDIS_URL=redis://localhost:6379  # khi chạy local
   ```

3. **Auth required:**
   ```bash
   REDIS_URL=redis://:password@host:6379
   ```

### 3.2 Streams không nhận messages

**Triệu chứng:** Workers chạy nhưng không xử lý gì.

**Check:**
```bash
docker compose exec redis redis-cli

# Stream length
> XLEN stream:ws
> XLEN stream:ai
> XLEN stream:db

# Consumer groups
> XINFO GROUPS stream:ai
```

**Fix:** Restart workers:
```bash
docker compose restart server
```

### 3.3 DLQ tăng cao

**Triệu chứng:**
```
> XLEN stream:dlq
   1247
```

**Diagnose:**
```bash
> XRANGE stream:dlq - + COUNT 5
```

**Fix:**
- Check worker logs: `docker compose logs server | grep -i "error"`
- Tăng `RETRY_MAX_COUNT` (nhưng sẽ tốn thời gian)
- Fix root cause (DB down, network, etc.)

---

## 4. Qdrant / RAG

### 4.1 Qdrant unavailable

**Triệu chứng:**
```
Qdrant unavailable; RAG will run in fallback mode
```

**Fallback behavior:** AI vẫn reply nhưng không retrieve context.

**Fix:**
```bash
docker compose ps qdrant
docker compose logs qdrant
```

Restart nếu cần:
```bash
docker compose restart qdrant
```

### 4.2 AI hallucination / sai context

**Triệu chứng:** Trả lời không đúng theo knowledge base.

**Check:**

1. **Documents ingested?**
   ```bash
   docker compose exec server /app/ingest
   ```

2. **Vector count:**
   ```bash
   curl http://localhost:6333/collections/dongdo_kb
   ```

3. **System prompt:** Admin UI → Config → kiểm tra prompt yêu cầu "TUYỆT ĐỐI KHÔNG sử dụng kiến thức bên ngoài"

### 4.3 Embedding dim mismatch

**Triệu chứng:**
```
Vector dimension mismatch: got 768, expected 384
```

**Fix:** Match `EMBEDDING_MODEL` với Qdrant collection vector dim:
- `all-MiniLM-L6-v2` → 384 dim
- `text-embedding-3-small` (OpenAI) → 1536 dim

Tạo lại collection nếu cần:
```bash
curl -X DELETE http://localhost:6333/collections/dongdo_kb
docker compose exec server /app/ingest
```

---

## 5. WebSocket

### 5.1 Disconnect liên tục

**Triệu chứng:** Frontend reconnect loop.

**Check:**

1. **Server logs:**
   ```bash
   docker compose logs server | grep -i "ws"
   ```

2. **Browser DevTools → Network → WS:** Xem close code (1006 = abnormal)

3. **Heartbeat:** Server gửi ping mỗi 30s; client phải respond.

**Fix:**

- Tăng `WS_PING_INTERVAL` nếu network chậm
- Check reverse proxy (nginx) timeout — phải ≥60s cho WebSocket

Nginx config:
```nginx
location /ws {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 300s;  # ≥ WS_PING_INTERVAL
}
```

### 5.2 Events không đến admin_inbox

**Triệu chứng:** Admin không thấy update.

**Check:**

1. **Channel name:** Backend publish đến `session_id` channel. Admin cần connect với `session_id=admin_inbox`.

2. **Browser DevTools → WS frame:** Xem event types.

3. **Backend logs:** `WS hub: client buffer full, dropping event` — buffer đầy.

**Fix:** Tăng Hub broadcast buffer hoặc giảm event rate.

### 5.3 Browser WS không connect

**Triệu chứng:** `WebSocket connection to 'ws://...' failed`

**Check:**

1. **CORS:** Backend allow `*` (development). Production: thêm domain.
2. **URL:** Đúng port (8080).
3. **Proxy/firewall:** Cho phép WS upgrade.

---

## 6. Asterisk / Telephony

Xem chi tiết [TELEPHONY.md §11](./TELEPHONY.md#11-troubleshooting). Tóm tắt:

### 6.1 AMI authentication fails

```
AMI login failed: Authentication failed
```

Fix: `ASTERISK_AMI_PASS` env khớp với `manager.conf` (sau khi entrypoint template).

### 6.2 Calls không bridge

`voice_calls.status` stuck ở `RINGING`.

Check: `docker compose exec asterisk asterisk -rx "pjsip show endpoints"` — agent phải `Avail`.

### 6.3 One-way audio

NAT traversal → set `force_rport, comedia, rtp_symmetric=yes`. Mở firewall `10000-20000/udp`.

### 6.4 SIP register fails

`401 Unauthorized` → sai password. `Timeout` → port 5060 blocked.

### 6.5 AMI disconnects

Tăng `ReadTimeout`, gửi `Action: Ping` mỗi 30s.

---

## 7. Frontend (Next.js)

### 7.1 Hydration mismatch

**Triệu chứng:** Console warning "Hydration failed".

**Nguyên nhân:** Server-rendered HTML ≠ client HTML (thường do store data).

**Fix:** Dùng `useEffect` + state cho data chỉ có ở client:
```tsx
const [hasHydrated, setHasHydrated] = useState(false);
useEffect(() => setHasHydrated(true), []);
if (!hasHydrated) return <Loading />;
```

### 7.2 API call 401

**Triệu chứng:** `401 Unauthorized` cho protected endpoints.

**Fix:** Check token trong localStorage / cookie:
```ts
const token = localStorage.getItem('auth_token');
// Hoặc qua auth store
```

### 7.3 WebRTC mic permission

**Triệu chứng:** Mic không hoạt động.

**Fix:**
- HTTPS required (hoặc localhost)
- Browser settings → site permissions → microphone

### 7.4 Next.js build fails — type errors

```bash
cd web
pnpm type-check
```

Fix errors, rồi:
```bash
pnpm build
```

---

## 8. Performance

### 8.1 Backend CPU cao

**Diagnose:**
```bash
docker compose exec server sh
top -bn1 | head -20
```

**Fix:**
- Scale workers (multi consumer groups)
- Tăng `DB_BATCH_SIZE` để giảm INSERT overhead
- Profile với `pprof`:
  ```go
  import _ "net/http/pprof"
  go func() { http.ListenAndServe(":6060", nil) }()
  ```
  Sau đó: `go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30`

### 8.2 Postgres CPU cao

```sql
-- Long running queries
SELECT pid, now() - pg_stat_activity.query_start AS duration, query
FROM pg_stat_activity
WHERE state != 'idle' AND now() - pg_stat_activity.query_start > interval '5 seconds';
```

**Fix:** Kill query hoặc optimize (index, query rewrite).

### 8.3 Redis memory cao

**Check:**
```bash
docker compose exec redis redis-cli INFO memory
```

**Fix:**
- Trim stream (`XADD MAXLEN ~`)
- Set TTL cho keys
- Tăng `maxmemory` trong `redis.conf`

### 8.4 Recordings disk full

```bash
df -h /var/spool/asterisk/monitor
```

**Fix:**
```bash
# Sync to S3
./scripts/sync-recordings-s3.sh

# Delete local >7 days
find /var/spool/asterisk/monitor -type f -mtime +7 -delete
```

---

## 9. Authentication

### 9.1 Login fails — wrong password

Check user trong DB:
```sql
SELECT username, role, is_active FROM users;
```

Reset password (qua Admin UI hoặc SQL):
```sql
-- Generate PBKDF2 hash + salt rồi update
UPDATE users SET password_hash = '...', salt = '...' WHERE username = 'admin';
```

### 9.2 Token expired

`JWT_EXPIRY_HOURS` quá ngắn. Mặc định 168h (7 days).

Tăng:
```bash
JWT_EXPIRY_HOURS=720  # 30 days
```

Restart server:
```bash
docker compose restart server
```

### 9.3 Permission denied

User role không đủ. Check:
```sql
SELECT username, role FROM users WHERE username = 'cskh01';
```

Update role:
```sql
UPDATE users SET role = 'admin' WHERE username = 'cskh01';
```

Hoặc qua Admin UI → Users.

---

## 10. Recording / Files

### 10.1 Recording files 0 bytes

**Fix:**
- Check `queues.conf`: `monitor_format=wav`, `monitor_type=MixMonitor`
- Check codec (Opus vs WAV)
- Asterisk permissions trên `/var/spool/asterisk/monitor/`

### 10.2 Upload recording fails

**Triệu chứng:** Frontend upload trả 500.

**Check:**
```bash
ls -la ./recordings/
```

**Fix:**
```bash
mkdir -p ./recordings
chmod 0755 ./recordings
```

### 10.3 Static file không serve

**Triệu chứng:** `/static/recordings/xyz.webm` 404.

**Fix:**
- Check file tồn tại
- Path trong handler: `recordingsDir := filepath.Join(h.docsDir, "..", "recordings")`
- Trong Docker: volume mount `./recordings:/app/recordings`

---

## 11. Cross-references

- [TELEPHONY.md §11 Troubleshooting](./TELEPHONY.md#11-troubleshooting) — Asterisk specific
- [DEPLOYMENT.md](./DEPLOYMENT.md) — production
- [DEVELOPMENT.md §9 Debugging](./DEVELOPMENT.md#9-debugging)
- [CONFIGURATION.md](./CONFIGURATION.md) — env tuning
- [API.md](./API.md) — endpoint reference

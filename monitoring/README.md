# DongDo CS — Observability & Monitoring

Stack monitoring cho production của hệ thống CSKH. Chạy song song với
`docker-compose.yml` chính thông qua Docker Compose `profile: monitoring`,
nên mặc định `make up` sẽ **không** khởi động monitoring.

---

## Kiến trúc

```
┌──────────────────────────────────────────────────────────────────────┐
│                         EC2 host                                      │
│                                                                       │
│   Application stack (docker-compose.yml)      Monitoring stack         │
│   ──────────────────────────────────────      ─────────────────────   │
│                                               │                       │
│   ┌─────────┐                                  │  ┌──────────────┐    │
│   │ server  │  ── /metrics (port 9090) ──────►│  │  prometheus │    │
│   └────┬────┘                                  │  └──────┬───────┘    │
│        │                                       │         │             │
│   ┌────▼────┐  ┌────────┐  ┌──────────┐       │   ┌─────▼─────┐     │
│   │postgres │  │ redis  │  │  qdrant  │       │   │ grafana   │     │
│   └─────────┘  └────────┘  └──────────┘       │   │  :3000    │     │
│        │             │                         │   └───────────┘     │
│        │             │                         │                    │
│   ┌────▼─────────────▼────────────────┐        │  ┌──────────────┐   │
│   │  pg-exporter     redis-exporter   │ ◄─────►│  │ alertmanager │   │
│   └───────────────────────────────────┘        │  └──────────────┘   │
│                                                 │                    │
│   ┌───────────────────────────────┐            │  ┌──────────────┐   │
│   │   node-exporter (host)         │  ◄──────────►│  │   node-exp   │   │
│   └───────────────────────────────┘            │  └──────────────┘   │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘

  Golang app: /metrics (port 9090) và /debug/pprof (port 6060)
              ↘ Prometheus scrape
                 ↘ Grafana hiển thị
                 ↘ Alertmanager kích hoạt alert
```

Toàn bộ service monitoring được bật qua `monitoring` Docker Compose profile.
Mặc định `make up` **không** khởi động chúng.

---

## Yêu cầu

- Docker & Docker Compose v2
- Docker network mặc định của app phải tồn tại. Chạy `make up` ít nhất một lần
  trước khi khởi động monitoring stack để tạo network `dongdo_cs_v2_default`.
  `make monitoring-up` sẽ tự tạo fallback network nếu chưa có.

## Xử lý lỗi thường gặp

### `Bind for 0.0.0.0:9090 failed: port is already allocated` khi `make up`

Docker từ chối start `dongdo_server` vì **đã có container cũ (thường là
prometheus hoặc postgres-exporter)** đang nắm port 9090 trên host. Dù
`make monitoring-up` bind 127.0.0.1:9091 (không xung đột trực tiếp), nhưng
**một số container orphan** (vd. chạy lệnh cũ với `--profile` khác, hoặc
`tắt máy giữa chừng`) vẫn chiếm host port 9090.

Triệu chứng:

```
Error response from daemon: failed to set up container networking: ...
Bind for 0.0.0.0:9090 failed: port is already allocated
WARN[0038] Found orphan containers (dongdo_prometheus, ...)
```

Cách xử lý:

```bash
# Bước 1: tìm ai đang giữ 9090.
lsof -nP -iTCP:9090 -sTCP:LISTEN

# Bước 2: dừng toàn bộ monitoring stack (kể cả orphan).
docker compose -p dongdo_cs_v2 \
  --env-file .env.monitoring \
  -f docker-compose.yml \
  -f monitoring/docker-compose.monitoring.yml \
  --profile monitoring --remove-orphans down

# Bước 3: nếu vẫn còn listener lạ, restart Docker Desktop hoặc kill PID đó.

# Bước 4: thử lại.
make up
```

Nếu hay gặp tình trạng này, thêm `--remove-orphans` vào Makefile
target `up`/`monitoring-up`.

### `network dongdo_cs_v2_default declared as external, but could not be found`

Bạn chạy `make monitoring-up` mà chưa chạy `make up` trước, nên network của app
chưa được tạo. Hai cách xử lý:

```bash
# Cách 1 (khuyến nghị): khởi động app stack trước
make up
make monitoring-up

# Cách 2: để Makefile tự tạo fallback network
make monitoring-up
# Makefile sẽ in:
#   ⚠️  dongdo_cs_v2_default missing — creating empty bridge.
#   (Chạy `make up` trước để postgres/redis được resolve.)
# Lưu ý: với fallback, postgres-exporter và redis-exporter KHÔNG thể
# resolve `postgres` / `redis` vì các container đó chưa chạy.
# Chỉ Prometheus + Grafana + node-exporter sẽ healthy.
```

### `localhost:9091` / `localhost:9093` refused to connect

File compose chưa publish port. Đảm bảo bạn đang dùng phiên bản mới nhất:

```bash
docker compose -p dongdo_cs_v2 \
  -f docker-compose.yml \
  -f monitoring/docker-compose.monitoring.yml \
  --profile monitoring ps
# Cả prometheus và alertmanager phải show
# 127.0.0.1:9091->9090 (Prometheus, bound to loopback) và 0.0.0.0:9093->9093 trong cột PORTS.
```

Nếu port bị thiếu, rebuild từ `main` mới nhất (hoặc kiểm tra không chạy
image cũ bị cached).

### Grafana trên 3000 trùng với `npm run dev`

File compose map Grafana sang **3050:3000**. Ghi đè bằng:

```bash
GRAFANA_PORT=4050 make monitoring-up
```

## Bắt đầu lần đầu

```bash
# 1. Khởi động app stack trước (tạo network của app).
make up

# 2. Khởi động monitoring stack bên trên.
make monitoring-up

# 3. Kiểm tra toàn bộ scrape target đã UP (chờ ~15 giây).
curl -s http://localhost:9091/api/v1/targets \
  | python3 -c "import json,sys; t=json.load(sys.stdin)['data']['activeTargets']; \
     [print(f\"{'✅' if x['health']=='up' else '❌'} {x['labels']['job']} ({x['health']})\") for x in t]"

# 4. Mở dashboard.
open http://localhost:3050      # Grafana — admin / admin (đổi mật khẩu!)  [3050 ≠ 3000 tránh trùng npm run dev]
open http://localhost:9091      # Prometheus — query ad-hoc metrics
open http://localhost:9093      # Alertmanager — lịch sử alert
```

## Khởi động nhanh (app đã chạy rồi)

```bash
# Khởi động monitoring stack.
make monitoring-up

# Tắt monitoring thôi (app vẫn chạy).
make monitoring-down

# Xem logs monitoring.
make monitoring-logs

# Kiểm tra container nào đang chạy.
make monitoring-status
```

Tắt toàn bộ:

```bash
make monitoring-down    # chỉ monitoring
make down              # chỉ app
```

---

## Cấu trúc thư mục

```
monitoring/
├── docker-compose.monitoring.yml   # Prometheus, Grafana, exporters
├── prometheus/
│   ├── prometheus.yml            # scrape jobs
│   └── alert.rules.yml           # alerting rules
├── alertmanager/
│   └── alertmanager.yml          # webhook receivers
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/datasource.yml     # tự đăng ký Prometheus
│   │   └── dashboards/dashboards.yml      # tự load dashboard
│   └── dashboards/
│       ├── infrastructure.json   # Host CPU / memory / disk / network
│       ├── golang-api.json       # Request rate, P95, errors, GC, heap
│       ├── chat-websocket.json  # WS connections, messages, agent states
│       ├── database.json         # Postgres pool, queries, locks
│       └── redis.json            # Memory, commands, streams, pools
└── README.md (file này)
```

Instrumentation của Go nằm trong `internal/observability/`:
- `metrics.go`           — Prometheus registry + Go runtime collectors
- `http_metrics.go`      — Gin middleware + HTTP server /metrics
- `ws_metrics.go`        — WebSocket counters/gauges
- `business_metrics.go`  — Business gauges driven by polling
- `pprof.go`             — net/http/pprof trên dedicated internal port
- `logger.go`            — request_id middleware + structured access log

---

## Endpoints & port mặc định

| Service            | Address (từ host)                | Ghi chú                              |
|--------------------|----------------------------------|--------------------------------------|
| App API            | `http://localhost:8080`          | public                               |
| App /metrics       | `http://localhost:9090/metrics`  | loopback only — xem security          |
| App /debug/pprof   | `http://localhost:6060/debug/pprof/` | loopback only — xem security     |
| Grafana            | `http://localhost:3050`          | admin / admin (đổi mật khẩu!). **3050 không phải 3000** — tránh trùng `npm run dev`. |
| Prometheus         | `http://localhost:9091`          | UI để query ad-hoc. **9091 tránh trùng với Go server /metrics trên 9090**. Prometheus bind loopback (127.0.0.1) nên chỉ accessible từ host. |
| Alertmanager       | `http://localhost:9093`           | UI để xem lịch sử alert              |
| Node exporter      | (không publish port ra host)     | chỉ Prometheus scrape                |
| PG exporter        | (không publish port ra host)     | chỉ Prometheus scrape                |
| Redis exporter     | (không publish port ra host)     | chỉ Prometheus scrape                |

### Ghi chú về port

- **Grafana trên 3000** trùng với Next.js `npm run dev`. Map sang
  `3050:3000` để chạy cả hai cùng lúc. Ghi đè bằng
  `GRAFANA_PORT=<port-tự-chọn> make monitoring-up`.
- **Prometheus UI đã đổi sang 9091** (loopback only). Go server vẫn giữ
  `/metrics` trên host:9090. Hai service không còn cùng port number nên
  không có cách nào nhầm lẫn. Prometheus scrape `server:9090` qua Docker
  network (container name, không phải host port). Ghi đè bằng
  `PROMETHEUS_PORT=<port-tự-chọn> make monitoring-up`.

---

## Mô hình bảo mật

- **/metrics và /debug/pprof** bind vào `127.0.0.1` mặc định. Chỉ reachable
  từ bên trong host.
- **Toàn bộ exporters** (node / postgres / redis) sống trên private
  `dongdo-monitoring` Docker network. Không có host port nào được publish.
- **Grafana / Prometheus / Alertmanager** được expose ra host để tiện dùng.
  Trước khi deploy production:
  1. Đổi `GRAFANA_ADMIN_PASSWORD`.
  2. Đặt sau SSH tunnel hoặc VPN.
  3. Thêm authentication (reverse proxy) trước khi public ra internet.
- **Không có credentials hardcoded**. Override qua `.env`
  (ví dụ: `GRAFANA_ADMIN_PASSWORD`, `DATA_SOURCE_NAME`).

---

## Thao tác thường dùng

### Debug CPU cao trong Go app

```bash
# Capture 30 giây CPU profile (mở interactive pprof prompt).
make pprof-cpu

# Trong pprof:
#   top 20           — top 20 hàm nóng nhất
#   list <func>      — source có annotation của một hàm
#   web              — mở flame graph trong trình duyệt
```

### Debug memory leak

```bash
# Snapshot heap (alloc + inuse).
make pprof-heap

# Trong pprof:
#   top 20 -cum      — top allocator theo cumulative bytes
#   list <func>      — source có annotation
```

### Debug goroutine leak

```bash
# Kiểm tra toàn bộ goroutine đang chạy.
make pprof-goroutine

# Trong pprof:
#   top 20           — top stack traces theo count
#   traces           — hiển thị full text của mỗi top trace
```

### Tìm SQL query chậm

Mở Prometheus (`http://localhost:9091`) và query:

```promql
# Top 10 query chậm nhất theo mean execution time
topk(10, pg_stat_statements_mean_exec_time_seconds / 1000)

# Top 10 query được gọi nhiều nhất
topk(10, rate(pg_stat_statements_calls_total[5m]))

# Query có total time cao nhất trong 1 giờ qua
topk(10, increase(pg_stat_statements_total_exec_time_seconds[1h]) / 1000)
```

Dashboard `Database` trong Grafana có sẵn các panel này.

### Kiểm tra Redis Streams

```bash
# Qua Prometheus (UI hoặc curl)
curl -s http://localhost:9091/metrics | grep redis_stream

# Hoặc trực tiếp qua redis-cli trong container:
docker exec -it dongdo_redis \
  redis-cli XINFO STREAM stream:ws
docker exec -it dongdo_redis \
  redis-cli XPENDING stream:ws ws_group
```

### Xem ai đang online

Mở Grafana → dashboard "Chat / WebSocket":

- **Active WS Connections (by role)** — số client đang kết nối.
- **Agent States (online / available / busy)** — tình trạng nhân viên.
- **Customers Waiting** — cases cần nhân viên tiếp nhận.

---

## Cấu trúc env

Monitoring stack dùng **2 file env riêng biệt**, tách biệt hoàn toàn:

| File | Ai quản lý | Chứa |
|------|------------|------|
| `.env` | App dev | Server config, DB, Redis, AI keys, JWT secrets |
| `.env.monitoring` | SRE / Observability | Grafana credentials, Grafana/Prometheus ports |

**Lý do tách**: SRE team chỉ cần access `.env.monitoring` để rotate Grafana password
hoặc đổi port, không cần biết app secrets (API keys, JWT secret).

```bash
# .env.monitoring — chỉ monitoring, KHÔNG chứa app secrets
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=admin
GRAFANA_ROOT_URL=http://localhost:3050
GRAFANA_PORT=3050
PROMETHEUS_PORT=9090
```

Makefile tự động truyền `--env-file .env.monitoring` khi chạy monitoring commands.
Nếu chạy trực tiếp `docker compose` (không qua Makefile), nhớ thêm flag này:

```bash
docker compose \
  --env-file .env.monitoring \
  -f docker-compose.yml \
  -f monitoring/docker-compose.monitoring.yml \
  --profile monitoring up -d
```

---

## Cấu hình

Toàn bộ setting điều khiển qua environment variable. Giá trị mặc định an toàn
(bind loopback).

### App-side observability (trong `.env`)

| Env var                     | Mặc định            | Mục đích                            |
|-----------------------------|----------------------|-------------------------------------|
| `METRICS_ENABLED`           | `true`               | Master switch cho /metrics          |
| `METRICS_ADDR`              | `127.0.0.1:9090`     | Địa chỉ Prometheus scrape           |
| `PPROF_ENABLED`             | `true`               | Master switch cho pprof             |
| `PPROF_ADDR`               | `127.0.0.1:6060`     | Địa chỉ listen của pprof            |
| `BUSINESS_METRICS_ENABLED`  | `true`               | Bật business gauge poller           |
| `BUSINESS_METRICS_POLL_SEC` | `15`                 | Polling interval (giây)              |
| `REDIS_POOL_METRICS_ENABLED`| `true`               | Export go-redis pool stats          |

### Monitoring stack (trong `.env.monitoring`)

| Env var                  | Mặc định            | Mục đích                            |
|--------------------------|----------------------|-------------------------------------|
| `GRAFANA_ADMIN_USER`     | `admin`              | Username đăng nhập Grafana          |
| `GRAFANA_ADMIN_PASSWORD` | `admin`              | Password đăng nhập Grafana          |
| `GRAFANA_ROOT_URL`       | `http://localhost:3050` | Public URL cho OAuth/deep links  |
| `GRAFANA_PORT`           | `3050`               | Host port expose Grafana UI         |
| `PROMETHEUS_PORT`        | `9090`               | Host port expose Prometheus UI      |

> **Lưu ý**: Bảng ở phiên bản cũ đã gộp Grafana vars vào `.env`. Sau khi tách,
> Grafana vars chuyển sang `.env.monitoring`. App vars (`METRICS_*`, `PPROF_*`,
> `BUSINESS_METRICS_*`, `REDIS_POOL_*`) vẫn nằm trong `.env` vì chúng điều khiển
> Go server bind `/metrics` và `/debug/pprof`.

---

## Alerting

Alert rules nằm trong `monitoring/prometheus/alert.rules.yml`. Mặc định cover:

- Host CPU / memory / disk pressure
- Go goroutine leak / heap growth / GC pauses
- HTTP 5xx rate / P95 latency
- WebSocket drops / errors / long waits
- Postgres connection saturation / slow queries
- Redis memory / stream lag / exporter down

Alertmanager đã được wire nhưng mặc định là null webhook. Để tích hợp
Slack, Telegram, PagerDuty hoặc OpsGenie, sửa
`monitoring/alertmanager/alertmanager.yml` và thêm URL receiver thật.

---

## Kiểm tra hoạt động

```bash
# 1. Config hợp lệ.
make monitoring-config

# 2. App /metrics phản hồi (trong shell khác, sau khi make up).
make metrics

# 3. Toàn bộ Prometheus targets phải là UP.
curl -s http://localhost:9091/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'

# 4. Alert rules compile được.
curl -s http://localhost:9091/api/v1/rules | jq '.data.groups[].rules[].name'

# 5. Grafana datasource kết nối thành công.
# Truy cập http://localhost:3050/datasources và kiểm tra Prometheus màu xanh.
```

---

## Ghi chú về tên network

`monitoring/docker-compose.monitoring.yml` tham gia vào app network
`dongdo_cs_v2_default` (default bridge mà `docker-compose.yml` tạo).
Nếu project name của bạn khác (vì đổi tên thư mục, hoặc dùng
`COMPOSE_PROJECT_NAME=foo`), hãy update network declaration
`external: true` trong file monitoring cho khớp với
`${COMPOSE_PROJECT_NAME}_default`.

Kiểm tra tên network thực tế:

```bash
docker network ls | grep default
```

# Hướng dẫn sử dụng Grafana — DongDo CS

> **Ngôn ngữ:** Tiếng Việt
> **Phiên bản:** 2026-09-08
> **Yêu cầu:** Đã chạy `make up` + `make monitoring-up`

---

## Mục lục

1. [Truy cập Grafana](#1-truy-cập-grafana)
2. [Tình trạng thực tế metrics hiện tại](#2-tình-trạng-thực-tế-metrics-hiện-tại)
3. [Cách đọc từng dashboard](#3-cách-đọc-từng-dashboard)
4. [Xem metrics thực tế trên Prometheus](#4-xem-metrics-thực-tế-trên-prometheus)
5. [Sau khi rebuild — metrics đầy đủ](#5-sau-khi-rebuild--metrics-đầy-đủ)
6. [Troubleshooting — Dashboard trống](#6-troubleshooting--dashboard-trống)

---

## 1. Truy cập Grafana

Mở trình duyệt và vào:

```
http://localhost:3050
```

- **Username:** `admin`
- **Password:** `admin` (nên đổi sau khi setup xong)

### Cách vào đúng dashboard

Sau khi login, bên trái màn hình có menu **Dashboards**:

```
Dashboards → Browse
```

Hoặc click **+** → **Dashboard** → **New** → **Import** (nếu dashboard chưa tự load).

Mặc định 5 dashboard đã được auto-load:

| Dashboard | Mục đích |
|---|---|
| **DongDo CS — Infrastructure** | CPU, RAM, disk, network của máy chủ |
| **DongDo CS — Golang API** | Request rate, lỗi, latency, goroutines, memory |
| **DongDo CS — Chat / WebSocket** | Kết nối WS, tin nhắn, trạng thái agent |
| **DongDo CS — Database (Postgres)** | Connections, slow query, locks |
| **DongDo CS — Redis** | Memory, commands/sec, streams, pool |

---

## 2. Tình trạng thực tế metrics hiện tại

**Kiểm tra nhanh bằng terminal:**

```bash
# Xem Prometheus đang scrape những gì
curl -s http://localhost:9091/api/v1/targets | python3 -c "
import sys,json
d=json.load(sys.stdin)
for t in d['data']['activeTargets']:
    h = t['health']
    job = t['labels']['job']
    print(f'  {job:15} → {h}')
"

# Đếm số metrics trong Prometheus (Prometheus cũng expose /metrics endpoint)
curl -s http://localhost:9091/metrics | grep -c "^[a-z_]"

# Xem metrics Go runtime (luôn có)
curl -s http://localhost:9090/metrics | grep "^go_"

# Xem metrics app-specific (CẦN REBUILD)
curl -s http://localhost:9090/metrics | grep -E "^(http_|chat_|ws_|ai_|partner_|business_)"
```

### Bảng trạng thái metrics

| Metric | Trước rebuild | Sau rebuild |
|---|---|---|
| `go_goroutines`, `go_memstats_*`, `process_*` | ✅ Có | ✅ Có |
| `http_requests_total`, `http_request_duration_seconds_*` | ❌ Không | ✅ Có |
| `http_requests_in_flight` | ❌ Không | ✅ Có |
| `ws_active_connections`, `ws_messages_*` | ❌ Không | ✅ Có |
| `chat_messages_total`, `ai_responses_total` | ❌ Không | ✅ Có |
| `db_pool_acquired`, `db_pool_idle`, `db_pool_total` | ❌ Không | ✅ Có |
| `redis_memory_used_bytes`, `redis_commands_*` | ⚠️ Exporter OK, cần Redis dùng | ✅ Có |

---

## 3. Cách đọc từng dashboard

### 3.1. Infrastructure

**Dùng khi:** Server chậm, muốn biết máy chủ có bị quá tải không.

| Biểu đồ | Ý nghĩa | Ngưỡng cảnh báo |
|---|---|---|
| **CPU Usage** | % CPU đang dùng | > 80% trong 5 phút |
| **Memory Usage** | RAM đã dùng / tổng | > 85% |
| **Disk Usage** | Ổ cứng còn bao nhiêu | < 10GB hoặc < 10% |
| **Load Average** | Độ tải trung bình (1/5/15 phút) | > số CPU cores |
| **Network I/O** | Lưu lượng vào/ra (bytes/s) | Tùy bandwidth |

**Cách đọc:**
- Nếu **CPU Usage** liên tục > 80% → có hàm nào đó ngốn CPU (cần pprof debug)
- Nếu **Memory** tăng đều không giảm → có thể memory leak
- Nếu **Disk** gần full → dọn log, old data

### 3.2. Golang API

**Dùng khi:** API trả lời chậm, lỗi 500 tăng, muốn biết endpoint nào chậm.

| Biểu đồ | Ý nghĩa | Ngưỡng cảnh báo |
|---|---|---|
| **Request Rate (req/s)** | Số request mỗi giây, phân theo route | Tùy load |
| **Error Rate (5xx %)** | % request trả lỗi server | > 1% |
| **Latency P50 / P95 / P99** | Thời gian response trung bình | P99 > 1s |
| **Goroutines** | Số goroutine đang chạy | Tăng đột ngột |
| **Heap In-Use / Allocated** | Bộ nhớ heap của Go app | Tăng liên tục |
| **GC Duration P99** | Thời gian Garbage Collection | > 100ms |
| **In-Flight Requests** | Request đang xử lý cùng lúc | Quá nhiều |

**Cách đọc:**
- **Error Rate > 1%** → Có lỗi 5xx, vào tab Explore của Grafana query `http_requests_total{status=~"5.."}`
- **P99 > 1s** → API chậm, có thể do DB query chậm hoặc AI call
- **Goroutines tăng liên tục** → Goroutine leak (cần pprof)
- **Heap tăng không giảm** → Memory leak (cần pprof heap profile)

### 3.3. Chat / WebSocket

**Dùng khi:** Khách hàng phản ánh chat bị đứt, tin nhắn không gửi được.

| Biểu đồ | Ý nghĩa | Ngưỡng cảnh báo |
|---|---|---|
| **Active Connections** | Số kết nối WebSocket đang mở | Tùy số user |
| **Messages In / Out per sec** | Tin nhắn vào/ra mỗi giây | = 0 liên tục = có bug |
| **Agent States** | Đang rảnh / đang chat / offline | Quá nhiều offline |
| **Waiting Customers** | Khách đang chờ agent trả lời | > 0 quá lâu |
| **Queue Length** | Hàng đợi chưa được assign | > 10 |

**Cách đọc:**
- **Active Connections = 0** mà có user đang dùng → WS không hoạt động
- **Waiting Customers tăng** → Agent không đủ, hoặc agent offline
- **Messages/sec = 0** → AI worker có thể bị stuck

### 3.4. Database (Postgres)

**Dùng khi:** API chậm, nghi ngờ SQL query chậm.

| Biểu đồ | Ý nghĩa | Ngưỡng cảnh báo |
|---|---|---|
| **DB Connections** | Pool: acquired / idle / total | Acquired = max → pool đầy |
| **Postgres Active Connections** | Tổng kết nối thực vào Postgres | Gần max_connections |
| **Top Queries — Mean Exec Time** | 10 query chậm nhất (giây) | > 1s |
| **Top Queries — Calls/sec** | Query được gọi nhiều nhất | Query thường xuyên + chậm = nguy hiểm |
| **Locks — Waiting Queries** | Query đang chờ lock (deadlock indicator) | > 0 kéo dài |

**Cách đọc:**
- **Connections = max** → Cần tăng pool size hoặc fix connection leak
- **Slow query > 1s** → Cần EXPLAIN ANALYZE, xem có missing index không
- **Locks > 0** → Kiểm tra `SELECT * FROM pg_stat_activity WHERE wait_event_type = 'Lock'`

### 3.5. Redis

**Dùng khi:** Redis lạ, session bị mất, message queue đầy.

| Biểu đồ | Ý nghĩa | Ngưỡng cảnh báo |
|---|---|---|
| **Memory Used** | RAM Redis đang dùng / max | > 80% của maxmemory |
| **Connected Clients** | Số client đang kết nối | Tăng đột ngột |
| **Commands / sec** | Tổng Redis commands mỗi giây | Tùy traffic |
| **Cache Hit Rate** | % read hit (nên > 80%) | < 60% |
| **Stream Length** | Độ dài mỗi stream (queue) | Quá dài = worker không xử lý kịp |
| **Stream Pending** | Số message chưa được acknowledge | Tăng = lag |

**Cách đọc:**
- **Memory > maxmemory** → Redis evict keys, session/message có thể mất
- **Stream Length tăng** → Worker đang chậm, cần thêm worker hoặc fix worker bug
- **Hit Rate < 60%** → Cache không hiệu quả, kiểm tra key TTL

---

## 4. Xem metrics thực tế trên Prometheus

Nếu dashboard Grafana trống hoặc muốn query tự do:

### Cách 1: Prometheus UI (nhanh nhất)

Mở: `http://localhost:9091`

Vào tab **Graph** → gõ PromQL → Execute

**Ví dụ một số query hay:**

```promql
# Tất cả metrics đang có
{__name__=~".+"}

# Go runtime
go_goroutines
go_memstats_heap_inuse_bytes
rate(process_cpu_seconds_total[5m])

# HTTP metrics (cần rebuild)
rate(http_requests_total[1m])
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))

# WebSocket (cần rebuild)
ws_active_connections
rate(ws_messages_in_total[1m])

# Postgres
pg_stat_activity_count
pg_stat_statements_mean_exec_time_seconds / 1000

# Redis
redis_memory_used_bytes
rate(redis_commands_processed_total[1m])
```

### Cách 2: Grafana Explore (trực quan hơn)

Trong Grafana: click **Explore** (biểu tượng la bàn) bên trái → chọn datasource **Prometheus** → gõ query

Explore cho phép:
- Chạy nhiều query song song
- Xem raw data dạng bảng
- Xem dữ liệu dạng graph tạm thời
- Không cần tạo dashboard mới

---

## 5. Sau khi rebuild — metrics đầy đủ

### Bước 1: Rebuild Go server

```bash
make rebuild
```

Container sẽ build lại binary với observability middleware và restart.

### Bước 2: Đợi Prometheus scrape lại

Prometheus scrape mỗi **30 giây** (scrape_interval trong prometheus.yml).
Sau khi container restart (~10-30 giây), vào Prometheus check:

```bash
curl -s http://localhost:9090/metrics | grep -E "^(http_|chat_|ws_|ai_|db_pool)"
```

Nếu thấy dòng `http_requests_total ...` → đã có metrics.

### Bước 3: Kiểm tra trên Grafana

1. Vào dashboard **Golang API** → đổi time range từ "Last 5 minutes" → "Last 15 minutes" (vì vừa có data)
2. Refresh dashboard (F5 hoặc click biểu tượng refresh)
3. Nếu panel vẫn "No data" → xem [Section 6](#6-troubleshooting--dashboard-trống)

### Danh sách metrics mới xuất hiện sau rebuild

```bash
# HTTP
http_requests_total{endpoint="/api/...",method="GET",status="200"}
http_request_duration_seconds_bucket{endpoint="/api/...",le="0.1"}
http_requests_in_flight{endpoint="/api/..."}

# WebSocket
ws_active_connections{session_type="customer|agent"}
ws_messages_in_total{session_type="customer|agent"}
ws_messages_out_total{session_type="customer|agent"}

# Business
chat_messages_total{channel="whatsapp|web|chat",direction="inbound|outbound"}
ai_responses_total{status="success|error|timeout"}
partner_revenue_vnd{partner_id="..."}

# Database pool
db_pool_acquired{pool="postgres|redis"}
db_pool_idle{pool="postgres|redis"}
db_pool_total{pool="postgres|redis"}
```

---

## 6. Troubleshooting — Dashboard trống

### Triệu chứng: Vào dashboard thấy "No data"

**Bước 1: Kiểm tra Prometheus có data không**

```bash
curl -s http://localhost:9091/api/v1/query?query=up
```

Kết quả mong đợi:
```
instance="host.docker.internal:9090"   job="server"       value=1
instance="postgres-exporter:9187"      job="postgres"      value=1
instance="redis-exporter:9121"         job="redis"        value=1
instance="node-exporter:9100"          job="node"         value=1
```

Nếu `server` = 0 → Go app không expose `/metrics`.
→ **Cần rebuild:** `make rebuild`

Nếu `postgres` = 0 → postgres-exporter không kết nối được Postgres.
→ Kiểm tra: `docker logs dongdo_postgres_exporter`

Nếu `redis` = 0 → redis-exporter không kết nối được Redis.
→ Kiểm tra: `docker logs dongdo_redis_exporter`

**Bước 2: Kiểm tra metric có trong Prometheus không**

Ví dụ với dashboard Golang API:

```bash
# Metric Go runtime (luôn có sau khi scrape)
curl -s http://localhost:9091/api/v1/query?query=go_goroutines

# Metric HTTP (cần rebuild)
curl -s http://localhost:9091/api/v1/query?query=http_requests_total
```

Nếu metric Go runtime có → Prometheus scrape đúng.
Nếu metric HTTP không có → binary chưa rebuild.

**Bước 3: Kiểm tra dashboard datasource**

Trong Grafana, vào dashboard → click **Panel title** → **Edit** → tab **Queries**:
- Datasource phải là **Prometheus** (uid: `prometheus`)
- Query phải không có lỗi (màu xanh bên trái query)

**Bước 4: Kiểm tra time range**

Mặc định dashboard set **Last 1 hour**.
Nếu app mới start, có thể chưa đủ 1 tiếng data.
→ Đổi time range: góc trên bên phải → chọn **Last 5 minutes** hoặc **Last 15 minutes**

**Bước 5: Xem container logs**

```bash
# Go server logs
docker logs dongdo_server --tail=50 -f

# Prometheus logs
docker logs dongdo_prometheus --tail=50 -f

# postgres-exporter logs
docker logs dongdo_postgres_exporter --tail=30
```

### Triệu chứng: Panel "N/A" hoặc "Error"

Kiểm tra query trong panel đó:
1. Click panel → **Edit** → **Queries**
2. Click **Inspector** → **Stats** → xem error message
3. Thường là do metric name không đúng hoặc label không match

### Triệu chứng: Prometheus container không start

```bash
# Xem logs
docker logs dongdo_prometheus

# Thường do volume permission hoặc prometheus.yml lỗi
# Kiểm tra config
docker run --rm --network dongdo-monitoring \
  prom/prometheus:v2.55.1 promtool check config \
  /etc/prometheus/prometheus.yml
```

---

## Tổng kết nhanh

| Thao tác | Câu lệnh |
|---|---|
| Mở Grafana | `http://localhost:3050` |
| Mở Prometheus | `http://localhost:9091` |
| Rebuild server (lấy metrics mới) | `make rebuild` |
| Kiểm tra targets | `curl -s http://localhost:9091/api/v1/targets` |
| Kiểm tra metric cụ thể | `curl -s http://localhost:9091/api/v1/query?query=TEN_METRIC` |
| Xem server logs | `docker logs dongdo_server --tail=50 -f` |
| Xem Prometheus logs | `docker logs dongdo_prometheus --tail=30` |
| Đổi time range dashboard | Góc trên phải → chọn "Last 5 minutes" |

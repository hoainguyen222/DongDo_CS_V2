# Production Load Test Plan - CSKH System

## 📋 Tổng Quan

Document này mô tả kế hoạch test load toàn diện cho hệ thống CSKH, bao gồm:
- Các trường hợp test thực tế theo kịch bản kinh doanh
- Phương pháp đo lường hiệu năng
- Tiêu chí đánh giá trước khi deploy production

---

## 🎯 Mục Tiêu Test

| Chỉ số | Target | Critical |
|--------|--------|----------|
| Concurrent users | 100+ | 50 |
| Concurrent chats | 500+ | 200 |
| Messages/second | 100+ | 50 |
| Response time (API) | < 500ms | < 1s |
| Response time (AI) | < 3s | < 5s |
| Error rate | < 1% | < 5% |
| Uptime | 99.9% | 99% |

---

## 📊 Kịch Bản Test Thực Tế

### 1. Kịch Bản Giờ Cao Điểm (Peak Hours)

**Mô tả:** Mô phỏng lưu lượng trong giờ làm việc cao điểm

```
Thời gian test: 10-15 phút
Users đồng thời: 50-100
Messages/user: 10-20
Think time: 2-5 giây giữa các messages
```

**Test cases:**
- [ ] 50 users chat đồng thời
- [ ] 100 users chat đồng thời
- [ ] AI response time với 50 concurrent users
- [ ] CS agent nhận cases trong peak

### 2. Kịch Bản Rush Hour (Đột Ngột)

**Mô tả:** Mô phỏng lưu lượng tăng đột ngột (sau TVC quảng cáo, khuyến mãi)

```
Thời gian ramp-up: 30 giây
Users tăng: 0 -> 100
Messages burst: 10 messages/user trong 10 giây
```

**Test cases:**
- [ ] Burst 100 messages trong 10 giây
- [ ] 200 users đăng ký trong 1 phút
- [ ] 500 messages đồng thời
- [ ] System recovery sau burst

### 3. Kịch Bản Ca Làm Việc (Shift Coverage)

**Mô tả:** Test theo ca CSKH thực tế

```
Ca sáng: 8:00-12:00 (3 agents, 20 customers)
Ca chiều: 13:00-17:00 (3 agents, 20 customers)
Ca tối: 18:00-22:00 (2 agents, 10 customers)
```

**Test cases:**
- [ ] 3 CS agents xử lý 20 concurrent cases
- [ ] Case distribution giữa các agents
- [ ] Escalation khi queue đầy
- [ ] Handover giữa các ca

### 4. Kịch Bản Ngày Lễ/Cuối Tuần (Holiday)

**Mô tả:** Volume cao hơn bình thường 50-100%

```
Users đồng thời: 150
Messages/user: 15
Duration: 30 phút
```

**Test cases:**
- [ ] 150 concurrent users
- [ ] AI handling 70% queries
- [ ] Human escalation 30%
- [ ] Knowledge base queries

### 5. Kịch Bản Stress Test

**Mô tả:** Tìm giới hạn của hệ thống

```
Progressive increase:
- 10 -> 50 -> 100 -> 200 -> 500 users
- Monitor tại mỗi level
- Find breaking point
```

**Test cases:**
- [ ] Tìm max concurrent connections
- [ ] Tìm max messages/second
- [ ] Database connection pool saturation
- [ ] Memory leak detection

---

## 🔬 Các Loại Test Chi Tiết

### A. Connection Tests

| Test | Mô tả | Metrics |
|------|--------|---------|
| `connection_flood` | 500+ connections đồng thời | Connection time, success rate |
| `connection_sustained` | Giữ 100 connections trong 30 phút | Connection drop rate |
| `connection_reconnect` | Reconnect sau disconnect | Success rate, time |

### B. Chat Throughput Tests

| Test | Mô tả | Metrics |
|------|--------|---------|
| `chat_parallel` | Nhiều user chat song song | Messages/sec, latency |
| `chat_burst` | 100 messages trong 5 giây | Success rate, queue depth |
| `chat_sustained` | 10 messages/user trong 10 phút | Response time trend |

### C. AI Response Tests

| Test | Mô tả | Metrics |
|------|--------|---------|
| `ai_response_time` | 50 concurrent AI requests | Avg/Max/P95 response time |
| `ai_consistency` | 100 requests cùng câu hỏi | Response consistency |
| `ai_fallback` | Test khi AI fail | Fallback success rate |

### D. CS Agent Tests

| Test | Mô tả | Metrics |
|------|--------|---------|
| `cs_take_case` | Agent nhận case | Success rate, time |
| `cs_reply` | Agent reply customer | Message delivery |
| `cs_escalation` | Chuyển AI -> Human | Escalation time |
| `cs_workload` | Phân phối cases | Load balancing |

### E. System Resource Tests

| Test | Mô tả | Metrics |
|------|--------|---------|
| `memory_usage` | Monitor memory | Memory leak detection |
| `cpu_usage` | Monitor CPU | CPU under load |
| `db_connections` | Database pool | Connection saturation |
| `network_latency` | Network latency | Latency distribution |

---

## 📈 Metrics Thu Thập

### Performance Metrics
```
- Response Time (Avg, P50, P90, P95, P99, Max)
- Throughput (Requests/Second)
- Error Rate (%)
- Timeout Rate (%)
- Success Rate (%)
```

### Resource Metrics
```
- CPU Usage (%)
- Memory Usage (MB/%)
- Database Connections
- Network I/O
- Disk I/O
```

### Business Metrics
```
- Chat Completion Rate
- AI Resolution Rate (%)
- Human Escalation Rate (%)
- Average Handle Time
- Customer Satisfaction Score
```

---

## ⚙️ Test Environment Requirements

### Minimum (Development)
```
- Server: 2 CPU, 4GB RAM
- Database: Local PostgreSQL
- Network: localhost
- Concurrent users: 50
```

### Recommended (Staging)
```
- Server: 4 CPU, 8GB RAM
- Database: PostgreSQL on same network
- Network: Low latency
- Concurrent users: 200
```

### Production-ready (Pre-launch)
```
- Server: 8 CPU, 16GB RAM
- Database: Managed PostgreSQL
- Load Balancer: Yes
- Concurrent users: 500+
```

---

## 📝 Run Commands

### Quick Smoke Test (2 phút)
```bash
npx ts-node production-load-test.ts --scenario smoke
```

### Medium Load Test (15 phút)
```bash
npx ts-node production-load-test.ts --scenario medium --verbose
```

### Full Production Test (30-60 phút)
```bash
npx ts-node production-load-test.ts --scenario full --report
```

### Stress Test (Find Limits)
```bash
npx ts-node production-load-test.ts --scenario stress --progressive
```

### Custom Test
```bash
npx ts-node production-load-test.ts --users 100 --duration 10m --ramp 30s
```

---

## ✅ Pre-Production Checklist

### Performance
- [ ] Response time < 500ms (P95)
- [ ] Error rate < 1%
- [ ] No memory leaks after 1 hour
- [ ] Database connections stable

### Functionality
- [ ] AI responses working
- [ ] CS agent workflow complete
- [ ] WebSocket connections stable
- [ ] Case escalation working

### Scalability
- [ ] Tested with 2x expected peak load
- [ ] Auto-scaling configured (if cloud)
- [ ] Load balancing verified
- [ ] Caching working

### Monitoring
- [ ] APM tools configured
- [ ] Alerts set up
- [ ] Dashboards created
- [ ] Log aggregation working

---

## 📊 Sample Report Structure

```json
{
  "test_run": {
    "scenario": "peak_hours",
    "timestamp": "2024-01-15T10:00:00Z",
    "duration_minutes": 15
  },
  "load_profile": {
    "concurrent_users": 100,
    "messages_per_user": 15,
    "think_time_seconds": 3
  },
  "results": {
    "api": {
      "avg_response_ms": 245,
      "p95_response_ms": 480,
      "error_rate": 0.5
    },
    "ai": {
      "avg_response_ms": 2100,
      "p95_response_ms": 3500,
      "success_rate": 98.5
    }
  },
  "system_metrics": {
    "cpu_peak": 65,
    "memory_peak_mb": 2048,
    "db_connections_peak": 45
  },
  "verdict": "PASS" | "FAIL" | "CONDITIONAL_PASS"
}
```

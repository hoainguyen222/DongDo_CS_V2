# CSKH Automation Test Suite

Bộ test automation cho hệ thống Chăm Sóc Khách Hàng (CSKH) của Đông Đô Partners.

## 📁 Cấu Trúc

```
tests/automation/
├── config.ts                  # Cấu hình test cơ bản
├── config.agents.json         # Cấu hình agent users & bearer tokens
├── agent-config.ts            # Agent config loader & token manager
├── guest-load-test.ts         # Test số lượng guest đồng thời
├── message-load-test.ts        # Test số lượng message đồng thời
├── websocket-test.ts          # Test kết nối WebSocket
├── ai-response-test.ts        # Test AI response
├── team-agent-test.ts         # Test đội ngũ CSKH
├── batch-runner.ts            # Batch/parallel test runner
├── run-all-tests.ts           # Test runner chính
├── index.ts                   # Export tất cả functions
├── browser-test-template.ts   # Template HTML test trên browser
├── quick-smoke-test.js        # Test nhanh smoke test
├── package.json               # Dependencies & scripts
├── tsconfig.json             # TypeScript config
└── README.md                 # Documentation
```

## 🚀 Cài Đặt

```bash
# Di chuyển vào folder tests
cd tests/automation

# Cài đặt dependencies
npm install

# Hoặc cài đặt trực tiếp
npm install ts-node typescript @types/node ws node-fetch
```

## 🎯 Các Loại Test

### 1. Guest Concurrency Test
- **Mục tiêu**: Đo lường số lượng guest tối đa có thể kết nối cùng lúc
- **Test cases**:
  - Đăng ký nhiều guest đồng thời
  - Test gradual increase để tìm điểm giới hạn
  - Đo thời gian kết nối trung bình

### 2. Message Concurrency Test
- **Mục tiêu**: Test throughput và latency của hệ thống chat
- **Test cases**:
  - Gửi nhiều message đồng thời từ 1 guest
  - Nhiều guest gửi message đồng thời
  - Burst messages (gửi liên tục không delay)

### 3. WebSocket Test
- **Mục tiêu**: Test kết nối real-time ổn định
- **Test cases**:
  - Kết nối WebSocket cơ bản
  - Nhận message qua WebSocket
  - Reconnection khi mất kết nối
  - Đo latency
  - Nhiều kết nối đồng thời

### 4. AI Response Test
- **Mục tiêu**: Test khả năng và chất lượng trả lời của AI
- **Test cases**:
  - Test với các loại câu hỏi khác nhau
  - Đo thời gian phản hồi trung bình
  - Test consistency (độ ổn định)
  - Test fallback khi AI fail
  - Test trigger chuyển sang human CS
  - Test concurrent AI requests

### 5. Team Agent Support Test
- **Mục tiêu**: Test khả năng hỗ trợ của đội ngũ CSKH
- **Test cases**:
  - Chuyển case từ AI sang human CS
  - CS agent nhận và xử lý case
  - CS agent gửi reply
  - Resolve case
  - Real-time notification cho CS team
  - Phân phối case cho nhiều CS agents

## 📝 Batch Testing với Agent Users

### Cấu hình Agent Users (config.agents.json)

File `config.agents.json` chứa thông tin các agent users để test với bearer tokens:

```json
{
  "agent_users": [
    {
      "id": "agent_001",
      "username": "cskh01",
      "password": "DongDo@123",
      "full_name": "CSKH Agent 01",
      "role": "cs_agent",
      "enabled": true
    },
    {
      "id": "agent_002",
      "username": "cskh02",
      "password": "DongDo@123",
      "full_name": "CSKH Agent 02",
      "role": "cs_agent",
      "enabled": true
    },
    {
      "id": "agent_003",
      "username": "cskh03",
      "password": "DongDo@123",
      "full_name": "CSKH Agent 03",
      "role": "cs_agent",
      "enabled": true
    },
    {
      "id": "agent_004",
      "username": "cskh04",
      "password": "DongDo@123",
      "full_name": "CSKH Agent 04",
      "role": "cs_agent",
      "enabled": true
    },
    {
      "id": "agent_005",
      "username": "cskh05",
      "password": "DongDo@123",
      "full_name": "CSKH Agent 05",
      "role": "cs_agent",
      "enabled": true
    },
    {
      "id": "admin_001",
      "username": "admin",
      "password": "DongDo@2026",
      "full_name": "System Admin",
      "role": "owner",
      "enabled": true
    }
  ]
}
```

### Test với Guest Users (Không cần đăng nhập)

Guest users có thể chat trực tiếp mà không cần đăng ký tài khoản:
- Chỉ cần nhập **Tên** + **SĐT** (tùy chọn)
- Hệ thống sẽ tạo session tạm thời
- Có thể test nhiều chatbox cùng lúc để đo độ chịu tải

```typescript
// Ví dụ: Đăng ký guest nhanh
POST /guest/register
{
  "display_name": "Khách hàng A",
  "phone": "0909123456"  // optional
}
```

### Chạy Batch Test

```bash
# Chạy batch test mặc định (3 agents, parallel)
npm run batch

# Chạy batch test với nhiều agents hơn
npm run batch:parallel      # 5 agents song song
npm run batch:sequential   # 5 agents lần lượt

# Chạy theo scenario
npm run batch:smoke        # smoke_test scenario
npm run batch:light        # light_load scenario  
npm run batch:medium       # medium_load scenario
npm run batch:heavy        # heavy_load scenario
npm run batch:stress       # stress_test scenario

# Tùy chỉnh
npx ts-node batch-runner.ts --agents 10 --parallel
npx ts-node batch-runner.ts --scenario heavy_load --agents 15
npx ts-node batch-runner.ts --test cs --agents 5
```

### Batch Test Scenarios

| Scenario | Guests | Messages/Guest | Agents | Mô tả |
|----------|-------|---------------|--------|-------|
| smoke_test | 1 | 2 | 1 | Test nhanh cơ bản |
| light_load | 10 | 5 | 5 | Load nhẹ |
| medium_load | 25 | 8 | 10 | Load trung bình |
| heavy_load | 50 | 10 | 15 | Load nặng |
| stress_test | 100 | 5 | 20 | Stress test |

### Batch Test Types

- **chat**: Test chat từ phía guest
- **cs**: Test hỗ trợ từ CS agent
- **mixed**: Test cả hai

### Ví dụ Output Batch Test

```
🔷🔷🔷  BATCH PARALLEL TEST RUNNER  🔷🔷🔷

[TEST INFO] Configuration:
[TEST INFO]   Agents: 5
[TEST INFO]   Scenario: medium_load
[TEST INFO]   Test Type: chat
[TEST INFO]   Mode: PARALLEL

# BATCH TEST REPORT #
==================================================
  AGENTS SUMMARY
  ──────────────────────────────────────
  Total Agents: 5
  ✅ Successful: 5
  ❌ Failed: 0
  ──────────────────────────────────────
  PERFORMANCE METRICS
  ──────────────────────────────────────
  Avg Duration: 12500ms
  Total Messages: 1000
  Successful: 980
  Failed: 20
```

### Chạy tất cả các test
```bash
npx ts-node run-all-tests.ts --all
```

### Chạy từng loại test cụ thể
```bash
# Test guest concurrency
npx ts-node run-all-tests.ts --guest

# Test message concurrency
npx ts-node run-all-tests.ts --message

# Test WebSocket
npx ts-node run-all-tests.ts --ws

# Test AI response
npx ts-node run-all-tests.ts --ai

# Test team agent support
npx ts-node run-all-tests.ts --team
```

### Chạy nhiều loại test cùng lúc
```bash
npx ts-node run-all-tests.ts --guest --ai
npx ts-node run-all-tests.ts --ws --ai --team
```

### Cấu hình số lượng test
```bash
# Test với 20 guests
npx ts-node run-all-tests.ts --guest --count 20

# Test với 50 messages
npx ts-node run-all-tests.ts --message --count 50
```

## ⚙️ Cấu Hình

Chỉnh sửa file `config.ts` để thay đổi:

```typescript
export const TEST_CONFIG = {
  // Backend API URL
  API_BASE: 'http://localhost:8080',
  
  // WebSocket URL
  WS_BASE: 'ws://localhost:8080',
  
  // Số lượng guest tối đa test đồng thời
  MAX_CONCURRENT_GUESTS: 50,
  
  // Số lượng message tối đa test đồng thời
  MAX_CONCURRENT_MESSAGES: 100,
  
  // Timeout cho mỗi request (ms)
  REQUEST_TIMEOUT: 30000,
  
  // Thời gian chờ AI response (ms)
  AI_RESPONSE_TIMEOUT: 60000,
  
  // Delay giữa các message (ms)
  MESSAGE_DELAY: 100,
};
```

Hoặc sử dụng environment variables:

```bash
export TEST_API_BASE=http://localhost:8080
export TEST_WS_BASE=ws://localhost:8080
export MAX_CONCURRENT_GUESTS=50
export MAX_CONCURRENT_MESSAGES=100
export REQUEST_TIMEOUT=30000
export AI_RESPONSE_TIMEOUT=60000
```

## 📊 Kết Quả Test Thực Tế

### Quick Limit Test (5-10 phút)
```
✅ 7/7 tests passed
- Concurrent Connections: 500 users OK
- Message Throughput: 50/50 OK
- Burst Traffic: 100/100 OK
- AI Response: 3031ms OK
- Sustained Load: 100/100 OK
- Database Load: 1ms avg OK
```

### Realistic Load Test (3 phút)
```
Users: 10
Duration: 3.1 minutes
Messages: 289 (100% success)
Errors: 0
Throughput: 1.54 msg/sec
✅ PASS - System ready
```

### Production Load Test Scenarios

| Scenario | Users | Duration | Use Case |
|----------|-------|----------|----------|
| `smoke` | 5 | 2 min | Quick validation |
| `light` | 20 | 5 min | Development testing |
| `medium` | 50 | 10 min | Staging deployment |
| `heavy` | 100 | 15 min | Pre-production |
| `stress` | 200 | 20 min | Find limits |
| `peak_hours` | 80 | 30 min | Simulate peak traffic |
| `rush_hour` | 150 | 5 min | Sudden spike |
| `full` | 200 | 60 min | Full production simulation |

### Production Readiness Checklist

Trước khi deploy, đảm bảo:

- [x] Quick limit test passed
- [x] Realistic load test passed (10 users, 3 min)
- [ ] Medium load test passed (50 users, 10 min)
- [ ] Stress test find limits known
- [ ] AI response time < 5s (P95)
- [ ] Error rate < 1%
- [ ] No memory leaks after 30 phút
- [ ] Database connections stable

### Report Output

```json
{
  "test_run": {
    "scenario": "Medium Load",
    "duration_minutes": 10
  },
  "results": {
    "connections": {
      "successful": 50,
      "avg_time_ms": 245
    },
    "messages": {
      "sent": 500,
      "failed": 2
    },
    "ai": {
      "avg_response_ms": 2500,
      "p95_response_ms": 4500
    },
    "api": {
      "avg_response_ms": 120,
      "p95_response_ms": 350
    }
  },
  "verdict": "PASS"
}
```

## 🎨 Ví Dụ Output

```
# Test Guest Concurrency
============================================================
  Guest Concurrency
============================================================
  Status: ✅ PASSED
  Duration: 5.23s
  Metrics:
    - Total: 10
    - Success: 10
    - Failed: 0
  Details:
    - Avg connection time: 245ms

# Test AI Response
============================================================
  AI Response
============================================================
  Status: ✅ PASSED
  Duration: 45.67s
  Metrics:
    - Total: 8
    - Success: 8
    - Failed: 0
  Details:
    - Avg AI response: 3500ms
    - Fallbacks: 1
```

## 🔧 Troubleshooting

### Kết nối thất bại
- Kiểm tra server đang chạy ở port 8080
- Kiểm tra CORS settings
- Kiểm tra network connection

### AI response timeout
- Tăng `AI_RESPONSE_TIMEOUT` trong config
- Kiểm tra Claude API key
- Kiểm tra Qdrant vector DB

### WebSocket lỗi
- Kiểm tra WebSocket server đang chạy
- Kiểm tra firewall settings
- Kiểm tra proxy settings

## 📈 Performance Metrics

Các metrics được thu thập:

- **Connection Time**: Thời gian kết nối (ms)
- **Send Time**: Thời gian gửi message (ms)
- **AI Response Time**: Thời gian AI phản hồi (ms)
- **Latency**: Độ trễ WebSocket (ms)
- **Throughput**: Số lượng request/giây
- **Success Rate**: Tỷ lệ thành công (%)
- **Error Rate**: Tỷ lệ lỗi (%)

## 📝 Notes

- Test được thiết kế để chạy độc lập, không ảnh hưởng đến production
- Nên chạy test trong môi trường development/staging trước
- Monitoring server resources khi chạy load test
- Test reports được lưu tự động vào folder `reports/`

## 📝 Production Load Test (Trước Khi Deploy)

### Quick Limit Test - Test Nhanh Giới Hạn (5-10 phút)

```bash
# Chạy test nhanh để check giới hạn hệ thống
npx ts-node quick-limit-test.ts
```

**Các bài test bao gồm:**
- Concurrent connections (50 users đồng thời)
- Message throughput (50 messages)
- Burst traffic (100 messages liên tục)
- AI response time
- Sustained load (20 users × 5 messages)
- Database load (50 queries)
- Find max connections

### Production Load Test - Test Toàn Diện

```bash
# Smoke test (2 phút)
npx ts-node production-load-test.ts --scenario smoke

# Light load (5 phút)
npx ts-node production-load-test.ts --scenario light

# Medium load (10 phút) - Khuyến nghị
npx ts-node production-load-test.ts --scenario medium --verbose

# Heavy load (15 phút)
npx ts-node production-load-test.ts --scenario heavy

# Peak hours simulation (30 phút)
npx ts-node production-load-test.ts --scenario peak_hours

# Rush hour spike (5 phút)
npx ts-node production-load-test.ts --scenario rush_hour

# Stress test - Tìm giới hạn (20-30 phút)
npx ts-node production-load-test.ts --scenario stress --progressive

# Full production (60 phút)
npx ts-node production-load-test.ts --scenario full --report
```

### Tùy Chỉnh Test

```bash
# Custom số users và thời gian
npx ts-node production-load-test.ts --users 100 --duration 15m

# Custom ramp-up time
npx ts-node production-load-test.ts --users 50 --ramp 60s

# Custom messages per user
npx ts-node production-load-test.ts --users 30 --messages 20

# Tạo report JSON
npx ts-node production-load-test.ts --scenario medium --report
```

### Kịch Bản Test Thực Tế

| Scenario | Users | Duration | Use Case |
|----------|-------|----------|----------|
| `smoke` | 5 | 2 min | Quick validation |
| `light` | 20 | 5 min | Development testing |
| `medium` | 50 | 10 min | Staging deployment |
| `heavy` | 100 | 15 min | Pre-production |
| `stress` | 200 | 20 min | Find limits |
| `peak_hours` | 80 | 30 min | Simulate peak traffic |
| `rush_hour` | 150 | 5 min | Sudden spike |
| `full` | 200 | 60 min | Full production simulation |

### Production Readiness Checklist

Trước khi deploy, đảm bảo:

- [ ] Smoke test passed (2 phút)
- [ ] Medium load test passed (10-15 phút)
- [ ] Stress test find limits known
- [ ] AI response time < 5s (P95)
- [ ] Error rate < 1%
- [ ] No memory leaks after 30 phút
- [ ] Database connections stable

### Report Output

```json
{
  "test_run": {
    "scenario": "Medium Load",
    "duration_minutes": 10
  },
  "results": {
    "connections": {
      "successful": 50,
      "avg_time_ms": 245
    },
    "messages": {
      "sent": 500,
      "failed": 2
    },
    "ai": {
      "avg_response_ms": 2500,
      "p95_response_ms": 4500
    },
    "api": {
      "avg_response_ms": 120,
      "p95_response_ms": 350
    }
  },
  "verdict": "PASS"
}
```

---

## 📧 Liên Hệ

Nếu có câu hỏi hoặc vấn đề, vui lòng liên hệ đội ngũ phát triển.

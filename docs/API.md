# 📡 API Reference — REST + WebSocket

> **Phiên bản:** v2.0
> **Base URL (dev):** `http://localhost:8080`
> **WebSocket URL:** `ws://localhost:8080/ws?session_id=...&user_id=...&role=...`
> **Cập nhật lần cuối:** Sep 2026

## Mục lục

1. [Authentication](#1-authentication)
2. [REST API](#2-rest-api)
   - [Auth endpoints](#21-auth-endpoints)
   - [Guest & Chat](#22-guest--chat)
   - [Cases (CS Inbox)](#23-cases-cs-inbox)
   - [Customers](#24-customers)
   - [Voice Calls](#25-voice-calls)
   - [Continuous Learning](#26-continuous-learning)
   - [Knowledge Base](#27-knowledge-base)
   - [Analytics & Reports](#28-analytics--reports)
   - [System Config](#29-system-config)
   - [Users & Permissions](#210-users--permissions)
   - [System Errors](#211-system-errors)
3. [WebSocket events](#3-websocket-events)
4. [Error format](#4-error-format)
5. [Cross-references](#5-cross-references)

---

## 1. Authentication

### 1.1 Bearer token (CSKH / Admin)

```http
Authorization: Bearer <session_token>
```

Token nhận từ `POST /auth/login`, lưu trong DB `sessions` table với TTL (`JWT_EXPIRY_HOURS`, default 168h = 7 days).

### 1.2 Alternative headers

```http
X-Auth-Token: <session_token>
```

(Support cho legacy clients.)

### 1.3 Guest token

Guest (chat user) nhận token từ `POST /guest/register`. Token kèm theo response body, không qua header.

### 1.4 RBAC roles

| Role | Quyền |
|---|---|
| `owner` | Toàn quyền (chỉ owner mới được grant owner) |
| `admin` | Quản lý users, config, knowledge, learning |
| `leader` | CSKH + reports |
| `cskh` | Chat, reply case, voice |

---

## 2. REST API

### 2.1 Auth endpoints

#### `POST /auth/login` — Đăng nhập CSKH/Admin

**Auth:** Public

**Request body:**
```json
{
  "username": "admin",
  "password": "DongDo@2026"
}
```

**Response 200:**
```json
{
  "token": "a1b2c3d4e5f6...",
  "username": "admin",
  "full_name": "Quản trị viên Đông Đô",
  "role": "owner"
}
```

**Errors:**
- `400` — Thiếu field
- `401` — Sai username/password

#### `GET /auth/me` — Lấy thông tin user hiện tại

**Auth:** Bearer token

**Response 200:**
```json
{
  "username": "admin",
  "full_name": "Quản trị viên Đông Đô",
  "role": "owner"
}
```

#### `POST /auth/logout` — Đăng xuất

**Auth:** Bearer token

**Response 200:**
```json
{
  "message": "Đã đăng xuất thành công."
}
```

---

### 2.2 Guest & Chat

#### `POST /guest/register` — Đăng ký guest

**Auth:** Public

**Request body:**
```json
{
  "display_name": "Nguyễn Văn A",
  "phone": "0987654321"
}
```

**Response 200:**
```json
{
  "guest_id": "550e8400-e29b-41d4-a716-446655440000",
  "display_name": "Nguyễn Văn A",
  "phone": "0987654321",
  "session_id": "session-abc-1716438234087",
  "token": "guest-token-xyz"
}
```

#### `POST /chat` — Gửi tin nhắn chat

**Auth:** Public (guest token optional)

**Request body:**
```json
{
  "session_id": "session-abc-1716438234087",
  "customer_name": "Nguyễn Văn A",
  "message": "Hàng hóa phái sinh là gì?",
  "client_msg_id": "uuid-v4-idempotent"
}
```

**Response 200:**
```json
{
  "session_id": "session-abc-1716438234087",
  "message_id": 12345,
  "status": "RECEIVED"
}
```

> AI reply sẽ push qua WebSocket event `type:message` (không phải REST response).

#### `GET /history/:session_id` — Lấy lịch sử chat

**Auth:** Public

**Response 200:**
```json
{
  "session_id": "session-abc",
  "status": "AI_ACTIVE",
  "assigned_cs": "",
  "messages": [
    {
      "id": 12345,
      "session_id": "session-abc",
      "sender_type": "guest",
      "sender_id": "Nguyễn Văn A",
      "content": "Hàng hóa phái sinh là gì?",
      "created_at": "2024-09-04T15:30:00Z"
    },
    {
      "id": 12346,
      "session_id": "session-abc",
      "sender_type": "ai",
      "sender_id": "Đông Đô AI",
      "content": "Hàng hóa phái sinh là...",
      "created_at": "2024-09-04T15:30:02Z"
    }
  ]
}
```

#### `POST /api/chat/typing` — Gửi typing indicator

**Auth:** Public

**Request body:**
```json
{
  "session_id": "session-abc"
}
```

**Response 200:**
```json
{ "success": true }
```

---

### 2.3 Cases (CS Inbox)

#### `GET /api/admin/cases` — Danh sách cases

**Auth:** Bearer (CSKH+)

**Query params:**
- `status` — `AI_ACTIVE` | `NEEDS_HUMAN_CS` | `HUMAN_CS_ACTIVE` | `RESOLVED`
- `page` — default 1
- `limit` — default 10, max 100
- `search` — keyword search

**Response 200:**
```json
{
  "cases": [
    {
      "id": 42,
      "session_id": "session-abc",
      "customer_name": "Nguyễn Văn A",
      "customer_phone": "0987654321",
      "status": "AI_ACTIVE",
      "assigned_cs": "",
      "last_message": "Hàng hóa phái sinh là gì?",
      "created_at": "2024-09-04T15:30:00Z",
      "updated_at": "2024-09-04T15:30:00Z"
    }
  ],
  "total": 12,
  "page": 1,
  "limit": 10,
  "total_pages": 2
}
```

#### `POST /api/admin/cases/:session_id/take` — Tiếp nhận case

**Auth:** Bearer (CSKH+)

**Response 200:**
```json
{
  "success": true,
  "message": "Đã tiếp nhận case thành công"
}
```

#### `POST /api/admin/cases/:session_id/reply` — Reply trong case

**Auth:** Bearer (CSKH+)

**Request body:**
```json
{
  "message": "Dạ anh/chị, hàng hóa phái sinh là..."
}
```

**Response 200:**
```json
{
  "success": true,
  "message": "Đã gửi tin nhắn thành công"
}
```

#### `POST /api/admin/cases/:session_id/resolve` — Đóng case

**Auth:** Bearer (CSKH+)

**Request body:**
```json
{
  "resolution_note": "Đã giải đáp về hàng hóa phái sinh",
  "extract_pairs": [
    {
      "question": "Hàng hóa phái sinh là gì?",
      "answer": "Hàng hóa phái sinh là các công cụ tài chính..."
    }
  ]
}
```

**Response 200:**
```json
{
  "success": true,
  "auto_learned": true,
  "learned_count": 1,
  "message": "Đã đóng case thành công (1 mẩu tri thức xử lý)."
}
```

#### `PUT /api/admin/cases/:session_id/customer` — Update thông tin khách

**Auth:** Bearer (CSKH+)

**Request body:**
```json
{
  "customer_name": "Nguyễn Văn B",
  "customer_phone": "0987654321"
}
```

**Response 200:**
```json
{
  "success": true,
  "message": "Đã cập nhật thông tin khách hàng thành công"
}
```

#### `DELETE /api/admin/cases/:session_id` — Xóa case

**Auth:** Bearer (Admin+)

#### `POST /api/admin/cases/clear-all` — Xóa toàn bộ cases

**Auth:** Bearer (Admin+)

---

### 2.4 Customers

#### `GET /api/admin/customers` — Danh sách khách hàng

**Auth:** Bearer (CSKH+)

**Query:** `page`, `limit`, `search`

#### `PUT /api/admin/customers/:guest_id` — Update thông tin khách

**Auth:** Bearer (CSKH+)

#### `DELETE /api/admin/customers/:guest_id` — Xóa khách hàng

**Auth:** Bearer (Admin+)

---

### 2.5 Voice Calls

#### `POST /api/voice/initiate` — Khởi tạo cuộc gọi

**Auth:** Public

**Request body:**
```json
{
  "session_id": "session-abc",
  "caller_type": "guest",
  "caller_id": "Nguyễn Văn A",
  "callee_type": "cskh",
  "callee_id": ""
}
```

**Response 200:**
```json
{
  "id": 42,
  "session_id": "session-abc",
  "caller_type": "guest",
  "caller_id": "Nguyễn Văn A",
  "callee_type": "cskh",
  "callee_id": "",
  "status": "RINGING",
  "duration_seconds": 0,
  "created_at": "2024-09-04T15:30:00Z"
}
```

> Backend publish `call_ring` event lên WS cho admin_inbox. AMI gọi `Action: Originate` tới Asterisk.

#### `POST /api/voice/accept` — CSKH nhận cuộc gọi

**Auth:** Bearer (CSKH+)

**Request body:**
```json
{
  "call_id": 42,
  "session_id": "session-abc"
}
```

**Response 200:**
```json
{
  "id": 42,
  "status": "ACTIVE",
  "duration_seconds": 0
}
```

> Backend `Action: Originate` SIP endpoint của agent → Bridge vào channel đang RINGING.

#### `POST /api/voice/end` — Kết thúc cuộc gọi

**Auth:** Public

**Request body:**
```json
{
  "call_id": 42,
  "session_id": "session-abc",
  "duration_seconds": 123,
  "recording_url": "/static/recordings/call_42_xyz.webm"
}
```

**Response 200:**
```json
{
  "success": true,
  "message": "Cuộc gọi đã kết thúc"
}
```

#### `POST /api/voice/decline` — Guest từ chối

**Auth:** Public

**Request body:**
```json
{
  "session_id": "session-abc"
}
```

**Response 200:**
```json
{ "success": true }
```

> Backend publish `call_end` event để CSKH thấy guest từ chối.

#### `POST /api/voice/missed` — Đánh dấu gọi nhỡ

**Auth:** Bearer (CSKH+)

**Request body:**
```json
{
  "call_id": 42,
  "session_id": "session-abc"
}
```

**Response 200:**
```json
{
  "success": true,
  "message": "Cuộc gọi đã được đánh dấu là gọi nhỡ"
}
```

#### `POST /api/voice/upload-recording` — Upload ghi âm

**Auth:** Public (multipart/form-data)

**Form fields:**
- `audio` — file (audio/webm hoặc audio/wav)
- `session_id` — string
- `call_id` — int64 (optional)
- `duration_seconds` — int (optional)
- `transcript` — string (optional, từ browser STT)

**Response 200:**
```json
{
  "success": true,
  "recording_url": "/static/recordings/call_1716438234087_session-abc_xyz.webm",
  "message": "Ghi âm cuộc gọi đã được lưu và tự động bóc tách nội dung đưa vào hàng chờ Học Tri Thức Mới"
}
```

#### `GET /api/voice/status/:call_id` — Trạng thái cuộc gọi

**Auth:** Public

**Response 200:** VoiceCall object

#### `GET /static/recordings/:filename` — Play recording

**Auth:** Public

Returns audio file (Content-Type: audio/webm).

#### `GET /api/admin/voice/calls` — Lịch sử cuộc gọi

**Auth:** Bearer (CSKH+)

**Query:** `session_id`, `page`, `limit`

**Response 200:**
```json
{
  "calls": [
    {
      "id": 42,
      "session_id": "session-abc",
      "caller_type": "guest",
      "caller_id": "Nguyễn Văn A",
      "callee_type": "cskh",
      "callee_id": "",
      "status": "ENDED",
      "duration_seconds": 123,
      "recording_url": "/static/recordings/call_42.webm",
      "transcript": "...",
      "created_at": "2024-09-04T15:30:00Z",
      "ended_at": "2024-09-04T15:32:03Z"
    }
  ],
  "total": 50,
  "page": 1,
  "limit": 10,
  "total_pages": 5
}
```

#### `DELETE /api/admin/voice/calls/:call_id` — Xóa lịch sử cuộc gọi

**Auth:** Bearer (Admin+)

---

### 2.6 Continuous Learning

#### `GET /api/admin/learning/pending` — Queue tri thức chờ duyệt

**Auth:** Bearer (CSKH+)

**Response 200:**
```json
{
  "pending_items": [
    {
      "id": 100,
      "session_id": "session-abc",
      "question": "Hàng hóa phái sinh là gì?",
      "answer": "...",
      "status": "PENDING",
      "created_by": "auto_voice_call",
      "approved_by": "",
      "created_at": "2024-09-04T15:32:00Z"
    }
  ],
  "total": 5,
  "page": 1,
  "limit": 10,
  "total_pages": 1
}
```

#### `PUT /api/admin/learning/:item_id` — Sửa Q&A

**Auth:** Bearer (Admin+)

**Request:**
```json
{
  "question": "...",
  "answer": "..."
}
```

#### `POST /api/admin/learning/approve/:item_id` — Duyệt + nạp vào Qdrant

**Auth:** Bearer (Admin+)

**Request:**
```json
{
  "question": "Hàng hóa phái sinh là gì?",
  "answer": "Hàng hóa phái sinh là..."
}
```

> Body optional — nếu muốn edit trước khi approve.

#### `POST /api/admin/learning/reject/:item_id` — Từ chối

**Auth:** Bearer (Admin+)

#### `GET /api/admin/learning/settings` — Auto-learning flag

**Auth:** Bearer (CSKH+)

#### `POST /api/admin/learning/settings` — Bật/tắt auto-learning

**Auth:** Bearer (Admin+)

**Request:**
```json
{
  "auto_learning_enabled": true
}
```

#### `POST /api/admin/learning/reset` — Reset toàn bộ tri thức đã học

**Auth:** Bearer (Admin+)

---

### 2.7 Knowledge Base

#### `GET /api/admin/knowledge` — Overview

**Auth:** Bearer (CSKH+)

**Response:**
```json
{
  "total_chunks": 1500,
  "total_documents": 12,
  "documents": [
    {
      "filename": "huong_dan_ddp_invest.docx",
      "size_kb": "234.5"
    }
  ]
}
```

#### `POST /api/admin/knowledge/upload` — Upload .docx

**Auth:** Bearer (Admin+)

**Form:** `file` (multipart)

> Sau khi upload, cần chạy `ingest` để chunk + embed vào Qdrant:
> ```bash
> docker compose exec server /app/ingest
> ```

#### `DELETE /api/admin/knowledge/document?filename=...` — Xóa document

**Auth:** Bearer (Admin+)

---

### 2.8 Analytics & Reports

#### `GET /api/admin/analytics` — Dashboard stats

**Auth:** Bearer (CSKH+)

**Response:**
```json
{
  "total_cases": 250,
  "total_sessions": 180,
  "ai_active_cases": 30,
  "needs_human_cases": 15,
  "active_human_cases": 5,
  "resolved_cases": 200,
  "ai_self_service_rate": 0.72,
  "total_learned_qa": 120,
  "pending_learn_count": 8
}
```

#### `GET /api/admin/partner/reports/overview` — Tổng quan

**Auth:** Bearer (CSKH+)

Query: `startDate`, `endDate` (ISO 8601)

#### `GET /api/admin/partner/reports/ai-performance` — AI Performance

#### `GET /api/admin/partner/reports/staff-performance` — Staff Performance

#### `GET /api/admin/partner/reports/cx` — Customer Experience (CX)

#### `POST /api/admin/partner/reports/csat` — Submit CSAT

**Auth:** Public

**Request:**
```json
{
  "session_id": "session-abc",
  "rating": 5,
  "comment": "..."
}
```

#### `GET /api/admin/partner/reports/operational` — Operational

#### `GET /api/admin/partner/reports/issue-analysis` — Issue Analysis

#### `GET /api/admin/partner/reports/ai-learning` — AI Learning Stats

#### `GET /api/admin/partner/dashboard` — Partner dashboard

---

### 2.9 System Config

#### `GET /api/admin/config` — System config hiện tại

**Auth:** Bearer (Admin+)

**Response:**
```json
{
  "system_prompt": "Bạn là chuyên viên CSKH...",
  "llm_model": "claude-haiku-4-5-20251001",
  "temperature": 0.1
}
```

#### `POST /api/admin/config` — Update system prompt + LLM model

**Auth:** Bearer (Admin+)

**Request:**
```json
{
  "system_prompt": "Bạn là...",
  "llm_model": "claude-haiku-4-5-20251001",
  "temperature": 0.1
}
```

---

### 2.10 Users & Permissions

#### `GET /api/admin/users` — Danh sách users

**Auth:** Bearer (Admin+)

#### `POST /api/admin/users` — Tạo user

**Auth:** Bearer (Admin+)

**Request:**
```json
{
  "email": "cskh06@dongdo.vn",
  "full_name": "Chuyên viên CSKH 06",
  "role": "cskh",
  "password": "default-pass-123"
}
```

#### `PUT /api/admin/users/:username` — Update user

**Auth:** Bearer (Admin+)

#### `DELETE /api/admin/users/:username` — Xóa user

**Auth:** Bearer (Owner only)

#### `GET /api/admin/partner/config/permissions` — Role permissions matrix

**Auth:** Bearer (Admin+)

#### `POST /api/admin/partner/config/permissions` — Upsert permission

**Auth:** Bearer (Admin+)

#### `GET /api/admin/partner/config/audit-logs` — Audit log

**Auth:** Bearer (Owner only)

#### `GET /api/admin/partner/config/templates` — Quick templates

#### `POST /api/admin/partner/config/templates` — Create template

#### `PUT /api/admin/partner/config/templates/:id` — Update template

#### `DELETE /api/admin/partner/config/templates/:id` — Delete template

---

### 2.11 System Errors

#### `GET /api/admin/system-errors` — Danh sách errors

**Auth:** Bearer (Admin+)

#### `POST /api/admin/system-errors` — Report error

**Auth:** Bearer (Admin+)

**Request:** `SystemErrorRecord` (id, source, severity, message, ...)

#### `PUT /api/admin/system-errors/:id/handled` — Mark handled

**Auth:** Bearer (Admin+)

---

### 2.12 Health

#### `GET /health` — Health check

**Auth:** Public

**Response:**
```json
{
  "status": "healthy",
  "engine": "Golang Clean Architecture (Gin + SQLC)"
}
```

---

## 3. WebSocket events

### 3.1 Endpoint

```
ws://<host>:8080/ws?session_id=<session_id>&user_id=<user_id>&role=<guest|cskh|admin>
```

Ví dụ:
```
ws://localhost:8080/ws?session_id=session-abc&user_id=Nguyễn%20Văn%20A&role=guest
```

### 3.2 Frame format

```json
{
  "type": "message",
  "session_id": "session-abc",
  "payload": { ... },
  "sender_id": "ai",
  "timestamp": "2024-09-04T15:30:02Z"
}
```

### 3.3 Event types

| `type` | Payload | Description |
|---|---|---|
| `message` | `{ message: Message, sources?, is_fallback? }` | New chat message |
| `typing` | `{ sender_id, typing: true\|false }` | Typing indicator |
| `stop_typing` | `{ sender_id }` | Stop typing |
| `unread` | `{ count }` | Unread counter update |
| `case_update` | `{ session_id, status, ... }` | Case state changed |
| `learning_update` | `{ item_id, status }` | Learning queue update |
| `call_offer` | `{ type: "offer", sdp: "..." }` | WebRTC offer (fallback) |
| `call_answer` | `{ type: "answer", sdp: "..." }` | WebRTC answer |
| `call_ice` | `{ candidate, sdpMid, sdpMLineIndex }` | ICE candidate |
| `call_ring` | `{ call_id, caller_id, caller_name, ... }` | Incoming call (Asterisk-mediated) |
| `call_status` | `{ status: "connected"\|"ended", ... }` | Call status update |
| `call_end` | `{ reason }` | Call ended |
| `ai_status` | `{ ai_active: bool }` | AI service health |

### 3.4 Channel semantics

Backend phân biệt hai loại channel:

- **`session_id`** — chỉ client thuộc session đó nhận events
- **`admin_inbox`** — admin clients subscribe để xem tất cả events

Khi backend publish WS event, nó broadcast đến **CẢ HAI**:
- `session_id` channel
- `admin_inbox` channel (nếu là `message`, `case_update`, `call_*`, ...)

### 3.5 Client → Server frames

Client chỉ gửi ping:

```json
{ "type": "ping" }
```

Server pong (echo) hoặc timeout close sau 30s không có ping.

### 3.6 Ví dụ: lắng nghe call_ring (admin)

```javascript
ws.on('call_ring', (event) => {
  console.log('Incoming call:', event.payload.call_id);
  showIncomingCallBanner(event.payload);
});
```

### 3.7 Ví dụ: nhận AI message

```javascript
ws.on('message', (event) => {
  const msg = event.payload.message;
  console.log('AI says:', msg.content);
  appendChatMessage(msg);
});
```

---

## 4. Error format

Mọi error response đều theo format:

```json
{
  "detail": "Mô tả lỗi (tiếng Việt)",
  "error": "..."
}
```

Status codes:

| Code | Ý nghĩa |
|---|---|
| 200 | OK |
| 400 | Bad request — dữ liệu không hợp lệ |
| 401 | Unauthorized — thiếu/sai token |
| 403 | Forbidden — không đủ quyền |
| 404 | Not found |
| 500 | Internal server error |

Ví dụ error:

```json
{
  "detail": "Vui lòng đăng nhập để sử dụng tính năng này."
}
```

---

## 5. Cross-references

- [ARCHITECTURE.md §4 Backend](./ARCHITECTURE.md#4-backend-go) — handler structure
- [CONFIGURATION.md](./CONFIGURATION.md) — env config
- [TELEPHONY.md](./TELEPHONY.md) — voice call flow
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) — debugging

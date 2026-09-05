# 📝 CHANGELOG

> Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
> Versioning: [Semantic Versioning](https://semver.org/)

---

## [2.0.0] - 2026-09-04 — Asterisk Migration Release

> **Đây là một BREAKING RELEASE.** Migration từ WebRTC P2P sang Asterisk PBX.

### 🔥 Breaking Changes

- **WebRTCManager giữ legacy API cho WebRTC fallback** — production dùng Asterisk.
  - File `web/src/lib/webrtc.ts` vẫn export `WebRTCVoiceManager` nhưng code path chính là REST + AMI.
  - Frontend `voiceApi.acceptCall` giờ gọi `/api/voice/accept` (backend sẽ bridge qua Asterisk thay vì SDP exchange).
- **Endpoint mới `POST /api/voice/accept`** — backend `Action: Originate` SIP endpoint của agent thay vì forward WebRTC answer.
- **Voice recording flow** thay đổi:
  - Backend lưu recording qua `voice_calls.recording_url` (đường dẫn local) hoặc qua Asterisk MixMonitor (S3 sync).
  - Frontend `MediaRecorder` chỉ dùng cho WebRTC fallback.

### ✨ New Features

#### Backend (Go)

- ✅ **Asterisk AMI client** (`internal/infra/ami`) — giao tiếp với Asterisk qua TCP port 5038
  - Actions: `Originate`, `Hangup`, `Redirect`, `Hold`, `MuteAudio`, `Bridge`
  - Events: `Dial`, `BridgeEnter`, `BridgeLeave`, `Hangup`, `Hold`, `QueueMember*`
- ✅ **VoiceUseCase** (`internal/usecase/voice_usecase.go`) — orchestrate AMI calls
- ✅ **New endpoints:**
  - `POST /api/voice/accept` — CSKH nhận cuộc gọi (AMI Originate tới PJSIP/<agent>)
  - `GET /api/voice/status/:call_id` — poll status fallback
- ✅ **Recording sync** — `MixMonitor` output mounted vào `./recordings/`

#### Docker / DevOps

- ✅ **Asterisk 20 container** (`docker/asterisk/Dockerfile`) — multi-stage build, `asterisk:20-current-bookworm` base
- ✅ **Asterisk config** (`docker/asterisk/etc/asterisk/`):
  - `pjsip.conf` — endpoints (agent 1001-1010, guest, trunk-pstn, ARI)
  - `extensions.conf` — dialplan (5 contexts)
  - `queues.conf` — `dongdo-queue`, `dongdo-supervisor`, `dongdo-sales`
  - `manager.conf` — AMI users (`dongdo`, `monitor`)
  - `ari.conf` — ARI stasis app `dongdo-ivr`
  - `rtp.conf`, `cdr.conf`, `cel.conf`, `features.conf`, ...
- ✅ **Entrypoint** (`docker/asterisk/entrypoint.sh`):
  - Wait-for postgres/redis
  - Template secrets từ env vào config files
  - Validate dialplan trước khi start
- ✅ **docker-compose.yml** — thêm service `asterisk` với:
  - Ports: 5060/udp (SIP), 5061/tcp (SIPS/TLS), 5038 (AMI), 8088 (ARI), 10000-20000/udp (RTP)
  - Volumes: `./recordings:/var/spool/asterisk/monitor`

#### Documentation

- ✅ Toàn bộ docs rewrite (`docs/*.md`, `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `LICENSE`)
- ✅ Mermaid diagrams (`docs/diagrams/*.mmd`)
- ✅ CI workflow (`.github/workflows/ci.yml`)
- ✅ Smoke test script (`scripts/smoke-test.sh`)

### 🛠 Improvements

- **WebSocket Hub** (`internal/delivery/ws/hub.go`) — fix bug drop client khi buffer đầy; giờ chỉ drop event đó, giữ client online
- **DBWorker** — tăng `DB_BATCH_SIZE` mặc định từ 10 → 50
- **Frontend cleanup** — refactor `web/src/lib/api/index.ts`, `voice.ts`, `webrtc.ts`, `ConfirmDialog.tsx`, `AdminSidebar.tsx`

### 🐛 Bug Fixes

- WS hub: client mất kết nối khi broadcast buffer đầy → fix giữ kết nối
- DB worker: race condition khi flush batch → lock mutex đúng
- Migration: race khi boot nhiều replicas → thêm advisory lock

### 📦 Migration Guide (cho deployments hiện tại)

> Nếu bạn đang chạy v1.x (WebRTC P2P) và upgrade lên v2.0:

**Bước 1: Backup**

```bash
# Postgres
docker compose exec postgres pg_dump -U postgres -Fc dongdo_cs > backup_v1.dump

# Recordings (nếu có trên S3 hoặc local)
```

**Bước 2: Pull v2.0**

```bash
git fetch origin
git checkout v2.0.0
git pull
```

**Bước 3: Cập nhật .env**

Thêm:
```bash
ASTERISK_AMI_PASS=<new-password>
ASTERISK_GUEST_PASS=<new-password>
ASTERISK_AGENT_PASS_PREFIX=<new-prefix>
```

**Bước 4: Bring up stack mới**

```bash
make down
docker volume rm dongdo_cs_v2_pgdata    # CHỈ nếu muốn reset DB (cân nhập lại data từ backup)
make up
```

**Bước 5: Verify**

```bash
# Health
curl http://localhost:8080/health

# Asterisk
docker compose exec asterisk asterisk -rx "pjsip show endpoints"

# Login admin
# http://localhost:8080/admin/login → admin / DongDo@2026
```

**Bước 6: Cấu hình SIP agents**

Agent SIP hardphone / softphone:
```
Server:   <host>
Username: 1001
Password: <ASTERISK_AGENT_PASS_PREFIX>1001
Realm:    dongdo.local
```

### ⚠️ Known Issues

- Multi-instance WebSocket Hub chưa support (single-node sticky session cần thiết)
- ARI stasis app `dongdo-ivr` chưa implemented (chỉ config placeholder)
- Recordings chưa auto-convert WAV → MP3

### 🙏 Credits

Asterisk integration được thiết kế dựa trên:
- [Asterisk 20 documentation](https://docs.asterisk.org/)
- [PJSIP Configuration](https://wiki.asterisk.org/wiki/display/AST/Configuring+res_pjsip)
- [AMI Reference](https://wiki.asterisk.org/wiki/display/AST/Asterisk+Manager+Interface)

---

## [1.x.x] - Legacy WebRTC Release

> Pre-Asterisk. Sử dụng WebRTC P2P qua WebSocket signaling.
> Xem git history `git log --oneline --before="2026-09-04"` cho details.

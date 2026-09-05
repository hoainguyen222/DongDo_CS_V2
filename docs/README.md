# 📚 Tài liệu Đông Đô CS V2

> Documentation index cho hệ thống CSKH Đông Đô Partners v2.0.

## Bắt đầu từ đâu?

| Bạn là | Đọc gì đầu tiên |
|---|---|
| 🆕 Dev mới onboard | [README.md](../README.md) → [docs/DEVELOPMENT.md](./DEVELOPMENT.md) |
| 🏗️ Kiến trúc sư | [docs/ARCHITECTURE.md](./ARCHITECTURE.md) |
| 📞 VoIP engineer | [docs/TELEPHONY.md](./TELEPHONY.md) |
| 🚀 DevOps / SRE | [docs/DEPLOYMENT.md](./DEPLOYMENT.md) + [docs/CONFIGURATION.md](./CONFIGURATION.md) |
| 💻 Backend dev | [docs/ARCHITECTURE.md](./ARCHITECTURE.md) + [docs/API.md](./API.md) + [docs/DEVELOPMENT.md](./DEVELOPMENT.md) |
| 🎨 Frontend dev | [docs/API.md](./API.md) + [docs/DEVELOPMENT.md](./DEVELOPMENT.md) (frontend section) |
| 🐛 Bug hunter | [docs/TROUBLESHOOTING.md](./TROUBLESHOOTING.md) |
| 🔧 Tuner | [docs/CONFIGURATION.md](./CONFIGURATION.md) |

## Toàn bộ tài liệu

### Core
- [README.md](../README.md) — Project overview, quick start
- [CHANGELOG.md](../CHANGELOG.md) — Version history (v2.0: Asterisk migration)
- [CONTRIBUTING.md](../CONTRIBUTING.md) — Contribution guide
- [LICENSE](../LICENSE) — MIT

### Architecture & Reference
- [ARCHITECTURE.md](./ARCHITECTURE.md) — Layers, data flow, ER diagram, sequence diagrams
- [API.md](./API.md) — REST API + WebSocket events
- [CONFIGURATION.md](./CONFIGURATION.md) — Env vars + Asterisk config

### Operations
- [DEPLOYMENT.md](./DEPLOYMENT.md) — Production deployment, monitoring, backup
- [DEVELOPMENT.md](./DEVELOPMENT.md) — Local dev workflow, hot reload, testing
- [TELEPHONY.md](./TELEPHONY.md) — Asterisk integration, AMI, dialplan
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) — Common issues & fixes

### Diagrams
- [diagrams/architecture.mmd](./diagrams/architecture.mmd) — System architecture
- [diagrams/sequence-call.mmd](./diagrams/sequence-call.mmd) — Voice call flow
- [diagrams/deployment.mmd](./diagrams/deployment.mmd) — Production topology

### Scripts
- [../scripts/smoke-test.sh](../scripts/smoke-test.sh) — End-to-end smoke test

### CI
- [../.github/workflows/ci.yml](../.github/workflows/ci.yml) — GitHub Actions pipeline

## Conventions

- **Tiếng Việt** cho phần business logic (phrased tự nhiên như đang nói chuyện)
- **Tiếng Anh** cho technical terms (REST, WebSocket, AMI, ...)
- Mọi file `.md` đều có Table of Contents ở đầu
- Cross-references dùng relative paths
- Code blocks có syntax highlighting (` ```go `, ` ```bash `, ` ```yaml `)
- Mermaid diagrams cho visual flow

## Cập nhật docs

Khi thay đổi code, cần update docs tương ứng:

| Code change | Doc update |
|---|---|
| New env var | [CONFIGURATION.md §1](./CONFIGURATION.md#1-environment-variables) |
| New REST endpoint | [API.md §2](./API.md#2-rest-api) |
| New WS event | [API.md §3](./API.md#3-websocket-events) |
| New worker / stream | [ARCHITECTURE.md §10](./ARCHITECTURE.md#10-worker-responsibilities) |
| Schema change (new table) | [ARCHITECTURE.md §8](./ARCHITECTURE.md#8-database-schema-overview) |
| Asterisk config change | [TELEPHONY.md](./TELEPHONY.md) |
| Migration | [DEVELOPMENT.md §7](./DEVELOPMENT.md#7-workflow-goose-migrations) + CHANGELOG |
| Docker change | [DEPLOYMENT.md](./DEPLOYMENT.md) (§3 Cài đặt) |

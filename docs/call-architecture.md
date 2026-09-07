# Call System Architecture (v2)

> Customer-Support voice calls: customer browser ↔ Asterisk (SIP/WSS+WebRTC) ↔ agent browser. Go service is the **business-logic brain** and the **only authoritative source for call state**.

---

## 1. High-Level Architecture

```
                          ┌────────────────────────────────────────┐
                          │            Customer Browser            │
                          │  (SIP.js / JsSIP over WSS, WebRTC)    │
                          └──────────────────┬─────────────────────┘
                                             │ SIP/WSS
                                             ▼
┌────────────────────┐              ┌──────────────────────┐
│   Agent Browser    │◄─SIP/WSS────►│       Asterisk       │
│ (SIP.js, headset)  │              │   (PJSIP/WSS/RTP)    │
└────────────────────┘              └──────────┬───────────┘
                                                ▲
                                  ARI REST +    │
                                  ARI WebSocket │
                                                │
                                  ┌─────────────┴────────────────────┐
                                  │         Go Call Service           │
                                  │                                    │
                                  │  ┌──────────────────────────────┐  │
                                  │  │  HTTP API (Gin)              │  │
                                  │  │   /api/calls                 │  │
                                  │  │   /api/calls/{id}/accept     │  │
                                  │  │   /api/calls/{id}/reject     │  │
                                  │  │   /api/calls/{id}/hangup     │  │
                                  │  └────────────┬─────────────────┘  │
                                  │               │                    │
                                  │  ┌────────────▼─────────────────┐  │
                                  │  │  CallUseCase (state machine, │  │
                                  │  │  idempotency, business logic)│  │
                                  │  └─┬────────────────────────┬───┘  │
                                  │    │                        │      │
                                  │  ┌─▼──────────────┐   ┌─────▼─────┐ │
                                  │  │ AsteriskGateway│   │  Redis    │ │
                                  │  │   (ARI HTTP +  │   │ • Queue   │ │
                                  │  │    ARI WS)     │   │ • Agent   │ │
                                  │  └────┬───────────┘   │   state   │ │
                                  │       │               │ • Idem-   │ │
                                  │  ┌────▼──────────┐    │   key     │ │
                                  │  │  ARI consumer │    │ • Call    │ │
                                  │  │  (long-run)   │    │   state   │ │
                                  │  └───────────────┘    └─────┬─────┘ │
                                  │                             │       │
                                  │  ┌──────────────────────────▼───┐   │
                                  │  │  PostgreSQL (call log +      │   │
                                  │  │  call_events audit, append-  │   │
                                  │  │  only)                       │   │
                                  │  └──────────────────────────────┘   │
                                  │                                    │
                                  │  ┌──────────────────────────────┐  │
                                  │  │  WebSocket Hub (realtime UI  │  │
                                  │  │  only — never source of      │  │
                                  │  │  truth)                      │  │
                                  │  └──────────────────────────────┘  │
                                  └─────────────┬──────────────────────┘
                                                │
                                                ▼
                                          Agent Browser
```

### Trust Boundaries

| Component                          | Source of truth for       | Must NOT do                                |
| ---------------------------------- | ------------------------- | ------------------------------------------ |
| Go Call Service                    | Call state, Agent state   | Process RTP, handle SDP, do WebRTC         |
| Asterisk                           | Channel/media state       | Store business data, own call logic        |
| PostgreSQL                         | Call history & audit log  | Hold ephemeral realtime state              |
| Redis                              | Ephemeral realtime        | Replace Postgres for audit trail           |
| Customer / Agent Browser           | Render UI                 | Decide call state (must read it from Go)   |

---

## 2. Call State Machine

```
              ┌─────────┐
              │ CREATED │  (in-memory + PG row inserted)
              └────┬────┘
                   │ enqueue + find agent
                   ▼
              ┌──────────┐
   ┌─────────►│ WAITING  │  (in Redis queue)
   │          └────┬─────┘
   │               │ agent reserved
   │               ▼
   │        ┌─────────────┐
   │   ┌───►│WAITING_AGENT│  (WebSocket: incoming_call pushed to agent)
   │   │    └─┬───────────┘
   │   │      │ agent accept
   │   │      ▼
   │   │   ┌───────────┐
   │   │   │ CONNECTING│  (ARI: originate customer + agent channels)
   │   │   └─┬─────────┘
   │   │     │ ARI: StasisStart + Channel answered
   │   │     ▼
   │   │   ┌─────────┐
   │   │   │ RINGING │
   │   │   └─┬───────┘
   │   │     │ ARI: Channel answered (agent picked up)
   │   │     ▼
   │   │   ┌────────────┐
   │   │   │ IN_PROGRESS│  (Asterisk is source of truth for media)
   │   │   └─┬──────────┘
   │   │     │ ARI: StasisEnd OR explicit Hangup API
   │   │     ▼
   │   │   ┌──────┐
   │   └─...│ENDED│  (terminal; frees agent + triggers queue retry)
   │        └──────┘
   │
   │  Failure / cancellation transitions:
   │        ┌──────────┐
   ├────────│ REJECTED │  (agent clicked reject)
   │        └──────────┘
   ├────────┐
   │        │ CANCELLED│  (customer cancelled before connect)
   │  ┌─────┴──────────┐
   │  │ MISSED         │  (no agent answered within timeout)
   │  └────────────────┘
   ├────────┐
   │        │ FAILED     │  (Asterisk unavailable / ARI error)
   └────────┤TIMEOUT     │
            └────────────┘
```

### Allowed transitions (CallStateMachine)

| From          | To              | Trigger                                  |
| ------------- | --------------- | ---------------------------------------- |
| CREATED       | WAITING         | enqueue + save                           |
| WAITING       | WAITING_AGENT   | agent reserved atomically                |
| WAITING       | CANCELLED       | customer cancel API                      |
| WAITING       | TIMEOUT         | queue TTL exceeded                       |
| WAITING_AGENT | RINGING         | agent accept + ARI originate start       |
| WAITING_AGENT | WAITING         | agent reject / agent timeout             |
| WAITING_AGENT | CANCELLED       | customer cancel API                      |
| WAITING_AGENT | MISSED          | agent timeout (no one accepts)           |
| CONNECTING    | RINGING         | ARI: at least one channel up             |
| CONNECTING    | FAILED          | ARI error / Asterisk unreachable         |
| RINGING       | IN_PROGRESS     | ARI: both channels answered              |
| RINGING       | FAILED          | ARI error                                |
| IN_PROGRESS   | ENDED           | ARI StasisEnd or Hangup API              |
| IN_PROGRESS   | FAILED          | Asterisk crash                           |
| ENDED, REJECTED, CANCELLED, MISSED, FAILED, TIMEOUT | _(terminal)_ | — |

---

## 3. Agent State Machine

```
        OFFLINE
            │  (WebSocket connect + heartbeat)
            ▼
        AVAILABLE ────────────────────────────────┐
            │                                    │  ARI StasisEnd
            │  atomic reserve (Lua)              │
            ▼                                    │
        RESERVED                                │
            │                                    │
            │  agent accept                      │
            ▼                                    │
        RINGING ─── agent timeout ──► AVAILABLE │
            │                                    │
            │  ARI: both channels answered       │
            ▼                                    │
         BUSY ───────────────────────────────────┘
            │
            │  agent clicks "Go Away"
            ▼
          AWAY ──────── agent clicks "Available" ─► AVAILABLE
```

---

## 4. Call Flow (Happy Path)

```
Customer Browser                Go Call Service                 Asterisk              Agent Browser
       │                              │                            │                         │
       │ POST /api/calls              │                            │                         │
       │ {customer_id, idem_key}      │                            │                         │
       │─────────────────────────────►│                            │                         │
       │                              │ 1) Insert call(WAITING)    │                         │
       │                              │ 2) Idempotency check       │                         │
       │                              │ 3) ZADD call_queue         │                         │
       │                              │ 4) Try atomic reserve      │                         │
       │                              │                            │                         │
       │                              │──► publish WS:             │                         │
       │                              │    call_waiting            │                         │
       │ 201 {call_id, status:        │                            │                         │
       │   WAITING}                   │                            │                         │
       │◄─────────────────────────────│                            │                         │
       │                              │                            │                         │
       │                              │ WS: incoming_call          │                         │
       │                              │─────────────────────────────────────────────────────►│
       │                              │                            │                         │
       │                              │                            │  POST /api/calls/{id}/accept │
       │                              │◄─────────────────────────────────────────────────────│
       │                              │ 1) Validate Agent=RESERVED│                         │
       │                              │ 2) Call = CONNECTING       │                         │
       │                              │ 3) ARI: POST /channels    │                         │
       │                              │    originate customer      │                         │
       │                              │───────────────────────────►│                         │
       │                              │    originate agent         │                         │
       │                              │───────────────────────────►│                         │
       │  SIP INVITE (WSS)            │                            │                         │
       │◄─────────────────────────────────────────────────────────│                         │
       │                              │                            │  SIP INVITE (WSS)        │
       │                              │─────────────────────────────────────────────────────►│
       │                              │                            │                         │
       │                              │ ARI WS: StasisStart        │                         │
       │                              │◄──────────────────────────│                         │
       │                              │ Call = RINGING             │                         │
       │                              │                            │                         │
       │                              │ ARI WS: Channel answered   │                         │
       │                              │◄──────────────────────────│                         │
       │                              │ Call = IN_PROGRESS         │                         │
       │                              │ Agent = BUSY               │                         │
       │                              │                            │  ◄──► RTP media ──►      │
       │                              │                            │                         │
       │                              │ ARI WS: StasisEnd          │                         │
       │                              │◄──────────────────────────│                         │
       │                              │ Call = ENDED               │                         │
       │                              │ Agent = AVAILABLE          │                         │
       │                              │ Trigger routing for next   │                         │
       │                              │ WS: call_ended             │                         │
       │◄─────────────────────────────│                            │                         │
       │◄─────────────────────────────────────────────────────────│                         │
```

---

## 5. Redis Keys

| Key                                  | Type   | TTL            | Purpose                                       |
| ------------------------------------ | ------ | -------------- | --------------------------------------------- |
| `call:queue:waiting`                 | ZSET   | none           | score = `-priority*1e13 + now_ms`; FIFO+priority |
| `agent:{agent_id}:state`             | STRING | none           | OFFLINE / AVAILABLE / RESERVED / RINGING / BUSY / AWAY |
| `agent:{agent_id}:current_call`      | STRING | none           | call_id while not AVAILABLE                   |
| `agent:{agent_id}:session`           | STRING | 60s sliding    | last WS heartbeat (used to detect offline)    |
| `agents:available`                   | ZSET   | none           | score = epoch s; AVAILABLE agents only       |

The routing worker also listens to an in-process channel
(`call.RouterSignal()`) as an immediate wake-up signal — there is no Redis
PubSub dependency for routing triggers.

### Atomic Reserve (Lua, single round-trip)

```lua
-- KEYS[1] = agent:{id}:state, KEYS[2] = agent:{id}:current_call, KEYS[3] = agents:available
-- ARGV[1] = agent_id, ARGV[2] = call_id
local st = redis.call("GET", KEYS[1])
if not st then
  redis.call("SET", KEYS[1], "AVAILABLE")
  st = "AVAILABLE"
end
if st ~= "AVAILABLE" then return {0, st} end
redis.call("SET", KEYS[1], "RESERVED")
redis.call("SET", KEYS[2], ARGV[2])
redis.call("ZREM", KEYS[3], ARGV[1])
return {1, "RESERVED"}
```

If two requests arrive concurrently, only one wins the EVAL — the loser
sees the new state and bails. This is the only mechanism that guarantees
a single agent cannot be assigned to two calls.

### Release Agent (Lua)

```lua
-- KEYS[1] = agent:{id}:state, KEYS[2] = agent:{id}:current_call
-- ARGV[1] = agent_id
local st = redis.call("GET", KEYS[1])
if not st then return {0, "MISSING"} end
if st ~= "RESERVED" and st ~= "RINGING" and st ~= "BUSY" then
  return {0, st}
end
redis.call("SET", KEYS[1], "AVAILABLE")
redis.call("DEL", KEYS[2])
return {1, "AVAILABLE"}
```

(Redis manager then re-adds the agent to `agents:available` outside the script.)

### Enqueue (`ZADD` with score = `-priority*1e13 + now_ms`)

```
ZADD call:queue:waiting <score> <call_id>
```

`PopNext` uses `ZPOPMIN` (lowest score first) which preserves FIFO within
the same priority and breaks ties by insertion time.

### Idempotency (DB, not Redis)

Idempotency is kept in PostgreSQL (`call_idempotency` table) rather than
Redis so that a Redis restart does not lose the dedup window. The
`POST /api/calls` flow uses an atomic `INSERT … ON CONFLICT DO NOTHING`
(claim) followed by either replay or fall-through — see §11 of the test
suite for the race semantics.

---

## 6. Asterisk / ARI Integration

### Components

| Path                              | Role                                                      |
| --------------------------------- | --------------------------------------------------------- |
| `internal/infrastructure/asterisk/ari_client.go`        | HTTP client (`POST /ari/channels`, `/bridges`, …) |
| `internal/infrastructure/asterisk/ari_ws.go`            | ARI WebSocket consumer (StasisStart/End, Channel events) |
| `internal/infrastructure/asterisk/ari_mock.go`          | In-memory mock for tests + dev without Asterisk          |
| `internal/infrastructure/asterisk/gateway.go`           | `AsteriskGateway` interface (domain-facing)               |
| `internal/infrastructure/asterisk/dialplan/pjsip.conf`  | PJSIP config (sip.js → WSS)                              |

### AsteriskGateway interface (actual)

```go
type AsteriskGateway interface {
    // Customer
    OriginateCustomer(ctx, callID uuid.UUID, customerEndpoint string) (channelID string, err error)
    OriginateAgent   (ctx, callID uuid.UUID, agentID string)        (channelID string, err error)

    // Bridge
    CreateBridge  (ctx, bridgeType string) (bridgeID string, err error)
    BridgeChannels(ctx, bridgeID string, channelIDs ...string) error

    // Lifecycle
    Hangup(ctx, callID uuid.UUID) error

    // Recording
    StartRecording(ctx, callID uuid.UUID, bridgeID string) error
    StopRecording (ctx, callID uuid.UUID) error

    // Health
    HealthCheck(ctx) error
}
```

Concrete types:
- `*asterisk.Client`      — real HTTP client.
- `*asterisk.MockGateway` — in-memory for tests & dev without Asterisk.

SIP endpoint mapping lives in `internal/infrastructure/asterisk`:
`asterisk.CustomerEndpoint(customerID)` and the internal `agentSIPEndpoint`.

### ARI events the consumer listens to

```
StasisStart, StasisEnd
ChannelCreated, ChannelDestroyed, ChannelStateChange
ChannelEnteredBridge, ChannelLeftBridge
RecordingStarted, RecordingFinished
BridgeCreated, BridgeDestroyed
```

Each event becomes a `call_event` row (`source=ARI`) and triggers a state transition.

### Asterisk container (docker-compose)

```yaml
asterisk:
  image: andrius/asterisk:latest
  ports: ["8088:8088", "5060:5060/udp", "5060:5060/tcp", "5061:5061/tcp"]
  environment:
    - ASTERISK_ARI_USER=callservice
    - ASTERISK_ARI_PASSWORD=callsecret
  volumes:
    - ./internal/infrastructure/asterisk/dialplan:/etc/asterisk
```

---

## 7. Failure Handling

| Scenario                               | Go action                                                                 |
| -------------------------------------- | ------------------------------------------------------------------------- |
| Customer disconnect while WAITING      | WS hub's `disconnectHook` → `CallUseCase.CustomerDisconnected(id)` → ZREM queue, Call=CANCELLED, agent (if any) → AVAILABLE |
| Agent reject                           | `RejectCall` → Agent RESERVED→AVAILABLE; if Call still WAITING_AGENT, WAITING_AGENT→WAITING + re-enqueue |
| Agent timeout (no accept in `ringTimeout`) | `armRingTimeout` goroutine → Agent RESERVED→AVAILABLE; Call WAITING_AGENT→WAITING + re-enqueue at head |
| Asterisk unavailable                   | `Gateway.HealthCheck` returns ErrAsteriskUnavailable in AcceptCall → Call=FAILED(reason=`asterisk_unavailable`), agent released |
| Asterisk reconnect                     | ARI WS consumer reconnects with exponential backoff up to 30s |
| Duplicate ARI event                    | `call_events` UNIQUE INDEX on (call_id, event_type, occurred_at); state machine is idempotent because transitions check CAS current state |
| Out-of-order ARI event                 | State machine refuses to regress (e.g. ENDED → IN_PROGRESS is rejected); `ChannelStateChange→Up` after ENDED is ignored |
| Go service restart                     | In-flight calls are read from PG; `Reconcile` worker frees stuck agents and fails stuck calls. `UseCase.Shutdown(ctx)` cancels all internal ring-timeout goroutines for clean shutdown. |
| Redis reconnect                        | The Lua-backed manager reconnects via go-redis client retry; enqueue/route failures are logged and the call remains in PG with status WAITING |
| Agent stuck RESERVED/RINGING/BUSY      | Reconciliation worker (every `ReconcileEvery`) lists active agents via SCAN; if current_call points to a terminal/missing call → release; if call is non-terminal but stale (> maxDuration) → failCall + release |

### Reconciliation worker (configurable interval, default 30s)

```pseudo
for each agent in ACTIVE_STATES = {RESERVED, RINGING, BUSY}:
    call = repo.Get(agent.current_call)
    if call == nil OR call.status in TERMINAL:
        queue.ReleaseAgent(agent)
    elif now - call.last_event_at > CALL_MAX_DURATION:
        failCall(call, "max_duration_exceeded")
        queue.ReleaseAgent(agent)

for each call in repo.ListActive(older=queueTTL):
    if call.status == WAITING:
        queue.RemoveFromQueue(call)
        transition(call, TIMEOUT, "queue_ttl_exceeded")
```

---

## 8. API Reference

| Method | Path                          | Caller    | Idempotent | Notes                          |
| ------ | ----------------------------- | --------- | ---------- | ------------------------------ |
| POST   | `/api/calls`                  | Customer  | ✅ `Idempotency-Key` header (atomic claim → cache replay) | Create call + enqueue |
| GET    | `/api/calls/:id`              | Both      | n/a        | Authoritative state from PG (use after WS reconnect) |
| POST   | `/api/calls/:id/accept`       | Agent     | ✅ (`call_id+role=agent+action=accept` UNIQUE) | WAITING_AGENT→CONNECTING |
| POST   | `/api/calls/:id/reject`       | Agent     | ✅         | WAITING_AGENT→WAITING, re-route |
| POST   | `/api/calls/:id/hangup`       | Both      | ✅ (no-op on terminal) | non-terminal → ENDED    |
| POST   | `/api/calls/:id/cancel`       | Customer  | ✅         | WAITING/WAITING_AGENT → CANCELLED |
| POST   | `/api/agents/:id/heartbeat`   | Agent     | n/a        | Refresh 60s agent session    |
| POST   | `/api/agents/:id/status`      | Agent     | n/a        | Body `{status: "AVAILABLE"\|"AWAY"\|"OFFLINE"}` |

### Idempotency

- `POST /api/calls` uses an atomic `ClaimIdempotency` (single `INSERT ... ON CONFLICT DO NOTHING` on `call_idempotency.idempotency_key`). Exactly one concurrent retry wins the claim and creates the call; others poll briefly for the cached response and replay it.
- `POST /api/calls/{id}/{accept|reject|hangup|cancel}` are guarded by `(call_id, role, action)` UNIQUE INDEX on `call_participants`. A duplicate insert returns `ErrDuplicateAction`, which the use case treats as a successful no-op.
- State transitions are CAS-guarded: each `UPDATE calls SET status = ? WHERE id = ? AND status = ?` only succeeds if the row was actually in the expected `from` state, so concurrent API retries cannot regress or duplicate a transition.

---

## 9. WebSocket Events (read-only from client perspective)

Server → client:

```
call_waiting            → customer: you're in queue
incoming_call           → agent: someone is calling
call_connecting         → both: ARI is setting up
call_ringing            → both: media is ringing
call_started            → both: ARI confirmed both answered
call_ended              → both: call finished
call_failed             → both: ARI error / timeout
agent_status_changed    → admin: agent went AVAILABLE/BUSY/AWAY
queue_position_changed  → customer: your position in queue
```

Client → server (only for UI, never authoritative):

```
agent_set_status { status: AVAILABLE|AWAY }
```

> Reconnect strategy: client must `GET /api/calls/{id}` after WS reconnect to recover authoritative state.

---

## 10. Sequence: 100 Customers vs 5 Agents

```
Test scenario: 100 customers POST /api/calls at once, 5 agents AVAILABLE.

t=0    100 calls enqueued (ZADD)
       100 goroutines try atomic reserve:
         - first 5 win (Lua returns {1,"RESERVED"})
         - remaining 95 see state≠AVAILABLE → wait on notify PubSub
t=0+ε  5 WS incoming_call events pushed to 5 agents
t=≤30s agents either accept or timeout
       On accept: Call=CONNECTING → ARI originate
       On reject:  agent=AVAILABLE; PubSub notifies queue worker
                   next call in ZSET is popped → repeat
       On timeout: same as reject (but Call=MISSED)
t=30s+ queue depth ≈ 95 - 5 = 90 (or fewer if some accepted)

Concurrency guarantees (Lua + ZSET + WATCH-free):
  - No two agents end up RESERVED for the same call (single Lua call).
  - No call is left without an agent (or in MISSED) until queue is drained.
  - Reconciliation worker fixes any drift every 30s.
```

---

## 11. Database Schema (sketch)

```sql
-- Replace voice_calls with richer calls table.
-- voice_calls is kept as an audit-only view (no app writes).

CREATE TYPE call_status AS ENUM (
    'CREATED','WAITING','WAITING_AGENT','CONNECTING','RINGING','IN_PROGRESS',
    'ENDED','REJECTED','CANCELLED','MISSED','FAILED','TIMEOUT'
);

CREATE TYPE call_event_source AS ENUM ('API','ARI','SYSTEM');
CREATE TYPE call_party_role AS ENUM ('customer','agent','system');
CREATE TYPE call_recording_state AS ENUM ('pending','recording','stopped','failed');

CREATE TABLE calls (
    id               UUID PRIMARY KEY,
    customer_id      TEXT NOT NULL,
    agent_id         TEXT,
    status           call_status NOT NULL DEFAULT 'CREATED',
    priority         INT NOT NULL DEFAULT 0,
    requested_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_at      TIMESTAMPTZ,
    started_at       TIMESTAMPTZ,
    ended_at         TIMESTAMPTZ,
    duration_seconds INT NOT NULL DEFAULT 0,
    last_event_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ari_bridge_id    TEXT,
    failure_reason   TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE call_events (
    id            BIGSERIAL PRIMARY KEY,
    call_id       UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    event_type    TEXT NOT NULL,
    source        call_event_source NOT NULL,
    agent_id      TEXT,
    payload       JSONB NOT NULL DEFAULT '{}',
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX ux_call_events_dedupe
    ON call_events(call_id, event_type, occurred_at);

CREATE INDEX ix_call_events_call ON call_events(call_id, occurred_at);

CREATE TABLE call_participants (
    id              BIGSERIAL PRIMARY KEY,
    call_id         UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    participant_id  TEXT NOT NULL,        -- agent_id or customer_id
    role            call_party_role NOT NULL,
    action          TEXT NOT NULL,        -- accept | reject | hangup | cancel
    action_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (call_id, role, action)        -- idempotency at DB layer
);

CREATE TABLE call_recordings (
    id            BIGSERIAL PRIMARY KEY,
    call_id       UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    ari_recording TEXT,
    storage_url   TEXT,
    started_at    TIMESTAMPTZ,
    ended_at      TIMESTAMPTZ,
    state         call_recording_state NOT NULL DEFAULT 'pending'
);

-- voice_calls table is left in place but reads-only.
-- A migration script copies rows to call_events for historical audit.
```

---

## 12. Configuration (env vars)

| Var                          | Default                    | Purpose                            |
| ---------------------------- | -------------------------- | ---------------------------------- |
| `ASTERISK_ARI_URL`           | `http://asterisk:8088/ari` | ARI base URL                       |
| `ASTERISK_ARI_USER`          | `callservice`              | ARI user                           |
| `ASTERISK_ARI_PASSWORD`      | `callsecret`               | ARI password                       |
| `ASTERISK_ARI_APP`           | `callapp`                  | Stasis app name (must match pjsip.conf) |
| `ASTERISK_RECORDING_ENABLED` | `false`                    | Start MixMonitor on answer         |
| `CALL_AGENT_RING_TIMEOUT`    | `30s`                      | RESERVED → WAITING if no accept    |
| `CALL_MAX_DURATION`          | `1800s`                    | hard cap on IN_PROGRESS            |
| `CALL_QUEUE_TTL`             | `600s`                     | drop WAITING calls older than this |
| `CALL_RECONCILE_INTERVAL`    | `30s`                      | cleanup worker period              |
| `CALL_ASTERISK_HEALTHCHECK`  | `10s`                      | pre-flight before originate        |

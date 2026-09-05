# Call Architecture (Refactored)

> Status: **Refactor in progress** — see `docs/ARCHITECTURE.md` for the
> high-level app architecture. This document focuses only on the
> **Customer Support Voice Call** flow, which now follows the rules
> laid out in `Untitled-1.md` (Call Center Spec).

---

## 1. Goals

1. Many customers can request a call at the same time.
2. Few agents service many customers.
3. Asterisk handles **all** media (SIP / WSS / WebRTC / SRTP / RTP).
4. Go owns **all** business logic: queueing, agent routing, call state,
   audit log, WebSocket notification.
5. The frontend **never** decides call state. It only:
   - displays the UI,
   - calls `POST /api/calls/{id}/accept|reject|hangup`,
   - listens for server-sent WS events,
   - hosts a sip.js WebRTC agent extension registered with Asterisk.

---

## 2. High-level diagram

```
                 Customer Browser
                        │
              REST POST /api/calls
                        ▼
                ┌───────────────┐
                │  Go Call Svc  │
                │               │
                │  PostgreSQL ◄─┼─── Call Log + Call Events (audit)
                │               │
                │  Redis     ◄──┼─── Queue (FIFO) + Agent State + Idempotency Keys
                │               │
                │  WebSocket  ──┼───► Agent Browser (banner, status updates)
                └──────┬────────┘
                       │ ARI REST + ARI WebSocket
                       ▼
                ┌───────────────┐
                │   Asterisk    │
                │  (Stasis app) │
                └──┬─────────┬──┘
                   │ SIP/WSS │
                   ▼         ▼
           Customer      Agent
           Browser       Browser
```

---

## 3. Components and responsibilities

| Layer | Owns | Does NOT own |
|-------|------|--------------|
| Asterisk | SIP signaling, PJSIP, RTP/SRTP, DTLS, ICE, media bridge, MixMonitor | Call state, queue, agent routing, persistence |
| Go: `delivery/http` | REST input validation, RBAC | Business logic, ARI/AMI calls |
| Go: `usecase` | Call flow, agent routing, idempotency, state transitions | Wire protocols |
| Go: `infra/asterisk` | ARI client, ARI service, AMI client | Domain logic, DB |
| Go: `infra/redis` | Queue, agent state, idempotency keys, locks | Call records |
| Frontend (Next.js) | sip.js WebRTC, UI, REST calls, WS listeners | Call state authority |

---

## 4. Call flow (end-to-end)

### 4.1 Step 1 — Customer initiates call

```
POST /api/calls
{ "customer_id": "..." }
```

Go flow:

1. Validate customer.
2. Generate `call_id` (UUID or DB serial).
3. Persist call record (status `WAITING`) → **PostgreSQL**.
4. Push into Redis FIFO queue (`queue:call`).
5. Ask `AgentRouter` to find an available agent.

### 4.2 Step 2 — Atomic agent assignment

`AgentRouter.AssignNextAgent` runs a Redis Lua script that, atomically:

```
1. LRANGE queue:call 0 0       (peek the head)
2. SMEMBERS agents:available   (agent_id list)
3. For each agent:
       EVAL  atomic Lua:
          GET  agent:{id}:state     → must be "AVAILABLE"
          SET  agent:{id}:state "RESERVED" EX 30
          RPOP queue:call
          return agent_id
       If success → return
4. None available → return nil
```

This is atomic — no GET/UPDATE race is possible because the entire
script runs inside Redis without yielding.

### 4.3 Step 3 — Notify agent

When an agent is reserved:

- Go updates the call row: `status = WAITING_AGENT`, `agent_id = ext`,
  `assigned_at = now()`.
- Go publishes WS event `incoming_call` to the agent's session
  (`session_id = "agent:{ext}"`).
- A 30-second timer starts. On timeout, agent is released
  (`RESERVED → AVAILABLE`), the call returns to the head of the queue.

### 4.4 Step 4 — Agent accepts

```
POST /api/calls/{call_id}/accept
```

Go flow:

1. Validate call.
2. Validate agent owns the reservation (`agent:{id}:state == RESERVED`
   and call.agent_id == ext).
3. **Idempotency check** by `idempotency_key` header (or auto-derived
   from call_id + actor).
4. State transitions:

   ```
   Agent:    RESERVED → RINGING
   Call:     WAITING_AGENT → CONNECTING
   ```

5. Cancel the timeout timer.
6. Emit WS `call_connecting` to both customer and agent.
7. Call `AsteriskGateway.OriginateAgentCall(callID, agentExt)`.
8. **Asterisk** originates a `PJSIP/{ext}` channel into our Stasis app.

### 4.5 Step 5 — Asterisk bridges

The Asterisk Stasis app (configured in `docker/asterisk/etc/asterisk/extensions.conf`)
brings both legs into a holding bridge:

```
Customer Browser ─SIP/WSS─► Asterisk ─► Stasis app ─► Bridge
Agent Browser    ─SIP/WSS─► Asterisk ─► Stasis app ─► Bridge
```

Asterisk handles all media. Go subscribes to ARI WebSocket events
(`StasisStart`, `ChannelEnteredBridge`, `ChannelLeftBridge`,
`ChannelDestroyed`).

### 4.6 Step 6 — Bridge confirmed

When the ARI event `ChannelEnteredBridge` arrives for the last expected
leg, Go:

- updates call `status = IN_PROGRESS`, `started_at = now()`
- updates agent `state = BUSY`
- persists a `call_events` row (`source=ARI`, `type=BRIDGE_ENTER`)
- publishes WS `call_started` to both sides

### 4.7 Step 7 — Call end

Either party hangs up → sip.js → Asterisk → ARI `ChannelDestroyed`.

Go:

- updates call `status = ENDED`, `ended_at`, `duration_seconds`
- updates agent `state = AVAILABLE` and clears `current_call`
- persists `call_events` row (`source=ARI`, `type=HANGUP`)
- publishes WS `call_ended`
- triggers `AgentRouter.TryRoute` to drain the next queued caller

---

## 5. State machines

### 5.1 Call state machine

```
CREATED
  ↓
WAITING             (in queue, no agent)
  ↓
WAITING_AGENT       (reserved to an agent, awaiting accept)
  ↓
CONNECTING          (Asterisk originating legs)
  ↓
RINGING             (one or both legs ringing)
  ↓
IN_PROGRESS         (bridge established, media flowing)
  ↓
ENDED

Terminal alternatives (any non-terminal state can transition):
  REJECTED   (agent rejected)
  CANCELLED  (customer cancelled before accept)
  MISSED     (agent timeout)
  FAILED     (Asterisk unavailable, all retries exhausted)
  TIMEOUT    (queue wait exceeded maximum)
```

Validation is enforced by `domain.CallStateMachine.TransitionTo(next)`,
which returns an error for illegal transitions (e.g. `WAITING → IN_PROGRESS`).

### 5.2 Agent state machine

```
OFFLINE         (no WS connection)
  ↓ login
AVAILABLE       (idle, can be reserved)
  ↓ reserve
RESERVED        (matched to a call, awaiting accept)
  ↓ accept
RINGING         (Asterisk calling agent leg)
  ↓ answer
BUSY            (bridge up)
  ↓ hangup / bridge destroy
AVAILABLE       (back to pool)

Rejections:
  RESERVED → AVAILABLE   (timeout / agent reject / customer cancel)
  RINGING  → AVAILABLE   (no answer on agent leg)
```

Same validation pattern: `domain.AgentStateMachine.TransitionTo(next)`.

---

## 6. Redis keys

| Key | Type | Purpose |
|-----|------|---------|
| `queue:call` | LIST (RPUSH/LPOP) | FIFO queue of waiting call_ids |
| `call:{id}:state` | STRING | Mirror of PostgreSQL status for cheap reads |
| `call:{id}:agent` | STRING | agent_ext currently assigned (if any) |
| `call:{id}:idem:{op}` | STRING with TTL | Idempotency keys for `accept|reject|hangup` |
| `agent:{ext}:state` | STRING | OFFLINE / AVAILABLE / RESERVED / RINGING / BUSY |
| `agent:{ext}:current_call` | STRING | call_id currently served (or empty) |
| `agent:{ext}:last_activity` | STRING (epoch ms) | For stale-state cleanup |
| `agents:available` | SET | Quick O(1) "who's free" check (rebuilt on state change) |

---

## 7. Idempotency

| Endpoint | Idempotency strategy |
|----------|----------------------|
| `POST /api/calls` | Reject if `customer_id + idempotency_key` already produced an `in_flight` call. |
| `POST /api/calls/{id}/accept` | `idempotency_key` header OR derived (`callID + actor`). Return cached response on retry. |
| `POST /api/calls/{id}/reject` | Same pattern. |
| `POST /api/calls/{id}/hangup` | DB unique constraint on `(call_id, type='HANGUP')` call_event; first writer wins. |

---

## 8. Failure handling

| Failure | Behavior |
|---------|----------|
| Customer disconnect during WAITING | Remove from queue, call → `CANCELLED` |
| Agent rejects | Agent → `AVAILABLE`, call → `WAITING`, retry routing |
| Agent timeout (30 s) | Agent → `AVAILABLE`, call → `WAITING` / `MISSED`, retry routing |
| Asterisk unavailable | Call → `FAILED`. UI shows "Hệ thống tổng đài đang bận". |
| ARI WebSocket drops | Reconnect with exponential backoff. On reconnect, run reconciliation pass against PostgreSQL. |
| Service restart | On boot, reconcile: for any call with `status in (WAITING_AGENT, CONNECTING, RINGING, IN_PROGRESS)`, rehydrate Redis state from PostgreSQL. |
| Stale agent (last_activity > 5 min, state=AVAILABLE/RESERVED) | Periodic cleaner flips back to OFFLINE. |

---

## 9. Asterisk / ARI integration

ARI is the **only** wire interface to Asterisk in the refactored flow
(AMI is deprecated for call-control; it stays around only as a legacy
originate path for outbound PSTN).

The ARI client is hidden behind:

```go
// internal/domain/asterisk.go
type AsteriskGateway interface {
    OriginateGuestCall(ctx context.Context, callID int64, sessionID string) error
    OriginateAgentCall(ctx context.Context, callID int64, sessionID, agentExt string) error
    HangupCall(ctx context.Context, callID int64) error
    StartRecording(ctx context.Context, callID int64) error
}
```

The HTTP layer **never** imports `internal/infra/asterisk`; it only
sees the interface.

### 9.1 Stasis app flow

`docker/asterisk/etc/asterisk/extensions.conf`:

```
[from-internal]
exten => 88,1,Stasis(dongdo-ivr)
exten => 88,n,Hangup()

[dongdo-ivr-in]
exten => s,1,NoOp(Inbound to dongdo-ivr)
 same => n,Stasis(dongdo-ivr)
 same => n,Hangup()
```

`pjsip.conf`:

- Transport `transport-wss` on port 8089 (browser WebRTC).
- `webrtc = yes` on the agent template.
- Codecs: opus, alaw, ulaw, g729.

### 9.2 ARI events consumed

- `StasisStart` — channel entered the app
- `StasisEnd` — channel left the app
- `ChannelEnteredBridge`
- `ChannelLeftBridge`
- `ChannelDestroyed`
- `ChannelStateChange`

Each event is mapped to a `domain.CallEvent` and stored in
`call_events` (PostgreSQL) with `source = 'ARI'`.

---

## 10. Sequence diagrams

### 10.1 Happy path

```
Customer    Go Call Svc     Redis      Asterisk      Agent Browser
   │            │             │           │                │
   │ POST /calls│             │           │                │
   │───────────►│             │           │                │
   │            │ Create row  │           │                │
   │            │ Push queue  │           │                │
   │            │────────────►│           │                │
   │            │ Assign agent│           │                │
   │            │────────────►│           │                │
   │            │ Set reserved│           │                │
   │            │────────────►│           │                │
   │ 202        │             │           │                │
   │◄───────────│             │           │                │
   │            │ WS incoming_call        │                │
   │            │─────────────────────────────────────────►│
   │            │             │           │   SIP REGISTER │
   │            │             │           │◄───────────────│
   │            │ POST /accept │           │                │
   │            │◄────────────────────────│                │
   │            │ Validate + state→RINGING│                │
   │            │ ARI Originate│          │                │
   │            │──────────────────────►│                │
   │            │             │  SIP INVITE               │
   │            │             │──────────────────────────►│
   │            │             │           │ 200 OK         │
   │            │             │◄─────────────────────────│
   │            │             │  BridgeEnter              │
   │            │ ARI BridgeEnter           │                │
   │            │◄──────────────────────│                │
   │            │ state→IN_PROGRESS       │                │
   │            │ Publish call_started     │                │
   │            │─────────────────────────────────────────►│
   │            │             │           │   ... media... │
```

### 10.2 Agent timeout

```
... (steps 1-3 identical) ...
Go                Redis                  Agent Browser
 │  Set 30s TTL    │                            │
 │────────────────►│                            │
 │                 │                            │
 │ (no accept in  │                            │
 │  30 s)          │                            │
 │                 │                            │
 │ Lua:            │                            │
 │   agent state   │                            │
 │   RESERVED→AVAILABLE                         │
 │   call state    │                            │
 │   WAITING_AGENT→WAITING                      │
 │   re-queue head │                            │
 │  retry routing  │                            │
```

---

## 11. Reconciliation

The voice usecase runs a single `reconcile(ctx)` goroutine on startup
plus every 60 s:

1. List calls in PostgreSQL with `status IN (WAITING_AGENT, CONNECTING,
   RINGING, IN_PROGRESS)`.
2. For each:
   - Read `agent:{ext}:state` from Redis.
   - If mismatch, update Redis from PostgreSQL (PostgreSQL wins for
     active sessions).
3. List agents with `last_activity` older than 5 min.
4. Flip them to `OFFLINE` and release any reservation.

This handles process restarts and missed ARI events.

---

## 12. Testing strategy

### 12.1 Unit tests

- `domain.CallStateMachine` — all valid + invalid transitions.
- `domain.AgentStateMachine` — same.
- `usecase.VoiceUseCase` — uses fake `CallRepo`, `QueueManager`,
  `EventBus`, `AsteriskGateway`.

### 12.2 Integration tests

- `infra/redis` queue manager against `miniredis` (in-process).
- `infra/asterisk` ARI service against a fake ARI server.

### 12.3 Load / race tests

- 100 customers calling simultaneously with 5 agents → exactly 5
  calls get `agent_id` set; rest stay in `WAITING`.
- Agent double-accept → only the first wins; second returns 200 (cached
  idempotent response).
- ARI event storm with out-of-order delivery → all calls end in a
  consistent state.

---

## 13. Migration / compatibility

- Existing `POST /api/voice/*` endpoints are **kept** as deprecated
  thin wrappers around the new `POST /api/calls/*` use cases.
- New endpoints take precedence:
  - `POST /api/calls`             (create + queue)
  - `POST /api/calls/{id}/accept`
  - `POST /api/calls/{id}/reject`
  - `POST /api/calls/{id}/hangup`
  - `GET  /api/calls/{id}`        (recover state after reconnect)
- The frontend migration is incremental: the existing
  `IncomingCallBanner` continues to work; only its "accept" click
  switches to `POST /api/calls/{id}/accept` and the sip.js layer is
  reused for the actual media.

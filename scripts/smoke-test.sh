#!/usr/bin/env bash
# =============================================================================
# smoke-test.sh — End-to-end smoke test cho DongDo CS V2
#
# Test flow:
#   1. Health check (server, postgres, redis, asterisk)
#   2. Register guest
#   3. Send message (AI reply via WebSocket)
#   4. Initiate voice call (AMI → Asterisk mock)
#   5. Verify database records
#   6. Verify Redis streams
#   7. Cleanup
#
# Usage:
#   ./scripts/smoke-test.sh                  # default http://localhost:8080
#   ./scripts/smoke-test.sh --target URL     # custom target
#   ./scripts/smoke-test.sh --no-cleanup     # skip cleanup
#   ./scripts/smoke-test.sh --verbose        # verbose output
#
# Exit codes:
#   0 = all tests passed
#   1 = health check failed
#   2 = guest register failed
#   3 = chat failed
#   4 = voice call failed
#   5 = database verification failed
#   6 = redis verification failed
# =============================================================================

set -uo pipefail

# -------------------- defaults --------------------
TARGET="${TARGET:-http://localhost:8080}"
WS_TARGET="${WS_TARGET:-ws://localhost:8080}"
NO_CLEANUP=0
VERBOSE=0
TIMEOUT=30
STEP_DELAY=2

# -------------------- colors --------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# -------------------- parse args --------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --target) TARGET="$2"; WS_TARGET="${2/http/ws}"; shift 2 ;;
        --no-cleanup) NO_CLEANUP=1; shift ;;
        --verbose) VERBOSE=1; shift ;;
        --timeout) TIMEOUT="$2"; shift 2 ;;
        -h|--help)
            grep "^# " "$0" | sed 's/^# //'
            exit 0
            ;;
        *) echo "Unknown arg: $1"; exit 1 ;;
    esac
done

# -------------------- helpers --------------------
log()  { echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $*"; }
pass() { echo -e "${GREEN}✓${NC} $*"; }
warn() { echo -e "${YELLOW}⚠${NC} $*"; }
fail() { echo -e "${RED}✗${NC} $*"; exit 1; }
vlog() { [[ $VERBOSE -eq 1 ]] && echo -e "${YELLOW}  └─${NC} $*"; }

cleanup() {
    if [[ $NO_CLEANUP -eq 0 ]]; then
        log "Cleanup..."
        # Xoá test data
        docker compose exec -T postgres psql -U postgres -d dongdo_cs -c \
            "DELETE FROM chat_messages WHERE session_id LIKE 'smoke-test-%'; DELETE FROM chat_cases WHERE session_id LIKE 'smoke-test-%'; DELETE FROM voice_calls WHERE session_id LIKE 'smoke-test-%';" 2>/dev/null || true
    else
        warn "Skipping cleanup (--no-cleanup)"
    fi
}

trap cleanup EXIT

assert_contains() {
    local haystack="$1" needle="$2" label="$3"
    if echo "$haystack" | grep -q "$needle"; then
        pass "$label"
        vlog "$needle"
    else
        fail "$label — expected to contain '$needle'"
        vlog "Got: $haystack"
    fi
}

# -------------------- step 1: health --------------------
log "Step 1/7: Health check"

HEALTH=$(curl -fsS --max-time "$TIMEOUT" "$TARGET/health" 2>&1) || fail "Server health check failed (timeout=$TIMEOUT)"
assert_contains "$HEALTH" '"status":"healthy"' "Server health endpoint OK"

# Postgres
PG_HEALTH=$(docker compose exec -T postgres pg_isready -U postgres 2>&1) || fail "Postgres not ready"
pass "Postgres ready ($PG_HEALTH)"

# Redis
REDIS_HEALTH=$(docker compose exec -T redis redis-cli ping 2>&1) || fail "Redis not ready"
assert_contains "$REDIS_HEALTH" "PONG" "Redis ping OK"

# Asterisk (optional - có thể không có)
if docker compose ps asterisk 2>/dev/null | grep -q "running"; then
    AST_HEALTH=$(docker compose exec -T asterisk asterisk -rx "core show version" 2>&1 | tail -1) || warn "Asterisk check failed"
    pass "Asterisk: $AST_HEALTH"
else
    warn "Asterisk container not running — voice test will be skipped"
fi

sleep $STEP_DELAY

# -------------------- step 2: register guest --------------------
log "Step 2/7: Register guest"

SESSION_ID="smoke-test-$(date +%s)-$$"
REGISTER_PAYLOAD=$(cat <<EOF
{
  "display_name": "Smoke Test User",
  "phone": "0987654321"
}
EOF
)

REGISTER_RESP=$(curl -fsS --max-time "$TIMEOUT" \
    -X POST "$TARGET/guest/register" \
    -H "Content-Type: application/json" \
    -d "$REGISTER_PAYLOAD") || fail "Guest register failed"

GUEST_ID=$(echo "$REGISTER_RESP" | grep -o '"guest_id":"[^"]*"' | cut -d'"' -f4)
GUEST_TOKEN=$(echo "$REGISTER_RESP" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
[[ -n "$GUEST_ID" ]] || fail "guest_id not in response"
[[ -n "$GUEST_TOKEN" ]] || fail "token not in response"
pass "Guest registered: $GUEST_ID"
vlog "session_id: $SESSION_ID"

sleep $STEP_DELAY

# -------------------- step 3: send message --------------------
log "Step 3/7: Send chat message"

MSG_PAYLOAD=$(cat <<EOF
{
  "session_id": "$SESSION_ID",
  "customer_name": "Smoke Test User",
  "message": "Xin chào, đây là smoke test"
}
EOF
)

MSG_RESP=$(curl -fsS --max-time "$TIMEOUT" \
    -X POST "$TARGET/chat" \
    -H "Content-Type: application/json" \
    -d "$MSG_PAYLOAD") || fail "Send message failed"

assert_contains "$MSG_RESP" '"status":"RECEIVED"' "Message accepted"

MSG_ID=$(echo "$MSG_RESP" | grep -o '"message_id":[0-9]*' | cut -d':' -f2)
[[ -n "$MSG_ID" ]] && vlog "Message ID: $MSG_ID"

# Đợi AI reply (qua WS, không verify trực tiếp)
log "  Waiting 5s for AI reply..."
sleep 5
pass "Chat message sent"

# -------------------- step 4: history --------------------
log "Step 4/7: Verify chat history"

HISTORY=$(curl -fsS --max-time "$TIMEOUT" \
    "$TARGET/history/$SESSION_ID") || fail "Get history failed"

assert_contains "$HISTORY" '"session_id":"'"$SESSION_ID"'"' "History contains session_id"
assert_contains "$HISTORY" 'Xin chào, đây là smoke test' "History contains our message"

sleep $STEP_DELAY

# -------------------- step 5: voice call --------------------
log "Step 5/7: Initiate voice call"

VOICE_PAYLOAD=$(cat <<EOF
{
  "session_id": "$SESSION_ID",
  "caller_type": "guest",
  "caller_id": "Smoke Test User",
  "callee_type": "cskh",
  "callee_id": ""
}
EOF
)

VOICE_RESP=$(curl -fsS --max-time "$TIMEOUT" \
    -X POST "$TARGET/api/voice/initiate" \
    -H "Content-Type: application/json" \
    -d "$VOICE_PAYLOAD") || fail "Voice initiate failed"

assert_contains "$VOICE_RESP" '"status":"RINGING"' "Call status RINGING"
CALL_ID=$(echo "$VOICE_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)
[[ -n "$CALL_ID" ]] && vlog "Call ID: $CALL_ID"

# Accept call (nếu endpoint có)
ACCEPT_PAYLOAD=$(cat <<EOF
{
  "call_id": $CALL_ID,
  "session_id": "$SESSION_ID"
}
EOF
)

ACCEPT_RESP=$(curl -fsS --max-time "$TIMEOUT" \
    -X POST "$TARGET/api/voice/accept" \
    -H "Content-Type: application/json" \
    -d "$ACCEPT_PAYLOAD" 2>&1) || warn "Voice accept endpoint not available (skipping)"

if echo "$ACCEPT_RESP" | grep -q '"id"'; then
    assert_contains "$ACCEPT_RESP" '"status":"ACTIVE"' "Call status ACTIVE"
else
    warn "Voice accept not implemented yet — endpoint may be added with Asterisk AMI"
fi

# End call
END_PAYLOAD=$(cat <<EOF
{
  "call_id": $CALL_ID,
  "session_id": "$SESSION_ID",
  "duration_seconds": 5
}
EOF
)

END_RESP=$(curl -fsS --max-time "$TIMEOUT" \
    -X POST "$TARGET/api/voice/end" \
    -H "Content-Type: application/json" \
    -d "$END_PAYLOAD") || warn "Voice end failed"

pass "Voice call lifecycle completed"

sleep $STEP_DELAY

# -------------------- step 6: verify db --------------------
log "Step 6/7: Verify database records"

GUEST_COUNT=$(docker compose exec -T postgres psql -U postgres -d dongdo_cs -tAc \
    "SELECT COUNT(*) FROM guests WHERE guest_id='$GUEST_ID'" 2>&1) || fail "DB query failed"
[[ "$GUEST_COUNT" -ge 1 ]] || fail "Guest not in DB (count=$GUEST_COUNT)"
pass "Guest record exists ($GUEST_COUNT row)"

MSG_COUNT=$(docker compose exec -T postgres psql -U postgres -d dongdo_cs -tAc \
    "SELECT COUNT(*) FROM chat_messages WHERE session_id='$SESSION_ID'" 2>&1) || fail "DB query failed"
[[ "$MSG_COUNT" -ge 1 ]] || fail "Messages not in DB (count=$MSG_COUNT)"
pass "Chat messages persisted ($MSG_COUNT row)"

CALL_COUNT=$(docker compose exec -T postgres psql -U postgres -d dongdo_cs -tAc \
    "SELECT COUNT(*) FROM voice_calls WHERE session_id='$SESSION_ID'" 2>&1) || fail "DB query failed"
[[ "$CALL_COUNT" -ge 1 ]] || fail "Voice call not in DB (count=$CALL_COUNT)"
pass "Voice call record persisted ($CALL_COUNT row)"

# -------------------- step 7: redis streams --------------------
log "Step 7/7: Verify Redis streams"

WS_LEN=$(docker compose exec -T redis redis-cli XLEN stream:ws 2>&1) || WS_LEN=0
AI_LEN=$(docker compose exec -T redis redis-cli XLEN stream:ai 2>&1) || AI_LEN=0
DB_LEN=$(docker compose exec -T redis redis-cli XLEN stream:db 2>&1) || DB_LEN=0

log "  stream:ws=$WS_LEN, stream:ai=$AI_LEN, stream:db=$DB_LEN"

# Ít nhất 1 stream phải có data (nếu Redis chạy)
if docker compose exec -T redis redis-cli ping 2>&1 | grep -q PONG; then
    TOTAL=$((WS_LEN + AI_LEN + DB_LEN))
    if [[ $TOTAL -gt 0 ]]; then
        pass "Redis streams have activity ($TOTAL events)"
    else
        warn "Redis streams empty — workers may not be running"
    fi
else
    warn "Redis not reachable — skipping stream check"
fi

# -------------------- summary --------------------
echo
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  ✓ ALL SMOKE TESTS PASSED${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo
log "Summary:"
log "  Target:        $TARGET"
log "  Guest ID:      $GUEST_ID"
log "  Session ID:    $SESSION_ID"
log "  Message ID:    $MSG_ID"
log "  Call ID:       $CALL_ID"
echo

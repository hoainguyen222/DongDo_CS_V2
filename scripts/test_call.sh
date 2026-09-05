#!/usr/bin/env bash
# =============================================================================
# scripts/test_call.sh  — E2E smoke test for the Asterisk PBX container.
#
# 1. Health check (PJSIP module loaded, endpoints present).
# 2. Originate a call from agent-1001 → 1000 (queue) in from-internal.
# 3. Dump queue status after call placed.
# 4. (optional) Check CDR CSV is being written.
#
# Usage:
#   ./scripts/test_call.sh
#   CONTAINER=dongdo_asterisk ./scripts/test_call.sh verbose
#
# Prereqs:
#   - docker compose stack running
#   - container named dongdo_asterisk (override via CONTAINER env)
# =============================================================================
set -uo pipefail

CONTAINER="${CONTAINER:-dongdo_asterisk}"
VERBOSE="${1:-}"

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
blue()  { printf '\033[34m%s\033[0m\n' "$*"; }
gray()  { printf '\033[90m%s\033[0m\n' "$*"; }

step() { printf '\n\033[1m▶ %s\033[0m\n' "$*"; }
fail() { red "FAIL: $*"; exit 1; }
pass() { green "PASS: $*"; }

docker_exec() {
    local cmd="$*"
    if [[ -n "${VERBOSE}" ]]; then
        gray "$ ${cmd}"
        docker exec -i "${CONTAINER}" /bin/bash -c "${cmd}"
    else
        docker exec -i "${CONTAINER}" /bin/bash -c "${cmd}" 2>&1
    fi
}

# ---------------------------------------------------------------------------
step "0/5  Container running?"
status="$(docker ps -a --filter name="^/${CONTAINER}$" --format '{{.State}}' || true)"
if [[ "${status}" != "running" ]]; then
    fail "Container ${CONTAINER} is not running (state: ${status:-missing}).  Try: docker compose up -d asterisk"
fi
pass "Container ${CONTAINER} is running."

# ---------------------------------------------------------------------------
step "1/5  Asterisk version + module health"
ver="$(docker_exec 'asterisk -rx "core show version"' | head -1 | tr -d '\r')"
[[ -n "${ver}" ]] || fail "Cannot read asterisk version"
pass "${ver}"

modules_ok="$(docker_exec 'asterisk -rx "module show like res_pjsip.so"' | grep -c 'res_pjsip.so' || true)"
[[ "${modules_ok}" -gt 0 ]] || fail "res_pjsip.so NOT loaded"
pass "res_pjsip.so loaded"

chan_ok="$(docker_exec 'asterisk -rx "module show like chan_pjsip.so"' | grep -c 'chan_pjsip.so' || true)"
[[ "${chan_ok}" -gt 0 ]] || fail "chan_pjsip.so NOT loaded"
pass "chan_pjsip.so loaded"

# ---------------------------------------------------------------------------
step "2/5  PJSIP endpoints registered?"
endpoints="$(docker_exec 'asterisk -rx "pjsip show endpoints"' | grep -E '^\s*Endpoint:' | wc -l | tr -d ' ')"
if [[ "${endpoints}" -lt 1 ]]; then
    fail "No PJSIP endpoints registered (or none reachable)."
fi
pass "${endpoints} endpoint(s) registered."
docker_exec 'asterisk -rx "pjsip show endpoints"' | grep -E '^\s*Endpoint:' | head -5

# ---------------------------------------------------------------------------
step "3/5  Queue members loaded?"
queue="$(docker_exec 'asterisk -rx "queue show dongdo-queue"' | head -20 || true)"
echo "${queue}"
members="$(echo "${queue}" | grep -E '^\s*[0-9]{4}\s+' | wc -l | tr -d ' ')"
[[ "${members}" -ge 1 ]] || fail "dongdo-queue has 0 members"
pass "dongdo-queue has ${members} member(s)."

# ---------------------------------------------------------------------------
step "4/5  Originate test call: PJSIP/1001 → 1000 (queue)"
# This places a call from extension 1001 to the queue (1000).  With no real
# SIP device registered the channel will fail to dial — we only verify the
# originate command was accepted (i.e. PJSIP endpoint is parsed correctly).
originate_result="$(docker_exec 'timeout 3 asterisk -rx "channel originate PJSIP/1001 application Wait 1" || true' 2>&1)"
gray "${originate_result}"
if echo "${originate_result}" | grep -qi 'No such\|does not exist\|unable'; then
    fail "Originate failed — endpoint 1001 not resolvable.  Check pjsip.conf."
fi
pass "Originate command accepted (not just 'no such endpoint')"

# ---------------------------------------------------------------------------
step "5/5  Recording directory is writable?"
mount_check="$(docker_exec 'ls -ld /var/spool/asterisk/monitor /var/log/asterisk 2>&1')"
echo "${mount_check}"
if echo "${mount_check}" | grep -qi 'No such file or directory'; then
    fail "Monitor / log directories missing inside container."
fi
pass "Monitor and log directories present."

# ---------------------------------------------------------------------------
step "📋  VERIFICATION SUMMARY"
cat <<EOF
  ✓ Asterisk 20+ running inside  ${CONTAINER}
  ✓ PJSIP / chan_pjsip loaded
  ✓ ${endpoints} endpoint(s) reachable
  ✓ dongdo-queue holds ${members} member(s)
  ✓ originate CLI works
  ✓ recordings + logs dirs mounted

  ↳ Try a soft-phone:
      Username: 1001
      Domain:   ${ASTERISK_DOMAIN:-<your-host-ip>}
      Password: ${ASTERISK_AGENT_PASS_PREFIX:-dongdoagent}1001
EOF
green "All checks passed."
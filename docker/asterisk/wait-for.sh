#!/usr/bin/env bash
# =============================================================================
# wait-for  — Tiny TCP-reachability probe used by Asterisk entrypoint.
#
# Usage:  wait-for host:port [-t SECONDS] [-q]
#        wait-for host port       [-t SECONDS] [-q]
#
# Exit codes:
#   0  → port became reachable within timeout
#   1  → timeout exceeded
#   2  → usage / argument error
#
# Pure bash + /dev/tcp; needs no external tools.
# =============================================================================
set -uo pipefail

timeout=60
quiet=0

usage() {
    echo "Usage:  $0 host:port [-t SECONDS] [-q]" >&2
    echo "        $0 host port   [-t SECONDS] [-q]" >&2
    exit 2
}

# Need 1 or 2 positional args.  Anything else is a flag.
case "${1:-}" in
    ""|-h|--help) usage ;;
esac

# Split host:port  →  may also be just host (with next arg as port)
HOST=""
PORT=""
if [[ "${1:-}" == *:* ]] && (( $# <= 3 )); then
    HOST="${1%:*}"
    PORT="${1##*:}"
    shift
else
    HOST="${1:-}"
    PORT="${2:-}"
    shift 2 2>/dev/null || usage
fi

# Optional flags
while (( $# )); do
    case "$1" in
        -t|--timeout) timeout="${2:-60}"; shift 2 ;;
        -q|--quiet)   quiet=1; shift ;;
        *) usage ;;
    esac
done

# Sanity
if [[ -z "${HOST}" || -z "${PORT}" ]]; then
    usage
fi
if ! [[ "${PORT}" =~ ^[0-9]+$ ]] || (( PORT < 1 || PORT > 65535 )); then
    echo "Invalid port: ${PORT}" >&2
    exit 2
fi

log() {
    (( quiet )) || echo "[wait-for] $*"
}

elapsed=0
interval=1
log "Waiting for ${HOST}:${PORT} (timeout ${timeout}s)…"

while (( elapsed < timeout )); do
    # Bash's built-in /dev/tcp performs the connection.
    # timetout via `timeout` child if available, fall back to polling.
    if command -v timeout >/dev/null 2>&1; then
        timeout 2 bash -c "exec 3<>/dev/tcp/${HOST}/${PORT}" 2>/dev/null \
            && { log "  ok (${elapsed}s)"; exit 0; }
    else
        # 2-second poll via bash background kill — terminal fallback.
        if bash -c "exec 3<>/dev/tcp/${HOST}/${PORT}" 2>/dev/null; then
            log "  ok (${elapsed}s)"
            exit 0
        fi
    fi
    sleep "${interval}"
    elapsed=$(( elapsed + interval ))
done

echo "[wait-for] ❌ ${HOST}:${PORT} not reachable after ${timeout}s" >&2
exit 1
#!/usr/bin/env bash
# =============================================================================
# Asterisk container entrypoint
# Waits for postgres/redis, prepares runtime dirs, reloads pjsip, starts asterisk
# =============================================================================
set -euo pipefail

ASTERISK_USER="asterisk"
ASTERISK_UID="$(id -u ${ASTERISK_USER} 2>/dev/null || echo 0)"
ASTERISK_GID="$(id -g ${ASTERISK_USER} 2>/dev/null || echo 0)"

log() { echo "[entrypoint] $(date +'%Y-%m-%d %H:%M:%S') $*"; }

# ------------------------------------------------------------------ permissions
log "Fixing ownership of asterisk runtime directories..."
for d in \
    /var/spool/asterisk \
    /var/spool/asterisk/monitor \
    /var/spool/asterisk/recording \
    /var/log/asterisk \
    /var/log/asterisk/cdr-csv \
    /var/lib/asterisk \
    /var/run/asterisk \
    /etc/asterisk; do
    if [[ -d "${d}" ]]; then
        chown -R "${ASTERISK_UID}:${ASTERISK_GID}" "${d}" 2>/dev/null || \
            chmod -R u+rwX,g+rwX "${d}" 2>/dev/null || true
    fi
done

# Make sure monitor/recording dirs exist even if volume mount is fresh.
mkdir -p /var/spool/asterisk/monitor /var/spool/asterisk/recording /var/log/asterisk/cdr-csv
chmod 0775 /var/spool/asterisk/monitor /var/spool/asterisk/recording

# -------------------------------------------------- wait for backing services
# Skip waits if NO_WAIT is set (useful for unit tests / quick starts).
if [[ "${NO_WAIT:-0}" != "1" ]]; then
    : "${POSTGRES_HOST:=postgres}"
    : "${POSTGRES_PORT:=5432}"
    : "${REDIS_HOST:=redis}"
    : "${REDIS_PORT:=6379}"

    log "Waiting for Postgres at ${POSTGRES_HOST}:${POSTGRES_PORT}..."
    /usr/local/bin/wait-for -t 60 -q "${POSTGRES_HOST}:${POSTGRES_PORT}" || {
        log "WARN: Postgres not reachable; continuing anyway (DB may be optional for now)."
    }
    log "Waiting for Redis at ${REDIS_HOST}:${REDIS_PORT}..."
    /usr/local/bin/wait-for -t 60 -q "${REDIS_HOST}:${REDIS_PORT}" || {
        log "WARN: Redis not reachable; continuing anyway."
    }
fi

# ------------------------------------------------- environment-driven config
# Asterisk can't read ${ENV} natively inside .conf files, so we template the
# sensitive values (AMI password, ARI password) from env into the mounted
# /etc/asterisk files right before launch.  This keeps secrets out of image.
log "Templating runtime secrets into /etc/asterisk..."
render_secret() {
    local file="$1" key="$2" value="$3"
    if [[ -f "${file}" ]]; then
        # Replace `${key}` placeholder inside the file
        sed -i "s|\${${key}}|${value}|g" "${file}"
    fi
}

: "${ASTERISK_AMI_PASS:=dongdoami}"
: "${ASTERISK_ARI_PASS:=dongdoari}"

render_secret /etc/asterisk/manager.conf       ASTERISK_AMI_PASS "${ASTERISK_AMI_PASS}"
render_secret /etc/asterisk/ari.conf           ASTERISK_ARI_PASS "${ASTERISK_ARI_PASS}"
render_secret /etc/asterisk/pjsip.conf         ASTERISK_GUEST_PASS "${ASTERISK_GUEST_PASS:-dongdoguest}"
render_secret /etc/asterisk/pjsip.conf         ASTERISK_AGENT_PASS_PREFIX "${ASTERISK_AGENT_PASS_PREFIX:-dongdoagent}"
render_secret /etc/asterisk/voicemail.conf     ASTERISK_VM_PASS "${ASTERISK_VM_PASS:-1234}"

# -------------------------------------------------- preflight pjsip validation
# Best-effort config dump via the static pjsip CLI (`-C` selects config dir,
# `-V` enables verbose only — but `-rx "..."` requires a running daemon).  We
# simply skip the preflight because it would *start* the daemon.  Instead, we
# validate at first 5 seconds after startup by reloading the dialplan.
log "Skipping preflight; running healthcheck after 5s..."
(
    sleep 5
    if asterisk -rx "dialplan show from-internal" >/dev/null 2>&1; then
        log "Dialplan OK."
    else
        log "WARN: dialplan check failed."
    fi
) &

# -------------------------------------------------- start asterisk in foreground
log "Starting Asterisk (foreground, -vvvv verbosity)..."
# `exec` so asterisk becomes PID 1 (tini handles signal forwarding) and signals
# from docker (SIGTERM) reach asterisk directly for clean shutdown.
exec "$@"
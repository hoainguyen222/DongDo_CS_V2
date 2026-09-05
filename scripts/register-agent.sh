#!/usr/bin/env bash
# =============================================================================
# scripts/register-agent.sh  — Provision an agent extension in pjsip.conf
#   Usage: ./scripts/register-agent.sh 1006  (registers agent 1006 with default pw prefix)
#
# Adds a `[agent-1006]` (endpoint + auth + aor) block to pjsip.conf and reloads
# chan_pjsip via `asterisk -rx "module reload res_pjsip.so"`.
# =============================================================================
set -euo pipefail

PJSIP_CONF="${PJSIP_CONF:-docker/asterisk/etc/asterisk/pjsip.conf}"
CONTAINER="${CONTAINER:-dongdo_asterisk}"
EXTEN="${1:-}"
PASS_PREFIX="${ASTERISK_AGENT_PASS_PREFIX:-dongdoagent}"

if [[ -z "${EXTEN}" || ! "${EXTEN}" =~ ^10[0-9]{2}$ ]]; then
    echo "Usage: $0 <extension>    # extension must match 10XX (e.g. 1006)"
    exit 2
fi

CGI_TAG="[agent-${EXTEN}]"
if grep -q "${CGI_TAG}" "${PJSIP_CONF}"; then
    echo "Agent ${EXTEN} already provisioned in ${PJSIP_CONF}"
    exit 0
fi

cat <<EOF >> "${PJSIP_CONF}"

${CGI_TAG}(template-dongdo)
type=endpoint
aors=aor-${CGI_TAG}
auth=auth-${CGI_TAG}
callerid="Agent ${EXTEN}" <${EXTEN}>
allow=opus,alaw,ulaw,g729
disallow=all

[auth-${CGI_TAG}](auth-dongdo)
type=auth
username=${EXTEN}
password=${PASS_PREFIX}${EXTEN}

[aor-${CGI_TAG}](aor-dongdo)
type=aor
max_contacts=3
remove_existing=yes
EOF

echo "✅ Provisioned agent ${EXTEN} → ${PASS_PREFIX}${EXTEN}"

# Reload pjsip in the running container (best-effort)
if command -v docker >/dev/null 2>&1; then
    if docker exec -i "${CONTAINER}" asterisk -rx "module reload res_pjsip.so" 2>/dev/null; then
        echo "↻ Reloaded res_pjsip.so"
    else
        echo "(skip reload — container ${CONTAINER} not running; will load on next start)"
    fi
fi
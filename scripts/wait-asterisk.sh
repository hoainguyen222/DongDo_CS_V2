#!/usr/bin/env bash
# =============================================================================
# scripts/wait-asterisk.sh  — Wait until the asterisk container is healthy.
# Used in CI pipelines / deploys to make sure subsequent steps find a live PBX.
# =============================================================================
set -uo pipefail

CONTAINER="${CONTAINER:-dongdo_asterisk}"
TIMEOUT="${TIMEOUT:-120}"
INTERVAL="${INTERVAL:-3}"

elapsed=0
echo "Waiting for ${CONTAINER} to become healthy (timeout ${TIMEOUT}s)..."
while (( elapsed < TIMEOUT )); do
    status="$(docker inspect -f '{{.State.Health.Status}}' "${CONTAINER}" 2>/dev/null || echo 'missing')"
    case "${status}" in
        healthy)
            echo "✅ ${CONTAINER} is healthy."
            exit 0
            ;;
        starting|unhealthy|missing|"")
            :
            ;;
        *)
            echo "Unknown state: ${status}"
            ;;
    esac
    sleep "${INTERVAL}"
    elapsed=$(( elapsed + INTERVAL ))
done

echo "❌ ${CONTAINER} did not become healthy in ${TIMEOUT}s."
exit 1
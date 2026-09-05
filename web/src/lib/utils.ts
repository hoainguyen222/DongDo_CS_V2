// ============================================================
// Shared client-side utilities
// ============================================================

/**
 * Derive the Asterisk SIP extension for an authenticated agent.
 *
 * Project convention (see scripts/register-agent.sh):
 *   - Staff extensions are numbered `1001..1099`.
 *   - The username on the dashboard is one of:
 *       * an explicit "10XX" string (e.g. "1006") → return as-is
 *       * a suffix like "cskh1", "agent3", "staff_05" → use the trailing
 *         digits and map to `1000 + <digits>` (clamped to 1001..1099).
 *       * anything else → fall back to "1000" so the backend still
 *         receives a value and the call is at least routed somewhere.
 *
 * The result is sent in the body of POST /api/voice/accept/:call_id as
 * `agent_extension` so Asterisk can redirect the inbound channel.
 */
export function resolveAgentExtension(username?: string | null): string {
  if (!username) return '1000';

  const trimmed = username.trim();

  // Already a 10XX number — return verbatim.
  if (/^10\d{2}$/.test(trimmed)) return trimmed;

  // Pull the last trailing digit-run out of the username.
  const match = trimmed.match(/(\d+)\s*$/);
  if (match) {
    const n = parseInt(match[1], 10);
    if (!Number.isNaN(n)) {
      const ext = 1000 + n;
      if (ext >= 1001 && ext <= 1099) return String(ext);
    }
  }

  return '1000';
}

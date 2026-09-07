'use client';

/**
 * useAgentHeartbeat — auto-pings `POST /api/agents/:username/heartbeat`
 * every `intervalMs` while the admin/agent session is authenticated.
 *
 * This keeps the agent's "last_seen" fresh in Redis so the Call v2 router
 * keeps them in the AVAILABLE pool. Auto-availability is set on login
 * (server-side in HandleLogin) — this hook just refreshes the liveness
 * signal without any user action.
 *
 * Safe to mount in any client component under the admin layout.
 */

import { useEffect } from 'react';
import { useAuthStore } from '@/lib/stores/authStore';
import { api } from '@/lib/api';

interface UseAgentHeartbeatOptions {
  intervalMs?: number;
  enabled?: boolean;
}

export function useAgentHeartbeat({
  intervalMs = 30_000,
  enabled = true,
}: UseAgentHeartbeatOptions = {}) {
  const user = useAuthStore((s) => s.user);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  useEffect(() => {
    if (!enabled || !isAuthenticated || !user?.username) return;

    let cancelled = false;

    const ping = async () => {
      if (cancelled) return;
      try {
        await api.agentHeartbeat(user.username);
      } catch {
        // Network blips are normal in mobile/laptop networks; the next
        // interval will retry. Do not spam the console.
      }
    };

    // Immediate first ping so a freshly-loaded admin tab keeps the
    // AVAILABLE state from drifting if the tab was open before login.
    void ping();

    const id = window.setInterval(ping, intervalMs);

    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [enabled, isAuthenticated, user?.username, intervalMs]);
}

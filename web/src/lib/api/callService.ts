const CALL_SERVICE_BASE = typeof window !== 'undefined' && window.location.hostname
  ? `${window.location.protocol}//${window.location.hostname}:8081`
  : 'http://localhost:8081';

export interface CallV2 {
  id: string;
  session_id: string;
  customer_id: string;
  agent_id?: string;
  status: string;
  requested_at: string;
  assigned_at?: string;
  started_at?: string;
  ended_at?: string;
  duration_seconds: number;
  created_at: string;
  recording?: {
    recording_url: string;
    transcript?: string;
  };
}

export async function requestCall(customerID: string, sessionID?: string): Promise<CallV2> {
  const res = await fetch(`${CALL_SERVICE_BASE}/api/v1/calls`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ customer_id: customerID, session_id: sessionID }),
  });
  if (!res.ok) {
    throw new Error(`Request call failed: ${res.statusText}`);
  }
  return res.json();
}

export async function acceptCall(callID: string, agentID: string): Promise<void> {
  const res = await fetch(`${CALL_SERVICE_BASE}/api/v1/calls/${callID}/accept`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ agent_id: agentID }),
  });
  if (!res.ok) {
    throw new Error(`Accept call failed: ${res.statusText}`);
  }
}

export async function rejectCall(callID: string, agentID: string): Promise<void> {
  const res = await fetch(`${CALL_SERVICE_BASE}/api/v1/calls/${callID}/reject`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ agent_id: agentID }),
  });
  if (!res.ok) {
    throw new Error(`Reject call failed: ${res.statusText}`);
  }
}

export async function hangupCall(callID: string, durationSeconds: number = 0): Promise<void> {
  const res = await fetch(`${CALL_SERVICE_BASE}/api/v1/calls/${callID}/hangup`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ duration_seconds: durationSeconds }),
  });
  if (!res.ok) {
    throw new Error(`Hangup call failed: ${res.statusText}`);
  }
}

export async function fetchCallHistory(limit: number = 100): Promise<{ calls: CallV2[]; total: number }> {
  const res = await fetch(`${CALL_SERVICE_BASE}/api/v1/calls?limit=${limit}`);
  if (!res.ok) {
    // Legacy endpoint on the main backend was removed — the admin API
    // (/api/admin/voice/calls) is the only call-history source now.
    // Throw so callers can fall back to the admin endpoint.
    throw new Error(`Fetch history failed: ${res.statusText}`);
  }
  return res.json();
}

export async function setAgentStatus(agentID: string, status: 'AVAILABLE' | 'OFFLINE' | 'AWAY'): Promise<void> {
  await fetch(`${CALL_SERVICE_BASE}/api/v1/agents/${agentID}/status`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  }).catch(() => {});
}

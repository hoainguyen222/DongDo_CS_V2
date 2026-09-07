// ============================================================
// Voice Calls API — WebRTC call history (Call Service V2)
// ============================================================

import { apiClient, API_BASE } from './client';
import { fetchCallHistory, hangupCall, rejectCall } from './callService';

export interface ListVoiceCallsParams {
  sessionId?: string;
  page?: number;
  limit?: number;
}

export interface ListVoiceCallsResult {
  calls: any[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export const voiceApi = {
  async list(params: ListVoiceCallsParams = {}): Promise<ListVoiceCallsResult> {
    const { page = 1, limit = 10 } = params;
    try {
      const data = await fetchCallHistory(limit * 5);
      const calls = data.calls || [];
      const total = data.total || calls.length;
      return {
        calls,
        total,
        page,
        limit,
        total_pages: Math.ceil(total / limit) || 1,
      };
    } catch (err) {
      // Legacy API Fallback
      const qs = new URLSearchParams();
      if (params.sessionId) qs.append('session_id', params.sessionId);
      qs.append('page', page.toString());
      qs.append('limit', limit.toString());
      return apiClient.get<ListVoiceCallsResult>(`/api/admin/voice/calls?${qs.toString()}`);
    }
  },

  async delete(callID: number | string): Promise<void> {
    await apiClient.delete(`/api/admin/voice/calls/${callID}`).catch(() => {});
  },

  async endCall(sessionID: string, durationSeconds: number = 0): Promise<void> {
    await hangupCall(sessionID, durationSeconds).catch(async () => {
      await fetch(`${API_BASE}/api/voice/end`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session_id: sessionID,
          duration_seconds: durationSeconds,
        }),
      });
    });
  },

  async declineCall(sessionID: string): Promise<void> {
    await rejectCall(sessionID, 'agent').catch(async () => {
      await fetch(`${API_BASE}/api/voice/decline`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_id: sessionID }),
      });
    });
  },
};

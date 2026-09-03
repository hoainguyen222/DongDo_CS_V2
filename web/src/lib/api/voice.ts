// ============================================================
// Voice Calls API — WebRTC call history
// ============================================================

import { apiClient, API_BASE } from './client';

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
    const { sessionId, page = 1, limit = 10 } = params;
    const qs = new URLSearchParams();
    if (sessionId) qs.append('session_id', sessionId);
    qs.append('page', page.toString());
    qs.append('limit', limit.toString());
    return apiClient.get<ListVoiceCallsResult>(`/api/admin/voice/calls?${qs.toString()}`);
  },

  async delete(callID: number): Promise<void> {
    await apiClient.delete(`/api/admin/voice/calls/${callID}`);
  },

  async endCall(sessionID: string, durationSeconds: number = 0): Promise<void> {
    const res = await fetch(`${API_BASE}/api/voice/end`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: sessionID,
        duration_seconds: durationSeconds,
      }),
    });
    if (!res.ok) throw new Error('Không thể kết thúc cuộc gọi');
  },

  async declineCall(sessionID: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/voice/decline`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: sessionID }),
    });
    if (!res.ok) throw new Error('Không thể từ chối cuộc gọi');
  },
};

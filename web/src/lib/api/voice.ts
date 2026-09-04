// ============================================================
// Voice Calls API — WebRTC call history
// ============================================================

import { apiClient, API_BASE } from './client';

export interface InitiateCallParams {
  sessionId: string;
  callerType: 'guest' | 'cskh';
  callerId: string;
  calleeType: 'guest' | 'cskh';
  calleeId?: string;
}

export interface VoiceCallResponse {
  id: number;
  session_id: string;
  caller_type: string;
  caller_id: string;
  callee_type: string;
  callee_id: string;
  status: string;
  duration_seconds: number;
  recording_url?: string;
  transcript?: string;
  created_at: string;
  ended_at?: string;
}

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

export interface TeamAgentGuestCallNotification {
  session_id: string;
  guest_name: string;
  guest_id: string;
  call_id?: number;
  timestamp: string;
}

export const voiceApi = {
  async initiateCall(params: InitiateCallParams): Promise<VoiceCallResponse> {
    const res = await fetch(`${API_BASE}/api/voice/initiate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: params.sessionId,
        caller_type: params.callerType,
        caller_id: params.callerId,
        callee_type: params.calleeType,
        callee_id: params.calleeId || '',
      }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ detail: 'Không thể khởi tạo cuộc gọi' }));
      throw new Error(err.detail || 'Không thể khởi tạo cuộc gọi');
    }
    return res.json();
  },

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

  async markMissed(callID: number, sessionID: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/voice/missed`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ call_id: callID, session_id: sessionID }),
    });
    if (!res.ok) throw new Error('Không thể đánh dấu cuộc gọi nhỡ');
  },
};

// ============================================================
// Voice Calls API — Click-to-call model
// ============================================================
// Backend owns media (Asterisk). Frontend only:
//   - initiateCall: create a call record (status=RINGING) so backend
//     can dial both guest + agent and bridge them.
//   - getCallStatus: poll the DB for status transitions
//     (RINGING -> ACTIVE -> ENDED).
//   - endCall: mark call ENDED + duration.
//   - acceptCall: agent clicks "Nghe máy" — backend forwards to
//     Asterisk so the agent's SIP client is bridged.
//   - declineCall / markMissed: bookkeeping endpoints.
// ============================================================

import { apiClient, API_BASE } from './client';

export interface InitiateCallParams {
  sessionId: string;
  callerType: 'guest' | 'cskh';
  callerId: string;
  calleeType: 'guest' | 'cskh';
  calleeId?: string;
  /**
   * E.164 or local VN phone number to dial back. Required when the
   * click-to-call flow has the guest entering their phone number
   * so Asterisk knows which PSTN endpoint to bridge.
   */
  phoneNumber?: string;
  /**
   * Optional scheduled timestamp (ISO 8601) for "Đặt lịch gọi lại".
   * When omitted, backend treats it as an immediate "Gọi ngay".
   */
  scheduledAt?: string;
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
  phone_number?: string;
  scheduled_at?: string;
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
    return apiClient.post<VoiceCallResponse>(`/api/voice/initiate`, {
      session_id: params.sessionId,
      caller_type: params.callerType,
      caller_id: params.callerId,
      callee_type: params.calleeType,
      callee_id: params.calleeId || '',
      phone_number: params.phoneNumber,
      scheduled_at: params.scheduledAt,
    });
  },

  /**
   * Poll the backend for the current status of a call. The frontend
   * uses this to drive its status display ("Đang kết nối" →
   * "Đang đổ chuông" → "Đã kết nối" → "Kết thúc") because real-time
   * updates are also pushed over WS, but polling is a safe fallback.
   */
  async getCallStatus(callId: number): Promise<VoiceCallResponse> {
    if (!callId || Number.isNaN(callId)) {
      throw new Error('callId không hợp lệ');
    }
    return apiClient.get<VoiceCallResponse>(`/api/voice/status/${callId}`);
  },

  /**
   * Agent clicks "Nghe máy". Backend signals Asterisk to bridge the
   * agent's SIP endpoint into the active call channel.
   */
  /**
   * Agent clicks "Nghe máy". Backend signals Asterisk to bridge the
   * agent's SIP endpoint into the active call channel.
   *
   * `agentExtension` is the SIP extension registered in Asterisk
   * (e.g. "1001") — must match the agent provisioned via
   * scripts/register-agent.sh.
   */
  async acceptCall(callId: number, sessionId: string, agentExtension?: string): Promise<VoiceCallResponse> {
    return apiClient.post<VoiceCallResponse>(`/api/voice/accept/${callId}`, {
      session_id: sessionId,
      agent_extension: agentExtension || '',
    });
  },

  /**
   * WebRTC-only accept: drives Asterisk's Stasis app (ARI) to
   * originate the agent leg and bridge it with the guest leg.
   * Audio is carried by sip.js → Asterisk → guest, all native.
   */
  async webrtcAcceptCall(callId: number, sessionId: string, agentExtension?: string): Promise<VoiceCallResponse> {
    return apiClient.post<VoiceCallResponse>(`/api/voice/webrtc-accept/${callId}`, {
      session_id: sessionId,
      agent_extension: agentExtension || '',
    });
  },

  /**
   * Fetch the Asterisk SIP credentials (websocket URL, AOR, password)
   * for the currently-authenticated user so the browser can register
   * the agent's extension via sip.js (WSS).  Returns null on 404 (user
   * has no SIP extension assigned).
   */
  async getMySipConfig(): Promise<{
    websocket_url: string;
    sip_server: string;
    agent_extension: string;
    agent_password: string;
    stun_servers: string[];
    display_name: string;
  } | null> {
    try {
      return await apiClient.get(`/api/me/sip-config`);
    } catch (err) {
      // 404 = no extension assigned — fall back to no-softphone mode.
      if (err instanceof Error && (err as any).status === 404) return null;
      // Network errors / 5xx: log + degrade to null so the UI still works.
      console.warn('getMySipConfig failed', err);
      return null;
    }
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

  async endCall(sessionID: string, callId?: number, durationSeconds: number = 0): Promise<void> {
    await apiClient.post(`/api/voice/end`, {
      session_id: sessionID,
      call_id: callId || undefined,
      duration_seconds: durationSeconds,
    });
  },

  async declineCall(sessionID: string, callId?: number): Promise<void> {
    await apiClient.post(`/api/voice/decline`, {
      session_id: sessionID,
      call_id: callId || undefined,
    });
  },

  async markMissed(callID: number, sessionID: string): Promise<void> {
    await apiClient.post(`/api/voice/missed`, {
      call_id: callID,
      session_id: sessionID,
    });
  },
};

// ============================================================
// Guest API — Register, chat history, send message
// ============================================================

import { apiClient, API_BASE } from './client';
import type { GuestSession, Message } from '@/lib/types';

export const guestApi = {
  async register(displayName: string, phone?: string): Promise<GuestSession> {
    const res = await fetch(`${API_BASE}/guest/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ display_name: displayName, phone: phone || '' }),
    });
    if (!res.ok) throw new Error('Không thể tạo phiên khách hàng');
    return res.json();
  },

  async sendMessage(
    sessionID: string,
    customerName: string,
    message: string,
    clientMsgID?: string
  ): Promise<any> {
    return apiClient.post('/chat', {
      session_id: sessionID,
      customer_name: customerName,
      message,
      client_msg_id: clientMsgID,
    });
  },

  async getHistory(
    sessionID: string
  ): Promise<{ session_id: string; status: string; assigned_cs: string; messages: Message[] }> {
    return apiClient.get(`/history/${sessionID}`);
  },

  async sendTyping(sessionID: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/chat/typing`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: sessionID }),
    });
    if (!res.ok) throw new Error('Không thể gửi trạng thái đang nhập');
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

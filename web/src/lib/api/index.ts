// ============================================================
// API Index — Barrel exports
// ============================================================

export { apiClient, ApiError, API_BASE, getAuthToken, setAuthToken, setAuthUser } from './client';

export { authApi } from './auth';
export { guestApi } from './guest';
export { casesApi } from './cases';
export { customersApi } from './customers';
export { voiceApi } from './voice';
export { learningApi } from './learning';
export { knowledgeApi } from './knowledge';
export { analyticsApi } from './analytics';
export { configApi, systemApi } from './system';
export { usersApi, permissionsApi } from './users';
export { tagsApi } from './tags';

// Call v2 — agent availability helpers. Mounted by useAgentHeartbeat so
// authenticated staff keep their Redis AVAILABLE state fresh without any
// user action.
import { apiClient, API_BASE } from './client';
export const callApi = {
  /** POST /api/calls — enqueue a call into the Redis-backed routing pool. */
  async requestCall(params: { customerId: string; priority?: number; idempotencyKey?: string }): Promise<{ call_id: string; status: string; queue_position?: number; replay?: boolean }> {
    const headers: Record<string, string> = {};
    if (params.idempotencyKey) headers['Idempotency-Key'] = params.idempotencyKey;
    const res = await fetch(`${API_BASE}/api/calls`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...headers },
      body: JSON.stringify({
        customer_id: params.customerId,
        priority: params.priority ?? 0,
      }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ detail: 'Không thể tạo cuộc gọi' }));
      throw new Error(err.detail || 'Không thể tạo cuộc gọi');
    }
    return res.json();
  },

  async heartbeat(username: string): Promise<void> {
    await apiClient.post(`/api/agents/${encodeURIComponent(username)}/heartbeat`);
  },

  /**
   * GET /api/agents/:id/active-calls — recover the ringing banner after
   * a page reload. The WS broadcast for `incoming_call` is fire-and-forget;
   * if the admin was offline when it fired, the banner never appears. This
   * endpoint returns the calls still assigned to the agent in any
   * non-terminal state so the layout can re-create the banner.
   */
  async getActiveCalls(username: string): Promise<{ calls: Array<{ id: string; customer_id: string; agent_id?: string; status: string; priority?: number }> }> {
    const res = await fetch(`${API_BASE}/api/agents/${encodeURIComponent(username)}/active-calls`);
    if (!res.ok) {
      return { calls: [] };
    }
    return res.json();
  },

  /** POST /api/calls/:id/accept — agent accepts a queued call (Call v2). */
  async acceptCall(callId: string): Promise<void> {
    // Use apiClient (which injects the Authorization header) instead of
    // raw fetch — the raw fetch variant was returning 401 "Vui lòng đăng
    // nhập" because the Authorization header was missing.
    await apiClient.post(`/api/calls/${encodeURIComponent(callId)}/accept`, {});
  },

  /** POST /api/calls/:id/reject — agent rejects a queued call (Call v2). */
  async rejectCall(callId: string): Promise<void> {
    await apiClient.post(`/api/calls/${encodeURIComponent(callId)}/reject`, {});
  },

  /** POST /api/calls/:id/hangup — agent or customer ends the call. */
  async hangupCall(callId: string): Promise<void> {
    await apiClient.post(`/api/calls/${encodeURIComponent(callId)}/hangup`, {});
  },

  /**
   * POST /api/calls/:id/cancel — customer cancels while still in queue
   * (Call v2). The customer_id is sent via the X-Customer-ID header so
   * the backend can authorize the request without requiring a Bearer
   * token (guests do not have one).
   */
  async cancelCall(callId: string, customerId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/calls/${encodeURIComponent(callId)}/cancel`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Customer-ID': customerId,
      },
    });
    if (!res.ok && res.status !== 404) {
      const err = await res.json().catch(() => ({ detail: 'Không thể hủy cuộc gọi' }));
      throw new Error(err.detail || 'Không thể hủy cuộc gọi');
    }
  },
};

import { authApi } from './auth';
import { guestApi } from './guest';
import { casesApi } from './cases';
import { customersApi } from './customers';
import { voiceApi } from './voice';
import { learningApi } from './learning';
import { knowledgeApi } from './knowledge';
import { analyticsApi } from './analytics';
import { configApi, systemApi } from './system';
import { usersApi, permissionsApi } from './users';
import { tagsApi } from './tags';
import type { ListCasesResult } from './cases';
import type { ListCustomersResult } from './customers';
import type { ListVoiceCallsResult } from './voice';
import type { ListLearningResult } from './learning';

// Legacy-compatible `api` object — wraps named APIs for backward compatibility.
// Prefer importing named APIs directly for better tree-shaking.
export const api = {
  // Auth
  login: authApi.login,
  logout: authApi.logout,
  getMe: authApi.getMe,

  // Guest & Chat
  registerGuest: guestApi.register,
  sendMessage: guestApi.sendMessage,
  getHistory: guestApi.getHistory,
  getCaseDetail: guestApi.getHistory,

  // Cases
  listCases: (
    statusOrParams?: string | { status?: string; page?: number; limit?: number; search?: string },
    page = 1,
    limit = 10,
    search?: string
  ): Promise<ListCasesResult> => {
    // Support both old positional signature and new object signature
    if (typeof statusOrParams === 'object' && statusOrParams !== null) {
      return casesApi.list(statusOrParams);
    }
    return casesApi.list({
      status: statusOrParams as string | undefined,
      page,
      limit,
      search,
    });
  },
  getCases: (status?: string, page = 1, limit = 10, search?: string) =>
    casesApi.list({ status, page, limit, search }).then((d) => d.cases),
  takeCase: casesApi.take,
  replyCase: casesApi.reply,
  sendCSMessage: casesApi.sendCSMessage,
  resolveCase: casesApi.resolve,
  deleteCase: casesApi.delete,
  updateCustomerInfo: casesApi.updateCustomerInfo,
  clearAllCases: casesApi.clearAll,

  // Customers
  getCustomers: (
    pageOrParams?: number | { page?: number; limit?: number; search?: string },
    limit?: number,
    search?: string
  ): Promise<ListCustomersResult> => {
    if (typeof pageOrParams === 'object' && pageOrParams !== null) {
      return customersApi.list(pageOrParams);
    }
    return customersApi.list({ page: pageOrParams, limit, search });
  },
  updateCustomer: customersApi.update,
  deleteCustomer: customersApi.delete,

  // Voice
  getVoiceCalls: (
    sessionIdOrParams?: string | { sessionId?: string; page?: number; limit?: number },
    page?: number,
    limit?: number
  ): Promise<ListVoiceCallsResult> => {
    if (typeof sessionIdOrParams === 'object' && sessionIdOrParams !== null) {
      return voiceApi.list(sessionIdOrParams);
    }
    return voiceApi.list({ sessionId: sessionIdOrParams as string | undefined, page, limit });
  },
  deleteVoiceCall: voiceApi.delete,
  endCall: voiceApi.endCall,
  declineCall: voiceApi.declineCall,
  markMissedCall: voiceApi.markMissed,

  // Learning
  listPendingLearning: (
    pageOrParams?: number | { page?: number; limit?: number },
    limit?: number
  ): Promise<ListLearningResult> => {
    if (typeof pageOrParams === 'object' && pageOrParams !== null) {
      return learningApi.listPending(pageOrParams);
    }
    return learningApi.listPending({ page: pageOrParams, limit });
  },
  getPendingLearning: (page = 1, limit = 10) =>
    learningApi.listPending({ page, limit }).then((d) => d.pending_items),
  approveLearning: learningApi.approve,
  rejectLearning: learningApi.reject,
  updateLearningItem: learningApi.updateItem,
  getLearningSettings: learningApi.getSettings,
  setLearningSettings: learningApi.setSettings,
  updateLearningSettings: learningApi.setSettings,
  resetLearnedKnowledge: learningApi.resetAll,
  resetAllLearning: learningApi.resetAll,

  // Knowledge
  getKnowledgeOverview: knowledgeApi.overview,
  getKnowledge: knowledgeApi.overview,
  uploadDocument: knowledgeApi.upload,
  deleteKnowledgeDocument: knowledgeApi.deleteDocument,

  // Analytics
  getAnalytics: analyticsApi.getStats,
  getGeneralOverviewReport: (startDate?: string, endDate?: string) =>
    analyticsApi.getGeneralOverview({ startDate, endDate }),
  getAIPerformanceReport: (startDate?: string, endDate?: string) =>
    analyticsApi.getAIPerformance({ startDate, endDate }),
  getStaffPerformanceReport: (startDate?: string, endDate?: string) =>
    analyticsApi.getStaffPerformance({ startDate, endDate }),
  getCXReport: (startDate?: string, endDate?: string) =>
    analyticsApi.getCX({ startDate, endDate }),
  getOperationalReport: (startDate?: string, endDate?: string) =>
    analyticsApi.getOperational({ startDate, endDate }),
  getIssueAnalysisReport: (startDate?: string, endDate?: string) =>
    analyticsApi.getIssueAnalysis({ startDate, endDate }),
  getAILearningReportStats: analyticsApi.getAILearningStats,

  // Config
  getConfig: configApi.get,
  saveConfig: configApi.save,
  updateConfig: configApi.save,

  // Users
  listUsers: usersApi.list,
  createUser: usersApi.create,
  updateUser: usersApi.update,
  deleteUser: usersApi.delete,

  // RBAC
  listRolePermissions: permissionsApi.list,
  upsertRolePermission: permissionsApi.upsert,

  // System errors
  listSystemErrors: systemApi.listErrors,
  createSystemError: systemApi.createError,
  markSystemErrorHandled: systemApi.markErrorHandled,

  // Call v2 — agent heartbeat (keeps AVAILABLE pool state fresh).
  agentHeartbeat: callApi.heartbeat,
  // Call v2 — guest-facing queue entry point.
  requestCall: callApi.requestCall,
  // Call v2 — agent accepts / rejects a queued call.
  acceptCall: callApi.acceptCall,
  rejectCall: callApi.rejectCall,
  hangupCall: callApi.hangupCall,
  cancelCall: callApi.cancelCall,
  // Call v2 — recover active calls on WS reconnect.
  getAgentActiveCalls: callApi.getActiveCalls,
  // Call v2 — paginated call history for the admin history view.
  // Uses /api/admin/calls (Call v2 calls table) which captures all
  // post-v2 traffic; the legacy /api/admin/voice/calls is kept separate.
  listCalls: async (page: number = 1, limit: number = 20) =>
    apiClient.get<{ calls: any[]; page: number; limit: number }>(`/api/admin/calls?page=${page}&limit=${limit}`),
};

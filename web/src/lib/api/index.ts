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
};

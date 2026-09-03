/**
 * TanStack Query Hooks — API wrappers with caching, polling, and mutations
 *
 * Quy ước:
 * - Query keys: ['entity', ...params]
 * - useQuery cho GET
 * - useMutation cho POST/PUT/DELETE
 * - Stale time mặc định: 30s (tăng lên nếu data ít thay đổi)
 */

import {
  useQuery,
  useMutation,
  useQueryClient,
  type UseQueryOptions,
  type UseMutationOptions,
} from '@tanstack/react-query';
import { api } from '@/lib/api';
import type {
  GuestSession,
  ChatCase,
  CustomerProfile,
  Message,
  LearningItem,
  KnowledgeOverview,
  AnalyticsStats,
  SystemConfig,
  QAPair,
  SystemErrorItem,
  RolePermissionItem,
  PermissionLevel,
  SessionUser,
} from '@/lib/types';

// ============================================================
// Query Key Factories
// ============================================================
export const queryKeys = {
  // Auth
  me: ['auth', 'me'] as const,

  // Cases
  cases: (status?: string, page?: number, limit?: number, search?: string) =>
    ['cases', { status, page, limit, search }] as const,
  caseDetail: (sessionId: string) => ['caseDetail', sessionId] as const,

  // Customers
  customers: (page?: number, limit?: number, search?: string) =>
    ['customers', { page, limit, search }] as const,

  // Voice calls
  voiceCalls: (sessionId?: string, page?: number, limit?: number) =>
    ['voiceCalls', { sessionId, page, limit }] as const,

  // Learning
  pendingLearning: (page?: number, limit?: number) =>
    ['pendingLearning', { page, limit }] as const,
  learningSettings: ['learningSettings'] as const,
  knowledge: ['knowledge'] as const,

  // Analytics & Config
  analytics: ['analytics'] as const,
  config: ['config'] as const,

  // RBAC
  rolePermissions: ['rolePermissions'] as const,
  users: ['users'] as const,

  // System errors
  systemErrors: ['systemErrors'] as const,

  // Chat (guest)
  chatHistory: (sessionId: string) => ['chatHistory', sessionId] as const,
};

// ============================================================
// Generic fetch wrapper
// ============================================================
type QueryFn<T> = () => Promise<T>;
type QueryOpts<T> = Omit<UseQueryOptions<T, Error>, 'queryKey' | 'queryFn'>;

// ============================================================
// ─── AUTH ───────────────────────────────────────────────────

export function useMe(options?: QueryOpts<SessionUser>) {
  return useQuery({
    queryKey: queryKeys.me,
    queryFn: () => api.getMe(),
    staleTime: 1000 * 60 * 5, // 5 phút
    retry: false,
    ...options,
  });
}

// ============================================================
// ─── CASES ──────────────────────────────────────────────────

export function useCases(
  status?: string,
  page = 1,
  limit = 10,
  search?: string,
  options?: QueryOpts<{ cases: ChatCase[]; total: number }>
) {
  return useQuery({
    queryKey: queryKeys.cases(status, page, limit, search),
    queryFn: () => api.listCases(status, page, limit, search),
    staleTime: 30_000, // 30s; refreshed via WS case_update event
    ...options,
  });
}

export function useCaseDetail(sessionId: string, options?: QueryOpts<{ session_id: string; messages: Message[] }>) {
  // Only enable query when sessionId is a non-empty string
  const enabled = Boolean(sessionId) && sessionId.length > 0;
  return useQuery({
    queryKey: queryKeys.caseDetail(sessionId),
    queryFn: () => api.getCaseDetail(sessionId),
    staleTime: 5_000,
    enabled,
    ...options,
  });
}

export function useTakeCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) => api.takeCase(sessionId),
    onSuccess: (_data, sessionId) => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: queryKeys.caseDetail(sessionId) });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

export function useResolveCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      sessionId,
      resolutionNote,
      extractPairs,
    }: {
      sessionId: string;
      resolutionNote: string;
      extractPairs?: QAPair[];
    }) => api.resolveCase(sessionId, resolutionNote, extractPairs),
    onSuccess: (_data, { sessionId }) => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: queryKeys.caseDetail(sessionId) });
      qc.invalidateQueries({ queryKey: ['pendingLearning'] });
      qc.invalidateQueries({ queryKey: ['customers'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

export function useDeleteCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) => api.deleteCase(sessionId),
    onSuccess: (_data, sessionId) => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: queryKeys.caseDetail(sessionId) });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

export function useClearAllCases() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.clearAllCases(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

// ============================================================
// ─── CUSTOMERS ──────────────────────────────────────────────

export function useCustomers(
  page = 1,
  limit = 10,
  search?: string,
  options?: QueryOpts<{ customers: CustomerProfile[]; total: number }>
) {
  return useQuery({
    queryKey: queryKeys.customers(page, limit, search),
    queryFn: () => api.getCustomers(page, limit, search),
    staleTime: 15_000,
    ...options,
  });
}

export function useUpdateCustomer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      guestId,
      name,
      phone,
    }: {
      guestId: string;
      name: string;
      phone: string;
    }) => api.updateCustomer(guestId, name, phone),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['customers'] });
      qc.invalidateQueries({ queryKey: ['cases'] });
    },
  });
}

export function useDeleteCustomer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (guestId: string) => api.deleteCustomer(guestId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['customers'] });
    },
  });
}

// ============================================================
// ─── VOICE CALLS ────────────────────────────────────────────

export function useVoiceCalls(
  sessionId?: string,
  page = 1,
  limit = 10,
  options?: QueryOpts<{ calls: any[]; total: number }>
) {
  return useQuery({
    queryKey: queryKeys.voiceCalls(sessionId, page, limit),
    queryFn: () => api.getVoiceCalls(sessionId, page, limit),
    staleTime: 10_000,
    ...options,
  });
}

export function useDeleteVoiceCall() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (callId: number) => api.deleteVoiceCall(callId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['voiceCalls'] });
    },
  });
}

// ============================================================
// ─── LEARNING ───────────────────────────────────────────────

export function usePendingLearning(
  page = 1,
  limit = 10,
  options?: QueryOpts<{ pending_items: LearningItem[]; total: number }>
) {
  return useQuery({
    queryKey: queryKeys.pendingLearning(page, limit),
    queryFn: () => api.listPendingLearning(page, limit),
    staleTime: 30_000, // 30s; refreshed via WS learning_update event
    ...options,
  });
}

export function useApproveLearning() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      itemId,
      question,
      answer,
    }: {
      itemId: number;
      question?: string;
      answer?: string;
    }) => api.approveLearning(itemId, question, answer),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pendingLearning'] });
      qc.invalidateQueries({ queryKey: ['knowledge'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

export function useRejectLearning() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (itemId: number) => api.rejectLearning(itemId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pendingLearning'] });
    },
  });
}

export function useUpdateLearningItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      itemId,
      question,
      answer,
    }: {
      itemId: number;
      question: string;
      answer: string;
    }) => api.updateLearningItem(itemId, question, answer),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pendingLearning'] });
    },
  });
}

export function useLearningSettings(options?: QueryOpts<{ auto_learning_enabled: boolean }>) {
  return useQuery({
    queryKey: queryKeys.learningSettings,
    queryFn: () => api.getLearningSettings(),
    staleTime: 30_000,
    ...options,
  });
}

export function useUpdateLearningSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) => api.updateLearningSettings(enabled),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['learningSettings'] });
    },
  });
}

export function useResetAllLearning() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.resetAllLearning(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pendingLearning'] });
      qc.invalidateQueries({ queryKey: ['knowledge'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

// ============================================================
// ─── KNOWLEDGE ──────────────────────────────────────────────

export function useKnowledge(options?: QueryOpts<KnowledgeOverview>) {
  return useQuery({
    queryKey: queryKeys.knowledge,
    queryFn: () => api.getKnowledge(),
    staleTime: 30_000,
    ...options,
  });
}

export function useUploadDocument() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (file: File) => api.uploadDocument(file),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['knowledge'] });
    },
  });
}

// ============================================================
// ─── ANALYTICS ─────────────────────────────────────────────

export function useAnalytics(options?: QueryOpts<AnalyticsStats>) {
  return useQuery({
    queryKey: queryKeys.analytics,
    queryFn: () => api.getAnalytics(),
    staleTime: 30_000,
    refetchInterval: 60_000, // 1 phút
    ...options,
  });
}

// ============================================================
// ─── CONFIG ─────────────────────────────────────────────────

export function useConfig(options?: QueryOpts<SystemConfig>) {
  return useQuery({
    queryKey: queryKeys.config,
    queryFn: () => api.getConfig(),
    staleTime: 60_000, // 1 phút
    ...options,
  });
}

export function useUpdateConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (config: SystemConfig) => api.updateConfig(config),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['config'] });
    },
  });
}

// ============================================================
// ─── RBAC / USERS ──────────────────────────────────────────

export function useRolePermissions(options?: QueryOpts<RolePermissionItem[]>) {
  return useQuery({
    queryKey: queryKeys.rolePermissions,
    queryFn: () => api.listRolePermissions(),
    staleTime: 60_000,
    ...options,
  });
}

export function useUpsertRolePermission() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      roleName,
      featureKey,
      permissionLevel,
    }: {
      roleName: string;
      featureKey: string;
      permissionLevel: PermissionLevel;
    }) => api.upsertRolePermission(roleName, featureKey, permissionLevel),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['rolePermissions'] });
    },
  });
}

export function useUsers(options?: QueryOpts<any[]>) {
  return useQuery({
    queryKey: queryKeys.users,
    queryFn: () => api.listUsers(),
    staleTime: 30_000,
    ...options,
  });
}

export function useCreateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      fullName,
      email,
      role,
      password,
    }: {
      fullName: string;
      email: string;
      role: string;
      password?: string;
    }) => api.createUser(fullName, email, role, password),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] });
    },
  });
}

export function useUpdateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      username,
      data,
    }: {
      username: string;
      data: { fullName: string; role: string; isActive: boolean; password?: string };
    }) => api.updateUser(username, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] });
    },
  });
}

export function useDeleteUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (username: string) => api.deleteUser(username),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] });
    },
  });
}

// ============================================================
// ─── SYSTEM ERRORS ──────────────────────────────────────────

export function useSystemErrors(options?: QueryOpts<SystemErrorItem[]>) {
  return useQuery({
    queryKey: queryKeys.systemErrors,
    queryFn: () => api.listSystemErrors(),
    staleTime: 10_000,
    ...options,
  });
}

export function useCreateSystemError() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (error: SystemErrorItem) => api.createSystemError(error),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['systemErrors'] });
    },
  });
}

export function useMarkSystemErrorHandled() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.markSystemErrorHandled(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['systemErrors'] });
    },
  });
}

// ============================================================
// ─── GUEST CHAT ────────────────────────────────────────────

export function useChatHistory(
  sessionId: string,
  options?: QueryOpts<{ session_id: string; messages: Message[] }>
) {
  return useQuery({
    queryKey: queryKeys.chatHistory(sessionId),
    queryFn: () => api.getHistory(sessionId),
    staleTime: 5_000,
    enabled: !!sessionId,
    ...options,
  });
}

// ============================================================
// ─── AUTH MUTATIONS ────────────────────────────────────────

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      username,
      password,
    }: {
      username: string;
      password: string;
    }) => api.login(username, password),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: ['customers'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
      qc.invalidateQueries({ queryKey: ['rolePermissions'] });
      qc.invalidateQueries({ queryKey: ['users'] });
    },
  });
}

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.logout(),
    onSuccess: () => {
      // Nuke all per-user caches on sign-out
      qc.clear();
    },
  });
}

// ============================================================
// ─── GUEST CHAT ────────────────────────────────────────────

export function useRegisterGuest() {
  return useMutation({
    mutationFn: ({
      displayName,
      phone,
    }: {
      displayName: string;
      phone?: string;
    }) => api.registerGuest(displayName, phone),
  });
}

export function useSendMessage(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      customerName,
      message,
      clientMsgId,
    }: {
      customerName: string;
      message: string;
      clientMsgId?: string;
    }) => api.sendMessage(sessionId, customerName, message, clientMsgId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.chatHistory(sessionId) });
    },
  });
}

export function useReplyCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      sessionId,
      message,
    }: {
      sessionId: string;
      message: string;
    }) => api.replyCase(sessionId, message),
    onSuccess: (_data, { sessionId }) => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: queryKeys.caseDetail(sessionId) });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

// ============================================================
// ─── ENHANCED INVALIDATIONS ────────────────────────────────

export function useTakeCaseEnhanced() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) => api.takeCase(sessionId),
    onSuccess: (_data, sessionId) => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: queryKeys.caseDetail(sessionId) });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

export function useResolveCaseEnhanced() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      sessionId,
      resolutionNote,
      extractPairs,
    }: {
      sessionId: string;
      resolutionNote: string;
      extractPairs?: QAPair[];
    }) => api.resolveCase(sessionId, resolutionNote, extractPairs),
    onSuccess: (_data, { sessionId }) => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: queryKeys.caseDetail(sessionId) });
      qc.invalidateQueries({ queryKey: ['pendingLearning'] });
      qc.invalidateQueries({ queryKey: ['customers'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

export function useDeleteCaseEnhanced() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) => api.deleteCase(sessionId),
    onSuccess: (_data, sessionId) => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: queryKeys.caseDetail(sessionId) });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

export function useClearAllCasesEnhanced() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.clearAllCases(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

export function useApproveLearningEnhanced() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      itemId,
      question,
      answer,
    }: {
      itemId: number;
      question?: string;
      answer?: string;
    }) => api.approveLearning(itemId, question, answer),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pendingLearning'] });
      qc.invalidateQueries({ queryKey: ['knowledge'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

export function useResetAllLearningEnhanced() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.resetAllLearning(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pendingLearning'] });
      qc.invalidateQueries({ queryKey: ['knowledge'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    },
  });
}

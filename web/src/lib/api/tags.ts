// ============================================================
// Chat Tag & Alert Config API Client
// ============================================================

import { apiClient } from './client';
import { ChatTag, CaseTag, AlertConfig } from '@/lib/types';

export const tagsApi = {
  // --- Tag CRUD ---
  listTags: async (): Promise<ChatTag[]> => {
    const res = await apiClient.get<{ tags: ChatTag[] }>('/api/admin/chat/tags');
    return res?.tags || [];
  },

  createTag: async (data: { name: string; description?: string; color: string }): Promise<ChatTag> => {
    return apiClient.post<ChatTag>('/api/admin/chat/tags', data);
  },

  updateTag: async (id: number, data: { name: string; description?: string; color: string }): Promise<{ status: string; message: string }> => {
    return apiClient.put<{ status: string; message: string }>(`/api/admin/chat/tags/${id}`, data);
  },

  deleteTag: async (id: number): Promise<{ status: string; message: string }> => {
    return apiClient.delete<{ status: string; message: string }>(`/api/admin/chat/tags/${id}`);
  },

  // --- Case Tags ---
  getCaseTags: async (sessionId: string): Promise<CaseTag[]> => {
    const res = await apiClient.get<{ tags: CaseTag[] }>(`/api/admin/cases/${encodeURIComponent(sessionId)}/tags`);
    return res?.tags || [];
  },

  attachTag: async (sessionId: string, tagId: number): Promise<{ status: string; message: string }> => {
    return apiClient.post<{ status: string; message: string }>(`/api/admin/cases/${encodeURIComponent(sessionId)}/tags`, { tag_id: tagId });
  },

  detachTag: async (sessionId: string, tagId: number): Promise<{ status: string; message: string }> => {
    return apiClient.delete<{ status: string; message: string }>(`/api/admin/cases/${encodeURIComponent(sessionId)}/tags/${tagId}`);
  },

  // --- Alert Config ---
  getAlertConfig: async (): Promise<AlertConfig> => {
    return apiClient.get<AlertConfig>('/api/admin/chat/alert-config');
  },

  saveAlertConfig: async (data: { is_enabled: boolean; timeout_seconds: number; alert_content: string }): Promise<AlertConfig> => {
    return apiClient.post<AlertConfig>('/api/admin/chat/alert-config', data);
  },

  resolveAlertEvent: async (sessionId: string): Promise<{ status: string }> => {
    return apiClient.post<{ status: string }>(`/api/admin/chat/alert-events/${encodeURIComponent(sessionId)}/resolve`, {});
  },
};

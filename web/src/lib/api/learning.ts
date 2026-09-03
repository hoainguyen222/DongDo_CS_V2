// ============================================================
// Learning API — Knowledge queue management
// ============================================================

import { apiClient } from './client';
import type { LearningItem } from '@/lib/types';

export interface ListLearningParams {
  page?: number;
  limit?: number;
}

export interface ListLearningResult {
  pending_items: LearningItem[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export const learningApi = {
  async listPending(params: ListLearningParams = {}): Promise<ListLearningResult> {
    const { page = 1, limit = 10 } = params;
    const qs = new URLSearchParams();
    qs.append('page', page.toString());
    qs.append('limit', limit.toString());
    return apiClient.get<ListLearningResult>(`/api/admin/learning/pending?${qs.toString()}`);
  },

  async approve(itemID: number, question?: string, answer?: string): Promise<{ success: boolean; message: string }> {
    return apiClient.post(`/api/admin/learning/approve/${itemID}`, {
      question: question || '',
      answer: answer || '',
    });
  },

  async reject(itemID: number): Promise<void> {
    await apiClient.post(`/api/admin/learning/reject/${itemID}`);
  },

  async updateItem(itemID: number, question: string, answer: string): Promise<void> {
    await apiClient.put(`/api/admin/learning/${itemID}`, { question, answer });
  },

  async getSettings(): Promise<{ auto_learning_enabled: boolean }> {
    return apiClient.get<{ auto_learning_enabled: boolean }>('/api/admin/learning/settings');
  },

  async setSettings(enabled: boolean): Promise<void> {
    await apiClient.post('/api/admin/learning/settings', { auto_learning_enabled: enabled });
  },

  async resetAll(): Promise<{ deleted_count: number }> {
    return apiClient.post('/api/admin/learning/reset');
  },
};

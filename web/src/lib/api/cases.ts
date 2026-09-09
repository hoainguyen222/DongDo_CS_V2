// ============================================================
// Cases API — CS Studio inbox operations
// ============================================================

import { apiClient } from './client';
import type { ChatCase, CaseStatusCounts, Message, QAPair } from '@/lib/types';

export interface ListCasesParams {
  status?: string;
  page?: number;
  limit?: number;
  search?: string;
}

export interface ListCasesResult {
  cases: ChatCase[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
  status_counts?: CaseStatusCounts;
}

export const casesApi = {
  async list(params: ListCasesParams = {}): Promise<ListCasesResult> {
    const { status, page = 1, limit = 10, search } = params;
    const qs = new URLSearchParams();
    if (status) qs.append('status', status);
    qs.append('page', page.toString());
    qs.append('limit', limit.toString());
    if (search) qs.append('search', search);
    return apiClient.get<ListCasesResult>(`/api/admin/cases?${qs.toString()}`);
  },

  async getDetail(sessionID: string): Promise<{ session_id: string; status: string; assigned_cs: string; messages: Message[] }> {
    return apiClient.get(`/history/${sessionID}`);
  },

  async take(sessionID: string): Promise<void> {
    await apiClient.post(`/api/admin/cases/${sessionID}/take`);
  },

  async resumeAI(sessionID: string): Promise<{ success: boolean; message: string }> {
    return apiClient.post<{ success: boolean; message: string }>(`/api/admin/cases/${sessionID}/resume-ai`);
  },

  async reply(sessionID: string, message: string): Promise<void> {
    await apiClient.post(`/api/admin/cases/${sessionID}/reply`, { message });
  },

  async sendCSMessage(sessionID: string, message: string): Promise<void> {
    return casesApi.reply(sessionID, message);
  },

  async resolve(
    sessionID: string,
    resolutionNote: string,
    extractPairs?: QAPair[]
  ): Promise<{ success: boolean; auto_learned: boolean; learned_count: number; message: string }> {
    return apiClient.post(`/api/admin/cases/${sessionID}/resolve`, {
      resolution_note: resolutionNote,
      extract_pairs: extractPairs || [],
    });
  },

  async delete(sessionID: string): Promise<void> {
    await apiClient.delete(`/api/admin/cases/${sessionID}`);
  },

  async updateCustomerInfo(
    sessionID: string,
    customerName: string,
    customerPhone: string
  ): Promise<void> {
    await apiClient.put(`/api/admin/cases/${sessionID}/customer`, {
      customer_name: customerName,
      customer_phone: customerPhone,
    });
  },

  async clearAll(): Promise<void> {
    await apiClient.post('/api/admin/cases/clear-all');
  },

  async submitHelper(sessionID: string, helpContent: string): Promise<void> {
    await apiClient.post(`/api/admin/cases/${sessionID}/helper`, { help_content: helpContent });
  },

  async getHelperCases(status?: string): Promise<{ cases: ChatCase[]; total: number }> {
    const query = status ? `?status=${encodeURIComponent(status)}` : '';
    return apiClient.get(`/api/admin/cases/helper-cases${query}`);
  },

  async processHelper(sessionID: string, action: 'take_over' | 'transfer', targetUsername?: string): Promise<void> {
    await apiClient.post(`/api/admin/cases/${sessionID}/process-helper`, {
      action,
      target_username: targetUsername,
    });
  },

  async updateHelperStatus(sessionID: string, status: string): Promise<void> {
    await apiClient.put(`/api/admin/cases/${sessionID}/helper-status`, { status });
  },
};

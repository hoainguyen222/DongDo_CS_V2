// ============================================================
// Knowledge API — RAG document management
// ============================================================

import { apiClient } from './client';
import type { KnowledgeOverview } from '@/lib/types';

export const knowledgeApi = {
  async overview(): Promise<KnowledgeOverview> {
    return apiClient.get<KnowledgeOverview>('/api/admin/knowledge');
  },

  async upload(file: File): Promise<{ filename: string; message: string }> {
    return apiClient.uploadFile('/api/admin/knowledge/upload', file);
  },

  async deleteDocument(filename: string): Promise<{ success: boolean; filename: string; deleted_chunks: number; message: string }> {
    const params = new URLSearchParams({ filename });
    return apiClient.delete(`/api/admin/knowledge/document?${params.toString()}`);
  },
};

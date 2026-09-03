// ============================================================
// System API — Config + System errors
// ============================================================

import { apiClient } from './client';
import type { SystemConfig, SystemErrorItem } from '@/lib/types';

export const configApi = {
  async get(): Promise<SystemConfig> {
    return apiClient.get<SystemConfig>('/api/admin/config');
  },

  async save(config: SystemConfig): Promise<void> {
    await apiClient.post('/api/admin/config', config);
  },
};

export const systemApi = {
  async listErrors(): Promise<SystemErrorItem[]> {
    return apiClient.get<SystemErrorItem[]>('/api/admin/system-errors');
  },

  async createError(record: SystemErrorItem): Promise<void> {
    await apiClient.post('/api/admin/system-errors', {
      id: record.id,
      source: record.source,
      title: record.title,
      details: record.details,
      severity: record.severity || 'high',
      is_handled: record.isHandled || false,
      suggested_fix: record.suggestedFix || '',
    });
  },

  async markErrorHandled(id: string): Promise<void> {
    await apiClient.put(`/api/admin/system-errors/${id}/handled`);
  },
};

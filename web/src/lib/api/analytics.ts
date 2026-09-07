// ============================================================
// Analytics API — Stats & 7 sub-reports
// ============================================================

import { apiClient } from './client';
import type { AnalyticsStats } from '@/lib/types';

export interface DateRange {
  startDate?: string;
  endDate?: string;
}

export const analyticsApi = {
  async getStats(): Promise<AnalyticsStats> {
    return apiClient.get<AnalyticsStats>('/api/admin/analytics');
  },

  async getGeneralOverview({ startDate, endDate }: DateRange = {}): Promise<any> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    return apiClient.get<any>(`/api/admin/partner/reports/overview?${qs}`);
  },

  async getAIPerformance({ startDate, endDate }: DateRange = {}): Promise<any> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    return apiClient.get<any>(`/api/admin/partner/reports/ai-perf?${qs}`);
  },

  async getStaffPerformance({ startDate, endDate }: DateRange = {}): Promise<any[]> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    const data = await apiClient.get<{ staff_reports: any[] }>(`/api/admin/partner/reports/staff-perf?${qs}`);
    return data.staff_reports || [];
  },

  async getCX({ startDate, endDate }: DateRange = {}): Promise<any> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    return apiClient.get<any>(`/api/admin/partner/reports/cx?${qs}`);
  },

  async getOperational({ startDate, endDate }: DateRange = {}): Promise<any[]> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    const data = await apiClient.get<{ hourly_load: any[] }>(`/api/admin/partner/reports/operational?${qs}`);
    return data.hourly_load || [];
  },

  async getIssueAnalysis({ startDate, endDate }: DateRange = {}): Promise<any[]> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    const data = await apiClient.get<{ issues: any[] }>(`/api/admin/partner/reports/issues?${qs}`);
    return data.issues || [];
  },

  async getAILearningStats(): Promise<any> {
    return apiClient.get<any>('/api/admin/partner/reports/ai-learning');
  },
};

function buildQuery(params: Record<string, string | undefined>): string {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v) qs.append(k, v);
  });
  return qs.toString();
}

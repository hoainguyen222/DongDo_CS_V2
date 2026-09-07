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
    try {
      return await apiClient.get<AnalyticsStats>('/api/admin/analytics');
    } catch {
      return { total_cases: 0, ai_resolved: 0, cs_handover: 0, pending: 0 } as any;
    }
  },

  async getGeneralOverview({ startDate, endDate }: DateRange = {}): Promise<any> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    try {
      return await apiClient.get<any>(`/api/admin/partner/reports/overview?${qs}`);
    } catch {
      return {};
    }
  },

  async getAIPerformance({ startDate, endDate }: DateRange = {}): Promise<any> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    try {
      return await apiClient.get<any>(`/api/admin/partner/reports/ai-performance?${qs}`);
    } catch {
      return {};
    }
  },

  async getStaffPerformance({ startDate, endDate }: DateRange = {}): Promise<any[]> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    try {
      const data = await apiClient.get<{ staff_reports: any[] }>(`/api/admin/partner/reports/staff-performance?${qs}`);
      return data?.staff_reports || [];
    } catch {
      return [];
    }
  },

  async getCX({ startDate, endDate }: DateRange = {}): Promise<any> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    try {
      return await apiClient.get<any>(`/api/admin/partner/reports/cx?${qs}`);
    } catch {
      return {};
    }
  },

  async getOperational({ startDate, endDate }: DateRange = {}): Promise<any[]> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    try {
      const data = await apiClient.get<{ hourly_load: any[] }>(`/api/admin/partner/reports/operational?${qs}`);
      return data?.hourly_load || [];
    } catch {
      return [];
    }
  },

  async getIssueAnalysis({ startDate, endDate }: DateRange = {}): Promise<any[]> {
    const qs = buildQuery({ start_date: startDate, end_date: endDate });
    try {
      const data = await apiClient.get<{ issues: any[] }>(`/api/admin/partner/reports/issue-analysis?${qs}`);
      return data?.issues || [];
    } catch {
      return [];
    }
  },

  async getAILearningStats(): Promise<any> {
    try {
      return await apiClient.get<any>('/api/admin/partner/reports/ai-learning');
    } catch {
      return {};
    }
  },
};

function buildQuery(params: Record<string, string | undefined>): string {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v) qs.append(k, v);
  });
  return qs.toString();
}

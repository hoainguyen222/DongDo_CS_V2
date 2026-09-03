// ============================================================
// Customers API — Customer profiles management
// ============================================================

import { apiClient } from './client';
import type { CustomerProfile } from '@/lib/types';

export interface ListCustomersParams {
  page?: number;
  limit?: number;
  search?: string;
}

export interface ListCustomersResult {
  customers: CustomerProfile[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export const customersApi = {
  async list(params: ListCustomersParams = {}): Promise<ListCustomersResult> {
    const { page = 1, limit = 10, search } = params;
    const qs = new URLSearchParams();
    qs.append('page', page.toString());
    qs.append('limit', limit.toString());
    if (search) qs.append('search', search);
    return apiClient.get<ListCustomersResult>(`/api/admin/customers?${qs.toString()}`);
  },

  async update(guestID: string, customerName: string, customerPhone: string): Promise<void> {
    await apiClient.put(`/api/admin/customers/${guestID}`, {
      customer_name: customerName,
      customer_phone: customerPhone,
    });
  },

  async delete(guestID: string): Promise<void> {
    await apiClient.delete(`/api/admin/customers/${guestID}`);
  },
};

// ============================================================
// Auth API — Login, logout, bootstrap
// ============================================================

import { apiClient, setAuthToken, setAuthUser } from './client';
import type { SessionUser } from '@/lib/types';

export const authApi = {
  async login(username: string, password: string): Promise<SessionUser> {
    const session = await apiClient.post<SessionUser>('/auth/login', { username, password });
    setAuthToken(session.token);
    setAuthUser(session);
    return session;
  },

  async logout(): Promise<void> {
    await apiClient.post('/auth/logout').catch(() => {});
    setAuthToken(null);
    setAuthUser(null);
  },

  async getMe(): Promise<SessionUser> {
    return apiClient.get<SessionUser>('/auth/me');
  },

  async checkBootstrapStatus(): Promise<{ needs_setup: boolean; is_enabled: boolean }> {
    return apiClient.get<{ needs_setup: boolean; is_enabled: boolean }>('/api/bootstrap/status');
  },

  async installBootstrap(data: { username: string; password: string; fullName: string }): Promise<any> {
    return apiClient.post('/api/bootstrap/install', data);
  },
};

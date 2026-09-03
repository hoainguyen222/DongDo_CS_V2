// ============================================================
// Users API — Account management
// ============================================================

import { apiClient } from './client';
import type { PermissionLevel, RolePermissionItem } from '@/lib/types';

export interface AdminUser {
  id: string;
  fullName: string;
  email: string;
  role: string;
  rawRole: string;
  status: string;
  isActive: boolean;
  createdAt: string;
}

export const usersApi = {
  async list(): Promise<AdminUser[]> {
    const data = await apiClient.get<{ users: any[] }>('/api/admin/users');
    return (data.users || []).map(mapUser);
  },

  async create(
    fullName: string,
    email: string,
    role: string,
    password?: string
  ): Promise<AdminUser> {
    const u = await apiClient.post<any>('/api/admin/users', {
      full_name: fullName,
      email,
      role,
      password: password || '12345678',
    });
    return mapUser(u);
  },

  async update(
    username: string,
    data: { fullName: string; role: string; isActive: boolean; password?: string }
  ): Promise<AdminUser> {
    const u = await apiClient.put<any>(`/api/admin/users/${encodeURIComponent(username)}`, {
      full_name: data.fullName,
      role: data.role,
      is_active: data.isActive,
      password: data.password || '',
    });
    return mapUser(u);
  },

  async delete(username: string): Promise<void> {
    await apiClient.delete(`/api/admin/users/${username}`);
  },
};

function mapUser(u: any): AdminUser {
  const rLower = (u.role || '').toLowerCase();
  let roleLabel = 'Staff CS';
  if (rLower.includes('owner')) roleLabel = 'Owner';
  else if (rLower.includes('admin')) roleLabel = 'Admin';
  else if (rLower.includes('leader')) roleLabel = 'Leader CS';

  return {
    id: u.id ? u.id.toString() : u.username,
    fullName: u.full_name || u.username,
    email: u.username,
    role: roleLabel,
    rawRole: u.role,
    status: u.is_active ? 'Hoạt động' : 'Tạm khóa',
    isActive: u.is_active,
    createdAt: u.created_at
      ? new Date(u.created_at).toISOString().split('T')[0]
      : 'N/A',
  };
}

export const permissionsApi = {
  async list(): Promise<RolePermissionItem[]> {
    const data = await apiClient.get<{ permissions: RolePermissionItem[] }>(
      '/api/admin/partner/config/permissions'
    );
    return data.permissions || [];
  },

  async upsert(roleName: string, featureKey: string, permissionLevel: PermissionLevel): Promise<void> {
    await apiClient.post('/api/admin/partner/config/permissions', {
      role_name: roleName,
      feature_key: featureKey,
      permission_level: permissionLevel,
      can_view: permissionLevel !== 'none',
      can_edit: permissionLevel === 'act',
    });
  },
};

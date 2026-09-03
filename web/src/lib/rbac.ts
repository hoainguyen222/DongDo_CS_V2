/**
 * RBAC (Role-Based Access Control) utilities
 * Dùng để check permissions trên frontend
 */

import type { TabType } from '@/lib/types';

// Role hierarchy: Owner > Admin > Leader > Staff
export type RoleLevel = 'Staff' | 'Leader' | 'Admin' | 'Owner';

const ROLE_HIERARCHY: Record<RoleLevel, number> = {
  Staff: 0,
  Leader: 1,
  Admin: 2,
  Owner: 3,
};

/**
 * Normalize role string to standard RoleLevel
 */
export function normalizeRole(role?: string): RoleLevel {
  if (!role) return 'Staff';
  const r = role.trim().toLowerCase();
  if (r.includes('owner')) return 'Owner';
  if (r.includes('admin')) return 'Admin';
  if (r.includes('leader')) return 'Leader';
  return 'Staff';
}

/**
 * Check if user has required role level
 */
export function hasRoleLevel(userRole: string, requiredRole: RoleLevel): boolean {
  const userLevel = ROLE_HIERARCHY[normalizeRole(userRole)] ?? 0;
  const requiredLevel = ROLE_HIERARCHY[requiredRole] ?? 0;
  return userLevel >= requiredLevel;
}

/**
 * Check if user can access a feature based on role and permissions
 */
export function canAccessFeature(
  userRole: string,
  featureKey: string,
  permissions: { role_name: string; feature_key: string; permission_level: string }[] = []
): 'act' | 'view' | 'none' {
  const normRole = normalizeRole(userRole);
  
  // Owner has full access
  if (normRole === 'Owner') return 'act';
  
  // Check explicit permissions
  const item = permissions.find(
    (p) => p.role_name.toLowerCase() === normRole.toLowerCase() && p.feature_key === featureKey
  );
  if (item) return item.permission_level as 'act' | 'view' | 'none';
  
  // Check parent feature fallback
  if (featureKey.includes('.')) {
    const parent = featureKey.split('.')[0];
    const parentItem = permissions.find(
      (p) => p.role_name.toLowerCase() === normRole.toLowerCase() && p.feature_key === parent
    );
    if (parentItem) return parentItem.permission_level as 'act' | 'view' | 'none';
  }
  
  // Default permissions by role
  if (normRole === 'Leader') {
    if (featureKey.startsWith('config') || featureKey.startsWith('partner_config')) return 'none';
    return 'act';
  }
  
  if (featureKey.startsWith('inbox') || featureKey.startsWith('calls')) return 'act';
  if (featureKey.startsWith('partner_dashboard') || featureKey.startsWith('customers') || featureKey.startsWith('knowledge')) return 'view';
  
  return 'none';
}

/**
 * Navigation items with required roles
 */
export const NAV_REQUIREMENTS: Record<TabType, { minRole: RoleLevel; ownerOnly?: boolean }> = {
  partner_dashboard: { minRole: 'Staff' },
  inbox: { minRole: 'Staff' },
  customers: { minRole: 'Staff' },
  calls: { minRole: 'Staff' },
  learning: { minRole: 'Staff' },
  knowledge: { minRole: 'Staff' },
  analytics: { minRole: 'Staff' },
  config: { minRole: 'Admin' },
  partner_analytics: { minRole: 'Staff' },
  partner_config: { minRole: 'Admin' },
  update_data_test: { minRole: 'Owner' },
};

/**
 * Check if user can see a navigation tab
 */
export function canSeeTab(userRole: string, tab: TabType): boolean {
  const req = NAV_REQUIREMENTS[tab];
  if (!req) return false;
  
  if (req.ownerOnly) {
    return normalizeRole(userRole) === 'Owner';
  }
  
  return hasRoleLevel(userRole, req.minRole);
}

/**
 * Check if user can perform actions on a tab
 */
export function canPerformTabAction(userRole: string, tab: TabType): boolean {
  const access = canAccessFeature(userRole, tab, []);
  return access === 'act';
}

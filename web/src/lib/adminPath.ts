/**
 * Admin path configuration helper
 * - Server-side (middleware): reads ADMIN_PATH env directly
 * - Client-side: reads NEXT_PUBLIC_ADMIN_PATH (exposed via webpack)
 */

const DEFAULT_ADMIN_PATH = '/admin';

export function getAdminPath(): string {
  if (typeof window === 'undefined') {
    // Server-side (middleware, server components)
    return process.env.ADMIN_PATH || DEFAULT_ADMIN_PATH;
  }
  // Client-side (browser)
  return (process.env.NEXT_PUBLIC_ADMIN_PATH as string | undefined) || DEFAULT_ADMIN_PATH;
}

export function adminPath(suffix: string): string {
  return `${getAdminPath()}${suffix}`;
}

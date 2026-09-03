// ============================================================
// AdminGuard — Protects admin routes from unauthenticated access
// Renders loading screen while auth hydration completes (SSR-safe)
// ============================================================

'use client';

import { useEffect, useSyncExternalStore, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ShieldCheck } from 'lucide-react';
import { useAuthStore } from '@/lib/stores/authStore';
import styles from './AdminGuard.module.scss';

function subscribeNoop() {
  return () => {};
}

/**
 * useHasMounted — returns false during SSR + first render, true after.
 * Uses useSyncExternalStore for stable identity across renders so
 * Rules of Hooks aren't violated during hydration.
 */
function useHasMounted(): boolean {
  return useSyncExternalStore(
    subscribeNoop,
    () => true,
    () => false
  );
}

export function AdminGuard({ children }: { children: ReactNode }) {
  const router = useRouter();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const hasMounted = useHasMounted();

  // Only check auth AFTER component has mounted (avoids SSR/CSR mismatch)
  useEffect(() => {
    if (hasMounted && !isAuthenticated) {
      router.replace('/admin/login');
    }
  }, [hasMounted, isAuthenticated, router]);

  if (!hasMounted) return <LoadingScreen />;
  if (!isAuthenticated) return <LoadingScreen />;

  return <>{children}</>;
}

function LoadingScreen() {
  return (
    <div className={styles.shell}>
      <div className={styles.blobNavy} />
      <div className={styles.blobRed} />
      <div className={styles.content}>
        <div className={styles.iconBox}>
          <ShieldCheck className={styles.iconPulse} />
        </div>
        <div className={styles.title}>Đang xác thực phiên làm việc...</div>
      </div>
    </div>
  );
}

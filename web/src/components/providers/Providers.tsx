'use client';

import React, { useState, useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { X, CheckCircle2, XCircle, AlertTriangle, Info } from 'lucide-react';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useUIStore, type ToastVariant } from '@/lib/stores/uiStore';
import { useAuthStore } from '@/lib/stores/authStore';
import styles from './Providers.module.scss';

// ============================================================
// QueryClient setup
// ============================================================
function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: 1,
        refetchOnWindowFocus: false,
      },
      mutations: { retry: 0 },
    },
  });
}

let browserQueryClient: QueryClient | undefined;

function getQueryClient() {
  if (typeof window === 'undefined') return makeQueryClient();
  if (!browserQueryClient) browserQueryClient = makeQueryClient();
  return browserQueryClient;
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => getQueryClient());
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

// ============================================================
// Toast container
// ============================================================
const variantConfig: Record<
  ToastVariant,
  { icon: React.ReactNode; className: string }
> = {
  success: {
    icon: <CheckCircle2 style={{ width: 16, height: 16 }} />,
    className: styles.toastItemSuccess,
  },
  error: {
    icon: <XCircle style={{ width: 16, height: 16 }} />,
    className: styles.toastItemError,
  },
  warning: {
    icon: <AlertTriangle style={{ width: 16, height: 16 }} />,
    className: styles.toastItemWarning,
  },
  info: {
    icon: <Info style={{ width: 16, height: 16 }} />,
    className: styles.toastItemInfo,
  },
};

export function ToastContainer() {
  const { toasts, removeToast } = useUIStore();
  if (toasts.length === 0) return null;

  return (
    <div className={styles.toastContainer}>
      {toasts.map((toast) => {
        const cfg = variantConfig[toast.variant];
        return (
          <div key={toast.id} className={`${styles.toastItem} ${cfg.className}`}>
            <span className={styles.toastIcon}>{cfg.icon}</span>
            <div className={styles.toastBody}>
              <p className={styles.toastTitle}>{toast.title}</p>
              {toast.message && <p className={styles.toastMessage}>{toast.message}</p>}
            </div>
            <button onClick={() => removeToast(toast.id)} className={styles.toastClose} aria-label="Đóng">
              <X style={{ width: 14, height: 14 }} />
            </button>
          </div>
        );
      })}
    </div>
  );
}

// ============================================================
// Viewport height setter (iOS Safari fallback for 100vh)
// ============================================================
// 100vh trên iOS Safari KHÔNG tự co lại khi thanh URL ẩn/hiện → gây jump layout.
// 100dvh đã được support từ iOS 15.4+ nhưng vẫn có browser cũ.
// Component này set CSS variable --vh = window.innerHeight * 1% để dùng làm
// fallback khi cần (các component có thể dùng calc(var(--vh) * 100) nếu muốn).
function ViewportHeightSetter() {
  useEffect(() => {
    const setVH = () => {
      // Trừ 0.5px để tránh sub-pixel rounding khiến scroll xuất hiện
      const vh = window.innerHeight * 0.01;
      document.documentElement.style.setProperty('--vh', `${vh}px`);
    };
    setVH();
    window.addEventListener('resize', setVH);
    window.addEventListener('orientationchange', setVH);
    // visualViewport API: trên mobile, fire khi toolbar ẩn/hiện
    if (window.visualViewport) {
      window.visualViewport.addEventListener('resize', setVH);
    }
    return () => {
      window.removeEventListener('resize', setVH);
      window.removeEventListener('orientationchange', setVH);
      if (window.visualViewport) {
        window.visualViewport.removeEventListener('resize', setVH);
      }
    };
  }, []);
  return null;
}

// ============================================================
// Top-level UI shell (toast + confirm)
// ============================================================
export function GlobalUI() {
  return (
    <>
      <ViewportHeightSetter />
      <ToastContainer />
      <ConfirmDialog />
    </>
  );
}

// ============================================================
// Error boundary placeholder (kept for future)
// ============================================================
export function GlobalErrorHandler({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}

// ============================================================
// UI Store — Global UI state (toasts, confirm dialogs, error center)
// ============================================================

import { create } from 'zustand';
import type { SystemErrorItem } from '@/lib/types';

export type ToastVariant = 'success' | 'error' | 'warning' | 'info';

export interface Toast {
  id: string;
  title: string;
  message?: string;
  variant: ToastVariant;
  duration?: number;
}

export interface ConfirmDialog {
  title: string;
  message: string;
  onConfirm: () => void | Promise<void>;
  confirmText?: string;
  cancelText?: string;
  variant?: 'danger' | 'warning' | 'info';
}

interface UIState {
  // Toasts
  toasts: Toast[];
  addToast: (toast: Omit<Toast, 'id'>) => void;
  removeToast: (id: string) => void;

  // Confirm dialog
  confirmDialog: ConfirmDialog | null;
  openConfirm: (config: ConfirmDialog) => void;
  closeConfirm: () => void;

  // Pending call (call v2 — shared so the float banner on /admin and the
  // detail page on /admin/inbox can render "Nghe" buttons in sync).
  // The list (not a single record) lets multiple sessions ring at once —
  // the float banner shows the newest, the detail page shows any whose
  // session_id matches the currently selected case.
  pendingCalls: PendingCall[];
  pushPendingCall: (call: PendingCall) => void;
  removePendingCall: (callId: string) => void;
  clearPendingCalls: () => void;

  // Error center
  systemErrors: SystemErrorItem[];
  showErrorCenter: boolean;
  setSystemErrors: (errors: SystemErrorItem[]) => void;
  addSystemError: (error: SystemErrorItem) => void;
  markErrorHandled: (id: string) => void;
  clearHandledErrors: () => void;
  setShowErrorCenter: (show: boolean) => void;

  // Global loading overlay
  globalLoading: boolean;
  setGlobalLoading: (loading: boolean) => void;
}

// PendingCall tracks an incoming call for as long as the admin hasn't
// answered, declined, or missed it. The admin layout's float banner and
// the inbox detail page both read from this list.
export interface PendingCall {
  call_id: string;
  session_id: string;
  caller_id: string;
  offer?: any;
  received_at: number;
}

let toastIdCounter = 0;

export const useUIStore = create<UIState>((set) => ({
  toasts: [],
  addToast: (toast) => {
    const id = `toast-${++toastIdCounter}`;
    const duration = toast.duration ?? 5000;
    set((state) => ({
      toasts: [...state.toasts, { ...toast, id }],
    }));
    setTimeout(() => {
      set((state) => ({
        toasts: state.toasts.filter((t) => t.id !== id),
      }));
    }, duration);
  },
  removeToast: (id) =>
    set((state) => ({
      toasts: state.toasts.filter((t) => t.id !== id),
    })),

  confirmDialog: null,
  openConfirm: (config) => set({ confirmDialog: config }),
  closeConfirm: () => set({ confirmDialog: null }),

  pendingCalls: [],
  pushPendingCall: (call) =>
    set((state) => {
      // Replace if same call_id already present (dedup the dedup key from
      // backend MarkAnnounced).
      const filtered = state.pendingCalls.filter((c) => c.call_id !== call.call_id);
      // Auto-expire entries older than 60s so we don't leak forever if the
      // backend never sends a call_ended.
      const fresh = filtered.filter((c) => Date.now() - c.received_at < 60_000);
      return { pendingCalls: [call, ...fresh].slice(0, 10) };
    }),
  removePendingCall: (callId) =>
    set((state) => ({
      pendingCalls: state.pendingCalls.filter((c) => c.call_id !== callId),
    })),
  clearPendingCalls: () => set({ pendingCalls: [] }),

  systemErrors: [],
  showErrorCenter: false,
  setSystemErrors: (errors) => set({ systemErrors: errors }),
  addSystemError: (error) =>
    set((state) => ({
      systemErrors: [error, ...state.systemErrors],
    })),
  markErrorHandled: (id) =>
    set((state) => ({
      systemErrors: state.systemErrors.map((e) =>
        e.id === id ? { ...e, isHandled: true } : e
      ),
    })),
  clearHandledErrors: () =>
    set((state) => ({
      systemErrors: state.systemErrors.filter((e) => !e.isHandled),
    })),
  setShowErrorCenter: (show) => set({ showErrorCenter: show }),

  globalLoading: false,
  setGlobalLoading: (loading) => set({ globalLoading: loading }),
}));

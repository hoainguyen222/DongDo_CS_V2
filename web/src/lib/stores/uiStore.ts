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

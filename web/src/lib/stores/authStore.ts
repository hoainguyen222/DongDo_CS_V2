// ============================================================
// Auth Store — Zustand + persist
// Manages authentication state synced with localStorage + cookies
// ============================================================

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { SessionUser } from '@/lib/types';
import { authApi, setAuthToken, setAuthUser } from '@/lib/api';

interface AuthState {
  token: string | null;
  user: SessionUser | null;
  isAuthenticated: boolean;
  hasHydrated: boolean;

  // Actions
  login: (token: string, user: SessionUser) => void;
  logout: () => Promise<void>;
  setToken: (token: string | null) => void;
  setUser: (user: SessionUser | null) => void;
  setHasHydrated: (v: boolean) => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      isAuthenticated: false,
      hasHydrated: false,

      login: (token, user) => {
        setAuthToken(token);
        setAuthUser(user);
        set({ token, user, isAuthenticated: true });
      },

      logout: async () => {
        await authApi.logout().catch(() => {});
        set({ token: null, user: null, isAuthenticated: false });
      },

      setToken: (token) => {
        setAuthToken(token);
        set({ token, isAuthenticated: !!token });
      },

      setUser: (user) => set({ user }),

      setHasHydrated: (v) => set({ hasHydrated: v }),
    }),
    {
      name: 'dongdo-auth',
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
      onRehydrateStorage: () => (state) => {
        if (state?.token && typeof document !== 'undefined') {
          document.cookie = `dongdo_auth_token=${encodeURIComponent(state.token)};path=/;max-age=${7 * 24 * 60 * 60};samesite=strict`;
        }
        state?.setHasHydrated(true);
      },
    }
  )
);

// ── Convenience selectors ───────────────────────────────────
export const selectToken = (s: AuthState) => s.token;
export const selectUser = (s: AuthState) => s.user;
export const selectIsAuth = (s: AuthState) => s.isAuthenticated;

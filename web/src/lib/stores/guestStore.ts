// ============================================================
// Guest Store — Customer-facing session
// ============================================================

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { GuestSession } from '@/lib/types';

interface GuestState {
  session: GuestSession | null;
  isRegistered: boolean;
  setSession: (session: GuestSession) => void;
  clearSession: () => void;
}

export const useGuestStore = create<GuestState>()(
  persist(
    (set) => ({
      session: null,
      isRegistered: false,
      setSession: (session) => set({ session, isRegistered: true }),
      clearSession: () => set({ session: null, isRegistered: false }),
    }),
    {
      name: 'dongdo-guest-session',
      partialize: (state) => ({
        session: state.session,
        isRegistered: state.isRegistered,
      }),
    }
  )
);

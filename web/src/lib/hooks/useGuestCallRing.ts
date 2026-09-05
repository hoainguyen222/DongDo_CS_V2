'use client';

/**
 * useGuestCallRing — Listen for inbound WebSocket `call_ring` events from the
 * backend and surface them as a floating banner on the current page.
 *
 * Designed to work on ANY page (public or authenticated) — it opens a WS
 * connection to the backend `admin_inbox` channel so that CSKH staff who are
 * on the login screen (or any public page) still see incoming call rings.
 *
 * IMPORTANT: We deliberately use a SEPARATE WS connection from
 * /admin/layout.tsx's WS. Each call of this hook manages its own connection
 * so it can be mounted/unmounted independently (e.g. on /admin/login) without
 * stomping on the admin inbox listener. The backend Hub broadcasts events to
 * every connected WS client keyed by sessionID, so duplicate connections
 * receive duplicates — this is acceptable for "show me rings everywhere".
 *
 * Usage:
 *   const { incomingRing, dismissRing } = useGuestCallRing({
 *     enabled: true,
 *     channelId: 'login_watcher',     // optional, defaults to random
 *   });
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { WSClient } from '@/lib/ws';

export interface GuestCallRingEvent {
  session_id: string;
  caller_id: string;
  call_id?: number;
  phone?: string;
  received_at: number;
}

export interface UseGuestCallRingOptions {
  /** Set false to disable the WS connection entirely. */
  enabled?: boolean;
  /** Optional: visible-to-backend session id (defaults to a stable per-tab id). */
  channelId?: string;
  /** User id sent to backend (anonymous for the login page). */
  userId?: string;
  /** Role sent to backend (defaults to 'admin' so backend forwards admin_inbox events). */
  role?: string;
  /** Milliseconds after which an unanswered ring self-dismisses. */
  autoDismissMs?: number;
}

const DEFAULT_AUTO_DISMISS_MS = 60_000;

function makeChannelId(seed?: string): string {
  if (seed) return seed;
  if (typeof window === 'undefined') return 'admin_watcher_ssr';
  try {
    const k = 'dongdo_admin_ws_channel';
    let v = window.sessionStorage.getItem(k);
    if (!v) {
      v = `admin_watcher_${Math.random().toString(36).slice(2, 10)}`;
      window.sessionStorage.setItem(k, v);
    }
    return v;
  } catch (_) {
    return `admin_watcher_${Math.random().toString(36).slice(2, 10)}`;
  }
}

export function useGuestCallRing(opts: UseGuestCallRingOptions = {}) {
  const {
    enabled = true,
    channelId,
    userId = 'admin_watcher',
    role = 'admin',
    autoDismissMs = DEFAULT_AUTO_DISMISS_MS,
  } = opts;

  const wsRef = useRef<WSClient | null>(null);
  const dismissTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [incomingRing, setIncomingRing] = useState<GuestCallRingEvent | null>(null);

  const dismissRing = useCallback(() => {
    setIncomingRing(null);
    if (dismissTimerRef.current) {
      clearTimeout(dismissTimerRef.current);
      dismissTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    if (!enabled) return;
    if (typeof window === 'undefined') return;

    const cid = makeChannelId(channelId);
    const ws = new WSClient(cid, userId, role);
    wsRef.current = ws;

    const onRing = (event: any) => {
      // Defensive parsing — backend may wrap payload or send flat fields.
      const sID =
        event?.payload?.session_id ||
        event?.session_id ||
        event?.payload?.call?.session_id;
      if (!sID) return;
      const ring: GuestCallRingEvent = {
        session_id: sID,
        caller_id:
          event?.payload?.caller_id ||
          event?.sender_id ||
          event?.payload?.call?.caller_id ||
          'Khách hàng',
        call_id:
          event?.payload?.call_id ||
          event?.call_id ||
          event?.payload?.call?.id,
        phone:
          event?.payload?.phone ||
          event?.payload?.phone_number ||
          event?.payload?.call?.phone ||
          event?.phone,
        received_at: Date.now(),
      };

      // If a ring for the same session arrives again, refresh the timer.
      setIncomingRing((prev) => {
        if (prev && prev.session_id === ring.session_id) {
          return { ...ring, received_at: Date.now() };
        }
        return ring;
      });

      if (dismissTimerRef.current) clearTimeout(dismissTimerRef.current);
      dismissTimerRef.current = setTimeout(() => {
        setIncomingRing((cur) => (cur?.session_id === ring.session_id ? null : cur));
      }, autoDismissMs);
    };

    // Subscribe to BOTH the modern and legacy event names.
    //
    // `call_ring` / `call_offer` are the original ring notifications,
    // fired by ARI on guest INVITE, accept and hangup events.
    // `call_status_update` is the unified transition event published
    // on every state change (CreateCall, AcceptCall, HangupCall, ARI).
    // We treat any RINGING/WAITING_AGENT transition as a fresh ring.
    const offRing = ws.on('call_ring', onRing);
    const offOffer = ws.on('call_offer', onRing);
    const offStatus = ws.on('call_status_update', (event: any) => {
      const status: string = (event.payload?.status || '').toUpperCase();
      if (status === 'RINGING' || status === 'WAITING' || status === 'WAITING_AGENT' || status === 'CONNECTING') {
        onRing(event);
      } else if (status === 'ENDED' || status === 'COMPLETED' || status === 'MISSED' || status === 'REJECTED') {
        // Terminal transition — hide banner immediately, no waiting
        // for the auto-dismiss timer.
        setIncomingRing((cur) => {
          const sID = event?.payload?.session_id || event?.session_id;
          if (cur && sID && cur.session_id === sID) {
            if (dismissTimerRef.current) {
              clearTimeout(dismissTimerRef.current);
              dismissTimerRef.current = null;
            }
            return null;
          }
          return cur;
        });
      }
    });
    const offEnd = ws.on('call_end', () => dismissRing());

    ws.connect();

    return () => {
      try { offRing(); } catch (_) {}
      try { offOffer(); } catch (_) {}
      try { offStatus(); } catch (_) {}
      try { offEnd(); } catch (_) {}
      try { ws.disconnect(); } catch (_) {}
      wsRef.current = null;
      if (dismissTimerRef.current) {
        clearTimeout(dismissTimerRef.current);
        dismissTimerRef.current = null;
      }
    };
  }, [enabled, channelId, userId, role, autoDismissMs, dismissRing]);

  return {
    incomingRing,
    dismissRing,
    wsRef,
  };
}

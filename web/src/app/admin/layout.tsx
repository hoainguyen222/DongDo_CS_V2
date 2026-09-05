'use client';

import React, { useEffect, useRef, useState, useCallback } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { useAuthStore } from '@/lib/stores/authStore';
import { useRolePermissions, useVoiceCalls, usePendingLearning, useCases } from '@/lib/hooks/useApi';
import { resolveAgentExtension } from '@/lib/utils';
import { WSClient } from '@/lib/ws';
import { useAsteriskSip } from '@/lib/hooks/useAsteriskSip';
import type { AsteriskSipConfig, IncomingCallInfo } from '@/lib/asteriskSip';
import { SoftphoneBanner } from '@/components/admin/SoftphoneBanner';
import {
  AdminSidebar,
  IncomingCallBanner,
  useTeamAgentNotifications,
  TeamAgentGuestCallBanner,
} from '@/components/admin/AdminSidebar';
import { VoiceHistoryModal, ToastErrorBanner, ErrorCenterWrapper } from '@/components/admin/AdminModals';
import { useUIStore } from '@/lib/stores/uiStore';
import { playNotificationTone } from '@/lib/audioStream';
import { AdminLoadingScreen } from './AdminLoadingScreen';
import styles from './AdminLayout.module.scss';

// Public admin routes — bypass auth & layout chrome entirely
const ADMIN_PUBLIC_PATHS = ['/admin/login'];

function isAdminPublic(pathname: string): boolean {
  return ADMIN_PUBLIC_PATHS.some((p) => pathname === p || pathname.startsWith(p + '/'));
}

const MISSED_CALL_TIMEOUT_MS = 60_000;

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const publicPath = isAdminPublic(pathname);

  const { token, user, isAuthenticated, hasHydrated } = useAuthStore();
  const { setShowErrorCenter, systemErrors, addToast } = useUIStore();
  const queryClient = useQueryClient();
  const wsRef = useRef<WSClient | null>(null);

  const [showVoiceHistoryModal, setShowVoiceHistoryModal] = useState(false);
  const [toastError, setToastError] = useState<{ title: string; source: string; details: string } | null>(null);

  // Click-to-call display state (no audio handling on admin side)
  const [incomingCall, setIncomingCall] = useState<{
    session_id: string;
    caller_id: string;
    call_id?: number;
    phone?: string;
  } | null>(null);
  const missedCallTimerRef = useRef<any>(null);

  // Asterisk sip.js softphone — the agent's SIP extension is registered
  // automatically once we have credentials from /api/me/sip-config.  The
  // ARI backend originates to this extension when an admin accepts a
  // guest call, so the sip.js client rings natively.
  const [sipConfig, setSipConfig] = useState<AsteriskSipConfig | null>(null);
  const [sipIncoming, setSipIncoming] = useState<IncomingCallInfo | null>(null);
  const [sipActive, setSipActive] = useState(false);

  // Fetch the SIP credentials after auth.  Skipped on public paths.
  useEffect(() => {
    if (!hasHydrated) return;
    if (publicPath) return;
    if (!token || !user) {
      setSipConfig(null);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const { voiceApi } = await import('@/lib/api/voice');
        const cfg = await voiceApi.getMySipConfig();
        if (cancelled) return;
        if (!cfg) {
          setSipConfig(null);
          return;
        }
        setSipConfig({
          websocketUrl: cfg.websocket_url,
          sipServer: cfg.sip_server,
          agentExtension: cfg.agent_extension,
          agentPassword: cfg.agent_password,
          stunServers: cfg.stun_servers,
          displayName: cfg.display_name,
        });
      } catch (err) {
        console.warn('fetch sip config failed', err);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [hasHydrated, publicPath, token, user]);

  const sip = useAsteriskSip({
    enabled: !!sipConfig && !publicPath,
    config: sipConfig,
    onIncomingCall: (info) => {
      setSipIncoming(info);
      setSipActive(false);
      // Also play the existing notification tone for parity with the
      // WS-driven banner.
      void playNotificationTone().catch(() => {});
    },
    onSessionEnded: () => {
      setSipIncoming(null);
      setSipActive(false);
    },
    onError: (err) => {
      console.warn('sip.js error', err);
    },
  });

  // Wrap the hook callbacks so we can flip the local "active" flag.
  const sipAnswer = useCallback(async () => {
    await sip.answer();
    setSipActive(true);
  }, [sip]);

  const sipDecline = useCallback(async () => {
    await sip.decline();
    setSipIncoming(null);
  }, [sip]);

  const sipHangup = useCallback(async () => {
    await sip.hangup();
    setSipActive(false);
    setSipIncoming(null);
  }, [sip]);

  // Redirect unauthenticated users to login (client-side only, after hydration)
  useEffect(() => {
    if (!hasHydrated) return;
    if (!isAuthenticated && !publicPath) {
      router.replace('/admin/login');
    }
  }, [hasHydrated, isAuthenticated, publicPath, router]);

  // Only fetch data when authenticated (and skip public paths entirely)
  const { data: casesData } = useCases('', 1, 10, undefined, {
    enabled: hasHydrated && isAuthenticated && !publicPath,
  });
  const { data: voiceCallsData } = useVoiceCalls(undefined, 1, 10, {
    enabled: hasHydrated && isAuthenticated && !publicPath,
  });
  const { data: learningData } = usePendingLearning(1, 10, {
    enabled: hasHydrated && isAuthenticated && !publicPath,
  });
  const { data: permissionsData } = useRolePermissions({
    enabled: hasHydrated && isAuthenticated && !publicPath,
  });

  // Setup WebSocket connection (only for authenticated users on protected paths).
  // MUST be declared before any early `return` to satisfy Rules of Hooks.
  //
  // QUAN TRỌNG: Phải giữ WSClient ổn định cho cả phiên admin để listeners đăng ký qua
  // useTeamAgentNotifications KHÔNG bị mất khi component re-render.
  // Tạo 1 lần duy nhất khi (token, user.username, user.role) thay đổi — disconnect cũ nếu có.
  useEffect(() => {
    if (!hasHydrated) return;
    if (publicPath) return;
    if (!token || !user) return;

    // Nếu đã có WS hợp lệ trỏ tới đúng user → không tạo lại (giữ listeners cũ)
    const existing = wsRef.current;
    if (existing && (existing as any).sessionID === 'admin_inbox' && (existing as any).userID === user.username) {
      return;
    }

    // Cleanup WS cũ (nếu có) trước khi tạo mới
    if (existing) {
      try { existing.disconnect(); } catch (_) {}
      wsRef.current = null;
    }

    const ws = new WSClient('admin_inbox', user.username, user.role);
    wsRef.current = ws;
    ws.connect();

    // Invalidate relevant TanStack Query caches when the backend pushes
    // real-time events. This replaces the prior polling approach (was:
    // refetchInterval: 15_000 on useCases / usePendingLearning).
    ws.on('case_update', () => {
      queryClient.invalidateQueries({ queryKey: ['cases'] });
      queryClient.invalidateQueries({ queryKey: ['customers'] });
      queryClient.invalidateQueries({ queryKey: ['analytics'] });
      queryClient.invalidateQueries({ queryKey: ['voiceCalls'] });
    });
    ws.on('learning_update', () => {
      queryClient.invalidateQueries({ queryKey: ['pendingLearning'] });
      queryClient.invalidateQueries({ queryKey: ['knowledge'] });
    });
    ws.on('message', () => {});
    ws.on('call_end', () => {
      queryClient.invalidateQueries({ queryKey: ['voiceCalls'] });
      setIncomingCall(null);
      if (missedCallTimerRef.current) {
        clearTimeout(missedCallTimerRef.current);
        missedCallTimerRef.current = null;
      }
    });

    // Shared handler — used by both the legacy `call_ring`/`call_offer`
    // events and the new unified `call_status_update` event so any
    // transition into a "ringing" state surfaces the banner.
    const showIncomingFromEvent = (event: any) => {
      const sID = event.payload?.session_id || event.session_id;
      const cID = event.payload?.caller_id || event.sender_id || 'Khách hàng';
      const callId = event.payload?.call_id || event.call_id;
      const phone = event.payload?.phone || event.payload?.phone_number;
      if (!sID) return;
      setIncomingCall({ session_id: sID, caller_id: cID, call_id: callId, phone });
      addToast({
        title: '📞 Cuộc gọi đến',
        message: `${cID}${phone ? ` — ${phone}` : ''}`,
        variant: 'info',
        duration: 6000,
      });
      // Best-effort notification tone; silently fails on browsers without WebAudio.
      void playNotificationTone();

      // Auto-mark missed if not picked up.
      if (missedCallTimerRef.current) clearTimeout(missedCallTimerRef.current);
      missedCallTimerRef.current = setTimeout(async () => {
        try {
          const { api } = await import('@/lib/api');
          if (callId) {
            await api.markMissedCall(callId, sID).catch(() => {});
          }
        } catch (_) {
          /* noop */
        }
        setIncomingCall(null);
      }, MISSED_CALL_TIMEOUT_MS);
    };

    // Click-to-call incoming: show toast + notification tone, plus banner.
    ws.on('call_ring', showIncomingFromEvent);

    // call_offer kept for backward-compat with old backend events — treat
    // the same as call_ring. Use functional setter so we don't need to
    // read stale `incomingCall` from a closure.
    ws.on('call_offer', (event: any) => {
      const sID = event.payload?.session_id || event.session_id;
      if (!sID) return;
      setIncomingCall((prev) => {
        if (prev) return prev; // already showing
        const cID = event.payload?.caller_id || event.sender_id || 'Khách hàng';
        const callId = event.payload?.call_id || event.call_id;
        const phone = event.payload?.phone || event.payload?.phone_number;
        return { session_id: sID, caller_id: cID, call_id: callId, phone };
      });
    });

    // Unified status transitions: surface ring on any non-terminal
    // status; clear the banner as soon as the call terminates.
    ws.on('call_status_update', (event: any) => {
      const status: string = (event.payload?.status || '').toUpperCase();
      if (
        status === 'RINGING' ||
        status === 'WAITING' ||
        status === 'WAITING_AGENT' ||
        status === 'CONNECTING'
      ) {
        showIncomingFromEvent(event);
      } else if (
        status === 'ENDED' ||
        status === 'COMPLETED' ||
        status === 'MISSED' ||
        status === 'REJECTED'
      ) {
        setIncomingCall((cur) => {
          const sID = event?.payload?.session_id || event?.session_id;
          if (cur && sID && cur.session_id === sID) {
            if (missedCallTimerRef.current) {
              clearTimeout(missedCallTimerRef.current);
              missedCallTimerRef.current = null;
            }
            return null;
          }
          return cur;
        });
      }
    });

    return () => {
      // Cleanup chỉ khi thực sự unmount (token/user đổi), không phải mỗi render
      try { ws.disconnect(); } catch (_) {}
      wsRef.current = null;
      if (missedCallTimerRef.current) {
        clearTimeout(missedCallTimerRef.current);
        missedCallTimerRef.current = null;
      }
    };
    // NOTE: incomingCall intentionally NOT in deps — including it would re-create
    // the WS (and thus all handlers) on every ring, instantly tearing down the
    // banner the moment it appears.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasHydrated, publicPath, token, user?.username, user?.role, queryClient, addToast]);

  // Initialize team agent guest call notifications hook
  // MUST be declared before any early `return` to satisfy Rules of Hooks.
  const handleTeamAgentTakeCall = useCallback((sessionId: string, _guestName: string) => {
    // Navigate to the session or open the case
    router.push(`/admin/cases?session=${sessionId}`);
  }, [router]);

  const {
    pendingGuestCalls,
    dismissCall,
    handleTakeCall,
  } = useTeamAgentNotifications(wsRef, handleTeamAgentTakeCall);

  // Click-to-call handlers
  const handleAnswerCall = useCallback(async (callId: number, sessionId: string) => {
    if (missedCallTimerRef.current) {
      clearTimeout(missedCallTimerRef.current);
      missedCallTimerRef.current = null;
    }
    try {
      const { api } = await import('@/lib/api');
      const agentExtension = resolveAgentExtension(user?.username);
      // Triggers ARI: backend originates an agent leg and bridges it.
      await api.webrtcAcceptCall(callId, sessionId, agentExtension).catch(() => {});
      // If sip.js has already received the INVITE (race), answer now;
      // otherwise the SoftphoneBanner will appear when it arrives.
      try {
        if (sipActive) await sip.answer();
      } catch (_) { /* INVITE may not have arrived yet */ }
    } catch (_) {
      /* noop */
    }
  }, [user?.username, sip, sipActive]);

  const handleDeclineCall = useCallback(async (callId: number | undefined, sessionId: string) => {
    if (missedCallTimerRef.current) {
      clearTimeout(missedCallTimerRef.current);
      missedCallTimerRef.current = null;
    }
    try {
      const { api } = await import('@/lib/api');
      if (callId) {
        await api.markMissedCall(callId, sessionId).catch(() => {});
      } else {
        await api.declineCall(sessionId).catch(() => {});
      }
    } catch (_) {
      /* noop */
    }
    setIncomingCall(null);
  }, []);

  // Show loading screen until hydration completes
  if (!hasHydrated) {
    return <AdminLoadingScreen />;
  }

  // Public paths render without layout chrome
  if (publicPath) {
    return <>{children}</>;
  }

  // Show loading screen while redirecting unauthenticated users
  if (!isAuthenticated) {
    return <AdminLoadingScreen />;
  }

  const cases = casesData?.cases ?? [];
  const voiceCalls = voiceCallsData?.calls ?? [];
  const pendingLearning = learningData?.pending_items ?? [];
  const permissions = permissionsData ?? [];

  const waitingCasesCount = cases.filter((c: any) => c.status === 'NEEDS_HUMAN_CS').length;
  const role = user?.role ?? '';

  const handleLogout = async () => {
    try {
      const { api } = await import('@/lib/api');
      await api.logout();
    } catch (_) {}
    useAuthStore.getState().logout();
    router.push('/admin/login');
  };

  return (
    <div className={styles.shell}>
      <AdminSidebar
        user={user}
        role={role}
        permissions={permissions}
        waitingCasesCount={waitingCasesCount}
        customersCount={0}
        voiceCallsCount={voiceCalls.length}
        pendingLearningCount={pendingLearning.length}
        onLogout={handleLogout}
      />

      <main className={styles.main}>{children}</main>

      <VoiceHistoryModal
        isOpen={showVoiceHistoryModal}
        calls={voiceCalls}
        isLoading={false}
        onClose={() => setShowVoiceHistoryModal(false)}
      />

      <ErrorCenterWrapper
        errors={systemErrors}
        onMarkAsHandled={() => {}}
      />

      <ToastErrorBanner
        error={toastError}
        onClose={() => setToastError(null)}
        onViewDetails={() => setShowErrorCenter(true)}
      />

      {/* Click-to-call incoming banner (display only — admin picks up via SIP/softphone) */}
      <IncomingCallBanner
        incomingCall={incomingCall}
        onAnswer={handleAnswerCall}
        onDecline={handleDeclineCall}
      />

      {/* Asterisk-native WebRTC softphone — rings via sip.js when ARI
          originates the agent leg.  Audio is handled by Asterisk + DTLS-SRTP. */}
      <SoftphoneBanner
        incoming={sipIncoming}
        state={sip.state}
        error={sip.error}
        onAnswer={sipAnswer}
        onDecline={sipDecline}
        onHangup={sipHangup}
        active={sipActive}
        pageLabel={pathname}
      />

      {/* Team Agent Guest Call Notifications */}
      {pendingGuestCalls.map((call) => (
        <TeamAgentGuestCallBanner
          key={call.session_id}
          call={call}
          onTakeCall={handleTakeCall}
          onDismiss={dismissCall}
        />
      ))}
    </div>
  );
}

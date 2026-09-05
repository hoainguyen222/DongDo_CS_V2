'use client';

import React, { useEffect, useRef, useState, useCallback } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { useAuthStore } from '@/lib/stores/authStore';
import { useRolePermissions, useVoiceCalls, usePendingLearning, useCases } from '@/lib/hooks/useApi';
import { WSClient } from '@/lib/ws';
import {
  AdminSidebar,
  useAdminWebRTC,
  useTeamAgentNotifications,
  IncomingCallBanner,
  ActiveCallBar,
  TeamAgentGuestCallBanner,
} from '@/components/admin/AdminSidebar';
import { VoiceHistoryModal, ToastErrorBanner, ErrorCenterWrapper } from '@/components/admin/AdminModals';
import { MessageAlertBanner } from '@/components/admin/MessageAlertBanner';
import { useUIStore } from '@/lib/stores/uiStore';
import { AdminLoadingScreen } from './AdminLoadingScreen';
import styles from './AdminLayout.module.scss';

// Public admin routes — bypass auth & layout chrome entirely
const ADMIN_PUBLIC_PATHS = ['/admin/login'];

function isAdminPublic(pathname: string): boolean {
  return ADMIN_PUBLIC_PATHS.some((p) => pathname === p || pathname.startsWith(p + '/'));
}

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const publicPath = isAdminPublic(pathname);

  const { token, user, isAuthenticated, hasHydrated } = useAuthStore();
  const { setShowErrorCenter, systemErrors } = useUIStore();
  const queryClient = useQueryClient();
  const wsRef = useRef<WSClient | null>(null);

  const [showVoiceHistoryModal, setShowVoiceHistoryModal] = useState(false);
  const [toastError, setToastError] = useState<{ title: string; source: string; details: string } | null>(null);

  // Call state
  const [callActiveSession, setCallActiveSession] = useState<string | null>(null);

  // Handle call end - invalidate voice calls cache
  const handleCallEnd = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['voiceCalls'] });
    setCallActiveSession(null);
  }, [queryClient]);

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
  useEffect(() => {
    if (!hasHydrated) return;
    if (publicPath) return;
    if (!token || !user) return;

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
    });

    return () => {
      ws.disconnect();
      wsRef.current = null;
    };
  }, [hasHydrated, publicPath, token, user?.username, user?.role, queryClient]);

  // Initialize WebRTC hook for handling incoming calls
  // MUST be declared before any early `return` to satisfy Rules of Hooks.
  const {
    incomingCall,
    isCallActive,
    callDuration,
    isMuted,
    handleAnswerCall,
    handleDeclineCall,
    handleEndCall,
    toggleMute,
  } = useAdminWebRTC(wsRef, 'admin', handleCallEnd);

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

      {/* Floating call UI */}
      <IncomingCallBanner
        incomingCall={incomingCall}
        onAnswer={handleAnswerCall}
        onDecline={handleDeclineCall}
      />
      <ActiveCallBar
        isCallActive={isCallActive}
        callDuration={callDuration}
        isMuted={isMuted}
        onToggleMute={toggleMute}
        onEndCall={handleEndCall}
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

      {/* Floating Message Alert Banner */}
      <MessageAlertBanner />
    </div>
  );
}

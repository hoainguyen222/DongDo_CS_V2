'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import Image from 'next/image';
import {
  LogOut,
  Phone,
  Mic,
  MicOff,
  PhoneOff,
  Users,
  X,
  Headphones,
} from 'lucide-react';
import { WSClient, acquireWSClient, releaseWSClient, getSharedWSClient } from '@/lib/ws';
import { WebRTCManager } from '@/lib/webrtc';
import { useUIStore } from '@/lib/stores/uiStore';
import { useAuthStore } from '@/lib/stores/authStore';
import styles from './AdminSidebar.module.scss';

// ── RBAC Helpers ───────────────────────────────────────────
export function getNormalizedRole(role?: string): string {
  if (!role) return 'Staff';
  const r = role.trim().toLowerCase();
  if (r.includes('owner')) return 'Owner';
  if (r.includes('admin')) return 'Admin';
  if (r.includes('leader')) return 'Leader';
  return 'Staff';
}

export type PermissionLevel = 'act' | 'view' | 'none';

export function getFeaturePermission(
  featureKey: string,
  role?: string,
  permissions: { role_name: string; feature_key: string; permission_level: string }[] = []
): PermissionLevel {
  if (!role) return 'act';
  const normRole = getNormalizedRole(role);
  if (normRole === 'Owner') return 'act';

  const item = permissions.find(
    (p) =>
      p.role_name.toLowerCase() === normRole.toLowerCase() &&
      p.feature_key === featureKey
  );
  if (item) return item.permission_level as PermissionLevel;

  if (featureKey.includes('.')) {
    const parent = featureKey.split('.')[0];
    const parentItem = permissions.find(
      (p) =>
        p.role_name.toLowerCase() === normRole.toLowerCase() &&
        p.feature_key === parent
    );
    if (parentItem) return parentItem.permission_level as PermissionLevel;
  }

  if (normRole === 'Leader') {
    if (featureKey.startsWith('config') || featureKey.startsWith('partner_config')) return 'none';
    if (featureKey.startsWith('partner_config')) return 'view';
    return 'act';
  }
  if (featureKey.startsWith('inbox') || featureKey.startsWith('calls')) return 'act';
  if (
    featureKey.startsWith('partner_dashboard') ||
    featureKey.startsWith('customers') ||
    featureKey.startsWith('knowledge')
  )
    return 'view';
  return 'none';
}

// ── Nav Button ──────────────────────────────────────────────
type BadgeVariant = 'rose' | 'emerald' | 'amber' | 'cyan';

function NavButton({
  href,
  icon,
  label,
  badge,
  badgeVariant = 'rose',
}: {
  href: string;
  icon: React.ReactNode;
  label: string;
  badge?: number;
  badgeVariant?: BadgeVariant;
}) {
  const pathname = usePathname();
  const isActive = pathname === href || (href !== '/admin' && pathname.startsWith(href));

  const badgeStyleClass: Record<BadgeVariant, string> = {
    rose: styles.badgeRose,
    emerald: styles.badgeEmerald,
    amber: styles.badgeAmber,
    cyan: styles.badgeCyan,
  };

  return (
    <Link
      href={href}
      className={`${styles.navLink} ${isActive ? styles.navLinkActive : ''}`}
    >
      <span className={styles.navLinkLeft}>
        <span className={styles.navLinkIcon}>{icon}</span>
        <span>{label}</span>
      </span>
      {!!badge && (
        <span
          className={`${styles.badge} ${badgeStyleClass[badgeVariant]} ${
            badgeVariant === 'rose' ? styles.badgePulse : ''
          }`}
        >
          {badge}
        </span>
      )}
    </Link>
  );
}

// ── Call Components (kept inside this file for cohesion) ─────
// Reads the latest pending call from the UI store. This makes the
// banner robust against React state races: the missed-call timer clears
// `incomingCall` (hook state) after 60s while the call is still valid,
// but the store still has the latest entry and the banner appears.
export function IncomingCallBanner({
  incomingCall,
  onAnswer,
  onDecline,
}: {
  incomingCall: { session_id: string; caller_id: string; offer?: any } | null;
  onAnswer: () => void;
  onDecline: () => void;
}) {
  // Subscribe to the store — when a pending call arrives, this re-renders
  // and the banner appears. The store (pendingCalls) is the source of truth;
  // incomingCall (hook state) can be cleared by the 60s missed-call timeout
  // while the call is still valid, so we always prefer the store entry.
  const pendingCalls = useUIStore((s) => s.pendingCalls);
  const display = pendingCalls[0] || null;
  if (!display) return null;

  return (
    <div className={styles.callBanner}>
      <div className={styles.callBannerIcon}>
        <Phone style={{ width: 24, height: 24 }} />
      </div>
      <div className={styles.callBannerText}>
        <div className={styles.callBannerLabel}>Cuộc gọi thoại đến!</div>
        <div className={styles.callBannerCaller}>{display.caller_id}</div>
      </div>
      <div className={styles.callBannerActions}>
        <button
          onClick={() => {
            // Re-derive the latest pending call at click-time so we never
            // call onAnswer with a stale closure.
            const fresh = useUIStore.getState().pendingCalls[0] || incomingCall;
            if (!fresh) return;
            // Populate the hook state so handleAnswerCall (which reads
            // `incomingCall`) and the WebRTC manager both see the right
            // session_id. The hook also accepts an override argument so
            // we pass the fresh record through.
            (window as any).__adminCall?.answerBySession?.(fresh.session_id) ||
              // Fallback to legacy click handler if window bridge not set.
              onAnswer();
          }}
          className={styles.callAcceptBtn}
        >
          <Phone style={{ width: 16, height: 16 }} />
          <span>Nghe máy</span>
        </button>
        <button onClick={onDecline} className={styles.callDeclineBtn} aria-label="Từ chối cuộc gọi">
          <PhoneOff style={{ width: 16, height: 16 }} />
        </button>
      </div>
    </div>
  );
}

export function ActiveCallBar({
  isCallActive,
  callDuration,
  isMuted,
  onToggleMute,
  onEndCall,
}: {
  isCallActive: boolean;
  callDuration: number;
  isMuted: boolean;
  onToggleMute: () => void;
  onEndCall: () => void;
}) {
  if (!isCallActive) return null;

  const formatTime = (s: number) => {
    const m = Math.floor(s / 60);
    const sec = s % 60;
    return `${m.toString().padStart(2, '0')}:${sec.toString().padStart(2, '0')}`;
  };

  return (
    <div className={styles.activeCallBar}>
      <div className={styles.activeCallIcon}>
        <Phone style={{ width: 20, height: 20 }} />
      </div>
      <div className={styles.activeCallText}>
        <div className={styles.activeCallLabel}>Đang đàm thoại WebRTC</div>
        <div className={styles.activeCallDuration}>{formatTime(callDuration)}</div>
      </div>
      <button onClick={onToggleMute} className={styles.activeCallBtn} aria-label="Bật/tắt mic">
        {isMuted ? (
          <MicOff style={{ width: 16, height: 16, color: '#f87171' }} />
        ) : (
          <Mic style={{ width: 16, height: 16 }} />
        )}
      </button>
      <button onClick={onEndCall} className={styles.endCallBtn}>
        <PhoneOff style={{ width: 16, height: 16 }} />
        <span>Kết thúc</span>
      </button>
    </div>
  );
}

// ── Pagination ───────────────────────────────────────────────
export function Pagination({
  currentPage,
  pageSize,
  totalItems,
  onPageChange,
  onPageSizeChange,
}: {
  currentPage: number;
  pageSize: number;
  totalItems: number;
  onPageChange: (p: number) => void;
  onPageSizeChange: (s: number) => void;
}) {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  if (totalItems === 0) return null;

  return (
    <div className={styles.pagination}>
      <div className={styles.paginationInfo}>
        <span style={{ fontSize: 11 }}>Hiển thị:</span>
        <select
          value={pageSize}
          onChange={(e) => {
            onPageSizeChange(Number(e.target.value));
            onPageChange(1);
          }}
          className={styles.paginationSelect}
        >
          <option value={10}>10 / trang</option>
          <option value={20}>20 / trang</option>
          <option value={50}>50 / trang</option>
        </select>
        <span style={{ fontSize: 11, color: '#64748b' }}>({totalItems} total)</span>
      </div>
      <div className={styles.paginationControls}>
        <button
          onClick={() => onPageChange(Math.max(1, currentPage - 1))}
          disabled={currentPage <= 1}
          className={styles.paginationBtn}
        >
          ◀
        </button>
        <span className={styles.paginationCurrent}>
          {currentPage} / {totalPages}
        </span>
        <button
          onClick={() => onPageChange(Math.min(totalPages, currentPage + 1))}
          disabled={currentPage >= totalPages}
          className={styles.paginationBtn}
        >
          ▶
        </button>
      </div>
    </div>
  );
}

// ── Main Sidebar ────────────────────────────────────────────
export function AdminSidebar({
  user,
  role,
  permissions,
  waitingCasesCount,
  customersCount,
  voiceCallsCount,
  pendingLearningCount,
  onLogout,
}: {
  user: any;
  role: string;
  permissions: any[];
  waitingCasesCount: number;
  customersCount: number;
  voiceCallsCount: number;
  pendingLearningCount: number;
  onLogout: () => void;
}) {
  const navIcon = {
    dashboard: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#c084fc" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <rect x="3" y="3" width="7" height="9" />
        <rect x="14" y="3" width="7" height="5" />
        <rect x="14" y="12" width="7" height="9" />
        <rect x="3" y="16" width="7" height="5" />
      </svg>
    ),
    inbox: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#38bdf8" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <path d="M22 12h-6l-2 3h-4l-2-3H2" />
        <path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" />
      </svg>
    ),
    users: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#34d399" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
        <circle cx="8.5" cy="7" r="4" />
        <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
        <path d="M16 3.13a4 4 0 0 1 0 7.75" />
      </svg>
    ),
    headphones: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#22d3ee" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <path d="M3 18v-6a9 9 0 0 1 18 0v6" />
        <path d="M21 19a2 2 0 0 1-2 2h-1a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2h3zM3 19a2 2 0 0 0 2 2h1a2 2 0 0 0 2-2v-3a2 2 0 0 0-2-2H3z" />
      </svg>
    ),
    brain: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24 2.5 2.5 0 0 1 1.98-3A2.5 2.5 0 0 1 9.5 2Z" />
        <path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24 2.5 2.5 0 0 0-1.98-3A2.5 2.5 0 0 0 14.5 2Z" />
      </svg>
    ),
    book: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#818cf8" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" />
        <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z" />
      </svg>
    ),
    trending: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#34d399" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <polyline points="22 7 13.5 15.5 8.5 10.5 2 17" />
        <polyline points="16 7 22 7 22 13" />
      </svg>
    ),
    sliders: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#38bdf8" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <line x1="4" y1="21" x2="4" y2="14" />
        <line x1="4" y1="10" x2="4" y2="3" />
        <line x1="12" y1="21" x2="12" y2="12" />
        <line x1="12" y1="8" x2="12" y2="3" />
        <line x1="20" y1="21" x2="20" y2="16" />
        <line x1="20" y1="12" x2="20" y2="3" />
        <line x1="1" y1="14" x2="7" y2="14" />
        <line x1="9" y1="8" x2="15" y2="8" />
        <line x1="17" y1="16" x2="23" y2="16" />
      </svg>
    ),
    settings: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#94a3b8" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
      </svg>
    ),
    testTube: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#fb7185" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <path d="M14.5 2v17.5a3.5 3.5 0 1 1-7 0V2" />
        <path d="M9.5 2h7" />
        <path d="M9.5 16h7" />
      </svg>
    ),
  };

  return (
    <aside className={styles.aside}>
      <div className={styles.logoBar}>
        <Image
          src="/logo/Logo Dọc_Trắng.svg"
          alt="Logo Đông Đô Partners"
          width={40}
          height={40}
          className={styles.logoImg}
          onError={(e) => {
            (e.target as HTMLImageElement).style.display = 'none';
          }}
        />
        <div className={styles.logoText}>
          <h2 className={styles.logoTitle}>Đông Đô CS</h2>
          <span className={styles.logoBadge}>STUDIO V2.0 (GO)</span>
        </div>
      </div>

      <nav className={styles.nav}>
        <div className={styles.section}>
          {getFeaturePermission('partner_dashboard', role, permissions) !== 'none' && (
            <NavButton href="/admin" icon={navIcon.dashboard} label="Trang Chủ / Dashboard" />
          )}
          {getFeaturePermission('inbox', role, permissions) !== 'none' && (
            <NavButton
              href="/admin/inbox"
              icon={navIcon.inbox}
              label="Live CS Inbox"
              badge={waitingCasesCount > 0 ? waitingCasesCount : undefined}
              badgeVariant="rose"
            />
          )}
          {getFeaturePermission('customers', role, permissions) !== 'none' && (
            <NavButton
              href="/admin/customers"
              icon={navIcon.users}
              label="Quản Lý Khách Hàng"
              badge={customersCount > 0 ? customersCount : undefined}
              badgeVariant="emerald"
            />
          )}
          {getFeaturePermission('calls', role, permissions) !== 'none' && (
            <NavButton
              href="/admin/calls"
              icon={navIcon.headphones}
              label="Lịch Sử Cuộc Gọi"
              badge={voiceCallsCount > 0 ? voiceCallsCount : undefined}
              badgeVariant="cyan"
            />
          )}
          {getFeaturePermission('learning', role, permissions) !== 'none' && (
            <NavButton
              href="/admin/learning"
              icon={navIcon.brain}
              label="Học Tri Thức Mới"
              badge={pendingLearningCount > 0 ? pendingLearningCount : undefined}
              badgeVariant="amber"
            />
          )}
          {getFeaturePermission('knowledge', role, permissions) !== 'none' && (
            <NavButton href="/admin/knowledge" icon={navIcon.book} label="Kho Tri Thức" />
          )}
          {getFeaturePermission('partner_analytics', role, permissions) !== 'none' && (
            <NavButton href="/admin/analytics" icon={navIcon.trending} label="Báo Cáo & Thống Kê CX" />
          )}
          {getFeaturePermission('partner_config', role, permissions) !== 'none' && (
            <NavButton href="/admin/permissions" icon={navIcon.sliders} label="Cấu Hình & Phân Quyền" />
          )}
          {getFeaturePermission('config', role, permissions) !== 'none' && (
            <NavButton href="/admin/config" icon={navIcon.settings} label="Cấu Hình LLM Studio" />
          )}
          {role?.toLowerCase() === 'owner' && (
            <NavButton href="/admin/test-data" icon={navIcon.testTube} label="Test Data Upload" />
          )}
        </div>
      </nav>

      <div className={styles.footer}>
        <div className={styles.userInfo}>
          <div className={styles.userText}>
            <div className={styles.userName}>{user?.full_name || user?.username}</div>
            <div className={styles.userRole}>{role}</div>
          </div>
          <button
            onClick={onLogout}
            className={styles.logoutBtn}
            aria-label="Đăng xuất"
            title="Đăng xuất"
          >
            <LogOut style={{ width: 16, height: 16 }} />
          </button>
        </div>
      </div>
    </aside>
  );
}

// ── WebRTC Hook for Admin ───────────────────────────────────
const MISSED_CALL_TIMEOUT = 60; // seconds before marking as missed

export function useAdminWebRTC(_wsRef: React.RefObject<WSClient | null> | null, sessionId: string, onCallEnd: () => void) {
  const rtcRef = useRef<WebRTCManager | null>(null);
  const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
  const callTimerRef = useRef<any>(null);
  const missedCallTimerRef = useRef<any>(null);
  // Cleanup table for WS subscribers registered in the effect below.
  // Without storing the unsub functions, re-renders would stack duplicate
  // handlers (since WSClient.on keeps growing the handler Set, and the
  // previous useEffect did not unregister). Symptom: every incoming call
  // triggered N copies of `setIncomingCall` and `pushPendingCall` with
  // Date.now()-based call_ids, so the float banner fought itself and the
  // "answer" button sometimes hooked the wrong call.
  const unsubsRef = useRef<Array<() => void>>([]);
  const [isCallActive, setIsCallActive] = useState(false);
  const [callDuration, setCallDuration] = useState(0);
  const [isMuted, setIsMuted] = useState(false);
  const [incomingCall, setIncomingCall] = useState<{ session_id: string; caller_id: string; call_id?: string | number; offer?: any } | null>(null);
  const [isMissedCall, setIsMissedCall] = useState(false);

  // The hook owns its own WS client so it does not depend on an external
  // wsRef being populated by another useEffect — which would otherwise be
  // a race: hook effect runs before layout's WS-setup effect, sees
  // wsRef.current === null, early-returns and never registers handlers.
  // We re-use the layout's wsRef if it's already there (so we don't open
  // two sockets), otherwise we create our own.
  const user = useAuthStore((s) => s.user);
  const internalWsRef = useRef<WSClient | null>(null);
  const wsClient = _wsRef?.current || internalWsRef.current;

  const clearMissedCallTimer = useCallback(() => {
    if (missedCallTimerRef.current) {
      clearTimeout(missedCallTimerRef.current);
      missedCallTimerRef.current = null;
    }
  }, []);

  const startMissedCallTimer = useCallback((callSessionId: string, _callerId: string, callId?: any) => {
    clearMissedCallTimer();
    setIsMissedCall(false);
    missedCallTimerRef.current = setTimeout(async () => {
      // Mark as missed call if no one answered
      setIsMissedCall(true);
      try {
        const { api } = await import('@/lib/api');
        // Use the dedicated markMissedCall API to set status to MISSED
        if (callId) {
          await api.markMissedCall(callId, callSessionId);
        } else {
          await api.endCall(callSessionId, 0); // Fallback: 0 duration = missed
        }
      } catch (_) {}
      // Also clean the UI store so pendingCalls is cleared — the 60s
      // auto-expire in pushPendingCall only removes entries older than 60s
      // from received_at, but without explicit remove the store would hold
      // a stale entry for this call_id indefinitely.
      useUIStore.getState().removePendingCall(String(callId || `legacy-${callSessionId}`));
      // Auto-clear after showing
      setTimeout(() => {
        setIncomingCall(null);
        setIsMissedCall(false);
      }, 5000);
    }, MISSED_CALL_TIMEOUT * 1000);
  }, [clearMissedCallTimer]);

  const startCallTimer = useCallback(() => {
    clearInterval(callTimerRef.current);
    setCallDuration(0);
    callTimerRef.current = setInterval(() => {
      setCallDuration((p) => p + 1);
    }, 1000);
  }, []);

  // Accept an incoming call. If `overrideCall` is supplied (e.g. clicked
  // from a session-detail page where the float banner has already
  // disappeared), use that instead of the local `incomingCall` state.
  //
  // Call v2 (Asterisk ARI): no SDP offer — the agent acknowledges via
  // POST /api/calls/:id/accept and ARI bridges the media. In this case
  // we skip WebRTC entirely and rely on the backend's `call_connecting`
  // event (fired by AcceptCall → gateway.OriginateChannel) to set
  // isCallActive=true and start the timer.
  // Legacy (WebRTC): offer is present — set up WebRTC normally.
  const handleAnswerCall = useCallback(async (overrideCall?: { session_id: string; caller_id: string; call_id?: any; offer?: any }) => {
    const target = overrideCall || incomingCall;
    const client = _wsRef?.current || internalWsRef.current;
    if (!target || !client) return;
    clearMissedCallTimer();
    const callData = { ...target };
    // Replace the local state with the override so subsequent state
    // changes (e.g. handleEndCall) read the same data.
    setIncomingCall(callData);
    setIsMissedCall(false);

    if (callData.offer) {
      // Legacy WebRTC call — set up peer connection.
      setIsCallActive(true);
      startCallTimer();
      const rtc = new WebRTCManager(client, callData.session_id, (state: any) => {
        if (state === 'connected') startCallTimer();
        else if (state === 'ended') {
          setIsCallActive(false);
          setIncomingCall(null);
          clearInterval(callTimerRef.current);
          setCallDuration(0);
          if (callData.call_id) useUIStore.getState().removePendingCall(String(callData.call_id));
          onCallEnd();
        }
      }, (stream: any) => {
        if (remoteAudioRef.current) {
          remoteAudioRef.current.srcObject = stream;
          remoteAudioRef.current.play().catch(() => {});
        }
      });
      rtcRef.current = rtc;
      await rtc.handleOffer(callData.offer);
    } else {
      // Call v2 — no WebRTC needed. POST accept to backend and wait for
      // `call_connecting` event (emitted by AcceptCall → ARI originate).
      // Mark active immediately so the UI reflects the user's intent.
      setIsCallActive(true);
      startCallTimer();
      try {
        const { api } = await import('@/lib/api');
        if (callData.call_id) {
          await api.acceptCall(callData.call_id);
        }
      } catch (_) {
        // Non-fatal: the backend may have already moved on. The call
        // state is reflected in the UI; if accept failed the admin can
        // still see the active bar and end the call if needed.
      }
    }
  }, [incomingCall, _wsRef, internalWsRef, startCallTimer, onCallEnd, clearMissedCallTimer]);

  const handleDeclineCall = useCallback(async () => {
    clearMissedCallTimer();
    const callId = incomingCall?.call_id;
    // Try Call v2 reject first; fall back to legacy decline.
    if (callId) {
      try {
        const { api: apiLib } = await import('@/lib/api');
        await apiLib.rejectCall(String(callId));
      } catch (_) {
        // Fall through to legacy
      }
    }
    // Legacy fallback
    try {
      const { voiceApi } = await import('@/lib/api');
      await voiceApi.declineCall(incomingCall?.session_id || '');
    } catch (_) { /* ignore */ }
    setIncomingCall(null);
    setIsMissedCall(false);
    useUIStore.getState().removePendingCall(String(callId || `legacy-${incomingCall?.session_id}`));
  }, [incomingCall, clearMissedCallTimer]);

  const handleEndCall = useCallback(async () => {
    // Capture callId BEFORE clearing state — incomingCall may be nulled
    // synchronously and if a re-render fires before the async fetch runs,
    // we must still have the id to hit the backend.
    const callId = incomingCall?.call_id;
    const targetSessionId = incomingCall?.session_id || sessionId;

    clearMissedCallTimer();
    setIsCallActive(false);
    setIncomingCall(null);
    clearInterval(callTimerRef.current);
    setCallDuration(0);
    setIsMissedCall(false);
    useUIStore.getState().clearPendingCalls();

    // Call v2: POST /api/calls/:id/hangup. This is the primary path
    // for Call v2 (Asterisk ARI). It tells the backend to transition
    // the call to ENDED and broadcast call_ended_v2 to all sessions.
    //
    // Must use the api client (api.hangupCall) instead of raw fetch —
    // the raw fetch below was the source of the 401 "Vui lòng đăng
    // nhập" error because it didn't send the Authorization header.
    if (callId) {
      try {
        const { api: apiLib } = await import('@/lib/api');
        await apiLib.hangupCall(String(callId));
      } catch (err) {
        // eslint-disable-next-line no-console
        console.warn('[call][handleEndCall] hangup API failed', err);
      }
    }

    // Legacy WebRTC cleanup (only if rtcRef.current is set — Call v2
    // uses Asterisk ARI and has no WebRTC peer connection).
    if (rtcRef.current) {
      await rtcRef.current.endCall(false, callDuration).catch(() => {});
      rtcRef.current = null;
    }

    // Legacy voice API (idempotent — no-op if session doesn't exist).
    try {
      const { voiceApi } = await import('@/lib/api');
      await voiceApi.endCall(targetSessionId, callDuration);
    } catch (_) { /* ignore */ }

    onCallEnd();
  }, [callDuration, sessionId, incomingCall, onCallEnd, clearMissedCallTimer]);

  const toggleMute = useCallback(() => {
    if (rtcRef.current) {
      const muted = rtcRef.current.toggleMute();
      setIsMuted(muted);
      return muted;
    }
    return false;
  }, []);

  // Cleanup internal WS on unmount so we don't leak a connection when the
  // layout re-mounts (HMR / route navigation). External WS is owned by
  // the layout and is left alone here.
  useEffect(() => {
    return () => {
      if (internalWsRef.current && user?.username) {
        // Decrement ref-count on the singleton. If we are the only
        // consumer (no layout / inbox page holding a reference), this
        // is what actually closes the underlying socket.
        releaseWSClient('admin_inbox', user.username, user.role || 'admin');
        internalWsRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.username]);

  // Recover ringing banner after page reload. WS broadcasts are
  // fire-and-forget; if the admin reloads while a call is in flight, the
  // original `incoming_call` event was already missed. We poll the
  // /api/agents/:id/active-calls endpoint on mount and on every WS
  // reconnect so the banner comes back without the agent needing to wait
  // for a fresh call. The recovered call only stays in the UI for
  // MISSED_CALL_TIMEOUT — same timeout as a live ring — so it auto-cleans.
  const recoverActiveCalls = useCallback(async () => {
    const username = user?.username;
    if (!username) return;
    try {
      const { api: apiLib } = await import('@/lib/api');
      const { calls } = await apiLib.getAgentActiveCalls(username);
      if (!calls || calls.length === 0) return;
      // Take the first non-terminal call (most recently updated).
      const active = calls[0];
      if (!active?.id || !active?.customer_id) return;
      // eslint-disable-next-line no-console
      console.info('[call][recover] rehydrating ringing banner from /active-calls', active);
      const callId = String(active.id);
      const sessionId = active.customer_id;
      const callerId = active.customer_id;
      setIncomingCall({ session_id: sessionId, caller_id: callerId, call_id: callId });
      useUIStore.getState().pushPendingCall({
        call_id: callId,
        session_id: sessionId,
        caller_id: callerId,
        received_at: Date.now(),
      });
      startMissedCallTimer(sessionId, callerId, callId);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.warn('[call][recover] failed', err);
    }
  }, [user?.username, startMissedCallTimer]);

  // Run on mount after the user is available.
  useEffect(() => {
    if (!user?.username) return;
    // eslint-disable-next-line no-console
    console.info('[call][recover] running on mount');
    void recoverActiveCalls();
  }, [user?.username, recoverActiveCalls]);

  // Re-fetch active calls whenever the WS reconnects. The /active-calls
  // endpoint is the source of truth; WS events are best-effort.
  useEffect(() => {
    if (!wsClient) return;
    const handleOpen = () => {
      // eslint-disable-next-line no-console
      console.info('[call][recover] WS open, re-fetching active calls');
      void recoverActiveCalls();
    };
    // WSClient doesn't expose onopen; emulate by polling shortly after
    // each reconnect (schedule a probe — cheap and idempotent).
    const id = window.setInterval(() => {
      const c: any = wsClient;
      if (c?.ws && c.ws.readyState === 1 /* OPEN */) {
        handleOpen();
      }
    }, 2000);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wsClient, recoverActiveCalls]);

  useEffect(() => {
    // eslint-disable-next-line no-console
    console.info('[call][ws-hook] effect running, external wsRef =', _wsRef?.current ? 'WSClient' : 'null', 'internal wsRef =', internalWsRef.current ? 'WSClient' : 'null');

    // Use the SHARED singleton so we never open a second WebSocket for
    // the same (session, user, role). The layout and inbox/page both
    // already hold ref-counted references to this singleton; we add
    // our own via acquireWSClient and release it on cleanup.
    let created = false;
    if (user?.username) {
      const existing = _wsRef?.current || getSharedWSClient('admin_inbox', user.username, user.role || 'admin');
      if (!existing) {
        internalWsRef.current = acquireWSClient('admin_inbox', user.username, user.role || 'admin');
        created = true;
        // eslint-disable-next-line no-console
        console.info('[call][ws-hook] acquired own singleton WS client');
      } else {
        // eslint-disable-next-line no-console
        console.info('[call][ws-hook] reusing existing singleton WS client');
      }
    }

    // If still no client (no auth yet), bail until auth lands.
    const client = _wsRef?.current || internalWsRef.current;
    if (!client) {
      // eslint-disable-next-line no-console
      console.info('[call][ws-hook] no WS client yet (no auth?), will retry on next render');
      return;
    }

    // The created client needs a moment to fire its 'open' before we
    // register handlers (handlers fire from `onmessage` → `emit`). The
    // handlers register regardless, but if open fails we unsubscribe
    // below via `created` flag in cleanup.
    void created;

    // Detach anything from a previous effect run before registering again
    // — keeps the WS handler Set bounded to one set per logical session.
    // Symptom of NOT doing this: every incoming call triggered N copies
    // of `setIncomingCall` and `pushPendingCall` (Date.now()-based ids,
    // so each one spawned a fresh pending entry), causing the float
    // banner to flicker / never settle on the right call.
    const previousUnsubs = unsubsRef.current;
    previousUnsubs.forEach((u) => {
      try { u(); } catch (_) { /* noop */ }
    });
    unsubsRef.current = [];

    const register = (event: any, handler: (e: any) => void) => {
      const off = client.on(event, handler);
      unsubsRef.current.push(off);
    };

    const { pushPendingCall } = useUIStore.getState();

    // Wildcard listener for diagnostic logs. Helps confirm that the WS
    // subscription is alive even if a specific handler early-returns.
    register('*', (event: any) => {
      // eslint-disable-next-line no-console
      console.info('[call][ws-hook] * event received:', event?.type, 'session:', event?.session_id, 'payload.session_id:', event?.payload?.session_id);
    });

    register('call_ring', (event: any) => {
      const sID = event.payload?.session_id || event.session_id;
      const cID = event.payload?.caller_id || event.sender_id || 'Khách hàng';
      const offerData = event.payload?.offer || event.payload;
      if (!sID) return;
      // Use session_id as a stable per-call id — legacy flow never
      // sends a real call_id, and `Date.now()` would create a brand
      // new id for every duplicated handler.
      const callId = event.payload?.call_id || event.call_id || `legacy-${sID}`;
      // eslint-disable-next-line no-console
      console.info('[call] call_ring event', { sID, callId, cID });
      setIncomingCall({ session_id: sID, caller_id: cID, call_id: callId, offer: offerData });
      pushPendingCall({ call_id: String(callId), session_id: sID, caller_id: cID, offer: offerData, received_at: Date.now() });
      // Start missed call timer with call_id for proper missed call marking.
      // Guarded against already-active to avoid resetting a real call mid-stream.
      startMissedCallTimer(sID, cID, callId);
    });

    register('call_offer', (event: any) => {
      const sID = event.payload?.session_id || event.session_id;
      const cID = event.payload?.caller_id || event.sender_id || 'Khách hàng';
      if (!sID) return;
      const callId = event.payload?.call_id || event.call_id || `legacy-${sID}`;
      setIncomingCall({ session_id: sID, caller_id: cID, call_id: callId, offer: event.payload });
      pushPendingCall({ call_id: String(callId), session_id: sID, caller_id: cID, offer: event.payload, received_at: Date.now() });
      startMissedCallTimer(sID, cID, callId);
    });

    // Call v2 — backend (call.UseCase.publishAgentEvent) emits
    // `incoming_call` to session `agent:<username>` AND, after the hub
    // whitelist fix, also mirrors to `admin_inbox`. Show the banner.
    // Note: Call v2 uses Asterisk ARI for media, so there is no SDP
    // `offer` here — the agent just needs to acknowledge via
    // POST /api/calls/:id/accept and the ARI bridge carries the audio.
    register('incoming_call', (event: any) => {
      const payload = event.payload || {};
      const sID = payload.customer_id || payload.session_id || event.session_id;
      const callId = payload.call_id || event.call_id;
      const cID = payload.customer_id || event.sender_id || 'Khách hàng';
      // eslint-disable-next-line no-console
      console.info('[call] incoming_call event', { sID, callId, cID, payload });
      if (!sID) return;
      setIncomingCall({ session_id: sID, caller_id: cID, call_id: callId });
      pushPendingCall({ call_id: String(callId || `v2-${sID}`), session_id: sID, caller_id: cID, received_at: Date.now() });
      startMissedCallTimer(sID, cID, callId);
    });

    // Call v2 — backend also emits `call_waiting` to the customer's
    // session and to admin_inbox. Used for "queue updated" toast/state.
    register('call_waiting', (event: any) => {
      const payload = event.payload || {};
      const sID = payload.customer_id || payload.session_id || event.session_id;
      if (!sID) return;
      const callId = payload.call_id || event.call_id || `waiting-${sID}`;
      setIncomingCall({
        session_id: sID,
        caller_id: payload.customer_id || 'Khách hàng',
        call_id: callId,
      });
      pushPendingCall({ call_id: String(callId), session_id: sID, caller_id: payload.customer_id || 'Khách hàng', received_at: Date.now() });
      startMissedCallTimer(sID, payload.customer_id || 'Khách hàng', callId);
    });

    // Call v2: backend fires `call_connecting` when AcceptCall transitions
    // WAITING_AGENT → CONNECTING (ARI bridge created, channels originating).
    // This confirms the backend accepted the call — clear the ringing banner
    // and make sure isCallActive is true (it was set optimistically in
    // handleAnswerCall; this is the server-confirmed signal).
    register('call_connecting', (event: any) => {
      // eslint-disable-next-line no-console
      console.info('[call] call_connecting received, backend accepted call', event?.payload);
      setIsCallActive(true);
      // Clear the ringing banner now that the call is confirmed connecting.
      useUIStore.getState().clearPendingCalls();
    });

    // Call v2: both channels answered → IN_PROGRESS. The ARI bridge is
    // now active; the timer was already started in handleAnswerCall.
    register('call_started', (event: any) => {
      // eslint-disable-next-line no-console
      console.info('[call] call_started — media bridge active', event?.payload);
      setIsCallActive(true);
      useUIStore.getState().clearPendingCalls();
    });

    // Call v2: call failed (Asterisk error, channel not answered, etc.).
    // Revert the optimistic isCallActive=true set in handleAnswerCall.
    register('call_failed', (event: any) => {
      // eslint-disable-next-line no-console
      console.warn('[call] call_failed received', event?.payload);
      setIsCallActive(false);
      setIncomingCall(null);
      setIsMissedCall(false);
      clearMissedCallTimer();
      clearInterval(callTimerRef.current);
      setCallDuration(0);
      rtcRef.current = null;
      useUIStore.getState().clearPendingCalls();
      onCallEnd();
    });

    register('call_end', async () => {
      setIsCallActive(false);
      setIncomingCall(null);
      setIsMissedCall(false);
      clearMissedCallTimer();
      clearInterval(callTimerRef.current);
      setCallDuration(0);
      rtcRef.current = null;
      // Also clear any pending entries (call ended).
      useUIStore.getState().clearPendingCalls();
      onCallEnd();
    });

    // Backend also pushes `call_ended_v2` from the use case (Call v2). Make
    // sure both legacy (`call_end`) and v2 (`call_ended_v2`) clear pending state.
    register('call_ended_v2', () => {
      setIsCallActive(false);
      setIncomingCall(null);
      setIsMissedCall(false);
      clearMissedCallTimer();
      clearInterval(callTimerRef.current);
      setCallDuration(0);
      rtcRef.current = null;
      useUIStore.getState().clearPendingCalls();
      onCallEnd();
    });

    return () => {
      clearInterval(callTimerRef.current);
      clearMissedCallTimer();
      // Unbind the WS handlers we registered so re-runs of this effect
      // don't keep stacking duplicates (each `on()` call appends to a
      // Set inside WSClient and has no idempotency guard).
      const unsubs = unsubsRef.current;
      unsubsRef.current = [];
      unsubs.forEach((u) => {
        try { u(); } catch (_) { /* noop */ }
      });
    };
    // NOTE: isCallActive / incomingCall intentionally NOT in deps — they
    // change on every event, which would tear down and rebuild handlers.
    // Reading from setters inside the handlers uses functional updates so
    // the latest state always wins without re-binding.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.username, _wsRef, onCallEnd, startMissedCallTimer, clearMissedCallTimer]);

  return {
    rtcRef,
    remoteAudioRef,
    isCallActive,
    callDuration,
    isMuted,
    incomingCall,
    isMissedCall,
    handleAnswerCall,
    handleDeclineCall,
    handleEndCall,
    toggleMute,
  };
}

// ── Types for Team Agent Guest Call Notifications ────────────
export interface TeamAgentGuestCall {
  session_id: string;
  guest_name: string;
  guest_id: string;
  call_id?: number;
  timestamp: string;
}

// ── Hook for Team Agent Guest Call Notifications ────────────
export function useTeamAgentNotifications(
  wsRef: React.RefObject<WSClient | null>,
  onTakeCall?: (sessionId: string, guestName: string) => void
) {
  const [pendingGuestCalls, setPendingGuestCalls] = useState<TeamAgentGuestCall[]>([]);

  useEffect(() => {
    if (!wsRef.current) return;

    const handleTeamAgentCall = (event: any) => {
      const guestCall: TeamAgentGuestCall = {
        session_id: event.payload?.session_id || event.session_id,
        guest_name: event.payload?.guest_name || event.guest_name || 'Khách hàng',
        guest_id: event.payload?.guest_id || event.guest_id || '',
        call_id: event.payload?.call_id || event.call_id,
        timestamp: event.payload?.timestamp || event.timestamp || new Date().toISOString(),
      };
      // Avoid duplicates
      setPendingGuestCalls((prev) => {
        if (prev.some((c) => c.session_id === guestCall.session_id)) return prev;
        return [...prev, guestCall];
      });
    };

    const unsubscribe = wsRef.current.on('team_agent_call', handleTeamAgentCall);

    return () => {
      unsubscribe();
    };
  }, [wsRef]);

  const dismissCall = useCallback((sessionId: string) => {
    setPendingGuestCalls((prev) => prev.filter((c) => c.session_id !== sessionId));
  }, []);

  const handleTakeCall = useCallback((call: TeamAgentGuestCall) => {
    onTakeCall?.(call.session_id, call.guest_name);
    dismissCall(call.session_id);
  }, [onTakeCall, dismissCall]);

  return {
    pendingGuestCalls,
    dismissCall,
    handleTakeCall,
  };
}

// ── Team Agent Guest Call Banner Component ──────────────────
export function TeamAgentGuestCallBanner({
  call,
  onTakeCall,
  onDismiss,
}: {
  call: TeamAgentGuestCall;
  onTakeCall: (call: TeamAgentGuestCall) => void;
  onDismiss: (sessionId: string) => void;
}) {
  const sessionPreview = call.session_id.length > 12
    ? call.session_id.slice(0, 12) + '...'
    : call.session_id;

  return (
    <div className={styles.teamAgentBanner}>
      <div className={styles.teamAgentIcon}>
        <Headphones style={{ width: 20, height: 20 }} />
      </div>
      <div className={styles.teamAgentText}>
        <div className={styles.teamAgentLabel}>Khách hàng đang chờ</div>
        <div className={styles.teamAgentGuest}>{call.guest_name}</div>
        <div className={styles.teamAgentSession}>{sessionPreview}</div>
      </div>
      <div className={styles.teamAgentActions}>
        <button
          onClick={() => onTakeCall(call)}
          className={styles.teamAgentBtn}
        >
          <Phone style={{ width: 14, height: 14 }} />
          <span>Nhận cuộc gọi</span>
        </button>
        <button
          onClick={() => onDismiss(call.session_id)}
          className={styles.teamAgentDismissBtn}
          aria-label="Bỏ qua thông báo"
        >
          <X style={{ width: 16, height: 16 }} />
        </button>
      </div>
    </div>
  );
}

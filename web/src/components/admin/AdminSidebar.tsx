'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import Image from 'next/image';
import {
  LogOut,
  Phone,
  PhoneOff,
  Users,
  X,
  Headphones,
  ExternalLink,
} from 'lucide-react';
import { WSClient } from '@/lib/ws';
import { buildSipUri } from '@/lib/audioStream';
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

// ── Incoming Call Banner (click-to-call display only) ────────
// Admin does NOT pick up in the browser. The backend Asterisk
// bridges the call to the agent's SIP client / softphone /
// desk phone. This banner only:
//   - announces the incoming call (with caller name + phone)
//   - offers "Mở softphone" → builds a SIP URI for the agent's
//     default SIP handler
//   - offers "Từ chối" → marks the call as missed via REST API
export function IncomingCallBanner({
  incomingCall,
  onAnswer,
  onDecline,
  asteriskHost,
}: {
  incomingCall: { session_id: string; caller_id: string; call_id?: number; phone?: string } | null;
  onAnswer: (callId: number, sessionId: string) => void;
  onDecline: (callId: number | undefined, sessionId: string) => void;
  asteriskHost?: string;
}) {
  if (!incomingCall) return null;

  const sipUri = buildSipUri(incomingCall.phone || incomingCall.caller_id || 'guest', asteriskHost);

  const handleAnswer = () => {
    if (incomingCall.call_id && onAnswer) {
      onAnswer(incomingCall.call_id, incomingCall.session_id);
    }
    // Also open SIP URI in default handler.
    if (typeof window !== 'undefined') {
      try {
        window.location.href = sipUri;
      } catch (_) {
        /* noop */
      }
    }
  };

  const handleDecline = () => {
    onDecline(incomingCall.call_id, incomingCall.session_id);
  };

  return (
    <div className={styles.callBanner}>
      <div className={styles.callBannerIcon}>
        <Phone style={{ width: 24, height: 24 }} />
      </div>
      <div className={styles.callBannerText}>
        <div className={styles.callBannerLabel}>Cuộc gọi thoại đến!</div>
        <div className={styles.callBannerCaller}>{incomingCall.caller_id}</div>
        {incomingCall.phone && (
          <div className={styles.callBannerPhone}>{incomingCall.phone}</div>
        )}
      </div>
      <div className={styles.callBannerActions}>
        <button onClick={handleAnswer} className={styles.callAcceptBtn}>
          <ExternalLink style={{ width: 16, height: 16 }} />
          <span>Mở softphone</span>
        </button>
        <button onClick={handleDecline} className={styles.callDeclineBtn} aria-label="Từ chối cuộc gọi">
          <PhoneOff style={{ width: 16, height: 16 }} />
        </button>
      </div>
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

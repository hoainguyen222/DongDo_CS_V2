'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import Image from 'next/image';
import {
  LogOut,
  Phone,
  Headphones,
  Mic,
  MicOff,
  PhoneOff,
  PhoneCall,
  X,
  CheckCircle2,
  Clock,
  Minimize2,
  Maximize2,
  Bell,
} from 'lucide-react';
import { useAuthStore } from '@/lib/stores/authStore';
import { WSClient } from '@/lib/ws';
import { WebRTCManager } from '@/lib/webrtc';
import { acceptCall } from '@/lib/api/callService';
import { useHelperCases } from '@/lib/hooks/useApi';
import styles from './AdminSidebar.module.scss';

function formatCallTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
}

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

// ── Call Components ─────────────────────────────────────────
export function IncomingCallBanner({
  incomingCall,
  onAnswer,
  onDecline,
}: {
  incomingCall: { session_id: string; caller_id: string; offer?: any } | null;
  onAnswer: () => void;
  onDecline: () => void;
}) {
  if (!incomingCall) return null;

  return (
    <div className={styles.callBanner}>
      <div className={styles.callBannerIcon}>
        <Phone style={{ width: 24, height: 24 }} />
      </div>
      <div className={styles.callBannerText}>
        <div className={styles.callBannerLabel}>Cuộc gọi thoại đến!</div>
        <div className={styles.callBannerCaller}>{incomingCall.caller_id}</div>
      </div>
      <div className={styles.callBannerActions}>
        <button onClick={onAnswer} className={styles.callAcceptBtn}>
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
  pendingCalls = [],
  onAcceptPendingCall,
  onLogout,
  webRtc,
}: {
  user: any;
  role: string;
  permissions: any[];
  waitingCasesCount: number;
  customersCount: number;
  voiceCallsCount: number;
  pendingLearningCount: number;
  pendingCalls?: any[];
  onAcceptPendingCall?: (call: any) => void;
  onLogout: () => void;
  webRtc?: any;
}) {
  const [showPendingModal, setShowPendingModal] = useState(false);
  const { data: helperData } = useHelperCases();
  const helperCasesCount = helperData?.cases?.length || 0;

  const hasPending = pendingCalls && pendingCalls.length > 0;

  useEffect(() => {
    if (pendingCalls && pendingCalls.length > 0) {
      setShowPendingModal(true);
    }
  }, [pendingCalls?.length]);

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
    <>
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
              <NavButton href="/admin/dashboard" icon={navIcon.dashboard} label="Trang Chủ / Dashboard" />
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
            <NavButton
              href="/admin/helper-cases"
              icon={<Bell style={{ width: 16, height: 16, color: '#f59e0b' }} />}
              label="Case Cần Hỗ Trợ"
              badge={helperCasesCount && helperCasesCount > 0 ? helperCasesCount : undefined}
              badgeVariant="amber"
            />
          </div>
        </nav>

        <div className={styles.footer}>
          <div className={styles.userInfo}>
            <div className={styles.userText}>
              <div className={styles.userName}>{user?.full_name || user?.username}</div>
              <div className={styles.userRole}>{role}</div>
            </div>

            <div className={styles.footerActions}>
              {/* Desk Phone Icon Call Alert Button */}
              <button
                onClick={() => setShowPendingModal(true)}
                className={`${styles.deskPhoneBtn} ${hasPending ? styles.deskPhoneBlink : ''}`}
                aria-label="Cuộc gọi đang chờ"
                title={hasPending ? `${pendingCalls.length} cuộc gọi đang chờ tiếp nhận!` : 'Tổng đài thoại WebRTC'}
              >
                <PhoneCall style={{ width: 18, height: 18 }} />
                {hasPending && (
                  <span className={styles.phoneBadge}>{pendingCalls.length}</span>
                )}
              </button>

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
        </div>
      </aside>

      {/* Pending Calls Modal Popover */}
      {showPendingModal && (
        <div className={styles.pendingModalOverlay} onClick={() => setShowPendingModal(false)}>
          <div className={styles.pendingModal} onClick={(e) => e.stopPropagation()}>
            <div className={styles.pendingModalHeader}>
              <h3>
                <PhoneCall style={{ width: 18, height: 18, color: '#10b981' }} />
                <span>Cuộc Gọi Đang Chờ Tiếp Nhận ({pendingCalls.length})</span>
              </h3>
              <button
                onClick={() => setShowPendingModal(false)}
                className={styles.logoutBtn}
                style={{ width: 28, height: 28 }}
              >
                <X style={{ width: 16, height: 16 }} />
              </button>
            </div>

            <div className={styles.pendingModalBody}>
              {!hasPending ? (
                <div style={{ textAlign: 'center', padding: '24px 12px', color: '#94a3b8', fontSize: 13 }}>
                  Hiện tại không có cuộc gọi nào đang chờ tiếp nhận.
                </div>
              ) : (
                pendingCalls.map((call) => (
                  <div key={call.call_id || call.session_id} className={styles.pendingCallItem}>
                    <div className={styles.pendingCallInfo}>
                      <div className={styles.pendingCallerName}>
                        <Phone style={{ width: 14, height: 14, color: '#34d399' }} />
                        <span>{call.caller_id || call.customer_id || 'Khách hàng'}</span>
                      </div>
                      <div className={styles.pendingCallMeta}>
                        Mã phiên: {call.session_id || call.call_id}
                      </div>
                      <div className={styles.pendingCallMeta} style={{ color: '#f59e0b', display: 'flex', alignItems: 'center', gap: 4, marginTop: 4 }}>
                        <Clock style={{ width: 12, height: 12 }} />
                        <span>Đang đổ chuông (Tự hủy sau 45s)</span>
                      </div>
                    </div>

                    {/* ONLY ACCEPT BUTTON - NO DECLINE BUTTON */}
                    <button
                      onClick={() => {
                        if (onAcceptPendingCall) onAcceptPendingCall(call);
                        if (pendingCalls.length <= 1) setShowPendingModal(false);
                      }}
                      className={styles.acceptCallBtnOnly}
                    >
                      <CheckCircle2 style={{ width: 16, height: 16 }} />
                      <span>Tiếp nhận</span>
                    </button>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}

      {/* Remote Audio element for Staff */}
      {webRtc?.remoteAudioRef && (
        <audio ref={webRtc.remoteAudioRef} autoPlay style={{ display: 'none' }} />
      )}

      {/* Active In-Call Modal (Staff) */}
      {webRtc?.isCallActive && !webRtc?.isCallMinimized && (
        <div className={styles.activeCallModalOverlay}>
          <div className={styles.activeCallModal}>
            <div className={styles.activeCallHeader}>
              <div className={styles.activeCallIconPulse}>
                <Headphones style={{ width: 32, height: 32, color: '#10b981' }} />
              </div>
              <div className={styles.activeCallBadge}>
                <span className={styles.activeDotPulse} />
                <span>Đang nghe máy • Tư vấn khách hàng</span>
              </div>
              <h3 className={styles.activeCallerName}>
                {webRtc.activeCallInfo?.caller_id || 'Khách hàng'}
              </h3>
              <p className={styles.activeCallSession}>
                Mã phiên: {webRtc.activeCallInfo?.session_id || '—'}
              </p>
            </div>

            <div className={styles.activeCallTimerBox}>
              <Clock style={{ width: 20, height: 20, color: '#38bdf8' }} />
              <span className={styles.activeCallTimerText}>{formatCallTime(webRtc.callDuration || 0)}</span>
            </div>

            <div className={styles.activeCallActions}>
              <button
                onClick={webRtc.toggleMute}
                className={`${styles.activeCallBtn} ${webRtc.isMuted ? styles.btnMuted : styles.btnUnmuted}`}
                title={webRtc.isMuted ? 'Mở micro' : 'Tắt micro'}
              >
                {webRtc.isMuted ? <MicOff style={{ width: 18, height: 18 }} /> : <Mic style={{ width: 18, height: 18 }} />}
                <span>{webRtc.isMuted ? 'Đã tắt mic' : 'Mic bật'}</span>
              </button>

              <button
                onClick={() => webRtc.setIsCallMinimized && webRtc.setIsCallMinimized(true)}
                className={`${styles.activeCallBtn} ${styles.btnMinimize}`}
                title="Thu nhỏ để vừa tư vấn vừa tra cứu thông tin"
              >
                <Minimize2 style={{ width: 18, height: 18 }} />
                <span>Ẩn cuộc gọi</span>
              </button>

              <button
                onClick={webRtc.handleEndCall}
                className={`${styles.activeCallBtn} ${styles.btnEndCall}`}
                title="Kết thúc cuộc gọi"
              >
                <PhoneOff style={{ width: 18, height: 18 }} />
                <span>Kết thúc</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Minimized Floating Call Bar (Staff) */}
      {webRtc?.isCallActive && webRtc?.isCallMinimized && (
        <div className={styles.floatingCallBar}>
          <div className={styles.floatingCallPulseIcon}>
            <PhoneCall style={{ width: 16, height: 16, color: '#10b981' }} />
          </div>
          <div className={styles.floatingCallMeta}>
            <span className={styles.floatingCaller}>
              {webRtc.activeCallInfo?.caller_id || 'Khách hàng'}
            </span>
            <span className={styles.floatingTimer}>
              {formatCallTime(webRtc.callDuration || 0)}
            </span>
          </div>
          <div className={styles.floatingCallActions}>
            <button
              onClick={() => webRtc.setIsCallMinimized && webRtc.setIsCallMinimized(false)}
              className={styles.floatingBtnRestore}
              title="Mở lại popup đàm thoại"
            >
              <Maximize2 style={{ width: 14, height: 14 }} />
              <span>Hiện lại</span>
            </button>
            <button
              onClick={webRtc.handleEndCall}
              className={styles.floatingBtnEnd}
              title="Kết thúc cuộc gọi"
            >
              <PhoneOff style={{ width: 14, height: 14 }} />
              <span>Kết thúc</span>
            </button>
          </div>
        </div>
      )}
    </>
  );
}

// ── WebRTC Hook for Admin ───────────────────────────────────
export function useAdminWebRTC(wsRef: React.RefObject<WSClient | null>, sessionId: string, onCallEnd: () => void) {
  const rtcRef = useRef<WebRTCManager | null>(null);
  const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
  const callTimerRef = useRef<any>(null);
  const onCallEndRef = useRef(onCallEnd);

  useEffect(() => {
    onCallEndRef.current = onCallEnd;
  }, [onCallEnd]);

  const [isCallActive, setIsCallActive] = useState(false);
  const [isCallMinimized, setIsCallMinimized] = useState(false);
  const [activeCallInfo, setActiveCallInfo] = useState<{ session_id: string; caller_id: string; call_id?: string } | null>(null);
  const [callDuration, setCallDuration] = useState(0);
  const [isMuted, setIsMuted] = useState(false);
  const [incomingCall, setIncomingCall] = useState<{ session_id: string; caller_id: string; offer?: any; call_id?: string } | null>(null);
  const [pendingCalls, setPendingCalls] = useState<any[]>([]);

  const callDurationRef = useRef(0);

  const stopCallTimer = useCallback(() => {
    if (callTimerRef.current) {
      clearInterval(callTimerRef.current);
      callTimerRef.current = null;
    }
  }, []);

  const startCallTimer = useCallback(() => {
    if (callTimerRef.current) {
      clearInterval(callTimerRef.current);
    }
    callTimerRef.current = setInterval(() => {
      callDurationRef.current += 1;
      setCallDuration((prev) => prev + 1);
    }, 1000);
  }, []);

  const handleAnswerCall = useCallback(async (callToAnswer?: any) => {
    const targetCall = callToAnswer || incomingCall;
    if (!targetCall || !wsRef.current) return;

    // Trigger Call Service accept API
    if (targetCall.call_id) {
      acceptCall(targetCall.call_id, 'CSKH_Agent').catch(() => {});
    }

    setIsCallActive(true);
    setIsCallMinimized(false);
    setActiveCallInfo({
      session_id: targetCall.session_id,
      caller_id: targetCall.caller_id || targetCall.customer_id || 'Khách hàng',
      call_id: targetCall.call_id,
    });
    setIncomingCall(null);
    setPendingCalls((prev) => prev.filter((c) => c.session_id !== targetCall.session_id && c.call_id !== targetCall.call_id));
    callDurationRef.current = 0;
    setCallDuration(0);
    startCallTimer();

    const rtc = new WebRTCManager(wsRef.current, targetCall.session_id, (state: any) => {
      if (state === 'connected') {
        startCallTimer();
      } else if (state === 'ended') {
        setIsCallActive(false);
        setIsCallMinimized(false);
        setActiveCallInfo(null);
        setIncomingCall(null);
        stopCallTimer();
        setCallDuration(0);
        callDurationRef.current = 0;
        onCallEndRef.current?.();
      }
    }, (stream: any) => {
      if (remoteAudioRef.current) {
        remoteAudioRef.current.srcObject = stream;
        remoteAudioRef.current.play().catch(() => {});
      }
    });
    rtcRef.current = rtc;
    if (targetCall.offer && targetCall.offer.type && targetCall.offer.sdp) {
      await rtc.handleOffer(targetCall.offer);
    }
  }, [incomingCall, wsRef, startCallTimer, stopCallTimer]);

  const handleDeclineCall = useCallback(() => {
    const targetSession = incomingCall?.session_id || sessionId;
    const { voiceApi } = require('@/lib/api');
    if (targetSession) {
      voiceApi.declineCall(targetSession).catch(() => {});
    }
    setIncomingCall(null);
  }, [incomingCall, sessionId]);

  const handleEndCall = useCallback(async () => {
    const finalDuration = callDurationRef.current;
    const targetSession = activeCallInfo?.session_id || incomingCall?.session_id || sessionId;
    const targetCallID = activeCallInfo?.call_id || incomingCall?.call_id;

    setIsCallActive(false);
    setIsCallMinimized(false);
    setActiveCallInfo(null);
    setIncomingCall(null);
    stopCallTimer();
    setCallDuration(0);
    callDurationRef.current = 0;
    if (rtcRef.current) {
      await rtcRef.current.endCall(false, finalDuration).catch(() => {}); // local cleanup only
      rtcRef.current = null;
    }
    if (targetSession) {
      const { voiceApi } = require('@/lib/api');
      await voiceApi.endCall(targetSession, finalDuration, targetCallID).catch(() => {});
    }
    onCallEndRef.current?.();
  }, [sessionId, activeCallInfo, incomingCall, stopCallTimer]);

  const toggleMute = useCallback(() => {
    if (rtcRef.current) {
      const muted = rtcRef.current.toggleMute();
      setIsMuted(muted);
      return muted;
    }
    return false;
  }, []);

  useEffect(() => {
    let bound = false;

    const bindListeners = () => {
      const ws = wsRef.current;
      if (!ws || bound) return false;

      const addCallToPending = (callData: any) => {
        setPendingCalls((prev) => {
          const exists = prev.some((c) => (c.call_id && c.call_id === callData.call_id) || (c.session_id && c.session_id === callData.session_id));
          if (exists) return prev;
          return [...prev, callData];
        });

        // Automatic 45-second frontend timeout removal
        setTimeout(() => {
          setPendingCalls((prev) => prev.filter((c) => c.session_id !== callData.session_id && c.call_id !== callData.call_id));
        }, 45000);
      };

      ws.on('incoming_call', (event: any) => {
        const payload = event.payload || event;
        const callData = {
          call_id: payload.call_id,
          session_id: payload.session_id || payload.call_id,
          caller_id: payload.customer_id || payload.caller_id || 'Khách hàng',
          offer: (payload.offer && payload.offer.type && payload.offer.sdp) ? payload.offer : undefined,
        };
        setIncomingCall(callData);
        addCallToPending(callData);
      });

      ws.on('call_ring', (event: any) => {
        const sID = event.payload?.session_id || event.session_id;
        const cID = event.payload?.caller_id || event.sender_id || 'Khách hàng';
        const offerData = (event.payload?.offer && event.payload?.offer.type && event.payload?.offer.sdp) ? event.payload.offer : undefined;
        const callData = { session_id: sID, caller_id: cID, offer: offerData, call_id: event.payload?.call_id };
        if (sID) {
          setIncomingCall(callData);
          addCallToPending(callData);
        }
      });

      ws.on('call_offer', (event: any) => {
        const sID = event.payload?.session_id || event.session_id;
        const cID = event.payload?.caller_id || event.sender_id || 'Khách hàng';
        const callData = { session_id: sID, caller_id: cID, offer: event.payload, call_id: event.payload?.call_id };
        if (sID) {
          if (rtcRef.current && activeCallInfo?.session_id === sID) {
            rtcRef.current.handleOffer(event.payload).catch(() => {});
          } else {
            setIncomingCall(callData);
            addCallToPending(callData);
          }
        }
      });

      ws.on('call_end', async (event: any) => {
        const payload = event?.payload || event;
        const endedID = payload?.call_id || payload?.session_id;
        setIsCallActive(false);
        setIsCallMinimized(false);
        setActiveCallInfo(null);
        setIncomingCall(null);
        if (endedID) {
          setPendingCalls((prev) => prev.filter((c) => c.session_id !== endedID && c.call_id !== endedID));
        } else {
          setPendingCalls([]);
        }
        stopCallTimer();
        setCallDuration(0);
        rtcRef.current = null;
        onCallEndRef.current?.();
      });

      bound = true;
      return true;
    };

    if (!bindListeners()) {
      const interval = setInterval(() => {
        if (bindListeners()) {
          clearInterval(interval);
        }
      }, 200);
      return () => clearInterval(interval);
    }
  }, [wsRef, stopCallTimer]);

  return {
    rtcRef,
    remoteAudioRef,
    isCallActive,
    isCallMinimized,
    setIsCallMinimized,
    activeCallInfo,
    callDuration,
    isMuted,
    incomingCall,
    pendingCalls,
    handleAnswerCall,
    handleDeclineCall,
    handleEndCall,
    toggleMute,
  };
}

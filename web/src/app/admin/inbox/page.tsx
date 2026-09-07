'use client';

import React, { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Inbox, Headphones, MessageCircle, RefreshCw } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import {
  useCases,
  useVoiceCalls,
  useClearAllCases,
} from '@/lib/hooks/useApi';
import { useWebSocket } from '@/lib/hooks/useWebSocket';
import { useAuthStore } from '@/lib/stores/authStore';
import { Pagination } from '@/components/admin/AdminSidebar';
import { useUIStore } from '@/lib/stores/uiStore';
import { useListUrlParams } from '@/lib/hooks/useListUrlParams';
import type { ChatCase } from '@/lib/types';
import styles from './page.module.scss';

const STATUS_LABELS: Record<string, string> = {
  AI_ACTIVE: 'AI',
  NEEDS_HUMAN_CS: 'Chờ CSKH',
  HUMAN_CS_ACTIVE: 'Đang CSKH',
  RESOLVED: 'Đã giải quyết',
};

const STATUS_CLASS: Record<string, string> = {
  AI_ACTIVE: styles.statusAi,
  NEEDS_HUMAN_CS: styles.statusNeedsHuman,
  HUMAN_CS_ACTIVE: styles.statusHumanActive,
  RESOLVED: styles.statusResolved,
};

type InboxTab = 'all' | 'NEEDS_HUMAN_CS' | 'HUMAN_CS_ACTIVE' | 'RESOLVED';

export default function InboxPage() {
  const { addToast, openConfirm } = useUIStore();
  const { user } = useAuthStore();
  const queryClient = useQueryClient();
  const router = useRouter();

  const {
    page: casePage,
    limit: casePageSize,
    search: caseFilter,
    status: activeTab,
    setPage: setCasePage,
    setLimit: setCasePageSize,
    setSearch: setCaseFilter,
    setStatus: setActiveTab,
    buildUrl,
  } = useListUrlParams<InboxTab>({
    defaultPage: 1,
    defaultLimit: 10,
    defaultSearch: '',
    defaultStatus: 'all',
    paramNames: { search: 'q', status: 'status', page: 'page', limit: 'limit' },
  });

  // Track last sender from realtime WS so we can flag unread cases
  const [lastSenderMap, setLastSenderMap] = useState<Record<string, string>>({});
  const [showVoiceHistoryModal, setShowVoiceHistoryModal] = useState(false);

  // Data fetching: DB-level pagination & filtering by tab status and search keyword
  const { data: casesData, isLoading: isLoadingCases } = useCases(
    activeTab === 'all' ? '' : activeTab,
    casePage,
    casePageSize,
    caseFilter
  );
  const { data: voiceCallsData } = useVoiceCalls();

  // Real-time WebSocket connection to receive case updates instantly
  // The WS server broadcasts WSEventCaseUpdate to the `admin_inbox` channel whenever
  // any case changes (new message, status change, etc.). This keeps the list in sync
  // without polling the REST API on a timer.
  useWebSocket({
    sessionId: 'admin_inbox',
    username: user?.username || 'admin',
    role: user?.role || 'admin',
    onCaseUpdate: () => {
      queryClient.invalidateQueries({ queryKey: ['cases'] });
    },
    onMessage: (event?: any) => {
      const sid = event?.session_id || event?.payload?.session_id;
      const senderType = event?.sender_type || event?.payload?.sender_type || 'guest';
      const content = event?.content || event?.payload?.content;

      if (sid) {
        setLastSenderMap((prev) => ({ ...prev, [sid]: senderType }));

        // Optimistically update the cached case list
        queryClient.setQueriesData({ queryKey: ['cases'] }, (oldData: any) => {
          if (!oldData || !oldData.cases) return oldData;
          const nowISO = new Date().toISOString();
          const updatedCases = oldData.cases.map((c: ChatCase) => {
            if (c.session_id === sid) {
              return {
                ...c,
                last_message: content ?? c.last_message,
                last_sender_type: senderType,
                updated_at: nowISO,
              };
            }
            return c;
          });
          return { ...oldData, cases: updatedCases };
        });

        queryClient.invalidateQueries({ queryKey: ['cases'] });
      }
    },
  });

  const clearAllMutation = useClearAllCases();

  const allCases = casesData?.cases ?? [];
  const voiceCalls = voiceCallsData?.calls ?? [];

  // Populate lastSenderMap whenever allCases changes
  useEffect(() => {
    allCases.forEach((c: ChatCase) => {
      const st = c.last_sender_type;
      if (st) {
        setLastSenderMap((prev) => {
          if (prev[c.session_id] === st) return prev;
          return { ...prev, [c.session_id]: st };
        });
      }
    });
  }, [allCases]);

  // Robust check: case is unreplied if last sender is the guest
  const isCaseUnreplied = useCallback(
    (c: ChatCase): boolean => {
      if (c.status === 'RESOLVED') return false;
      if (c.status === 'NEEDS_HUMAN_CS') return true;

      if (c.last_sender_type === 'guest') return true;
      if (c.last_sender_type === 'human_cs' || c.last_sender_type === 'cs' || c.last_sender_type === 'ai') return false;

      const mapSender = lastSenderMap[c.session_id];
      if (mapSender === 'guest') return true;
      if (mapSender === 'human_cs' || mapSender === 'cs' || mapSender === 'ai') return false;

      return false;
    },
    [lastSenderMap]
  );

  // Status counts returned from database GetCaseStatusCounts query
  const statusCounts = casesData?.status_counts;
  const unrepliedAllCount = statusCounts?.needs_human ?? 0;
  const waitingCount = statusCounts?.needs_human ?? 0;
  const unrepliedActiveCount = statusCounts?.human_active ?? 0;
  const resolvedCount = statusCounts?.resolved ?? 0;

  // Sorting: unreplied customer conversations pinned at the top within current page
  const pagedCases = [...allCases].sort((a, b) => {
    const unrepliedA = isCaseUnreplied(a);
    const unrepliedB = isCaseUnreplied(b);
    if (unrepliedA !== unrepliedB) {
      return unrepliedA ? -1 : 1;
    }
    return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
  });

  const caseTotal = casesData?.total ?? 0;

  // Click case -> router push to detail page (Next.js route)
  const handleSelectCase = useCallback(
    (c: ChatCase) => {
      router.push(`/admin/cases/${encodeURIComponent(c.session_id)}`);
    },
    [router]
  );

  // Clear all cases
  const handleClearAllCases = () => {
    openConfirm({
      title: 'Xóa toàn bộ case?',
      message: 'Bạn có chắc chắn muốn xóa toàn bộ danh sách case test không?',
      confirmText: 'Xóa hết',
      variant: 'danger',
      onConfirm: async () => {
        try {
          await clearAllMutation.mutateAsync();
          addToast({ title: 'Đã dọn dẹp toàn bộ case', variant: 'success' });
        } catch (err: any) {
          addToast({ title: err.message || 'Lỗi', variant: 'error' });
        }
      },
    });
  };

  return (
    <div className={styles.shell}>
      {/* Header */}
      <div className={styles.header}>
        <div className={styles.headerRow}>
          <div className={styles.title}>
            <div className={styles.titleIcon}>
              <Inbox size={20} />
            </div>
            <div className={styles.titleText}>
              <h2>Live CS Inbox</h2>
              <p>Quản lý hội thoại & tiếp nhận hỗ trợ</p>
            </div>
          </div>
          <button
            onClick={() => setShowVoiceHistoryModal(true)}
            className={styles.voiceHistoryBtn}
          >
            <Headphones size={16} />
            <span>Lịch sử gọi</span>
          </button>
        </div>

        {/* Search and filters */}
        <div className={styles.searchRow}>
          <input
            type="text"
            placeholder="Tìm kiếm..."
            value={caseFilter}
            onChange={(e) => setCaseFilter(e.target.value)}
            className={styles.searchInput}
          />
          <button onClick={handleClearAllCases} className={styles.clearAllBtn}>
            🗑️ Xóa test
          </button>
        </div>
      </div>

      {/* Main content - split view: listPanel (left) + emptyChat (right) */}
      <div className={styles.split}>
        {/* Case list - left panel */}
        <div className={styles.listPanel}>
          {/* Tab selector bar */}
          <div className={styles.filterTabs}>
            <button
              type="button"
              className={`${styles.tabBtn} ${activeTab === 'all' ? styles.tabBtnActive : ''}`}
              onClick={() => setActiveTab('all')}
            >
              <span>Tất cả</span>
              {unrepliedAllCount > 0 && (
                <span className={`${styles.tabBadge} ${styles.tabBadgeRose}`}>
                  {unrepliedAllCount}
                </span>
              )}
            </button>

            <button
              type="button"
              className={`${styles.tabBtn} ${activeTab === 'NEEDS_HUMAN_CS' ? styles.tabBtnActive : ''}`}
              onClick={() => setActiveTab('NEEDS_HUMAN_CS')}
            >
              <span>Chờ CSKH</span>
              {waitingCount > 0 && (
                <span className={`${styles.tabBadge} ${styles.tabBadgeRose}`}>
                  {waitingCount}
                </span>
              )}
            </button>

            <button
              type="button"
              className={`${styles.tabBtn} ${activeTab === 'HUMAN_CS_ACTIVE' ? styles.tabBtnActive : ''}`}
              onClick={() => setActiveTab('HUMAN_CS_ACTIVE')}
            >
              <span>Đang CSKH</span>
              {unrepliedActiveCount > 0 && (
                <span className={`${styles.tabBadge} ${styles.tabBadgeAmber}`}>
                  {unrepliedActiveCount}
                </span>
              )}
            </button>

            <button
              type="button"
              className={`${styles.tabBtn} ${activeTab === 'RESOLVED' ? styles.tabBtnActive : ''}`}
              onClick={() => setActiveTab('RESOLVED')}
            >
              <span>Đã đóng</span>
              {resolvedCount > 0 && (
                <span className={`${styles.tabBadge} ${styles.tabBadgeMuted}`}>
                  {resolvedCount}
                </span>
              )}
            </button>
          </div>

          <div className={styles.listScroll}>
            {isLoadingCases ? (
              <div className={styles.empty}>
                <RefreshCw className={styles.spinIcon} style={{ display: 'inline', marginRight: 6 }} />
                Đang tải...
              </div>
            ) : pagedCases.length === 0 ? (
              <div className={styles.empty}>Không có case nào.</div>
            ) : (
              pagedCases.map((c) => {
                const isUnreplied = isCaseUnreplied(c);
                const statusClass = STATUS_CLASS[c.status] || '';
                return (
                  <Link
                    key={c.id || c.session_id}
                    href={buildUrl(`/admin/cases/${encodeURIComponent(c.session_id)}`)}
                    onClick={() => handleSelectCase(c)}
                    className={`${styles.caseItem} ${isUnreplied ? styles.caseItemUnreplied : ''}`}
                    style={{ textDecoration: 'none', color: 'inherit' }}
                  >
                    <div className={styles.caseRow}>
                      <span className={styles.caseName}>
                        {isUnreplied && <span className={styles.unreadDot} title="Chưa trả lời khách" />}
                        {c.customer_name}
                      </span>
                      <span className={`${styles.statusBadge} ${statusClass}`}>
                        {STATUS_LABELS[c.status] || c.status}
                      </span>
                    </div>
                    <div className={styles.casePreview}>{c.last_message || '...'}</div>
                    <div className={styles.caseMeta}>
                      {new Date(c.updated_at).toLocaleTimeString('vi-VN')}
                      {c.assigned_cs && <span> · 👨‍💼 {c.assigned_cs}</span>}
                    </div>
                  </Link>
                );
              })
            )}
          </div>
          <div className={styles.paginationBar}>
            <Pagination
              currentPage={casePage}
              pageSize={casePageSize}
              totalItems={caseTotal}
              onPageChange={setCasePage}
              onPageSizeChange={setCasePageSize}
            />
          </div>
        </div>

        {/* Empty chat - right panel (placeholder) */}
        <div className={styles.chatPanel}>
          <div className={styles.emptyChat}>
            <div className={styles.emptyChatIcon}>
              <MessageCircle size={32} />
            </div>
            <h3 className={styles.emptyChatTitle}>
              Chọn một case bên trái để bắt đầu hỗ trợ khách hàng
            </h3>
            <p className={styles.emptyChatDesc}>
              Khi AI không thể trả lời, case sẽ tự động xuất hiện ở đây.
              Click vào một case để mở khung chat với WebSocket realtime.
            </p>
          </div>
        </div>
      </div>

      {/* Voice History Modal */}
      {showVoiceHistoryModal && (
        <div className={styles.modalBackdrop} onClick={() => setShowVoiceHistoryModal(false)}>
          <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <h3 className={styles.modalTitle}>🎧 Lịch Sử Cuộc Gọi</h3>
              <button
                onClick={() => setShowVoiceHistoryModal(false)}
                className={styles.modalClose}
              >
                ✕
              </button>
            </div>
            <div className={styles.modalBody}>
              {voiceCalls.length === 0 ? (
                <div className={styles.empty}>Chưa có cuộc gọi nào.</div>
              ) : (
                voiceCalls.map((call: any) => (
                  <div key={call.id} className={styles.callItem}>
                    <div className={styles.callRow}>
                      <span className={styles.callLabel}>
                        {call.caller_type === 'guest' ? '👤 Khách' : '🎧 CSKH'} ({call.caller_id})
                      </span>
                      <span className={styles.callDate}>
                        {new Date(call.created_at).toLocaleString('vi-VN')}
                      </span>
                    </div>
                    <div className={styles.callMeta}>
                      <span>⏱️ {call.duration_seconds}s</span>
                      <span className={styles.callStatus}>{call.status}</span>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

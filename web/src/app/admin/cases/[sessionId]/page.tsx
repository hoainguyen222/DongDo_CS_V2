'use client';

import React, { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { ArrowLeft, Trash2, CheckCircle2, UserCheck, Send, Tag as TagIcon, X, MessageCircle, RefreshCw, Inbox } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import {
  useCaseDetail,
  useCases,
  useTakeCase,
  useResolveCase,
  useDeleteCase,
  useChatTags,
  useCaseTags,
  useAttachTag,
  useDetachTag,
} from '@/lib/hooks/useApi';
import { WSClient } from '@/lib/ws';
import { useAuthStore } from '@/lib/stores/authStore';
import { MarkdownRenderer } from '@/components/MarkdownRenderer';
import { useUIStore } from '@/lib/stores/uiStore';
import { Pagination } from '@/components/admin/AdminSidebar';
import { useListUrlParams } from '@/lib/hooks/useListUrlParams';
import type { Message, QAPair, ChatCase } from '@/lib/types';
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

interface CaseDetailPayload {
  session_id: string;
  customer_name: string;
  customer_phone?: string;
  status: string;
  assigned_cs: string;
  updated_at: string;
}

type CaseTab = 'all' | 'NEEDS_HUMAN_CS' | 'HUMAN_CS_ACTIVE' | 'RESOLVED';

export default function CaseDetailPage() {
  const params = useParams<{ sessionId: string }>();
  const router = useRouter();
  const sessionId = decodeURIComponent(params?.sessionId || '');
  const { addToast, openConfirm } = useUIStore();
  const { user } = useAuthStore();
  const queryClient = useQueryClient();

  const [replyText, setReplyText] = useState('');
  const [isSendingReply, setIsSendingReply] = useState(false);
  const [showResolveModal, setShowResolveModal] = useState(false);
  const [resolveNote, setResolveNote] = useState('');
  const [modalQAPairs, setModalQAPairs] = useState<QAPair[]>([{ question: '', answer: '' }]);
  const [modalEnableLearn, setModalEnableLearn] = useState(true);
  const [currentCase, setCurrentCase] = useState<CaseDetailPayload | null>(null);
  const [lastSenderMap, setLastSenderMap] = useState<Record<string, string>>({});

  // ── listPanel state (URL-synced)
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
  } = useListUrlParams<CaseTab>({
    defaultPage: 1,
    defaultLimit: 10,
    defaultSearch: '',
    defaultStatus: 'all',
    paramNames: { search: 'q', status: 'status', page: 'page', limit: 'limit' },
  });

  const chatContainerRef = useRef<HTMLDivElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WSClient | null>(null);
  const inboxWsRef = useRef<WSClient | null>(null);

  // ── listPanel: fetch cases for the side list
  const { data: casesData, isLoading: isLoadingCases } = useCases(
    activeTab === 'all' ? '' : activeTab,
    casePage,
    casePageSize,
    caseFilter
  );

  const allCases = casesData?.cases ?? [];

  const statusCounts = casesData?.status_counts;
  const unrepliedAllCount = statusCounts?.needs_human ?? 0;
  const waitingCount = statusCounts?.needs_human ?? 0;
  const unrepliedActiveCount = statusCounts?.human_active ?? 0;
  const resolvedCount = statusCounts?.resolved ?? 0;

  // Sync lastSenderMap from listPanel data
  useEffect(() => {
    allCases.forEach((c) => {
      const st = c.last_sender_type;
      if (st) {
        setLastSenderMap((prev) => {
          if (prev[c.session_id] === st) return prev;
          return { ...prev, [c.session_id]: st };
        });
      }
    });
  }, [allCases]);

  // isCaseUnreplied helper (mirrored from inbox)
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

  // Sort: unreplied pinned at top within current page
  const pagedCases = useMemo(() => {
    return [...allCases].sort((a, b) => {
      const unrepliedA = isCaseUnreplied(a);
      const unrepliedB = isCaseUnreplied(b);
      if (unrepliedA !== unrepliedB) return unrepliedA ? -1 : 1;
      return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
    });
  }, [allCases, isCaseUnreplied]);

  const caseTotal = casesData?.total ?? 0;

  // ── WebSocket for current session (chat realtime)
  useEffect(() => {
    if (!sessionId || !user) return;
    const ws = new WSClient(sessionId, user.username, user.role);
    wsRef.current = ws;
    ws.connect();

    const unsubscribeMsg = ws.on('message', () => {
      queryClient.invalidateQueries({ queryKey: ['caseDetail', sessionId] });
    });

    const unsubscribeCaseUpdate = ws.on('case_update', (event: any) => {
      queryClient.invalidateQueries({ queryKey: ['caseDetail', sessionId] });
      queryClient.invalidateQueries({ queryKey: ['cases'] });
      const payload = event?.payload || event;
      if (payload && payload.session_id === sessionId) {
        setCurrentCase((prev) => ({
          session_id: sessionId,
          customer_name: payload.customer_name || prev?.customer_name || 'Khách hàng',
          customer_phone: payload.customer_phone || prev?.customer_phone,
          status: payload.status || prev?.status || 'AI_ACTIVE',
          assigned_cs: payload.assigned_cs || prev?.assigned_cs || '',
          updated_at: prev?.updated_at || new Date().toISOString(),
        }));
      } else if (payload?.type === 'case_deleted' && payload?.session_id === sessionId) {
        router.push('/admin/inbox');
      }
    });

    return () => {
      unsubscribeMsg();
      unsubscribeCaseUpdate();
      ws.disconnect();
      wsRef.current = null;
    };
  }, [sessionId, user?.username, user?.role, queryClient, router]);

  // ── WebSocket for admin_inbox (listPanel realtime)
  useEffect(() => {
    if (!user) return;
    const ws = new WSClient('admin_inbox', user.username, user.role);
    inboxWsRef.current = ws;
    ws.connect();

    const unsubscribe = ws.on('case_update', () => {
      queryClient.invalidateQueries({ queryKey: ['cases'] });
    });

    return () => {
      unsubscribe();
      ws.disconnect();
      inboxWsRef.current = null;
    };
  }, [user?.username, user?.role, queryClient]);

  // ── Initial case fetch via REST (one-time)
  useEffect(() => {
    if (!sessionId) return;
    let cancelled = false;
    (async () => {
      try {
        const { api } = await import('@/lib/api');
        const result = await api.getCaseDetail(sessionId);
        if (!cancelled) {
          setCurrentCase((prev) => ({
            session_id: sessionId,
            customer_name: prev?.customer_name || 'Khách hàng',
            customer_phone: prev?.customer_phone,
            status: result.status || prev?.status || 'AI_ACTIVE',
            assigned_cs: result.assigned_cs || prev?.assigned_cs || '',
            updated_at: prev?.updated_at || new Date().toISOString(),
          }));
        }
      } catch (_) {}
    })();
    return () => {
      cancelled = true;
    };
  }, [sessionId]);

  // ── Fetch customer info from cases list as fallback
  useEffect(() => {
    if (!sessionId || !currentCase || currentCase.customer_name !== 'Khách hàng') return;
    let cancelled = false;
    (async () => {
      try {
        const { api } = await import('@/lib/api');
        const result = await api.listCases('', 1, 100);
        if (cancelled) return;
        const found = result.cases.find((c: any) => c.session_id === sessionId);
        if (found) {
          setCurrentCase({
            session_id: sessionId,
            customer_name: found.customer_name,
            customer_phone: found.customer_phone,
            status: found.status,
            assigned_cs: found.assigned_cs,
            updated_at: found.updated_at,
          });
        }
      } catch (_) {}
    })();
    return () => {
      cancelled = true;
    };
  }, [sessionId, currentCase?.customer_name]);

  // ── Chat history (REST + WS invalidation)
  const { data: caseDetailData } = useCaseDetail(sessionId);
  const caseMessages = caseDetailData?.messages ?? [];

  // ── Tags
  const [showTagPicker, setShowTagPicker] = useState(false);
  const { data: allTags = [] } = useChatTags();
  const { data: attachedTags = [] } = useCaseTags(sessionId);
  const attachTagMutation = useAttachTag();
  const detachTagMutation = useDetachTag();

  const takeCaseMutation = useTakeCase();
  const resolveCaseMutation = useResolveCase();
  const deleteCaseMutation = useDeleteCase();

  // Scroll to bottom when messages change
  useEffect(() => {
    if (chatContainerRef.current) {
      chatContainerRef.current.scrollTop = chatContainerRef.current.scrollHeight;
    }
  }, [caseMessages]);

  // ── Click case in listPanel: navigate within same page (no full reload)
  const handleSelectCase = useCallback(
    (c: ChatCase) => {
      if (c.session_id === sessionId) return;
      router.push(buildUrl(`/admin/cases/${encodeURIComponent(c.session_id)}`));
    },
    [router, sessionId, buildUrl]
  );

  const handleTakeCase = async () => {
    if (!currentCase) return;
    try {
      await takeCaseMutation.mutateAsync(currentCase.session_id);
      addToast({ title: 'Đã tiếp nhận case', variant: 'success' });
    } catch (err: any) {
      addToast({ title: 'Lỗi tiếp nhận case', message: err.message, variant: 'error' });
    }
  };

  const handleSendReply = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!replyText.trim() || !currentCase || isSendingReply) return;
    const content = replyText.trim();
    setReplyText('');
    setIsSendingReply(true);
    try {
      const { api } = await import('@/lib/api');
      await api.sendCSMessage(currentCase.session_id, content);
    } catch (err: any) {
      addToast({ title: err.message || 'Lỗi gửi tin nhắn', variant: 'error' });
    } finally {
      setIsSendingReply(false);
    }
  };

  const extractAllQAPairs = (messages: Message[]): QAPair[] => {
    const pairs: QAPair[] = [];
    let lastUserText = '';
    for (const m of messages) {
      if (m.sender_type === 'guest') {
        lastUserText = m.content.trim();
      } else if ((m.sender_type === 'cs' || m.sender_type === 'human_cs') && lastUserText) {
        const csReply = m.content.trim();
        if (csReply && !csReply.includes('Em đã tham gia cuộc trò chuyện')) {
          const existing = pairs[pairs.length - 1];
          if (existing && existing.question === lastUserText) {
            existing.answer += '\n' + csReply;
          } else {
            pairs.push({ question: lastUserText, answer: csReply });
          }
        }
      }
    }
    return pairs;
  };

  const openResolveModal = () => {
    if (!currentCase) return;
    const extracted = extractAllQAPairs(caseMessages);
    setModalQAPairs(extracted.length > 0 ? extracted : [{ question: '', answer: '' }]);
    setModalEnableLearn(true);
    setResolveNote('');
    setShowResolveModal(true);
  };

  const handleResolveCase = async () => {
    if (!currentCase) return;
    const validPairs = modalEnableLearn
      ? modalQAPairs.filter((p) => p.question.trim() && p.answer.trim())
      : [];
    try {
      await resolveCaseMutation.mutateAsync({
        sessionId: currentCase.session_id,
        resolutionNote: resolveNote,
        extractPairs: validPairs,
      });
      setShowResolveModal(false);
      addToast({ title: 'Đã đóng case thành công!', variant: 'success' });
      router.push(buildUrl('/admin/inbox'));
    } catch (err: any) {
      addToast({ title: err.message || 'Lỗi đóng case', variant: 'error' });
    }
  };

  const handleDeleteCase = () => {
    if (!currentCase) return;
    openConfirm({
      title: 'Xóa case?',
      message: 'Xóa ca hỗ trợ này khỏi Live CS Inbox?',
      confirmText: 'Xóa',
      variant: 'danger',
      onConfirm: async () => {
        try {
          await deleteCaseMutation.mutateAsync(currentCase.session_id);
          addToast({ title: 'Đã xóa case', variant: 'success' });
          router.push(buildUrl('/admin/inbox'));
        } catch (err: any) {
          addToast({ title: err.message || 'Lỗi xóa case', variant: 'error' });
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
          <Link href={buildUrl('/admin/inbox')} className={styles.voiceHistoryBtn}>
            <ArrowLeft size={14} />
            <span>Về danh sách</span>
          </Link>
        </div>

        <input
          type="text"
          placeholder="Tìm kiếm case..."
          value={caseFilter}
          onChange={(e) => setCaseFilter(e.target.value)}
          className={styles.searchInput}
        />
      </div>

      <div className={styles.split}>
        {/* ── listPanel: tab filters + case list ──────────────── */}
        <div className={styles.listPanel}>
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
                const isActive = c.session_id === sessionId;
                const statusClass = STATUS_CLASS[c.status] || '';
                return (
                  <div
                    key={c.id || c.session_id}
                    onClick={() => handleSelectCase(c)}
                    className={`${styles.caseItem} ${isUnreplied ? styles.caseItemUnreplied : ''} ${isActive ? styles.caseItemActive : ''}`}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        handleSelectCase(c);
                      }
                    }}
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
                  </div>
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

        {/* ── chatPanel: realtime messages + actions ─────────── */}
        <div className={styles.chatPanel}>
          <div className={styles.detailHeader}>
            <div className={styles.detailTitle}>
              <div>
                <span className={styles.detailName}>
                  {currentCase?.customer_name || 'Đang tải...'}
                </span>
                {currentCase?.customer_phone && (
                  <span className={styles.phonePill}>
                    📱 {currentCase.customer_phone}
                  </span>
                )}
                <div className={styles.detailSession}>
                  Mã phiên: <code>{sessionId}</code>
                  {currentCase && (
                    <span className={`${styles.statusBadge} ${STATUS_CLASS[currentCase.status] || ''}`} style={{ marginLeft: 8 }}>
                      {STATUS_LABELS[currentCase.status] || currentCase.status}
                    </span>
                  )}
                </div>
                {attachedTags.length > 0 && (
                  <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', marginTop: '6px' }}>
                    {attachedTags.map((t) => (
                      <span
                        key={t.id}
                        style={{
                          background: `${t.color || '#6366f1'}22`,
                          color: t.color || '#6366f1',
                          border: `1px solid ${t.color || '#6366f1'}66`,
                          borderRadius: '12px',
                          padding: '2px 8px',
                          fontSize: '11px',
                          fontWeight: 700,
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '4px',
                        }}
                      >
                        {t.tag_name}
                        <X
                          size={12}
                          style={{ cursor: 'pointer', opacity: 0.8 }}
                          onClick={async (ev) => {
                            ev.stopPropagation();
                            try {
                              await detachTagMutation.mutateAsync({
                                sessionId: currentCase!.session_id,
                                tagId: t.tag_id,
                              });
                              addToast({ title: 'Đã gỡ tag', variant: 'success' });
                            } catch (err: any) {
                              addToast({ title: err.message || 'Lỗi gỡ tag', variant: 'error' });
                            }
                          }}
                        />
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>

            <div className={styles.actionBtnGroup}>
              <div style={{ position: 'relative' }}>
                <button
                  onClick={() => setShowTagPicker(!showTagPicker)}
                  className={styles.secondaryBtn}
                  style={{
                    background: showTagPicker ? 'rgba(99,102,241,0.2)' : undefined,
                    borderColor: showTagPicker ? '#6366f1' : undefined,
                  }}
                >
                  <TagIcon size={14} />
                  <span>Tag ({attachedTags.length})</span>
                </button>

                {showTagPicker && (
                  <div
                    style={{
                      position: 'absolute',
                      top: '100%',
                      right: 0,
                      marginTop: '6px',
                      background: '#0f172a',
                      border: '1px solid rgba(255,255,255,0.15)',
                      borderRadius: '10px',
                      padding: '10px',
                      width: '220px',
                      zIndex: 100,
                      boxShadow: '0 10px 25px rgba(0,0,0,0.5)',
                    }}
                  >
                    <div
                      style={{
                        fontSize: '12px',
                        fontWeight: 700,
                        color: '#94a3b8',
                        marginBottom: '8px',
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                      }}
                    >
                      <span>Gắn / Gỡ Tag</span>
                      <X
                        size={14}
                        style={{ cursor: 'pointer' }}
                        onClick={() => setShowTagPicker(false)}
                      />
                    </div>

                    {allTags.length === 0 ? (
                      <div style={{ fontSize: '12px', color: '#64748b', textAlign: 'center', padding: '12px 0' }}>
                        Chưa có tag nào.
                      </div>
                    ) : (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '180px', overflowY: 'auto' }}>
                        {allTags.map((tag) => {
                          const isAttached = attachedTags.some((at) => at.tag_id === tag.id);
                          return (
                            <button
                              key={tag.id}
                              onClick={async () => {
                                try {
                                  if (isAttached) {
                                    await detachTagMutation.mutateAsync({
                                      sessionId: sessionId,
                                      tagId: tag.id,
                                    });
                                    addToast({ title: `Đã gỡ tag [${tag.name}]`, variant: 'success' });
                                  } else {
                                    await attachTagMutation.mutateAsync({
                                      sessionId: sessionId,
                                      tagId: tag.id,
                                    });
                                    addToast({ title: `Đã gắn tag [${tag.name}]`, variant: 'success' });
                                  }
                                } catch (err: any) {
                                  addToast({ title: err.message || 'Thao tác tag thất bại', variant: 'error' });
                                }
                              }}
                              style={{
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'space-between',
                                padding: '6px 10px',
                                borderRadius: '6px',
                                border: '1px solid rgba(255,255,255,0.06)',
                                background: isAttached ? 'rgba(99,102,241,0.15)' : 'rgba(255,255,255,0.03)',
                                cursor: 'pointer',
                                fontSize: '12px',
                                color: '#fff',
                                transition: 'all 0.15s',
                              }}
                            >
                              <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <span
                                  style={{
                                    width: '8px',
                                    height: '8px',
                                    borderRadius: '50%',
                                    background: tag.color,
                                    display: 'inline-block',
                                  }}
                                />
                                {tag.name}
                              </span>
                              {isAttached && <span style={{ color: '#6366f1', fontWeight: 700 }}>✓</span>}
                            </button>
                          );
                        })}
                      </div>
                    )}
                  </div>
                )}
              </div>

              {currentCase?.status !== 'HUMAN_CS_ACTIVE' && (
                <button
                  onClick={handleTakeCase}
                  disabled={takeCaseMutation.isPending}
                  className={styles.primaryBtn}
                >
                  <UserCheck size={14} />
                  <span>Tiếp Nhận</span>
                </button>
              )}
              <button onClick={openResolveModal} className={styles.secondaryBtn}>
                <CheckCircle2 size={14} />
                <span>Đóng Case</span>
              </button>
              <button onClick={handleDeleteCase} className={styles.deleteBtn}>
                <Trash2 size={14} />
              </button>
            </div>
          </div>

          {/* Messages */}
          <div ref={chatContainerRef} className={styles.messages}>
            {caseMessages.length === 0 ? (
              <div style={{ textAlign: 'center', color: '#94a3b8', padding: 24, fontSize: 13 }}>
                <RefreshCw className={styles.spinIcon} style={{ display: 'inline', marginRight: 8 }} />
                Đang tải tin nhắn qua WebSocket...
              </div>
            ) : (
              caseMessages.map((m, idx) => {
                const isGuest = m.sender_type === 'guest';
                const isCS = m.sender_type === 'cs' || m.sender_type === 'human_cs';
                const rowClass = isGuest ? styles.guest : styles.cs;
                const bubbleClass = isGuest
                  ? styles.guestBubble
                  : isCS
                    ? styles.csBubble
                    : styles.aiBubble;
                return (
                  <div key={m.id || idx} className={`${styles.msgRow} ${rowClass}`}>
                    <div className={styles.msgMeta}>
                      {isGuest
                        ? `👤 ${currentCase?.customer_name || 'Khách'}`
                        : isCS
                          ? '👨‍💼 CSKH'
                          : '🤖 AI'}
                      {' · '}
                      {new Date(m.created_at).toLocaleTimeString('vi-VN', {
                        hour: '2-digit',
                        minute: '2-digit',
                      })}
                    </div>
                    <div className={`${styles.msgBubble} ${bubbleClass}`}>
                      {isGuest ? (
                        <p style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{m.content}</p>
                      ) : (
                        <MarkdownRenderer content={m.content} />
                      )}
                    </div>
                  </div>
                );
              })
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Reply Box */}
          <form onSubmit={handleSendReply} className={styles.replyForm}>
            <div className={styles.replyRow}>
              <textarea
                rows={2}
                value={replyText}
                onChange={(e) => setReplyText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    handleSendReply(e);
                  }
                }}
                placeholder="Nhập tin nhắn phản hồi... (Enter để gửi)"
                className={styles.replyInput}
              />
              <button
                type="submit"
                disabled={!replyText.trim() || isSendingReply}
                className={styles.replySend}
              >
                <Send size={16} />
              </button>
            </div>
          </form>
        </div>
      </div>

      {/* Resolve Case Modal */}
      {showResolveModal && currentCase && (
        <div className={styles.modalBackdrop} onClick={() => setShowResolveModal(false)}>
          <div className={styles.resolveModal} onClick={(e) => e.stopPropagation()}>
            <h3 className={styles.resolveTitle}>🎯 Đóng & Giải Quyết Ca Hỗ Trợ</h3>

            <label className={styles.resolveToggle}>
              <input
                type="checkbox"
                checked={modalEnableLearn}
                onChange={(e) => setModalEnableLearn(e.target.checked)}
              />
              <span style={{ fontSize: '0.875rem', color: 'inherit' }}>Trích xuất Q&A để dạy AI</span>
            </label>

            {modalEnableLearn && (
              <div className={styles.pairList}>
                {modalQAPairs.map((pair, idx) => (
                  <div key={idx} className={styles.pairCard}>
                    <div className={`${styles.pairLabel} ${styles.pairQ}`}>❓ Câu hỏi:</div>
                    <div className={styles.pairText}>{pair.question}</div>
                    <div className={`${styles.pairLabel} ${styles.pairA}`}>💡 Câu trả lời:</div>
                    <div className={`${styles.pairText} ${styles.pairAnswer}`}>
                      {pair.answer}
                    </div>
                  </div>
                ))}
              </div>
            )}

            <textarea
              rows={2}
              value={resolveNote}
              onChange={(e) => setResolveNote(e.target.value)}
              placeholder="Ghi chú giải quyết (tùy chọn)..."
              className={styles.resolveNote}
            />

            <div className={styles.resolveActions}>
              <button
                onClick={() => setShowResolveModal(false)}
                className={styles.cancelBtn}
              >
                Hủy
              </button>
              <button
                onClick={handleResolveCase}
                disabled={resolveCaseMutation.isPending}
                className={styles.confirmBtn}
              >
                {modalEnableLearn ? 'Hoàn Tất & Dạy AI' : 'Đóng Case'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

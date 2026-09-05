'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Inbox, Trash2, CheckCircle2, UserCheck, Send, Headphones, Tag as TagIcon, X } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import {
  useCases,
  useCaseDetail,
  useVoiceCalls,
  useTakeCase,
  useResolveCase,
  useDeleteCase,
  useClearAllCases,
  useChatTags,
  useCaseTags,
  useAttachTag,
  useDetachTag,
} from '@/lib/hooks/useApi';
import { useWebSocket } from '@/lib/hooks/useWebSocket';
import { useAuthStore } from '@/lib/stores/authStore';
import { Pagination } from '@/components/admin/AdminSidebar';
import { MarkdownRenderer } from '@/components/MarkdownRenderer';
import { useUIStore } from '@/lib/stores/uiStore';
import type { ChatCase, Message, QAPair } from '@/lib/types';
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

  // Tab & Pagination state
  const [activeTab, setActiveTab] = useState<InboxTab>('all');
  const [casePage, setCasePage] = useState(1);
  const [casePageSize, setCasePageSize] = useState(10);
  const [caseFilter, setCaseFilter] = useState('');

  // Case selection
  const [selectedCase, setSelectedCase] = useState<ChatCase | null>(null);
  const [lastSenderMap, setLastSenderMap] = useState<Record<string, string>>({});

  // UI state
  const [replyText, setReplyText] = useState('');
  const [isSendingReply, setIsSendingReply] = useState(false);
  const [showResolveModal, setShowResolveModal] = useState(false);
  const [showVoiceHistoryModal, setShowVoiceHistoryModal] = useState(false);
  const [resolveNote, setResolveNote] = useState('');
  const [modalQAPairs, setModalQAPairs] = useState<QAPair[]>([{ question: '', answer: '' }]);
  const [modalEnableLearn, setModalEnableLearn] = useState(true);

  // Refs
  const chatContainerRef = useRef<HTMLDivElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Data fetching: fetch up to 100 cases to allow real-time tab counting & filtering
  const { data: casesData, isLoading: isLoadingCases } = useCases('', 1, 100);
  const { data: caseDetailData, refetch: refetchCaseDetail } = useCaseDetail(selectedCase?.session_id ?? '');
  const { data: voiceCallsData } = useVoiceCalls();

  // Tags state & hooks
  const [showTagPicker, setShowTagPicker] = useState(false);
  const { data: allTags = [] } = useChatTags();
  const { data: attachedTags = [] } = useCaseTags(selectedCase?.session_id ?? '');
  const attachTagMutation = useAttachTag();
  const detachTagMutation = useDetachTag();

  // Real-time WebSocket connection to receive guest messages & case updates instantly
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

  // Mutations
  const takeCaseMutation = useTakeCase();
  const resolveCaseMutation = useResolveCase();
  const deleteCaseMutation = useDeleteCase();
  const clearAllMutation = useClearAllCases();

  const allCases = casesData?.cases ?? [];
  const caseMessages = caseDetailData?.messages ?? [];
  const voiceCalls = voiceCallsData?.calls ?? [];

  // Populate lastSenderMap whenever allCases changes
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

  // Sync lastSenderMap when case detail messages are loaded
  useEffect(() => {
    if (selectedCase && caseMessages.length > 0) {
      const lastMsg = caseMessages[caseMessages.length - 1];
      if (lastMsg) {
        setLastSenderMap((prev) => {
          if (prev[selectedCase.session_id] === lastMsg.sender_type) return prev;
          return { ...prev, [selectedCase.session_id]: lastMsg.sender_type };
        });
      }
    }
  }, [selectedCase, caseMessages]);

  // Robust multi-tier check to determine if a case has an unreplied customer message
  const isCaseUnreplied = useCallback(
    (c: ChatCase): boolean => {
      if (c.status === 'RESOLVED') return false;
      if (c.status === 'NEEDS_HUMAN_CS') return true;

      // 1. Explicit last_sender_type from ChatCase (backend)
      if (c.last_sender_type === 'guest') return true;
      if (c.last_sender_type === 'human_cs' || c.last_sender_type === 'cs' || c.last_sender_type === 'ai') return false;

      // 2. Check local lastSenderMap state
      const mapSender = lastSenderMap[c.session_id];
      if (mapSender === 'guest') return true;
      if (mapSender === 'human_cs' || mapSender === 'cs' || mapSender === 'ai') return false;

      // 3. Inspect currently loaded caseMessages for selected case
      if (selectedCase?.session_id === c.session_id && caseMessages.length > 0) {
        const lastMsg = caseMessages[caseMessages.length - 1];
        if (lastMsg.sender_type === 'guest') return true;
        if (lastMsg.sender_type === 'human_cs' || lastMsg.sender_type === 'cs' || lastMsg.sender_type === 'ai') return false;
      }

      return false;
    },
    [lastSenderMap, selectedCase?.session_id, caseMessages]
  );

  // Real-time tab counts calculation
  const unrepliedAllCount = allCases.filter((c) => isCaseUnreplied(c)).length;
  const waitingCount = allCases.filter((c) => c.status === 'NEEDS_HUMAN_CS').length;
  const unrepliedActiveCount = allCases.filter(
    (c) => c.status === 'HUMAN_CS_ACTIVE' && isCaseUnreplied(c)
  ).length;
  const resolvedCount = allCases.filter((c) => c.status === 'RESOLVED').length;

  // Filter cases based on active tab and search keyword
  let tabFilteredCases = allCases.filter((c) => {
    if (activeTab === 'NEEDS_HUMAN_CS') return c.status === 'NEEDS_HUMAN_CS';
    if (activeTab === 'HUMAN_CS_ACTIVE') return c.status === 'HUMAN_CS_ACTIVE';
    if (activeTab === 'RESOLVED') return c.status === 'RESOLVED';
    return true; // 'all'
  });

  if (caseFilter.trim()) {
    const sLower = caseFilter.trim().toLowerCase();
    tabFilteredCases = tabFilteredCases.filter(
      (c) =>
        c.customer_name.toLowerCase().includes(sLower) ||
        (c.customer_phone && c.customer_phone.toLowerCase().includes(sLower)) ||
        c.session_id.toLowerCase().includes(sLower) ||
        (c.last_message && c.last_message.toLowerCase().includes(sLower))
    );
  }

  // Sorting logic: unreplied customer conversations ALWAYS pinned at the top!
  const sortedCases = [...tabFilteredCases].sort((a, b) => {
    const unrepliedA = isCaseUnreplied(a);
    const unrepliedB = isCaseUnreplied(b);

    if (unrepliedA !== unrepliedB) {
      return unrepliedA ? -1 : 1;
    }

    return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
  });

  // Pagination calculation
  const caseTotal = sortedCases.length;
  const pagedCases = sortedCases.slice((casePage - 1) * casePageSize, casePage * casePageSize);

  // Scroll to bottom when messages change
  useEffect(() => {
    if (chatContainerRef.current) {
      chatContainerRef.current.scrollTop = chatContainerRef.current.scrollHeight;
    }
  }, [caseMessages]);

  // Select case handler
  const handleSelectCase = useCallback((c: ChatCase) => {
    setSelectedCase(c);
    setReplyText('');
  }, []);

  // Take case handler
  const handleTakeCase = async () => {
    if (!selectedCase) return;
    try {
      await takeCaseMutation.mutateAsync(selectedCase.session_id);
      setSelectedCase({ ...selectedCase, status: 'HUMAN_CS_ACTIVE' });
      addToast({ title: 'Đã tiếp nhận case', variant: 'success' });
    } catch (err: any) {
      addToast({ title: 'Lỗi tiếp nhận case', message: err.message, variant: 'error' });
    }
  };

  // Send reply handler
  const handleSendReply = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!replyText.trim() || !selectedCase || isSendingReply) return;
    const content = replyText.trim();
    setReplyText('');
    setIsSendingReply(true);
    try {
      const { api } = await import('@/lib/api');
      await api.sendCSMessage(selectedCase.session_id, content);
      setSelectedCase((prev) => (prev ? { ...prev, last_sender_type: 'human_cs', last_message: content } : null));
      refetchCaseDetail();
    } catch (err: any) {
      addToast({ title: err.message || 'Lỗi gửi tin nhắn', variant: 'error' });
    } finally {
      setIsSendingReply(false);
    }
  };

  // Resolve case handlers
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
    if (!selectedCase) return;
    const extracted = extractAllQAPairs(caseMessages);
    setModalQAPairs(extracted.length > 0 ? extracted : [{ question: '', answer: '' }]);
    setModalEnableLearn(true);
    setResolveNote('');
    setShowResolveModal(true);
  };

  const handleResolveCase = async () => {
    if (!selectedCase) return;
    const validPairs = modalEnableLearn
      ? modalQAPairs.filter((p) => p.question.trim() && p.answer.trim())
      : [];
    try {
      await resolveCaseMutation.mutateAsync({
        sessionId: selectedCase.session_id,
        resolutionNote: resolveNote,
        extractPairs: validPairs,
      });
      setShowResolveModal(false);
      setSelectedCase(null);
      addToast({ title: 'Đã đóng case thành công!', variant: 'success' });
    } catch (err: any) {
      addToast({ title: err.message || 'Lỗi đóng case', variant: 'error' });
    }
  };

  // Delete case handler
  const handleDeleteCase = (sessionID: string) => {
    openConfirm({
      title: 'Xóa case?',
      message: 'Xóa ca hỗ trợ này khỏi Live CS Inbox?',
      confirmText: 'Xóa',
      variant: 'danger',
      onConfirm: async () => {
        try {
          await deleteCaseMutation.mutateAsync(sessionID);
          if (selectedCase?.session_id === sessionID) setSelectedCase(null);
          addToast({ title: 'Đã xóa case', variant: 'success' });
        } catch (err: any) {
          addToast({ title: err.message || 'Lỗi xóa case', variant: 'error' });
        }
      },
    });
  };

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
          setSelectedCase(null);
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
            onChange={(e) => {
              setCaseFilter(e.target.value);
              setCasePage(1);
            }}
            className={styles.searchInput}
          />
          <button onClick={handleClearAllCases} className={styles.clearAllBtn}>
            🗑️ Xóa test
          </button>
        </div>
      </div>

      {/* Main content - split view */}
      <div className={styles.split}>
        {/* Case list - left panel */}
        <div className={styles.listPanel}>
          {/* Tab selector bar */}
          <div className={styles.filterTabs}>
            <button
              type="button"
              className={`${styles.tabBtn} ${activeTab === 'all' ? styles.tabBtnActive : ''}`}
              onClick={() => {
                setActiveTab('all');
                setCasePage(1);
              }}
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
              onClick={() => {
                setActiveTab('NEEDS_HUMAN_CS');
                setCasePage(1);
              }}
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
              onClick={() => {
                setActiveTab('HUMAN_CS_ACTIVE');
                setCasePage(1);
              }}
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
              onClick={() => {
                setActiveTab('RESOLVED');
                setCasePage(1);
              }}
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
              <div className={styles.empty}>Đang tải...</div>
            ) : pagedCases.length === 0 ? (
              <div className={styles.empty}>Không có case nào.</div>
            ) : (
              pagedCases.map((c) => {
                const isUnreplied = isCaseUnreplied(c);
                const isActive = selectedCase?.session_id === c.session_id;
                const statusClass = STATUS_CLASS[c.status] || '';
                return (
                  <div
                    key={c.id || c.session_id}
                    onClick={() => handleSelectCase(c)}
                    className={`${styles.caseItem} ${isActive ? styles.caseItemActive : ''} ${
                      isUnreplied ? styles.caseItemUnreplied : ''
                    }`}
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

        {/* Chat area - right panel */}
        <div className={styles.chatPanel}>
          {!selectedCase ? (
            <div className={styles.emptyChat}>
              <div className={styles.emptyChatIcon}>💬</div>
              <h3 className={styles.emptyChatTitle}>
                Chọn một case bên trái để xem hội thoại
              </h3>
              <p className={styles.emptyChatDesc}>
                Các câu hỏi khách hỏi mà AI chưa có dữ liệu sẽ tự động xuất hiện ở đây.
              </p>
            </div>
          ) : (
            <>
              {/* Detail Header */}
              <div className={styles.detailHeader}>
                <div className={styles.detailTitle}>
                  <div>
                    <span className={styles.detailName}>{selectedCase.customer_name}</span>
                    {selectedCase.customer_phone && (
                      <span className={styles.phonePill}>
                        📱 {selectedCase.customer_phone}
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
                            onClick={async () => {
                              try {
                                await detachTagMutation.mutateAsync({
                                  sessionId: selectedCase.session_id,
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
                <div className={styles.detailSession}>
                  Mã phiên: <code>{selectedCase.session_id}</code>
                </div>

                <div className={styles.actionBtnGroup}>
                  {/* Tag Button & Popover */}
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
                                          sessionId: selectedCase.session_id,
                                          tagId: tag.id,
                                        });
                                        addToast({ title: `Đã gỡ tag [${tag.name}]`, variant: 'success' });
                                      } else {
                                        await attachTagMutation.mutateAsync({
                                          sessionId: selectedCase.session_id,
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

                  {selectedCase.status !== 'HUMAN_CS_ACTIVE' && (
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
                  <button
                    onClick={() => handleDeleteCase(selectedCase.session_id)}
                    className={styles.deleteBtn}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>

              {/* Messages */}
              <div ref={chatContainerRef} className={styles.messages}>
                {caseMessages.map((m, idx) => {
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
                          ? `👤 ${selectedCase.customer_name}`
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
                })}
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
            </>
          )}
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

      {/* Resolve Case Modal */}
      {showResolveModal && selectedCase && (
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

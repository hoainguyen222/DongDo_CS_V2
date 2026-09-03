'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Inbox, Trash2, CheckCircle2, UserCheck, Send, Headphones } from 'lucide-react';
import { useCases, useCaseDetail, useVoiceCalls, useTakeCase, useResolveCase, useDeleteCase, useClearAllCases } from '@/lib/hooks/useApi';
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

export default function InboxPage() {
  const { addToast, openConfirm } = useUIStore();

  // Pagination state
  const [casePage, setCasePage] = useState(1);
  const [casePageSize, setCasePageSize] = useState(10);
  const [caseFilter, setCaseFilter] = useState('');

  // Case selection
  const [selectedCase, setSelectedCase] = useState<ChatCase | null>(null);

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

  // Data fetching
  const { data: casesData, isLoading: isLoadingCases } = useCases(caseFilter, casePage, casePageSize);
  const { data: caseDetailData, refetch: refetchCaseDetail } = useCaseDetail(selectedCase?.session_id ?? '');
  const { data: voiceCallsData } = useVoiceCalls();

  // Mutations
  const takeCaseMutation = useTakeCase();
  const resolveCaseMutation = useResolveCase();
  const deleteCaseMutation = useDeleteCase();
  const clearAllMutation = useClearAllCases();

  const cases = casesData?.cases ?? [];
  const caseMessages = caseDetailData?.messages ?? [];
  const caseTotal = casesData?.total ?? 0;
  const voiceCalls = voiceCallsData?.calls ?? [];

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
          <div className={styles.listScroll}>
            {isLoadingCases ? (
              <div className={styles.empty}>Đang tải...</div>
            ) : cases.length === 0 ? (
              <div className={styles.empty}>Không có case nào.</div>
            ) : (
              cases.map((c) => {
                const isActive = selectedCase?.session_id === c.session_id;
                const statusClass = STATUS_CLASS[c.status] || '';
                return (
                  <div
                    key={c.id || c.session_id}
                    onClick={() => handleSelectCase(c)}
                    className={`${styles.caseItem} ${isActive ? styles.caseItemActive : ''}`}
                  >
                    <div className={styles.caseRow}>
                      <span className={styles.caseName}>{c.customer_name}</span>
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
                  <span className={styles.detailName}>{selectedCase.customer_name}</span>
                  {selectedCase.customer_phone && (
                    <span className={styles.phonePill}>
                      📱 {selectedCase.customer_phone}
                    </span>
                  )}
                </div>
                <div className={styles.detailSession}>
                  Mã phiên: <code>{selectedCase.session_id}</code>
                </div>

                <div className={styles.actionBtnGroup}>
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

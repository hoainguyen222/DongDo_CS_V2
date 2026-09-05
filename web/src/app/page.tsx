'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import Image from 'next/image';
import { Send, Sparkles, Phone, PhoneOff, Lock, MessageSquarePlus, ShieldCheck, LogOut, Calendar } from 'lucide-react';
import { WSClient } from '@/lib/ws';
import { GuestRegisterForm } from '@/components/forms/GuestRegisterForm';
import { useAuthStore } from '@/lib/stores/authStore';
import { useUIStore } from '@/lib/stores/uiStore';
import { MarkdownRenderer } from '@/components/MarkdownRenderer';
import { useGuestStore } from '@/lib/stores/guestStore';
import { useRouter } from 'next/navigation';
import { requestMicrophonePermission } from '@/lib/audioStream';
import styles from './page.module.scss';

interface Message {
  id: number;
  session_id: string;
  sender_type: string;
  sender_id: string;
  content: string;
  client_msg_id?: string;
  created_at: string;
}

/**
 * High-level status we display in the call modal. Backend pushes
 * RINGING / ACTIVE / ENDED / MISSED via WS `call_status_update` and
 * via the polling endpoint /api/voice/status/:callId.
 */
type CallDisplayStatus = 'idle' | 'connecting' | 'ringing' | 'connected' | 'ended' | 'error';

function generateClientMsgId(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

/**
 * Validate a Vietnamese phone number (10–11 digits, starting with 0).
 * Accepts loose formatting (spaces/dashes stripped before testing).
 */
function isValidVNPhone(input: string): boolean {
  if (!input) return false;
  const cleaned = input.replace(/[\s\-().+]/g, '');
  return /^0\d{9,10}$/.test(cleaned);
}

function normalizeVNPhone(input: string): string {
  return input.replace(/[\s\-().+]/g, '');
}

// ============================================================
// Customer Chat Page
// ============================================================
export default function CustomerChatPage() {
  const router = useRouter();
  const { addToast } = useUIStore();
  const { session: guest, isRegistered, setSession, clearSession } = useGuestStore();
  const authLogout = useAuthStore((s) => s.logout);
  const isAuthed = useAuthStore((s) => s.isAuthenticated);

  const handleLogout = useCallback(async () => {
    try {
      await authLogout();
    } catch (_) {}
    try {
      clearSession();
    } catch (_) {}
    addToast({ title: 'Đã đăng xuất', message: 'Phiên đăng nhập đã kết thúc', variant: 'info' });
    window.location.href = '/login';
  }, [authLogout, clearSession, addToast]);

  const [registerError, setRegisterError] = useState('');

  const [messages, setMessages] = useState<Message[]>([]);
  const [inputText, setInputText] = useState('');
  const [isAiTyping, setIsAiTyping] = useState(false);
  const [isSending, setIsSending] = useState(false);

  // Click-to-call state (display-only — backend Asterisk handles audio)
  const [callStatus, setCallStatus] = useState<CallDisplayStatus>('idle');
  const [callStatusText, setCallStatusText] = useState('');
  const [callDuration, setCallDuration] = useState(0);
  const [activeCallId, setActiveCallId] = useState<number | null>(null);
  const [showPhoneModal, setShowPhoneModal] = useState(false);

  const wsRef = useRef<WSClient | null>(null);
  const chatContainerRef = useRef<HTMLDivElement | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const callTimerRef = useRef<any>(null);
  const statusPollTimerRef = useRef<any>(null);

  useEffect(() => {
    if (!guest) return;

    const ws = new WSClient(guest.session_id, guest.display_name || guest.guest_id, 'guest');
    wsRef.current = ws;
    ws.connect();

    import('@/lib/api')
      .then(({ api }) => api.getHistory(guest.session_id))
      .then((data) => {
        if (data?.messages) setMessages(data.messages);
      })
      .catch(() => {});

    ws.on('message', (event: any) => {
      setIsAiTyping(false);
      const incomingMsg: Message | undefined =
        event.payload?.message ||
        (event.payload?.id || event.payload?.content ? event.payload : undefined);

      if (incomingMsg) {
        setMessages((prev) => {
          if (prev.some((m) => m.id === incomingMsg.id)) return prev;

          if (incomingMsg.client_msg_id) {
            const idx = prev.findIndex((m) => m.client_msg_id === incomingMsg.client_msg_id);
            if (idx !== -1) {
              const updated = [...prev];
              updated[idx] = incomingMsg;
              return updated;
            }
          }

          if (incomingMsg.sender_type === 'guest') {
            const idx = prev.findIndex(
              (m) =>
                m.sender_type === 'guest' &&
                m.content.trim() === incomingMsg.content.trim() &&
                (!m.id || m.id > 1700000000000)
            );
            if (idx !== -1) {
              const updated = [...prev];
              updated[idx] = incomingMsg;
              return updated;
            }
          }

          return [...prev, incomingMsg];
        });
      }
    });

    ws.on('typing', (event: any) => {
      setIsAiTyping(event.payload?.typing === true);
    });

    // Backend pushes call status transitions here. We only consume
    // these for the UI display — actual audio is handled by Asterisk.
    ws.on('call_status_update', (event: any) => {
      const status: string = (event.payload?.status || '').toUpperCase();
      if (status === 'RINGING') {
        setCallStatus('ringing');
        setCallStatusText('Đang đổ chuông...');
      } else if (status === 'ACTIVE') {
        setCallStatus('connected');
        setCallStatusText('Đã kết nối — vui lòng nghe máy');
        startCallTimer();
      } else if (status === 'ENDED' || status === 'COMPLETED') {
        setCallStatus('ended');
        setCallStatusText('Cuộc gọi đã kết thúc');
        clearInterval(callTimerRef.current);
        stopStatusPolling();
      } else if (status === 'MISSED' || status === 'REJECTED') {
        setCallStatus('ended');
        setCallStatusText(status === 'MISSED' ? 'Cuộc gọi nhỡ' : 'Cuộc gọi bị từ chối');
        clearInterval(callTimerRef.current);
        stopStatusPolling();
      }
    });

    ws.on('call_end', () => {
      setCallStatus('ended');
      setCallStatusText('Cuộc gọi đã kết thúc');
      clearInterval(callTimerRef.current);
      stopStatusPolling();
    });

    return () => {
      ws.disconnect();
      clearInterval(callTimerRef.current);
      stopStatusPolling();
    };
  }, [guest?.session_id]);

  useEffect(() => {
    chatContainerRef.current?.scrollTo({ top: chatContainerRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages, isAiTyping]);

  const handleRegister = async (data: { displayName: string; phone?: string }) => {
    setRegisterError('');
    try {
      const { api } = await import('@/lib/api');
      const session = await api.registerGuest(data.displayName, data.phone);
      setSession(session);
    } catch (err: any) {
      setRegisterError(err.message || 'Lỗi khởi tạo phiên chat');
    }
  };

  const handleSendMessage = useCallback(
    async (textToSend?: string) => {
      const content = (textToSend || inputText).trim();
      if (!content || !guest || isSending) return;

      setInputText('');
      setIsSending(true);

      const clientMsgId = generateClientMsgId();
      const optimisticMsg: Message = {
        id: Date.now(),
        session_id: guest.session_id,
        sender_type: 'guest',
        sender_id: guest.display_name || guest.guest_id,
        content,
        client_msg_id: clientMsgId,
        created_at: new Date().toISOString(),
      };

      setMessages((prev) => [...prev, optimisticMsg]);
      setIsAiTyping(true);

      try {
        const { api } = await import('@/lib/api');
        await api.sendMessage(guest.session_id, guest.display_name, content, clientMsgId);
      } catch (err: any) {
        addToast({ title: 'Lỗi gửi tin nhắn', message: err.message, variant: 'error' });
      } finally {
        setIsSending(false);
      }
    },
    [inputText, guest, isSending, addToast]
  );

  const handleNewChat = () => {
    clearSession();
    setMessages([]);
    setInputText('');
  };

  const startCallTimer = () => {
    clearInterval(callTimerRef.current);
    setCallDuration(0);
    callTimerRef.current = setInterval(() => setCallDuration((p) => p + 1), 1000);
  };

  /**
   * Stop the call timer + status polling and reset display state.
   * Does NOT touch the backend — backend Asterisk owns call lifecycle.
   */
  const resetCallUI = useCallback(() => {
    clearInterval(callTimerRef.current);
    stopStatusPolling();
    setCallDuration(0);
  }, []);

  /**
   * Begin polling /api/voice/status/:callId as a SAFETY NET only.
   *
   * The backend now publishes `call_status_update` over WS on every
   * transition (CreateCall / Accept / Hangup / ARI events), and the WS
   * event also drives a re-fetch via queryClient.invalidateQueries in
   * many places. Polling was previously the primary mechanism and
   * caused a runaway 2s setInterval that hammered the DB indefinitely.
   *
   * We now:
   *   1. Trust WS events as the source of truth (see `call_status_update`
   *      listener in the WS bootstrap effect).
   *   2. Only start this slow (10s) poll as a fallback when no WS event
   *      has arrived for a while — the function is kept for
   *      backward compat / extreme proxy cases.
   *   3. Stop polling as soon as the call reaches a terminal status
   *      (ENDED, MISSED, REJECTED).
   */
  const startStatusPolling = useCallback((callId: number) => {
    stopStatusPolling();
    let consecutiveFailures = 0;
    statusPollTimerRef.current = setInterval(async () => {
      try {
        const { api } = await import('@/lib/api');
        const data = await api.getCallStatus(callId);
        consecutiveFailures = 0;
        const s = (data?.status || '').toUpperCase();
        if (s === 'ACTIVE') {
          setCallStatus('connected');
          setCallStatusText('Đã kết nối — vui lòng nghe máy');
        } else if (s === 'RINGING') {
          setCallStatus('ringing');
          setCallStatusText('Đang đổ chuông...');
        } else if (s === 'ENDED' || s === 'COMPLETED') {
          setCallStatus('ended');
          setCallStatusText('Cuộc gọi đã kết thúc');
          resetCallUI();
        } else if (s === 'MISSED' || s === 'REJECTED') {
          setCallStatus('ended');
          setCallStatusText(s === 'MISSED' ? 'Cuộc gọi nhỡ' : 'Cuộc gọi bị từ chối');
          resetCallUI();
        }
      } catch (_err) {
        // ignore polling failures; WS will still drive transitions.
        consecutiveFailures++;
        // After 3 consecutive failures we bail — repeated network
        // errors here signal a deeper issue and the runaway polling
        // should not continue forever.
        if (consecutiveFailures >= 3) {
          stopStatusPolling();
        }
      }
    }, 10_000);
  }, [resetCallUI]);

  function stopStatusPolling() {
    if (statusPollTimerRef.current) {
      clearInterval(statusPollTimerRef.current);
      statusPollTimerRef.current = null;
    }
  }

  const formatCallTime = (s: number) => {
    const m = Math.floor(s / 60);
    const sec = s % 60;
    return `${m.toString().padStart(2, '0')}:${sec.toString().padStart(2, '0')}`;
  };

  /**
   * Open the click-to-call modal. We "test the mic" first so the
   * browser prompts for permission up-front; this is purely UX, the
   * stream is released immediately after the test.
   */
  const handleRequestCall = async () => {
    if (!guest) return;
    setShowPhoneModal(true);
    // Fire-and-forget: pre-warm mic permission UX. Don't block modal.
    void requestMicrophonePermission();
  };

  /**
   * Submit phone + schedule preference. Backend's POST /api/voice/initiate
   * creates the call record and Asterisk dials guest + agent, then bridges.
   */
  const handleSubmitCallRequest = useCallback(
    async (phoneNumber: string, scheduledAt?: string) => {
      if (!guest) return;
      const normalizedPhone = normalizeVNPhone(phoneNumber);
      if (!isValidVNPhone(normalizedPhone)) {
        addToast({
          title: 'Số điện thoại không hợp lệ',
          message: 'Vui lòng nhập số điện thoại Việt Nam 10–11 số, bắt đầu bằng 0.',
          variant: 'warning',
        });
        return;
      }

      setShowPhoneModal(false);
      setCallStatus('connecting');
      setCallStatusText(scheduledAt ? 'Đã đặt lịch — chờ đến giờ gọi' : 'Đang kết nối...');
      setCallDuration(0);

      try {
        const { api } = await import('@/lib/api');
        const createdCall = await api.initiateVoiceCall({
          sessionId: guest.session_id,
          callerType: 'guest',
          callerId: guest.display_name || guest.guest_id || 'guest',
          calleeType: 'cskh',
          calleeId: '',
          phoneNumber: normalizedPhone,
          scheduledAt,
        });
        if (createdCall?.id) {
          setActiveCallId(createdCall.id);
          startStatusPolling(createdCall.id);
        }
        if (scheduledAt) {
          // Scheduled callback — Asterisk will dial at the requested time.
          addToast({
            title: 'Đã đặt lịch gọi lại',
            message: 'Chuyên viên sẽ gọi cho bạn theo lịch đã chọn.',
            variant: 'success',
          });
        } else {
          setCallStatus('ringing');
          setCallStatusText('Đang đổ chuông...');
        }
      } catch (err: any) {
        console.error('Failed to initiate call via API:', err);
        setCallStatus('error');
        setCallStatusText('Không thể kết nối CSKH');
        addToast({
          title: 'Lỗi cuộc gọi',
          message: err?.message || 'Không thể bắt đầu cuộc gọi. Vui lòng thử lại.',
          variant: 'error',
        });
      }
    },
    [guest, addToast, startStatusPolling]
  );

  /**
   * Guest hangs up — tell backend to terminate the bridge and clear UI.
   */
  const handleEndCall = useCallback(async () => {
    const finalDuration = callDuration;
    const callId = activeCallId;
    resetCallUI();
    setCallStatus('ended');
    setCallStatusText('Cuộc gọi đã kết thúc');
    try {
      const { api } = await import('@/lib/api');
      await api.endCall(guest?.session_id || '', callId ?? undefined, finalDuration);
    } catch (_) {
      /* noop */
    }
    setActiveCallId(null);
  }, [callDuration, activeCallId, guest?.session_id, resetCallUI]);

  // ============================================================
  // PRE-CHAT SCREEN — redirect sang /login nếu chưa có guest session
  // ============================================================
  const [hasHydrated, setHasHydrated] = useState(false);

  useEffect(() => {
    // Đánh dấu hydration xong (client-side only)
    setHasHydrated(true);
  }, []);

  useEffect(() => {
    // Chỉ redirect SAU khi hydration xong và confirmed không có session
    if (hasHydrated && !guest) {
      router.replace('/login');
    }
  }, [hasHydrated, guest, router]);

  // Hiển thị loading trong khi hydration hoặc redirect
  if (!hasHydrated || !guest) {
    return (
      <div className={styles.preChat}>
        <div className={styles.blobNavy} />
        <div className={styles.blobRed} />
        <div className={styles.preChatCard}>
          <div className={styles.preChatHeader}>
            <div className={styles.brandBlock}>
              <div className={styles.brandIconBox}>
                <Image
                  src="/logo/Logo Dọc_Trắng.svg"
                  alt="Đông Đô Partners"
                  width={56}
                  height={56}
                  className={styles.brandLogo}
                  onError={(e) => {
                    (e.target as HTMLImageElement).style.display = 'none';
                  }}
                />
              </div>
              <h1 className={styles.brandTitle}>Đông Đô Partners</h1>
              <p className={styles.brandSubtitle}>
                Cổng Tư Vấn Hàng Hóa Phái Sinh &amp; DDP Invest
              </p>
            </div>
          </div>
          <p style={{ color: '#cbd5e1', textAlign: 'center', marginTop: '20px' }}>
            Đang chuyển hướng...
          </p>
          <div className={styles.footerNote}>
            <ShieldCheck style={{ width: 14, height: 14, color: '#10b981' }} />
            <span>Hệ thống bảo mật dữ liệu khách hàng 100%</span>
          </div>
        </div>
      </div>
    );
  }

  // ============================================================
  // MAIN CHAT INTERFACE
  // ============================================================
  const isCallInProgress =
    callStatus === 'connecting' || callStatus === 'ringing' || callStatus === 'connected';

  return (
    <div className={styles.shell}>
      {/* Phone request modal (click-to-call entry) */}
      {showPhoneModal && (
        <PhoneRequestModal
          defaultPhone={guest.phone || ''}
          onCancel={() => setShowPhoneModal(false)}
          onSubmit={handleSubmitCallRequest}
        />
      )}

      {/* Call status modal — display-only; backend Asterisk owns audio */}
      {isCallInProgress && (
        <div className={styles.callModalBackdrop}>
          <div className={styles.callModal}>
            <div className={styles.callModalHead}>
              <div className={styles.callModalIcon}>
                <Phone style={{ width: 40, height: 40 }} />
              </div>
              <div>
                <h3 className={styles.callModalTitle}>Chăm Sóc Khách Hàng Đông Đô</h3>
                <p className={styles.callModalStatus}>{callStatusText}</p>
                <div className={styles.callModalDuration}>{formatCallTime(callDuration)}</div>
              </div>
            </div>
            <div className={styles.callModalControls}>
              <button onClick={handleEndCall} className={styles.callEndBtn} aria-label="Kết thúc">
                <PhoneOff style={{ width: 24, height: 24 }} />
              </button>
            </div>
          </div>
        </div>
      )}

      <header className={styles.header}>
        <div className={styles.headerBrand}>
          <Image
            src="/logo/Logo Ngang_Trắng.svg"
            alt="Đông Đô Partners"
            width={140}
            height={36}
            className={styles.headerLogo}
            onError={(e) => {
              (e.target as HTMLImageElement).style.display = 'none';
            }}
          />
          <div className={styles.headerTitles}>
            <h1 className={styles.brand}>Đông Đô Partners</h1>
            <div className={styles.presence}>
              <span className={styles.presenceDot} />
              <span className={styles.presenceText}>Chuyên viên CSKH đang trực tuyến</span>
            </div>
          </div>
        </div>

        <div className={styles.headerActions}>
          <button
            onClick={handleRequestCall}
            disabled={isCallInProgress}
            className={styles.btnCallHeader}
            title="Yêu cầu chuyên viên gọi lại qua điện thoại"
          >
            <Phone style={{ width: 14, height: 14 }} />
            <span>Yêu cầu gọi lại</span>
          </button>
          <a href="/admin/login" className={styles.btnPortalLink}>
            <Lock style={{ width: 14, height: 14 }} />
            <span style={{ display: 'inline' }}>Cổng CSKH</span>
          </a>
          <button
            onClick={handleNewChat}
            className={styles.btnNewChat}
            title="Bắt đầu cuộc trò chuyện mới (reset hội thoại hiện tại)"
            aria-label="Bắt đầu cuộc trò chuyện mới"
          >
            <MessageSquarePlus style={{ width: 14, height: 14 }} />
            <span>Mới</span>
          </button>
          {isAuthed && (
            <button
              onClick={handleLogout}
              className={styles.btnLogout}
              title="Đăng xuất phiên hiện tại"
              aria-label="Đăng xuất"
            >
              <LogOut style={{ width: 14, height: 14 }} />
              <span>Đăng xuất</span>
            </button>
          )}
        </div>
      </header>

      <main ref={chatContainerRef} className={styles.main}>
        {messages.length === 0 ? (
          <div className={styles.emptyHero}>
            <div className={styles.emptyHeroIcon}>
              <Sparkles style={{ width: 32, height: 32 }} />
            </div>
            <div>
              <h2 className={styles.emptyHeroTitle}>Xin chào {guest.display_name}! 👋</h2>
              <p className={styles.emptyHeroBody}>
                Em là chuyên viên Chăm sóc Khách hàng của <strong>Đông Đô Partners</strong>. Em có thể giải đáp cho anh/chị về:
              </p>
            </div>
            <div className={styles.suggestionGrid}>
              {[
                { label: 'Hàng hóa phái sinh là gì?', icon: '💹' },
                { label: 'Hướng dẫn nạp tiền vào tài khoản', icon: '💰' },
                { label: 'Giới thiệu nền tảng DDP Invest', icon: '📱' },
                { label: 'Cách quản trị rủi ro khi giao dịch?', icon: '🛡️' },
              ].map((chip, idx) => (
                <button
                  key={idx}
                  onClick={() => handleSendMessage(chip.label)}
                  className={styles.suggestionChip}
                >
                  <span className={styles.suggestionEmoji}>{chip.icon}</span>
                  <span>{chip.label}</span>
                </button>
              ))}
            </div>
          </div>
        ) : (
          messages.map((m, idx) => {
            const isGuest = m.sender_type === 'guest';
            const isCS = m.sender_type === 'cs' || m.sender_type === 'human_cs';
            return (
              <div
                key={m.id || idx}
                className={`${styles.messageRow} ${isGuest ? styles.messageRowGuest : styles.messageRowOther}`}
              >
                <div className={styles.messageMeta}>
                  {isGuest
                    ? `👤 ${guest.display_name}`
                    : isCS
                    ? `👨‍💼 Chuyên viên ${m.sender_id || 'CSKH'}`
                    : '🤖 Trợ lý AI Đông Đô'}
                  {' • '}
                  {new Date(m.created_at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}
                </div>
                <div
                  className={`${styles.messageBubble} ${
                    isGuest ? styles.bubbleGuest : isCS ? styles.bubbleCS : styles.bubbleAI
                  }`}
                >
                  {isGuest ? <p>{m.content}</p> : <MarkdownRenderer content={m.content} />}
                </div>
              </div>
            );
          })
        )}

        {isAiTyping && (
          <div className={`${styles.messageRow} ${styles.messageRowOther}`}>
            <div className={styles.messageMeta}>🤖 Trợ lý AI Đông Đô đang gõ...</div>
            <div className={`${styles.messageBubble} ${styles.bubbleAI} ${styles.typingIndicator}`}>
              <span className={styles.typingDot} />
              <span className={styles.typingDot} />
              <span className={styles.typingDot} />
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </main>

      <footer className={styles.footer}>
        <form
          onSubmit={(e) => { e.preventDefault(); handleSendMessage(); }}
          className={styles.inputRow}
        >
          <input
            type="text"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            placeholder="Nhập câu hỏi của bạn về Hàng hóa phái sinh, DDP Invest..."
            className={styles.chatInput}
          />
          <button type="submit" disabled={!inputText.trim() || isSending} className={styles.sendBtn}>
            <Send style={{ width: 16, height: 16 }} />
          </button>
        </form>
        <div className={styles.footerHint}>
          Đông Đô Partners AI hỗ trợ trực tuyến 24/7 • Dữ liệu mã hóa an toàn
        </div>
      </footer>
    </div>
  );
}

// ============================================================
// PhoneRequestModal — collects guest phone + schedule preference
// ============================================================
function PhoneRequestModal({
  defaultPhone,
  onCancel,
  onSubmit,
}: {
  defaultPhone?: string;
  onCancel: () => void;
  onSubmit: (phone: string, scheduledAt?: string) => Promise<void> | void;
}) {
  const [phone, setPhone] = useState(defaultPhone || '');
  const [scheduleMode, setScheduleMode] = useState<'now' | 'later'>('now');
  const [scheduleDate, setScheduleDate] = useState<string>('');
  const [submitting, setSubmitting] = useState(false);

  const normalized = normalizeVNPhone(phone);
  const phoneValid = isValidVNPhone(normalized);
  const scheduleValid =
    scheduleMode === 'now' || (scheduleMode === 'later' && !!scheduleDate);

  const canSubmit = phoneValid && scheduleValid && !submitting;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setSubmitting(true);
    try {
      const scheduledAt =
        scheduleMode === 'later' && scheduleDate
          ? new Date(scheduleDate).toISOString()
          : undefined;
      await onSubmit(normalized, scheduledAt);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className={styles.callModalBackdrop}>
      <div className={styles.callModal} style={{ textAlign: 'left' }}>
        <div className={styles.callModalHead} style={{ alignItems: 'flex-start' }}>
          <div className={styles.callModalIcon} style={{ width: 64, height: 64 }}>
            <Phone style={{ width: 28, height: 28 }} />
          </div>
          <div>
            <h3 className={styles.callModalTitle}>Yêu cầu gọi lại</h3>
            <p className={styles.callModalStatus}>
              Nhập số điện thoại để chuyên viên Đông Đô liên hệ tư vấn qua điện thoại.
            </p>
          </div>
        </div>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label
              htmlFor="phone-input"
              style={{ fontSize: 12, color: '#cbd5e1', fontWeight: 600 }}
            >
              Số điện thoại
            </label>
            <input
              id="phone-input"
              type="tel"
              inputMode="tel"
              autoComplete="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder="VD: 0912 345 678"
              style={{
                padding: '12px 14px',
                borderRadius: 12,
                background: 'rgba(15, 24, 48, 0.8)',
                border: `1px solid ${phone && !phoneValid ? '#ef4444' : 'rgba(255,255,255,0.1)'}`,
                color: '#fff',
                fontSize: 14,
                outline: 'none',
              }}
            />
            {phone && !phoneValid && (
              <span style={{ fontSize: 11, color: '#fca5a5' }}>
                Số điện thoại phải có 10–11 chữ số và bắt đầu bằng 0.
              </span>
            )}
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <label style={{ fontSize: 12, color: '#cbd5e1', fontWeight: 600 }}>
              Thời gian gọi
            </label>
            <div style={{ display: 'flex', gap: 8 }}>
              <button
                type="button"
                onClick={() => setScheduleMode('now')}
                style={{
                  flex: 1,
                  padding: '10px 12px',
                  borderRadius: 10,
                  border:
                    scheduleMode === 'now'
                      ? '1px solid #10b981'
                      : '1px solid rgba(255,255,255,0.1)',
                  background:
                    scheduleMode === 'now' ? 'rgba(16,185,129,0.15)' : 'rgba(15,24,48,0.8)',
                  color: scheduleMode === 'now' ? '#6ee7b7' : '#cbd5e1',
                  fontWeight: 600,
                  cursor: 'pointer',
                  fontSize: 13,
                }}
              >
                <Phone style={{ width: 14, height: 14, display: 'inline', marginRight: 6 }} />
                Gọi ngay
              </button>
              <button
                type="button"
                onClick={() => setScheduleMode('later')}
                style={{
                  flex: 1,
                  padding: '10px 12px',
                  borderRadius: 10,
                  border:
                    scheduleMode === 'later'
                      ? '1px solid #f59e0b'
                      : '1px solid rgba(255,255,255,0.1)',
                  background:
                    scheduleMode === 'later' ? 'rgba(245,158,11,0.15)' : 'rgba(15,24,48,0.8)',
                  color: scheduleMode === 'later' ? '#fcd34d' : '#cbd5e1',
                  fontWeight: 600,
                  cursor: 'pointer',
                  fontSize: 13,
                }}
              >
                <Calendar style={{ width: 14, height: 14, display: 'inline', marginRight: 6 }} />
                Đặt lịch
              </button>
            </div>
            {scheduleMode === 'later' && (
              <input
                type="datetime-local"
                value={scheduleDate}
                onChange={(e) => setScheduleDate(e.target.value)}
                style={{
                  padding: '10px 12px',
                  borderRadius: 10,
                  background: 'rgba(15,24,48,0.8)',
                  border: '1px solid rgba(255,255,255,0.1)',
                  color: '#fff',
                  fontSize: 13,
                  outline: 'none',
                }}
              />
            )}
          </div>

          <div className={styles.callModalControls} style={{ paddingTop: 4 }}>
            <button
              type="button"
              onClick={onCancel}
              style={{
                padding: '10px 18px',
                borderRadius: 9999,
                background: 'rgba(30,41,59,0.8)',
                color: '#cbd5e1',
                border: '1px solid rgba(255,255,255,0.08)',
                fontWeight: 600,
                cursor: 'pointer',
              }}
            >
              Hủy
            </button>
            <button
              type="submit"
              disabled={!canSubmit}
              style={{
                padding: '10px 22px',
                borderRadius: 9999,
                background: canSubmit ? '#10b981' : 'rgba(16,185,129,0.4)',
                color: '#fff',
                border: 'none',
                fontWeight: 700,
                cursor: canSubmit ? 'pointer' : 'not-allowed',
                opacity: canSubmit ? 1 : 0.7,
              }}
            >
              {submitting ? 'Đang gửi...' : scheduleMode === 'now' ? 'Gọi ngay' : 'Đặt lịch'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import Image from 'next/image';
import { Send, Sparkles, Phone, PhoneOff, Mic, MicOff, Lock, MessageSquarePlus, ShieldCheck, LogOut } from 'lucide-react';
import { WSClient } from '@/lib/ws';
import { WebRTCManager } from '@/lib/webrtc';
import { GuestRegisterForm } from '@/components/forms/GuestRegisterForm';
import { MarkdownRenderer } from '@/components/MarkdownRenderer';
import { useUIStore } from '@/lib/stores/uiStore';
import { useGuestStore } from '@/lib/stores/guestStore';
import { useAuthStore } from '@/lib/stores/authStore';
import { useRouter } from 'next/navigation';
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
  const [caseStatus, setCaseStatus] = useState<string>('AI_ACTIVE');
  const isClosed = caseStatus === 'RESOLVED';

  const [isCallActive, setIsCallActive] = useState(false);
  const [incomingCall, setIncomingCall] = useState<{ sender_id: string; offer: any } | null>(null);
  const [callStatusText, setCallStatusText] = useState('Đang kết nối...');
  const [callDuration, setCallDuration] = useState(0);
  const [isMuted, setIsMuted] = useState(false);

  const wsRef = useRef<WSClient | null>(null);
  const rtcRef = useRef<WebRTCManager | null>(null);
  const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
  const chatContainerRef = useRef<HTMLDivElement | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const callTimerRef = useRef<any>(null);

  useEffect(() => {
    if (!guest) return;

    const ws = new WSClient(guest.session_id, guest.display_name || guest.guest_id, 'guest');
    wsRef.current = ws;
    ws.connect();

    import('@/lib/api')
      .then(({ api }) => api.getHistory(guest.session_id))
      .then((data) => {
        if (data?.messages) setMessages(data.messages);
        if (data?.status) setCaseStatus(data.status);
      })
      .catch(() => {});

    ws.on('case_update', (event: any) => {
      const payload = event?.payload || event;
      if (payload?.status) {
        setCaseStatus(payload.status);
      }
    });

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

    ws.on('call_offer', (event: any) => {
      setIncomingCall({
        sender_id: event.sender_id || 'Chuyên viên CSKH Đông Đô',
        offer: event.payload,
      });
    });

    ws.on('call_answer', (event: any) => {
      setCallStatusText('Đang đàm thoại');
      startCallTimer();
      if (rtcRef.current && event.payload) {
        rtcRef.current.handleAnswer(event.payload);
      }
    });

    ws.on('call_end', async () => {
      const finalDuration = callDurationRef.current;
      setIsCallActive(false);
      setIncomingCall(null);
      stopCallTimer();
      setCallDuration(0);
      callDurationRef.current = 0;
      const rtc = rtcRef.current;
      rtcRef.current = null;
      if (rtc) {
        try { await rtc.endCall(false, finalDuration); } catch (_) {}
      }
    });

    return () => {
      ws.disconnect();
      stopCallTimer();
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
      if (isClosed) return;
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
    [isClosed, inputText, guest, isSending, addToast]
  );

  const handleNewChat = () => {
    clearSession();
    setMessages([]);
    setInputText('');
  };

  const callDurationRef = useRef(0);

  const stopCallTimer = useCallback(() => {
    if (callTimerRef.current) {
      clearInterval(callTimerRef.current);
      callTimerRef.current = null;
    }
  }, []);

  const saveCallStateToStorage = useCallback((active: boolean, startTimeMs?: number) => {
    if (typeof window === 'undefined') return;
    const sessionKey = guest?.session_id ? `active_voice_call_${guest.session_id}` : null;
    if (!active) {
      if (sessionKey) sessionStorage.removeItem(sessionKey);
      sessionStorage.removeItem('active_voice_call_customer');
    } else {
      const data = {
        isCallActive: true,
        startTime: startTimeMs || (Date.now() - (callDurationRef.current * 1000)),
        sessionId: guest?.session_id,
      };
      if (sessionKey) sessionStorage.setItem(sessionKey, JSON.stringify(data));
      sessionStorage.setItem('active_voice_call_customer', JSON.stringify(data));
    }
  }, [guest?.session_id]);

  const startCallTimer = useCallback(() => {
    if (callTimerRef.current) {
      clearInterval(callTimerRef.current);
    }
    callTimerRef.current = setInterval(() => {
      callDurationRef.current += 1;
      setCallDuration((prev) => prev + 1);
      saveCallStateToStorage(true);
    }, 1000);
  }, [saveCallStateToStorage]);

  // Prevent accidental F5 reload or tab close during active call
  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (isCallActive) {
        e.preventDefault();
        e.returnValue = 'Cuộc gọi đang diễn ra. Bạn có chắc chắn muốn tải lại trang?';
        return e.returnValue;
      }
    };
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [isCallActive]);

  // Auto-restore active call state, popup modal, timer and WebRTC from sessionStorage on page reload (F5)
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const sessionKey = guest?.session_id ? `active_voice_call_${guest.session_id}` : null;
    const savedStr = (sessionKey && sessionStorage.getItem(sessionKey)) || sessionStorage.getItem('active_voice_call_customer');

    if (savedStr) {
      try {
        const saved = JSON.parse(savedStr);
        if (saved.isCallActive && saved.startTime) {
          const elapsed = Math.floor((Date.now() - saved.startTime) / 1000);
          if (elapsed >= 0 && elapsed < 1800) {
            setIsCallActive(true);
            setCallStatusText('Đang đàm thoại');
            callDurationRef.current = elapsed;
            setCallDuration(elapsed);
            startCallTimer();

            // Attempt WebRTC reconnect if WS is ready
            if (wsRef.current && guest?.session_id && !rtcRef.current) {
              const rtc = new WebRTCManager(
                wsRef.current,
                guest.session_id,
                (state: any) => {
                  if (state === 'connected') {
                    setCallStatusText('Đang đàm thoại');
                  } else if (state === 'ended') {
                    setIsCallActive(false);
                    setIncomingCall(null);
                    stopCallTimer();
                    setCallDuration(0);
                    callDurationRef.current = 0;
                    saveCallStateToStorage(false);
                  }
                },
                (stream) => {
                  if (remoteAudioRef.current) {
                    remoteAudioRef.current.srcObject = stream;
                    remoteAudioRef.current.play().catch(() => {});
                  }
                }
              );
              rtcRef.current = rtc;
              rtc.startCall().catch(() => {});
            }
          } else {
            if (sessionKey) sessionStorage.removeItem(sessionKey);
            sessionStorage.removeItem('active_voice_call_customer');
          }
        }
      } catch (_) {
        if (sessionKey) sessionStorage.removeItem(sessionKey);
        sessionStorage.removeItem('active_voice_call_customer');
      }
    }
  }, [guest?.session_id, startCallTimer, stopCallTimer, saveCallStateToStorage]);

  const formatCallTime = (s: number) => {
    const m = Math.floor(s / 60);
    const sec = s % 60;
    return `${m.toString().padStart(2, '0')}:${sec.toString().padStart(2, '0')}`;
  };

  const handleStartCall = async () => {
    if (!guest || !wsRef.current) return;
    setIsCallActive(true);
    setCallStatusText('Đang đổ chuông tới CSKH...');
    callDurationRef.current = 0;
    setCallDuration(0);
    stopCallTimer();
    saveCallStateToStorage(true, Date.now());

    const rtc = new WebRTCManager(
      wsRef.current,
      guest.session_id,
      (state: any) => {
        if (state === 'connected') {
          setCallStatusText('Đang đàm thoại');
          startCallTimer();
        } else if (state === 'ended') {
          setIsCallActive(false);
          setIncomingCall(null);
          stopCallTimer();
          setCallDuration(0);
          callDurationRef.current = 0;
          saveCallStateToStorage(false);
        }
      },
      (stream) => {
        if (remoteAudioRef.current) {
          remoteAudioRef.current.srcObject = stream;
          remoteAudioRef.current.play().catch(() => {});
        }
      }
    );
    rtcRef.current = rtc;
    await rtc.startCall();
  };

  const handleEndCall = async () => {
    const finalDuration = callDurationRef.current;
    setIsCallActive(false);
    setIncomingCall(null);
    stopCallTimer();
    setCallDuration(0);
    callDurationRef.current = 0;
    saveCallStateToStorage(false);

    if (rtcRef.current) {
      try { await rtcRef.current.endCall(false, finalDuration); } catch (_) {}
      rtcRef.current = null;
    }
    // Call API to end call record
    try {
      const { api } = await import('@/lib/api');
      await api.endCall(guest?.session_id || '', finalDuration);
    } catch (_) {}
  };

  const handleAnswerCall = async () => {
    if (!incomingCall) return;
    setIsCallActive(true);
    setCallStatusText('Đang đàm thoại');
    callDurationRef.current = 0;
    setCallDuration(0);
    saveCallStateToStorage(true, Date.now());
    startCallTimer();
    const rtc = new WebRTCManager(
      wsRef.current!,
      guest!.session_id,
      (state: any) => {
        if (state === 'connected') { startCallTimer(); }
        else if (state === 'ended') {
          setIsCallActive(false);
          setIncomingCall(null);
          stopCallTimer();
          setCallDuration(0);
          callDurationRef.current = 0;
          saveCallStateToStorage(false);
        }
      },
      (stream) => {
        if (remoteAudioRef.current) {
          remoteAudioRef.current.srcObject = stream;
          remoteAudioRef.current.play().catch(() => {});
        }
      }
    );
    rtcRef.current = rtc;
    if (incomingCall.offer) await rtc.handleOffer(incomingCall.offer).catch(() => {});
    setIncomingCall(null);
  };

  const handleDeclineCall = async () => {
    setIncomingCall(null);
    try {
      const { api } = await import('@/lib/api');
      await api.declineCall(guest?.session_id || '');
    } catch (_) {}
  };

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
  return (
    <div className={styles.shell}>
      <audio ref={remoteAudioRef} autoPlay className={styles.audioHidden} />

      {incomingCall && (
        <div className={styles.callBanner}>
          <div className={styles.callBannerIcon}>
            <Phone style={{ width: 24, height: 24 }} />
          </div>
          <div className={styles.callBannerText}>
            <div className={styles.label}>Cuộc gọi từ Đông Đô!</div>
            <div className={styles.caller}>{incomingCall.sender_id} đang gọi...</div>
          </div>
          <div className={styles.callBannerActions}>
            <button onClick={handleAnswerCall} className={styles.btnAccept}>
              <Phone style={{ width: 16, height: 16 }} />
              <span>Nghe máy</span>
            </button>
            <button onClick={handleDeclineCall} className={styles.btnDecline} aria-label="Từ chối">
              <PhoneOff style={{ width: 16, height: 16 }} />
            </button>
          </div>
        </div>
      )}

      {isCallActive && (
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
              <button
                onClick={() => {
                  if (rtcRef.current) setIsMuted(rtcRef.current.toggleMute());
                }}
                className={styles.callRoundBtn}
                aria-label="Bật/tắt mic"
              >
                {isMuted ? (
                  <MicOff style={{ width: 24, height: 24, color: '#f87171' }} />
                ) : (
                  <Mic style={{ width: 24, height: 24 }} />
                )}
              </button>
              <button onClick={() => handleEndCall()} className={styles.callEndBtn} aria-label="Kết thúc">
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
            onClick={handleStartCall}
            disabled={isCallActive}
            className={styles.btnCallHeader}
            title="Đàm thoại trực tiếp qua Internet miễn phí"
          >
            <Phone style={{ width: 14, height: 14 }} />
            <span>Gọi CSKH (Miễn phí)</span>
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
        {isClosed && (
          <div
            style={{
              margin: '0 auto 8px auto',
              padding: '8px 14px',
              maxWidth: '600px',
              background: 'rgba(148, 163, 184, 0.12)',
              border: '1px solid rgba(148, 163, 184, 0.25)',
              borderRadius: '8px',
              color: '#94a3b8',
              fontSize: '12px',
              textAlign: 'center',
            }}
          >
            🔒 Phiên hỗ trợ này đã được đóng. Bạn chỉ có quyền xem lại lịch sử trò chuyện.
          </div>
        )}
        <form
          onSubmit={(e) => { e.preventDefault(); if (!isClosed) handleSendMessage(); }}
          className={styles.inputRow}
        >
          <input
            type="text"
            value={inputText}
            disabled={isClosed}
            onChange={(e) => setInputText(e.target.value)}
            placeholder={
              isClosed
                ? 'Hội thoại đã đóng. Cảm ơn bạn đã liên hệ!'
                : 'Nhập câu hỏi của bạn về Hàng hóa phái sinh, DDP Invest...'
            }
            className={styles.chatInput}
            style={{
              opacity: isClosed ? 0.6 : 1,
              cursor: isClosed ? 'not-allowed' : 'text',
            }}
          />
          <button type="submit" disabled={isClosed || !inputText.trim() || isSending} className={styles.sendBtn}>
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

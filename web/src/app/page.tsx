'use client';

import React, { useState, useEffect, useRef } from 'react';
import {
  Send,
  Sparkles,
  User,
  Phone,
  PhoneOff,
  Mic,
  MicOff,
  Lock,
  ExternalLink,
  ShieldCheck,
  RotateCcw,
} from 'lucide-react';
import { api } from '@/lib/api';
import { WSClient } from '@/lib/ws';
import { WebRTCManager } from '@/lib/webrtc';
import { GuestSession, Message, WSEvent } from '@/lib/types';
import { MarkdownRenderer } from '@/components/MarkdownRenderer';

export default function CustomerChatPage() {
  // Guest state
  const [guest, setGuest] = useState<GuestSession | null>(null);
  const [inputName, setInputName] = useState('');
  const [inputPhone, setInputPhone] = useState('');
  const [isRegistering, setIsRegistering] = useState(false);

  // Chat state
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputText, setInputText] = useState('');
  const [isAiTyping, setIsAiTyping] = useState(false);
  const [isSending, setIsSending] = useState(false);

  // WebRTC Voice Call state
  const [isCallActive, setIsCallActive] = useState(false);
  const [incomingCall, setIncomingCall] = useState<{ sender_id: string; offer: any } | null>(null);
  const [callStatusText, setCallStatusText] = useState('Đang kết nối...');
  const [callDuration, setCallDuration] = useState(0);
  const [isMuted, setIsMuted] = useState(false);
  const rtcRef = useRef<WebRTCManager | null>(null);
  const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
  const callTimerRef = useRef<any>(null);

  // WebSocket & DOM refs
  const wsRef = useRef<WSClient | null>(null);
  const chatContainerRef = useRef<HTMLDivElement | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  // Load saved session on mount
  useEffect(() => {
    const saved = localStorage.getItem('dongdo_guest_session');
    if (saved) {
      try {
        const parsed: GuestSession = JSON.parse(saved);
        setGuest(parsed);
      } catch (e) {
        localStorage.removeItem('dongdo_guest_session');
      }
    }
  }, []);

  // Connect WebSocket when guest session is established
  useEffect(() => {
    if (!guest) return;

    const ws = new WSClient(guest.session_id, guest.display_name || guest.guest_id, 'guest');
    wsRef.current = ws;
    ws.connect();

    // Load initial history
    api
      .getHistory(guest.session_id)
      .then((data) => {
        if (data && data.messages) {
          setMessages(data.messages);
        }
      })
      .catch(console.error);

    // Listen for incoming messages
    ws.on('message', (event: WSEvent) => {
      setIsAiTyping(false);
      const incomingMsg: Message | undefined =
        event.payload?.message ||
        (event.payload?.id || event.payload?.content ? event.payload : undefined);

      if (incomingMsg) {
        setMessages((prev) => {
          // 1. Exact database ID match
          if (prev.some((m) => m.id === incomingMsg.id)) {
            return prev;
          }

          // 2. Client Msg ID match -> replace optimistic message
          if (incomingMsg.client_msg_id) {
            const optIdx = prev.findIndex((m) => m.client_msg_id === incomingMsg.client_msg_id);
            if (optIdx !== -1) {
              const updated = [...prev];
              updated[optIdx] = incomingMsg;
              return updated;
            }
          }

          // 3. Fallback for guest own message deduplication
          if (incomingMsg.sender_type === 'guest') {
            const optIdx = prev.findIndex(
              (m) =>
                m.sender_type === 'guest' &&
                m.content.trim() === incomingMsg.content.trim() &&
                ((typeof m.id === 'number' && m.id > 1700000000000) || !m.id)
            );
            if (optIdx !== -1) {
              const updated = [...prev];
              updated[optIdx] = incomingMsg;
              return updated;
            }
          }

          return [...prev, incomingMsg];
        });
      }
    });

    // Listen for typing indicator
    ws.on('typing', (event: WSEvent) => {
      if (event.payload) {
        setIsAiTyping(event.payload.typing === true);
      }
    });

    // Listen for WebRTC Signaling
    ws.on('call_offer', (event: WSEvent) => {
      setIncomingCall({
        sender_id: event.sender_id || 'Chuyên viên CSKH Đông Đô',
        offer: event.payload,
      });
    });

    ws.on('call_answer', (event: WSEvent) => {
      setCallStatusText('Đang đàm thoại');
      startCallTimer();
      if (rtcRef.current && event.payload) {
        rtcRef.current.handleAnswer(event.payload);
      }
    });

    ws.on('call_end', async () => {
      setIsCallActive(false);
      setIncomingCall(null);
      clearInterval(callTimerRef.current);
      setCallDuration(0);
      const rtc = rtcRef.current;
      rtcRef.current = null;
      if (rtc) {
        try {
          await rtc.endCall(false);
        } catch (e) {}
      }
    });

    return () => {
      ws.disconnect();
      clearInterval(callTimerRef.current);
    };
  }, [guest]);

  // Scroll to bottom on new message smoothly inside message container
  useEffect(() => {
    if (chatContainerRef.current) {
      chatContainerRef.current.scrollTo({
        top: chatContainerRef.current.scrollHeight,
        behavior: 'smooth',
      });
    }
  }, [messages, isAiTyping]);

  const handleRegisterGuest = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputName.trim()) return;

    setIsRegistering(true);
    try {
      const session = await api.registerGuest(inputName.trim(), inputPhone.trim());
      setGuest(session);
      localStorage.setItem('dongdo_guest_session', JSON.stringify(session));
    } catch (err: any) {
      alert(err.message || 'Lỗi khởi tạo phiên chat');
    } finally {
      setIsRegistering(false);
    }
  };

  const generateUUID = () => {
    if (typeof crypto !== 'undefined' && crypto.randomUUID) {
      return crypto.randomUUID();
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const r = (Math.random() * 16) | 0;
      const v = c === 'x' ? r : (r & 0x3) | 0x8;
      return v.toString(16);
    });
  };

  const handleSendMessage = async (textToSend?: string) => {
    const content = (textToSend || inputText).trim();
    if (!content || !guest || isSending) return;

    setInputText('');
    setIsSending(true);

    const clientMsgId = generateUUID();
    const optimisticMsg: Message = {
      id: Date.now(),
      session_id: guest.session_id,
      sender_type: 'guest',
      sender_id: guest.display_name || guest.guest_id,
      content: content,
      client_msg_id: clientMsgId,
      created_at: new Date().toISOString(),
    };

    setMessages((prev) => [...prev, optimisticMsg]);
    setIsAiTyping(true);

    try {
      // 1. Try sending via WebSocket with correct target session ID
      let sentViaWS = false;
      if (wsRef.current) {
        sentViaWS = wsRef.current.send('message', content, null, guest.session_id, clientMsgId);
      }

      // 2. Only backup via REST API if WS is disconnected
      if (!sentViaWS) {
        await api.sendMessage(guest.session_id, guest.display_name, content, clientMsgId);
      }
    } catch (err: any) {
      console.error('Failed to send message:', err);
    } finally {
      setIsSending(false);
    }
  };

  const handleNewChat = () => {
    localStorage.removeItem('dongdo_guest_session');
    setGuest(null);
    setMessages([]);
    setInputName('');
    setInputPhone('');
  };

  // WebRTC Voice Call Handlers
  const handleStartCall = async () => {
    if (!guest || !wsRef.current) return;

    setIsCallActive(true);
    setCallStatusText('Đang đổ chuông tới CSKH...');
    setCallDuration(0);
    clearInterval(callTimerRef.current);

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
          clearInterval(callTimerRef.current);
          setCallDuration(0);
        }
      },
      (stream) => {
        if (remoteAudioRef.current) {
          remoteAudioRef.current.srcObject = stream;
          remoteAudioRef.current.play().catch(console.error);
        }
      }
    );

    rtcRef.current = rtc;
    await rtc.startCall();
  };

  const handleEndCall = async (broadcast: boolean = true) => {
    const finalDuration = callDuration;
    setIsCallActive(false);
    setIncomingCall(null);
    clearInterval(callTimerRef.current);
    setCallDuration(0);

    const rtc = rtcRef.current;
    rtcRef.current = null;
    if (rtc) {
      try {
        await rtc.endCall(broadcast, finalDuration);
      } catch (err) {
        console.warn('Error ending RTC call:', err);
      }
    }
  };

  const handleAnswerCall = async () => {
    if (!guest || !wsRef.current || !incomingCall) return;
    setIsCallActive(true);
    setCallStatusText('Đang đàm thoại');
    startCallTimer();

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
          clearInterval(callTimerRef.current);
          setCallDuration(0);
        }
      },
      (stream) => {
        if (remoteAudioRef.current) {
          remoteAudioRef.current.srcObject = stream;
          remoteAudioRef.current.play().catch(console.error);
        }
      }
    );
    rtcRef.current = rtc;
    if (incomingCall.offer) {
      await rtc.handleOffer(incomingCall.offer);
    }
    setIncomingCall(null);
  };

  const handleDeclineCall = () => {
    if (wsRef.current && guest) {
      wsRef.current.send('call_end', '', {}, guest.session_id);
    }
    setIncomingCall(null);
  };

  const startCallTimer = () => {
    clearInterval(callTimerRef.current);
    setCallDuration(0);
    callTimerRef.current = setInterval(() => {
      setCallDuration((prev) => prev + 1);
    }, 1000);
  };

  const formatCallTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  };

  // ============================================================
  // PRE-CHAT NAME MODAL (BẮT BUỘC NHẬP TÊN TRƯỚC KHI CHAT/CALL)
  // ============================================================
  if (!guest) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4 bg-[#0A0F1D] relative overflow-hidden">
        <div className="absolute -top-40 -left-40 w-96 h-96 bg-[#1C2D56]/40 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute -bottom-40 -right-40 w-96 h-96 bg-[#95252E]/30 rounded-full blur-3xl pointer-events-none" />

        <div className="w-full max-w-md glass-panel-brand p-8 rounded-2xl relative z-10 shadow-2xl">
          <div className="text-center mb-8">
            <div className="inline-flex items-center justify-center p-3 rounded-2xl bg-[#1C2D56] border border-[#95252E]/40 mb-4 shadow-lg">
              <img
                src="/logo/Logo Dọc_Trắng.svg"
                alt="Đông Đô Partners"
                className="h-14 w-auto object-contain"
                onError={(e) => { e.currentTarget.style.display = 'none'; }}
              />
            </div>
            <h1 className="text-2xl font-bold text-white tracking-tight">Đông Đô Partners</h1>
            <p className="text-sm text-slate-400 mt-1">Cổng Tư Vấn Hàng Hóa Phái Sinh &amp; DDP Invest</p>
          </div>

          <form onSubmit={handleRegisterGuest} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                Họ và tên của bạn <span className="text-rose-400">*</span>
              </label>
              <div className="relative">
                <User className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
                <input
                  type="text"
                  required
                  value={inputName}
                  onChange={(e) => setInputName(e.target.value)}
                  placeholder="Ví dụ: Anh Tuấn / Chị Lan"
                  className="w-full pl-10 pr-4 py-3 rounded-xl glass-input text-sm"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                Số điện thoại / Zalo <span className="text-slate-500 font-normal">(Tùy chọn)</span>
              </label>
              <div className="relative">
                <Phone className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
                <input
                  type="tel"
                  value={inputPhone}
                  onChange={(e) => setInputPhone(e.target.value)}
                  placeholder="Để chuyên viên gọi lại khi cần"
                  className="w-full pl-10 pr-4 py-3 rounded-xl glass-input text-sm"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={isRegistering}
              className="w-full py-3.5 px-4 rounded-xl btn-brand-primary flex items-center justify-center space-x-2 text-sm font-semibold mt-6 cursor-pointer"
            >
              <span>{isRegistering ? 'Đang kết nối...' : 'Bắt Đầu Tư Vấn Ngay'}</span>
            </button>
          </form>

          <div className="mt-8 text-center text-xs text-slate-500 flex items-center justify-center space-x-1.5 border-t border-slate-700/50 pt-4">
            <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
            <span>Hệ thống bảo mật dữ liệu khách hàng 100%</span>
          </div>
        </div>
      </div>
    );
  }

  // ============================================================
  // MAIN CUSTOMER CHAT INTERFACE
  // ============================================================
  return (
    <div className="min-h-screen bg-[#0A0F1D] flex flex-col justify-between">
      <audio ref={remoteAudioRef} autoPlay />

      {/* Incoming Call Notification Banner for Guest */}
      {incomingCall && (
        <div className="fixed top-6 right-6 z-50 p-4 rounded-2xl glass-panel-brand border-2 border-rose-500 shadow-2xl flex items-center space-x-4 animate-bounce">
          <div className="w-12 h-12 rounded-full bg-rose-600 flex items-center justify-center text-white animate-pulse">
            <Phone className="w-6 h-6" />
          </div>
          <div>
            <div className="text-xs font-semibold text-rose-400 uppercase tracking-wider">Cuộc gọi từ Đông Đô!</div>
            <div className="text-sm font-bold text-white">{incomingCall.sender_id} đang gọi...</div>
          </div>
          <div className="flex items-center space-x-2">
            <button
              onClick={handleAnswerCall}
              className="p-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs flex items-center space-x-1"
            >
              <Phone className="w-4 h-4" />
              <span>Nghe máy</span>
            </button>
            <button
              onClick={handleDeclineCall}
              className="p-2.5 rounded-xl bg-slate-700 hover:bg-slate-600 text-slate-300 text-xs"
            >
              <PhoneOff className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* Voice Call Active Modal */}
      {isCallActive && (
        <div className="fixed inset-0 z-50 bg-black/80 flex items-center justify-center p-4">
          <div className="w-full max-w-sm glass-panel-brand p-8 rounded-3xl text-center space-y-6 shadow-2xl">
            <div className="w-24 h-24 mx-auto rounded-full bg-[#1C2D56] border-2 border-[#95252E] flex items-center justify-center text-rose-400 animate-pulse shadow-lg">
              <Phone className="w-10 h-10" />
            </div>

            <div>
              <h3 className="text-lg font-bold text-white">Chăm Sóc Khách Hàng Đông Đô</h3>
              <p className="text-xs text-rose-300 mt-1">{callStatusText}</p>
              <div className="text-2xl font-mono font-bold text-white mt-3">
                {formatCallTime(callDuration)}
              </div>
            </div>

            <div className="flex items-center justify-center space-x-4 pt-2">
              <button
                onClick={() => {
                  if (rtcRef.current) {
                    const muted = rtcRef.current.toggleMute();
                    setIsMuted(muted);
                  }
                }}
                className="p-4 rounded-full bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
              >
                {isMuted ? <MicOff className="w-6 h-6 text-rose-400" /> : <Mic className="w-6 h-6" />}
              </button>

              <button
                onClick={() => handleEndCall()}
                className="p-4 rounded-full bg-rose-600 hover:bg-rose-500 text-white transition shadow-lg"
              >
                <PhoneOff className="w-6 h-6" />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Main Header */}
      <header className="h-16 px-6 bg-[#0D1527] border-b border-[#1C2D56] flex items-center justify-between shrink-0 shadow-md">
        <div className="flex items-center space-x-3">
          <img
            src="/logo/Logo Ngang_Trắng.svg"
            alt="Đông Đô Partners"
            className="h-9 w-auto object-contain"
            onError={(e) => { e.currentTarget.style.display = 'none'; }}
          />
          <div>
            <h1 className="text-sm font-bold text-white leading-tight">Đông Đô Partners</h1>
            <div className="flex items-center space-x-1.5 mt-0.5">
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
              <span className="text-[11px] text-slate-400">Chuyên viên CSKH đang trực tuyến</span>
            </div>
          </div>
        </div>

        <div className="flex items-center space-x-2.5">
          {/* Voice Call Button */}
          <button
            onClick={handleStartCall}
            disabled={isCallActive}
            className="px-3.5 py-2 rounded-xl bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 border border-emerald-500/40 text-xs font-semibold flex items-center space-x-1.5 transition shadow"
            title="Đàm thoại thoại trực tiếp qua Internet miễn phí"
          >
            <Phone className="w-3.5 h-3.5" />
            <span>Gọi CSKH (Miễn phí)</span>
          </button>

          {/* CS Portal Link */}
          <a
            href="/admin"
            target="_blank"
            rel="noreferrer"
            className="px-3 py-2 rounded-xl bg-[#1C2D56] hover:bg-[#2A3F74] text-xs text-slate-300 border border-slate-700/60 flex items-center space-x-1 transition"
            title="Cổng Quản trị CSKH"
          >
            <Lock className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Cổng CSKH</span>
          </a>

          {/* New Chat Button */}
          <button
            onClick={handleNewChat}
            className="p-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white transition"
            title="Bắt đầu cuộc trò chuyện mới"
          >
            <RotateCcw className="w-4 h-4" />
          </button>
        </div>
      </header>

      {/* Messages Scroll Area */}
      <main ref={chatContainerRef} className="flex-1 max-w-4xl w-full mx-auto p-4 sm:p-6 overflow-y-auto space-y-4">
        {messages.length === 0 ? (
          <div className="py-12 px-4 text-center max-w-xl mx-auto space-y-6">
            <div className="w-16 h-16 mx-auto rounded-2xl bg-[#1C2D56] border border-[#95252E]/40 flex items-center justify-center text-rose-400 shadow-xl">
              <Sparkles className="w-8 h-8" />
            </div>

            <div>
              <h2 className="text-xl font-bold text-white">Xin chào {guest.display_name}! 👋</h2>
              <p className="text-xs sm:text-sm text-slate-300 mt-2 leading-relaxed">
                Em là chuyên viên Chăm sóc Khách hàng của <strong>Đông Đô Partners</strong>. Em có thể giải đáp cho anh/chị về:
              </p>
            </div>

            {/* Quick Suggestion Chips */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5 pt-2">
              {[
                { label: 'Hàng hóa phái sinh là gì?', icon: '💹' },
                { label: 'Hướng dẫn nạp tiền vào tài khoản', icon: '💰' },
                { label: 'Giới thiệu nền tảng DDP Invest', icon: '📱' },
                { label: 'Cách quản trị rủi ro khi giao dịch?', icon: '🛡️' },
              ].map((chip, idx) => (
                <button
                  key={idx}
                  onClick={() => handleSendMessage(chip.label)}
                  className="p-3 rounded-xl bg-[#162344]/70 hover:bg-[#1C2D56] border border-slate-700/60 text-left text-xs text-slate-200 hover:text-white transition-all flex items-center space-x-2.5"
                >
                  <span className="text-base">{chip.icon}</span>
                  <span className="font-medium">{chip.label}</span>
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
                className={`flex flex-col ${isGuest ? 'items-end' : 'items-start'} space-y-1`}
              >
                <div className="text-[10px] text-slate-500 px-1">
                  {isGuest ? `👤 ${guest.display_name}` : isCS ? `👨‍💼 Chuyên viên ${m.sender_id || 'CSKH'}` : '🤖 Trợ lý AI Đông Đô'}
                  {' • '}
                  {new Date(m.created_at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}
                </div>

                <div
                  className={`max-w-[88%] sm:max-w-[78%] p-4 rounded-2xl text-sm leading-relaxed shadow-sm ${
                    isGuest
                      ? 'bg-[#162344] text-white border border-slate-700/70 rounded-tr-sm'
                      : isCS
                      ? 'bg-[#95252E] text-white shadow-lg rounded-tl-sm'
                      : 'bg-[#1C2D56] text-slate-100 border border-[#2A3F74] rounded-tl-sm'
                  }`}
                >
                  {isGuest ? (
                    <p className="whitespace-pre-wrap">{m.content}</p>
                  ) : (
                    <MarkdownRenderer content={m.content} />
                  )}
                </div>
              </div>
            );
          })
        )}

        {/* AI Typing Animation */}
        {isAiTyping && (
          <div className="flex flex-col items-start space-y-1">
            <div className="text-[10px] text-slate-500 px-1">🤖 Trợ lý AI Đông Đô đang gõ...</div>
            <div className="p-3.5 rounded-2xl bg-[#1C2D56] border border-[#2A3F74] rounded-tl-sm flex items-center space-x-1.5">
              <span className="typing-dot" />
              <span className="typing-dot" />
              <span className="typing-dot" />
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </main>

      {/* Input Area */}
      <footer className="p-4 bg-[#0D1527] border-t border-[#1C2D56]">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            handleSendMessage();
          }}
          className="max-w-4xl mx-auto flex items-center space-x-2"
        >
          <input
            type="text"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            placeholder="Nhập câu hỏi của bạn về Hàng hóa phái sinh, DDP Invest..."
            className="flex-1 px-4 py-3.5 rounded-xl glass-input text-sm"
          />
          <button
            type="submit"
            disabled={!inputText.trim() || isSending}
            className="px-5 py-3.5 rounded-xl btn-brand-primary flex items-center justify-center font-semibold text-sm cursor-pointer disabled:opacity-50 transition shadow-lg"
          >
            <Send className="w-4 h-4" />
          </button>
        </form>
        <div className="text-center text-[10px] text-slate-500 mt-2">
          Đông Đô Partners AI hỗ trợ trực tuyến 24/7 • Dữ liệu mã hóa an toàn
        </div>
      </footer>
    </div>
  );
}

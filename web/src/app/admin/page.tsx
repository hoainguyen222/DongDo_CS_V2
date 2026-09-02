'use client';

import React, { useState, useEffect, useRef } from 'react';
import {
  Inbox,
  Brain,
  BookOpen,
  BarChart3,
  Settings,
  RefreshCw,
  LogOut,
  Send,
  CheckCircle2,
  XCircle,
  Phone,
  PhoneOff,
  Mic,
  MicOff,
  UserCheck,
  AlertCircle,
  Trash2,
  Upload,
  ExternalLink,
  MessageSquare,
  Search,
  Check,
  Edit3,
  Headphones,
  Users,
  PhoneCall,
  ShieldAlert,
  AlertTriangle,
} from 'lucide-react';
import { api } from '@/lib/api';
import { WSClient } from '@/lib/ws';
import { WebRTCManager } from '@/lib/webrtc';
import { ChatCase, CustomerProfile, LearningItem, KnowledgeOverview, AnalyticsStats, SystemConfig, Message, QAPair, SystemErrorItem, RolePermissionItem } from '@/lib/types';
import { MarkdownRenderer } from '@/components/MarkdownRenderer';
import { PartnerDashboardView } from '@/components/partner/PartnerDashboardView';
import { PartnerAnalyticsView } from '@/components/partner/PartnerAnalyticsView';
import { PartnerConfigView } from '@/components/partner/PartnerConfigView';
import { TestDataUploadView } from '@/components/partner/TestDataUploadView';
import { ErrorCenterModal, generateSuggestedFix } from '@/components/ErrorCenterModal';
import { LayoutDashboard, TrendingUp, SlidersHorizontal, TestTube } from 'lucide-react';

type TabType = 'inbox' | 'customers' | 'calls' | 'learning' | 'knowledge' | 'analytics' | 'config' | 'partner_dashboard' | 'partner_analytics' | 'partner_config' | 'update_data_test';

interface PaginationProps {
  currentPage: number;
  pageSize: number;
  totalItems: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  compact?: boolean;
}

const Pagination: React.FC<PaginationProps> = ({
  currentPage,
  pageSize,
  totalItems,
  onPageChange,
  onPageSizeChange,
  compact = false,
}) => {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  const startItem = totalItems === 0 ? 0 : (currentPage - 1) * pageSize + 1;
  const endItem = Math.min(currentPage * pageSize, totalItems);

  if (totalItems === 0) return null;

  return (
    <div className={`flex flex-col ${compact ? 'space-y-2' : 'sm:flex-row'} items-center justify-between gap-3 pt-3.5 border-t border-slate-800/80 text-xs text-slate-400`}>
      <div className="flex items-center space-x-2">
        <span className="text-[11px] text-slate-400">Hiển thị:</span>
        <select
          value={pageSize}
          onChange={(e) => {
            onPageSizeChange(Number(e.target.value));
            onPageChange(1);
          }}
          className="bg-[#0A0F1D] border border-slate-700 text-white rounded-lg px-2 py-1 text-xs focus:outline-none focus:border-[#B32D38] cursor-pointer"
        >
          <option value={10}>10 / trang</option>
          <option value={20}>20 / trang</option>
          <option value={50}>50 / trang</option>
        </select>
        <span className="text-[11px] text-slate-500">
          ({startItem}-{endItem} / <strong className="text-slate-300 font-semibold">{totalItems}</strong>)
        </span>
      </div>

      <div className="flex items-center space-x-1.5">
        <button
          type="button"
          onClick={() => onPageChange(1)}
          disabled={currentPage <= 1}
          className="px-2 py-1 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-slate-800 transition text-[11px] cursor-pointer"
          title="Trang đầu"
        >
          ⏮
        </button>
        <button
          type="button"
          onClick={() => onPageChange(Math.max(1, currentPage - 1))}
          disabled={currentPage <= 1}
          className="px-2.5 py-1 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-slate-800 transition text-[11px] cursor-pointer"
          title="Trang trước"
        >
          ◀
        </button>

        <span className="px-2.5 py-1 text-[11px] font-semibold text-white bg-[#1C2D56]/80 rounded-lg border border-slate-700">
          {currentPage} / {totalPages}
        </span>

        <button
          type="button"
          onClick={() => onPageChange(Math.min(totalPages, currentPage + 1))}
          disabled={currentPage >= totalPages}
          className="px-2.5 py-1 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-slate-800 transition text-[11px] cursor-pointer"
          title="Trang sau"
        >
          ▶
        </button>
        <button
          type="button"
          onClick={() => onPageChange(totalPages)}
          disabled={currentPage >= totalPages}
          className="px-2 py-1 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-slate-800 transition text-[11px] cursor-pointer"
          title="Trang cuối"
        >
          ⏭
        </button>
      </div>
    </div>
  );
};

export default function AdminPage() {
  // Auth state
  const [token, setToken] = useState<string | null>(null);
  const [currentUser, setCurrentUser] = useState<{ username: string; full_name: string; role: string } | null>(null);
  const [loginUsername, setLoginUsername] = useState('');
  const [loginPassword, setLoginPassword] = useState('');
  const [loginError, setLoginError] = useState('');
  const [isLoggingIn, setIsLoggingIn] = useState(false);

  // Active Tab
  const [activeTab, setActiveTab] = useState<TabType>('inbox');

  // Customer Management Tab
  const [customers, setCustomers] = useState<CustomerProfile[]>([]);
  const [customerSearch, setCustomerSearch] = useState('');
  const [selectedCustomer, setSelectedCustomer] = useState<CustomerProfile | null>(null);
  const [isLoadingCustomers, setIsLoadingCustomers] = useState(false);
  const [customerPage, setCustomerPage] = useState(1);
  const [customerPageSize, setCustomerPageSize] = useState(10);
  const [customerTotal, setCustomerTotal] = useState(0);

  // Tab 1: Live CS Inbox
  const [cases, setCases] = useState<ChatCase[]>([]);
  const [caseFilter, setCaseFilter] = useState('');
  const [selectedCase, setSelectedCase] = useState<ChatCase | null>(null);
  const [caseMessages, setCaseMessages] = useState<Message[]>([]);
  const [replyText, setReplyText] = useState('');
  const [isSendingReply, setIsSendingReply] = useState(false);
  const [resolveNote, setResolveNote] = useState('');
  const [showResolveModal, setShowResolveModal] = useState(false);
  const [modalQAPairs, setModalQAPairs] = useState<QAPair[]>([]);
  const [modalEnableLearn, setModalEnableLearn] = useState(true);
  const [casePage, setCasePage] = useState(1);
  const [casePageSize, setCasePageSize] = useState(10);
  const [caseTotal, setCaseTotal] = useState(0);

  // Tab 2: Learning
  const [pendingLearning, setPendingLearning] = useState<LearningItem[]>([]);
  const [autoLearnEnabled, setAutoLearnEnabled] = useState(false);
  const [editingItemId, setEditingItemId] = useState<number | null>(null);
  const [editQuestion, setEditQuestion] = useState('');
  const [editAnswer, setEditAnswer] = useState('');
  const [learningPage, setLearningPage] = useState(1);
  const [learningPageSize, setLearningPageSize] = useState(10);
  const [learningTotal, setLearningTotal] = useState(0);

  // Tab 3: Knowledge
  const [knowledge, setKnowledge] = useState<KnowledgeOverview | null>(null);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [uploadMsg, setUploadMsg] = useState('');
  const [docPage, setDocPage] = useState(1);
  const [docPageSize, setDocPageSize] = useState(10);

  // Tab 4: Analytics
  const [analytics, setAnalytics] = useState<AnalyticsStats | null>(null);

  // Tab 5: Config
  const [config, setConfig] = useState<SystemConfig | null>(null);
  const [configPrompt, setConfigPrompt] = useState('');
  const [configModel, setConfigModel] = useState('');
  const [configTemp, setConfigTemp] = useState(0.1);
  const [saveConfigMsg, setSaveConfigMsg] = useState('');

  // Detailed Error Reporting System State
  const [systemErrors, setSystemErrors] = useState<SystemErrorItem[]>([]);
  const [showErrorCenterModal, setShowErrorCenterModal] = useState(false);
  const [toastError, setToastError] = useState<{ title: string; source: string; details: string } | null>(null);

  const reportSystemError = (source: string, title: string, details: string, severity: 'low' | 'medium' | 'high' = 'high') => {
    const now = new Date();
    const pad = (n: number) => n.toString().padStart(2, '0');
    const timestamp = `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())} - ${pad(now.getDate())}/${pad(now.getMonth() + 1)}/${now.getFullYear()}`;
    const id = Date.now().toString() + '-' + Math.random().toString(36).substr(2, 4);
    const suggestedFix = generateSuggestedFix(source, details || title);

    const newError: SystemErrorItem = {
      id,
      timestamp,
      source,
      title,
      details: details || title,
      severity,
      isHandled: false,
      suggestedFix,
    };

    setSystemErrors((prev) => [newError, ...prev]);
    setToastError({ source, title, details: details || title });
    setTimeout(() => setToastError(null), 6000);

    // Persist to Backend DB
    api.createSystemError(newError);
  };

  const handleMarkErrorAsHandled = (id: string) => {
    setSystemErrors((prev) =>
      prev.map((e) => (e.id === id ? { ...e, isHandled: true } : e))
    );
    // Persist to Backend DB
    api.markSystemErrorHandled(id);
  };

  const handleClearHandledErrors = () => {
    setSystemErrors((prev) => prev.filter((e) => !e.isHandled));
  };

  // Voice Call History & Pagination
  const [showVoiceHistoryModal, setShowVoiceHistoryModal] = useState(false);
  const [voiceCalls, setVoiceCalls] = useState<any[]>([]);
  const [isLoadingVoiceCalls, setIsLoadingVoiceCalls] = useState(false);
  const [callPage, setCallPage] = useState(1);
  const [callPageSize, setCallPageSize] = useState(10);
  const [callTotal, setCallTotal] = useState(0);

  // WebRTC Voice Call
  const [incomingCall, setIncomingCall] = useState<{ session_id: string; caller_id: string; offer?: any } | null>(null);
  const [isCallActive, setIsCallActive] = useState(false);
  const [callDuration, setCallDuration] = useState(0);
  const [isMuted, setIsMuted] = useState(false);
  const rtcRef = useRef<WebRTCManager | null>(null);
  const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
  const callTimerRef = useRef<any>(null);

  // Edit Customer Modal
  const [showEditCustomerModal, setShowEditCustomerModal] = useState(false);
  const [editCustomerName, setEditCustomerName] = useState('');
  const [editCustomerPhone, setEditCustomerPhone] = useState('');
  const [isUpdatingCustomer, setIsUpdatingCustomer] = useState(false);

  // WebSocket & Dynamic selectedCaseRef
  const wsRef = useRef<WSClient | null>(null);
  const selectedCaseRef = useRef<ChatCase | null>(null);
  const chatContainerRef = useRef<HTMLDivElement | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    selectedCaseRef.current = selectedCase;
  }, [selectedCase]);

  // Check existing session on mount
  useEffect(() => {
    const savedToken = localStorage.getItem('dongdo_token');
    const savedUser = localStorage.getItem('dongdo_user');
    if (savedToken && savedUser) {
      try {
        setToken(savedToken);
        setCurrentUser(JSON.parse(savedUser));
      } catch (e) {
        localStorage.removeItem('dongdo_token');
        localStorage.removeItem('dongdo_user');
      }
    }
  }, []);

  // RBAC Role Permissions State
  const [userPermissions, setUserPermissions] = useState<RolePermissionItem[]>([]);

  const getNormalizedRole = (role?: string): string => {
    if (!role) return 'Staff';
    const r = role.trim().toLowerCase();
    if (r.includes('owner')) return 'Owner';
    if (r.includes('admin')) return 'Admin';
    if (r.includes('leader')) return 'Leader';
    return 'Staff';
  };

  const getFeaturePermission = (featureKey: string): 'act' | 'view' | 'none' => {
    if (!currentUser) return 'act';
    const normRole = getNormalizedRole(currentUser.role);
    if (normRole === 'Owner') return 'act'; // Only Owner bypasses matrix lookup

    // 1. Exact match lookup (main key or sub-feature key)
    const item = userPermissions.find(
      (p) => p.role_name.toLowerCase() === normRole.toLowerCase() && p.feature_key === featureKey
    );
    if (item && item.permission_level) {
      return item.permission_level;
    }

    // 2. Fallback to parent feature key if sub-feature key (e.g. partner_config.accounts -> partner_config)
    if (featureKey.includes('.')) {
      const parentKey = featureKey.split('.')[0];
      const parentItem = userPermissions.find(
        (p) => p.role_name.toLowerCase() === normRole.toLowerCase() && p.feature_key === parentKey
      );
      if (parentItem && parentItem.permission_level) {
        return parentItem.permission_level;
      }
    }

    // Default fallbacks if perms not loaded yet
    if (normRole === 'Leader') {
      return featureKey.startsWith('config') ? 'none' : featureKey.startsWith('partner_config') ? 'view' : 'act';
    }
    if (featureKey.startsWith('inbox') || featureKey.startsWith('calls')) return 'act';
    if (featureKey.startsWith('partner_dashboard') || featureKey.startsWith('customers') || featureKey.startsWith('knowledge')) return 'view';
    return 'none';
  };

  // Load persisted system errors & RBAC permissions from DB when logged in
  useEffect(() => {
    if (!token) return;
    api.listSystemErrors().then((errs) => {
      if (errs && errs.length > 0) {
        setSystemErrors(errs);
      }
    }).catch(() => {});

    api.listRolePermissions().then((perms) => {
      if (perms && perms.length > 0) {
        setUserPermissions(perms);
      }
    }).catch(() => {});
  }, [token]);

  // Auto-redirect if activeTab has 'none' permission level
  useEffect(() => {
    if (!currentUser || userPermissions.length === 0) return;
    if (getFeaturePermission(activeTab) === 'none') {
      const allowedTabs: TabType[] = ['partner_dashboard', 'inbox', 'customers', 'calls', 'learning', 'knowledge', 'partner_analytics', 'partner_config', 'config'];
      const firstAllowed = allowedTabs.find((t) => getFeaturePermission(t) !== 'none');
      if (firstAllowed) {
        setActiveTab(firstAllowed);
      }
    }
  }, [activeTab, currentUser, userPermissions]);

  // Initialize WebSocket when logged in
  useEffect(() => {
    if (!token || !currentUser) return;

    const ws = new WSClient('admin_inbox', currentUser.username, currentUser.role);
    ws.connect();
    wsRef.current = ws;

    // Listen for realtime events
    ws.on('case_update', () => {
      loadCases();
      loadCustomers(false);
      loadAnalytics();
    });

    ws.on('message', (event) => {
      const active = selectedCaseRef.current;
      if (active && event.session_id === active.session_id) {
        loadCaseDetail(active.session_id);
      }
      loadCases();
      loadCustomers(false);
    });

    ws.on('call_ring', (event) => {
      const sID = (event.payload && event.payload.session_id) || (event.session_id !== 'admin_inbox' ? event.session_id : '');
      const cID = (event.payload && event.payload.caller_id) || event.sender_id || 'Khách hàng';
      const offerData = (event.payload && event.payload.offer) || event.payload;
      if (sID) {
        setIncomingCall({
          session_id: sID,
          caller_id: cID,
          offer: offerData,
        });
      }
    });

    ws.on('call_offer', (event) => {
      const sID = (event.payload && event.payload.session_id) || (event.session_id !== 'admin_inbox' ? event.session_id : '');
      const cID = (event.payload && event.payload.caller_id) || event.sender_id || 'Khách hàng';
      if (sID) {
        setIncomingCall({
          session_id: sID,
          caller_id: cID,
          offer: event.payload,
        });
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
      loadCases();
      loadVoiceCalls(false);
      loadLearningQueue();
    });

    // Initial data load
    loadAllData();

    // Polling backup every 15s without disruptive loading indicators
    const interval = setInterval(() => {
      loadCases();
      loadLearningQueue();
    }, 15000);

    return () => {
      ws.disconnect();
      clearInterval(interval);
    };
  }, [token, currentUser?.username]);

  // Scroll messages to bottom inside chat container only (prevents full admin page jump)
  useEffect(() => {
    if (chatContainerRef.current) {
      chatContainerRef.current.scrollTop = chatContainerRef.current.scrollHeight;
    }
  }, [caseMessages]);

  const loadCustomers = async (showLoading: boolean = false, page: number = customerPage, pageSize: number = customerPageSize, search: string = customerSearch) => {
    if (showLoading) setIsLoadingCustomers(true);
    try {
      const data = await api.getCustomers(page, pageSize, search);
      setCustomers(data.customers || []);
      setCustomerTotal(data.total ?? (data.customers?.length || 0));
    } catch (err) {
      console.error('Failed to load customers:', err);
    } finally {
      if (showLoading) setIsLoadingCustomers(false);
    }
  };

  const loadVoiceCalls = async (showLoading: boolean = false, page: number = callPage, pageSize: number = callPageSize) => {
    if (showLoading) setIsLoadingVoiceCalls(true);
    try {
      const data = await api.getVoiceCalls(undefined, page, pageSize);
      setVoiceCalls(data.calls || []);
      setCallTotal(data.total ?? (data.calls?.length || 0));
    } catch (err) {
      console.error('Failed to load voice calls:', err);
    } finally {
      if (showLoading) setIsLoadingVoiceCalls(false);
    }
  };

  const loadAllData = () => {
    loadCases(1, casePageSize, caseFilter);
    loadCustomers(false, 1, customerPageSize, customerSearch);
    loadVoiceCalls(false, 1, callPageSize);
    loadLearningQueue(1, learningPageSize);
    loadKnowledge();
    loadAnalytics();
    loadConfig();
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoginError('');
    setIsLoggingIn(true);
    try {
      const data = await api.login(loginUsername, loginPassword);
      setToken(data.token);
      const user = { username: data.username, full_name: data.full_name, role: data.role };
      setCurrentUser(user);
      localStorage.setItem('dongdo_token', data.token);
      localStorage.setItem('dongdo_user', JSON.stringify(user));
    } catch (err: any) {
      setLoginError(err.message || 'Tên đăng nhập hoặc mật khẩu không chính xác');
    } finally {
      setIsLoggingIn(false);
    }
  };

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch (e) {}
    localStorage.removeItem('dongdo_token');
    localStorage.removeItem('dongdo_user');
    setToken(null);
    setCurrentUser(null);
    setSelectedCase(null);
  };

  const loadCases = async (page = casePage, pageSize = casePageSize, filter = caseFilter) => {
    try {
      const data = await api.listCases(filter, page, pageSize);
      setCases(data.cases || []);
      setCaseTotal(data.total ?? (data.cases?.length || 0));
    } catch (e) {}
  };

  const loadCaseDetail = async (sessionID: string) => {
    try {
      const data = await api.getCaseDetail(sessionID);
      setCaseMessages(data.messages || []);
    } catch (e) {}
  };

  const selectCase = (c: ChatCase) => {
    setSelectedCase(c);
    loadCaseDetail(c.session_id);
  };

  const handleTakeCase = async () => {
    if (!selectedCase) return;
    try {
      await api.takeCase(selectedCase.session_id);
      setSelectedCase({ ...selectedCase, status: 'HUMAN_CS_ACTIVE', assigned_cs: currentUser?.username || '' });
      loadCases();
      loadCaseDetail(selectedCase.session_id);
    } catch (err: any) {
      alert(err.message);
    }
  };

  const extractAllQAPairs = (messages: Message[]): QAPair[] => {
    const pairs: QAPair[] = [];
    let lastUserText = '';

    for (let i = 0; i < messages.length; i++) {
      const m = messages[i];
      if (m.sender_type === 'guest') {
        lastUserText = m.content.trim();
      } else if ((m.sender_type === 'cs' || m.sender_type === 'human_cs') && lastUserText) {
        const csReply = m.content.trim();
        if (csReply) {
          if (csReply.includes('Em đã tham gia cuộc trò chuyện và sẽ hỗ trợ')) {
            continue;
          }
          const existing = pairs[pairs.length - 1];
          if (existing && existing.question === lastUserText) {
            existing.answer += '\n' + csReply;
          } else {
            pairs.push({
              question: lastUserText,
              answer: csReply,
            });
          }
        }
      }
    }
    return pairs;
  };

  const openResolveModal = () => {
    if (!selectedCase) return;
    const extracted = extractAllQAPairs(caseMessages);
    if (extracted.length === 0) {
      const userMsgs = caseMessages.filter((m) => m.sender_type === 'guest');
      const lastQ = userMsgs.length > 0 ? userMsgs[userMsgs.length - 1].content.trim() : '';
      setModalQAPairs([{ question: lastQ, answer: '' }]);
    } else {
      setModalQAPairs(extracted);
    }
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
      const res = await api.resolveCase(selectedCase.session_id, resolveNote, validPairs);
      setShowResolveModal(false);
      setResolveNote('');
      setSelectedCase({ ...selectedCase, status: 'RESOLVED' });
      loadCases();
      loadLearningQueue();
      loadCustomers();
      alert(res.message || 'Đã đóng case thành công!');
    } catch (err: any) {
      const msg = err.message || 'Lỗi đóng ca CSKH';
      alert(msg);
      reportSystemError('Đóng Case CSKH', `Lỗi giải quyết ca hỗ trợ [${selectedCase.session_id}]`, msg);
    }
  };

  const handleSendCSMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!replyText.trim() || !selectedCase || isSendingReply) return;

    const content = replyText.trim();
    setReplyText('');
    setIsSendingReply(true);

    try {
      await api.sendCSMessage(selectedCase.session_id, content);
      loadCaseDetail(selectedCase.session_id);
      loadCases();
    } catch (err: any) {
      const msg = err.message || 'Lỗi gửi tin nhắn';
      alert('Lỗi gửi tin: ' + msg);
      reportSystemError('Gửi Tin Nhắn CSKH', `Lỗi gửi tin nhắn cho [${selectedCase.customer_name}]`, msg);
    } finally {
      setIsSendingReply(false);
    }
  };

  const handleClearAllCases = async () => {
    if (!confirm('Bạn có chắc chắn muốn xóa toàn bộ danh sách case test không?')) return;
    try {
      await api.clearAllCases();
      setSelectedCase(null);
      setCaseMessages([]);
      loadCases();
    } catch (err: any) {
      alert(err.message);
      reportSystemError('Xóa Tất Cả Case', 'Lỗi dọn dẹp toàn bộ case test', err.message);
    }
  };

  const handleOpenEditCustomerModal = (customer: CustomerProfile) => {
    setSelectedCustomer(customer);
    setEditCustomerName(customer.display_name || '');
    setEditCustomerPhone(customer.phone || '');
    setShowEditCustomerModal(true);
  };

  const handleSaveCustomerInfo = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCustomer) return;
    setIsUpdatingCustomer(true);
    try {
      await api.updateCustomer(selectedCustomer.guest_id, editCustomerName.trim(), editCustomerPhone.trim());
      setShowEditCustomerModal(false);
      loadCustomers();
      loadCases();
    } catch (err: any) {
      alert(err.message || 'Lỗi cập nhật thông tin khách hàng');
    } finally {
      setIsUpdatingCustomer(false);
    }
  };

  const handleDeleteCustomer = async (guestID: string) => {
    if (!confirm('Bạn có chắc chắn muốn xóa thông tin khách hàng này không?')) return;
    try {
      await api.deleteCustomer(guestID);
      loadCustomers();
    } catch (err: any) {
      alert(err.message || 'Lỗi xóa khách hàng');
    }
  };

  const handleDeleteCase = async (sessionID: string) => {
    if (!confirm('Bạn có chắc chắn muốn xóa ca hỗ trợ này khỏi Live CS Inbox không? (Thông tin khách hàng ở mục Quản lý khách hàng vẫn được giữ nguyên)')) return;
    try {
      await api.deleteCase(sessionID);
      if (selectedCase?.session_id === sessionID) {
        setSelectedCase(null);
        setCaseMessages([]);
      }
      loadCases();
    } catch (err: any) {
      alert(err.message || 'Lỗi xóa case');
    }
  };

  const handleOpenVoiceHistory = async (sessionID?: string) => {
    setShowVoiceHistoryModal(true);
    setIsLoadingVoiceCalls(true);
    try {
      const data = await api.getVoiceCalls(sessionID);
      setVoiceCalls(data.calls || []);
    } catch (err) {
      console.error('Failed to load voice calls:', err);
    } finally {
      setIsLoadingVoiceCalls(false);
    }
  };

  // Learning Queue
  const loadLearningQueue = async (page: number = learningPage, pageSize: number = learningPageSize) => {
    try {
      const data = await api.listPendingLearning(page, pageSize);
      setPendingLearning(data.pending_items || []);
      setLearningTotal(data.total ?? (data.pending_items?.length || 0));
      const settings = await api.getLearningSettings();
      setAutoLearnEnabled(settings.auto_learning_enabled);
    } catch (e) {}
  };

  const handleToggleAutoLearn = async (enabled: boolean) => {
    try {
      await api.updateLearningSettings(enabled);
      setAutoLearnEnabled(enabled);
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleApproveLearning = async (id: number, question?: string, answer?: string) => {
    try {
      const res = await api.approveLearning(id, question, answer);
      loadLearningQueue();
      loadKnowledge();
      if (res?.message) {
        alert(res.message);
      }
    } catch (err: any) {
      const msg = err.message || 'Lỗi phê duyệt mẩu tri thức';
      alert(msg);
      reportSystemError('Duyệt Tri Thức Mới', `Lỗi phê duyệt mẩu tri thức #${id}`, msg);
    }
  };

  const handleRejectLearning = async (id: number) => {
    try {
      await api.rejectLearning(id);
      loadLearningQueue();
    } catch (err: any) {
      alert(err.message);
      reportSystemError('Duyệt Tri Thức Mới', `Lỗi từ chối mẩu tri thức #${id}`, err.message);
    }
  };

  const handleSaveEditLearning = async (id: number) => {
    try {
      await api.updateLearningItem(id, editQuestion, editAnswer);
      setEditingItemId(null);
      loadLearningQueue();
    } catch (err: any) {
      alert(err.message);
      reportSystemError('Duyệt Tri Thức Mới', `Lỗi chỉnh sửa mẩu tri thức #${id}`, err.message);
    }
  };

  const handleResetLearning = async () => {
    if (!confirm('Đặt lại toàn bộ tri thức đã học? Cần ingest lại tài liệu sau khi đặt lại.')) return;
    try {
      await api.resetAllLearning();
      loadLearningQueue();
      loadKnowledge();
    } catch (err: any) {
      alert(err.message);
      reportSystemError('Kho Tri Thức', 'Lỗi đặt lại toàn bộ tri thức đã học', err.message);
    }
  };

  // Knowledge
  const loadKnowledge = async () => {
    try {
      const data = await api.getKnowledge();
      setKnowledge(data);
    } catch (e) {}
  };

  const handleUploadDoc = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!uploadFile) return;
    setIsUploading(true);
    setUploadMsg('Đang tải lên và xử lý vector hoá tài liệu...');
    try {
      const res = await api.uploadDocument(uploadFile);
      setUploadMsg(`✅ Nạp thành công: ${res.message}`);
      setUploadFile(null);
      loadKnowledge();
    } catch (err: any) {
      const msg = err.message || 'Lỗi nạp file tri thức';
      setUploadMsg(`❌ Lỗi: ${msg}`);
      reportSystemError('Kho Tri Thức', `Lỗi nạp tài liệu [${uploadFile.name}]`, msg);
    } finally {
      setIsUploading(false);
    }
  };

  // Analytics
  const loadAnalytics = async () => {
    try {
      const data = await api.getAnalytics();
      setAnalytics(data);
    } catch (e) {}
  };

  // Config
  const loadConfig = async () => {
    try {
      const data = await api.getConfig();
      setConfig(data);
      setConfigPrompt(data.system_prompt);
      setConfigModel(data.llm_model);
      setConfigTemp(data.temperature);
    } catch (e) {}
  };

  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.updateConfig({
        system_prompt: configPrompt,
        llm_model: configModel,
        temperature: configTemp,
      });
      setSaveConfigMsg('✅ Đã lưu cấu hình hệ thống thành công!');
      setTimeout(() => setSaveConfigMsg(''), 4000);
    } catch (err: any) {
      alert(err.message);
    }
  };

  // Voice Call handlers
  const handleAnswerCall = async () => {
    if (!incomingCall || !wsRef.current) return;
    const callData = { ...incomingCall };
    setIsCallActive(true);
    setIncomingCall(null);
    startCallTimer();

    const rtc = new WebRTCManager(
      wsRef.current,
      callData.session_id,
      (state: any) => {
        if (state === 'connected') {
          startCallTimer();
        } else if (state === 'ended') {
          setIsCallActive(false);
          setIncomingCall(null);
          clearInterval(callTimerRef.current);
          setCallDuration(0);
        }
      },
      (stream: any) => {
        if (remoteAudioRef.current) {
          remoteAudioRef.current.srcObject = stream;
          remoteAudioRef.current.play().catch(console.error);
        }
      }
    );
    rtcRef.current = rtc;
    if (callData.offer) {
      await rtc.handleOffer(callData.offer);
    }
  };

  const handleDeclineCall = () => {
    if (!incomingCall || !wsRef.current) return;
    wsRef.current.send('call_end', '', {}, incomingCall.session_id);
    setIncomingCall(null);
  };

  const handleStartCallToGuest = async () => {
    if (!selectedCase || !wsRef.current) return;
    setIsCallActive(true);
    setCallDuration(0);
    clearInterval(callTimerRef.current);

    const rtc = new WebRTCManager(
      wsRef.current,
      selectedCase.session_id,
      (state: any) => {
        if (state === 'connected') {
          startCallTimer();
        } else if (state === 'ended') {
          setIsCallActive(false);
          setIncomingCall(null);
          clearInterval(callTimerRef.current);
          setCallDuration(0);
        }
      },
      (stream: any) => {
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
    loadVoiceCalls(false);
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

  const handleDeleteVoiceCall = async (callID: number) => {
    if (!confirm('Bạn có chắc chắn muốn xóa bản ghi lịch sử cuộc gọi này khỏi hệ thống?')) return;
    try {
      await api.deleteVoiceCall(callID);
      setVoiceCalls((prev) => prev.filter((c) => c.id !== callID));
    } catch (err: any) {
      alert(err.message || 'Lỗi xóa cuộc gọi');
    }
  };

  // If not authenticated, render Login Form
  if (!token) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4 bg-[#0A0F1D] relative overflow-hidden">
        <div className="absolute -top-40 -left-40 w-96 h-96 bg-[#1C2D56]/40 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute -bottom-40 -right-40 w-96 h-96 bg-[#95252E]/30 rounded-full blur-3xl pointer-events-none" />

        <div className="w-full max-w-md glass-panel-brand p-8 rounded-2xl relative z-10 shadow-2xl">
          <div className="text-center mb-8">
            <div className="inline-flex items-center justify-center p-3 rounded-2xl bg-[#1C2D56] border border-[#95252E]/40 mb-4 shadow-lg">
              <img src="/logo/Logo Dọc_Trắng.svg" alt="Đông Đô Partners" className="h-14 w-auto object-contain" onError={(e) => { e.currentTarget.style.display = 'none'; }} />
            </div>
            <h1 className="text-2xl font-bold text-white tracking-tight">Đông Đô CS Studio</h1>
            <p className="text-sm text-slate-400 mt-1">Cổng Quản Trị Chăm Sóc Khách Hàng &amp; Dạy AI</p>
          </div>

          <form onSubmit={handleLogin} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                Tên đăng nhập CSKH / Admin
              </label>
              <input
                type="text"
                required
                value={loginUsername}
                onChange={(e) => setLoginUsername(e.target.value)}
                placeholder="cskh01..05 hoặc admin"
                className="w-full px-4 py-3 rounded-xl glass-input text-sm"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                Mật khẩu
              </label>
              <input
                type="password"
                required
                value={loginPassword}
                onChange={(e) => setLoginPassword(e.target.value)}
                placeholder="Nhập mật khẩu"
                className="w-full px-4 py-3 rounded-xl glass-input text-sm"
              />
            </div>

            {loginError && (
              <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center space-x-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{loginError}</span>
              </div>
            )}

            <button
              type="submit"
              disabled={isLoggingIn}
              className="w-full py-3.5 px-4 rounded-xl btn-brand-primary flex items-center justify-center space-x-2 text-sm font-semibold mt-6 cursor-pointer"
            >
              <span>{isLoggingIn ? 'Đang xác thực...' : 'Đăng Nhập Studio'}</span>
            </button>
          </form>

          <div className="mt-8 text-center text-xs text-slate-500 border-t border-slate-700/50 pt-4">
            🔒 Dành riêng cho Chuyên viên CSKH &amp; Quản trị viên Đông Đô Partners
          </div>
        </div>
      </div>
    );
  }

  const pendingCount = pendingLearning.length;
  const waitingCasesCount = cases.filter((c) => c.status === 'NEEDS_HUMAN_CS').length;

  return (
    <div className="min-h-screen bg-[#0A0F1D] text-slate-100 flex flex-col md:flex-row overflow-hidden">
      <audio ref={remoteAudioRef} autoPlay />

      {/* Incoming Call Notification Banner */}
      {incomingCall && (
        <div className="fixed top-6 right-6 z-50 p-4 rounded-2xl glass-panel-brand border-2 border-rose-500 shadow-2xl flex items-center space-x-4 animate-bounce">
          <div className="w-12 h-12 rounded-full bg-rose-600 flex items-center justify-center text-white animate-pulse">
            <Phone className="w-6 h-6" />
          </div>
          <div>
            <div className="text-xs font-semibold text-rose-400 uppercase tracking-wider">Cuộc gọi thoại đến!</div>
            <div className="text-sm font-bold text-white">{incomingCall.caller_id} đang gọi...</div>
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

      {/* Toast Error Alert Banner */}
      {toastError && (
        <div className="fixed top-6 right-6 z-[9999] p-4 rounded-2xl bg-[#180A12] border-2 border-rose-500 shadow-2xl text-white max-w-md animate-bounce flex items-start space-x-3 backdrop-blur-md">
          <div className="p-2 rounded-xl bg-rose-600 text-white shrink-0 mt-0.5 animate-pulse">
            <ShieldAlert className="w-5 h-5" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-xs font-bold text-rose-400 uppercase tracking-wider">⚠️ Phát sinh lỗi [{toastError.source}]</div>
            <div className="text-xs font-semibold text-white mt-0.5 truncate">{toastError.title}</div>
            <div className="text-[11px] text-rose-200/80 mt-1 line-clamp-2">{toastError.details}</div>
            <button
              onClick={() => setShowErrorCenterModal(true)}
              className="mt-2 text-[11px] font-bold text-rose-300 underline hover:text-white flex items-center space-x-1"
            >
              <span>👉 Click xem chi tiết &amp; Đề xuất hướng xử lý</span>
            </button>
          </div>
          <button onClick={() => setToastError(null)} className="text-slate-400 hover:text-white p-1">
            <XCircle className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Active Call Floating Bar */}
      {isCallActive && (
        <div className="fixed bottom-6 right-6 z-50 p-4 rounded-2xl glass-panel-brand border border-emerald-500 shadow-2xl flex items-center space-x-4">
          <div className="w-10 h-10 rounded-full bg-emerald-600/30 border border-emerald-500 flex items-center justify-center text-emerald-400 animate-pulse">
            <Phone className="w-5 h-5" />
          </div>
          <div>
            <div className="text-xs text-slate-400">Đang đàm thoại thoại WebRTC</div>
            <div className="text-sm font-bold text-emerald-400">{formatCallTime(callDuration)}</div>
          </div>
          <button
            onClick={() => {
              if (rtcRef.current) {
                const muted = rtcRef.current.toggleMute();
                setIsMuted(muted);
              }
            }}
            className="p-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300"
          >
            {isMuted ? <MicOff className="w-4 h-4 text-rose-400" /> : <Mic className="w-4 h-4" />}
          </button>
          <button
            onClick={() => handleEndCall()}
            className="p-2.5 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold flex items-center space-x-1"
          >
            <PhoneOff className="w-4 h-4" />
            <span>Kết thúc</span>
          </button>
        </div>
      )}

      {/* Resolve Case Modal (Extract & Review Q&A like Original) */}
      {showResolveModal && selectedCase && (
        <div className="fixed inset-0 z-50 bg-black/80 flex items-center justify-center p-4 overflow-y-auto">
          <div className="w-full max-w-2xl glass-panel-brand p-6 rounded-2xl shadow-2xl border border-slate-700/60 my-8">
            <div className="flex items-center justify-between mb-4 border-b border-slate-700/60 pb-3">
              <div>
                <h3 className="text-lg font-bold text-white flex items-center space-x-2">
                  <span>🎯 Đóng &amp; Giải Quyết Ca Hỗ Trợ</span>
                </h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  Mã phiên: <code className="text-sky-300 font-mono">{selectedCase.session_id}</code> | Khách: <span className="text-white font-medium">{selectedCase.customer_name}</span>
                </p>
              </div>
              <button
                onClick={() => setShowResolveModal(false)}
                className="p-1 rounded-lg text-slate-400 hover:text-white"
              >
                <XCircle className="w-5 h-5" />
              </button>
            </div>

            {/* Enable Learning Checkbox */}
            <div className="bg-slate-800/60 p-3.5 rounded-xl border border-slate-700 mb-4 flex items-center justify-between">
              <label className="flex items-center space-x-2.5 cursor-pointer text-xs font-semibold text-white">
                <input
                  type="checkbox"
                  checked={modalEnableLearn}
                  onChange={(e) => setModalEnableLearn(e.target.checked)}
                  className="w-4 h-4 rounded text-[#95252E] focus:ring-0 bg-slate-900 border-slate-700 cursor-pointer"
                />
                <span>🧠 Trích xuất &amp; Dạy AI các cặp Q&amp;A này khi đóng case</span>
              </label>
              <span className="text-[11px] text-amber-400 bg-amber-500/10 px-2 py-0.5 rounded border border-amber-500/20">
                {autoLearnEnabled ? '🟢 Tự động nạp vào Qdrant' : '⚪ Đưa vào hàng chờ duyệt'}
              </span>
            </div>

            {/* Extracted Q&A Pairs List */}
            {modalEnableLearn && (
              <div className="space-y-4 mb-4 max-h-[45vh] overflow-y-auto pr-1">
                <div className="flex items-center justify-between text-xs text-slate-300">
                  <span className="font-semibold">Danh sách cặp Q&amp;A được trích xuất ({modalQAPairs.length}):</span>
                  <button
                    type="button"
                    onClick={() => setModalQAPairs([...modalQAPairs, { question: '', answer: '' }])}
                    className="text-sky-400 hover:text-sky-300 font-medium flex items-center space-x-1"
                  >
                    <span>➕ Thêm cặp Q&amp;A</span>
                  </button>
                </div>

                {modalQAPairs.map((pair, idx) => (
                  <div key={idx} className="bg-[#0A0F1D] p-4 rounded-xl border border-slate-700/70 space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="text-[11px] font-bold text-amber-400 bg-amber-500/10 px-2 py-0.5 rounded border border-amber-500/20">
                        Cặp Q&amp;A #{idx + 1}
                      </span>
                      {modalQAPairs.length > 1 && (
                        <button
                          type="button"
                          onClick={() => setModalQAPairs(modalQAPairs.filter((_, i) => i !== idx))}
                          className="text-xs text-rose-400 hover:text-rose-300"
                        >
                          ✕ Xóa
                        </button>
                      )}
                    </div>
                    <div>
                      <label className="block text-[11px] font-semibold text-slate-300 mb-1">
                        ❓ Câu hỏi của Khách hàng:
                      </label>
                      <input
                        type="text"
                        value={pair.question}
                        onChange={(e) => {
                          const updated = [...modalQAPairs];
                          updated[idx].question = e.target.value;
                          setModalQAPairs(updated);
                        }}
                        placeholder="Ví dụ: nạp tiền vào DDP Invest như thế nào?"
                        className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-700 text-xs text-white focus:outline-none focus:border-[#B32D38]"
                      />
                    </div>
                    <div>
                      <label className="block text-[11px] font-semibold text-slate-300 mb-1">
                        💡 Câu trả lời chuẩn của CSKH:
                      </label>
                      <textarea
                        rows={3}
                        value={pair.answer}
                        onChange={(e) => {
                          const updated = [...modalQAPairs];
                          updated[idx].answer = e.target.value;
                          setModalQAPairs(updated);
                        }}
                        placeholder="Nhập câu trả lời chuẩn xác..."
                        className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-700 text-xs text-white focus:outline-none focus:border-[#B32D38]"
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* Optional Resolution Note */}
            <div className="mb-4">
              <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                📝 Ghi chú giải quyết (Tùy chọn):
              </label>
              <textarea
                rows={2}
                value={resolveNote}
                onChange={(e) => setResolveNote(e.target.value)}
                placeholder="Ghi chú thêm về phiên hỗ trợ..."
                className="w-full p-2.5 rounded-xl bg-[#0A0F1D] border border-slate-700 text-xs text-white focus:outline-none focus:border-[#B32D38]"
              />
            </div>

            {/* Modal Actions */}
            <div className="flex items-center justify-end space-x-3 pt-2 border-t border-slate-700/60">
              <button
                type="button"
                onClick={() => setShowResolveModal(false)}
                className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold"
              >
                Hủy
              </button>
              <button
                type="button"
                onClick={handleResolveCase}
                className="px-5 py-2 rounded-xl btn-brand-primary text-xs font-semibold flex items-center space-x-1.5"
              >
                <span>{modalEnableLearn ? 'Hoàn Tất & Dạy AI 🚀' : 'Đóng Case'}</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Edit Customer Modal */}
      {showEditCustomerModal && selectedCase && (
        <div className="fixed inset-0 z-50 bg-black/75 flex items-center justify-center p-4">
          <div className="w-full max-w-md glass-panel-brand p-6 rounded-2xl shadow-2xl border border-slate-700/60 animate-in fade-in zoom-in-95 duration-150">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-base font-bold text-white flex items-center space-x-2">
                <span>✏️ Chỉnh Sửa Thông Tin Khách Hàng</span>
              </h3>
              <button
                onClick={() => setShowEditCustomerModal(false)}
                className="p-1 rounded-lg text-slate-400 hover:text-white"
              >
                <XCircle className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleSaveCustomerInfo} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                  👤 Họ và Tên Khách Hàng:
                </label>
                <input
                  type="text"
                  value={editCustomerName}
                  onChange={(e) => setEditCustomerName(e.target.value)}
                  placeholder="Ví dụ: Anh Nam, Bác Hải, Chị Linh..."
                  required
                  className="w-full px-3.5 py-2.5 rounded-xl bg-[#0A0F1D] border border-slate-700 text-sm text-white focus:outline-none focus:border-[#B32D38]"
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                  📱 Số Điện Thoại / Zalo:
                </label>
                <input
                  type="text"
                  value={editCustomerPhone}
                  onChange={(e) => setEditCustomerPhone(e.target.value)}
                  placeholder="Ví dụ: 0988123456"
                  className="w-full px-3.5 py-2.5 rounded-xl bg-[#0A0F1D] border border-slate-700 text-sm text-white focus:outline-none focus:border-[#B32D38]"
                />
              </div>
              <div className="text-[11px] text-slate-400 bg-slate-800/50 p-3 rounded-xl border border-slate-700/50">
                <span>Mã phiên: </span>
                <code className="text-sky-300 font-mono">{selectedCase.session_id}</code>
              </div>
              <div className="flex items-center justify-end space-x-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowEditCustomerModal(false)}
                  className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold"
                >
                  Hủy
                </button>
                <button
                  type="submit"
                  disabled={isUpdatingCustomer}
                  className="px-4 py-2 rounded-xl btn-brand-primary text-xs font-semibold flex items-center space-x-1.5 shadow-lg"
                >
                  {isUpdatingCustomer ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Check className="w-3.5 h-3.5" />}
                  <span>Lưu Thay Đổi</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Voice Call History Modal */}
      {showVoiceHistoryModal && (
        <div className="fixed inset-0 z-50 bg-black/75 flex items-center justify-center p-4">
          <div className="w-full max-w-2xl glass-panel-brand p-6 rounded-2xl shadow-2xl border border-slate-700/60 max-h-[85vh] flex flex-col animate-in fade-in zoom-in-95 duration-150">
            <div className="flex items-center justify-between pb-4 border-b border-[#1C2D56] shrink-0">
              <div className="flex items-center space-x-3">
                <div className="w-10 h-10 rounded-xl bg-emerald-600/20 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
                  <Headphones className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="text-base font-bold text-white">🎧 Lịch Sử Cuộc Gọi Thoại</h3>
                  <p className="text-xs text-slate-400">Danh sách các cuộc đàm thoại giữa Khách hàng và CSKH</p>
                </div>
              </div>
              <div className="flex items-center space-x-2">
                <button
                  onClick={() => handleOpenVoiceHistory()}
                  className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold flex items-center space-x-1.5"
                  title="Tải lại toàn bộ lịch sử cuộc gọi"
                >
                  <RefreshCw className="w-3.5 h-3.5" />
                  <span>Tất cả</span>
                </button>
                <button
                  onClick={() => setShowVoiceHistoryModal(false)}
                  className="p-1 rounded-lg text-slate-400 hover:text-white"
                >
                  <XCircle className="w-5 h-5" />
                </button>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto py-4 space-y-3">
              {isLoadingVoiceCalls ? (
                <div className="py-12 flex flex-col items-center justify-center text-slate-400 text-xs space-y-2">
                  <RefreshCw className="w-6 h-6 animate-spin text-emerald-400" />
                  <span>Đang tải lịch sử cuộc gọi...</span>
                </div>
              ) : voiceCalls.length === 0 ? (
                <div className="py-12 text-center text-slate-500 text-xs">
                  Chưa có dữ liệu cuộc gọi thoại nào được ghi nhận.
                </div>
              ) : (
                voiceCalls.map((call) => (
                  <div
                    key={call.id}
                    className="p-4 rounded-xl bg-[#0A0F1D]/80 border border-slate-800 hover:border-slate-700 space-y-2.5 transition"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center space-x-2">
                        <span className="text-xs font-bold text-emerald-400">
                          {call.caller_type === 'guest' ? '👤 Khách hàng' : '🎧 CSKH'} ({call.caller_id})
                        </span>
                        <span className="text-slate-500 text-xs">➡️</span>
                        <span className="text-xs font-bold text-slate-200">
                          {call.callee_type === 'cskh' || call.callee_type === 'human_cs' ? '🎧 Chuyên viên CSKH' : '👤 Khách hàng'}
                        </span>
                      </div>
                      <span className="text-[11px] text-slate-400">
                        {new Date(call.created_at).toLocaleString('vi-VN')}
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-xs text-slate-400 pt-1">
                      <div className="flex items-center space-x-3">
                        <span>⏱️ Thời lượng: <strong className="text-white">{call.duration_seconds > 0 ? `${call.duration_seconds}s` : 'Chưa ghi nhận'}</strong></span>
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                          {call.status}
                        </span>
                      </div>
                      <span className="text-[11px] text-slate-500 font-mono">
                        {call.session_id}
                      </span>
                    </div>

                    {call.recording_url && (
                      <div className="pt-2">
                        <audio controls preload="metadata" className="w-full h-8 rounded-lg">
                          <source src={call.recording_url} type="audio/webm" />
                          <source src={call.recording_url.startsWith('http') ? call.recording_url : `http://localhost:8080${call.recording_url}`} type="audio/webm" />
                          Trình duyệt không hỗ trợ phát âm thanh.
                        </audio>
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}

      {/* ============================================================ */}
      {/* 1. SIDEBAR (BỐ CỤC CHUẨN ĐÔNG ĐÔ STUDIO) */}
      {/* ============================================================ */}
      <aside className="w-full md:w-72 bg-[#0D1527] border-r border-[#1C2D56] flex flex-col justify-between shrink-0">
        <div>
          {/* Brand Header */}
          <div className="p-5 border-b border-[#1C2D56] flex items-center space-x-3">
            <img
              src="/logo/Logo Dọc_Trắng.svg"
              alt="Logo"
              className="h-10 w-auto object-contain"
              onError={(e) => { e.currentTarget.style.display = 'none'; }}
            />
            <div>
              <h2 className="text-base font-bold text-white leading-tight">Đông Đô CS</h2>
              <span className="text-[10px] font-semibold text-rose-400 tracking-wider bg-rose-500/10 px-2 py-0.5 rounded border border-rose-500/20">
                STUDIO V2.0 (GO)
              </span>
            </div>
          </div>

          {/* Nav Menu */}
          <nav className="p-3 space-y-1">
            {getFeaturePermission('partner_dashboard') !== 'none' && (
              <button
                onClick={() => setActiveTab('partner_dashboard')}
                className={`w-full flex items-center space-x-3 px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'partner_dashboard'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-[#162344]'
                }`}
              >
                <LayoutDashboard className="w-4 h-4 text-purple-400" />
                <span>Trang Chủ / Dashboard</span>
              </button>
            )}

            {getFeaturePermission('inbox') !== 'none' && (
              <button
                onClick={() => setActiveTab('inbox')}
                className={`w-full flex items-center justify-between px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'inbox'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-[#162344]'
                }`}
              >
                <div className="flex items-center space-x-3">
                  <Inbox className="w-4 h-4 text-sky-400" />
                  <span>Live CS Inbox</span>
                </div>
                {waitingCasesCount > 0 && (
                  <span className="px-2 py-0.5 rounded-full text-xs font-bold bg-rose-500 text-white animate-pulse">
                    {waitingCasesCount}
                  </span>
                )}
              </button>
            )}

            {getFeaturePermission('customers') !== 'none' && (
              <button
                onClick={() => {
                  setActiveTab('customers');
                  loadCustomers();
                }}
                className={`w-full flex items-center justify-between px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'customers'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-[#162344]'
                }`}
              >
                <div className="flex items-center space-x-3">
                  <Users className="w-4 h-4 text-emerald-400" />
                  <span>Quản Lý Khách Hàng</span>
                </div>
                {customers.length > 0 && (
                  <span className="px-2 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                    {customers.length}
                  </span>
                )}
              </button>
            )}

            {getFeaturePermission('calls') !== 'none' && (
              <button
                onClick={() => {
                  setActiveTab('calls');
                  loadVoiceCalls();
                }}
                className={`w-full flex items-center justify-between px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'calls'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-[#162344]'
                }`}
              >
                <div className="flex items-center space-x-3">
                  <Headphones className="w-4 h-4 text-cyan-400" />
                  <span>Lịch Sử Cuộc Gọi</span>
                </div>
                {voiceCalls.length > 0 && (
                  <span className="px-2 py-0.5 rounded-full text-xs font-semibold bg-cyan-500/20 text-cyan-300 border border-cyan-500/30">
                    {voiceCalls.length}
                  </span>
                )}
              </button>
            )}

            {getFeaturePermission('learning') !== 'none' && (
              <button
                onClick={() => setActiveTab('learning')}
                className={`w-full flex items-center justify-between px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'learning'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-[#162344]'
                }`}
              >
                <div className="flex items-center space-x-3">
                  <Brain className="w-4 h-4 text-amber-400" />
                  <span>Học Tri Thức Mới</span>
                </div>
                {pendingCount > 0 && (
                  <span className="px-2 py-0.5 rounded-full text-xs font-bold bg-amber-500/20 text-amber-300 border border-amber-500/30">
                    {pendingCount}
                  </span>
                )}
              </button>
            )}

            {getFeaturePermission('knowledge') !== 'none' && (
              <button
                onClick={() => setActiveTab('knowledge')}
                className={`w-full flex items-center space-x-3 px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'knowledge'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-[#162344]'
                }`}
              >
                <BookOpen className="w-4 h-4 text-indigo-400" />
                <span>Kho Tri Thức</span>
              </button>
            )}

            {getFeaturePermission('partner_analytics') !== 'none' && (
              <button
                onClick={() => setActiveTab('partner_analytics')}
                className={`w-full flex items-center space-x-3 px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'partner_analytics'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-[#162344]'
                }`}
              >
                <TrendingUp className="w-4 h-4 text-emerald-400" />
                <span>Báo Cáo &amp; Thống Kê CX</span>
              </button>
            )}

            {getFeaturePermission('partner_config') !== 'none' && (
              <button
                onClick={() => setActiveTab('partner_config')}
                className={`w-full flex items-center space-x-3 px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'partner_config'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-[#162344]'
                }`}
              >
                <SlidersHorizontal className="w-4 h-4 text-sky-400" />
                <span>Cấu Hình &amp; Phân Quyền</span>
              </button>
            )}

            {getFeaturePermission('config') !== 'none' && (
              <button
                onClick={() => setActiveTab('config')}
                className={`w-full flex items-center space-x-3 px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'config'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-[#162344]'
                }`}
              >
                <Settings className="w-4 h-4 text-slate-400" />
                <span>Cấu Hình LLM Studio</span>
              </button>
            )}

            {(currentUser?.role || '').toLowerCase() === 'owner' && (
              <button
                onClick={() => setActiveTab('update_data_test')}
                className={`w-full flex items-center space-x-3 px-3.5 py-3 rounded-xl text-sm font-medium transition-all ${
                  activeTab === 'update_data_test'
                    ? 'bg-[#1C2D56] text-white border-l-4 border-[#B32D38] shadow-lg'
                    : 'text-purple-400 hover:text-purple-200 hover:bg-[#162344]'
                }`}
              >
                <TestTube className="w-4 h-4 text-purple-400" />
                <span>Update Data Test (Owner)</span>
              </button>
            )}
          </nav>
        </div>

        {/* Sidebar Footer */}
        <div className="p-4 border-t border-[#1C2D56] space-y-3">
          <div className="flex items-center space-x-3 p-2.5 rounded-xl bg-[#162344]/50 border border-slate-700/40">
            <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse" />
            <div className="text-xs">
              <div className="font-semibold text-slate-200">System Active</div>
              <div className="text-[11px] text-slate-400">RAG + Claude 3.5 Haiku</div>
            </div>
          </div>

          <a
            href="/"
            target="_blank"
            rel="noreferrer"
            className="w-full py-2.5 px-3 rounded-xl bg-slate-800/80 hover:bg-slate-700 text-xs font-semibold text-sky-400 flex items-center justify-center space-x-1.5 transition"
          >
            <span>💬 Mở Khung Chat Khách</span>
            <ExternalLink className="w-3.5 h-3.5" />
          </a>
        </div>
      </aside>

      {/* ============================================================ */}
      {/* 2. MAIN CONTENT AREA */}
      {/* ============================================================ */}
      <main className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Top Header */}
        <header className="h-16 px-6 bg-[#0D1527] border-b border-[#1C2D56] flex items-center justify-between shrink-0">
          <div>
            <h1 className="text-base font-bold text-white leading-tight">
              {activeTab === 'inbox' && 'Live CS Inbox - Tiếp Nhận & Hỗ Trợ Khách Hàng'}
              {activeTab === 'customers' && '👤 Quản Lý Khách Hàng & Sửa Thông Tin SĐT'}
              {activeTab === 'calls' && '🎧 Lịch Sử Cuộc Gọi Thoại WebRTC'}
              {activeTab === 'learning' && '🧠 Hàng Đợi & Cơ Chế Tự Động Học Tri Thức Cho AI'}
              {activeTab === 'knowledge' && '📚 Kho Tri Thức & Quản Trị Vector Database'}
              {activeTab === 'analytics' && '📊 Báo Cáo & Thống Kê Hiệu Năng Phục Vụ'}
              {activeTab === 'config' && '⚙️ Cấu Hình Hệ Thống & Tham Số LLM'}
              {activeTab === 'partner_dashboard' && '📊 Trang Chủ / Dashboard - Tổng Quan Hệ Thống (Mới)'}
              {activeTab === 'partner_analytics' && '📈 Báo Cáo & Thống Kê - Phân Tích Hiệu Năng Phục Vụ (Mới)'}
              {activeTab === 'partner_config' && '⚙️ Cấu Hình Hệ Thống AI Engine & Phân Quyền (Mới)'}
              {activeTab === 'update_data_test' && '🧪 Update Data Test (Engine Dữ Liệu Test Báo Cáo — Đặc Quyền Owner)'}
            </h1>
            <p className="text-xs text-slate-400 hidden sm:block">
              {activeTab === 'inbox' && 'Quản lý các case cần sự can thiệp của Chuyên viên CSKH thật'}
              {activeTab === 'customers' && 'Xem danh sách khách hàng, chỉnh sửa Tên, Số điện thoại / Zalo và xóa thông tin'}
              {activeTab === 'calls' && 'Theo dõi toàn bộ cuộc đàm thoại giữa Khách và CSKH, nghe lại file ghi âm'}
              {activeTab === 'learning' && 'Phê duyệt các cặp Q&A được trích xuất từ các ca CSKH'}
              {activeTab === 'knowledge' && 'Tài liệu nguồn .docx và 60 chunks embedding trong Qdrant'}
              {activeTab === 'analytics' && 'Tỷ lệ AI tự phục vụ và phân bổ trạng thái hỗ trợ'}
              {activeTab === 'config' && 'Điều chỉnh System Prompt, Claude model và nhiệt độ'}
              {activeTab === 'partner_dashboard' && 'Thống kê hiệu suất CSKH, tỷ lệ tự động hóa AI và hoạt động gần đây'}
              {activeTab === 'partner_analytics' && 'Phân tích chất lượng hỗ trợ, 7 sub-report chuẩn hệ thống'}
              {activeTab === 'partner_config' && 'Tùy chỉnh System Prompt, tài khoản, phân quyền RBAC và tin nhắn mẫu'}
            </p>
          </div>

          <div className="flex items-center space-x-3">
            <button
              onClick={() => handleOpenVoiceHistory()}
              className="p-2 px-3 rounded-xl bg-emerald-600/20 hover:bg-emerald-600/30 border border-emerald-500/30 text-emerald-400 text-xs flex items-center space-x-1.5 transition font-semibold"
              title="Xem và nghe lại lịch sử cuộc gọi thoại"
            >
              <Headphones className="w-3.5 h-3.5" />
              <span>Lịch Sử Gọi</span>
            </button>

            <button
              onClick={loadAllData}
              className="p-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs flex items-center space-x-1.5 transition"
              title="Làm mới dữ liệu"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Làm mới</span>
            </button>

            {/* System Error Center Warning Icon */}
            <button
              onClick={() => setShowErrorCenterModal(true)}
              className={`relative p-2 px-3 rounded-xl border text-xs flex items-center space-x-1.5 transition font-semibold ${
                systemErrors.filter((e) => !e.isHandled).length > 0
                  ? 'bg-rose-500/20 hover:bg-rose-500/30 border-rose-500/50 text-rose-300 shadow-lg shadow-rose-950/40'
                  : 'bg-slate-800 hover:bg-slate-700 border-slate-700 text-slate-300'
              }`}
              title="Trung tâm cảnh báo & xử lý lỗi hệ thống"
            >
              <AlertTriangle className={`w-3.5 h-3.5 ${systemErrors.filter((e) => !e.isHandled).length > 0 ? 'text-rose-400 animate-pulse' : 'text-slate-400'}`} />
              <span className="hidden sm:inline">Cảnh Báo Lỗi</span>
              {systemErrors.filter((e) => !e.isHandled).length > 0 && (
                <span className="px-1.5 py-0.5 rounded-full text-[10px] font-black bg-rose-600 text-white animate-bounce">
                  {systemErrors.filter((e) => !e.isHandled).length}
                </span>
              )}
            </button>

            <div className="flex items-center space-x-2 px-3 py-1.5 rounded-xl bg-[#162344] border border-[#1C2D56]">
              <span className="text-sm">👨‍💼</span>
              <span className="text-xs font-medium text-slate-200">{currentUser?.full_name || currentUser?.username || 'CSKH'}</span>
            </div>

            <button
              onClick={handleLogout}
              className="p-2 rounded-xl bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 text-xs transition"
              title="Đăng xuất"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </header>

        {/* Tab Body */}
        <div className="flex-1 p-6 overflow-y-auto">
          {/* ============================================================ */}
          {/* TAB 1: LIVE CS INBOX (GRID 2 CỘT CHUẨN BỐ CỤC CŨ) */}
          {/* ============================================================ */}
          {activeTab === 'inbox' && (
            <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 h-[calc(100vh-8rem)]">
              {/* Left Panel: Cases List */}
              <div className="lg:col-span-5 flex flex-col glass-panel-brand rounded-2xl overflow-hidden">
                <div className="p-4 border-b border-[#1C2D56] shrink-0">
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="text-sm font-bold text-white">Danh Sách Case Hỗ Trợ</h3>
                    <button
                      onClick={handleClearAllCases}
                      className="text-xs text-rose-400 hover:text-rose-300 flex items-center space-x-1 transition"
                      title="Xóa toàn bộ case test"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                      <span>Xóa tất cả</span>
                    </button>
                  </div>

                  <div className="flex items-center space-x-1.5 overflow-x-auto pb-1">
                    {[
                      { key: '', label: 'Tất cả' },
                      { key: 'NEEDS_HUMAN_CS', label: 'Chờ CSKH' },
                      { key: 'HUMAN_CS_ACTIVE', label: 'Đang CSKH' },
                      { key: 'RESOLVED', label: 'Đã Đóng' },
                    ].map((tab) => (
                      <button
                        key={tab.key}
                        onClick={() => {
                          setCaseFilter(tab.key);
                          setCasePage(1);
                          loadCases(1, casePageSize, tab.key);
                        }}
                        className={`px-3 py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap transition ${
                          caseFilter === tab.key
                            ? 'bg-[#95252E] text-white shadow'
                            : 'bg-slate-800/60 text-slate-400 hover:bg-slate-800'
                        }`}
                      >
                        {tab.label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="flex-1 overflow-y-auto p-3 space-y-2">
                  {cases.length === 0 ? (
                    <div className="text-center py-12 text-slate-500 text-xs">
                      Không có case nào phù hợp bộ lọc.
                    </div>
                  ) : (
                    cases.map((c) => {
                        const isSelected = selectedCase?.session_id === c.session_id;
                        return (
                          <div
                            key={c.session_id}
                            onClick={() => selectCase(c)}
                            className={`p-3.5 rounded-xl cursor-pointer border transition-all ${
                              isSelected
                                ? 'bg-[#1C2D56] border-[#B32D38] shadow-md'
                                : 'bg-[#162344]/40 border-slate-800/60 hover:bg-[#162344] hover:border-slate-700'
                            }`}
                          >
                            <div className="flex items-center justify-between mb-1.5">
                              <span className="text-sm font-bold text-white flex items-center space-x-1.5">
                                <span>👤</span>
                                <span>{c.customer_name || 'Khách hàng'}</span>
                              </span>
                              <span
                                className={`text-[10px] font-semibold px-2 py-0.5 rounded-md ${
                                  c.status === 'NEEDS_HUMAN_CS'
                                    ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                                    : c.status === 'HUMAN_CS_ACTIVE'
                                    ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                                    : c.status === 'RESOLVED'
                                    ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                                    : 'bg-sky-500/20 text-sky-300 border border-sky-500/30'
                                }`}
                              >
                                {c.status === 'NEEDS_HUMAN_CS'
                                  ? 'Chờ CSKH'
                                  : c.status === 'HUMAN_CS_ACTIVE'
                                  ? 'Đang CSKH'
                                  : c.status === 'RESOLVED'
                                  ? 'Đã giải quyết'
                                  : 'AI Tự phục vụ'}
                              </span>
                            </div>

                            {c.customer_phone && (
                              <div className="text-[11px] text-emerald-400 font-semibold mb-1.5 flex items-center space-x-1">
                                <span>📱</span>
                                <span>{c.customer_phone}</span>
                              </div>
                            )}

                            <p className="text-xs text-slate-300 line-clamp-2 mb-2">{c.last_message || 'Chưa có tin nhắn'}</p>

                            <div className="flex items-center justify-between text-[11px] text-slate-500">
                              <span>{new Date(c.updated_at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</span>
                              <div className="flex items-center space-x-2">
                                {c.assigned_cs && <span className="text-amber-400">CS: {c.assigned_cs}</span>}
                                <button
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    handleDeleteCase(c.session_id);
                                  }}
                                  className="p-1 rounded hover:bg-rose-500/20 text-slate-500 hover:text-rose-400 transition"
                                  title="Xóa case này"
                                >
                                  <Trash2 className="w-3 h-3" />
                                </button>
                              </div>
                            </div>
                          </div>
                        );
                      })
                  )}
                </div>

                {cases.length > 0 && (
                  <div className="px-3 pb-3 bg-[#0D1527]/50">
                    <Pagination
                      currentPage={casePage}
                      pageSize={casePageSize}
                      totalItems={caseTotal}
                      onPageChange={(page) => {
                        setCasePage(page);
                        loadCases(page, casePageSize, caseFilter);
                      }}
                      onPageSizeChange={(size) => {
                        setCasePageSize(size);
                        setCasePage(1);
                        loadCases(1, size, caseFilter);
                      }}
                      compact={true}
                    />
                  </div>
                )}
              </div>

              {/* Right Panel: Active Case Chat Detail */}
              <div className="lg:col-span-7 flex flex-col glass-panel-brand rounded-2xl overflow-hidden">
                {!selectedCase ? (
                  <div className="flex-1 flex flex-col items-center justify-center p-8 text-center">
                    <div className="w-16 h-16 rounded-2xl bg-[#1C2D56] border border-[#95252E]/30 flex items-center justify-center text-3xl mb-4">
                      💬
                    </div>
                    <h3 className="text-base font-bold text-white mb-1">
                      Chọn một case bên trái để xem hội thoại &amp; tiếp nhận hỗ trợ
                    </h3>
                    <p className="text-xs text-slate-400 max-w-sm">
                      Các câu hỏi khách hỏi mà AI chưa có dữ liệu sẽ tự động xuất hiện ở tab <strong>Chờ CSKH</strong>.
                    </p>
                  </div>
                ) : (
                  <>
                    {/* Detail Header */}
                    <div className="p-4 border-b border-[#1C2D56] flex items-center justify-between shrink-0 bg-[#0D1527]/70">
                      <div>
                        <div className="flex items-center space-x-2">
                          <span className="text-sm font-bold text-white">{selectedCase.customer_name}</span>
                          {selectedCase.customer_phone ? (
                            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                              📱 {selectedCase.customer_phone}
                            </span>
                          ) : (
                            <span className="text-[11px] text-slate-500 italic">Chưa để lại SĐT</span>
                          )}
                        </div>
                        <div className="text-[11px] text-slate-400 mt-0.5 flex items-center space-x-3">
                          <span>Mã phiên: <code className="text-slate-300">{selectedCase.session_id}</code></span>
                          <span className="text-amber-400">
                            {selectedCase.assigned_cs ? `• CS tiếp nhận: ${selectedCase.assigned_cs}` : '• Chưa có CS tiếp nhận'}
                          </span>
                        </div>
                      </div>

                      <div className="flex items-center space-x-2">
                        {selectedCase.status !== 'HUMAN_CS_ACTIVE' && (
                          <button
                            onClick={handleTakeCase}
                            className="px-3 py-2 rounded-xl btn-brand-primary text-xs font-semibold flex items-center space-x-1 shadow"
                          >
                            <UserCheck className="w-3.5 h-3.5" />
                            <span>Tiếp Nhận Case</span>
                          </button>
                        )}

                        <button
                          onClick={handleStartCallToGuest}
                          className="px-3 py-2 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold flex items-center space-x-1 transition shadow"
                          title="Gọi thoại trực tiếp tới khách qua WebRTC"
                        >
                          <Phone className="w-3.5 h-3.5" />
                          <span>Gọi Khách</span>
                        </button>

                        <button
                          onClick={openResolveModal}
                          className="px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-emerald-400 text-xs font-semibold flex items-center space-x-1 transition cursor-pointer"
                        >
                          <CheckCircle2 className="w-3.5 h-3.5" />
                          <span>Đóng Case</span>
                        </button>

                        <button
                          onClick={() => handleDeleteCase(selectedCase.session_id)}
                          className="p-2 rounded-xl bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 text-xs font-semibold flex items-center transition"
                          title="Xóa ca hỗ trợ này khỏi hệ thống"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>

                    {/* Messages Scroll Area */}
                    <div ref={chatContainerRef} className="flex-1 p-4 overflow-y-auto space-y-3 bg-[#0A0F1D]/40">
                      {caseMessages.map((m, idx) => {
                        const isGuest = m.sender_type === 'guest';
                        const isCS = m.sender_type === 'cs' || m.sender_type === 'human_cs';
                        return (
                          <div
                            key={m.id || idx}
                            className={`flex flex-col ${isGuest ? 'items-start' : 'items-end'}`}
                          >
                            <div className="text-[10px] text-slate-500 mb-1 px-1">
                              {isGuest ? `👤 ${selectedCase.customer_name}` : isCS ? `👨‍💼 ${m.sender_id || 'CSKH'}` : '🤖 Trợ lý AI Đông Đô'}
                              {' • '}
                              {new Date(m.created_at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}
                            </div>
                            <div
                              className={`max-w-[85%] p-3.5 rounded-2xl text-sm leading-relaxed ${
                                isGuest
                                  ? 'bg-[#162344] text-slate-200 border border-slate-700/60 rounded-tl-sm'
                                  : isCS
                                  ? 'bg-[#95252E] text-white shadow-md rounded-tr-sm'
                                  : 'bg-[#1C2D56] text-slate-100 border border-[#2A3F74] rounded-tr-sm'
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
                      })}
                      <div ref={messagesEndRef} />
                    </div>

                    {/* CS Reply Box */}
                    <form onSubmit={handleSendCSMessage} className="p-3 border-t border-[#1C2D56] bg-[#0D1527]/90 shrink-0">
                      <div className="flex space-x-2">
                        <textarea
                          rows={2}
                          value={replyText}
                          onChange={(e) => setReplyText(e.target.value)}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter' && !e.shiftKey) {
                              e.preventDefault();
                              handleSendCSMessage(e);
                            }
                          }}
                          placeholder="Nhập tin nhắn phản hồi trực tiếp tới khách hàng... (Ấn Enter để gửi)"
                          className="flex-1 p-3 rounded-xl glass-input text-sm resize-none"
                        />
                        <button
                          type="submit"
                          disabled={!replyText.trim() || isSendingReply}
                          className="px-5 rounded-xl btn-brand-primary flex items-center justify-center font-semibold text-sm cursor-pointer disabled:opacity-50"
                        >
                          <Send className="w-4 h-4" />
                        </button>
                      </div>
                      <div className="text-[11px] text-slate-500 mt-2 flex items-center space-x-1">
                        <span>💡</span>
                        <span>Tin nhắn của bạn sẽ hiển thị thời gian thực trên màn hình Chat của Khách</span>
                      </div>
                    </form>
                  </>
                )}
              </div>
            </div>
          )}

          {/* ============================================================ */}
          {/* TAB: QUẢN LÝ KHÁCH HÀNG & CHỈNH SỬA THÔNG TIN */}
          {/* ============================================================ */}
          {activeTab === 'customers' && (
            <div className="max-w-6xl mx-auto space-y-6">
              {/* Header Box */}
              <div className="glass-panel-brand p-6 rounded-2xl flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div>
                  <h2 className="text-base font-bold text-white mb-1 flex items-center space-x-2">
                    <Users className="w-5 h-5 text-emerald-400" />
                    <span>Quản Lý Thông Tin Khách Hàng</span>
                  </h2>
                  <p className="text-xs text-slate-400">
                    Xem toàn bộ danh sách khách hàng, chỉnh sửa Họ tên, Số điện thoại / Zalo hoặc xóa dữ liệu ca hỗ trợ.
                  </p>
                </div>

                <div className="flex items-center space-x-3 shrink-0">
                  <div className="relative">
                    <Search className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
                    <input
                      type="text"
                      placeholder="Tìm theo Tên, SĐT hoặc Mã phiên..."
                      value={customerSearch}
                      onChange={(e) => {
                        const val = e.target.value;
                        setCustomerSearch(val);
                        setCustomerPage(1);
                        loadCustomers(false, 1, customerPageSize, val);
                      }}
                      className="pl-9 pr-4 py-2 rounded-xl bg-slate-900/90 border border-slate-700 text-xs text-white placeholder:text-slate-500 focus:outline-none focus:border-[#B32D38] w-72"
                    />
                  </div>
                </div>
              </div>

              {/* Customers Table */}
              <div className="glass-panel-brand rounded-2xl overflow-hidden shadow-xl border border-[#1C2D56]">
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs text-slate-300">
                    <thead className="bg-[#1C2D56]/80 text-[11px] uppercase tracking-wider text-slate-400 border-b border-[#2A3F74]">
                      <tr>
                        <th className="py-3.5 px-4 font-semibold">Khách Hàng</th>
                        <th className="py-3.5 px-4 font-semibold">Số Điện Thoại / Zalo</th>
                        <th className="py-3.5 px-4 font-semibold">Mã Định Danh (Guest ID)</th>
                        <th className="py-3.5 px-4 font-semibold">Phiên Chat Gần Nhất</th>
                        <th className="py-3.5 px-4 font-semibold">Ngày Tạo</th>
                        <th className="py-3.5 px-4 font-semibold text-right">Thao Tác</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800/60">
                      {isLoadingCustomers ? (
                        <tr>
                          <td colSpan={6} className="text-center py-12 text-slate-400 text-xs">
                            <RefreshCw className="w-6 h-6 animate-spin mx-auto text-emerald-400 mb-2" />
                            Đang tải danh sách khách hàng...
                          </td>
                        </tr>
                      ) : customers.length === 0 ? (
                        <tr>
                          <td colSpan={6} className="text-center py-12 text-slate-500">
                            Chưa có dữ liệu khách hàng nào trong hệ thống.
                          </td>
                        </tr>
                      ) : (
                        customers.map((cust) => (
                            <tr key={cust.guest_id} className="hover:bg-[#162344]/50 transition">
                              <td className="py-3.5 px-4">
                                <div className="flex items-center space-x-3">
                                  <div className="w-9 h-9 rounded-xl bg-emerald-600/20 border border-emerald-500/30 flex items-center justify-center text-sm font-bold text-emerald-400 shrink-0">
                                    👤
                                  </div>
                                  <div>
                                    <div className="font-bold text-white text-sm">
                                      {cust.display_name || 'Khách hàng'}
                                    </div>
                                    <div className="text-[11px] text-slate-400 line-clamp-1 max-w-xs">
                                      {cust.last_message || 'Khách hàng đăng ký tư vấn'}
                                    </div>
                                  </div>
                                </div>
                              </td>
                              <td className="py-3.5 px-4">
                                {cust.phone ? (
                                  <span className="inline-flex items-center px-2.5 py-1 rounded-lg text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                                    📱 {cust.phone}
                                  </span>
                                ) : (
                                  <span className="text-slate-500 italic">Chưa để lại SĐT</span>
                                )}
                              </td>
                              <td className="py-3.5 px-4 font-mono text-[11px] text-slate-400">
                                {cust.guest_id}
                              </td>
                              <td className="py-3.5 px-4">
                                {cust.last_session_id ? (
                                  <span className="font-mono text-[11px] text-sky-300">
                                    {cust.last_session_id}
                                  </span>
                                ) : (
                                  <span className="text-slate-500 italic text-[11px]">Chưa có phiên chat</span>
                                )}
                              </td>
                              <td className="py-3.5 px-4 text-slate-400">
                                {new Date(cust.created_at).toLocaleString('vi-VN')}
                              </td>
                              <td className="py-3.5 px-4 text-right">
                                <div className="flex items-center justify-end space-x-2">
                                  <button
                                    onClick={() => handleOpenEditCustomerModal(cust)}
                                    className="p-1.5 px-2.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-sky-400 font-semibold text-[11px] flex items-center space-x-1 transition border border-slate-700"
                                    title="Chỉnh sửa Tên & SĐT"
                                  >
                                    <Edit3 className="w-3.5 h-3.5" />
                                    <span>Sửa</span>
                                  </button>
                                  {cust.last_session_id && (
                                    <button
                                      onClick={() => {
                                        const matched = cases.find((c) => c.session_id === cust.last_session_id);
                                        if (matched) {
                                          setSelectedCase(matched);
                                        }
                                        setActiveTab('inbox');
                                        loadCaseDetail(cust.last_session_id!);
                                      }}
                                      className="p-1.5 px-2.5 rounded-lg bg-[#95252E] hover:bg-[#B32D38] text-white font-semibold text-[11px] flex items-center space-x-1 transition shadow"
                                      title="Vào khung chat với khách này"
                                    >
                                      <MessageSquare className="w-3.5 h-3.5" />
                                      <span>Chat</span>
                                    </button>
                                  )}
                                  <button
                                    onClick={() => handleDeleteCustomer(cust.guest_id)}
                                    className="p-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 text-[11px] flex items-center transition border border-rose-500/20"
                                    title="Xóa thông tin khách này khỏi danh sách quản lý"
                                  >
                                    <Trash2 className="w-3.5 h-3.5" />
                                  </button>
                                </div>
                              </td>
                            </tr>
                          ))
                      )}
                    </tbody>
                  </table>
                </div>

                {customers.length > 0 && (
                  <div className="p-4 bg-[#0D1527]/60">
                    <Pagination
                      currentPage={customerPage}
                      pageSize={customerPageSize}
                      totalItems={customerTotal}
                      onPageChange={(page) => {
                        setCustomerPage(page);
                        loadCustomers(false, page, customerPageSize, customerSearch);
                      }}
                      onPageSizeChange={(size) => {
                        setCustomerPageSize(size);
                        setCustomerPage(1);
                        loadCustomers(false, 1, size, customerSearch);
                      }}
                    />
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ============================================================ */}
          {/* TAB: LỊCH SỬ CUỘC GỌI THOẠI WEBRTC */}
          {/* ============================================================ */}
          {activeTab === 'calls' && (
            <div className="max-w-6xl mx-auto space-y-6">
              {/* Header Box */}
              <div className="glass-panel-brand p-6 rounded-2xl flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div>
                  <h2 className="text-base font-bold text-white mb-1 flex items-center space-x-2">
                    <Headphones className="w-5 h-5 text-cyan-400" />
                    <span>Lịch Sử Cuộc Gọi Thoại WebRTC</span>
                  </h2>
                  <p className="text-xs text-slate-400">
                    Theo dõi toàn bộ các cuộc đàm thoại trực tiếp giữa Khách hàng và Chuyên viên CSKH Đông Đô Partners.
                  </p>
                </div>

                <div className="flex items-center space-x-3 shrink-0">
                  <button
                    onClick={() => loadVoiceCalls(true)}
                    className="p-2 px-3.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold flex items-center space-x-1.5 transition border border-slate-700"
                  >
                    <RefreshCw className="w-3.5 h-3.5" />
                    <span>Làm mới danh sách</span>
                  </button>
                </div>
              </div>

              {/* Calls List */}
              <div className="glass-panel-brand rounded-2xl overflow-hidden shadow-xl p-6 border border-[#1C2D56]">
                {isLoadingVoiceCalls ? (
                  <div className="py-16 flex flex-col items-center justify-center text-slate-400 text-xs space-y-3">
                    <RefreshCw className="w-8 h-8 animate-spin text-cyan-400" />
                    <span>Đang tải lịch sử cuộc gọi thoại...</span>
                  </div>
                ) : voiceCalls.length === 0 ? (
                  <div className="py-16 text-center text-slate-500 text-xs">
                    <PhoneCall className="w-12 h-12 mx-auto text-slate-600 mb-3 opacity-40" />
                    Chưa có cuộc gọi thoại nào được ghi nhận trong hệ thống.
                  </div>
                ) : (
                  <div className="space-y-3">
                    {voiceCalls.map((call) => (
                        <div
                          key={call.id}
                          className="p-4 rounded-xl bg-[#0A0F1D]/80 border border-slate-800 hover:border-slate-700 space-y-3 transition shadow"
                        >
                          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                            <div className="flex items-center space-x-2.5">
                              <div className="w-9 h-9 rounded-xl bg-emerald-600/20 border border-emerald-500/30 flex items-center justify-center text-emerald-400 shrink-0">
                                <Phone className="w-4 h-4" />
                              </div>
                              <div>
                                <div className="text-sm font-bold text-white flex items-center space-x-2">
                                  <span className="text-emerald-400">
                                    {call.caller_type === 'guest' ? '👤 Khách hàng' : '🎧 CSKH'} ({call.caller_id})
                                  </span>
                                  <span className="text-slate-500">➡️</span>
                                  <span className="text-slate-200">
                                    {call.callee_type === 'cskh' || call.callee_type === 'human_cs' ? '🎧 Chuyên viên CSKH' : '👤 Khách hàng'}
                                  </span>
                                </div>
                                <div className="text-[11px] text-slate-500 font-mono">
                                  Phiên chat: {call.session_id}
                                </div>
                              </div>
                            </div>

                            <div className="flex items-center space-x-2.5">
                              <span className="text-xs text-slate-400">
                                {new Date(call.created_at).toLocaleString('vi-VN')}
                              </span>
                              <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                                {call.status}
                              </span>
                              <button
                                onClick={() => handleDeleteVoiceCall(call.id)}
                                className="p-1 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 text-xs flex items-center transition border border-rose-500/20"
                                title="Xóa lịch sử cuộc gọi này"
                              >
                                <Trash2 className="w-3.5 h-3.5" />
                              </button>
                            </div>
                          </div>

                          <div className="flex items-center justify-between pt-2 border-t border-slate-800/60 text-xs text-slate-400">
                            <div>
                              ⏱️ Thời lượng đàm thoại: <strong className="text-white">{call.duration_seconds > 0 ? `${call.duration_seconds} giây` : 'Chưa ghi nhận'}</strong>
                            </div>
                            {call.session_id && (
                              <button
                                onClick={() => {
                                  const matched = cases.find((c) => c.session_id === call.session_id);
                                  if (matched) {
                                    setSelectedCase(matched);
                                    setActiveTab('inbox');
                                    loadCaseDetail(call.session_id);
                                  }
                                }}
                                className="text-xs text-sky-400 hover:text-sky-300 font-semibold flex items-center space-x-1"
                              >
                                <span>Xem phiên chat này</span>
                                <ExternalLink className="w-3 h-3" />
                              </button>
                            )}
                          </div>

                          {call.recording_url && (
                            <div className="pt-2">
                              <div className="text-[11px] font-semibold text-slate-300 mb-1 flex items-center space-x-1">
                                <span>🔊 Nghe lại ghi âm cuộc gọi 2 chiều:</span>
                              </div>
                              <audio controls preload="metadata" className="w-full h-9 rounded-lg">
                                <source src={call.recording_url} type="audio/webm" />
                                <source src={call.recording_url.startsWith('http') ? call.recording_url : `http://localhost:8080${call.recording_url}`} type="audio/webm" />
                                Trình duyệt không hỗ trợ phát âm thanh.
                              </audio>
                            </div>
                          )}
                        </div>
                      ))}

                    {voiceCalls.length > 0 && (
                      <Pagination
                        currentPage={callPage}
                        pageSize={callPageSize}
                        totalItems={callTotal}
                        onPageChange={(page) => {
                          setCallPage(page);
                          loadVoiceCalls(false, page, callPageSize);
                        }}
                        onPageSizeChange={(size) => {
                          setCallPageSize(size);
                          setCallPage(1);
                          loadVoiceCalls(false, 1, size);
                        }}
                      />
                    )}
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ============================================================ */}
          {/* TAB 2: CONTINUOUS LEARNING QUEUE (CHUẨN BẢN GỐC) */}
          {/* ============================================================ */}
          {activeTab === 'learning' && (
            <div className="max-w-5xl mx-auto space-y-6">
              {/* Auto Learn Toggle Box */}
              <div className="glass-panel-brand p-6 rounded-2xl flex flex-col md:flex-row md:items-center justify-between gap-4 border border-[#1C2D56]">
                <div className="space-y-1">
                  <h2 className="text-base font-bold text-white flex items-center space-x-2">
                    <span>🧠 Hàng Đợi &amp; Cơ Chế Tự Động Học Tri Thức Cho AI</span>
                  </h2>
                  <p className="text-xs text-slate-400 max-w-2xl leading-relaxed">
                    Khi Chuyên viên CSKH trả lời và đóng case, mẩu Q&amp;A sẽ được trích xuất. Bạn có thể bật chế độ <strong>Tự động học ngay</strong> hoặc <strong>Duyệt thủ công</strong> bên dưới.
                  </p>
                </div>

                {/* Auto-Learn Toggle Switch */}
                <div className="bg-slate-900/80 p-3.5 rounded-xl border border-slate-700/70 shrink-0 space-y-2">
                  <div className="flex items-center justify-between space-x-3">
                    <span className="text-xs font-semibold text-slate-200">
                      Chế độ nạp tri thức:
                    </span>
                    <button
                      type="button"
                      onClick={() => handleToggleAutoLearn(!autoLearnEnabled)}
                      className={`w-12 h-6 rounded-full transition-colors relative cursor-pointer ${
                        autoLearnEnabled ? 'bg-emerald-600' : 'bg-slate-700'
                      }`}
                    >
                      <span
                        className={`w-4 h-4 rounded-full bg-white absolute top-1 transition-transform ${
                          autoLearnEnabled ? 'right-1' : 'left-1'
                        }`}
                      />
                    </button>
                  </div>
                  <div className="text-[11px] text-slate-400">
                    {autoLearnEnabled
                      ? '🟢 Đang BẬT tự động: Q&A từ CSKH sẽ được nạp thẳng vào Qdrant ngay lập tức.'
                      : '⚪ Đang TẮT tự động: Q&A sẽ đưa vào hàng chờ bên dưới để duyệt thủ công.'}
                  </div>
                </div>
              </div>

              {/* Learning Items Header */}
              <div className="flex items-center justify-between pt-2">
                <h3 className="text-sm font-bold text-white flex items-center space-x-2">
                  <span>Danh sách mẩu Q&amp;A đang chờ phê duyệt</span>
                  <span className="px-2 py-0.5 rounded-full bg-amber-500/20 text-amber-300 text-xs font-semibold border border-amber-500/30">
                    {pendingLearning.length}
                  </span>
                </h3>
                <button
                  type="button"
                  onClick={handleResetLearning}
                  className="px-3 py-1.5 rounded-lg text-xs text-rose-300 hover:text-rose-200 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/30 font-medium transition cursor-pointer flex items-center space-x-1"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  <span>🗑️ Xóa / Reset Tri Thức Đã Nạp</span>
                </button>
              </div>

              {/* Items List */}
              <div className="space-y-4">
                {pendingLearning.length === 0 ? (
                  <div className="glass-panel-brand p-12 rounded-2xl text-center text-slate-400 text-sm border border-slate-800">
                    📭 Hiện tại không có mẩu Q&amp;A nào chờ phê duyệt. Khi chuyên viên CSKH đóng case ở chế độ duyệt thủ công hoặc sau các cuộc gọi thoại, Q&amp;A sẽ hiển thị tại đây để bạn kiểm tra, chỉnh sửa và phê duyệt cho AI học!
                  </div>
                ) : (
                  pendingLearning.map((item) => (
                      <div
                        key={item.id}
                        className="glass-panel-brand p-5 rounded-2xl border border-slate-700/70 space-y-3 shadow-lg"
                      >
                        <div className="flex items-center justify-between border-b border-slate-800 pb-2">
                          <div className="flex items-center space-x-2">
                            <span className="text-[11px] font-semibold text-amber-400 uppercase bg-amber-500/10 px-2 py-0.5 rounded border border-amber-500/20">
                              Mã phiên: {item.session_id || 'Thủ công'}
                            </span>
                            <span className="text-xs text-slate-400">
                              Nguồn: <span className="text-slate-300 font-medium">{item.created_by || 'CSKH'}</span>
                            </span>
                          </div>
                          <span className="text-[11px] text-slate-500">
                            {new Date(item.created_at).toLocaleString('vi-VN')}
                          </span>
                        </div>

                        <div className="space-y-3">
                          <div>
                            <label className="block text-xs font-semibold text-slate-300 mb-1">
                              ❓ Câu hỏi của Khách hàng / Chủ đề:
                            </label>
                            <input
                              type="text"
                              value={item.question}
                              onChange={(e) => {
                                const newQuestion = e.target.value;
                                setPendingLearning((prev) =>
                                  prev.map((it) => (it.id === item.id ? { ...it, question: newQuestion } : it))
                                );
                              }}
                              className="w-full px-3.5 py-2.5 rounded-xl bg-[#0A0F1D] border border-slate-700 text-sm text-white focus:outline-none focus:border-[#B32D38]"
                              placeholder="Nhập câu hỏi..."
                            />
                          </div>

                          <div>
                            <label className="block text-xs font-semibold text-slate-300 mb-1">
                              💡 Câu trả lời chuẩn của CSKH / Nội dung tư vấn:
                            </label>
                            <textarea
                              rows={3}
                              value={item.answer}
                              onChange={(e) => {
                                const newAnswer = e.target.value;
                                setPendingLearning((prev) =>
                                  prev.map((it) => (it.id === item.id ? { ...it, answer: newAnswer } : it))
                                );
                              }}
                              className="w-full px-3.5 py-2.5 rounded-xl bg-[#0A0F1D] border border-slate-700 text-sm text-white focus:outline-none focus:border-[#B32D38] leading-relaxed"
                              placeholder="Nhập câu trả lời chuẩn xác để AI học..."
                            />
                          </div>
                        </div>

                        <div className="flex items-center justify-end space-x-3 pt-2 border-t border-slate-800">
                          <button
                            type="button"
                            onClick={() => handleRejectLearning(item.id)}
                            className="px-3.5 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold flex items-center space-x-1 transition cursor-pointer"
                          >
                            <span>🗑️ Bỏ qua</span>
                          </button>
                          <button
                            type="button"
                            onClick={() => handleApproveLearning(item.id, item.question, item.answer)}
                            className="px-4 py-2 rounded-xl btn-brand-primary text-xs font-semibold flex items-center space-x-1.5 transition cursor-pointer"
                          >
                            <Check className="w-4 h-4" />
                            <span>✅ Phê Duyệt &amp; Nạp Cho AI Học</span>
                          </button>
                        </div>
                      </div>
                    ))
                )}

                {pendingLearning.length > 0 && (
                  <div className="glass-panel-brand p-4 rounded-2xl border border-slate-800">
                    <Pagination
                      currentPage={learningPage}
                      pageSize={learningPageSize}
                      totalItems={learningTotal}
                      onPageChange={(page) => {
                        setLearningPage(page);
                        loadLearningQueue(page, learningPageSize);
                      }}
                      onPageSizeChange={(size) => {
                        setLearningPageSize(size);
                        setLearningPage(1);
                        loadLearningQueue(1, size);
                      }}
                    />
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ============================================================ */}
          {/* TAB 3: KHO TRI THỨC (QDRANT VECTOR DB) */}
          {/* ============================================================ */}
          {activeTab === 'knowledge' && (
            <div className="max-w-5xl mx-auto space-y-6">
              {/* Document Upload Box */}
              <div className="glass-panel-brand p-6 rounded-2xl">
                <h3 className="text-base font-bold text-white mb-2">Tải Lên Tài Liệu Tri Thức Mới (.docx)</h3>
                <p className="text-xs text-slate-400 mb-4">
                  Hệ thống sẽ tự động trích xuất văn bản, chia đoạn (chunking 800 ký tự) và tạo vector nhúng nạp vào Qdrant.
                </p>

                <form onSubmit={handleUploadDoc} className="flex flex-col sm:flex-row items-center gap-3">
                  <input
                    type="file"
                    accept=".docx"
                    onChange={(e) => setUploadFile(e.target.files?.[0] || null)}
                    className="w-full sm:w-auto flex-1 p-2 rounded-xl glass-input text-xs"
                  />
                  <button
                    type="submit"
                    disabled={!uploadFile || isUploading}
                    className="w-full sm:w-auto px-5 py-2.5 rounded-xl btn-brand-primary text-xs font-semibold flex items-center justify-center space-x-2"
                  >
                    <Upload className="w-4 h-4" />
                    <span>{isUploading ? 'Đang nạp...' : 'Nạp Vào Qdrant'}</span>
                  </button>
                </form>

                {uploadMsg && <div className="text-xs mt-3 text-emerald-400">{uploadMsg}</div>}
              </div>

              {/* Vector DB Stats */}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="glass-panel-brand p-5 rounded-2xl">
                  <div className="text-xs text-slate-400">Vector Collection</div>
                  <div className="text-xl font-bold text-white mt-1">
                    {knowledge?.collection_name || 'dongdo_knowledge'}
                  </div>
                </div>
                <div className="glass-panel-brand p-5 rounded-2xl">
                  <div className="text-xs text-slate-400">Tổng Số Chunks</div>
                  <div className="text-xl font-bold text-sky-400 mt-1">
                    {knowledge?.total_chunks || 60} chunks
                  </div>
                </div>
                <div className="glass-panel-brand p-5 rounded-2xl">
                  <div className="text-xs text-slate-400">Kích Thước Vector</div>
                  <div className="text-xl font-bold text-rose-400 mt-1">384 Dimensions</div>
                </div>
              </div>

              {/* Documents List Card (Giống bản gốc) */}
              <div className="glass-panel-brand p-6 rounded-2xl">
                <div className="flex items-center justify-between mb-4">
                  <div>
                    <h3 className="text-sm font-bold text-white">Danh Sách File Tài Liệu Gốc (tailieu/)</h3>
                    <p className="text-xs text-slate-400 mt-0.5">Các file tài liệu nguồn đã được đồng bộ hóa và vector hóa</p>
                  </div>
                  <span className="px-2.5 py-1 rounded-lg bg-[#1C2D56] text-xs font-semibold text-sky-400 border border-slate-700">
                    {knowledge?.documents?.length || 2} tài liệu
                  </span>
                </div>

                <div className="overflow-x-auto rounded-xl border border-slate-800">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-[#1C2D56]/80 text-slate-200 uppercase tracking-wider font-semibold border-b border-slate-700/60">
                      <tr>
                        <th className="px-4 py-3">Tên File Tài Liệu</th>
                        <th className="px-4 py-3">Dung Lượng</th>
                        <th className="px-4 py-3">Trạng Thái Vector DB</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800/60 bg-[#0A0F1D]/40">
                      {knowledge?.documents && knowledge.documents.length > 0 ? (
                        knowledge.documents
                          .slice((docPage - 1) * docPageSize, docPage * docPageSize)
                          .map((doc, idx) => (
                            <tr key={idx} className="hover:bg-slate-800/30 transition-colors">
                              <td className="px-4 py-3 font-medium text-white flex items-center space-x-2">
                                <span className="text-base">📄</span>
                                <span>{doc.filename}</span>
                              </td>
                              <td className="px-4 py-3 text-slate-300">{doc.size_kb} KB</td>
                              <td className="px-4 py-3">
                                <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                                  🟢 Đã vector hóa vào Qdrant
                                </span>
                              </td>
                            </tr>
                          ))
                      ) : (
                        <tr>
                          <td colSpan={3} className="px-4 py-6 text-center text-slate-500">
                            Đang tải danh sách tài liệu...
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>

                {knowledge?.documents && knowledge.documents.length > 0 && (
                  <div className="pt-3">
                    <Pagination
                      currentPage={docPage}
                      pageSize={docPageSize}
                      totalItems={knowledge.documents.length}
                      onPageChange={setDocPage}
                      onPageSizeChange={setDocPageSize}
                    />
                  </div>
                )}
              </div>

              {/* Chunks Preview */}
              <div className="glass-panel-brand p-6 rounded-2xl">
                <h3 className="text-sm font-bold text-white mb-4">Danh Sách Đoạn Tri Thức Trong Qdrant (60 Chunks)</h3>
                <div className="space-y-3 max-h-96 overflow-y-auto pr-2">
                  {knowledge?.chunks && knowledge.chunks.length > 0 ? (
                    knowledge.chunks.map((chunk, idx) => (
                      <div key={idx} className="p-3.5 rounded-xl bg-[#0A0F1D]/60 border border-slate-800 text-xs">
                        <div className="text-[10px] text-rose-400 font-semibold mb-1">Chunk #{idx + 1}</div>
                        <p className="text-slate-300 leading-relaxed">{chunk.text}</p>
                      </div>
                    ))
                  ) : (
                    <div className="text-center py-6 text-slate-500 text-xs">
                      60 Chunks tri thức đã được nạp sẵn từ các tài liệu Đông Đô Partners vào collection <strong>dongdo_knowledge</strong>.
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* ============================================================ */}
          {/* TAB 4: BÁO CÁO & THỐNG KÊ */}
          {/* ============================================================ */}
          {activeTab === 'analytics' && (
            <div className="max-w-5xl mx-auto space-y-6">
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <div className="glass-panel-brand p-5 rounded-2xl">
                  <div className="text-xs text-slate-400">Tổng Số Ca Hỗ Trợ</div>
                  <div className="text-2xl font-extrabold text-white mt-1">
                    {analytics?.total_cases || 0}
                  </div>
                </div>
                <div className="glass-panel-brand p-5 rounded-2xl">
                  <div className="text-xs text-slate-400">Tỷ Lệ AI Tự Phục Vụ</div>
                  <div className="text-2xl font-extrabold text-sky-400 mt-1">
                    {analytics?.ai_service_rate || 100}%
                  </div>
                </div>
                <div className="glass-panel-brand p-5 rounded-2xl">
                  <div className="text-xs text-slate-400">Ca Cần CSKH Hỗ Trợ</div>
                  <div className="text-2xl font-extrabold text-rose-400 mt-1">
                    {analytics?.needs_human_cases || 0}
                  </div>
                </div>
                <div className="glass-panel-brand p-5 rounded-2xl">
                  <div className="text-xs text-slate-400">Tri Thức Đã Học Thêm</div>
                  <div className="text-2xl font-extrabold text-amber-400 mt-1">
                    {analytics?.total_learned_qa || 0}
                  </div>
                </div>
              </div>

              {/* Status Breakdown */}
              <div className="glass-panel-brand p-6 rounded-2xl">
                <h3 className="text-sm font-bold text-white mb-4">Phân Bổ Trạng Thái Các Ca Hỗ Trợ</h3>
                <div className="space-y-4">
                  <div>
                    <div className="flex justify-between text-xs mb-1">
                      <span className="text-slate-300">🤖 AI Đang Tự Động Phục Vụ</span>
                      <span className="font-semibold text-sky-400">{analytics?.ai_active_cases || 0} ca</span>
                    </div>
                    <div className="w-full h-2 rounded-full bg-slate-800 overflow-hidden">
                      <div className="h-full bg-sky-500 rounded-full" style={{ width: `${analytics?.ai_service_rate || 100}%` }} />
                    </div>
                  </div>

                  <div>
                    <div className="flex justify-between text-xs mb-1">
                      <span className="text-slate-300">⏳ Đang Chờ CSKH Tiếp Nhận</span>
                      <span className="font-semibold text-rose-400">{analytics?.needs_human_cases || 0} ca</span>
                    </div>
                    <div className="w-full h-2 rounded-full bg-slate-800 overflow-hidden">
                      <div
                        className="h-full bg-rose-500 rounded-full"
                        style={{
                          width: `${
                            analytics?.total_cases
                              ? ((analytics.needs_human_cases || 0) / analytics.total_cases) * 100
                              : 0
                          }%`,
                        }}
                      />
                    </div>
                  </div>

                  <div>
                    <div className="flex justify-between text-xs mb-1">
                      <span className="text-slate-300">👨‍💼 Đang Được CSKH Xử Lý</span>
                      <span className="font-semibold text-amber-400">{analytics?.active_human_cases || 0} ca</span>
                    </div>
                    <div className="w-full h-2 rounded-full bg-slate-800 overflow-hidden">
                      <div
                        className="h-full bg-amber-500 rounded-full"
                        style={{
                          width: `${
                            analytics?.total_cases
                              ? ((analytics.active_human_cases || 0) / analytics.total_cases) * 100
                              : 0
                          }%`,
                        }}
                      />
                    </div>
                  </div>

                  <div>
                    <div className="flex justify-between text-xs mb-1">
                      <span className="text-slate-300">✅ Đã Đóng &amp; Giải Quyết</span>
                      <span className="font-semibold text-emerald-400">{analytics?.resolved_cases || 0} ca</span>
                    </div>
                    <div className="w-full h-2 rounded-full bg-slate-800 overflow-hidden">
                      <div
                        className="h-full bg-emerald-500 rounded-full"
                        style={{
                          width: `${
                            analytics?.total_cases
                              ? ((analytics.resolved_cases || 0) / analytics.total_cases) * 100
                              : 0
                          }%`,
                        }}
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* ============================================================ */}
          {/* TAB 5: CẤU HÌNH HỆ THỐNG */}
          {/* ============================================================ */}
          {activeTab === 'config' && (
            <div className="max-w-4xl mx-auto space-y-6">
              <form onSubmit={handleSaveConfig} className="glass-panel-brand p-6 rounded-2xl space-y-5">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                    System Prompt (Quy Tắc Trả Lời Của AI)
                  </label>
                  <textarea
                    rows={12}
                    value={configPrompt}
                    onChange={(e) => setConfigPrompt(e.target.value)}
                    className="w-full p-4 rounded-xl glass-input text-xs font-mono leading-relaxed"
                  />
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                      Mô Hình AI (Claude)
                    </label>
                    <select
                      value={configModel}
                      onChange={(e) => setConfigModel(e.target.value)}
                      className="w-full p-3 rounded-xl glass-input text-sm"
                    >
                      <option value="claude-haiku-4-5-20251001">Claude 3.5 Haiku (Khuyên dùng - Nhanh &amp; Tiết kiệm)</option>
                      <option value="claude-3-5-sonnet-20241022">Claude 3.5 Sonnet (Thông minh nhất)</option>
                      <option value="claude-3-opus-20240229">Claude 3 Opus</option>
                    </select>
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                      Nhiệt Độ (Temperature: {configTemp})
                    </label>
                    <input
                      type="range"
                      min="0"
                      max="1"
                      step="0.05"
                      value={configTemp}
                      onChange={(e) => setConfigTemp(parseFloat(e.target.value))}
                      className="w-full mt-2 accent-[#95252E]"
                    />
                    <div className="text-[11px] text-slate-500 mt-1">
                      0.0 - 0.2: Trả lời chính xác theo tài liệu, không suy diễn
                    </div>
                  </div>
                </div>

                {saveConfigMsg && <div className="text-xs text-emerald-400 font-semibold">{saveConfigMsg}</div>}

                <div className="flex justify-end">
                  <button type="submit" className="px-6 py-3 rounded-xl btn-brand-primary text-sm font-semibold">
                    Lưu Cấu Hình
                  </button>
                </div>
              </form>
            </div>
          )}

          {activeTab === 'partner_dashboard' && (
            <PartnerDashboardView onSelectTab={(key) => setActiveTab(key as TabType)} />
          )}

          {activeTab === 'partner_analytics' && <PartnerAnalyticsView />}

          {activeTab === 'partner_config' && (
            <PartnerConfigView
              permissionLevel={getFeaturePermission('partner_config')}
              onReportError={reportSystemError}
            />
          )}

          {activeTab === 'update_data_test' && (
            (currentUser?.role || '').toLowerCase() === 'owner' ? (
              <TestDataUploadView onReportError={reportSystemError} />
            ) : (
              <div className="p-8 text-center text-rose-400 font-bold">
                🚫 Từ chối truy cập: Tính năng Update Data Test chỉ dành riêng cho tài khoản có vai trò Owner.
              </div>
            )
          )}
        </div>
      </main>

      <ErrorCenterModal
        isOpen={showErrorCenterModal}
        onClose={() => setShowErrorCenterModal(false)}
        errors={systemErrors}
        onMarkAsHandled={handleMarkErrorAsHandled}
        onClearHandled={handleClearHandledErrors}
      />
    </div>
  );
}

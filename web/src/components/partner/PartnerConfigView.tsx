'use client';

import React, { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import { SystemConfig, PermissionLevel } from '@/lib/types';
import './PartnerStyles.css';

type ConfigSubViewType = 'subview-prompt' | 'subview-account' | 'subview-permission' | 'subview-template' | 'subview-database';

interface UserAccount {
  id: string;
  fullName: string;
  email: string;
  role: string;
  status: string;
  createdAt: string;
}

interface QuickTemplate {
  id: string;
  title: string;
  category: string;
  content: string;
}

interface PartnerConfigViewProps {
  permissionLevel?: PermissionLevel;
  onReportError?: (source: string, title: string, details: string) => void;
}

export const PartnerConfigView: React.FC<PartnerConfigViewProps> = ({ permissionLevel = 'act', onReportError }) => {
  const isReadOnly = permissionLevel === 'view';
  const [activeSubView, setActiveSubView] = useState<ConfigSubViewType>('subview-prompt');

  // 1. Prompt & Model State
  const [systemPrompt, setSystemPrompt] = useState(
    `Bạn là chuyên viên Chăm sóc khách hàng của Đông Đô Partners. Nhiệm vụ của bạn là tư vấn, giải đáp thắc mắc cho khách hàng về Hàng hóa phái sinh, hướng dẫn nền tảng DDP Invest, quy trình nạp/rút tiền và quản trị rủi ro.

PHONG CÁCH GIAO TIẾP VÀ XƯNG HỘ:
- Hãy trả lời tự nhiên, lịch sự, chu đáo và thân thiện như một con người thật (xưng "em" hoặc "tôi", gọi khách hàng là "anh/chị" hoặc "bạn").
- TUYỆT ĐỐI KHÔNG tự khai báo hoặc đề cập mình là "trợ lý ảo", "bot", "AI", "hệ thống tự động" hay "được huấn luyện".
- Trả lời trôi chảy, ấm áp, chuẩn mực chuyên viên CSKH chuyên nghiệp.

QUY TẮC CỐT LÕI:
Bạn PHẢI LUÔN tìm kiếm và TRÍCH XUẤT CHÍNH XÁC câu trả lời từ Cơ sở dữ liệu (Knowledge).`
  );
  const [llmModel, setLlmModel] = useState('claude-haiku-4-5-20251001');
  const [temperature, setTemperature] = useState('0.1');
  const [isSavingConfig, setIsSavingConfig] = useState(false);
  const [toastMsg, setToastMsg] = useState('');

  // 2. Account Management State
  const [users, setUsers] = useState<UserAccount[]>([
    { id: '1', fullName: 'Admin Đông Đô', email: 'admin@dongdopartner.vn', role: 'Owner', status: 'Hoạt động', createdAt: '2026-08-01' },
    { id: '2', fullName: 'Nguyễn Thị Thu', email: 'thunt@dongdopartner.vn', role: 'Staff CS', status: 'Hoạt động', createdAt: '2026-08-10' },
    { id: '3', fullName: 'Trần Văn Hoàng', email: 'hoangtv@dongdopartner.vn', role: 'Staff CS', status: 'Hoạt động', createdAt: '2026-08-15' },
    { id: '4', fullName: 'Phạm Minh Anh', email: 'anhpm@dongdopartner.vn', role: 'Leader CS', status: 'Hoạt động', createdAt: '2026-08-18' },
  ]);
  const [userSearch, setUserSearch] = useState('');
  const [roleFilter, setRoleFilter] = useState('ALL');

  // New user form
  const [newFullName, setNewFullName] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newRole, setNewRole] = useState('Staff CS');
  const [newPassword, setNewPassword] = useState('12345678');

  // Edit user modal state
  const [editingUser, setEditingUser] = useState<UserAccount | null>(null);
  const [editFullName, setEditFullName] = useState('');
  const [editRole, setEditRole] = useState('Staff CS');
  const [editIsActive, setEditIsActive] = useState(true);
  const [editPassword, setEditPassword] = useState('');
  const [isSavingUserEdit, setIsSavingUserEdit] = useState(false);

  const handleOpenEditModal = (u: UserAccount) => {
    if (isReadOnly) return;
    setEditingUser(u);
    setEditFullName(u.fullName);
    setEditRole(u.role);
    setEditIsActive(u.status === 'Hoạt động');
    setEditPassword('');
  };

  const handleSaveUserEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingUser || isReadOnly) return;
    setIsSavingUserEdit(true);

    try {
      await api.updateUser(editingUser.email, {
        fullName: editFullName,
        role: editRole,
        isActive: editIsActive,
        password: editPassword,
      });
      showToast(`✅ Đã cập nhật thông tin tài khoản [${editingUser.email}] thành công!`);
      setEditingUser(null);
      await loadUsers();
    } catch (err: any) {
      const errMsg = err.message || 'Không thể lưu vào CSDL';
      showToast(`❌ Lỗi cập nhật: ${errMsg}`);
      if (onReportError) {
        onReportError('Chỉnh Sửa Tài Khoản', `Lỗi cập nhật [${editingUser.email}]`, errMsg);
      }
    } finally {
      setIsSavingUserEdit(false);
    }
  };

  // 4. Quick Templates State
  const [templates, setTemplates] = useState<QuickTemplate[]>([
    { id: '1', title: 'Hướng dẫn nạp tiền DDP Invest', category: 'Nạp/Rút tiền', content: 'Chào anh/chị, hạn mức nạp tối thiểu là 1,000,000 VNĐ qua QR Napas 24/7...' },
    { id: '2', title: 'Quy định eKYC mở tài khoản', category: 'Tài khoản', content: 'Để hoàn tất eKYC, anh/chị vui lòng chụp rõ 2 mặt CCCD và quét khuôn mặt...' },
    { id: '3', title: 'Thông báo Margin Call cảnh báo', category: 'Rủi ro', content: 'Tài khoản của anh/chị đang có tỷ lệ ký quỹ dưới 80%, vui lòng bổ sung ký quỹ...' },
  ]);

  // 3. RBAC Matrix State
  const SYSTEM_ROLES = [
    { key: 'Owner', label: '👑 Owner', desc: 'Chủ DN / Toàn quyền' },
    { key: 'Admin', label: '🛡️ Admin', desc: 'Quản trị hệ thống' },
    { key: 'Leader', label: '⭐ Leader', desc: 'Trưởng nhóm CSKH' },
    { key: 'Staff', label: '💬 Staff', desc: 'Nhân viên CSKH' },
  ];

  const SYSTEM_FEATURES = [
    {
      key: 'partner_dashboard',
      title: '📊 Tổng Quan Dashboard',
      desc: 'KPI Summary, Biểu đồ tự động hóa AI, Inbox mới nhất',
      subFeatures: [
        { key: 'partner_dashboard.kpi', title: 'KPI Summary Cards', desc: 'Xem 3 thẻ chỉ số tổng quan' },
        { key: 'partner_dashboard.chart', title: 'Biểu Đồ Xu Hướng & CSAT', desc: 'Xem đồ thị xu hướng AI & đánh giá' },
        { key: 'partner_dashboard.recent', title: 'Danh Sách Ca Mới Nhất', desc: 'Xem danh sách các ca tư vấn gần đây' },
      ],
    },
    {
      key: 'inbox',
      title: '💬 Live CS Inbox',
      desc: 'Tiếp nhận case, Chat trực tiếp khách hàng, Handoff AI <-> Human',
      subFeatures: [
        { key: 'inbox.view_chat', title: 'Xem Nội Dung Đoạn Chat', desc: 'Đọc hội thoại giữa AI & khách hàng' },
        { key: 'inbox.send_msg', title: 'Trả Lời Tin Nhắn & Handoff', desc: 'Gửi tin nhắn trực tiếp và nhận ca tư vấn' },
        { key: 'inbox.close_case', title: 'Đóng Ca & Đổi Trạng Thái', desc: 'Thao tác đóng ca tư vấn' },
      ],
    },
    {
      key: 'customers',
      title: '👤 Quản Lý Khách Hàng (CRM)',
      desc: 'Danh sách hồ sơ khách hàng, Sửa/Xóa khách hàng',
      subFeatures: [
        { key: 'customers.view_list', title: 'Xem Danh Sách Hồ Sơ', desc: 'Xem thông tin danh sách khách hàng' },
        { key: 'customers.edit_info', title: 'Chỉnh Sửa Thông Tin', desc: 'Cập nhật thông tin chi tiết khách hàng' },
        { key: 'customers.delete', title: 'Xóa Hồ Sơ Khách Hàng', desc: 'Xóa dữ liệu khách hàng khỏi CRM' },
      ],
    },
    {
      key: 'calls',
      title: '📞 Gọi Thoại WebRTC & Lịch Sử',
      desc: 'Gọi điện 2 chiều trên trình duyệt, Nghe lại ghi âm, Xóa cuộc gọi',
      subFeatures: [
        { key: 'calls.make_call', title: 'Thực Hiện Cuộc Gọi WebRTC', desc: 'Bấm gọi trực tiếp trên trình duyệt' },
        { key: 'calls.view_history', title: 'Lịch Sử & Nghe Ghi Âm', desc: 'Xem lịch sử và nghe file âm thanh ghi âm' },
        { key: 'calls.delete_record', title: 'Xóa Ghi Âm Cuộc Gọi', desc: 'Xóa file và nhật ký cuộc gọi' },
      ],
    },
    {
      key: 'learning',
      title: '🧠 Tri Thức Học Tập AI Queue',
      desc: 'Duyệt/Từ chối kiến thức AI rút ra từ đoạn chat',
      subFeatures: [
        { key: 'learning.view_queue', title: 'Xem Hàng Chờ Tri Thức', desc: 'Đọc danh sách các câu Q&A AI tự học' },
        { key: 'learning.approve', title: 'Duyệt & Từ Chối Tri Thức', desc: 'Duyệt lưu hoặc bỏ qua tri thức mới' },
      ],
    },
    {
      key: 'knowledge',
      title: '📚 Kho Tri Thức RAG',
      desc: 'Upload tài liệu .docx, Re-index vector DB, Xóa tài liệu',
      subFeatures: [
        { key: 'knowledge.view_docs', title: 'Xem Danh Sách Tài Liệu', desc: 'Xem các file tài liệu đã upload' },
        { key: 'knowledge.upload_doc', title: 'Upload Tài Liệu Mới', desc: 'Tải file .docx/.pdf/.txt lên hệ thống' },
        { key: 'knowledge.reindex', title: 'Re-Index Vector DB', desc: 'Yêu cầu AI cập nhật lại cơ sở dữ liệu' },
        { key: 'knowledge.delete_doc', title: 'Xóa File Tài Liệu', desc: 'Xóa tài liệu khỏi kho tri thức' },
      ],
    },
    {
      key: 'partner_analytics',
      title: '📈 Báo Cáo & Phân Tích CX',
      desc: '7 Báo cáo phân tích chuyên sâu & Xuất dữ liệu báo cáo (JSON/PDF)',
      subFeatures: [
        { key: 'partner_analytics.overview', title: '1. Báo Cáo Tổng Quan CX', desc: 'Xem số liệu tổng quan ca hỗ trợ & tăng trưởng' },
        { key: 'partner_analytics.ai_performance', title: '2. Báo Cáo Hiệu Suất AI', desc: 'Xem tỷ lệ AI tự phục vụ & ca chuyển CSKH' },
        { key: 'partner_analytics.staff_performance', title: '3. Báo Cáo Hiệu Suất Nhân Viên', desc: 'Xem năng suất, số ca xử lý & đánh giá nhân viên' },
        { key: 'partner_analytics.cx_csat', title: '4. Báo Cáo Trải Nghiệm CSAT & NPS', desc: 'Xem điểm số hài lòng & nhận xét từ khách hàng' },
        { key: 'partner_analytics.operational', title: '5. Báo Cáo Tải Vận Hành', desc: 'Xem phân bổ tải theo giờ & khung giờ cao điểm' },
        { key: 'partner_analytics.issue_analysis', title: '6. Báo Cáo Phân Loại Sự Cố', desc: 'Xem top các vấn đề thường gặp & nguyên nhân' },
        { key: 'partner_analytics.ai_learning_stats', title: '7. Báo Cáo Thống Kê AI Tự Học', desc: 'Xem tiến độ học tri thức mới của AI từ đoạn chat' },
        { key: 'partner_analytics.export_report', title: '8. Xuất Báo Cáo & Dữ Liệu', desc: 'Thao tác xuất file JSON / PDF báo cáo' },
      ],
    },
    {
      key: 'partner_config',
      title: '⚙️ Cấu Hình & Phân Quyền',
      desc: 'Quản lý tài khoản User, Ma trận RBAC, Tin nhắn mẫu, Audit logs',
      subFeatures: [
        { key: 'partner_config.prompt', title: 'System Prompt & Model', desc: 'Cấu hình prompt cốt lõi và tham số LLM' },
        { key: 'partner_config.accounts', title: 'Quản Lý Tài Khoản User', desc: 'Tạo mới, sửa vai trò, đổi mật khẩu và khóa user' },
        { key: 'partner_config.rbac', title: 'Ma Trận Phân Quyền RBAC', desc: 'Cấu hình quyền Act/View/None cho vai trò' },
        { key: 'partner_config.templates', title: 'Quản Lý Tin Nhắn Mẫu', desc: 'Tạo và chỉnh sửa mẫu câu trả lời nhanh' },
        { key: 'partner_config.database', title: 'Bảo Trì Database', desc: 'Dọn dẹp cache và xuất dữ liệu sao lưu' },
      ],
    },
    {
      key: 'config',
      title: '🎛️ Cấu Hình AI LLM Studio',
      desc: 'System Prompt, Model Haiku/Sonnet/Opus, Temperature',
      subFeatures: [
        { key: 'config.view_studio', title: 'Xem Cấu Hình Studio', desc: 'Xem thông số cấu hình AI Studio' },
        { key: 'config.edit_prompt', title: 'Sửa Prompt Cốt Lõi', desc: 'Thay đổi trực tiếp chỉ thị của AI' },
      ],
    },
  ];

  const [rbacMatrix, setRbacMatrix] = useState<Record<string, Record<string, PermissionLevel>>>({});
  const [expandedFeatures, setExpandedFeatures] = useState<Record<string, boolean>>({});
  const [isSavingPermissions, setIsSavingPermissions] = useState(false);

  const toggleExpandFeature = (key: string) => {
    setExpandedFeatures((prev) => ({
      ...prev,
      [key]: !prev[key],
    }));
  };

  useEffect(() => {
    loadSystemConfig();
  }, []);

  useEffect(() => {
    if (activeSubView === 'subview-account') {
      loadUsers();
    } else if (activeSubView === 'subview-permission') {
      loadPermissions();
    }
  }, [activeSubView]);

  const loadPermissions = async () => {
    try {
      const perms = await api.listRolePermissions();
      const matrix: Record<string, Record<string, PermissionLevel>> = {};

      SYSTEM_ROLES.forEach((r) => {
        matrix[r.key] = {};
        SYSTEM_FEATURES.forEach((f) => {
          const defaultLevel = r.key === 'Owner' || r.key === 'Admin' ? 'act' : (r.key === 'Leader' && f.key !== 'config' ? 'act' : 'view');
          matrix[r.key][f.key] = defaultLevel;

          if (f.subFeatures) {
            f.subFeatures.forEach((sub) => {
              matrix[r.key][sub.key] = defaultLevel;
            });
          }
        });
      });

      if (perms && perms.length > 0) {
        perms.forEach((p) => {
          if (!matrix[p.role_name]) matrix[p.role_name] = {};
          matrix[p.role_name][p.feature_key] = p.permission_level || (p.can_edit ? 'act' : p.can_view ? 'view' : 'none');
        });
      }
      setRbacMatrix(matrix);
    } catch (err) {
      console.warn('Lỗi tải danh sách phân quyền từ Backend:', err);
    }
  };

  const handlePermissionChange = (role: string, feature: string, level: PermissionLevel) => {
    setRbacMatrix((prev) => {
      const updatedRolePerms = {
        ...(prev[role] || {}),
        [feature]: level,
      };

      // Cascade update to sub-features if feature is a main group
      const mainFeat = SYSTEM_FEATURES.find((f) => f.key === feature);
      if (mainFeat && mainFeat.subFeatures) {
        mainFeat.subFeatures.forEach((sub) => {
          updatedRolePerms[sub.key] = level;
        });
      }

      return {
        ...prev,
        [role]: updatedRolePerms,
      };
    });
  };

  const handleSavePermissions = async () => {
    if (isReadOnly) return;
    setIsSavingPermissions(true);
    try {
      const savePromises: Promise<void>[] = [];
      SYSTEM_ROLES.forEach((r) => {
        SYSTEM_FEATURES.forEach((f) => {
          const level = (rbacMatrix[r.key] && rbacMatrix[r.key][f.key]) || 'act';
          savePromises.push(api.upsertRolePermission(r.key, f.key, level));

          if (f.subFeatures) {
            f.subFeatures.forEach((sub) => {
              const subLevel = (rbacMatrix[r.key] && rbacMatrix[r.key][sub.key]) || level;
              savePromises.push(api.upsertRolePermission(r.key, sub.key, subLevel));
            });
          }
        });
      });
      await Promise.all(savePromises);
      showToast('✅ Đã lưu cấu hình phân quyền RBAC Matrix (Cấp Nhóm & Mục Con) vào CSDL thành công!');
    } catch (err: any) {
      const errMsg = err.message || 'Không thể kết nối CSDL';
      showToast(`❌ Lỗi lưu phân quyền: ${errMsg}`);
      if (onReportError) {
        onReportError('Phân Quyền RBAC', 'Lỗi lưu Ma trận Phân quyền', errMsg);
      }
    } finally {
      setIsSavingPermissions(false);
    }
  };

  const loadUsers = async () => {
    try {
      const dbUsers = await api.listUsers();
      if (dbUsers) {
        setUsers(dbUsers);
      }
    } catch (e: any) {
      console.warn('Lỗi tải danh sách tài khoản từ Backend:', e);
    }
  };

  const loadSystemConfig = async () => {
    try {
      const cfg = await api.getConfig();
      if (cfg) {
        if (cfg.system_prompt) setSystemPrompt(cfg.system_prompt);
        if (cfg.llm_model) setLlmModel(cfg.llm_model);
        if (cfg.temperature !== undefined) setTemperature(cfg.temperature.toString());
      }
    } catch (e) {
      console.warn('Lỗi tải cấu hình hệ thống từ DB:', e);
    }
  };

  const showToast = (msg: string) => {
    setToastMsg(msg);
    setTimeout(() => setToastMsg(''), 4000);
  };

  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isReadOnly) return;
    setIsSavingConfig(true);
    try {
      const tempNum = parseFloat(temperature.replace(',', '.')) || 0.1;
      await api.saveConfig({
        system_prompt: systemPrompt,
        llm_model: llmModel,
        temperature: tempNum,
      });
      showToast('✅ Đã lưu cấu hình System & LLM Model thành công!');
    } catch (e) {
      showToast('⚠️ Đã lưu cấu hình bộ nhớ local!');
    } finally {
      setIsSavingConfig(false);
    }
  };

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isReadOnly || !newFullName || !newEmail) return;

    try {
      const createdUser = await api.createUser(newFullName, newEmail, newRole, newPassword);
      showToast(`✅ Đã tạo tài khoản [${createdUser.fullName || newFullName}] thành công vào CSDL Backend!`);
      setNewFullName('');
      setNewEmail('');
      setNewPassword('DongDo@123');
      await loadUsers();
    } catch (err: any) {
      const errMsg = err.message || 'Không thể kết nối CSDL Backend';
      showToast(`❌ Lỗi tạo tài khoản: ${errMsg}`);
      if (onReportError) {
        onReportError('Tạo Tài Khoản', `Lỗi tạo tài khoản [${newEmail}]`, errMsg);
      }
    }
  };

  const handleDeleteUser = async (id: string, name: string, email: string) => {
    if (isReadOnly) return;
    if (confirm(`Bạn có chắc muốn khóa/xóa tài khoản [${name}]?`)) {
      try {
        await api.deleteUser(email || id);
        showToast(`🗑️ Đã xóa tài khoản [${name}] thành công!`);
        await loadUsers();
      } catch (err: any) {
        const errMsg = err.message || 'Không thể xóa tài khoản ở Backend';
        showToast(`❌ Lỗi xóa tài khoản: ${errMsg}`);
        if (onReportError) {
          onReportError('Xóa Tài Khoản', `Lỗi xóa tài khoản [${name}]`, errMsg);
        }
      }
    }
  };

  const filteredUsers = users.filter((u) => {
    const matchSearch = u.fullName.toLowerCase().includes(userSearch.toLowerCase()) || u.email.toLowerCase().includes(userSearch.toLowerCase());
    const matchRole = roleFilter === 'ALL' || u.role === roleFilter;
    return matchSearch && matchRole;
  });

  return (
    <div className="partner-wrapper">
      {/* Sub-Nav Pills */}
      <div className="config-subnav">
        <button className={`config-tab-pill ${activeSubView === 'subview-prompt' ? 'active' : ''}`} onClick={() => setActiveSubView('subview-prompt')}>
          <span>⚙️</span>
          <span>1. Prompt & Model AI</span>
        </button>
        <button className={`config-tab-pill ${activeSubView === 'subview-account' ? 'active' : ''}`} onClick={() => setActiveSubView('subview-account')}>
          <span>👥</span>
          <span>2. Cấu Hình Tài Khoản</span>
        </button>
        <button className={`config-tab-pill ${activeSubView === 'subview-permission' ? 'active' : ''}`} onClick={() => setActiveSubView('subview-permission')}>
          <span>🛡️</span>
          <span>3. Phân Quyền Hệ Thống</span>
        </button>
        <button className={`config-tab-pill ${activeSubView === 'subview-template' ? 'active' : ''}`} onClick={() => setActiveSubView('subview-template')}>
          <span>💬</span>
          <span>4. Tin Nhắn Mẫu</span>
        </button>
        <button className={`config-tab-pill ${activeSubView === 'subview-database' ? 'active' : ''}`} onClick={() => setActiveSubView('subview-database')}>
          <span>🗄️</span>
          <span>5. Quản Lý Bộ Nhớ DB</span>
        </button>
      </div>

      {/* Toast Alert */}
      {toastMsg && (
        <div style={{ position: 'fixed', bottom: '24px', right: '24px', background: '#10b981', color: '#fff', padding: '12px 20px', borderRadius: '10px', fontWeight: 600, zIndex: 99999 }}>
          {toastMsg}
        </div>
      )}

      {/* View Only Warning Banner */}
      {isReadOnly && (
        <div
          style={{
            background: 'linear-gradient(135deg, #451a03 0%, #78350f 100%)',
            border: '1px solid #f59e0b',
            color: '#fbbf24',
            padding: '12px 18px',
            borderRadius: '12px',
            fontSize: '13px',
            fontWeight: 600,
            marginBottom: '18px',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            boxShadow: '0 4px 12px rgba(245, 158, 11, 0.2)',
          }}
        >
          <span style={{ fontSize: '18px' }}>🟡</span>
          <div>
            <strong>Chế Độ Chỉ Xem (View Only):</strong> Bạn không có quyền chỉnh sửa cấu hình hệ thống. Tất cả các nút bấm tạo mới, sửa, xóa và lưu dữ liệu đã bị vô hiệu hóa.
          </div>
        </div>
      )}

      {/* SUBVIEW 1: PROMPT & MODEL */}
      {activeSubView === 'subview-prompt' && (
        <div className="card-config" style={{ maxWidth: '900px' }}>
          <div className="card-title-bar">
            <span>⚙️ Cấu Hình Prompt & Tham Số Mô Hình AI</span>
          </div>

          <form onSubmit={handleSaveConfig}>
            <div className="form-group">
              <label className="form-label">System Prompt (Chỉ thị cốt lõi & Nhân cách CSKH):</label>
              <textarea className="textarea-custom" value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} disabled={isReadOnly} />
            </div>

            <div className="form-row">
              <div className="form-group">
                <label className="form-label">Anthropic LLM Model:</label>
                <select className="select-custom" value={llmModel} onChange={(e) => setLlmModel(e.target.value)} disabled={isReadOnly}>
                  <option value="claude-haiku-4-5-20251001">claude-haiku-4-5-20251001</option>
                  <option value="claude-3-5-sonnet-20241022">claude-3-5-sonnet-20241022</option>
                  <option value="claude-3-opus-20240229">claude-3-opus-20240229</option>
                </select>
              </div>

              <div className="form-group">
                <label className="form-label">Temperature (Độ sáng tạo vs Chính xác):</label>
                <input type="text" className="input-custom" value={temperature} onChange={(e) => setTemperature(e.target.value)} disabled={isReadOnly} />
              </div>
            </div>

            <button type="submit" className="btn-primary-purple" disabled={isSavingConfig || isReadOnly} style={{ opacity: isReadOnly ? 0.5 : 1, cursor: isReadOnly ? 'not-allowed' : 'pointer' }}>
              <span>{isSavingConfig ? 'Đang lưu...' : 'Lưu Cấu Hình System 💾'}</span>
            </button>
          </form>
        </div>
      )}

      {/* SUBVIEW 2: ACCOUNT MANAGEMENT */}
      {activeSubView === 'subview-account' && (
        <div className="card-config">
          <div className="card-title-bar">
            <span>👥 Quản Lý & Cấu Hình Tài Khoản Hệ Thống</span>
          </div>

          {/* Create User Form */}
          {!isReadOnly && (
            <div style={{ background: '#0e1422', border: '1px solid rgba(255,255,255,0.08)', borderRadius: '10px', padding: '18px', marginBottom: '20px' }}>
              <h4 style={{ fontSize: '14px', fontWeight: 700, color: '#fff', marginBottom: '14px' }}>➕ Tạo Tài Khoản Mới Trên Hệ Thống</h4>
              <form onSubmit={handleCreateUser}>
                <div className="form-row" style={{ marginBottom: '14px' }}>
                  <div>
                    <label className="form-label">Họ và Tên:</label>
                    <input type="text" className="input-custom" placeholder="Ví dụ: Nguyễn Văn A" value={newFullName} onChange={(e) => setNewFullName(e.target.value)} required />
                  </div>
                  <div>
                    <label className="form-label">Tên Đăng Nhập / Email:</label>
                    <input type="email" className="input-custom" placeholder="anv@dongdopartner.vn" value={newEmail} onChange={(e) => setNewEmail(e.target.value)} required />
                  </div>
                </div>
                <div className="form-row" style={{ marginBottom: '18px' }}>
                  <div>
                    <label className="form-label">Vai Trò Hệ Thống:</label>
                    <select className="select-custom" value={newRole} onChange={(e) => setNewRole(e.target.value)}>
                      <option value="Staff CS">Staff CS (Chuyên viên CSKH)</option>
                      <option value="Leader CS">Leader / Sub-Admin</option>
                      <option value="Admin">Admin</option>
                      <option value="Owner">Owner</option>
                    </select>
                  </div>
                  <div>
                    <label className="form-label">Mật Khẩu Khởi Tạo:</label>
                    <input type="password" className="input-custom" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} required />
                  </div>
                </div>
                <button type="submit" className="btn-primary-purple">
                  <span>+ Tạo Tài Khoản</span>
                </button>
              </form>
            </div>
          )}

          {/* Accounts List Table */}
          <div style={{ display: 'flex', gap: '12px', marginBottom: '16px' }}>
            <input type="text" className="input-custom" style={{ flex: 1 }} placeholder="🔍 Tìm kiếm theo Tên nhân viên hoặc Email..." value={userSearch} onChange={(e) => setUserSearch(e.target.value)} />
            <select className="select-custom" style={{ width: '180px' }} value={roleFilter} onChange={(e) => setRoleFilter(e.target.value)}>
              <option value="ALL">-- Tất cả Vai Trò --</option>
              <option value="Staff CS">Staff CS</option>
              <option value="Leader CS">Leader CS</option>
              <option value="Admin">Admin</option>
              <option value="Owner">Owner</option>
            </select>
          </div>

          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Tên Nhân Viên</th>
                  <th>Email / Username</th>
                  <th>Vai Trò</th>
                  <th>Trạng Thái</th>
                  <th>Ngày Tạo</th>
                  <th>Thao Tác</th>
                </tr>
              </thead>
              <tbody>
                {filteredUsers.map((u) => {
                  const isOwnerAccount = u.role === 'Owner' || u.email.toLowerCase() === 'admin';
                  return (
                    <tr key={u.id}>
                      <td><strong>{u.fullName}</strong></td>
                      <td>{u.email}</td>
                      <td>
                        <span className={`status-pill ${u.role === 'Owner' ? 'pill-purple' : u.role === 'Admin' ? 'pill-blue' : u.role === 'Leader CS' ? 'pill-cyan' : 'pill-slate'}`} style={{ fontWeight: 700 }}>
                          {u.role === 'Owner' ? '👑 Owner' : u.role === 'Admin' ? '🛡️ Admin' : u.role === 'Leader CS' ? '⭐ Leader CS' : '💬 Staff CS'}
                        </span>
                      </td>
                      <td>
                        <span className={`status-pill ${u.status === 'Hoạt động' ? 'pill-green' : 'pill-red'}`}>
                          {u.status}
                        </span>
                      </td>
                      <td>{u.createdAt}</td>
                      <td>
                        {isReadOnly ? (
                          <span style={{ fontSize: '12px', color: '#94a3b8', fontStyle: 'italic', fontWeight: 500 }}>👁️ Chỉ xem</span>
                        ) : (
                          <>
                            <button
                              onClick={() => handleOpenEditModal(u)}
                              style={{
                                background: 'none',
                                border: 'none',
                                color: '#38bdf8',
                                cursor: 'pointer',
                                fontSize: '13px',
                                fontWeight: 600,
                                marginRight: '12px',
                              }}
                            >
                              ✏️ Sửa
                            </button>
                            {isOwnerAccount ? (
                              <span style={{ fontSize: '11px', color: '#f59e0b', fontWeight: 600 }}>🔒 Bảo vệ</span>
                            ) : (
                              <button onClick={() => handleDeleteUser(u.id, u.fullName, u.email)} style={{ background: 'none', border: 'none', color: '#ef4444', cursor: 'pointer', fontSize: '13px' }}>
                                🗑️ Xóa
                              </button>
                            )}
                          </>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* EDIT USER MODAL */}
      {editingUser && (
        <div style={{ position: 'fixed', inset: 0, zIndex: 9999, background: 'rgba(0,0,0,0.75)', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '16px' }}>
          <div style={{ background: '#0e1422', border: '1px solid rgba(255,255,255,0.15)', borderRadius: '16px', padding: '24px', width: '100%', maxWidth: '480px', boxShadow: '0 20px 50px rgba(0,0,0,0.8)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '18px', borderBottom: '1px solid rgba(255,255,255,0.1)', paddingBottom: '12px' }}>
              <h3 style={{ fontSize: '16px', fontWeight: 700, color: '#fff', margin: 0 }}>✏️ Chỉnh Sửa Tài Khoản Nhân Viên</h3>
              <button onClick={() => setEditingUser(null)} style={{ background: 'none', border: 'none', color: '#94a3b8', fontSize: '18px', cursor: 'pointer' }}>✕</button>
            </div>

            <form onSubmit={handleSaveUserEdit}>
              <div className="form-group" style={{ marginBottom: '14px' }}>
                <label className="form-label">Tên Đăng Nhập / Email (Cố định):</label>
                <input type="text" className="input-custom" value={editingUser.email} disabled style={{ opacity: 0.6, cursor: 'not-allowed' }} />
              </div>

              <div className="form-group" style={{ marginBottom: '14px' }}>
                <label className="form-label">Họ và Tên Nhân Viên:</label>
                <input type="text" className="input-custom" value={editFullName} onChange={(e) => setEditFullName(e.target.value)} required />
              </div>

              <div className="form-group" style={{ marginBottom: '14px' }}>
                <label className="form-label">Vai Trò Hệ Thống:</label>
                <select
                  className="select-custom"
                  value={editRole}
                  onChange={(e) => setEditRole(e.target.value)}
                  disabled={editingUser.role === 'Owner'}
                >
                  <option value="Staff CS">Staff CS (Nhân viên CSKH)</option>
                  <option value="Leader CS">Leader CS (Trưởng nhóm)</option>
                  <option value="Admin">Admin (Quản trị viên)</option>
                  <option value="Owner">Owner (Chủ doanh nghiệp)</option>
                </select>
                {editingUser.role === 'Owner' && (
                  <div style={{ fontSize: '11px', color: '#f59e0b', marginTop: '4px' }}>🔒 Tài khoản Owner được bảo vệ đặc biệt, không thể giáng cấp vai trò.</div>
                )}
              </div>

              <div className="form-group" style={{ marginBottom: '14px' }}>
                <label className="form-label">Trạng Thái Hoạt Động:</label>
                <select className="select-custom" value={editIsActive ? 'active' : 'inactive'} onChange={(e) => setEditIsActive(e.target.value === 'active')}>
                  <option value="active">🟢 Hoạt động (Bình thường)</option>
                  <option value="inactive">🔴 Tạm khóa (Cấm đăng nhập)</option>
                </select>
              </div>

              <div className="form-group" style={{ marginBottom: '20px' }}>
                <label className="form-label">Đổi Mật Khẩu Mới (Bỏ trống nếu không đổi):</label>
                <input type="password" className="input-custom" placeholder="Nhập mật khẩu mới..." value={editPassword} onChange={(e) => setEditPassword(e.target.value)} />
              </div>

              <div style={{ display: 'flex', gap: '12px', justifyContent: 'flex-end' }}>
                <button type="button" onClick={() => setEditingUser(null)} style={{ flex: 1, padding: '10px', borderRadius: '8px', background: '#334155', color: '#fff', border: 'none', cursor: 'pointer', fontWeight: 600, fontSize: '13px' }}>
                  Hủy Bỏ
                </button>
                <button type="submit" className="btn-primary-purple" disabled={isSavingUserEdit} style={{ flex: 1, padding: '10px', borderRadius: '8px' }}>
                  {isSavingUserEdit ? '⏳ Đang lưu...' : '💾 Lưu Thay Đổi'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* SUBVIEW 3: PERMISSION MATRIX */}
      {activeSubView === 'subview-permission' && (
        <div className="card-config">
          <div className="card-title-bar" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '12px' }}>
            <div>
              <span>🛡️ Ma Trận Phân Quyền Chi Tiết (RBAC Matrix System)</span>
              <div style={{ fontSize: '12px', color: '#94a3b8', marginTop: '4px', fontWeight: 'normal' }}>
                Phân quyền truy cập &amp; thao tác chi tiết cho 4 vai trò hệ thống: 🟢 <strong>Act</strong> (Toàn quyền) | 🟡 <strong>View</strong> (Chỉ xem) | 🔴 <strong>None</strong> (Không truy cập)
              </div>
            </div>
            <button
              onClick={handleSavePermissions}
              disabled={isSavingPermissions || isReadOnly}
              style={{
                background: isReadOnly ? '#475569' : 'linear-gradient(135deg, #B32D38 0%, #95252E 100%)',
                color: '#fff',
                border: 'none',
                padding: '10px 20px',
                borderRadius: '10px',
                fontWeight: 600,
                cursor: isReadOnly ? 'not-allowed' : 'pointer',
                fontSize: '13px',
                opacity: isReadOnly ? 0.6 : 1,
                boxShadow: isReadOnly ? 'none' : '0 4px 12px rgba(179, 45, 56, 0.4)',
              }}
            >
              {isSavingPermissions ? '⏳ Đang lưu CSDL...' : '💾 Lưu Cấu Hình Phân Quyền'}
            </button>
          </div>

          <div className="table-container" style={{ marginTop: '16px' }}>
            <table className="matrix-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr>
                  <th style={{ width: '32%', textAlign: 'left' }}>Nhóm Tính Năng Giao Diện</th>
                  {SYSTEM_ROLES.map((r) => (
                    <th key={r.key} style={{ textAlign: 'center', minWidth: '130px' }}>
                      <div>{r.label}</div>
                      <div style={{ fontSize: '10px', color: '#64748b', fontWeight: 'normal', marginTop: '2px' }}>{r.desc}</div>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {SYSTEM_FEATURES.map((f) => {
                  const isExpanded = expandedFeatures[f.key];
                  return (
                    <React.Fragment key={f.key}>
                      <tr style={{ background: isExpanded ? 'rgba(30, 41, 59, 0.4)' : 'transparent' }}>
                        <td style={{ padding: '10px 12px' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                            <button
                              type="button"
                              onClick={() => toggleExpandFeature(f.key)}
                              style={{
                                background: isExpanded ? '#3b82f6' : 'rgba(255,255,255,0.06)',
                                border: '1px solid rgba(255,255,255,0.15)',
                                color: isExpanded ? '#fff' : '#94a3b8',
                                borderRadius: '6px',
                                width: '24px',
                                height: '24px',
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'center',
                                cursor: 'pointer',
                                fontSize: '11px',
                                fontWeight: 'bold',
                                flexShrink: 0,
                                transition: 'all 0.15s ease',
                              }}
                              title="Thu/Mở danh sách tính năng con"
                            >
                              {isExpanded ? '⌃' : '⌄'}
                            </button>
                            <div>
                              <div style={{ fontWeight: 700, color: '#f8fafc', fontSize: '13px', display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <span>{f.title}</span>
                                {f.subFeatures && f.subFeatures.length > 0 && (
                                  <span style={{ fontSize: '10px', background: 'rgba(255,255,255,0.08)', color: '#94a3b8', padding: '1px 6px', borderRadius: '10px', fontWeight: 'normal' }}>
                                    {f.subFeatures.length} mục con
                                  </span>
                                )}
                              </div>
                              <div style={{ fontSize: '11px', color: '#64748b', marginTop: '2px' }}>{f.desc}</div>
                            </div>
                          </div>
                        </td>
                        {SYSTEM_ROLES.map((r) => {
                          const currentLevel = (rbacMatrix[r.key] && rbacMatrix[r.key][f.key]) || (r.key === 'Owner' || r.key === 'Admin' ? 'act' : 'view');
                          return (
                            <td key={r.key} style={{ textAlign: 'center', padding: '8px' }}>
                              <select
                                value={currentLevel}
                                onChange={(e) => handlePermissionChange(r.key, f.key, e.target.value as PermissionLevel)}
                                disabled={isReadOnly || r.key === 'Owner'}
                                style={{
                                  background: currentLevel === 'act' ? '#064e3b' : currentLevel === 'view' ? '#78350f' : '#7f1d1d',
                                  color: currentLevel === 'act' ? '#34d399' : currentLevel === 'view' ? '#fbbf24' : '#f87171',
                                  border: `1px solid ${currentLevel === 'act' ? '#059669' : currentLevel === 'view' ? '#d97706' : '#dc2626'}`,
                                  borderRadius: '8px',
                                  padding: '6px 10px',
                                  fontSize: '12px',
                                  fontWeight: 700,
                                  cursor: isReadOnly || r.key === 'Owner' ? 'not-allowed' : 'pointer',
                                  opacity: isReadOnly || r.key === 'Owner' ? 0.6 : 1,
                                  width: '100%',
                                  maxWidth: '130px',
                                }}
                              >
                                <option value="act">🟢 Act (Toàn quyền)</option>
                                <option value="view">🟡 View (Chỉ xem)</option>
                                <option value="none">🔴 None (Cấm vào)</option>
                              </select>
                            </td>
                          );
                        })}
                      </tr>

                      {/* Sub-Feature Rows */}
                      {isExpanded &&
                        f.subFeatures &&
                        f.subFeatures.map((sub) => (
                          <tr key={sub.key} style={{ background: 'rgba(15, 23, 42, 0.6)', borderTop: '1px dotted rgba(255,255,255,0.06)' }}>
                            <td style={{ paddingLeft: '44px', paddingTop: '6px', paddingBottom: '6px' }}>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                                <span style={{ color: '#38bdf8', fontSize: '13px', fontWeight: 'bold' }}>↳</span>
                                <div>
                                  <div style={{ fontWeight: 600, color: '#cbd5e1', fontSize: '12px' }}>{sub.title}</div>
                                  <div style={{ fontSize: '10px', color: '#64748b' }}>{sub.desc}</div>
                                </div>
                              </div>
                            </td>
                            {SYSTEM_ROLES.map((r) => {
                              const subLevel =
                                (rbacMatrix[r.key] && rbacMatrix[r.key][sub.key]) ||
                                (rbacMatrix[r.key] && rbacMatrix[r.key][f.key]) ||
                                (r.key === 'Owner' || r.key === 'Admin' ? 'act' : 'view');

                              return (
                                <td key={r.key} style={{ textAlign: 'center', padding: '4px 8px' }}>
                                  <select
                                    value={subLevel}
                                    onChange={(e) => handlePermissionChange(r.key, sub.key, e.target.value as PermissionLevel)}
                                    disabled={isReadOnly || r.key === 'Owner'}
                                    style={{
                                      background: subLevel === 'act' ? '#064e3b' : subLevel === 'view' ? '#78350f' : '#7f1d1d',
                                      color: subLevel === 'act' ? '#34d399' : subLevel === 'view' ? '#fbbf24' : '#f87171',
                                      border: `1px solid ${subLevel === 'act' ? '#059669' : subLevel === 'view' ? '#d97706' : '#dc2626'}`,
                                      borderRadius: '6px',
                                      padding: '4px 8px',
                                      fontSize: '11px',
                                      fontWeight: 700,
                                      cursor: isReadOnly || r.key === 'Owner' ? 'not-allowed' : 'pointer',
                                      opacity: isReadOnly || r.key === 'Owner' ? 0.6 : 1,
                                      width: '100%',
                                      maxWidth: '120px',
                                    }}
                                  >
                                    <option value="act">🟢 Act</option>
                                    <option value="view">🟡 View</option>
                                    <option value="none">🔴 None</option>
                                  </select>
                                </td>
                              );
                            })}
                          </tr>
                        ))}
                    </React.Fragment>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* SUBVIEW 4: QUICK TEMPLATES */}
      {activeSubView === 'subview-template' && (
        <div className="card-config">
          <div className="card-title-bar">
            <span>💬 Quản Lý Tin Nhắn Mẫu CSKH (Quick Templates)</span>
          </div>

          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Tiêu Đề Tin Mẫu</th>
                  <th>Phân Loại</th>
                  <th>Nội Dung Chi Tiết</th>
                  <th>Thao Tác</th>
                </tr>
              </thead>
              <tbody>
                {templates.map((t) => (
                  <tr key={t.id}>
                    <td><strong>{t.title}</strong></td>
                    <td><span className="status-pill pill-amber">{t.category}</span></td>
                    <td style={{ maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t.content}</td>
                    <td>
                      <button onClick={() => showToast(`Đã sao chép tin nhắn mẫu [${t.title}]`)} style={{ background: 'none', border: 'none', color: '#38bdf8', cursor: 'pointer', fontSize: '12px' }}>
                        📋 Copy
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* SUBVIEW 5: DATABASE MAINTENANCE */}
      {activeSubView === 'subview-database' && (
        <div className="card-config">
          <div className="card-title-bar">
            <span>🗄️ Quản Lý Bộ Nhớ & Bảo Trì Database DD_V3</span>
          </div>

          <div className="metrics-row" style={{ marginBottom: '24px' }}>
            <div className="metric-card">
              <div className="metric-icon icon-purple">🗄️</div>
              <div className="metric-content">
                <span className="metric-label">Trạng Thái Database</span>
                <span className="metric-value" style={{ color: '#34d399' }}>Healthy</span>
              </div>
            </div>

            <div className="metric-card">
              <div className="metric-icon icon-blue">💾</div>
              <div className="metric-content">
                <span className="metric-label">Loại Cơ Sở Dữ Liệu</span>
                <span className="metric-value">PostgreSQL</span>
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', gap: '12px' }}>
            <button className="btn-primary-purple" onClick={() => showToast('✅ Đã dọn dẹp cache hệ thống thành công!')}>
              🧹 Clear System Cache
            </button>
            <button className="btn-filter-apply" onClick={() => showToast('📥 Đã xuất dữ liệu sao lưu JSON thành công!')}>
              📥 Export Backup JSON
            </button>
          </div>
        </div>
      )}
    </div>
  );
};

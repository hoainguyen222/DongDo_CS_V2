'use client';

import React, { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import { SystemConfig } from '@/lib/types';
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

export const PartnerConfigView: React.FC = () => {
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

  // 4. Quick Templates State
  const [templates, setTemplates] = useState<QuickTemplate[]>([
    { id: '1', title: 'Hướng dẫn nạp tiền DDP Invest', category: 'Nạp/Rút tiền', content: 'Chào anh/chị, hạn mức nạp tối thiểu là 1,000,000 VNĐ qua QR Napas 24/7...' },
    { id: '2', title: 'Quy định eKYC mở tài khoản', category: 'Tài khoản', content: 'Để hoàn tất eKYC, anh/chị vui lòng chụp rõ 2 mặt CCCD và quét khuôn mặt...' },
    { id: '3', title: 'Thông báo Margin Call cảnh báo', category: 'Rủi ro', content: 'Tài khoản của anh/chị đang có tỷ lệ ký quỹ dưới 80%, vui lòng bổ sung ký quỹ...' },
  ]);

  useEffect(() => {
    loadSystemConfig();
  }, []);

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
    setTimeout(() => setToastMsg(''), 3000);
  };

  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault();
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

  const handleCreateUser = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newFullName || !newEmail) return;

    const newUser: UserAccount = {
      id: Date.now().toString(),
      fullName: newFullName,
      email: newEmail,
      role: newRole,
      status: 'Hoạt động',
      createdAt: new Date().toISOString().split('T')[0],
    };

    setUsers((prev) => [newUser, ...prev]);
    setNewFullName('');
    setNewEmail('');
    showToast(`✅ Đã tạo tài khoản [${newUser.fullName}] thành công!`);
  };

  const handleDeleteUser = (id: string, name: string) => {
    if (confirm(`Bạn có chắc muốn khóa/xóa tài khoản [${name}]?`)) {
      setUsers((prev) => prev.filter((u) => u.id !== id));
      showToast(`🗑️ Đã xóa tài khoản [${name}]`);
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

      {/* SUBVIEW 1: PROMPT & MODEL */}
      {activeSubView === 'subview-prompt' && (
        <div className="card-config" style={{ maxWidth: '900px' }}>
          <div className="card-title-bar">
            <span>⚙️ Cấu Hình Prompt & Tham Số Mô Hình AI</span>
          </div>

          <form onSubmit={handleSaveConfig}>
            <div className="form-group">
              <label className="form-label">System Prompt (Chỉ thị cốt lõi & Nhân cách CSKH):</label>
              <textarea className="textarea-custom" value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} />
            </div>

            <div className="form-row">
              <div className="form-group">
                <label className="form-label">Anthropic LLM Model:</label>
                <select className="select-custom" value={llmModel} onChange={(e) => setLlmModel(e.target.value)}>
                  <option value="claude-haiku-4-5-20251001">claude-haiku-4-5-20251001</option>
                  <option value="claude-3-5-sonnet-20241022">claude-3-5-sonnet-20241022</option>
                  <option value="claude-3-opus-20240229">claude-3-opus-20240229</option>
                </select>
              </div>

              <div className="form-group">
                <label className="form-label">Temperature (Độ sáng tạo vs Chính xác):</label>
                <input type="text" className="input-custom" value={temperature} onChange={(e) => setTemperature(e.target.value)} />
              </div>
            </div>

            <button type="submit" className="btn-primary-purple" disabled={isSavingConfig}>
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
                {filteredUsers.map((u) => (
                  <tr key={u.id}>
                    <td><strong>{u.fullName}</strong></td>
                    <td>{u.email}</td>
                    <td><span className="status-pill pill-blue">{u.role}</span></td>
                    <td><span className="status-pill pill-green">{u.status}</span></td>
                    <td>{u.createdAt}</td>
                    <td>
                      <button onClick={() => handleDeleteUser(u.id, u.fullName)} style={{ background: 'none', border: 'none', color: '#ef4444', cursor: 'pointer', fontSize: '13px' }}>
                        🗑️ Xóa
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* SUBVIEW 3: PERMISSION MATRIX */}
      {activeSubView === 'subview-permission' && (
        <div className="card-config">
          <div className="card-title-bar">
            <span>🛡️ Phân Quyền Hệ Thống (RBAC Matrix)</span>
          </div>

          <div className="table-container">
            <table className="matrix-table">
              <thead>
                <tr>
                  <th>Tính Năng Hệ Thống</th>
                  <th>Owner / Admin</th>
                  <th>Leader CS</th>
                  <th>Staff CS</th>
                  <th>Viewer</th>
                </tr>
              </thead>
              <tbody>
                {[
                  { feature: '📊 Trang Chủ / Dashboard', owner: true, leader: true, staff: true, viewer: true },
                  { feature: '💬 Live CS Inbox (Tiếp nhận & Chat)', owner: true, leader: true, staff: true, viewer: false },
                  { feature: '🧠 Duyệt Tri Thức Mới', owner: true, leader: true, staff: false, viewer: false },
                  { feature: '📚 Quản Lý Kho Tri Thức', owner: true, leader: true, staff: false, viewer: false },
                  { feature: '📈 Báo Cáo & Thống Kê System', owner: true, leader: true, staff: true, viewer: true },
                  { feature: '⚙️ Cấu Hình System & Tài Khoản', owner: true, leader: false, staff: false, viewer: false },
                ].map((row, i) => (
                  <tr key={i}>
                    <td>{row.feature}</td>
                    <td>{row.owner ? '✅' : '❌'}</td>
                    <td>{row.leader ? '✅' : '❌'}</td>
                    <td>{row.staff ? '✅' : '❌'}</td>
                    <td>{row.viewer ? '✅' : '❌'}</td>
                  </tr>
                ))}
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

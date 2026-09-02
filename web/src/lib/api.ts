import {
  AnalyticsStats,
  ChatCase,
  CustomerProfile,
  GuestSession,
  KnowledgeOverview,
  LearningItem,
  Message,
  QAPair,
  SessionUser,
  SystemConfig,
  SystemErrorItem,
  RolePermissionItem,
  PermissionLevel,
} from './types';

function getApiBase(): string {
  if (process.env.NEXT_PUBLIC_API_URL) {
    return process.env.NEXT_PUBLIC_API_URL;
  }
  if (typeof window !== 'undefined') {
    if (window.location.port === '3000') {
      return `${window.location.protocol}//${window.location.hostname}:8080`;
    }
    return window.location.origin;
  }
  return 'http://localhost:8080';
}

const API_BASE = getApiBase();

function getHeaders(token?: string): HeadersInit {
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
  };
  const authToken = token || (typeof window !== 'undefined' ? localStorage.getItem('dongdo_token') : null);
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`;
  }
  return headers;
}

export const api = {
  // Guest & Chat
  async registerGuest(displayName: string, phone?: string): Promise<GuestSession> {
    const res = await fetch(`${API_BASE}/guest/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ display_name: displayName, phone: phone || '' }),
    });
    if (!res.ok) throw new Error('Không thể tạo phiên khách hàng');
    return res.json();
  },

  async sendMessage(sessionID: string, customerName: string, message: string, clientMsgID?: string) {
    const res = await fetch(`${API_BASE}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: sessionID,
        customer_name: customerName,
        message,
        client_msg_id: clientMsgID,
      }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ detail: 'Lỗi gửi tin nhắn' }));
      throw new Error(err.detail || 'Lỗi gửi tin nhắn');
    }
    return res.json();
  },

  async getHistory(sessionID: string): Promise<{ session_id: string; status: string; assigned_cs: string; messages: Message[] }> {
    const res = await fetch(`${API_BASE}/history/${sessionID}`);
    if (!res.ok) throw new Error('Không thể tải lịch sử chat');
    return res.json();
  },

  async getCaseDetail(sessionID: string): Promise<{ session_id: string; status: string; assigned_cs: string; messages: Message[] }> {
    return this.getHistory(sessionID);
  },

  // Auth
  async login(username: string, password: string): Promise<SessionUser> {
    const res = await fetch(`${API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ detail: 'Đăng nhập thất bại' }));
      throw new Error(err.detail || 'Tên đăng nhập hoặc mật khẩu không chính xác');
    }
    const data = await res.json();
    if (typeof window !== 'undefined') {
      localStorage.setItem('dongdo_token', data.token);
      localStorage.setItem('dongdo_user', JSON.stringify(data));
    }
    return data;
  },

  async getMe(): Promise<SessionUser> {
    const res = await fetch(`${API_BASE}/auth/me`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Phiên đăng nhập hết hạn');
    return res.json();
  },

  async logout(): Promise<void> {
    await fetch(`${API_BASE}/auth/logout`, {
      method: 'POST',
      headers: getHeaders(),
    }).catch(() => {});
    if (typeof window !== 'undefined') {
      localStorage.removeItem('dongdo_token');
      localStorage.removeItem('dongdo_user');
    }
  },

  // Cases (CS Studio)
  async listCases(status?: string, page: number = 1, limit: number = 10, search?: string): Promise<{ cases: ChatCase[]; total: number; page: number; limit: number; total_pages: number }> {
    const params = new URLSearchParams();
    if (status) params.append('status', status);
    if (page) params.append('page', page.toString());
    if (limit) params.append('limit', limit.toString());
    if (search) params.append('search', search);

    const query = params.toString() ? `?${params.toString()}` : '';
    const res = await fetch(`${API_BASE}/api/admin/cases${query}`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tải danh sách case');
    const data = await res.json();
    return {
      cases: data.cases || [],
      total: data.total ?? (data.cases?.length || 0),
      page: data.page ?? page,
      limit: data.limit ?? limit,
      total_pages: data.total_pages ?? 1,
    };
  },

  async getCases(status?: string, page: number = 1, limit: number = 10, search?: string): Promise<ChatCase[]> {
    const data = await this.listCases(status, page, limit, search);
    return data.cases || [];
  },

  async takeCase(sessionID: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/cases/${sessionID}/take`, {
      method: 'POST',
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tiếp nhận case');
  },

  async replyCase(sessionID: string, message: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/cases/${sessionID}/reply`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({ message }),
    });
    if (!res.ok) throw new Error('Không thể gửi tin nhắn phản hồi');
  },

  async sendCSMessage(sessionID: string, message: string): Promise<void> {
    return this.replyCase(sessionID, message);
  },

  async resolveCase(sessionID: string, resolutionNote: string, extractPairs?: QAPair[]): Promise<{ success: boolean; auto_learned: boolean; learned_count: number; message: string }> {
    const res = await fetch(`${API_BASE}/api/admin/cases/${sessionID}/resolve`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({ resolution_note: resolutionNote, extract_pairs: extractPairs || [] }),
    });
    if (!res.ok) throw new Error('Không thể đóng case');
    return res.json();
  },

  async deleteCase(sessionID: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/cases/${sessionID}`, {
      method: 'DELETE',
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể xóa case');
  },

  async updateCustomerInfo(sessionID: string, customerName: string, customerPhone: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/cases/${sessionID}/customer`, {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify({ customer_name: customerName, customer_phone: customerPhone }),
    });
    if (!res.ok) throw new Error('Không thể cập nhật thông tin khách hàng');
  },

  // Customer Profiles Management
  async getCustomers(page: number = 1, limit: number = 10, search?: string): Promise<{ customers: CustomerProfile[]; total: number; page: number; limit: number; total_pages: number }> {
    const params = new URLSearchParams();
    if (page) params.append('page', page.toString());
    if (limit) params.append('limit', limit.toString());
    if (search) params.append('search', search);

    const query = params.toString() ? `?${params.toString()}` : '';
    const res = await fetch(`${API_BASE}/api/admin/customers${query}`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tải danh sách khách hàng');
    const data = await res.json();
    return {
      customers: data.customers || [],
      total: data.total ?? (data.customers?.length || 0),
      page: data.page ?? page,
      limit: data.limit ?? limit,
      total_pages: data.total_pages ?? 1,
    };
  },

  async updateCustomer(guestID: string, customerName: string, customerPhone: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/customers/${guestID}`, {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify({ customer_name: customerName, customer_phone: customerPhone }),
    });
    if (!res.ok) throw new Error('Không thể cập nhật thông tin khách hàng');
  },

  async deleteCustomer(guestID: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/customers/${guestID}`, {
      method: 'DELETE',
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể xóa khách hàng');
  },

  async getVoiceCalls(sessionID?: string, page: number = 1, limit: number = 10): Promise<{ calls: any[]; total: number; page: number; limit: number; total_pages: number }> {
    const params = new URLSearchParams();
    if (sessionID) params.append('session_id', sessionID);
    if (page) params.append('page', page.toString());
    if (limit) params.append('limit', limit.toString());

    const query = params.toString() ? `?${params.toString()}` : '';
    const res = await fetch(`${API_BASE}/api/admin/voice/calls${query}`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tải lịch sử cuộc gọi');
    const data = await res.json();
    return {
      calls: data.calls || [],
      total: data.total ?? (data.calls?.length || 0),
      page: data.page ?? page,
      limit: data.limit ?? limit,
      total_pages: data.total_pages ?? 1,
    };
  },

  async deleteVoiceCall(callID: number): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/voice/calls/${callID}`, {
      method: 'DELETE',
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể xóa lịch sử cuộc gọi này');
  },

  async clearAllCases(): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/cases/clear-all`, {
      method: 'POST',
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể xóa tất cả case');
  },

  // Learning Queue
  async listPendingLearning(page: number = 1, limit: number = 10): Promise<{ pending_items: LearningItem[]; total: number; page: number; limit: number; total_pages: number }> {
    const params = new URLSearchParams();
    if (page) params.append('page', page.toString());
    if (limit) params.append('limit', limit.toString());

    const query = params.toString() ? `?${params.toString()}` : '';
    const res = await fetch(`${API_BASE}/api/admin/learning/pending${query}`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tải danh sách tri thức chờ duyệt');
    const data = await res.json();
    return {
      pending_items: data.pending_items || data.items || [],
      total: data.total ?? (data.pending_items?.length || 0),
      page: data.page ?? page,
      limit: data.limit ?? limit,
      total_pages: data.total_pages ?? 1,
    };
  },

  async getPendingLearning(page: number = 1, limit: number = 10): Promise<LearningItem[]> {
    const data = await this.listPendingLearning(page, limit);
    return data.pending_items || [];
  },

  async approveLearning(itemID: number, question?: string, answer?: string): Promise<{ success: boolean; message: string }> {
    const res = await fetch(`${API_BASE}/api/admin/learning/approve/${itemID}`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({ question: question || '', answer: answer || '' }),
    });
    if (!res.ok) throw new Error('Không thể duyệt mẩu tri thức');
    return res.json();
  },

  async rejectLearning(itemID: number): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/learning/reject/${itemID}`, {
      method: 'POST',
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể từ chối mẩu tri thức');
  },

  async updateLearningItem(itemID: number, question: string, answer: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/learning/${itemID}`, {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify({ question, answer }),
    });
    if (!res.ok) throw new Error('Không thể cập nhật mẩu tri thức');
  },

  async getLearningSettings(): Promise<{ auto_learning_enabled: boolean }> {
    const res = await fetch(`${API_BASE}/api/admin/learning/settings`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tải cài đặt học tự động');
    return res.json();
  },

  async setLearningSettings(enabled: boolean): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/learning/settings`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({ auto_learning_enabled: enabled }),
    });
    if (!res.ok) throw new Error('Không thể cập nhật cài đặt học tự động');
  },

  async updateLearningSettings(enabled: boolean): Promise<void> {
    return this.setLearningSettings(enabled);
  },

  async resetLearnedKnowledge(): Promise<{ deleted_count: number }> {
    const res = await fetch(`${API_BASE}/api/admin/learning/reset`, {
      method: 'POST',
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể đặt lại tri thức đã học');
    return res.json();
  },

  async resetAllLearning(): Promise<void> {
    await this.resetLearnedKnowledge();
  },

  // Knowledge & Documents
  async getKnowledgeOverview(): Promise<KnowledgeOverview> {
    const res = await fetch(`${API_BASE}/api/admin/knowledge`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tải thông tin kho tri thức');
    return res.json();
  },

  async getKnowledge(): Promise<KnowledgeOverview> {
    return this.getKnowledgeOverview();
  },

  async uploadDocument(file: File): Promise<{ filename: string; message: string }> {
    const formData = new FormData();
    formData.append('file', file);
    const authToken = typeof window !== 'undefined' ? localStorage.getItem('dongdo_token') : null;
    const headers: HeadersInit = {};
    if (authToken) headers['Authorization'] = `Bearer ${authToken}`;

    const res = await fetch(`${API_BASE}/api/admin/knowledge/upload`, {
      method: 'POST',
      headers,
      body: formData,
    });
    if (!res.ok) throw new Error('Lỗi tải lên tài liệu .docx');
    return res.json();
  },

  // Analytics & Config
  async getAnalytics(): Promise<AnalyticsStats> {
    const res = await fetch(`${API_BASE}/api/admin/analytics`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tải thống kê');
    return res.json();
  },

  async getConfig(): Promise<SystemConfig> {
    const res = await fetch(`${API_BASE}/api/admin/config`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tải cấu hình hệ thống');
    return res.json();
  },

  async saveConfig(config: SystemConfig): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/config`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify(config),
    });
    if (!res.ok) throw new Error('Không thể lưu cấu hình hệ thống');
  },

  async updateConfig(config: SystemConfig): Promise<void> {
    return this.saveConfig(config);
  },

  // Role Permissions Management (RBAC)
  async listRolePermissions(): Promise<RolePermissionItem[]> {
    const res = await fetch(`${API_BASE}/api/admin/partner/config/permissions`, {
      headers: getHeaders(),
    });
    if (!res.ok) return [];
    const data = await res.json();
    return data.permissions || [];
  },

  async upsertRolePermission(roleName: string, featureKey: string, permissionLevel: PermissionLevel): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/partner/config/permissions`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({
        role_name: roleName,
        feature_key: featureKey,
        permission_level: permissionLevel,
        can_view: permissionLevel !== 'none',
        can_edit: permissionLevel === 'act',
      }),
    });
    if (!res.ok) throw new Error(`Không thể cập nhật phân quyền cho [${roleName}] - [${featureKey}]`);
  },

  // User Accounts Management
  async listUsers(): Promise<any[]> {
    const res = await fetch(`${API_BASE}/api/admin/users`, {
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể tải danh sách tài khoản');
    const data = await res.json();
    return (data.users || []).map((u: any) => {
      const rLower = (u.role || '').toLowerCase();
      let roleLabel = 'Staff CS';
      if (rLower.includes('owner')) roleLabel = 'Owner';
      else if (rLower.includes('admin')) roleLabel = 'Admin';
      else if (rLower.includes('leader')) roleLabel = 'Leader CS';

      return {
        id: u.id ? u.id.toString() : u.username,
        fullName: u.full_name || u.username,
        email: u.username,
        role: roleLabel,
        rawRole: u.role,
        status: u.is_active ? 'Hoạt động' : 'Tạm khóa',
        isActive: u.is_active,
        createdAt: u.created_at ? new Date(u.created_at).toISOString().split('T')[0] : 'N/A',
      };
    });
  },

  async createUser(fullName: string, email: string, role: string, password?: string): Promise<any> {
    const res = await fetch(`${API_BASE}/api/admin/users`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({ full_name: fullName, email, role, password: password || '12345678' }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: `Lỗi kết nối Backend (Mã HTTP: ${res.status})` }));
      const msg = err.detail || err.error || err.message || `Không thể tạo tài khoản (HTTP ${res.status})`;
      throw new Error(msg);
    }
    const u = await res.json();
    const rLower = (u.role || '').toLowerCase();
    let roleLabel = 'Staff CS';
    if (rLower.includes('owner')) roleLabel = 'Owner';
    else if (rLower.includes('admin')) roleLabel = 'Admin';
    else if (rLower.includes('leader')) roleLabel = 'Leader CS';

    return {
      id: u.id ? u.id.toString() : u.username,
      fullName: u.full_name || u.username,
      email: u.username,
      role: roleLabel,
      rawRole: u.role,
      status: u.is_active ? 'Hoạt động' : 'Tạm khóa',
      isActive: u.is_active,
      createdAt: u.created_at ? new Date(u.created_at).toISOString().split('T')[0] : new Date().toISOString().split('T')[0],
    };
  },

  async updateUser(username: string, data: { fullName: string; role: string; isActive: boolean; password?: string }): Promise<any> {
    const res = await fetch(`${API_BASE}/api/admin/users/${encodeURIComponent(username)}`, {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify({
        full_name: data.fullName,
        role: data.role,
        is_active: data.isActive,
        password: data.password || '',
      }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: `Lỗi kết nối Backend (Mã HTTP: ${res.status})` }));
      const msg = err.detail || err.error || err.message || `Không thể cập nhật tài khoản (HTTP ${res.status})`;
      throw new Error(msg);
    }
    const u = await res.json();
    const rLower = (u.role || '').toLowerCase();
    let roleLabel = 'Staff CS';
    if (rLower.includes('owner')) roleLabel = 'Owner';
    else if (rLower.includes('admin')) roleLabel = 'Admin';
    else if (rLower.includes('leader')) roleLabel = 'Leader CS';

    return {
      id: u.id ? u.id.toString() : u.username,
      fullName: u.full_name || u.username,
      email: u.username,
      role: roleLabel,
      rawRole: u.role,
      status: u.is_active ? 'Hoạt động' : 'Tạm khóa',
      isActive: u.is_active,
      createdAt: u.created_at ? new Date(u.created_at).toISOString().split('T')[0] : 'N/A',
    };
  },

  async deleteUser(username: string): Promise<void> {
    const res = await fetch(`${API_BASE}/api/admin/users/${username}`, {
      method: 'DELETE',
      headers: getHeaders(),
    });
    if (!res.ok) throw new Error('Không thể xóa tài khoản');
  },

  // System Errors Persistence API
  async listSystemErrors(): Promise<SystemErrorItem[]> {
    const res = await fetch(`${API_BASE}/api/admin/system-errors`, {
      headers: getHeaders(),
    });
    if (!res.ok) return [];
    const data = await res.json();
    return (data.errors || []).map((e: any) => ({
      id: e.id,
      source: e.source,
      title: e.title,
      details: e.details,
      severity: e.severity || 'high',
      isHandled: e.is_handled,
      suggestedFix: e.suggested_fix,
      timestamp: e.created_at ? new Date(e.created_at).toLocaleString('vi-VN') : '',
    }));
  },

  async createSystemError(errRecord: any): Promise<void> {
    await fetch(`${API_BASE}/api/admin/system-errors`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({
        id: errRecord.id,
        source: errRecord.source,
        title: errRecord.title,
        details: errRecord.details,
        severity: errRecord.severity || 'high',
        is_handled: errRecord.isHandled || false,
        suggested_fix: errRecord.suggestedFix || '',
      }),
    }).catch(() => {});
  },

  async markSystemErrorHandled(id: string): Promise<void> {
    await fetch(`${API_BASE}/api/admin/system-errors/${id}/handled`, {
      method: 'PUT',
      headers: getHeaders(),
    }).catch(() => {});
  },

  // Analytics & Reports APIs (7 Sub-reports)
  async getGeneralOverviewReport(startDate?: string, endDate?: string): Promise<any> {
    const params = new URLSearchParams();
    if (startDate) params.append('start_date', startDate);
    if (endDate) params.append('end_date', endDate);
    const res = await fetch(`${API_BASE}/api/admin/partner/reports/overview?${params.toString()}`, {
      headers: getHeaders(),
    });
    if (!res.ok) return null;
    return res.json();
  },

  async getAIPerformanceReport(startDate?: string, endDate?: string): Promise<any> {
    const params = new URLSearchParams();
    if (startDate) params.append('start_date', startDate);
    if (endDate) params.append('end_date', endDate);
    const res = await fetch(`${API_BASE}/api/admin/partner/reports/ai-perf?${params.toString()}`, {
      headers: getHeaders(),
    });
    if (!res.ok) return null;
    return res.json();
  },

  async getStaffPerformanceReport(startDate?: string, endDate?: string): Promise<any[]> {
    const params = new URLSearchParams();
    if (startDate) params.append('start_date', startDate);
    if (endDate) params.append('end_date', endDate);
    const res = await fetch(`${API_BASE}/api/admin/partner/reports/staff-perf?${params.toString()}`, {
      headers: getHeaders(),
    });
    if (!res.ok) return [];
    const data = await res.json();
    return data.staff_reports || [];
  },

  async getCXReport(startDate?: string, endDate?: string): Promise<any> {
    const params = new URLSearchParams();
    if (startDate) params.append('start_date', startDate);
    if (endDate) params.append('end_date', endDate);
    const res = await fetch(`${API_BASE}/api/admin/partner/reports/cx?${params.toString()}`, {
      headers: getHeaders(),
    });
    if (!res.ok) return null;
    return res.json();
  },

  async getOperationalReport(startDate?: string, endDate?: string): Promise<any[]> {
    const params = new URLSearchParams();
    if (startDate) params.append('start_date', startDate);
    if (endDate) params.append('end_date', endDate);
    const res = await fetch(`${API_BASE}/api/admin/partner/reports/operational?${params.toString()}`, {
      headers: getHeaders(),
    });
    if (!res.ok) return [];
    const data = await res.json();
    return data.hourly_load || [];
  },

  async getIssueAnalysisReport(startDate?: string, endDate?: string): Promise<any[]> {
    const params = new URLSearchParams();
    if (startDate) params.append('start_date', startDate);
    if (endDate) params.append('end_date', endDate);
    const res = await fetch(`${API_BASE}/api/admin/partner/reports/issues?${params.toString()}`, {
      headers: getHeaders(),
    });
    if (!res.ok) return [];
    const data = await res.json();
    return data.issues || [];
  },

  async getAILearningReportStats(): Promise<any> {
    const res = await fetch(`${API_BASE}/api/admin/partner/reports/ai-learning`, {
      headers: getHeaders(),
    });
    if (!res.ok) return null;
    return res.json();
  },
};

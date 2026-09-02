'use client';

import React, { useState } from 'react';
import { AlertTriangle, X, CheckCircle, ShieldAlert, Wrench, Clock, FileText, ChevronRight, Check } from 'lucide-react';
import { SystemErrorItem } from '@/lib/types';

interface ErrorCenterModalProps {
  isOpen: boolean;
  onClose: () => void;
  errors: SystemErrorItem[];
  onMarkAsHandled: (id: string) => void;
  onClearHandled: () => void;
}

export const generateSuggestedFix = (source: string, errorMsg: string): string => {
  const msgLower = (errorMsg || '').toLowerCase();
  const srcLower = (source || '').toLowerCase();

  if (msgLower.includes('401') || msgLower.includes('unauthorized') || msgLower.includes('hết hạn')) {
    return `🔑 **Nguyên nhân:** Phiên đăng nhập Admin/CSKH đã hết hạn hoặc Token xác thực không hợp lệ.
👉 **Đề xuất hướng xử lý:**
1. Bấm nút Đăng Xuất ở góc trên bên phải màn hình.
2. Đăng nhập lại bằng tài khoản Quản trị viên (Admin) để khởi tạo Token phiên làm việc mới.
3. Thực hiện lại thao tác vừa bị gián đoạn.`;
  }

  if (msgLower.includes('đã tồn tại') || msgLower.includes('already exists') || msgLower.includes('duplicate')) {
    return `⚠️ **Nguyên nhân:** Tên đăng nhập hoặc Email này đã tồn tại trong CSDL.
👉 **Đề xuất hướng xử lý:**
1. Kiểm tra lại thông tin Email vừa nhập trong biểu mẫu.
2. Đảm bảo không đặt trùng với các tài khoản CSKH/Admin đã có trong danh sách.
3. Nếu cần đổi mật khẩu tài khoản cũ, sử dụng chức năng Đặt lại mật khẩu thay vì tạo mới.`;
  }

  if (msgLower.includes('failed to connect') || msgLower.includes('networkerror') || msgLower.includes('csdl') || msgLower.includes('connection refused')) {
    return `🔌 **Nguyên nhân:** Lỗi kết nối đến Backend Server (Golang API / PostgreSQL / SQLite).
👉 **Đề xuất hướng xử lý:**
1. Kiểm tra xem tiến trình Golang Backend (cmd/server/main.go) có đang chạy trên port 8080 hay không.
2. Kiểm tra trạng thái dịch vụ PostgreSQL / Redis / Qdrant trong Docker Container.
3. Thử tải lại trang để thiết lập lại kết nối HTTP/WebSocket.`;
  }

  if (msgLower.includes('.docx') || msgLower.includes('word') || msgLower.includes('file')) {
    return `📄 **Nguyên nhân:** Tập tin nạp tri thức không hợp lệ hoặc lỗi định dạng.
👉 **Đề xuất hướng xử lý:**
1. Đảm bảo file được tải lên có định dạng đuôi **.docx** của Microsoft Word.
2. Kiểm tra xem file có bị đặt mật khẩu bảo vệ hoặc bị hư hỏng (corrupted) hay không.
3. Đảm bảo dung lượng file không vượt quá giới hạn hệ thống cho phép.`;
  }

  if (srcLower.includes('tài khoản') || srcLower.includes('user')) {
    return `👤 **Nguyên nhân:** Lỗi xảy ra trong quá trình quản lý tài khoản nhân viên.
👉 **Đề xuất hướng xử lý:**
1. Đảm bảo điền đầy đủ các trường thông tin bắt buộc (Họ tên, Email, Mật khẩu khởi tạo).
2. Kiểm tra quyền hạn của tài khoản Admin hiện tại.
3. Kiểm tra xem Email có chứa ký tự đặc biệt không hợp lệ hay không.`;
  }

  return `🔍 **Nguyên nhân:** Phát sinh ngoại lệ chưa xác định khi thực hiện thao tác [${source}].
👉 **Đề xuất hướng xử lý:**
1. Đọc kỹ chi tiết thông báo lỗi kỹ thuật được ghi nhận ở khung bên cạnh.
2. Thử F5 làm mới lại trang web và thực hiện lại thao tác.
3. Nếu sự cố vẫn tiếp diễn, sao chép mã lỗi chi tiết và gửi cho đội ngũ Kỹ thuật Backend để kiểm tra nhật ký log.`;
};

export const ErrorCenterModal: React.FC<ErrorCenterModalProps> = ({
  isOpen,
  onClose,
  errors,
  onMarkAsHandled,
  onClearHandled,
}) => {
  const [filter, setFilter] = useState<'unhandled' | 'all' | 'handled'>('unhandled');
  const [selectedErrorId, setSelectedErrorId] = useState<string | null>(null);

  if (!isOpen) return null;

  const filteredErrors = errors.filter((e) => {
    if (filter === 'unhandled') return !e.isHandled;
    if (filter === 'handled') return e.isHandled;
    return true;
  });

  const selectedError = errors.find((e) => e.id === selectedErrorId) || filteredErrors[0] || null;
  const unhandledCount = errors.filter((e) => !e.isHandled).length;

  return (
    <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto">
      <div className="w-full max-w-5xl bg-[#0D1527] border border-[#1C2D56] rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[85vh]">
        {/* Header */}
        <div className="px-6 py-4 border-b border-[#1C2D56] bg-[#0A0F1D] flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className="p-2.5 rounded-xl bg-rose-500/20 border border-rose-500/30 text-rose-400">
              <ShieldAlert className="w-5 h-5 animate-pulse" />
            </div>
            <div>
              <h2 className="text-base font-bold text-white flex items-center space-x-2">
                <span>Trung Tâm Cảnh Báo &amp; Xử Lý Lỗi Hệ Thống</span>
                {unhandledCount > 0 && (
                  <span className="px-2 py-0.5 rounded-full text-xs font-extrabold bg-rose-600 text-white animate-bounce">
                    {unhandledCount} lỗi mới
                  </span>
                )}
              </h2>
              <p className="text-xs text-slate-400">Theo dõi, xem chi tiết nguyên nhân và đề xuất hướng xử lý cho các lỗi phát sinh</p>
            </div>
          </div>

          <div className="flex items-center space-x-3">
            <div className="flex items-center space-x-1 p-1 bg-[#162344] rounded-xl border border-slate-700/50 text-xs">
              <button
                onClick={() => setFilter('unhandled')}
                className={`px-3 py-1.5 rounded-lg transition font-medium ${
                  filter === 'unhandled' ? 'bg-rose-600 text-white shadow' : 'text-slate-400 hover:text-white'
                }`}
              >
                Chưa xử lý ({unhandledCount})
              </button>
              <button
                onClick={() => setFilter('all')}
                className={`px-3 py-1.5 rounded-lg transition font-medium ${
                  filter === 'all' ? 'bg-[#1C2D56] text-white shadow' : 'text-slate-400 hover:text-white'
                }`}
              >
                Tất cả ({errors.length})
              </button>
              <button
                onClick={() => setFilter('handled')}
                className={`px-3 py-1.5 rounded-lg transition font-medium ${
                  filter === 'handled' ? 'bg-emerald-600 text-white shadow' : 'text-slate-400 hover:text-white'
                }`}
              >
                Đã xử lý ({errors.length - unhandledCount})
              </button>
            </div>

            <button
              onClick={onClose}
              className="p-2 rounded-xl text-slate-400 hover:text-white hover:bg-slate-800 transition"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Content Body: Split 2 Columns */}
        <div className="flex-1 flex flex-col md:flex-row min-h-0 divide-y md:divide-y-0 md:divide-x divide-[#1C2D56]">
          {/* Left Column: Error List */}
          <div className="w-full md:w-5/12 flex flex-col bg-[#0B1120] overflow-y-auto">
            {filteredErrors.length === 0 ? (
              <div className="flex-1 flex flex-col items-center justify-center p-8 text-center text-slate-400">
                <CheckCircle className="w-12 h-12 text-emerald-400 mb-3 opacity-80" />
                <p className="text-sm font-semibold text-slate-200">Không có lỗi nào ở trạng thái này</p>
                <p className="text-xs text-slate-500 mt-1">Hệ thống đang vận hành ổn định và không ghi nhận sự cố.</p>
              </div>
            ) : (
              <div className="divide-y divide-slate-800/60">
                {filteredErrors.map((errItem) => {
                  const isSelected = selectedError?.id === errItem.id;
                  return (
                    <div
                      key={errItem.id}
                      onClick={() => setSelectedErrorId(errItem.id)}
                      className={`p-4 cursor-pointer transition flex items-start justify-between ${
                        isSelected
                          ? 'bg-[#1C2D56]/80 border-l-4 border-rose-500'
                          : 'hover:bg-slate-800/40'
                      }`}
                    >
                      <div className="space-y-1.5 flex-1 min-w-0 pr-2">
                        <div className="flex items-center space-x-2">
                          <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${
                            errItem.severity === 'high' ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' :
                            errItem.severity === 'medium' ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30' :
                            'bg-sky-500/20 text-sky-300 border border-sky-500/30'
                          }`}>
                            {errItem.source}
                          </span>
                          <span className="text-[11px] text-slate-400 flex items-center">
                            <Clock className="w-3 h-3 mr-1" />
                            {errItem.timestamp}
                          </span>
                        </div>
                        <h4 className="text-xs font-bold text-slate-100 truncate">{errItem.title}</h4>
                        <p className="text-[11px] text-slate-400 truncate">{errItem.details}</p>
                      </div>

                      <div className="flex items-center space-x-1 shrink-0 pt-1">
                        {errItem.isHandled ? (
                          <span className="p-1 rounded-full bg-emerald-500/20 text-emerald-400" title="Đã xử lý">
                            <Check className="w-3.5 h-3.5" />
                          </span>
                        ) : (
                          <span className="w-2.5 h-2.5 rounded-full bg-rose-500 animate-pulse" title="Chưa xử lý" />
                        )}
                        <ChevronRight className="w-4 h-4 text-slate-500" />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Right Column: Error Details & Recommended Action */}
          <div className="w-full md:w-7/12 flex flex-col bg-[#0D1527] p-6 overflow-y-auto">
            {selectedError ? (
              <div className="space-y-5 flex-1 flex flex-col justify-between">
                <div className="space-y-5">
                  {/* Title & Metadata */}
                  <div className="border-b border-slate-800 pb-4">
                    <div className="flex items-center space-x-2 mb-2">
                      <span className="px-2.5 py-1 rounded-md bg-rose-500/20 border border-rose-500/30 text-rose-300 text-xs font-bold">
                        📌 {selectedError.source}
                      </span>
                      <span className="text-xs text-slate-400 flex items-center">
                        <Clock className="w-3.5 h-3.5 mr-1" />
                        {selectedError.timestamp}
                      </span>
                      {selectedError.isHandled ? (
                        <span className="px-2.5 py-1 rounded-md bg-emerald-500/20 border border-emerald-500/30 text-emerald-300 text-xs font-semibold">
                          ✅ Đã xử lý
                        </span>
                      ) : (
                        <span className="px-2.5 py-1 rounded-md bg-rose-600/30 border border-rose-500/40 text-rose-200 text-xs font-semibold animate-pulse">
                          ⚠️ Chưa xử lý
                        </span>
                      )}
                    </div>
                    <h3 className="text-base font-bold text-white">{selectedError.title}</h3>
                  </div>

                  {/* Technical Error Details */}
                  <div className="space-y-2">
                    <label className="text-xs font-bold text-slate-400 uppercase tracking-wider flex items-center space-x-1.5">
                      <FileText className="w-4 h-4 text-amber-400" />
                      <span>Chi Tiết Thông Báo Lỗi Kỹ Thuật (Raw Output):</span>
                    </label>
                    <div className="bg-[#050811] p-3.5 rounded-xl border border-slate-800 text-xs font-mono text-rose-300 break-all leading-relaxed max-h-40 overflow-y-auto selection:bg-rose-900 selection:text-white">
                      {selectedError.details || selectedError.title}
                    </div>
                  </div>

                  {/* Recommended Action Plan (Đề Xuất Hướng Xử Lý) */}
                  <div className="space-y-2.5 bg-[#141E36] p-4 rounded-xl border border-[#23386B] shadow-inner">
                    <label className="text-xs font-extrabold text-sky-400 uppercase tracking-wider flex items-center space-x-1.5">
                      <Wrench className="w-4 h-4 text-sky-400" />
                      <span>💡 Đề Xuất Hướng Xử Lý Cụ Thể (Recommended Solution):</span>
                    </label>
                    <div className="text-xs text-slate-200 leading-relaxed whitespace-pre-line font-sans space-y-1">
                      {selectedError.suggestedFix || generateSuggestedFix(selectedError.source, selectedError.details || selectedError.title)}
                    </div>
                  </div>
                </div>

                {/* Footer Action Buttons */}
                <div className="pt-4 border-t border-slate-800 flex items-center justify-between mt-auto">
                  {!selectedError.isHandled ? (
                    <button
                      onClick={() => onMarkAsHandled(selectedError.id)}
                      className="px-4 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs flex items-center space-x-2 transition shadow-lg shadow-emerald-900/30"
                    >
                      <CheckCircle className="w-4 h-4" />
                      <span>Đã khắc phục / Đánh dấu hoàn tất</span>
                    </button>
                  ) : (
                    <span className="text-xs text-emerald-400 font-semibold flex items-center">
                      <Check className="w-4 h-4 mr-1" />
                      Lỗi này đã được đánh dấu là đã xử lý
                    </span>
                  )}

                  <button
                    onClick={onClose}
                    className="px-4 py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 font-semibold text-xs transition"
                  >
                    Đóng
                  </button>
                </div>
              </div>
            ) : (
              <div className="flex-1 flex flex-col items-center justify-center text-slate-500 text-xs">
                Chọn một mục lỗi từ danh sách bên trái để xem thông tin chi tiết và đề xuất giải quyết.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

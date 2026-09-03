'use client';

import React, { useState } from 'react';
import { AlertTriangle, X, CheckCircle, ShieldAlert, Wrench, Clock, FileText, ChevronRight, Check } from 'lucide-react';
import type { SystemErrorItem } from '@/lib/types';
import styles from './ErrorCenterModal.module.scss';

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
    <div className={styles.backdrop}>
      <div className={styles.container}>
        <div className={styles.header}>
          <div className={styles.headerLeft}>
            <div className={styles.headerIcon}>
              <ShieldAlert style={{ width: 20, height: 20 }} />
            </div>
            <div>
              <h2 className={styles.headerTitle}>
                <span>Trung Tâm Cảnh Báo &amp; Xử Lý Lỗi Hệ Thống</span>
                {unhandledCount > 0 && (
                  <span className={styles.headerCounter}>{unhandledCount} lỗi mới</span>
                )}
              </h2>
              <p className={styles.headerSubtitle}>
                Theo dõi, xem chi tiết nguyên nhân và đề xuất hướng xử lý cho các lỗi phát sinh
              </p>
            </div>
          </div>

          <div className={styles.headerControls}>
            <div className={styles.filterGroup}>
              <button
                onClick={() => setFilter('unhandled')}
                className={`${styles.filterBtn} ${filter === 'unhandled' ? styles.filterBtnActive : ''}`}
              >
                Chưa xử lý ({unhandledCount})
              </button>
              <button
                onClick={() => setFilter('all')}
                className={`${styles.filterBtn} ${filter === 'all' ? styles.filterBtnActiveInfo : ''}`}
              >
                Tất cả ({errors.length})
              </button>
              <button
                onClick={() => setFilter('handled')}
                className={`${styles.filterBtn} ${filter === 'handled' ? styles.filterBtnActiveHandled : ''}`}
              >
                Đã xử lý ({errors.length - unhandledCount})
              </button>
            </div>

            <button onClick={onClose} className={styles.closeBtn} aria-label="Đóng">
              <X style={{ width: 20, height: 20 }} />
            </button>
          </div>
        </div>

        <div className={styles.body}>
          <div className={styles.sidebar}>
            {filteredErrors.length === 0 ? (
              <div className={styles.emptySidebar}>
                <CheckCircle />
                <p>Không có lỗi nào ở trạng thái này</p>
                <small>Hệ thống đang vận hành ổn định và không ghi nhận sự cố.</small>
              </div>
            ) : (
              filteredErrors.map((errItem) => {
                const isSelected = selectedError?.id === errItem.id;
                const sev = (errItem.severity ?? 'low') as 'low' | 'medium' | 'high';
                return (
                  <div
                    key={errItem.id}
                    onClick={() => setSelectedErrorId(errItem.id)}
                    className={`${styles.errorItem} ${isSelected ? styles.errorItemSelected : ''}`}
                  >
                    <div className={styles.errorItemMain}>
                      <div className={styles.errorItemHeader}>
                        <span className={`${styles.errorBadge} ${styles[`errorBadge${sev.charAt(0).toUpperCase()}${sev.slice(1)}`] ?? ''}`}>
                          {errItem.source}
                        </span>
                        <span className={styles.errorTimestamp}>
                          <Clock style={{ width: 12, height: 12 }} />
                          {errItem.timestamp}
                        </span>
                      </div>
                      <h4 className={styles.errorTitle}>{errItem.title}</h4>
                      <p className={styles.errorDetails}>{errItem.details}</p>
                    </div>

                    <div className={styles.errorItemRight}>
                      {errItem.isHandled ? (
                        <span className={styles.handledDot} title="Đã xử lý">
                          <Check style={{ width: 14, height: 14 }} />
                        </span>
                      ) : (
                        <span className={styles.unhandledDot} title="Chưa xử lý" />
                      )}
                      <ChevronRight className={styles.chevron} />
                    </div>
                  </div>
                );
              })
            )}
          </div>

          <div className={styles.main}>
            {selectedError ? (
              <div className={styles.detail}>
                <div className={styles.detailHeader}>
                  <div className={styles.detailMeta}>
                    <span className={styles.detailMetaBadge}>📌 {selectedError.source}</span>
                    <span className={styles.errorTimestamp}>
                      <Clock style={{ width: 14, height: 14 }} />
                      {selectedError.timestamp}
                    </span>
                    {selectedError.isHandled ? (
                      <span className={styles.detailStatusHandled}>✅ Đã xử lý</span>
                    ) : (
                      <span className={styles.detailStatusUnresolved}>⚠️ Chưa xử lý</span>
                    )}
                  </div>
                  <h3 className={styles.detailTitle}>{selectedError.title}</h3>
                </div>

                <div>
                  <label className={styles.detailSectionLabel}>
                    <FileText style={{ width: 16, height: 16, color: '#fbbf24' }} />
                    <span>Chi Tiết Thông Báo Lỗi Kỹ Thuật (Raw Output):</span>
                  </label>
                  <div className={styles.detailRaw}>
                    {selectedError.details || selectedError.title}
                  </div>
                </div>

                <div className={styles.solutionBox}>
                  <label className={styles.solutionLabel}>
                    <Wrench style={{ width: 16, height: 16 }} />
                    <span>💡 Đề Xuất Hướng Xử Lý Cụ Thể (Recommended Solution):</span>
                  </label>
                  <div className={styles.solutionBody}>
                    {selectedError.suggestedFix || generateSuggestedFix(selectedError.source, selectedError.details || selectedError.title)}
                  </div>
                </div>

                <div className={styles.detailActions}>
                  {!selectedError.isHandled ? (
                    <button
                      onClick={() => onMarkAsHandled(selectedError.id)}
                      className={styles.markHandledBtn}
                    >
                      <CheckCircle style={{ width: 16, height: 16 }} />
                      <span>Đã khắc phục / Đánh dấu hoàn tất</span>
                    </button>
                  ) : (
                    <span className={styles.handledLabel}>
                      <Check style={{ width: 16, height: 16, marginRight: 4 }} />
                      Lỗi này đã được đánh dấu là đã xử lý
                    </span>
                  )}

                  <button onClick={onClose} className={styles.detailClose}>
                    Đóng
                  </button>
                </div>
              </div>
            ) : (
              <div className={styles.detailEmpty}>
                Chọn một mục lỗi từ danh sách bên trái để xem thông tin chi tiết và đề xuất giải quyết.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

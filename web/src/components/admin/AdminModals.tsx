'use client';

import React, { useState } from 'react';
import {
  X,
  XCircle,
  Headphones,
  RefreshCw,
  Phone,
} from 'lucide-react';
import { ResolveCaseForm } from '@/components/forms/ResolveCaseForm';
import { CustomerEditForm } from '@/components/forms/CustomerEditForm';
import { ErrorCenterModal } from '@/components/ErrorCenterModal';
import { useUIStore } from '@/lib/stores/uiStore';
import type { ChatCase, CustomerProfile, QAPair, Message, SystemErrorItem } from '@/lib/types';
import styles from './AdminModals.module.scss';
import layoutStyles from '@/app/admin/AdminLayout.module.scss';

// ── Resolve Case Modal ───────────────────────────────────────
export function ResolveCaseModal({
  isOpen,
  selectedCase,
  caseMessages,
  autoLearnEnabled,
  resolutionNote,
  onResolutionNoteChange,
  onSubmit,
  onClose,
  isLoading,
}: {
  isOpen: boolean;
  selectedCase: ChatCase | null;
  caseMessages: Message[];
  autoLearnEnabled: boolean;
  resolutionNote: string;
  onResolutionNoteChange: (v: string) => void;
  onSubmit: (data: { qaPairs: QAPair[] }, note: string, enableLearn: boolean) => Promise<void>;
  onClose: () => void;
  isLoading: boolean;
}) {
  if (!isOpen || !selectedCase) return null;

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

  const initialPairs = extractAllQAPairs(caseMessages);
  const [modalEnableLearn, setModalEnableLearn] = useState(true);

  return (
    <div className={styles.backdrop}>
      <div className={`${styles.container} ${styles.sizeLg}`}>
        <div className={styles.header}>
          <h3 className={styles.headerTitle}>
            <span>🎯</span>
            <span>Đóng &amp; Giải Quyết Ca Hỗ Trợ</span>
          </h3>
          <button onClick={onClose} className={styles.closeBtn} aria-label="Đóng">
            <XCircle style={{ width: 20, height: 20 }} />
          </button>
        </div>
        <ResolveCaseForm
          enableLearn={modalEnableLearn}
          onEnableLearnChange={setModalEnableLearn}
          initialPairs={initialPairs.length > 0 ? initialPairs : [{ question: '', answer: '' }]}
          autoLearnEnabled={autoLearnEnabled}
          resolutionNote={resolutionNote}
          onResolutionNoteChange={onResolutionNoteChange}
          onSubmit={(data) => onSubmit(data, resolutionNote, modalEnableLearn)}
          isLoading={isLoading}
          caseSessionId={selectedCase.session_id}
          customerName={selectedCase.customer_name}
        />
      </div>
    </div>
  );
}

// ── Edit Customer Modal ──────────────────────────────────────
export function EditCustomerModal({
  isOpen,
  customer,
  onSubmit,
  onClose,
  isLoading,
}: {
  isOpen: boolean;
  customer: CustomerProfile | null;
  onSubmit: (data: { displayName: string; phone?: string }) => Promise<void>;
  onClose: () => void;
  isLoading: boolean;
}) {
  if (!isOpen || !customer) return null;

  return (
    <div className={styles.backdrop}>
      <div className={`${styles.container} ${styles.sizeSm}`}>
        <div className={styles.header}>
          <h3 className={styles.headerTitle}>
            <span>✏️</span>
            <span>Chỉnh Sửa Thông Tin Khách Hàng</span>
          </h3>
          <button onClick={onClose} className={styles.closeBtn} aria-label="Đóng">
            <XCircle style={{ width: 20, height: 20 }} />
          </button>
        </div>
        <div className={styles.body}>
          <CustomerEditForm
            defaultValues={{
              displayName: customer.display_name || '',
              phone: customer.phone || '',
            }}
            onSubmit={onSubmit}
            isLoading={isLoading}
          />
        </div>
      </div>
    </div>
  );
}

// ── Voice History Modal ──────────────────────────────────────
export function VoiceHistoryModal({
  isOpen,
  calls,
  isLoading,
  onClose,
}: {
  isOpen: boolean;
  calls: any[];
  isLoading: boolean;
  onClose: () => void;
}) {
  if (!isOpen) return null;

  return (
    <div className={styles.backdrop}>
      <div className={`${styles.container} ${styles.sizeLg}`}>
        <div className={styles.header}>
          <div className={styles.titleGroup}>
            <div className={styles.titleBadge}>
              <Headphones style={{ width: 20, height: 20 }} />
            </div>
            <div className={styles.titleText}>
              <h3 className={styles.headerTitle}>🎧 Lịch Sử Cuộc Gọi Thoại</h3>
              <p className={styles.titleSubtext}>Danh sách cuộc đàm thoại</p>
            </div>
          </div>
          <button onClick={onClose} className={styles.closeBtn} aria-label="Đóng">
            <XCircle style={{ width: 20, height: 20 }} />
          </button>
        </div>

        <div className={styles.body}>
          {isLoading ? (
            <div className={styles.loadingState}>
              <RefreshCw style={{ width: 24, height: 24, color: '#34d399' }} />
              <span>Đang tải...</span>
            </div>
          ) : calls.length === 0 ? (
            <div className={styles.emptyState}>
              <Headphones />
              <div>Chưa có dữ liệu cuộc gọi.</div>
            </div>
          ) : (
            <div className={styles.voiceList}>
              {calls.map((call: any) => (
                <div key={call.id} className={styles.voiceItem}>
                  <div className={styles.voiceItemHeader}>
                    <div className={styles.voiceItemCaller}>
                      <span>
                        {call.caller_type === 'guest' ? '👤 Khách hàng' : '🎧 CSKH'} ({call.caller_id})
                      </span>
                      <span style={{ color: '#64748b' }}>➡️</span>
                      <span style={{ color: '#e2e8f0' }}>
                        {call.callee_type === 'cskh' ? '🎧 CSKH' : '👤 Khách'}
                      </span>
                    </div>
                    <span className={styles.voiceItemTime}>
                      {new Date(call.created_at).toLocaleString('vi-VN')}
                    </span>
                  </div>
                  <div className={styles.voiceItemMeta}>
                    <span>⏱️ {call.duration_seconds}s</span>
                    <span className={styles.voiceMetaStatus}>{call.status}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Error Center Modal Wrapper ───────────────────────────────
export function ErrorCenterWrapper({
  errors,
  onMarkAsHandled,
}: {
  errors: SystemErrorItem[];
  onMarkAsHandled: (id: string) => void;
}) {
  const { showErrorCenter, setShowErrorCenter, clearHandledErrors } = useUIStore();

  if (!showErrorCenter) return null;

  return (
    <ErrorCenterModal
      isOpen={showErrorCenter}
      onClose={() => setShowErrorCenter(false)}
      errors={errors}
      onMarkAsHandled={onMarkAsHandled}
      onClearHandled={() => clearHandledErrors()}
    />
  );
}

// ── Toast Error Banner ───────────────────────────────────────
export function ToastErrorBanner({
  error,
  onClose,
  onViewDetails,
}: {
  error: { title: string; source: string; details: string } | null;
  onClose: () => void;
  onViewDetails: () => void;
}) {
  if (!error) return null;

  return (
    <div className={layoutStyles.toastBanner}>
      <div className={layoutStyles.toastIcon}>
        <Phone style={{ width: 20, height: 20 }} />
      </div>
      <div className={layoutStyles.toastBody}>
        <div className={layoutStyles.toastTitle}>⚠️ Lỗi [{error.source}]</div>
        <div className={layoutStyles.toastHeading}>{error.title}</div>
        <div className={layoutStyles.toastDetails}>{error.details}</div>
        <button onClick={onViewDetails} className={layoutStyles.toastDetailLink}>
          👉 Xem chi tiết &amp; Hướng xử lý
        </button>
      </div>
      <button
        onClick={onClose}
        className={layoutStyles.toastCloseBtn}
        aria-label="Đóng thông báo"
      >
        <X style={{ width: 16, height: 16 }} />
      </button>
    </div>
  );
}

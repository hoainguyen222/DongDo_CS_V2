'use client';

import React, { useState } from 'react';
import { Brain, RefreshCw } from 'lucide-react';
import {
  usePendingLearning,
  useLearningSettings,
  useUpdateLearningSettings,
  useApproveLearning,
  useRejectLearning,
  useUpdateLearningItem,
  useResetAllLearning,
} from '@/lib/hooks/useApi';
import { Pagination } from '@/components/admin/AdminSidebar';
import { useUIStore } from '@/lib/stores/uiStore';
import styles from '@/components/admin/AdminPage.module.scss';
import local from './page.local.module.scss';

export default function LearningPage() {
  const { addToast, openConfirm } = useUIStore();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editQ, setEditQ] = useState('');
  const [editA, setEditA] = useState('');

  const { data: learningData, isLoading } = usePendingLearning(page, pageSize);
  const { data: settingsData } = useLearningSettings();
  const updateSettingsMutation = useUpdateLearningSettings();
  const approveLearningMutation = useApproveLearning();
  const rejectLearningMutation = useRejectLearning();
  const updateLearningMutation = useUpdateLearningItem();
  const resetLearningMutation = useResetAllLearning();

  const items = learningData?.pending_items ?? [];
  const total = learningData?.total ?? 0;
  const autoLearnEnabled = settingsData?.auto_learning_enabled ?? false;

  const handleToggleAutoLearn = async (enabled: boolean) => {
    try {
      await updateSettingsMutation.mutateAsync(enabled);
      addToast({ title: enabled ? 'Đã bật' : 'Đã tắt', variant: 'success' });
    } catch (err: any) {
      addToast({ title: err.message, variant: 'error' });
    }
  };

  const handleApprove = async (id: number) => {
    try {
      await approveLearningMutation.mutateAsync({ itemId: id });
      addToast({ title: 'Đã duyệt', variant: 'success' });
    } catch (err: any) {
      addToast({ title: err.message, variant: 'error' });
    }
  };

  const handleReject = async (id: number) => {
    try {
      await rejectLearningMutation.mutateAsync(id);
      addToast({ title: 'Đã từ chối', variant: 'info' });
    } catch (err: any) {
      addToast({ title: err.message, variant: 'error' });
    }
  };

  const handleStartEdit = (item: any) => {
    setEditQ(item.question);
    setEditA(item.answer);
    setEditingId(item.id);
  };

  const handleSaveEdit = async (id: number, question: string, answer: string) => {
    try {
      await updateLearningMutation.mutateAsync({ itemId: id, question, answer });
      setEditingId(null);
      addToast({ title: 'Đã cập nhật', variant: 'success' });
    } catch (err: any) {
      addToast({ title: err.message, variant: 'error' });
    }
  };

  const handleReset = () => {
    openConfirm({
      title: 'Đặt lại tri thức?',
      message: 'Đặt lại toàn bộ tri thức đã học?',
      confirmText: 'Đặt lại',
      variant: 'danger',
      onConfirm: async () => {
        try {
          await resetLearningMutation.mutateAsync();
          addToast({ title: 'Đã reset', variant: 'success' });
        } catch (err: any) {
          addToast({ title: err.message, variant: 'error' });
        }
      },
    });
  };

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.headerLeft}>
          <div className={`${styles.headerIcon} ${styles.headerIconAmber}`}>
            <Brain style={{ width: 20, height: 20 }} />
          </div>
          <div>
            <h2 className={styles.headerTitle}>Học Tri Thức Mới</h2>
            <p className={styles.headerSubtitle}>Phê duyệt mẫu Q&amp;A từ cuộc trò chuyện</p>
          </div>
        </div>
        <div className={styles.headerActions}>
          <label className={local.autoToggle}>
            <input
              type="checkbox"
              checked={autoLearnEnabled}
              onChange={(e) => handleToggleAutoLearn(e.target.checked)}
            />
            <span>Tự động dạy AI</span>
          </label>
          <button onClick={handleReset} className={styles.btnDanger}>🔄 Reset</button>
        </div>
      </div>

      <div className={local.stack}>
        {isLoading ? (
          <div className={`${styles.card} ${styles.cardPad}`} style={{ textAlign: 'center', color: '#94a3b8' }}>
            <RefreshCw className={styles.spinIcon} style={{ display: 'inline', marginRight: 8 }} />
            Đang tải...
          </div>
        ) : items.length === 0 ? (
          <div className={`${styles.card} ${styles.cardPad}`} style={{ textAlign: 'center', color: '#94a3b8' }}>
            Không có mẫu tri thức nào chờ duyệt.
          </div>
        ) : (
          items.map((item: any) => (
            <div key={item.id} className={`${styles.card} ${styles.cardPad} ${local.item}`}>
              {editingId === item.id ? (
                <>
                  <input
                    type="text"
                    value={editQ}
                    onChange={(e) => setEditQ(e.target.value)}
                    className={styles.searchInput}
                    placeholder="Câu hỏi..."
                  />
                  <textarea
                    rows={3}
                    value={editA}
                    onChange={(e) => setEditA(e.target.value)}
                    className={styles.searchInput}
                    style={{ resize: 'vertical', minHeight: 80 }}
                    placeholder="Câu trả lời..."
                  />
                  <div className={local.itemActions}>
                    <button onClick={() => handleSaveEdit(item.id, editQ, editA)} className={styles.btnSuccess}>
                      💾 Lưu
                    </button>
                    <button onClick={() => setEditingId(null)} className={styles.btnGhost}>Hủy</button>
                  </div>
                </>
              ) : (
                <div className={local.itemRow}>
                  <div className={local.itemBody}>
                    <div className={local.itemQuestion}>❓ {item.question}</div>
                    <div className={local.itemAnswer}>💡 {item.answer}</div>
                  </div>
                  <div className={local.itemButtons}>
                    <button onClick={() => handleStartEdit(item)} className={`${styles.btnGhost} ${styles.btnSm}`}>✏️</button>
                    <button onClick={() => handleApprove(item.id)} className={`${styles.btnSuccess} ${styles.btnSm}`}>✅</button>
                    <button onClick={() => handleReject(item.id)} className={`${styles.btnDanger} ${styles.btnSm}`}>✕</button>
                  </div>
                </div>
              )}
            </div>
          ))
        )}
      </div>

      {total > pageSize && (
        <div className="mt-4">
          <Pagination
            currentPage={page}
            pageSize={pageSize}
            totalItems={total}
            onPageChange={setPage}
            onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
          />
        </div>
      )}
    </div>
  );
}

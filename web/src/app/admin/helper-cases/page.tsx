'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Bell, RefreshCw, CheckCircle2, UserCheck, X, ExternalLink, Clock, MessageSquare } from 'lucide-react';
import { useHelperCases, useProcessHelperCase } from '@/lib/hooks/useApi';
import { useAuthStore } from '@/lib/stores/authStore';
import { useUIStore } from '@/lib/stores/uiStore';
import type { ChatCase, User } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import styles from './page.module.scss';

export default function HelperCasesPage() {
  const router = useRouter();
  const { user } = useAuthStore();
  const { addToast } = useUIStore();

  const isLeaderOrAbove =
    user?.role === 'owner' || user?.role === 'admin' || user?.role === 'leader';

  const { data: helperData, isLoading, refetch } = useHelperCases();
  const processMutation = useProcessHelperCase();

  // Fetch list of staff users for transfer modal
  const { data: usersData } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.listUsers(),
    enabled: isLeaderOrAbove,
  });

  const [transferCase, setTransferCase] = useState<ChatCase | null>(null);
  const [selectedTargetUser, setSelectedTargetUser] = useState<string>('');

  const cases = helperData?.cases || [];
  const staffUsers = (usersData || []).filter(
    (u: any) => u.isActive && (u.rawRole === 'cskh' || u.rawRole === '' || u.role?.includes('Staff CS'))
  );

  const handleTakeOver = async (c: ChatCase) => {
    try {
      await processMutation.mutateAsync({
        sessionId: c.session_id,
        action: 'take_over',
      });
      addToast({
        title: 'Thành công',
        message: `Đã tiếp nhận xử lý trực tiếp cuộc hội thoại ${c.customer_name}`,
        variant: 'success',
      });
      router.push(`/admin/cases/${c.session_id}`);
    } catch (err: any) {
      addToast({
        title: 'Lỗi xử lý',
        message: err.message || 'Không thể tiếp nhận case',
        variant: 'error',
      });
    }
  };

  const handleConfirmTransfer = async () => {
    if (!transferCase || !selectedTargetUser) return;
    try {
      await processMutation.mutateAsync({
        sessionId: transferCase.session_id,
        action: 'transfer',
        targetUsername: selectedTargetUser,
      });
      addToast({
        title: 'Chuyển giao thành công',
        message: `Đã chuyển giao hội thoại cho chuyên viên ${selectedTargetUser}`,
        variant: 'success',
      });
      setTransferCase(null);
      setSelectedTargetUser('');
    } catch (err: any) {
      addToast({
        title: 'Lỗi chuyển giao',
        message: err.message || 'Không thể chuyển giao case',
        variant: 'error',
      });
    }
  };

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <div className={styles.titleBlock}>
          <div className={styles.iconBox}>
            <Bell style={{ width: 24, height: 24 }} />
          </div>
          <div>
            <h1>Danh Sách Case Cần Hỗ Trợ (Helper)</h1>
            <p>Tổng hợp các cuộc hội thoại được Chuyên viên CSKH phát yêu cầu trợ giúp</p>
          </div>
        </div>

        <button onClick={() => refetch()} className={styles.refreshBtn}>
          <RefreshCw style={{ width: 14, height: 14 }} />
          <span>Làm mới ({cases.length})</span>
        </button>
      </header>

      <div className={styles.card}>
        {isLoading ? (
          <div className={styles.emptyState}>
            <p>Đang tải danh sách case cần hỗ trợ...</p>
          </div>
        ) : cases.length === 0 ? (
          <div className={styles.emptyState}>
            <Bell style={{ width: 40, height: 40, color: '#475569', margin: '0 auto' }} />
            <p>Hiện tại không có cuộc hội thoại nào yêu cầu hỗ trợ.</p>
          </div>
        ) : (
          <table className={styles.table}>
            <thead>
              <tr>
                <th style={{ width: '30%' }}>Đoạn Hội Thoại</th>
                <th style={{ width: '45%' }}>Nội Dung Cần Hỗ Trợ</th>
                <th style={{ width: '25%' }}>Hành Động</th>
              </tr>
            </thead>
            <tbody>
              {cases.map((c) => (
                <tr key={c.session_id}>
                  <td>
                    <Link href={`/admin/cases/${c.session_id}`} className={styles.conversationLink}>
                      <div className={styles.customerName}>
                        <MessageSquare style={{ width: 16, height: 16, color: '#38bdf8' }} />
                        <span>{c.customer_name}</span>
                        <ExternalLink style={{ width: 12, height: 12, opacity: 0.6 }} />
                      </div>
                      <div className={styles.sessionMeta}>
                        Phiên: {c.session_id} {c.customer_phone ? `• SĐT: ${c.customer_phone}` : ''}
                      </div>
                      {c.help_requested_at && (
                        <div className={styles.timeMeta}>
                          <Clock style={{ width: 12, height: 12 }} />
                          <span>
                            {new Date(c.help_requested_at).toLocaleString('vi-VN')}
                          </span>
                        </div>
                      )}
                    </Link>
                  </td>

                  <td>
                    <div className={styles.helpContentBox}>
                      <div className={styles.helpText}>
                        {c.help_content || 'Không có ghi chú chi tiết'}
                      </div>
                      <div className={styles.requestingBadge}>
                        <span>Yêu cầu bởi:</span>
                        <strong>{c.help_requested_by || c.assigned_cs || 'Staff'}</strong>
                      </div>
                    </div>
                  </td>

                  <td>
                    <div className={styles.actionCell}>
                      {isLeaderOrAbove ? (
                        <>
                          <button
                            onClick={() => handleTakeOver(c)}
                            disabled={processMutation.isPending}
                            className={styles.btnTakeOver}
                            title="Xử lý trực tiếp hội thoại này"
                          >
                            <CheckCircle2 style={{ width: 14, height: 14 }} />
                            <span>Xử lý</span>
                          </button>
                          <button
                            onClick={() => {
                              setTransferCase(c);
                              if (staffUsers.length > 0) {
                                setSelectedTargetUser(staffUsers[0].email);
                              }
                            }}
                            disabled={processMutation.isPending}
                            className={styles.btnTransfer}
                            title="Chuyển giao cho Chuyên viên khác"
                          >
                            <UserCheck style={{ width: 14, height: 14 }} />
                            <span>Chuyển giao</span>
                          </button>
                        </>
                      ) : (
                        <span className={styles.readOnlyBadge}>Chỉ xem</span>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Transfer Modal */}
      {transferCase && (
        <div className={styles.modalOverlay} onClick={() => setTransferCase(null)}>
          <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <h3>Chuyển Giao Hội Thoại Hỗ Trợ</h3>
              <button onClick={() => setTransferCase(null)} className={styles.closeBtn}>
                <X style={{ width: 18, height: 18 }} />
              </button>
            </div>

            <div className={styles.formGroup}>
              <label>Chọn Chuyên viên CSKH tiếp nhận:</label>
              <select
                value={selectedTargetUser}
                onChange={(e) => setSelectedTargetUser(e.target.value)}
              >
                {staffUsers.map((u: any) => (
                  <option key={u.email} value={u.email}>
                    {u.fullName} (@{u.email})
                  </option>
                ))}
              </select>
            </div>

            <div className={styles.modalActions}>
              <button onClick={() => setTransferCase(null)} className={styles.btnCancel}>
                Hủy
              </button>
              <button
                onClick={handleConfirmTransfer}
                disabled={!selectedTargetUser || processMutation.isPending}
                className={styles.btnConfirm}
              >
                Xác nhận chuyển
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

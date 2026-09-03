'use client';

import React, { useState } from 'react';
import { Headphones, Phone, RefreshCw, XCircle, Search } from 'lucide-react';
import { useVoiceCalls, useDeleteVoiceCall } from '@/lib/hooks/useApi';
import { Pagination } from '@/components/admin/AdminSidebar';
import { useUIStore } from '@/lib/stores/uiStore';
import styles from '@/components/admin/AdminPage.module.scss';

export default function CallsPage() {
  const { addToast } = useUIStore();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [searchTerm, setSearchTerm] = useState('');

  const { data, isLoading } = useVoiceCalls();
  const deleteVoiceMutation = useDeleteVoiceCall();

  const calls = data?.calls ?? [];
  const total = data?.total ?? 0;

  const filteredCalls = calls.filter((call: any) => {
    if (!searchTerm) return true;
    const term = searchTerm.toLowerCase();
    return (
      call.caller_id?.toLowerCase().includes(term) ||
      call.session_id?.toLowerCase().includes(term)
    );
  });

  const handleDelete = async (id: number) => {
    try {
      await deleteVoiceMutation.mutateAsync(id);
      addToast({ title: 'Đã xóa bản ghi cuộc gọi', variant: 'success' });
    } catch (err: any) {
      addToast({ title: err.message || 'Lỗi', variant: 'error' });
    }
  };

  const endedCount = calls.filter((c: any) => c.status === 'ENDED').length;
  const missedCount = calls.filter((c: any) => c.status === 'MISSED').length;
  const activeCount = calls.filter((c: any) => c.status === 'ACTIVE' || c.status === 'RINGING').length;

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.headerLeft}>
          <div className={`${styles.headerIcon} ${styles.headerIconCyan}`}>
            <Headphones style={{ width: 20, height: 20 }} />
          </div>
          <div>
            <h2 className={styles.headerTitle}>Lịch Sử Cuộc Gọi</h2>
            <p className={styles.headerSubtitle}>Danh sách cuộc đàm thoại WebRTC</p>
          </div>
        </div>
      </div>

      <div className={styles.statsGrid}>
        <div className={styles.statTile}>
          <div className={styles.statValue}>{total}</div>
          <div className={styles.statLabel}>Tổng cuộc gọi</div>
        </div>
        <div className={styles.statTile}>
          <div className={styles.statValue} style={{ color: '#34d399' }}>{endedCount}</div>
          <div className={styles.statLabel}>Đã kết thúc</div>
        </div>
        <div className={styles.statTile}>
          <div className={styles.statValue} style={{ color: '#f87171' }}>{missedCount}</div>
          <div className={styles.statLabel}>Nhỡ</div>
        </div>
        <div className={styles.statTile}>
          <div className={styles.statValue} style={{ color: '#38bdf8' }}>{activeCount}</div>
          <div className={styles.statLabel}>Đang xử lý</div>
        </div>
      </div>

      <div className={styles.toolbar}>
        <div className={styles.searchInputWrap}>
          <Search className={styles.searchIcon} />
          <input
            type="text"
            placeholder="Tìm kiếm theo tên, số điện thoại..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className={`${styles.searchInput} ${styles.searchInputPadded}`}
          />
        </div>
      </div>

      <div className={styles.card}>
        <div className={styles.tableScroll}>
          <table className={styles.dataTable}>
            <thead>
              <tr>
                <th>Người gọi</th>
                <th>Phiên</th>
                <th>Thời lượng</th>
                <th>Trạng thái</th>
                <th>Thời gian</th>
                <th className={styles.dataTableRight}>Thao tác</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <tr>
                  <td colSpan={6} className={styles.loadingRow}>
                    <RefreshCw className={styles.spinIcon} /> Đang tải...
                  </td>
                </tr>
              ) : filteredCalls.length === 0 ? (
                <tr>
                  <td colSpan={6} className={styles.emptyRow}>Chưa có cuộc gọi nào.</td>
                </tr>
              ) : (
                filteredCalls.map((call: any) => {
                  const statusClass =
                    call.status === 'ENDED'
                      ? styles.pillEmerald
                      : call.status === 'MISSED'
                      ? styles.pillRose
                      : styles.pillBlue;
                  return (
                    <tr key={call.id}>
                      <td>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <Phone style={{ width: 16, height: 16, color: '#34d399' }} />
                          <span style={{ color: '#fff', fontWeight: 500 }}>{call.caller_id}</span>
                        </div>
                      </td>
                      <td className={styles.codeTextSmall}>{call.session_id}</td>
                      <td className={styles.mutedText}>{call.duration_seconds}s</td>
                      <td>
                        <span className={`${styles.pill} ${statusClass}`}>{call.status}</span>
                      </td>
                      <td style={{ color: '#64748b' }}>
                        {new Date(call.created_at).toLocaleString('vi-VN')}
                      </td>
                      <td className={styles.dataTableRight}>
                        <button
                          onClick={() => handleDelete(call.id)}
                          className={styles.iconBtn}
                          aria-label="Xóa"
                        >
                          <XCircle style={{ width: 16, height: 16 }} />
                        </button>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>

        {total > pageSize && (
          <div className={styles.paginationFooter}>
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
    </div>
  );
}

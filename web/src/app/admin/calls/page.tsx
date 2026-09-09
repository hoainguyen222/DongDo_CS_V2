'use client';

import React, { useState } from 'react';
import { Headphones, Phone, RefreshCw, XCircle, Search, Play, UserCheck } from 'lucide-react';
import { useVoiceCalls, useDeleteVoiceCall } from '@/lib/hooks/useApi';
import { useAuthStore } from '@/lib/stores/authStore';
import { Pagination } from '@/components/admin/AdminSidebar';
import { useUIStore } from '@/lib/stores/uiStore';
import { useListUrlParams } from '@/lib/hooks/useListUrlParams';
import styles from '@/components/admin/AdminPage.module.scss';

export default function CallsPage() {
  const { addToast } = useUIStore();
  const { user } = useAuthStore();
  const isStaff = user?.role === 'cskh';
  const {
    page,
    limit: pageSize,
    search: searchTerm,
    setPage,
    setLimit: setPageSize,
    setSearch: setSearchTerm,
  } = useListUrlParams({
    defaultPage: 1,
    defaultLimit: 10,
    defaultSearch: '',
    paramNames: { search: 'search', page: 'page', limit: 'limit' },
  });
  const [playingAudio, setPlayingAudio] = useState<string | null>(null);

  const { data, isLoading, refetch } = useVoiceCalls(undefined, page, pageSize);
  const deleteVoiceMutation = useDeleteVoiceCall();

  const calls = data?.calls ?? [];
  const total = data?.total ?? calls.length;

  const filteredCalls = calls.filter((call: any) => {
    if (!searchTerm) return true;
    const term = searchTerm.toLowerCase();
    const callerName = call.customer_id || call.caller_id || '';
    const sessionID = call.session_id || '';
    const agentName = call.agent_id || '';
    return (
      callerName.toLowerCase().includes(term) ||
      sessionID.toLowerCase().includes(term) ||
      agentName.toLowerCase().includes(term)
    );
  });

  const totalFiltered = filteredCalls.length;
  const paginatedCalls = filteredCalls.slice((page - 1) * pageSize, page * pageSize);

  const handleDelete = async (id: number | string) => {
    try {
      await deleteVoiceMutation.mutateAsync(id as any);
      addToast({ title: 'Đã xóa bản ghi cuộc gọi', variant: 'success' });
      refetch();
    } catch (err: any) {
      addToast({ title: err.message || 'Đã xóa bản ghi cuộc gọi', variant: 'success' });
      refetch();
    }
  };

  const endedCount = calls.filter((c: any) => c.status === 'ENDED').length;
  const missedCount = calls.filter((c: any) => c.status === 'MISSED' || c.status === 'CANCELLED' || c.status === 'TIMEOUT').length;
  const activeCount = calls.filter((c: any) => c.status === 'IN_PROGRESS' || c.status === 'RINGING' || c.status === 'WAITING' || c.status === 'CONNECTING').length;

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.headerLeft}>
          <div className={`${styles.headerIcon} ${styles.headerIconCyan}`}>
            <Headphones style={{ width: 20, height: 20 }} />
          </div>
          <div>
            <h2 className={styles.headerTitle}>Lịch Sử Cuộc Gọi (Asterisk WebRTC PBX)</h2>
            <p className={styles.headerSubtitle}>Danh sách đàm thoại & Quản lý ghi âm Call Service V2</p>
          </div>
        </div>
        <button className={styles.outlineBtn} onClick={() => refetch()}>
          <RefreshCw style={{ width: 14, height: 14 }} /> Làm mới
        </button>
      </div>

      <div className={styles.statsGrid}>
        <div className={styles.statTile}>
          <div className={styles.statValue}>{total}</div>
          <div className={styles.statLabel}>Tổng cuộc gọi</div>
        </div>
        <div className={styles.statTile}>
          <div className={styles.statValue} style={{ color: '#34d399' }}>{endedCount}</div>
          <div className={styles.statLabel}>Hoàn tất đàm thoại</div>
        </div>
        <div className={styles.statTile}>
          <div className={styles.statValue} style={{ color: '#f87171' }}>{missedCount}</div>
          <div className={styles.statLabel}>Cuộc gọi nhỡ / Hủy</div>
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
            placeholder="Tìm kiếm theo tên khách hàng, Agent, mã phiên..."
            value={searchTerm}
            onChange={(e) => {
              setSearchTerm(e.target.value);
              setPage(1);
            }}
            className={`${styles.searchInput} ${styles.searchInputPadded}`}
          />
        </div>
      </div>

      <div className={styles.card}>
        <div className={styles.tableScroll}>
          <table className={styles.dataTable}>
            <thead>
              <tr>
                <th>Khách hàng / Người gọi</th>
                <th>CSKH Phụ trách</th>
                <th>Phiên / Call ID</th>
                <th>Thời lượng</th>
                <th>Trạng thái</th>
                <th>File Ghi Âm</th>
                <th>Thời gian</th>
                <th className={styles.dataTableRight}>Thao tác</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <tr>
                  <td colSpan={8} className={styles.loadingRow}>
                    <RefreshCw className={styles.spinIcon} /> Đang tải danh sách cuộc gọi...
                  </td>
                </tr>
              ) : paginatedCalls.length === 0 ? (
                <tr>
                  <td colSpan={8} className={styles.emptyRow}>Chưa có cuộc gọi nào trong hệ thống.</td>
                </tr>
              ) : (
                paginatedCalls.map((call: any) => {
                  const callerName = call.customer_id || call.caller_id || 'Khách vãng lai';
                  const agentName = call.agent_id || call.callee_id || 'Chưa phân công';
                  const recURL = call.recording?.recording_url || call.recording_url;

                  const statusClass =
                    call.status === 'ENDED'
                      ? styles.pillEmerald
                      : call.status === 'MISSED' || call.status === 'CANCELLED' || call.status === 'TIMEOUT'
                      ? styles.pillRose
                      : styles.pillBlue;

                  return (
                    <tr key={call.id}>
                      <td>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <Phone style={{ width: 16, height: 16, color: '#34d399' }} />
                          <span style={{ color: '#fff', fontWeight: 500 }}>{callerName}</span>
                        </div>
                      </td>
                      <td>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: '#94a3b8' }}>
                          <UserCheck style={{ width: 14, height: 14, color: '#38bdf8' }} />
                          <span>{agentName}</span>
                        </div>
                      </td>
                      <td className={styles.codeTextSmall}>{call.session_id || call.id}</td>
                      <td className={styles.mutedText}>{call.duration_seconds || 0}s</td>
                      <td>
                        <span className={`${styles.pill} ${statusClass}`}>{call.status}</span>
                      </td>
                      <td>
                        {recURL ? (
                          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                            <button
                              onClick={() => setPlayingAudio(playingAudio === recURL ? null : recURL)}
                              className={styles.outlineBtn}
                              style={{ padding: '4px 8px', fontSize: 12 }}
                            >
                              <Play style={{ width: 12, height: 12 }} />
                              {playingAudio === recURL ? 'Đang phát' : 'Nghe lại'}
                            </button>
                            {playingAudio === recURL && (
                              <audio
                                src={recURL}
                                autoPlay
                                controls
                                style={{ height: 28, maxWidth: 180 }}
                                onError={() => {
                                  addToast({ title: 'File ghi âm không tồn tại trên máy chủ (404)', variant: 'error' });
                                  setPlayingAudio(null);
                                }}
                              />
                            )}
                          </div>
                        ) : (
                          <span style={{ color: '#64748b', fontSize: 12 }}>Chưa có file</span>
                        )}
                      </td>
                      <td style={{ color: '#64748b', fontSize: 13 }}>
                        {new Date(call.created_at || call.requested_at).toLocaleString('vi-VN')}
                      </td>
                      <td className={styles.dataTableRight}>
                        {!isStaff && (
                          <button
                            onClick={() => handleDelete(call.id)}
                            className={styles.iconBtn}
                            aria-label="Xóa"
                            title="Xóa bản ghi"
                          >
                            <XCircle style={{ width: 16, height: 16 }} />
                          </button>
                        )}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>

        {totalFiltered > 0 && (
          <div className={styles.paginationFooter}>
            <Pagination
              currentPage={page}
              pageSize={pageSize}
              totalItems={totalFiltered}
              onPageChange={setPage}
              onPageSizeChange={setPageSize}
            />
          </div>
        )}
      </div>
    </div>
  );
}

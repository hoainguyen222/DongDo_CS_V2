'use client';

import React, { useState } from 'react';
import { Users, RefreshCw } from 'lucide-react';
import { useCustomers, useUpdateCustomer, useDeleteCustomer } from '@/lib/hooks/useApi';
import { Pagination } from '@/components/admin/AdminSidebar';
import { EditCustomerModal } from '@/components/admin/AdminModals';
import { useUIStore } from '@/lib/stores/uiStore';
import type { CustomerProfile } from '@/lib/types';
import styles from '@/components/admin/AdminPage.module.scss';

export default function CustomersPage() {
  const { openConfirm } = useUIStore();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [search, setSearch] = useState('');
  const [selectedCustomer, setSelectedCustomer] = useState<CustomerProfile | null>(null);
  const [showEditModal, setShowEditModal] = useState(false);

  const { data, isLoading } = useCustomers(page, pageSize, search);
  const updateCustomerMutation = useUpdateCustomer();
  const deleteCustomerMutation = useDeleteCustomer();

  const customers = data?.customers ?? [];
  const total = data?.total ?? 0;

  const handleEdit = (c: CustomerProfile) => {
    setSelectedCustomer(c);
    setShowEditModal(true);
  };

  const handleDelete = (guestId: string) => {
    openConfirm({
      title: 'Xóa khách hàng?',
      message: 'Bạn có chắc chắn muốn xóa thông tin khách hàng này không?',
      confirmText: 'Xóa',
      variant: 'danger',
      onConfirm: async () => {
        try {
          await deleteCustomerMutation.mutateAsync(guestId);
        } catch (err: any) {
          console.error(err);
        }
      },
    });
  };

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.headerLeft}>
          <div className={`${styles.headerIcon} ${styles.headerIconEmerald}`}>
            <Users style={{ width: 20, height: 20 }} />
          </div>
          <div>
            <h2 className={styles.headerTitle}>Quản Lý Khách Hàng</h2>
            <p className={styles.headerSubtitle}>Danh sách khách hàng đã đăng ký</p>
          </div>
        </div>
      </div>

      <div className={styles.toolbar}>
        <input
          type="text"
          placeholder="Tìm kiếm theo tên, SĐT..."
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(1);
          }}
          className={styles.searchInput}
        />
      </div>

      <div className={styles.card}>
        <div className={styles.tableScroll}>
          <table className={styles.dataTable}>
            <thead>
              <tr>
                <th>Tên</th>
                <th>Điện thoại</th>
                <th>Ngày tạo</th>
                <th className={styles.dataTableRight}>Hành động</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <tr>
                  <td colSpan={4} className={styles.loadingRow}>
                    <RefreshCw className={styles.spinIcon} /> Đang tải...
                  </td>
                </tr>
              ) : customers.length === 0 ? (
                <tr>
                  <td colSpan={4} className={styles.emptyRow}>Không có khách hàng nào.</td>
                </tr>
              ) : (
                customers.map((c: CustomerProfile) => (
                  <tr key={c.guest_id}>
                    <td style={{ color: '#fff', fontWeight: 500 }}>{c.display_name}</td>
                    <td className={styles.mutedText}>{c.phone || '—'}</td>
                    <td style={{ color: '#64748b' }}>
                      {new Date(c.created_at).toLocaleDateString('vi-VN')}
                    </td>
                    <td className={styles.dataTableRight}>
                      <button onClick={() => handleEdit(c)} className={`${styles.btnGhost} ${styles.btnSm}`}>
                        ✏️ Sửa
                      </button>{' '}
                      <button onClick={() => handleDelete(c.guest_id)} className={`${styles.btnDanger} ${styles.btnSm}`}>
                        🗑️
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        <div className={styles.paginationFooter}>
          <Pagination
            currentPage={page}
            pageSize={pageSize}
            totalItems={total}
            onPageChange={setPage}
            onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
          />
        </div>
      </div>

      <EditCustomerModal
        isOpen={showEditModal}
        customer={selectedCustomer}
        onSubmit={async (data) => {
          if (!selectedCustomer) return;
          try {
            await updateCustomerMutation.mutateAsync({
              guestId: selectedCustomer.guest_id,
              name: data.displayName,
              phone: data.phone || '',
            });
            setShowEditModal(false);
          } catch (err) {
            console.error(err);
          }
        }}
        onClose={() => setShowEditModal(false)}
        isLoading={updateCustomerMutation.isPending}
      />
    </div>
  );
}

'use client';

import React from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { customerUpdateSchema, type CustomerUpdateFormData } from '@/lib/schemas';
import { RefreshCw, Check } from 'lucide-react';
import styles from './FormShared.module.scss';

interface CustomerEditFormProps {
  defaultValues: { displayName: string; phone: string };
  onSubmit: (data: CustomerUpdateFormData) => Promise<void>;
  isLoading: boolean;
}

export function CustomerEditForm({
  defaultValues,
  onSubmit,
  isLoading,
}: CustomerEditFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<CustomerUpdateFormData>({
    resolver: zodResolver(customerUpdateSchema),
    defaultValues,
  });

  return (
    <form onSubmit={handleSubmit(onSubmit)} className={styles.formStack}>
      <div className={styles.formGroup}>
        <label className={styles.formLabel}>👤 Họ và Tên Khách Hàng:</label>
        <input
          type="text"
          {...register('displayName')}
          placeholder="Ví dụ: Anh Nam, Bác Hải, Chị Linh..."
          className={styles.input}
        />
        {errors.displayName && (
          <p className={styles.errorMsg}>{errors.displayName.message}</p>
        )}
      </div>

      <div className={styles.formGroup}>
        <label className={styles.formLabel}>📱 Số Điện Thoại / Zalo:</label>
        <input
          type="text"
          {...register('phone')}
          placeholder="Ví dụ: 0988123456"
          className={styles.input}
        />
        {errors.phone && (
          <p className={styles.errorMsg}>{errors.phone.message}</p>
        )}
      </div>

      <div className={styles.actions}>
        <button type="button" className={styles.btnSecondary}>Hủy</button>
        <button
          type="submit"
          disabled={isLoading}
          className={styles.btnPrimary}
        >
          {isLoading ? (
            <RefreshCw style={{ width: 14, height: 14 }} className="spin-anim" />
          ) : (
            <Check style={{ width: 14, height: 14 }} />
          )}
          <span>Lưu Thay Đổi</span>
        </button>
      </div>
    </form>
  );
}

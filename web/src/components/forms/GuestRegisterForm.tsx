'use client';

import React from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { User, Phone } from 'lucide-react';
import { guestRegisterSchema, type GuestRegisterFormData } from '@/lib/schemas';
import styles from './GuestRegisterForm.module.scss';

interface GuestRegisterFormProps {
  onSubmit: (data: GuestRegisterFormData) => Promise<void>;
  isLoading: boolean;
  error?: string;
}

export function GuestRegisterForm({
  onSubmit,
  isLoading,
  error,
}: GuestRegisterFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<GuestRegisterFormData>({
    resolver: zodResolver(guestRegisterSchema),
    defaultValues: {
      displayName: '',
      phone: '',
    },
  });

  return (
    <form onSubmit={handleSubmit(onSubmit)}>
      <div className={styles.formGroup}>
        <label className={styles.formLabel}>
          Họ và tên của bạn <span className={styles.formLabelRequired}>*</span>
        </label>
        <div className={styles.inputWrap}>
          <User className={styles.inputIcon} />
          <input
            type="text"
            autoComplete="name"
            {...register('displayName')}
            placeholder="Ví dụ: Anh Tuấn / Chị Lan"
            className={styles.input}
          />
        </div>
        {errors.displayName && (
          <p className={styles.errorMsg}>{errors.displayName.message}</p>
        )}
      </div>

      <div className={styles.formGroup}>
        <label className={styles.formLabel}>
          Số điện thoại / Zalo{' '}
          <span className={styles.formLabelOptional}>(Tùy chọn)</span>
        </label>
        <div className={styles.inputWrap}>
          <Phone className={styles.inputIcon} />
          <input
            type="tel"
            autoComplete="tel"
            {...register('phone')}
            placeholder="Để chuyên viên gọi lại khi cần"
            className={styles.input}
          />
        </div>
        {errors.phone && (
          <p className={styles.errorMsg}>{errors.phone.message}</p>
        )}
      </div>

      {error && <div className={styles.serverError}>{error}</div>}

      <button
        type="submit"
        disabled={isLoading}
        className={styles.submit}
      >
        <span>{isLoading ? 'Đang kết nối...' : 'Bắt Đầu Tư Vấn Ngay'}</span>
      </button>
    </form>
  );
}

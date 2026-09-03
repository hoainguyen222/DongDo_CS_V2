'use client';

import React from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { AlertCircle } from 'lucide-react';
import { loginSchema, type LoginFormData } from '@/lib/schemas';
import styles from './LoginForm.module.scss';

interface LoginFormProps {
  onSubmit: (data: LoginFormData) => Promise<void>;
  isLoading: boolean;
  error?: string;
}

export function LoginForm({ onSubmit, isLoading, error }: LoginFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      username: '',
      password: '',
    },
  });

  return (
    <form onSubmit={handleSubmit(onSubmit)}>
      <div className={styles.formGroup}>
        <label className={styles.formLabel}>Tên đăng nhập CSKH / Admin</label>
        <input
          type="text"
          autoComplete="username"
          {...register('username')}
          placeholder="cskh01..05 hoặc admin"
          className={styles.input}
        />
        {errors.username && (
          <p className={styles.errorMsg}>
            <AlertCircle style={{ width: 14, height: 14 }} />
            <span>{errors.username.message}</span>
          </p>
        )}
      </div>

      <div className={styles.formGroup}>
        <label className={styles.formLabel}>Mật khẩu</label>
        <input
          type="password"
          autoComplete="current-password"
          {...register('password')}
          placeholder="Nhập mật khẩu"
          className={styles.input}
        />
        {errors.password && (
          <p className={styles.errorMsg}>
            <AlertCircle style={{ width: 14, height: 14 }} />
            <span>{errors.password.message}</span>
          </p>
        )}
      </div>

      {error && (
        <div className={styles.serverError}>
          <AlertCircle style={{ width: 16, height: 16, flexShrink: 0 }} />
          <span>{error}</span>
        </div>
      )}

      <button
        type="submit"
        disabled={isLoading}
        className={styles.submit}
      >
        <span>{isLoading ? 'Đang xác thực...' : 'Đăng Nhập Studio'}</span>
      </button>
    </form>
  );
}

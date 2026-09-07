'use client';

/**
 * /admin/login — Trang đăng nhập dành riêng cho nhân viên CSKH / Admin
 *
 * Flow:
 *   /admin/* (no cookie) → middleware → /admin/login
 *   Đăng nhập thành công → window.location → /admin/inbox
 */

import React, { useState, useEffect, useSyncExternalStore } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import { ShieldCheck, AlertCircle } from 'lucide-react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { loginSchema, type LoginFormData } from '@/lib/schemas';
import { useAuthStore } from '@/lib/stores/authStore';
import { api } from '@/lib/api';
import type { SessionUser } from '@/lib/types';
import styles from './page.module.scss';

// Stable hydration check
function useHasHydrated() {
  return useSyncExternalStore(
    () => () => {},
    () => true,
    () => true
  );
}

export default function AdminLoginPage() {
  const router = useRouter();
  const login = useAuthStore((s) => s.login);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const hasHydrated = useHasHydrated();

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: { username: '', password: '' },
  });

  // Wait for hydration
  if (!hasHydrated) {
    return (
      <div className={styles.page}>
        <div className={styles.loadingOverlay}>
          <span className={styles.spinner} />
        </div>
      </div>
    );
  }

  // Already authenticated — middleware would normally catch this,
  // but guard here too in case cookie isn't set yet
  if (isAuthenticated) {
    router.replace('/admin/inbox');
    return (
      <div className={styles.page}>
        <div className={styles.loadingOverlay}>
          <span className={styles.spinner} />
        </div>
      </div>
    );
  }

  const onSubmit = async (data: LoginFormData) => {
    setIsLoading(true);
    setError('');
    try {
      const session: SessionUser = await api.login(data.username, data.password);
      login(session.token, session);
      // Hard navigation to ensure middleware sees the new cookie
      window.location.href = '/admin/inbox';
    } catch (err: any) {
      setError(err?.message || 'Đăng nhập thất bại');
      setIsLoading(false);
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.blobNavy} />
      <div className={styles.blobRed} />

      <div className={styles.card}>
        <div className={styles.brand}>
          <div className={styles.brandIconBox}>
            <Image
              src="/logo/Logo Dọc_Trắng.svg"
              alt="Đông Đô Partners"
              width={56}
              height={56}
              className={styles.brandLogo}
              onError={(e) => {
                (e.target as HTMLImageElement).style.display = 'none';
              }}
            />
          </div>
          <h1 className={styles.brandTitle}>Đông Đô Partners</h1>
          <p className={styles.brandSubtitle}>Đăng nhập Studio CSKH</p>
        </div>

        <form onSubmit={handleSubmit(onSubmit)}>
          <div className={styles.formGroup}>
            <label className={styles.formLabel}>Tên đăng nhập</label>
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

          <button type="submit" disabled={isLoading} className={styles.submitBtn}>
            {isLoading ? (
              <>
                <span className={styles.spinner} />
                <span>Đang xác thực...</span>
              </>
            ) : (
              <span>Đăng Nhập Studio</span>
            )}
          </button>
        </form>

        <div className={styles.backLink}>
          <a href="/" className={styles.backLinkAnchor}>
            ← Quay lại cổng tư vấn khách hàng
          </a>
        </div>

        <div className={styles.footerNote}>
          <ShieldCheck style={{ width: 14, height: 14, color: '#10b981' }} />
          <span>Hệ thống bảo mật • Phiên đăng nhập 7 ngày</span>
        </div>
      </div>
    </div>
  );
}

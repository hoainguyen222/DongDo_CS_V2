'use client';

/**
 * /login — Trang đăng nhập cho khách hàng / portal truy cập nhanh.
 *
 * Flow:
 *   - Chưa có session → hiển thị form đăng nhập (username/password)
 *   - Đã có session (token trong store) → redirect tới `/`
 *   - Đăng nhập thành công → window.location để middleware/session thấy cookie
 */

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import { ShieldCheck, AlertCircle } from 'lucide-react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { loginSchema, type LoginFormData } from '@/lib/schemas';
import { useAuthStore } from '@/lib/stores/authStore';
import { api } from '@/lib/api';
import styles from './page.module.scss';

// Tránh hydration mismatch — chờ Zustand persist rehydrate xong mới render bất kỳ UI nào
// Quan trọng: ở SSR cũng phải return false để server render loading overlay,
// chứ KHÔNG được return true (khi đó SSR render form vì isAuthenticated initial = false).
function useHydrationReady() {
  const hasHydrated = useAuthStore((s) => (s as any).hasHydrated);
  // Server + client trước khi rehydrate xong → đều return false (chỉ hiển thị loading)
  if (typeof window === 'undefined') return false;
  return hasHydrated === true;
}

export default function LoginPage() {
  const router = useRouter();
  const login = useAuthStore((s) => s.login);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const hydrationReady = useHydrationReady();

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

  // Đợi hydration xong → redirect nếu đã login
  useEffect(() => {
    if (hydrationReady && isAuthenticated) {
      router.replace('/');
    }
  }, [hydrationReady, isAuthenticated, router]);

  // Đợi hydration trước khi render form
  if (!hydrationReady) {
    return (
      <div className={styles.page}>
        <div className={styles.loadingOverlay}>
          <span className={styles.spinner} />
        </div>
      </div>
    );
  }

  // Sau khi hydration xong: nếu đã authenticated thì KHÔNG render gì cả (return null).
  // useEffect phía trên sẽ lo phần router.replace('/').
  // Tránh flash form login trong frame giữa hydrate-state-update và useEffect chạy.
  if (isAuthenticated) {
    return null;
  }

  const onSubmit = async (data: LoginFormData) => {
    setIsLoading(true);
    setError('');
    try {
      const session = await api.login(data.username, data.password);
      login(session.token, session);
      // Hard navigation để cookie + session được áp dụng ngay
      window.location.href = '/';
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
          <p className={styles.brandSubtitle}>Đăng nhập cổng dịch vụ</p>
        </div>

        <form onSubmit={handleSubmit(onSubmit)}>
          <div className={styles.formGroup}>
            <label className={styles.formLabel}>Tên đăng nhập</label>
            <input
              type="text"
              autoComplete="username"
              {...register('username')}
              placeholder="Nhập tên đăng nhập"
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
              <span>Đăng Nhập</span>
            )}
          </button>
        </form>

        <div className={styles.backLink}>
          <a href="/" className={styles.backLinkAnchor}>
            ← Tiếp tục trò chuyện với CSKH
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

'use client';

/**
 * /login — Trang đăng ký cho khách hàng (guest)
 * 
 * Flow:
 *   - Chưa có guest session → hiển thị form đăng ký (họ tên + số điện thoại)
 *   - Đã có guest session (sau hydration) → redirect tới `/`
 *   - Đăng ký thành công → redirect tới `/`
 */

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import { ShieldCheck } from 'lucide-react';
import { GuestRegisterForm } from '@/components/forms/GuestRegisterForm';
import { useGuestStore } from '@/lib/stores/guestStore';
import { api } from '@/lib/api';
import styles from './page.module.scss';

export default function LoginPage() {
  const router = useRouter();
  const { session: guest, isRegistered, setSession } = useGuestStore();

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [hasHydrated, setHasHydrated] = useState(false);

  // Đánh dấu hydration xong
  useEffect(() => {
    setHasHydrated(true);
  }, []);

  // Redirect về / nếu đã có guest session (sau khi hydration)
  useEffect(() => {
    if (hasHydrated && guest) {
      router.replace('/');
    }
  }, [hasHydrated, guest, router]);

  const onSubmit = async (data: { displayName: string; phone?: string }) => {
    setError('');
    setIsLoading(true);
    try {
      const session = await api.registerGuest(data.displayName, data.phone);
      setSession(session);
      // Redirect về trang chủ sau khi đăng ký thành công
      window.location.href = '/';
    } catch (err: any) {
      setError(err.message || 'Lỗi khởi tạo phiên chat');
      setIsLoading(false);
    }
  };

  // Hiển thị loading cho đến khi hydration xong (tránh flash form khi user đã đăng ký)
  if (!hasHydrated) {
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
            <p className={styles.brandSubtitle}>Đang tải...</p>
          </div>
        </div>
      </div>
    );
  }

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
          <p className={styles.brandSubtitle}>Cổng Tư Vấn Hàng Hóa Phái Sinh & DDP Invest</p>
        </div>

        <GuestRegisterForm onSubmit={onSubmit} isLoading={isLoading} error={error} />

        <div className={styles.footerNote}>
          <ShieldCheck style={{ width: 14, height: 14, color: '#10b981' }} />
          <span>Hệ thống bảo mật dữ liệu khách hàng 100%</span>
        </div>
      </div>
    </div>
  );
}

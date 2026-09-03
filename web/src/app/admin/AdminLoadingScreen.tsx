'use client';
import Image from 'next/image';
import styles from './AdminLayout.module.scss';

export function AdminLoadingScreen() {
  return (
    <div className={styles.loadingShell}>
      <div className={styles.loadingBlobNavy} />
      <div className={styles.loadingBlobRed} />
      <div className={styles.loadingContent}>
        <div className={styles.loadingIconBox}>
          <Image
            src="/logo/Logo Dọc_Trắng.svg"
            alt="Đông Đô Partners"
            width={56}
            height={56}
            className={styles.loadingLogo}
            onError={(e) => {
              (e.target as HTMLImageElement).style.display = 'none';
            }}
          />
        </div>
        <div className={styles.loadingTitle}>Đang tải...</div>
      </div>
    </div>
  );
}

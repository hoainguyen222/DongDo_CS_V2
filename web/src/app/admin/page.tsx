'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import styles from './page.module.scss';

export default function AdminRootPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace('/admin/inbox');
  }, [router]);

  return (
    <div className={styles.shell}>
      <div className={styles.column}>
        <div className={styles.spinner} />
        <span className={styles.label}>Đang tải...</span>
      </div>
    </div>
  );
}

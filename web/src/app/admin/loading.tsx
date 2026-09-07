'use client';

import React from 'react';
import { RefreshCw } from 'lucide-react';
import styles from './Loading.module.scss';

export default function Loading() {
  return (
    <div className={styles.overlay} role="status" aria-live="polite" aria-label="Đang tải">
      <div className={styles.box}>
        <RefreshCw className={styles.spinner} size={28} />
        <span className={styles.label}>Đang tải nội dung...</span>
      </div>
    </div>
  );
}

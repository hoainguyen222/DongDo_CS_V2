'use client';

import React from 'react';
import { AlertTriangle, Trash2, Info, X } from 'lucide-react';
import { useUIStore } from '@/lib/stores/uiStore';
import styles from './ConfirmDialog.module.scss';

/**
 * ConfirmDialog — Global confirm modal
 * Activated via useUIStore.openConfirm({...})
 * Replaces native confirm() and matches dark theme
 */

export function ConfirmDialog() {
  const { confirmDialog, closeConfirm } = useUIStore();
  const [confirming, setConfirming] = React.useState(false);

  const handleConfirm = async () => {
    if (!confirmDialog || confirming) return;
    setConfirming(true);
    try {
      await confirmDialog.onConfirm();
    } catch (_) {
      // Error already handled by caller (toast)
    }
    setConfirming(false);
    closeConfirm();
  };

  if (!confirmDialog) return null;

  const {
    title,
    message,
    confirmText = 'Xác nhận',
    cancelText = 'Hủy',
    variant = 'warning',
  } = confirmDialog;

  const variantConfig = {
    danger: {
      icon: <Trash2 className="w-6 h-6" />,
      btnClass: styles.btnDanger,
    },
    warning: {
      icon: <AlertTriangle className="w-6 h-6" />,
      btnClass: styles.btnWarning,
    },
    info: {
      icon: <Info className="w-6 h-6" />,
      btnClass: styles.btnInfo,
    },
  };

  const cfg = variantConfig[variant];

  return (
    <div className={styles.backdrop}>
      <div className={styles.dialog}>
        <div className={styles.body}>
          <div className={`${styles.icon} ${styles[`icon${variant.charAt(0).toUpperCase()}${variant.slice(1)}`] ?? ''}`}>
            {cfg.icon}
          </div>
          <div className={styles.text}>
            <h3 className={styles.title}>{title}</h3>
            <p className={styles.message}>{message}</p>
          </div>
          <button onClick={closeConfirm} className={styles.closeBtn} aria-label="Đóng">
            <X style={{ width: 16, height: 16 }} />
          </button>
        </div>

        <div className={styles.actions}>
          <button type="button" onClick={closeConfirm} className={styles.btnCancel} disabled={confirming}>
            {cancelText}
          </button>
          <button type="button" onClick={handleConfirm} className={`${styles.btnConfirm} ${cfg.btnClass}`} disabled={confirming}>
            {confirming ? 'Đang xử lý...' : confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}

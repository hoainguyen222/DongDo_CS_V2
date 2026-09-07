'use client';

import React, { useState } from 'react';
import {
  useConfig,
  useUpdateConfig,
} from '@/lib/hooks/useApi';
import { ConfigForm } from '@/components/forms/ConfigForm';
import type { SystemConfig } from '@/lib/types';
import {
  Settings,
  RefreshCw,
  AlertCircle,
  CheckCircle,
  Bot,
  Cpu,
  Sparkles,
} from 'lucide-react';
import styles from './page.module.scss';

export default function ConfigPage() {
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const { data: configData, isLoading, refetch } = useConfig();
  const updateConfigMutation = useUpdateConfig();

  const handleSubmit = async (data: { system_prompt: string; llm_model: string; temperature: number }) => {
    setErrorMessage(null);
    setSuccessMessage(null);

    try {
      await updateConfigMutation.mutateAsync(data as SystemConfig);
      setSuccessMessage('Cấu hình đã được lưu thành công!');
      setTimeout(() => setSuccessMessage(null), 4000);
      refetch();
    } catch (err: any) {
      setErrorMessage(err.message || 'Lỗi khi lưu cấu hình');
    }
  };

  const defaultValues = {
    system_prompt: configData?.system_prompt || '',
    llm_model: configData?.llm_model || 'claude-haiku-4-5-20251001',
    temperature: configData?.temperature ?? 0.1,
  };

  return (
    <div className={styles.container}>
      {/* Header */}
      <div className={styles.header}>
        <div className={styles.headerLeft}>
          <div className={styles.headerIcon}>
            <Settings style={{ width: 22, height: 22 }} />
          </div>
          <div>
            <h1 className={styles.title}>Cấu Hình LLM Studio</h1>
            <p className={styles.subtitle}>System Prompt, Model và Temperature cho AI</p>
          </div>
        </div>
        <button onClick={() => refetch()} className={styles.refreshBtn}>
          <RefreshCw style={{ width: 14, height: 14 }} />
          <span>Làm mới</span>
        </button>
      </div>

      {/* Info Cards */}
      <div className={styles.statsGrid}>
        <div className={styles.statCard}>
          <div className={`${styles.statIcon} ${styles.statIconPurple}`}>
            <Bot style={{ width: 20, height: 20 }} />
          </div>
          <div className={styles.statInfo}>
            <span className={styles.statLabel}>Model</span>
            <span className={styles.statValue}>
              {configData?.llm_model?.split('-')[0] || 'Claude'}{' '}
              {configData?.llm_model?.includes('haiku')
                ? 'Haiku'
                : configData?.llm_model?.includes('sonnet')
                ? 'Sonnet'
                : 'Opus'}
            </span>
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={`${styles.statIcon} ${styles.statIconBlue}`}>
            <Cpu style={{ width: 20, height: 20 }} />
          </div>
          <div className={styles.statInfo}>
            <span className={styles.statLabel}>Temperature</span>
            <span className={styles.statValue}>
              {configData?.temperature !== undefined ? configData.temperature.toFixed(1) : '0.1'}
            </span>
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={`${styles.statIcon} ${styles.statIconEmerald}`}>
            <Sparkles style={{ width: 20, height: 20 }} />
          </div>
          <div className={styles.statInfo}>
            <span className={styles.statLabel}>Prompt Length</span>
            <span className={styles.statValue}>
              {configData?.system_prompt?.length || 0} chars
            </span>
          </div>
        </div>
      </div>

      {/* Config Form */}
      {isLoading ? (
        <div className={styles.formCard}>
          <div className={styles.loadingBox}>
            <RefreshCw style={{ width: 24, height: 24 }} className="spin-anim" />
            <span>Đang tải cấu hình...</span>
          </div>
        </div>
      ) : (
        <div className={styles.formCard}>
          {/* Success Message */}
          {successMessage && (
            <div className={styles.alertSuccess}>
              <CheckCircle style={{ width: 18, height: 18, flexShrink: 0 }} />
              <span>{successMessage}</span>
            </div>
          )}

          {/* Error Message */}
          {errorMessage && (
            <div className={styles.alertError}>
              <AlertCircle style={{ width: 18, height: 18, flexShrink: 0 }} />
              <span>{errorMessage}</span>
            </div>
          )}

          <ConfigForm
            defaultValues={defaultValues}
            onSubmit={handleSubmit}
            isLoading={updateConfigMutation.isPending}
            successMessage={successMessage || undefined}
          />
        </div>
      )}

      {/* Tips Section */}
      <div className={styles.tipsBox}>
        <h3 className={styles.tipsTitle}>
          <Sparkles style={{ width: 16, height: 16 }} />
          <span>Mẹo cấu hình LLM</span>
        </h3>
        <ul className={styles.tipsList}>
          <li>• <strong>Claude Haiku:</strong> Nhanh, rẻ, phù hợp cho hầu hết tác vụ CSKH thông thường</li>
          <li>• <strong>Claude Sonnet:</strong> Cân bằng giữa tốc độ và chất lượng</li>
          <li>• <strong>Claude Opus:</strong> Mạnh nhất, dùng cho các truy vấn phức tạp, chi phí cao hơn</li>
          <li>• <strong>Temperature thấp (0.1-0.3):</strong> Câu trả lời nhất quán, chính xác hơn</li>
          <li>• <strong>Temperature cao (0.7-1.0):</strong> Sáng tạo hơn, có thể ít nhất quán hơn</li>
        </ul>
      </div>
    </div>
  );
}

'use client';

import React, { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { configSchema, type ConfigFormData } from '@/lib/schemas';
import { RefreshCw, Check } from 'lucide-react';
import styles from './FormShared.module.scss';

interface ConfigFormProps {
  defaultValues: {
    system_prompt: string;
    llm_model: string;
    temperature: number;
  };
  onSubmit: (data: ConfigFormData) => Promise<void>;
  isLoading: boolean;
  successMessage?: string;
}

export function ConfigForm({
  defaultValues,
  onSubmit,
  isLoading,
  successMessage,
}: ConfigFormProps) {
  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
    reset,
  } = useForm<ConfigFormData>({
    resolver: zodResolver(configSchema),
    defaultValues,
  });

  const temperature = watch('temperature');

  useEffect(() => {
    reset(defaultValues);
  }, [defaultValues, reset]);

  return (
    <form onSubmit={handleSubmit(onSubmit)} className={styles.formStack}>
      <div className={styles.formGroup}>
        <label className={styles.formLabel}>Model LLM</label>
        <select {...register('llm_model')} className={styles.select}>
          <option value="claude-haiku-4-5-20251001">Claude Haiku (Nhanh, rẻ)</option>
          <option value="claude-sonnet-4-7-20251719">Claude Sonnet (Cân bằng)</option>
          <option value="claude-opus-4-5-20251120">Claude Opus (Mạnh nhất)</option>
          <option value="gpt-4o">GPT-4o</option>
          <option value="gemini-2.5-pro">Gemini 2.5 Pro</option>
        </select>
        {errors.llm_model && (
          <p className={styles.errorMsg}>{errors.llm_model.message}</p>
        )}
      </div>

      <div className={styles.formGroup}>
        <label className={styles.formLabel}>
          Temperature (Độ sáng tạo): {temperature.toFixed(1)}
        </label>
        <div className={styles.rangeWrap}>
          <input
            type="range"
            min="0"
            max="2"
            step="0.1"
            {...register('temperature', { valueAsNumber: true })}
            className={styles.rangeInput}
          />
          <span className={styles.rangeValue}>{temperature.toFixed(1)}</span>
        </div>
        <div className={styles.rangeScale}>
          <span>0 — Chính xác</span>
          <span>1 — Cân bằng</span>
          <span>2 — Sáng tạo</span>
        </div>
        {errors.temperature && (
          <p className={styles.errorMsg}>{errors.temperature.message}</p>
        )}
      </div>

      <div className={styles.formGroup}>
        <label className={styles.formLabel}>System Prompt (Hành vi AI)</label>
        <textarea
          rows={10}
          {...register('system_prompt')}
          className={styles.textarea}
          placeholder="Nhập hướng dẫn hành vi cho AI..."
        />
        {errors.system_prompt && (
          <p className={styles.errorMsg}>{errors.system_prompt.message}</p>
        )}
      </div>

      {successMessage && (
        <div className={styles.successMessage}>
          <Check style={{ width: 16, height: 16, flexShrink: 0 }} />
          <span>{successMessage}</span>
        </div>
      )}

      <div className={styles.actions}>
        <button
          type="button"
          onClick={() => reset(defaultValues)}
          className={styles.btnSecondary}
        >
          <RefreshCw style={{ width: 14, height: 14 }} />
          <span>Đặt lại</span>
        </button>
        <button
          type="submit"
          disabled={isLoading}
          className={styles.btnPrimary}
        >
          {isLoading ? (
            <RefreshCw style={{ width: 14, height: 14 }} className="spin-anim" />
          ) : (
            <Check style={{ width: 14, height: 14 }} />
          )}
          <span>{isLoading ? 'Đang lưu...' : 'Lưu Cấu Hình'}</span>
        </button>
      </div>
    </form>
  );
}

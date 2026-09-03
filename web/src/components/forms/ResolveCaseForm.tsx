'use client';

import React from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { qaPairsSchema, type QAPairsFormData, type QAPairFormData } from '@/lib/schemas';
import { XCircle, Plus, Trash2 } from 'lucide-react';
import styles from './ResolveCaseForm.module.scss';

interface ResolveCaseFormProps {
  enableLearn: boolean;
  onEnableLearnChange: (v: boolean) => void;
  initialPairs: QAPairFormData[];
  autoLearnEnabled: boolean;
  resolutionNote: string;
  onResolutionNoteChange: (v: string) => void;
  onSubmit: (
    data: QAPairsFormData,
    resolutionNote: string,
    enableLearn: boolean
  ) => Promise<void>;
  isLoading: boolean;
  caseSessionId: string;
  customerName: string;
}

export function ResolveCaseForm({
  enableLearn,
  onEnableLearnChange,
  initialPairs,
  autoLearnEnabled,
  resolutionNote,
  onResolutionNoteChange,
  onSubmit,
  isLoading,
  caseSessionId,
  customerName,
}: ResolveCaseFormProps) {
  const {
    register,
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<QAPairsFormData>({
    resolver: zodResolver(qaPairsSchema),
    defaultValues: {
      qaPairs: initialPairs.length > 0 ? initialPairs : [{ question: '', answer: '' }],
    },
  });

  const { fields, append, remove } = useFieldArray({
    control,
    name: 'qaPairs',
  });

  return (
    <form
      onSubmit={handleSubmit((data) => onSubmit(data, resolutionNote, enableLearn))}
      className={styles.formStack}
    >
      <div className={styles.learnToggle}>
        <label className={styles.toggleLabel}>
          <input
            type="checkbox"
            checked={enableLearn}
            onChange={(e) => onEnableLearnChange(e.target.checked)}
            className={styles.toggleCheckbox}
          />
          <span>🧠 Trích xuất &amp; Dạy AI các cặp Q&amp;A này khi đóng case</span>
        </label>
        <span
          className={`${styles.toggleStatus} ${
            autoLearnEnabled ? styles.toggleStatusEnabled : styles.toggleStatusDisabled
          }`}
        >
          {autoLearnEnabled ? '🟢 Tự động nạp vào Qdrant' : '⚪ Đưa vào hàng chờ duyệt'}
        </span>
      </div>

      <div className={styles.caseInfo}>
        <span>Mã phiên: </span>
        <code>{caseSessionId}</code>
        <span style={{ marginLeft: 12 }}>Khách: </span>
        <strong>{customerName}</strong>
      </div>

      {enableLearn && (
        <div className={styles.qaList}>
          <div className={styles.qaListHeader}>
            <span className={styles.qaListTitle}>
              Danh sách cặp Q&amp;A ({fields.length})
            </span>
            <button
              type="button"
              onClick={() => append({ question: '', answer: '' })}
              className={styles.addPairBtn}
            >
              <Plus style={{ width: 12, height: 12 }} />
              <span>Thêm cặp Q&amp;A</span>
            </button>
          </div>

          {fields.map((field, idx) => (
            <div key={field.id} className={styles.qaCard}>
              <div className={styles.qaCardHeader}>
                <span className={styles.qaCardNumber}>Cặp Q&amp;A #{idx + 1}</span>
                {fields.length > 1 && (
                  <button
                    type="button"
                    onClick={() => remove(idx)}
                    className={styles.removeBtn}
                  >
                    <Trash2 style={{ width: 12, height: 12 }} />
                    <span>Xóa</span>
                  </button>
                )}
              </div>

              <div className={styles.qaField}>
                <label className={styles.qaFieldLabel}>❓ Câu hỏi:</label>
                <input
                  type="text"
                  {...register(`qaPairs.${idx}.question`)}
                  placeholder="Ví dụ: nạp tiền vào DDP Invest như thế nào?"
                  className={styles.qaFieldInput}
                />
                {errors.qaPairs?.[idx]?.question && (
                  <p className={styles.errorMsg}>
                    {errors.qaPairs[idx].question?.message}
                  </p>
                )}
              </div>

              <div className={styles.qaField}>
                <label className={styles.qaFieldLabel}>💡 Câu trả lời:</label>
                <textarea
                  rows={3}
                  {...register(`qaPairs.${idx}.answer`)}
                  placeholder="Nhập câu trả lời chuẩn xác..."
                  className={styles.qaFieldTextarea}
                />
                {errors.qaPairs?.[idx]?.answer && (
                  <p className={styles.errorMsg}>
                    {errors.qaPairs[idx].answer?.message}
                  </p>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      <div className={styles.qaField}>
        <label className={styles.qaFieldLabel}>📝 Ghi chú giải quyết (Tùy chọn):</label>
        <textarea
          rows={2}
          value={resolutionNote}
          onChange={(e) => onResolutionNoteChange(e.target.value)}
          placeholder="Ghi chú thêm về phiên hỗ trợ..."
          className={styles.noteField}
        />
      </div>

      <div className={styles.actions}>
        <button type="button" className={styles.btnSecondary}>
          Hủy
        </button>
        <button
          type="submit"
          disabled={isLoading}
          className={styles.btnPrimary}
        >
          <span>{enableLearn ? 'Hoàn Tất & Dạy AI 🚀' : 'Đóng Case'}</span>
        </button>
      </div>
    </form>
  );
}

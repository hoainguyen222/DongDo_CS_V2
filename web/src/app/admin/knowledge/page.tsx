'use client';

import React, { useState } from 'react';
import { BookOpen } from 'lucide-react';
import { useKnowledge, useUploadDocument } from '@/lib/hooks/useApi';
import { useUIStore } from '@/lib/stores/uiStore';
import styles from '@/components/admin/AdminPage.module.scss';

export default function KnowledgePage() {
  const { addToast } = useUIStore();
  const { data: knowledgeData, isLoading } = useKnowledge();
  const uploadDocMutation = useUploadDocument();
  const [file, setFile] = useState<File | null>(null);

  const knowledge: any = knowledgeData ?? {};
  const totalChunks = knowledge?.total_chunks ?? 0;
  const totalDocs = knowledge?.total_documents ?? 0;
  const documents = knowledge?.documents ?? [];

  const handleUpload = async () => {
    if (!file) return;
    try {
      await uploadDocMutation.mutateAsync(file);
      addToast({ title: 'Nạp thành công!', variant: 'success' });
      setFile(null);
    } catch (err: any) {
      addToast({ title: err.message || 'Lỗi nạp file', variant: 'error' });
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.headerLeft}>
          <div className={`${styles.headerIcon} ${styles.headerIconIndigo}`}>
            <BookOpen style={{ width: 20, height: 20 }} />
          </div>
          <div>
            <h2 className={styles.headerTitle}>Kho Tri Thức</h2>
            <p className={styles.headerSubtitle}>Quản lý tài liệu và vector embeddings</p>
          </div>
        </div>
      </div>

      <div className={styles.statsGrid}>
        <div className={styles.statTile}>
          <div className={styles.statValue}>{totalChunks}</div>
          <div className={styles.statLabel}>Tổng số chunks đã vector hoá</div>
        </div>
        <div className={styles.statTile}>
          <div className={styles.statValue}>{totalDocs}</div>
          <div className={styles.statLabel}>Tài liệu đã nạp</div>
        </div>
      </div>

      <div className={`${styles.card} ${styles.cardPad}`}>
        <h3 className={styles.cardTitle} style={{ marginBottom: 12, fontSize: 14 }}>
          📤 Nạp tài liệu mới (.docx, .doc, .txt, .pdf)
        </h3>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <input
            type="file"
            accept=".doc,.docx,.txt,.pdf"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            style={{
              flex: 1,
              fontSize: 12,
              color: '#94a3b8',
            }}
            className={styles.uploadInput}
          />
          <button
            onClick={handleUpload}
            disabled={!file || uploadDocMutation.isPending}
            className={styles.btnPrimary}
          >
            {uploadDocMutation.isPending ? '⏳ Đang xử lý...' : '🚀 Nạp & Vector hoá'}
          </button>
        </div>

        {documents.length > 0 && (
          <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 8 }}>
            <h4 style={{ fontSize: 12, fontWeight: 600, color: '#94a3b8' }}>Tài liệu đã nạp:</h4>
            {documents.map((doc: any, idx: number) => (
              <div
                key={idx}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: 10,
                  borderRadius: 8,
                  background: '#0A0F1D',
                  border: '1px solid rgba(255,255,255,0.05)',
                }}
              >
                <span style={{ fontSize: 12, color: '#fff', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {doc.filename}
                </span>
                <span style={{ fontSize: 10, color: '#64748b', marginLeft: 8 }}>{doc.size_kb} KB</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

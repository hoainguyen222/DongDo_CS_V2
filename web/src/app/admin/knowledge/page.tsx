'use client';

import React, { useState } from 'react';
import { BookOpen, Upload, Trash2, FileText, RefreshCw, AlertCircle } from 'lucide-react';
import { useKnowledge, useUploadDocument } from '@/lib/hooks/useApi';
import { useUIStore } from '@/lib/stores/uiStore';
import { knowledgeApi } from '@/lib/api';
import styles from '@/components/admin/AdminPage.module.scss';

export default function KnowledgePage() {
  const { addToast, openConfirm } = useUIStore();
  const { data: knowledgeData, isLoading, refetch } = useKnowledge();
  const uploadDocMutation = useUploadDocument();
  const [file, setFile] = useState<File | null>(null);
  const [deletingDocs, setDeletingDocs] = useState<Set<string>>(new Set());

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
      refetch();
    } catch (err: any) {
      addToast({ title: err.message || 'Lỗi nạp file', variant: 'error' });
    }
  };

  const handleDeleteDocument = async (filename: string) => {
    openConfirm({
      title: 'Xác nhận xóa tài liệu',
      message: `Bạn có chắc muốn xóa tài liệu "${filename}"? Hành động này sẽ xóa cả tài liệu và tất cả các chunks liên quan trong vector store.`,
      confirmText: 'Xóa',
      cancelText: 'Hủy',
      variant: 'danger',
      onConfirm: async () => {
        setDeletingDocs(prev => new Set(prev).add(filename));
        try {
          const result = await knowledgeApi.deleteDocument(filename);
          addToast({ 
            title: 'Xóa thành công!', 
            variant: 'success',
            message: `Đã xóa ${result.deleted_chunks} chunks`
          });
          refetch();
        } catch (err: any) {
          addToast({ title: err.message || 'Lỗi xóa tài liệu', variant: 'error' });
        } finally {
          setDeletingDocs(prev => {
            const next = new Set(prev);
            next.delete(filename);
            return next;
          });
        }
      },
    });
  };

  const handleRefresh = () => {
    refetch();
    addToast({ title: 'Đang làm mới dữ liệu...', variant: 'info' });
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
        <button onClick={handleRefresh} className={styles.btnSecondary} style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <RefreshCw style={{ width: 14, height: 14 }} />
          Làm mới
        </button>
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
          📤 Nạp tài liệu mới (.docx)
        </h3>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <input
            type="file"
            accept=".doc,.docx"
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
      </div>

      {/* Documents Table */}
      <div className={`${styles.card} ${styles.cardPad}`}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
          <h3 className={styles.cardTitle} style={{ marginBottom: 0, fontSize: 14 }}>
            📚 Tài liệu đã nạp ({documents.length})
          </h3>
          <div style={{ fontSize: 11, color: '#64748b', display: 'flex', alignItems: 'center', gap: 4 }}>
            <AlertCircle style={{ width: 12, height: 12 }} />
            Xóa tài liệu sẽ xóa luôn các chunks trong vector store
          </div>
        </div>

        {documents.length === 0 ? (
          <div style={{ 
            textAlign: 'center', 
            padding: '32px 16px', 
            color: '#64748b',
            border: '1px dashed rgba(255,255,255,0.1)',
            borderRadius: 8
          }}>
            <FileText style={{ width: 32, height: 32, margin: '0 auto 8px', opacity: 0.5 }} />
            <p>Chưa có tài liệu nào được nạp</p>
            <p style={{ fontSize: 12, marginTop: 4 }}>Tải lên file .docx để bắt đầu</p>
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ 
              width: '100%', 
              borderCollapse: 'collapse',
              fontSize: 12
            }}>
              <thead>
                <tr style={{ 
                  borderBottom: '1px solid rgba(255,255,255,0.1)',
                  color: '#94a3b8'
                }}>
                  <th style={{ padding: '8px 12px', textAlign: 'left', fontWeight: 500 }}>Tên tài liệu</th>
                  <th style={{ padding: '8px 12px', textAlign: 'right', fontWeight: 500 }}>Kích thước</th>
                  <th style={{ padding: '8px 12px', textAlign: 'center', fontWeight: 500 }}>Hành động</th>
                </tr>
              </thead>
              <tbody>
                {documents.map((doc: any, idx: number) => (
                  <tr 
                    key={idx} 
                    style={{ 
                      borderBottom: '1px solid rgba(255,255,255,0.05)',
                      transition: 'background 0.15s'
                    }}
                    onMouseEnter={(e) => (e.currentTarget.style.background = 'rgba(255,255,255,0.02)')}
                    onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                  >
                    <td style={{ padding: '10px 12px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <FileText style={{ width: 16, height: 16, color: '#818cf8', flexShrink: 0 }} />
                        <span style={{ 
                          color: '#fff', 
                          overflow: 'hidden', 
                          textOverflow: 'ellipsis', 
                          whiteSpace: 'nowrap',
                          maxWidth: '300px'
                        }}>
                          {doc.filename}
                        </span>
                      </div>
                    </td>
                    <td style={{ padding: '10px 12px', textAlign: 'right', color: '#64748b' }}>
                      {doc.size_kb} KB
                    </td>
                    <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                      <button
                        onClick={() => handleDeleteDocument(doc.filename)}
                        disabled={deletingDocs.has(doc.filename)}
                        style={{
                          background: 'rgba(239, 68, 68, 0.1)',
                          border: '1px solid rgba(239, 68, 68, 0.2)',
                          borderRadius: 6,
                          padding: '6px 10px',
                          cursor: deletingDocs.has(doc.filename) ? 'not-allowed' : 'pointer',
                          color: deletingDocs.has(doc.filename) ? '#64748b' : '#ef4444',
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: 4,
                          fontSize: 11,
                          transition: 'all 0.15s',
                          opacity: deletingDocs.has(doc.filename) ? 0.5 : 1
                        }}
                        onMouseEnter={(e) => {
                          if (!deletingDocs.has(doc.filename)) {
                            e.currentTarget.style.background = 'rgba(239, 68, 68, 0.2)';
                          }
                        }}
                        onMouseLeave={(e) => {
                          if (!deletingDocs.has(doc.filename)) {
                            e.currentTarget.style.background = 'rgba(239, 68, 68, 0.1)';
                          }
                        }}
                      >
                        {deletingDocs.has(doc.filename) ? (
                          <>
                            <RefreshCw style={{ width: 12, height: 12, animation: 'spin 1s linear infinite' }} />
                            Đang xóa...
                          </>
                        ) : (
                          <>
                            <Trash2 style={{ width: 12, height: 12 }} />
                            Xóa
                          </>
                        )}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Info Card */}
      <div className={`${styles.card} ${styles.cardPad}`} style={{ background: 'rgba(99, 102, 241, 0.1)', border: '1px solid rgba(99, 102, 241, 0.2)' }}>
        <h4 style={{ fontSize: 13, fontWeight: 600, color: '#a5b4fc', marginBottom: 8 }}>
          💡 Hướng dẫn sử dụng
        </h4>
        <ul style={{ fontSize: 12, color: '#94a3b8', margin: 0, paddingLeft: 16, lineHeight: 1.8 }}>
          <li>Tải lên file <strong style={{ color: '#fff' }}>.docx</strong> để thêm vào kho tri thức</li>
          <li>Hệ thống sẽ tự động chia nhỏ tài liệu thành các chunks để vector hoá</li>
          <li>Khi xóa tài liệu, toàn bộ chunks liên quan trong vector store cũng sẽ bị xóa</li>
          <li>AI sẽ sử dụng kho tri thức này để trả lời câu hỏi của khách hàng</li>
        </ul>
      </div>

      <style jsx global>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}

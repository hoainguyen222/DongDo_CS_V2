'use client';

import React, { useState, useEffect, useMemo } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { BellRing, ArrowRight, X } from 'lucide-react';
import { useAlertConfig, useCases } from '@/lib/hooks/useApi';

export const MessageAlertBanner: React.FC = () => {
  const router = useRouter();
  const pathname = usePathname();
  const [dismissedAt, setDismissedAt] = useState<number | null>(null);

  // 1-second ticker to dynamically re-evaluate overdue time without needing page reload
  const [now, setNow] = useState<number>(Date.now());

  useEffect(() => {
    const timer = setInterval(() => {
      setNow(Date.now());
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  const { data: alertConfig } = useAlertConfig();
  const { data: casesData } = useCases('', 1, 100, '', {
    refetchInterval: 3000, // Poll cases every 3s to stay synced
  });

  // Calculate overdue unreplied cases in real-time based on `now` ticker
  const overdueCases = useMemo(() => {
    if (!alertConfig?.is_enabled || !casesData?.cases) return [];

    const timeoutSec = alertConfig.timeout_seconds || 60;

    return casesData.cases.filter((c) => {
      // Only consider cases that need human response (NEEDS_HUMAN_CS or unreplied by CS)
      if (c.status === 'RESOLVED') return false;

      // Check if last message was sent by guest and unreplied
      const isUnreplied = c.status === 'NEEDS_HUMAN_CS' || c.last_sender_type === 'guest';
      if (!isUnreplied) return false;

      const updatedAt = new Date(c.updated_at).getTime();
      const elapsedSec = (now - updatedAt) / 1000;

      return elapsedSec >= timeoutSec;
    });
  }, [alertConfig, casesData, now]);

  // If feature disabled, no overdue cases, or user manually dismissed within last 30s
  if (
    !alertConfig?.is_enabled ||
    overdueCases.length === 0 ||
    (dismissedAt && Date.now() - dismissedAt < 30000)
  ) {
    return null;
  }

  const contentText =
    alertConfig.alert_content ||
    '⚠️ Có tin nhắn khách hàng chờ trả lời quá thời gian quy định!';

  return (
    <div
      style={{
        position: 'fixed',
        bottom: '24px',
        right: '24px',
        zIndex: 9999,
        maxWidth: '480px',
        width: 'calc(100vw - 48px)',
        background: 'linear-gradient(135deg, #7f1d1d, #991b1b, #b91c1c)',
        border: '1px solid #ef4444',
        borderRadius: '14px',
        padding: '14px 18px',
        color: '#ffffff',
        boxShadow: '0 12px 32px rgba(239, 68, 68, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.15)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: '12px',
        animation: 'pulseAlert 1.5s infinite alternate',
      }}
    >
      <style>{`
        @keyframes pulseAlert {
          0% {
            transform: scale(1);
            box-shadow: 0 12px 32px rgba(239, 68, 68, 0.35);
          }
          100% {
            transform: scale(1.02);
            box-shadow: 0 16px 40px rgba(239, 68, 68, 0.65), 0 0 20px rgba(239, 68, 68, 0.8);
          }
        }
      `}</style>

      <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
        <div
          style={{
            background: 'rgba(255, 255, 255, 0.2)',
            borderRadius: '50%',
            padding: '8px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            animation: 'ring 1s infinite ease-in-out',
          }}
        >
          <BellRing size={22} className="text-white" />
        </div>
        <div>
          <div style={{ fontWeight: 800, fontSize: '13px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
            🔥 Cảnh báo tin nhắn quá hạn ({overdueCases.length} ca)
          </div>
          <div style={{ fontSize: '12px', opacity: 0.95, marginTop: '2px', lineHeight: 1.4 }}>
            {contentText}
          </div>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        {pathname !== '/admin/inbox' && (
          <button
            onClick={() => router.push('/admin/inbox')}
            style={{
              background: '#ffffff',
              color: '#991b1b',
              border: 'none',
              borderRadius: '8px',
              padding: '6px 12px',
              fontSize: '12px',
              fontWeight: 700,
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
              whiteSpace: 'nowrap',
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.2)',
            }}
          >
            <span>Inbox</span>
            <ArrowRight size={14} />
          </button>
        )}
        <button
          onClick={() => setDismissedAt(Date.now())}
          style={{
            background: 'rgba(255, 255, 255, 0.15)',
            border: 'none',
            borderRadius: '6px',
            color: '#ffffff',
            padding: '4px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
          title="Tạm ẩn 30s"
        >
          <X size={16} />
        </button>
      </div>
    </div>
  );
};

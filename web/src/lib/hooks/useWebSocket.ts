/**
 * useWebSocket — custom hook kết hợp WSClient với TanStack Query invalidation
 * Tự động invalidate queries khi có sự kiện realtime từ WebSocket
 */

import { useEffect, useRef, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { WSClient } from '@/lib/ws';
import { queryKeys } from '@/lib/hooks/useApi';

interface UseWebSocketOptions {
  sessionId: string;
  username: string;
  role: string;
  enabled?: boolean;
  onMessage?: (event: any) => void;
  onCaseUpdate?: () => void;
  onTyping?: (typing: boolean) => void;
  onCallOffer?: (data: any) => void;
  onCallAnswer?: (data: any) => void;
  onCallEnd?: () => void;
}

export function useWebSocket({
  sessionId,
  username,
  role,
  enabled = true,
  onMessage,
  onCaseUpdate,
  onTyping,
  onCallOffer,
  onCallAnswer,
  onCallEnd,
}: UseWebSocketOptions) {
  const wsRef = useRef<WSClient | null>(null);
  const qc = useQueryClient();

  const connect = useCallback(() => {
    if (!enabled) return;

    const ws = new WSClient(sessionId, username, role);
    wsRef.current = ws;
    ws.connect();

    ws.on('message', (event: any) => {
      onMessage?.(event);
      // Auto-invalidate chat history
      if (event.session_id) {
        qc.invalidateQueries({ queryKey: queryKeys.chatHistory(event.session_id) });
      }
    });

    ws.on('typing', (event: any) => {
      onTyping?.(event.payload?.typing === true);
    });

    ws.on('case_update', () => {
      onCaseUpdate?.();
      qc.invalidateQueries({ queryKey: ['cases'] });
      qc.invalidateQueries({ queryKey: ['customers'] });
      qc.invalidateQueries({ queryKey: ['analytics'] });
    });

    ws.on('learning_update', () => {
      qc.invalidateQueries({ queryKey: ['pendingLearning'] });
      qc.invalidateQueries({ queryKey: ['knowledge'] });
    });

    ws.on('call_offer', (event: any) => {
      onCallOffer?.(event);
    });

    ws.on('call_answer', (event: any) => {
      onCallAnswer?.(event);
    });

    ws.on('call_end', () => {
      onCallEnd?.();
      qc.invalidateQueries({ queryKey: ['voiceCalls'] });
      qc.invalidateQueries({ queryKey: ['cases'] });
    });

    return ws;
  }, [sessionId, username, role, enabled]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const ws = connect();
    return () => {
      ws?.disconnect();
      wsRef.current = null;
    };
  }, [connect]);

  const send = useCallback(
    (
      type: any,
      content: string,
      payload?: any,
      targetSessionId?: string,
      clientMsgId?: string
    ) => {
      return wsRef.current?.send(type, content, payload, targetSessionId, clientMsgId) ?? false;
    },
    []
  );

  return { ws: wsRef.current, send };
}

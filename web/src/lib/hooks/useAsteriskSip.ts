'use client';

/**
 * useAsteriskSip — React hook that wraps AsteriskSipClient and exposes
 * its lifecycle to a layout-level component.
 *
 * Connects on mount when `enabled` is true and a config is available,
 * tears down on unmount.  Incoming INVITEs trigger `onIncomingCall`,
 * session terminations trigger `onSessionEnded`.
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { AsteriskSipClient, type AsteriskSipConfig, type IncomingCallInfo } from '@/lib/asteriskSip';

export interface UseAsteriskSipOptions {
  /** When false, no connection is attempted (used for guest/public pages). */
  enabled?: boolean;
  /** When null, the hook waits for a config from a parent. */
  config: AsteriskSipConfig | null;
  onIncomingCall?: (info: IncomingCallInfo) => void;
  onSessionEnded?: (cause: string) => void;
  onError?: (err: Error) => void;
}

export interface UseAsteriskSipResult {
  state: 'idle' | 'connecting' | 'connected' | 'failed' | 'disconnected';
  error: string | null;
  answer: () => Promise<void>;
  decline: () => Promise<void>;
  hangup: () => Promise<void>;
}

export function useAsteriskSip(opts: UseAsteriskSipOptions): UseAsteriskSipResult {
  const { enabled = true, config, onIncomingCall, onSessionEnded, onError } = opts;
  const clientRef = useRef<AsteriskSipClient | null>(null);
  const [state, setState] = useState<UseAsteriskSipResult['state']>('idle');
  const [error, setError] = useState<string | null>(null);

  // Keep the latest callbacks in refs so we don't churn the connect loop
  // when the parent re-renders.
  const onIncomingRef = useRef(onIncomingCall);
  const onEndedRef = useRef(onSessionEnded);
  const onErrorRef = useRef(onError);
  useEffect(() => {
    onIncomingRef.current = onIncomingCall;
    onEndedRef.current = onSessionEnded;
    onErrorRef.current = onError;
  }, [onIncomingCall, onSessionEnded, onError]);

  useEffect(() => {
    if (!enabled || !config) return;
    if (clientRef.current) return;

    let cancelled = false;
    setState('connecting');
    setError(null);

    const client = new AsteriskSipClient(config);
    client.setOnIncomingCall((info) => {
      onIncomingRef.current?.(info);
    });
    client.setOnSessionEnded((cause) => {
      onEndedRef.current?.(cause);
    });
    clientRef.current = client;

    client
      .connect()
      .then(() => {
        if (cancelled) return;
        setState('connected');
      })
      .catch((err) => {
        if (cancelled) return;
        const msg = err instanceof Error ? err.message : String(err);
        setError(msg);
        setState('failed');
        onErrorRef.current?.(err instanceof Error ? err : new Error(msg));
      });

    return () => {
      cancelled = true;
      if (clientRef.current) {
        clientRef.current.disconnect().catch(() => {});
        clientRef.current = null;
      }
      setState('disconnected');
    };
  }, [enabled, config]);

  const answer = useCallback(async () => {
    const c = clientRef.current;
    if (!c) throw new Error('SIP client not connected');
    await c.answer();
  }, []);

  const decline = useCallback(async () => {
    const c = clientRef.current;
    if (!c) throw new Error('SIP client not connected');
    await c.decline();
  }, []);

  const hangup = useCallback(async () => {
    const c = clientRef.current;
    if (!c) return;
    await c.hangup();
  }, []);

  return { state, error, answer, decline, hangup };
}

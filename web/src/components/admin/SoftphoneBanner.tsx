'use client';

/**
 * SoftphoneBanner — sip.js powered softphone UI for incoming calls.
 *
 * Receives incoming-call notifications from useAsteriskSip (which wraps
 * sip.js SimpleUser and connects directly to Asterisk over WSS) and
 * shows a banner with Answer / Decline buttons.  When the agent clicks
 * Answer we hand off to sip.js, which sends SIP 200 OK to Asterisk and
 * negotiates the DTLS-SRTP media stream — Asterisk then bridges the
 * guest and agent channels.
 *
 * The audio element here is only used to surface the local stream
 * volume (muted) so the OS-level microphone permission is requested
 * by the browser when the user clicks Answer.
 */

import React, { useEffect, useRef, useState, useCallback } from 'react';
import { Phone, PhoneOff } from 'lucide-react';
import styles from './GuestCallFloatBanner.module.scss'; // reuse the floating banner styles
import type { IncomingCallInfo } from '@/lib/asteriskSip';

export interface SoftphoneBannerProps {
  /** Current incoming call or null. */
  incoming: IncomingCallInfo | null;
  /** SIP client state. */
  state: 'idle' | 'connecting' | 'connected' | 'failed' | 'disconnected';
  /** Error message if state === 'failed'. */
  error: string | null;
  /** Triggers sip.js answer(). */
  onAnswer: () => Promise<void>;
  /** Triggers sip.js decline(). */
  onDecline: () => Promise<void>;
  /** Triggers sip.js hangup(). */
  onHangup: () => Promise<void>;
  /** True when there's an active SIP session (answered). */
  active: boolean;
  /** Optional page label for context. */
  pageLabel?: string;
}

export function SoftphoneBanner(props: SoftphoneBannerProps) {
  const { incoming, state, error, onAnswer, onDecline, onHangup, active, pageLabel } = props;
  const [busy, setBusy] = useState(false);
  const audioRef = useRef<HTMLAudioElement | null>(null);

  const handleAnswer = useCallback(async () => {
    if (busy) return;
    setBusy(true);
    try {
      await onAnswer();
    } catch (err) {
      console.error('sip answer failed', err);
    } finally {
      setBusy(false);
    }
  }, [busy, onAnswer]);

  const handleDecline = useCallback(async () => {
    if (busy) return;
    setBusy(true);
    try {
      await onDecline();
    } catch (err) {
      console.error('sip decline failed', err);
    } finally {
      setBusy(false);
    }
  }, [busy, onDecline]);

  const handleHangup = useCallback(async () => {
    if (busy) return;
    setBusy(true);
    try {
      await onHangup();
    } catch (err) {
      console.error('sip hangup failed', err);
    } finally {
      setBusy(false);
    }
  }, [busy, onHangup]);

  if (!incoming && !active) return null;

  return (
    <>
      {error && state === 'failed' && (
        <div className={styles.banner} style={{ background: '#fee' }} role="alert">
          <div className={styles.text}>
            <div className={styles.label}>⚠️ Softphone chưa kết nối</div>
            <div className={styles.caller}>{error}</div>
          </div>
        </div>
      )}

      {incoming && !active && (
        <div className={styles.banner} role="alert" aria-live="assertive">
          <div className={styles.icon}>
            <Phone style={{ width: 22, height: 22 }} />
          </div>
          <div className={styles.text}>
            <div className={styles.label}>📞 Cuộc gọi SIP đang đến!</div>
            <div className={styles.caller}>
              {incoming.callerName} ({incoming.callerNumber})
            </div>
            {pageLabel && (
              <div className={styles.pageHint}>Trang hiện tại: {pageLabel}</div>
            )}
          </div>
          <div className={styles.actions}>
            <button
              type="button"
              onClick={handleAnswer}
              className={styles.acceptBtn}
              disabled={busy}
            >
              <Phone style={{ width: 14, height: 14 }} />
              <span>Nghe</span>
            </button>
            <button
              type="button"
              onClick={handleDecline}
              className={styles.declineBtn}
              disabled={busy}
              aria-label="Từ chối"
            >
              <PhoneOff style={{ width: 14, height: 14 }} />
            </button>
          </div>
        </div>
      )}

      {active && (
        <div className={styles.banner} role="status">
          <div className={styles.icon} style={{ background: '#10b981' }}>
            <Phone style={{ width: 22, height: 22 }} />
          </div>
          <div className={styles.text}>
            <div className={styles.label}>🟢 Đang trong cuộc gọi (SIP)</div>
            <div className={styles.caller}>
              {incoming?.callerName ?? 'Đang gọi'} ({incoming?.callerNumber ?? '...'})
            </div>
          </div>
          <div className={styles.actions}>
            <button
              type="button"
              onClick={handleHangup}
              className={styles.declineBtn}
              disabled={busy}
              aria-label="Kết thúc"
            >
              <PhoneOff style={{ width: 14, height: 14 }} />
              <span style={{ marginLeft: 6 }}>Kết thúc</span>
            </button>
          </div>
          {/* Hidden audio element so the browser holds the mic permission. */}
          <audio ref={audioRef} autoPlay muted style={{ display: 'none' }} />
        </div>
      )}
    </>
  );
}

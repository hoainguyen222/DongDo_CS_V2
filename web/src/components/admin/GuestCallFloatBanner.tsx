'use client';

/**
 * GuestCallFloatBanner — Sticky floating call ring banner.
 *
 * Designed to be mounted on public pages (like /admin/login) so CSKH staff
 * can see+accept incoming guest calls WITHOUT first navigating to /admin/inbox.
 *
 * On accept (click "Mở softphone" or "Nghe ngay"):
 *   - If user is on /admin/login → we navigate to /admin/inbox first.
 *   - Then we trigger the same end-to-end: POST /api/voice/accept/{call_id}
 *     and open the configured softphone SIP URI (or directly the inline
 *     /admin/inbox call panel) so the agent can pick up.
 *
 * On decline: marks the call as missed via /api/voice/missed.
 */

import React, { useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { Phone, PhoneOff, ExternalLink, X } from 'lucide-react';
import type { GuestCallRingEvent } from '@/lib/hooks/useGuestCallRing';
import { useAuthStore } from '@/lib/stores/authStore';
import { resolveAgentExtension } from '@/lib/utils';
import styles from './GuestCallFloatBanner.module.scss';

function buildSipUri(target?: string, asteriskHost?: string): string {
  const host = asteriskHost || (typeof window !== 'undefined' ? window.location.hostname : 'asterisk');
  // SIP URI used by OS default handler to launch softphone. Browsers without a
  // registered SIP handler will simply prompt the user.
  return `sip:${target || 'guest'}@${host}`;
}

export interface GuestCallFloatBannerProps {
  ring: GuestCallRingEvent | null;
  onDismiss: () => void;
  /** Optional: name of the current page route to render a contextual helper. */
  pageLabel?: string;
  /** Asterisk host used to build the SIP URI. Defaults to current hostname. */
  asteriskHost?: string;
  /** Disable "Nghe ngay" button (e.g. when not authenticated). */
  disableAccept?: boolean;
  /** Override default labels. */
  labels?: {
    title?: string;
    caller?: string;
    phone?: string;
    accept?: string;
    decline?: string;
    loginFirst?: string;
  };
}

const DEFAULT_LABELS = {
  title: '📞 Cuộc gọi đang đến!',
  caller: 'Khách hàng',
  phone: 'SĐT',
  accept: 'Mở softphone',
  decline: 'Từ chối',
  loginFirst: 'Đăng nhập để nghe',
};

export function GuestCallFloatBanner({
  ring,
  onDismiss,
  pageLabel,
  asteriskHost,
  disableAccept,
  labels: labelOverrides,
}: GuestCallFloatBannerProps) {
  const router = useRouter();
  const labels = { ...DEFAULT_LABELS, ...labelOverrides };

  const handleDecline = useCallback(async () => {
    if (!ring) return;
    // Login page banner can NOT call protected admin endpoints without a
    // session token — when disableAccept is set we just close the banner.
    if (!disableAccept) {
      try {
        if (ring.call_id) {
          const { api } = await import('@/lib/api');
          await api.markMissedCall(ring.call_id, ring.session_id).catch(() => {});
        }
      } catch (_) {
        /* noop */
      }
    }
    onDismiss();
  }, [ring, onDismiss, disableAccept]);

  const handleAccept = useCallback(async () => {
    if (!ring) return;
    // Same guard — accept requires authenticated staff.
    if (!disableAccept) {
      const target = ring.phone || ring.caller_id;
      const sipUri = buildSipUri(target, asteriskHost);
      try {
        if (ring.call_id) {
          const { api } = await import('@/lib/api');
          const username = useAuthStore.getState().user?.username;
          const agentExtension = resolveAgentExtension(username);
          await api.webrtcAcceptCall(ring.call_id, ring.session_id, agentExtension).catch(() => {});
        }
      } catch (_) {
        /* noop */
      }
      try {
        if (typeof window !== 'undefined') {
          window.location.href = sipUri;
        }
      } catch (_) {
        /* noop */
      }
    }
  }, [ring, asteriskHost, disableAccept]);

  const handleLoginThenAccept = useCallback(() => {
    onDismiss();
    router.push(
      `/admin/login?next=${encodeURIComponent(
        `/admin/inbox?session=${ring?.session_id ?? ''}&call_id=${ring?.call_id ?? ''}`
      )}`
    );
  }, [onDismiss, router, ring]);

  if (!ring) return null;

  const showLoginHint = !!disableAccept;

  return (
    <>
      <div className={styles.banner} role="alert" aria-live="assertive">
        <div className={styles.icon}>
          <Phone style={{ width: 22, height: 22 }} />
        </div>

        <div className={styles.text}>
          <div className={styles.label}>{labels.title}</div>
          <div className={styles.caller}>
            {ring.caller_id || labels.caller}
            {ring.phone && (
              <span className={styles.phone}>
                {' '}
                • {labels.phone}: <strong>{ring.phone}</strong>
              </span>
            )}
          </div>
          {pageLabel && <div className={styles.pageHint}>Trang hiện tại: {pageLabel}</div>}
        </div>

        <div className={styles.actions}>
          {showLoginHint ? (
            <button
              type="button"
              onClick={handleLoginThenAccept}
              className={styles.acceptBtn}
            >
              <ExternalLink style={{ width: 14, height: 14 }} />
              <span>{labels.loginFirst}</span>
            </button>
          ) : (
            <button
              type="button"
              onClick={handleAccept}
              className={styles.acceptBtn}
            >
              <ExternalLink style={{ width: 14, height: 14 }} />
              <span>{labels.accept}</span>
            </button>
          )}
          <button
            type="button"
            onClick={handleDecline}
            className={styles.declineBtn}
            aria-label={labels.decline}
          >
            <PhoneOff style={{ width: 14, height: 14 }} />
          </button>
          <button
            type="button"
            onClick={onDismiss}
            className={styles.closeBtn}
            aria-label="Đóng"
          >
            <X style={{ width: 14, height: 14 }} />
          </button>
        </div>
      </div>
    </>
  );
}

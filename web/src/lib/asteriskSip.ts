'use client';

/**
 * asteriskSip.ts — sip.js wrapper for Asterisk-native WebRTC.
 *
 * Browsers register directly with Asterisk over WebSocket (WSS) using
 * sip.js.  Asterisk handles all the SIP signaling and DTLS-SRTP media
 * itself, so the browser just needs a SimpleUser instance per agent.
 *
 * Lifecycle:
 *   1. `connect()`    — opens the WebSocket and registers the agent's SIP extension.
 *   2. `setOnIncomingCall(handler)` — wired so admin UI can show "ringing".
 *   3. `answer()` / `decline()` — accept or reject an inbound INVITE.
 *   4. `hangup()`     — end the current session.
 *   5. `disconnect()` — unregister and close the socket.
 *
 * Configuration is read from NEXT_PUBLIC_ASTERISK_* environment variables
 * (see .env.example).  When Asterisk is not configured the client is a
 * no-op — callers should still check `isConnected()` before wiring handlers.
 *
 * Compatibility: written against sip.js 0.21.2.
 */

import { SimpleUser, SimpleUserDelegate } from 'sip.js/lib/platform/web/simple-user/index.js';

export interface AsteriskSipConfig {
  /** WebSocket URL of the PJSIP WSS transport, e.g. "wss://asterisk.local:8089" */
  websocketUrl: string;
  /** SIP server host (used to build the AOR), e.g. "asterisk.local" */
  sipServer: string;
  /** SIP extension to register as (e.g. "1001") */
  agentExtension: string;
  /** SIP auth password (must match pjsip.conf auth for this extension) */
  agentPassword: string;
  /** Optional STUN servers for NAT traversal (defaults to Google free STUN) */
  stunServers?: string[];
  /** Optional display name shown to the remote party */
  displayName?: string;
}

export interface IncomingCallInfo {
  sessionId: string;
  callerNumber: string;
  callerName: string;
}

export type IncomingCallHandler = (info: IncomingCallInfo) => void;
export type SessionEndedHandler = (cause: string) => void;

const DEFAULT_STUN = [
  { urls: 'stun:stun.l.google.com:19302' },
  { urls: 'stun:stun1.l.google.com:19302' },
];

/**
 * AsteriskSipClient — thin wrapper around sip.js SimpleUser tailored for
 * the DongDo agent softphone flow.  One instance per logged-in agent.
 *
 * NOTE: sip.js 0.21.x hides the underlying `Session` object and exposes
 * only delegate callbacks.  We therefore track call state via flags
 * driven by `onCallReceived` / `onCallHangup` / `onServerConnect` /
 * `onServerDisconnect`.  Detailed caller-id (number/name) is *not*
 * available through SimpleUser in 0.21.x — admin UI receives a
 * placeholder; the real caller info is meant to arrive via the backend
 * WS event (`call.incoming`) for ARI integration.
 */
export class AsteriskSipClient {
  private simpleUser: SimpleUser | null = null;
  private incomingHandler: IncomingCallHandler | null = null;
  private endedHandler: SessionEndedHandler | null = null;
  private connected = false;
  private ringing = false;

  constructor(private config: AsteriskSipConfig) {}

  /** True when the WebSocket + SIP registration are live. */
  isConnected(): boolean {
    return this.connected && this.simpleUser?.isConnected() === true;
  }

  /** True when a session is established (incoming or outgoing). */
  hasActiveCall(): boolean {
    return this.ringing;
  }

  /** Register the SIP extension and start listening for INVITEs. */
  async connect(): Promise<void> {
    if (this.simpleUser) {
      throw new Error('AsteriskSipClient already initialised');
    }
    if (!this.config.websocketUrl) {
      throw new Error('websocketUrl is required');
    }

    const stunServers =
      this.config.stunServers && this.config.stunServers.length > 0
        ? this.config.stunServers.map((s) => ({ urls: s }))
        : DEFAULT_STUN;

    const aor = `sip:${this.config.agentExtension}@${this.config.sipServer}`;

    const delegate: SimpleUserDelegate = {
      onServerConnect: () => {
        this.connected = true;
      },
      onServerDisconnect: () => {
        this.connected = false;
      },
      onCallReceived: () => {
        this.ringing = true;
        if (!this.incomingHandler) return;
        const info: IncomingCallInfo = {
          sessionId: `${Date.now()}`,
          callerNumber: 'unknown',
          callerName: 'Incoming call',
        };
        try {
          this.incomingHandler(info);
        } catch (err) {
          console.error('incomingCall handler error', err);
        }
      },
      onCallHangup: () => {
        this.ringing = false;
        if (this.endedHandler) {
          try {
            this.endedHandler('hangup');
          } catch (_) {
            /* ignore */
          }
        }
      },
    };

    const simpleUser = new SimpleUser(this.config.websocketUrl, {
      aor,
      media: {
        constraints: { audio: true, video: false },
      },
      userAgentOptions: {
        authorizationUsername: this.config.agentExtension,
        authorizationPassword: this.config.agentPassword,
        displayName: this.config.displayName ?? `Agent ${this.config.agentExtension}`,
        // sip.js 0.21.x still needs transportOptions for ICE/STUN.
        transportOptions: {
          iceServers: stunServers,
        } as any,
      },
      delegate,
    });

    this.simpleUser = simpleUser;
    await simpleUser.connect();
    await simpleUser.register();
  }

  /** Accept an incoming INVITE (already ringing). */
  async answer(): Promise<void> {
    if (!this.simpleUser) throw new Error('SIP client not connected');
    await this.simpleUser.answer();
  }

  /** Reject an incoming INVITE before answering. */
  async decline(): Promise<void> {
    if (!this.simpleUser) throw new Error('SIP client not connected');
    await this.simpleUser.decline();
  }

  /** Hang up the current session, if any. */
  async hangup(): Promise<void> {
    if (!this.simpleUser) return;
    try {
      await this.simpleUser.hangup();
    } catch (err) {
      // hangup may fail if the session is already terminated — that's fine.
      console.warn('hangup error (ignored):', err);
    }
  }

  /** Unregister and close the WebSocket. */
  async disconnect(): Promise<void> {
    if (!this.simpleUser) return;
    try {
      await this.simpleUser.unregister();
    } catch (_) {
      /* ignore */
    }
    try {
      await this.simpleUser.disconnect();
    } catch (_) {
      /* ignore */
    }
    this.simpleUser = null;
    this.connected = false;
    this.ringing = false;
  }

  /** Subscribe to inbound INVITEs. Replaces any previous handler. */
  setOnIncomingCall(handler: IncomingCallHandler | null): void {
    this.incomingHandler = handler;
  }

  /** Subscribe to session termination events. */
  setOnSessionEnded(handler: SessionEndedHandler | null): void {
    this.endedHandler = handler;
  }
}

// ============================================================================
// Module-level helpers
// ============================================================================

/**
 * Read the SIP config from NEXT_PUBLIC_* env vars.
 * Real credentials are injected per-user via /api/me/sip-config.
 */
export function resolveSipConfigFromEnv(): AsteriskSipConfig | null {
  if (typeof window === 'undefined') return null;
  const websocketUrl = process.env.NEXT_PUBLIC_ASTERISK_WSS_URL ?? '';
  const sipServer = process.env.NEXT_PUBLIC_ASTERISK_SIP_SERVER ?? '';
  const agentExtension = process.env.NEXT_PUBLIC_ASTERISK_AGENT_EXT ?? '';
  if (!websocketUrl || !sipServer || !agentExtension) return null;
  return {
    websocketUrl,
    sipServer,
    agentExtension,
    agentPassword: '', // password is injected per-user via /api/me/sip-config
    stunServers: (process.env.NEXT_PUBLIC_ASTERISK_STUN ?? '')
      .split(',')
      .filter(Boolean),
  };
}

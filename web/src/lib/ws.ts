import { WSEvent, WSEventType } from './types';

type MessageHandler = (event: WSEvent) => void;

// ── Singleton registry ─────────────────────────────────────
// Each unique (sessionID, userID, role) tuple maps to one shared
// WSClient. Without this, every component that calls
// `new WSClient('admin_inbox', ...)` opens its own WebSocket — the
// backend then sees N copies of the same connection, which causes
// every incoming WS event to fan out to N handlers (and was the root
// cause of duplicated floating banners stacking on top of each other).
const singletonRegistry = new Map<string, WSClient>();
const singletonRefCounts = new Map<string, number>();

function registryKey(sessionID: string, userID: string, role: string): string {
  return `${sessionID}::${userID}::${role}`;
}

/**
 * Get or create the shared WS client for the given identity. The caller
 * MUST pair this with `releaseWSClient` on unmount so the connection
 * closes only when the last consumer goes away.
 */
export function acquireWSClient(sessionID: string, userID: string, role: string = 'guest'): WSClient {
  const key = registryKey(sessionID, userID, role);
  let client = singletonRegistry.get(key);
  if (!client) {
    client = new WSClient(sessionID, userID, role);
    singletonRegistry.set(key, client);
    singletonRefCounts.set(key, 0);
    client.connect();
  }
  singletonRefCounts.set(key, (singletonRefCounts.get(key) ?? 0) + 1);
  return client;
}

/**
 * Decrement the ref-count for a singleton WS client. When the count
 * drops to zero (last consumer unmounted), the underlying socket is
 * closed so the backend doesn't keep an idle connection.
 */
export function releaseWSClient(sessionID: string, userID: string, role: string = 'guest'): void {
  const key = registryKey(sessionID, userID, role);
  const current = singletonRefCounts.get(key) ?? 0;
  const next = current - 1;
  if (next <= 0) {
    const client = singletonRegistry.get(key);
    if (client) {
      client.disconnect();
    }
    singletonRegistry.delete(key);
    singletonRefCounts.delete(key);
  } else {
    singletonRefCounts.set(key, next);
  }
}

/**
 * Look up a singleton without acquiring it (returns undefined if not yet
 * created). Useful when an effect wants to attach to an existing socket
 * without incrementing the ref-count.
 */
export function getSharedWSClient(sessionID: string, userID: string, role: string = 'guest'): WSClient | undefined {
  return singletonRegistry.get(registryKey(sessionID, userID, role));
}

export class WSClient {
  private ws: WebSocket | null = null;
  private url: string;
  private sessionID: string;
  private userID: string;
  private role: string;
  private handlers: Map<WSEventType | '*', Set<MessageHandler>> = new Map();
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectInterval = 1000;
  private isIntentionalClose = false;
  private pingTimer: any = null;

  constructor(sessionID: string, userID: string, role: string = 'guest') {
    this.sessionID = sessionID;
    this.userID = userID;
    this.role = role;
    const protocol = typeof window !== 'undefined' && window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const backendHost = typeof window !== 'undefined'
      ? (window.location.port === '3000' ? `${window.location.hostname}:8080` : window.location.host)
      : 'localhost:8080';
    this.url = `${protocol}//${backendHost}/ws?session_id=${encodeURIComponent(sessionID)}&user_id=${encodeURIComponent(userID)}&role=${encodeURIComponent(role)}`;
  }

  public connect(): void {
    if (typeof window === 'undefined') return;
    this.isIntentionalClose = false;

    try {
      this.ws = new WebSocket(this.url);

      this.ws.onopen = () => {
        this.reconnectAttempts = 0;
        this.reconnectInterval = 1000;
        this.startHeartbeat();
      };

      this.ws.onmessage = (event) => {
        try {
          const lines = event.data.split('\n');
          for (const line of lines) {
            if (!line.trim()) continue;
            const parsed: WSEvent = JSON.parse(line);
            this.emit(parsed);
          }
        } catch (err) {
          console.error('WS Parse Error:', err);
        }
      };

      this.ws.onclose = () => {
        this.stopHeartbeat();
        if (!this.isIntentionalClose) {
          this.scheduleReconnect();
        }
      };

      this.ws.onerror = (err) => {
        console.error('WS Connection Error:', err);
      };
    } catch (err) {
      console.error('Failed to create WebSocket:', err);
      this.scheduleReconnect();
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.warn('Max WebSocket reconnect attempts reached');
      return;
    }
    const delay = Math.min(this.reconnectInterval * Math.pow(1.5, this.reconnectAttempts), 15000);
    this.reconnectAttempts++;
    setTimeout(() => {
      this.connect();
    }, delay);
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.pingTimer = setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }));
      }
    }, 25000);
  }

  private stopHeartbeat(): void {
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
  }

  public send(type: WSEventType, content: string = '', payload?: any, targetSessionID?: string, clientMsgID?: string): boolean {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(
        JSON.stringify({
          type,
          session_id: targetSessionID || this.sessionID,
          content,
          payload,
          client_msg_id: clientMsgID,
        })
      );
      return true;
    }
    return false;
  }

  /**
   * sendTyping was removed - typing is now sent via REST API.
   * This method is kept as a no-op for backward compatibility with any legacy callers.
   * @deprecated Use REST API instead.
   */
  public sendTyping(): void {
    // Typing now sent via POST /api/chat/typing
  }

  public on(event: WSEventType | '*', handler: MessageHandler): () => void {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set());
    }
    this.handlers.get(event)!.add(handler);

    // Return unbind function
    return () => {
      const set = this.handlers.get(event);
      if (set) set.delete(handler);
    };
  }

  private emit(event: WSEvent): void {
    // Specific handlers
    const specific = this.handlers.get(event.type);
    if (specific) {
      specific.forEach((fn) => fn(event));
    }
    // Wildcard handlers
    const wildcard = this.handlers.get('*');
    if (wildcard) {
      wildcard.forEach((fn) => fn(event));
    }
  }

  public disconnect(): void {
    this.isIntentionalClose = true;
    this.stopHeartbeat();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}

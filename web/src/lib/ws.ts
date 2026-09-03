import { WSEvent, WSEventType } from './types';

type MessageHandler = (event: WSEvent) => void;

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

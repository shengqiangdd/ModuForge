import { getToken, isTokenExpiring, tryRefreshToken } from './api/client';

type WSEventCallback = (data: unknown) => void;

class WSClient {
  private ws: WebSocket | null = null;
  private listeners = new Map<string, Set<WSEventCallback>>();
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 20;
  private reconnectDelay = 1000;
  private closed = false;
  private pingInterval: ReturnType<typeof setInterval> | null = null;
  private connectTimeout: ReturnType<typeof setTimeout> | null = null;
  private connecting = false;
  private idleTimeout: ReturnType<typeof setTimeout> | null = null;
  private messageCount = 0;

  get readyState(): number {
    return this.ws?.readyState ?? WebSocket.CLOSED;
  }

  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  async connect() {
    // 已连接或正在连接中，跳过
    if (this.ws?.readyState === WebSocket.OPEN) return;
    if (this.ws?.readyState === WebSocket.CONNECTING) return;
    if (this.connecting) return;

    // 清理旧连接
    this.stopPing();
    this.stopIdleTimeout();
    if (this.connectTimeout) { clearTimeout(this.connectTimeout); this.connectTimeout = null; }
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.onmessage = null;
      try { this.ws.close(); } catch {}
      this.ws = null;
    }

    this.connecting = true;

    // Refresh token if expiring soon
    if (isTokenExpiring(300)) {
      const refreshed = await tryRefreshToken();
      if (!refreshed) {
        console.warn('[WS] token refresh failed, will retry later');
        this.connecting = false;
        this.scheduleReconnect();
        return;
      }
    }

    const token = getToken();
    if (!token) {
      console.warn('[WS] no token available, skipping connect');
      this.connecting = false;
      return;
    }

    this.closed = false;
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${location.host}/api/v1/ws?token=${encodeURIComponent(token)}`;

    try {
      this.ws = new WebSocket(url);
    } catch (e) {
      console.error('[WS] create failed:', e);
      this.connecting = false;
      this.scheduleReconnect();
      return;
    }

    // Connection timeout: 20s
    this.connectTimeout = setTimeout(() => {
      if (this.ws?.readyState === WebSocket.CONNECTING) {
        console.warn('[WS] connection timeout, closing');
        try { this.ws.close(); } catch {}
      }
    }, 20000);

    this.ws.onopen = () => {
      this.connecting = false;
      if (this.connectTimeout) {
        clearTimeout(this.connectTimeout);
        this.connectTimeout = null;
      }
      console.log('[WS] connected');
      this.reconnectAttempts = 0;
      this.messageCount = 0;
      this.startPing();
    };

    this.ws.onmessage = (event) => {
      this.messageCount++;
      this.resetIdleTimeout();
      try {
        const msg = JSON.parse(event.data);
        if (msg.event === 'ping') return;
        const callbacks = this.listeners.get(msg.event);
        if (callbacks) callbacks.forEach(cb => cb(msg.data));
        const allCallbacks = this.listeners.get('*');
        if (allCallbacks) allCallbacks.forEach(cb => cb(msg));
      } catch {}
    };

    this.ws.onclose = (event) => {
      this.connecting = false;
      if (this.connectTimeout) {
        clearTimeout(this.connectTimeout);
        this.connectTimeout = null;
      }
      this.stopPing();
      this.stopIdleTimeout();
      this.ws = null;
      if (!this.closed) {
        console.log('[WS] closed', event.code, event.reason);
        this.scheduleReconnect();
      }
    };

    this.ws.onerror = (event) => {
      console.error('[WS] error:', event);
      this.connecting = false;
    };
  }

  disconnect() {
    this.closed = true;
    this.connecting = false;
    this.stopPing();
    this.stopIdleTimeout();
    if (this.connectTimeout) { clearTimeout(this.connectTimeout); this.connectTimeout = null; }
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.onmessage = null;
      try { this.ws.close(); } catch {}
      this.ws = null;
    }
  }

  private startPing() {
    this.pingInterval = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        try { this.ws.send(JSON.stringify({ type: 'ping' })); } catch {}
      }
    }, 25000);
  }

  private stopPing() {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
  }

  // Idle timeout — disconnect if no messages received in 5 minutes
  private resetIdleTimeout() {
    this.stopIdleTimeout();
    this.idleTimeout = setTimeout(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        console.warn('[WS] idle timeout, reconnecting');
        try { this.ws.close(); } catch {}
      }
    }, 5 * 60 * 1000);
  }

  private stopIdleTimeout() {
    if (this.idleTimeout) {
      clearTimeout(this.idleTimeout);
      this.idleTimeout = null;
    }
  }

  private scheduleReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.warn('[WS] max reconnect attempts reached, stopping');
      return;
    }
    // 指数退避：2s, 3s, 4.5s, ... max 15s
    const delay = Math.min(2000 * Math.pow(1.5, this.reconnectAttempts), 15000);
    this.reconnectAttempts++;
    console.log(`[WS] reconnect in ${Math.round(delay)}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
    setTimeout(() => {
      if (!this.closed) this.connect();
    }, delay);
  }

  on(event: string, callback: WSEventCallback): () => void {
    if (!this.listeners.has(event)) this.listeners.set(event, new Set());
    this.listeners.get(event)!.add(callback);
    return () => this.listeners.get(event)?.delete(callback);
  }

  off(event: string, callback: WSEventCallback) {
    this.listeners.get(event)?.delete(callback);
  }

  // Clean up all listeners (call on app teardown)
  removeAllListeners() {
    this.listeners.clear();
  }
}

export const ws = new WSClient();

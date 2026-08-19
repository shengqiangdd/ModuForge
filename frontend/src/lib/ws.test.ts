import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ws } from './ws';

// Mock api/client，避免依赖 JWT 逻辑
vi.mock('./api/client', () => ({
  getToken: vi.fn(() => 'test-token'),
  isTokenExpiring: vi.fn(() => false),
  tryRefreshToken: vi.fn(() => Promise.resolve(true)),
}));

class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  url = '';
  readyState = FakeWebSocket.CONNECTING;
  onopen: ((e: unknown) => void) | null = null;
  onclose: ((e: unknown) => void) | null = null;
  onerror: ((e: unknown) => void) | null = null;
  onmessage: ((e: unknown) => void) | null = null;
  sent: string[] = [];
  closed = false;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }
  send(data: string) { this.sent.push(data); }
  close() {
    this.closed = true;
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.({ code: 1000, reason: 'manual' });
  }
  // 测试辅助：模拟服务端推消息
  emit(event: unknown) {
    this.onmessage?.({ data: JSON.stringify(event) });
  }
  // 测试辅助：模拟真实 WebSocket 的 onopen（打开后 readyState=OPEN）
  open() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.({});
  }
}

describe('ws client', () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    vi.stubGlobal('WebSocket', FakeWebSocket);
    // location stub
    Object.defineProperty(window, 'location', {
      value: { protocol: 'http:', host: 'localhost:8086' },
      configurable: true,
    });
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    ws.disconnect();
  });

  it('connect 建立 WebSocket 并携带 token', async () => {
    const p = ws.connect();
    expect(FakeWebSocket.instances.length).toBe(1);
    const inst = FakeWebSocket.instances[0];
    expect(inst.url).toContain('/api/v1/ws?token=test-token');
    await p;
  });

  it('onopen 后启动 ping 定时器', async () => {
    const p = ws.connect();
    const inst = FakeWebSocket.instances[0];
    inst.open();
    await p;
    expect(inst.readyState).toBe(FakeWebSocket.OPEN);
    // 25s 后应发 ping
    vi.advanceTimersByTime(26000);
    expect(inst.sent).toContain(JSON.stringify({ type: 'ping' }));
  });

  it('onmessage 按 event 分发到监听器', async () => {
    const p = ws.connect();
    const inst = FakeWebSocket.instances[0];
    inst.onopen?.({});
    const cb = vi.fn();
    ws.on('build:update', cb);
    inst.emit({ event: 'build:update', data: { id: 1 } });
    expect(cb).toHaveBeenCalledWith({ id: 1 });
    await p;
  });

  it('on("*") 通配监听收到完整消息', async () => {
    const p = ws.connect();
    const inst = FakeWebSocket.instances[0];
    inst.onopen?.({});
    const cb = vi.fn();
    ws.on('*', cb);
    inst.emit({ event: 'x', data: 42 });
    expect(cb).toHaveBeenCalledWith({ event: 'x', data: 42 });
    await p;
  });

  it('ping 事件不触发监听器', async () => {
    const p = ws.connect();
    const inst = FakeWebSocket.instances[0];
    inst.onopen?.({});
    const cb = vi.fn();
    ws.on('ping', cb);
    inst.emit({ event: 'ping' });
    expect(cb).not.toHaveBeenCalled();
    await p;
  });

  it('断线后按指数退避重连', async () => {
    const p = ws.connect();
    const inst = FakeWebSocket.instances[0];
    inst.onopen?.({});
    inst.onclose?.({ code: 1006, reason: 'abnormal' });
    // 第一次退避 2s
    expect(FakeWebSocket.instances.length).toBe(1);
    vi.advanceTimersByTime(2000);
    await Promise.resolve();
    expect(FakeWebSocket.instances.length).toBe(2);
    await p;
  });

  it('disconnect 后不重连', async () => {
    const p = ws.connect();
    const inst = FakeWebSocket.instances[0];
    inst.onopen?.({});
    ws.disconnect();
    inst.onclose?.({ code: 1006, reason: 'abnormal' });
    vi.advanceTimersByTime(10000);
    expect(FakeWebSocket.instances.length).toBe(1);
    await p;
  });

  it('on 返回取消订阅函数', async () => {
    const p = ws.connect();
    const inst = FakeWebSocket.instances[0];
    inst.onopen?.({});
    const cb = vi.fn();
    const off = ws.on('evt', cb);
    off();
    inst.emit({ event: 'evt', data: 1 });
    expect(cb).not.toHaveBeenCalled();
    await p;
  });
});

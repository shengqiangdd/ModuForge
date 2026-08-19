import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  getToken,
  setToken,
  clearToken,
  hasValidToken,
  isTokenExpiring,
  tryRefreshToken,
  authFetch,
  client,
  AuthError,
  streamRequest,
} from './client';

const TOKEN_KEY = 'moduforge_token';

// 生成一个带指定 exp(秒) 的伪 JWT，payload 合法可 base64 解码
function makeToken(expSec: number): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = btoa(JSON.stringify({ sub: 'u1', exp: expSec }));
  const sig = 'fake';
  return `${header}.${payload}.${sig}`;
}

function mockResponse(body: unknown, status = 200): Response {
  return new Response(
    typeof body === 'string' ? body : JSON.stringify(body),
    { status, headers: { 'Content-Type': 'application/json' } },
  );
}

describe('token helpers', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    vi.restoreAllMocks();
  });

  it('getToken returns empty when nothing stored', () => {
    expect(getToken()).toBe('');
  });

  it('setToken stores in localStorage and clears sessionStorage', () => {
    setToken('tok123', true);
    expect(localStorage.getItem(TOKEN_KEY)).toBe('tok123');
    expect(sessionStorage.getItem(TOKEN_KEY)).toBeNull();
  });

  it('clearToken removes both storages', () => {
    setToken('tok123', true);
    clearToken();
    expect(getToken()).toBe('');
  });

  it('hasValidToken rejects empty / malformed tokens', () => {
    expect(hasValidToken()).toBe(false);
    expect(hasValidToken()).toBe(false);
    // malformed: not 3 parts
    setToken('not-a-jwt', true);
    expect(hasValidToken()).toBe(false);
  });

  it('hasValidToken accepts an unexpired well-formed JWT', () => {
    const future = Math.floor(Date.now() / 1000) + 3600;
    setToken(makeToken(future), true);
    expect(hasValidToken()).toBe(true);
  });

  it('hasValidToken rejects an expired JWT (beyond grace period)', () => {
    const past = Math.floor(Date.now() / 1000) - 3600;
    setToken(makeToken(past), true);
    expect(hasValidToken()).toBe(false);
  });

  it('isTokenExpiring is true for no/expired token and false for far-future token', () => {
    expect(isTokenExpiring()).toBe(true);
    const future = Math.floor(Date.now() / 1000) + 3600;
    setToken(makeToken(future), true);
    expect(isTokenExpiring()).toBe(false);
  });
});

describe('tryRefreshToken', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    vi.restoreAllMocks();
  });

  it('returns false when no token present', async () => {
    expect(await tryRefreshToken()).toBe(false);
  });

  it('returns true and stores the new token on success', async () => {
    setToken(makeToken(Math.floor(Date.now() / 1000) - 100), true); // expiring
    vi.stubGlobal('fetch', vi.fn(async () => mockResponse({ token: 'newtok' })));
    expect(await tryRefreshToken()).toBe(true);
    expect(localStorage.getItem(TOKEN_KEY)).toBe('newtok');
  });

  it('returns false on non-ok response', async () => {
    setToken(makeToken(Math.floor(Date.now() / 1000) + 3600), true);
    vi.stubGlobal('fetch', vi.fn(async () => mockResponse({ error: 'x' }, 500)));
    expect(await tryRefreshToken()).toBe(false);
  });
});

describe('client.request', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    vi.restoreAllMocks();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it('GET returns parsed JSON', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => mockResponse({ ok: 1 })));
    const r = await client.get<{ ok: number }>('/x');
    expect(r.ok).toBe(1);
  });

  it('POST sends JSON body with Authorization header', async () => {
    // 用远期有效 JWT 避免触发顶层 auto-refresh
    const future = Math.floor(Date.now() / 1000) + 3600;
    setToken(makeToken(future), true);
    const fetchMock = vi.fn<typeof fetch>(async () => mockResponse({ id: 42 }) as Response);
    vi.stubGlobal('fetch', fetchMock);
    await client.post('/create', { name: 'n' });
    const firstCall = (fetchMock.mock.calls[0] ?? []) as [unknown, RequestInit];
    const [url, init] = firstCall;
    expect(String(url)).toContain('/api/v1/create');
    expect(init.method).toBe('POST');
    expect(init.body).toBe(JSON.stringify({ name: 'n' }));
    const headers = new Headers(init.headers);
    expect(headers.get('Authorization')).not.toBe('Bearer tok'); // 实际是 Bearer + 完整JWT
    expect(headers.get('Authorization')).toMatch(/^Bearer [A-Za-z0-9._=+-]+\.[A-Za-z0-9._=+-]+\.[A-Za-z0-9._=+-]+$/);
    expect(headers.get('Content-Type')).toBe('application/json');
  });

  it('throws readable error parsed from response body on non-ok', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => mockResponse({ error: '磁盘空间不足' }, 500)));
    await expect(client.get('/x')).rejects.toThrow('磁盘空间不足');
  });

  it('throws HTTP status when body has no error field', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => mockResponse({}, 400)));
    await expect(client.get('/x')).rejects.toThrow('HTTP 400');
  });

  it('returns undefined on 204', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 204 })));
    const r = await client.del('/x');
    expect(r).toBeUndefined();
  });

  it("does NOT refresh on 401 for login endpoint, surfaces server error", async () => {
    vi.stubGlobal('fetch', vi.fn(async () => mockResponse({ error: '用户名或密码错误' }, 401)));
    await expect(client.post('/auth/login', { u: 'a', p: 'b' })).rejects.toThrow('用户名或密码错误');
  });

  it('refreshes token once then retries on 401 for normal endpoint', async () => {
    // 用未过期 JWT，确保顶层 auto-refresh 不触发，让 401 分支独占 refresh 时序
    const future = Math.floor(Date.now() / 1000) + 3600;
    setToken(makeToken(future), true);
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(mockResponse({ error: 'expired' }, 401)) // original -> 401
      .mockResolvedValueOnce(mockResponse({ token: 'newtok' }))        // refresh
      .mockResolvedValueOnce(mockResponse({ ok: 1 }));                 // retry
    vi.stubGlobal('fetch', fetchMock);
    const r = await client.get<{ ok: number }>('/x');
    expect(r.ok).toBe(1);
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it('throws AuthError when 401 refresh fails', async () => {
    const future = Math.floor(Date.now() / 1000) + 3600;
    setToken(makeToken(future), true);
    vi.stubGlobal('fetch', vi.fn(async () => mockResponse({ error: 'e' }, 401)));
    await expect(client.get('/x')).rejects.toThrow(AuthError);
  });
});

describe('streamRequest', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    vi.restoreAllMocks();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  function wait(ms = 50) {
    return new Promise((res) => setTimeout(res, ms));
  }

  it('dispatches readable ai-stream-error on HTTP non-ok', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => mockResponse({ error: 'MCP 工具调用超时' }, 502)));
    const events: any[] = [];
    window.addEventListener('ai-stream-error', (e: any) => events.push(e.detail));
    streamRequest('/ai/stream', {});
    await wait(80);
    expect(events.length).toBeGreaterThanOrEqual(1);
    expect(events[0]).toBe('MCP 工具调用超时');
  });

  it('dispatches ai-stream-done after reading whole stream', async () => {
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(new TextEncoder().encode('hello chunk'));
        controller.close();
      },
    });
    const res = { ok: true, body: stream } as unknown as Response;
    vi.stubGlobal('fetch', vi.fn(async () => res));
    const done: any[] = [];
    window.addEventListener('ai-stream-done', () => done.push(1));
    streamRequest('/ai/stream', {});
    await wait(100);
    expect(done.length).toBe(1);
  });

  it('auto-retries an empty-stream failure up to MAX_EMPTY_RETRIES', async () => {
    vi.useFakeTimers();
    // First call fails with a non-abort network error (empty -> no bytes yet)
    const fetchMock = vi.fn()
      .mockRejectedValueOnce(new Error('网络连接失败'))
      .mockRejectedValueOnce(new Error('网络连接失败'))
      .mockRejectedValueOnce(new Error('网络连接失败'));
    vi.stubGlobal('fetch', fetchMock);
    streamRequest('/ai/stream', {});
    // advance backoff 2000ms + interval; retries scheduled via setTimeout
    await vi.runAllTimersAsync();
    // 1 initial + up to MAX_EMPTY_RETRIES retries
    expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(2);
  });
});

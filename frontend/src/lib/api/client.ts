import { globalLoading } from '$lib/stores/loading.svelte';

const BASE = '/api/v1';

const TOKEN_KEY = 'moduforge_token';

/**
 * Returns the stored auth token, or empty string if none.
 * Always returns string (never null) to prevent 'Bearer null' issues.
 */
export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || sessionStorage.getItem(TOKEN_KEY) || '';
}

export function setToken(token: string, remember: boolean) {
  // Always store in localStorage for Docker persistence
  localStorage.setItem(TOKEN_KEY, token);
  sessionStorage.removeItem(TOKEN_KEY);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
  sessionStorage.removeItem(TOKEN_KEY);
}

/**
 * Check if a token looks structurally valid AND is not expired.
 * Verifies JWT format and checks exp claim.
 */
export function hasValidToken(): boolean {
  const t = getToken();
  if (!t) return false;
  // JWT should have 3 parts separated by dots
  const parts = t.split('.');
  if (parts.length !== 3 || !parts[0] || !parts[1] || !parts[2]) return false;
  // Check expiry
  try {
    const payload = JSON.parse(atob(parts[1].replace(/-/g, '+').replace(/_/g, '/')));
    if (payload.exp) {
      const expiresAt = payload.exp * 1000;
      // Allow 5 minute grace period for refresh
      return Date.now() < expiresAt + 5 * 60 * 1000;
    }
    return true; // No exp claim = assume valid
  } catch {
    return false;
  }
}

/**
 * Decode JWT payload without verification (for expiry check only).
 */
function decodeJwtPayload(token: string): { exp?: number } | null {
  try {
    const payload = token.split('.')[1];
    const decoded = JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')));
    return decoded;
  } catch {
    return null;
  }
}

/**
 * Check if token is expired or will expire within `withinSeconds`.
 */
export function isTokenExpiring(withinSeconds = 300): boolean {
  const token = getToken();
  if (!token) return true;
  const payload = decodeJwtPayload(token);
  if (!payload || !payload.exp) return true;
  // exp is in seconds, Date.now() is in milliseconds
  const expiresAt = payload.exp * 1000;
  const now = Date.now();
  return expiresAt - now < withinSeconds * 1000;
}

/**
 * Attempt to refresh the JWT token using the /auth/refresh endpoint.
 * Returns true if refresh succeeded (new token stored), false otherwise.
 */
export async function tryRefreshToken(): Promise<boolean> {
  const token = getToken();
  if (!token) return false;

  try {
    const res = await fetch(`${BASE}/auth/refresh`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      body: '{}',
    });

    if (!res.ok) return false;

    const data = await res.json();
    if (data.token) {
      setToken(data.token, true);
      return true;
    }
    return false;
  } catch {
    return false;
  }
}

export async function authFetch(url: string, options: RequestInit = {}): Promise<Response> {
  // Auto-refresh token if expiring soon
  if (isTokenExpiring(300)) {
    await tryRefreshToken();
  }

  const token = getToken();
  const headers = new Headers(options.headers);
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 30000);
  const signal = controller.signal;

  try {
    const res = await fetch(url, { ...options, headers, signal });

    // On 401: try refresh once, then retry
    if (res.status === 401) {
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        const newToken = getToken();
        const retryHeaders = new Headers(options.headers);
        if (newToken) {
          retryHeaders.set('Authorization', `Bearer ${newToken}`);
        }
        const retryRes = await fetch(url, { ...options, headers: retryHeaders, signal });
        return retryRes;
      }
      throw new Error('Unauthorized');
    }

    return res;
  } finally {
    clearTimeout(timeoutId);
  }
}

async function request<T = unknown>(method: string, path: string, body?: unknown): Promise<T> {
	const headers: Record<string, string> = {};

	// Auto-refresh token if expiring soon
	if (isTokenExpiring(300)) {
		await tryRefreshToken();
	}

	const token = getToken();
	if (token) headers['Authorization'] = `Bearer ${token}`;

	const controller = new AbortController();
	const timeoutId = setTimeout(() => controller.abort(), 30000);

	const init: RequestInit = { method, headers, signal: controller.signal };
	if (body !== undefined) {
		headers['Content-Type'] = 'application/json';
		init.body = JSON.stringify(body);
	}

	globalLoading.inc();
	try {
		// 登录/注册等认证端点：不带旧 token，不尝试 refresh
		const isAuthEndpoint = path.startsWith('/auth/login') || path.startsWith('/auth/register');
		if (isAuthEndpoint) {
			delete headers['Authorization'];
		}

		let res = await fetch(`${BASE}${path}`, init);

		// On 401: 登录端点直接返回服务器错误信息，其他端点尝试 refresh
		if (res.status === 401) {
			if (isAuthEndpoint) {
				// 登录/注册失败：返回服务器实际错误信息（如"用户名或密码错误"）
				const err = await res.json().catch(() => ({ error: 'Authentication failed' }));
				throw new Error(err.error || '用户名或密码错误');
			}
			const refreshed = await tryRefreshToken();
			if (refreshed) {
				const newToken = getToken();
				if (newToken) headers['Authorization'] = `Bearer ${newToken}`;
				res = await fetch(`${BASE}${path}`, { ...init, headers });
			}
		}

		if (res.status === 401) {
			throw new AuthError('登录已过期，请重新登录');
		}

		if (!res.ok) {
			const err = await res.json().catch(() => ({ error: res.statusText }));
			throw new Error(err.error || `HTTP ${res.status}`);
		}
		if (res.status === 204 || res.headers.get('content-length') === '0') return undefined as T;
		return res.json() as Promise<T>;
	} finally {
		clearTimeout(timeoutId);
		globalLoading.dec();
	}
}

/**
 * Custom error class for auth failures.
 * Callers can check `instanceof AuthError` to handle 401 specifically.
 */
export class AuthError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AuthError';
  }
}

export const client = {
  get: <T = unknown>(path: string) => request<T>('GET', path),
  post: <T = unknown>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T = unknown>(path: string, body?: unknown) => request<T>('PUT', path, body),
  del: <T = unknown>(path: string) => request<T>('DELETE', path),
};

// ─── Conversation API helpers ───

export interface ConversationSummary {
  id: string;
  title: string;
  mode: string;
  model: string;
  message_count: number;
  created_at: string;
  updated_at: string;
}

export interface ConversationMessage {
  id: number;
  session_id: string;
  user_id: string;
  role: string;
  content: string;
  created_at: string;
}

export interface SessionInfo {
  session_id: string;
  started_at: string;
  last_at: string;
  msg_count: number;
}

export async function fetchConversations(): Promise<ConversationSummary[]> {
  const res = await client.get<{ conversations: ConversationSummary[] }>('/ai/conversations');
  return res.conversations || [];
}

export async function fetchConversationMessages(sessionId: string): Promise<ConversationMessage[]> {
  const res = await client.get<{ messages: ConversationMessage[] }>(`/ai/sessions/${sessionId}/messages`);
  return res.messages || [];
}

export async function deleteConversation(sessionId: string): Promise<void> {
  await client.del(`/ai/sessions/${sessionId}`);
}

export async function fetchSessions(): Promise<SessionInfo[]> {
  const res = await client.get<{ sessions: SessionInfo[] }>('/ai/sessions');
  return res.sessions || [];
}

// ─── Code Diff API ───

export interface DiffEntry {
  type: 'add' | 'remove' | 'context';
  line: number;
  old?: string;
  new?: string;
}

export interface DiffResult {
  diffs: DiffEntry[];
  file_path: string;
  old_lines: number;
  new_lines: number;
}

export async function computeDiff(oldCode: string, newCode: string, filePath: string): Promise<DiffResult> {
  return client.post<DiffResult>('/ai/diff', { old_code: oldCode, new_code: newCode, file_path: filePath });
}

/**
 * SSE-style streaming request with idle timeout.
 * Only times out after `idleMs` of NO data arriving.
 * As long as data flows, the stream keeps going indefinitely.
 */
export function streamRequest(path: string, body: unknown, idleMs = 90000): EventSource {
  const token = getToken();
  const ctrl = new AbortController();
  let idleTimer: ReturnType<typeof setTimeout> | null = null;
  // Retry only when NOTHING has been received yet — once any SSE chunk
  // arrived, the model is mid-generation and a retry would duplicate output.
  // This keeps auto-recovery safe for flaky connections.
  const MAX_EMPTY_RETRIES = 2;
  const RETRY_BACKOFF_MS = 2000;
  let receivedAny = false;

  function resetIdle() {
    if (idleTimer) clearTimeout(idleTimer);
    idleTimer = setTimeout(() => {
      ctrl.abort('idle timeout');
      window.dispatchEvent(new CustomEvent('ai-stream-timeout'));
    }, idleMs);
  }

  async function attempt(attemptNo: number) {
    try {
      const res = await fetch(`${BASE}${path}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify(body),
        signal: ctrl.signal,
      });
      if (!res.ok || !res.body) {
        if (idleTimer) clearTimeout(idleTimer);
        let errMsg = `HTTP ${res.status}`;
        try {
          const errText = await res.text();
          const errJson = JSON.parse(errText);
          errMsg = errJson.error || errJson.message || errMsg;
        } catch (e) { console.warn('parse error response failed:', e); }
        window.dispatchEvent(new CustomEvent('ai-stream-error', { detail: errMsg }));
        ctrl.abort('http error');
        return;
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        // Data arrived — reset idle timer
        receivedAny = true;
        resetIdle();
        const text = decoder.decode(value, { stream: true });
        window.dispatchEvent(new CustomEvent('ai-stream', { detail: text }));
      }
      // Stream finished normally — clear idle timer
      if (idleTimer) clearTimeout(idleTimer);
      window.dispatchEvent(new CustomEvent('ai-stream-done'));
    } catch (e: unknown) {
      if (idleTimer) clearTimeout(idleTimer);
      // AbortError means we intentionally closed (idle timeout or user stop)
      const err = e instanceof Error ? e : null;
      if (err && err.name === 'AbortError') return;
      const msg = err?.message || (typeof e === 'string' ? e : '网络连接失败');
      // Auto-retry ONLY on an empty (no bytes received) failure.
      if (!receivedAny && attemptNo < MAX_EMPTY_RETRIES) {
        window.dispatchEvent(new CustomEvent('ai-stream-retry', { detail: { attempt: attemptNo + 1 } }));
        setTimeout(() => attempt(attemptNo + 1), RETRY_BACKOFF_MS * attemptNo);
        return;
      }
      window.dispatchEvent(new CustomEvent('ai-stream-error', { detail: msg }));
    }
  }

  // Start idle timer immediately
  resetIdle();
  attempt(0);

  return { close: () => { if (idleTimer) clearTimeout(idleTimer); ctrl.abort('user cancelled'); } } as unknown as EventSource;
}

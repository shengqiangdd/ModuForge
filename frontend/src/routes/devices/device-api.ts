function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
  const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
  return { 'Authorization': `Bearer ${token}`, ...extra };
}

// Parse a response, preserving the backend error message when the HTTP status
// is not 2xx. Without this, callers would see a success-like object even for
// failed requests (e.g. adb connect returning 500/403).
async function parseResponse(res: Response): Promise<any> {
  let data: any = {};
  try { data = await res.json(); } catch { /* empty body */ }
  if (!res.ok) {
    return {
      error: data.error || data.message || `请求失败 (${res.status})`,
      ...data,
    };
  }
  return data;
}

export async function apiGet(url: string) {
  const res = await fetch(url, { headers: authHeaders() });
  return parseResponse(res);
}

export async function apiPost(url: string, body: Record<string, unknown>) {
  const res = await fetch(url, {
    method: 'POST',
    headers: authHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(body),
  });
  return parseResponse(res);
}

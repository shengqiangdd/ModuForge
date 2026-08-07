export async function apiGet(url: string) {
  const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
  const res = await fetch(url, { headers: { 'Authorization': `Bearer ${token}` } });
  return res.json();
}

export async function apiPost(url: string, body: Record<string, unknown>) {
  const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify(body),
  });
  return res.json();
}
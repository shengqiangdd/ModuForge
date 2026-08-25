import {
  getToken,
  type MarketModule, type Review, type ModuleVersion,
  type HealthScore, type ModuleTag, type ChangelogEntry,
  type InstallStat, type TemplateItem, type TemplateCategory,
} from './types';

// ═══════════════════════════════════════════
//  Module list
// ═══════════════════════════════════════════

export async function fetchModules(params: {
  page: number; perPage: number; sort: string;
  query?: string; category?: string; tag?: number | null;
}): Promise<{ modules: MarketModule[]; total: number }> {
  const { page, perPage, sort, query, category, tag } = params;
  const sp = new URLSearchParams({ page: String(page), per_page: String(perPage), sort });
  if (query) sp.set('query', query);
  if (category) sp.set('category', category);
  if (tag) sp.set('tag', String(tag));
  const token = getToken();
  const headers: Record<string, string> = {};
  if (token) headers['Authorization'] = `Bearer ${token}`;
  try {
    const res = await fetch(`/api/v1/market/modules?${sp}`, { headers });
    if (res.ok) { const d = await res.json(); return { modules: d.modules || [], total: d.total || 0 }; }
  } catch (e) { console.error('Failed to fetch modules:', e); }
  return { modules: [], total: 0 };
}

// ═══════════════════════════════════════════
//  Favorites
// ═══════════════════════════════════════════

export async function fetchFavoriteIds(): Promise<Set<string>> {
  const token = getToken();
  if (!token) return new Set();
  try {
    const r = await fetch('/api/v1/favorites?type=module', { headers: { Authorization: `Bearer ${token}` } });
    if (r.status === 401) { localStorage.removeItem('moduforge_token'); return new Set(); }
    if (r.ok) {
      const d = await r.json();
      const ids = new Set<string>();
      for (const f of (d.favorites || [])) ids.add(`mod_${String(f.item_id).padStart(4, '0')}`);
      return ids;
    }
  } catch (e) { console.error('Failed to fetch favorite IDs:', e); }
  return new Set();
}

export async function toggleFavoriteApi(modId: string, isFav: boolean): Promise<boolean | null> {
  const token = getToken();
  if (!token) return null;
  try {
    let r;
    if (isFav) {
      r = await fetch(`/api/v1/favorites/module/${modId}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
    } else {
      r = await fetch('/api/v1/favorites', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ type: 'module', id: parseInt(modId.replace('mod_', '')) || 0 }),
      });
    }
    if (r.status === 401) { localStorage.removeItem('moduforge_token'); return null; }
    return !isFav;
  } catch (e) { console.error('Failed to toggle favorite:', e); }
  return null;
}

// ═══════════════════════════════════════════
//  Tags
// ═══════════════════════════════════════════

export async function fetchAllTags(): Promise<{ id: number; name: string; color: string; usage_count: number }[]> {
  const token = getToken();
  const headers: Record<string, string> = {};
  if (token) headers['Authorization'] = `Bearer ${token}`;
  try {
    const r = await fetch('/api/v1/tags', { headers });
    if (r.status === 401) return [];
    if (r.ok) { const d = await r.json(); return d.tags || []; }
  } catch (e) { console.error('Failed to fetch tags:', e); }
  return [];
}

// ═══════════════════════════════════════════
//  Module detail
// ═══════════════════════════════════════════

export async function fetchReviews(slug: string): Promise<Review[]> {
  try {
    const res = await fetch(`/api/v1/market/module/${slug}/reviews`);
    if (res.ok) { const d = await res.json(); return d.reviews || []; }
  } catch (e) { console.error('Failed to fetch reviews:', e); }
  return [];
}

export async function fetchHealthScore(slug: string): Promise<HealthScore | null> {
  try {
    const r = await fetch(`/api/v1/market/module/${slug}/health`);
    if (r.ok) return await r.json();
  } catch (e) { console.error('Failed to fetch health score:', e); }
  return null;
}

export async function fetchModuleTags(slug: string): Promise<ModuleTag[]> {
  try {
    const r = await fetch(`/api/v1/market/module/${slug}/tags`);
    if (r.ok) { const d = await r.json(); return d.tags || []; }
  } catch (e) { console.error('Failed to fetch module tags:', e); }
  return [];
}

export async function starModuleApi(slug: string): Promise<number | null> {
  const token = getToken();
  try {
    const res = await fetch(`/api/v1/market/module/${slug}/star`, { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } });
    if (res.status === 401) { localStorage.removeItem('moduforge_token'); return null; }
    if (res.ok) { const d = await res.json(); return d.stars; }
  } catch (e) { console.error('Failed to star module:', e); }
  return null;
}

export async function submitReviewApi(slug: string, rating: number, comment: string): Promise<boolean> {
  const token = getToken();
  try {
    const res = await fetch(`/api/v1/market/module/${slug}/review`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ uid: 'anonymous', username: 'Anonymous', rating, comment }),
    });
    if (res.status === 401) { localStorage.removeItem('moduforge_token'); return false; }
    return res.ok;
  } catch (e) { console.error('Failed to submit review:', e); }
  return false;
}

// ═══════════════════════════════════════════
//  Versions
// ═══════════════════════════════════════════

export async function fetchVersions(slug: string): Promise<ModuleVersion[]> {
  const token = getToken();
  try {
    const res = await fetch(`/api/v1/market/module/${slug}/versions`, { headers: { 'Authorization': `Bearer ${token}` } });
    if (res.status === 401) { localStorage.removeItem('moduforge_token'); return []; }
    if (res.ok) { const d = await res.json(); return d.versions || []; }
  } catch (e) { console.error('Failed to fetch versions:', e); }
  return [];
}

export async function rollbackApi(slug: string, versionId: string): Promise<boolean> {
  const token = getToken();
  try {
    const res = await fetch(`/api/v1/market/module/${slug}/rollback`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ version_id: versionId }),
    });
    if (res.status === 401) { localStorage.removeItem('moduforge_token'); return false; }
    return res.ok;
  } catch (e) { console.error('Failed to rollback:', e); }
  return false;
}

export async function updateVersionApi(slug: string, versionCode: string, changelog: string): Promise<boolean> {
  const token = getToken();
  try {
    const res = await fetch(`/api/v1/market/module/${slug}/version`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ version_code: versionCode.trim(), changelog: changelog.trim() }),
    });
    if (res.status === 401) { localStorage.removeItem('moduforge_token'); return false; }
    return res.ok;
  } catch (e) { console.error('Failed to update version:', e); }
  return false;
}

// ═══════════════════════════════════════════
//  Changelogs
// ═══════════════════════════════════════════

export async function fetchChangelogs(slug: string): Promise<ChangelogEntry[]> {
  try {
    const r = await fetch(`/api/v1/market/module/${slug}/changelogs`);
    if (r.ok) { const d = await r.json(); return d.changelogs || []; }
  } catch (e) { console.error('Failed to fetch changelogs:', e); }
  return [];
}

// ═══════════════════════════════════════════
//  Stats
// ═══════════════════════════════════════════

export async function fetchInstallStats(slug: string, period: string): Promise<InstallStat[]> {
  try {
    const r = await fetch(`/api/v1/market/module/${slug}/install-stats?period=${period}&days=30`);
    if (r.ok) { const d = await r.json(); return d.stats || []; }
  } catch (e) { console.error('Failed to fetch install stats:', e); }
  return [];
}

export async function fetchTrending(): Promise<MarketModule[]> {
  try {
    const r = await fetch('/api/v1/market/stats/trending');
    if (r.ok) { const d = await r.json(); return d.modules?.slice(0, 10) || []; }
  } catch (e) { console.error('Failed to fetch trending:', e); }
  return [];
}

// ═══════════════════════════════════════════
//  Batch
// ═══════════════════════════════════════════

export async function runBatchApi(action: string, slugs: string[]): Promise<{ slug: string; status: string; error?: string }[] | null> {
  const token = getToken();
  if (!token) return null;
  try {
    const r = await fetch(`/api/v1/market/batch/${action}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ slugs }),
    });
    if (r.status === 401) { localStorage.removeItem('moduforge_token'); return null; }
    if (r.ok) {
      const d = await r.json();
      return (d.results || []).map((res: any) => ({ ...res, status: res.error ? 'failed' : 'ok' }));
    }
  } catch (e) { console.error('Failed to run batch action:', e); }
  return [];
}

// ═══════════════════════════════════════════
//  Install to device
// ═══════════════════════════════════════════

export async function fetchDevices(): Promise<{ serial: string; model: string; state: string }[]> {
  try {
    const r = await fetch('/api/v1/adb/devices');
    if (r.ok) { const d = await r.json(); return (d.devices || []).filter((dev: any) => dev.state === 'device'); }
  } catch (e) { console.error('Failed to fetch devices:', e); }
  return [];
}

export async function downloadModule(slug: string): Promise<{ blob: Blob; filename: string } | null> {
  const token = getToken();
  try {
    const res = await fetch(`/api/v1/market/module/${slug}/download`, { headers: token ? { Authorization: `Bearer ${token}` } : {} });
    if (!res.ok) throw new Error(`下载失败 (${res.status})`);
    const blob = await res.blob();
    return { blob, filename: `${slug}.zip` };
  } catch (e) { console.error('Failed to download module:', e); }
  return null;
}

export async function pushToDevice(serial: string, blob: Blob, filename: string): Promise<string> {
  const formData = new FormData();
  formData.append('serial', serial);
  formData.append('file', blob, filename);
  const pushRes = await fetch('/api/v1/adb/module/upload', { method: 'POST', body: formData });
  if (!pushRes.ok) { const errData = await pushRes.json().catch(() => ({})); throw new Error(errData.error || `推送失败 (${pushRes.status})`); }
  const pushData = await pushRes.json();
  return pushData.output || '推送完成';
}

// ═══════════════════════════════════════════
//  Templates
// ═══════════════════════════════════════════

export async function fetchTemplates(params: {
  page: number; sort: string; query?: string; category?: string;
}): Promise<{ templates: TemplateItem[]; total: number }> {
  const sp = new URLSearchParams({ page: String(params.page), per_page: '20', sort: params.sort });
  if (params.query) sp.set('query', params.query);
  if (params.category) sp.set('category', params.category);
  try {
    const r = await fetch(`/api/v1/templates/market?${sp}`);
    if (r.ok) { const d = await r.json(); return { templates: d.templates || [], total: d.total || 0 }; }
  } catch (e) { console.error('Failed to fetch templates:', e); }
  return { templates: [], total: 0 };
}

export async function fetchTemplateCategories(): Promise<TemplateCategory[]> {
  try {
    const r = await fetch('/api/v1/templates/market/categories');
    if (r.ok) { const d = await r.json(); return d.categories || []; }
  } catch (e) { console.error('Failed to fetch template categories:', e); }
  return [];
}

export async function useTemplateApi(id: number): Promise<string | null> {
  try {
    const r = await fetch(`/api/v1/templates/market/${id}/use`, { method: 'POST' });
    if (r.ok) { const d = await r.json(); return d.module_data; }
  } catch (e) { console.error('Failed to use template:', e); }
  return null;
}

export async function rateTemplateApi(id: number, rating: number): Promise<number | null> {
  const token = getToken();
  if (!token) return null;
  try {
    const r = await fetch(`/api/v1/templates/market/${id}/rate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ rating }),
    });
    if (r.ok) { const d = await r.json(); return d.rating; }
  } catch (e) { console.error('Failed to rate template:', e); }
  return null;
}

export async function publishTemplateApi(name: string, desc: string, category: string): Promise<boolean> {
  const token = getToken();
  if (!token) return false;
  try {
    const r = await fetch('/api/v1/templates/market', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ name, description: desc, category, module_data: JSON.stringify({ name, description: desc }) }),
    });
    return r.ok;
  } catch (e) { console.error('Failed to publish template:', e); }
  return false;
}

// ═══════════════════════════════════════════
//  Demo
// ═══════════════════════════════════════════

export async function fetchDemo(slug: string): Promise<any> {
  try {
    const res = await fetch(`/api/v1/market/module/${slug}/demo`);
    if (res.ok) return await res.json();
    return { error: '无法加载演示数据' };
  } catch { return { error: '网络错误' }; }
}

// ═══════════════════════════════════════════
//  Compare
// ═══════════════════════════════════════════

export async function runCompareApi(slug1: string, slug2: string): Promise<any> {
  try {
    const res = await fetch('/api/v1/market/compare', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ slug1, slug2 }),
    });
    if (res.ok) return await res.json();
    return { error: '对比失败' };
  } catch { return { error: '网络错误' }; }
}

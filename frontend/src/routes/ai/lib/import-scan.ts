import { client, authFetch } from '$lib/api/client';
import { toast } from '$lib/stores/toast.svelte';
import type { SecurityScanResult } from './types';

// ─── Import to project ───
export async function loadImportProjects(): Promise<{ id: string; name: string }[]> {
  try {
    return await client.get<{ id: string; name: string }[]>('/projects');
  } catch (e: unknown) {
    toast(e instanceof Error ? e.message : '加载项目列表失败', 'error');
    return [];
  }
}

export async function scanFiles(files: { path: string; content: string }[]): Promise<SecurityScanResult | null> {
  try {
    const res = await authFetch('/api/v1/security/scan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ files: Object.fromEntries(files.map(f => [f.path, f.content])) }),
    });
    return await res.json();
  } catch {
    return null;
  }
}

export async function importFilesToProject(
  projectId: string,
  files: { path: string; content: string }[],
): Promise<{ success: number; fail: number }> {
  let success = 0;
  let fail = 0;
  for (const f of files) {
    try {
      await client.put(`/projects/${projectId}/files/${encodeURIComponent(f.path)}`, { content: f.content });
      success++;
    } catch {
      fail++;
    }
  }
  return { success, fail };
}

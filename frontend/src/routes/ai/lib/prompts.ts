import { toast } from '$lib/stores/toast.svelte';
import type { AIPrompt, Mode } from './types';

// ─── Prompt management ───
export async function loadPrompts(): Promise<AIPrompt[]> {
  try {
    const token = localStorage.getItem('moduforge_token') || '';
    const res = await fetch('/api/v1/ai/prompts', { headers: { 'Authorization': `Bearer ${token}` } });
    if (res.ok) {
      const data = await res.json();
      return data.prompts || [];
    }
  } catch (e) {
    console.error('loadPrompts error:', e);
    toast('加载提示词失败，请刷新页面重试', 'error');
  }
  return [];
}

export async function savePromptToBackend(mode: Mode, content: string): Promise<boolean> {
  try {
    const token = localStorage.getItem('moduforge_token') || '';
    const res = await fetch('/api/v1/ai/prompts', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ mode, content }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || `HTTP ${res.status}`);
    }
    toast('提示词保存成功', 'success');
    return true;
  } catch (e) {
    toast(`保存提示词失败: ${e instanceof Error ? e.message : e}`, 'error');
    return false;
  }
}

export async function resetPromptToDefault(mode: Mode): Promise<string> {
  try {
    const token = localStorage.getItem('moduforge_token') || '';
    const res = await fetch(`/api/v1/ai/prompts/${mode}/reset`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}` },
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || `HTTP ${res.status}`);
    }
    const updated = await loadPrompts();
    const p = updated.find(x => x.mode === mode);
    toast('已恢复默认提示词', 'success');
    return p?.content || '';
  } catch (e) {
    toast(`恢复默认提示词失败: ${e instanceof Error ? e.message : e}`, 'error');
    return '';
  }
}

export async function loadPromptForMode(mode: Mode): Promise<string> {
  const prompts = await loadPrompts();
  const p = prompts.find(x => x.mode === mode);
  return p?.content || '';
}

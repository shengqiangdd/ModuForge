<script lang="ts">
import { onDestroy } from 'svelte';

let {
  show = false,
  onClose,
  onAddReference,
}: {
  show: boolean;
  onClose: () => void;
  onAddReference: (text: string) => void;
} = $props();

let abortController = $state<AbortController | null>(null);

onDestroy(() => abortController?.abort());

let url = $state('');
let loading = $state(false);
let loadingFiles = $state(false);
let adding = $state(false);
let error = $state('');
let info = $state<{ owner?: string; name?: string; stars?: number; topics?: string[]; license?: string } | null>(null);
let allFiles = $state<any[]>([]);
let selected = $state<string[]>([]);
let statusMsg = $state('');
let addedCount = $state(0);

function authHeader(): Record<string, string> {
  const token = localStorage.getItem('moduforge_token') || '';
  return { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` };
}

async function fetchRepo() {
  if (!url.trim()) { error = '请填写 GitHub 仓库 URL'; return; }
  loading = true; error = ''; info = null; allFiles = []; selected = [];
  try {
    abortController?.abort();
    abortController = new AbortController();
    const res = await fetch('/api/v1/repo/fetch', {
      method: 'POST', headers: authHeader(), body: JSON.stringify({ url: url.trim() }),
      signal: abortController.signal,
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || '获取仓库失败');
    info = data;
  } catch (e: any) {
    if (e?.name === 'AbortError') return;
    error = e.message || String(e);
  } finally { loading = false; }
}

async function loadAndSmartSelect() {
  if (!info?.owner || !info?.name || !url.trim()) { error = '请先获取仓库信息'; return; }
  loadingFiles = true; error = '';
  try {
    abortController?.abort();
    abortController = new AbortController();
    // 一次性拉取完整文件树（后端用 git trees API 递归，1 次调用，规避 rate-limit）
    const res = await fetch('/api/v1/repo/tree', {
      method: 'POST', headers: authHeader(), body: JSON.stringify({ url: url.trim() }),
      signal: abortController.signal,
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || '拉取文件树失败');
    const flat: any[] = Array.isArray(data) ? data : [];
    allFiles = flat;

    // 智能选择关键文件
    const sr = await fetch('/api/v1/repo/smart-select', {
      method: 'POST', headers: authHeader(), body: JSON.stringify({ files: flat }),
      signal: abortController.signal,
    });
    const sd = await sr.json();
    if (sr.ok && Array.isArray(sd.selected)) {
      selected = sd.selected;
    }
  } catch (e: any) {
    if (e?.name === 'AbortError') return;
    error = e.message || String(e);
  } finally { loadingFiles = false; }
}

function toggleFile(path: string) {
  selected = selected.includes(path) ? selected.filter(p => p !== path) : [...selected, path];
}

async function addReference() {
  if (selected.length === 0) { error = '请先选择要加入参考的文件'; return; }
  adding = true; error = ''; addedCount = 0;
  try {
    const parts: string[] = [];
    parts.push(`[参考仓库] ${info?.owner}/${info?.name}${info?.stars ? ' (★' + info.stars + ')' : ''}`);
    if (info?.topics?.length) parts.push(`Topics: ${info.topics.join(', ')}`);
    if (info?.license) parts.push(`License: ${info.license}`);
    parts.push('');
    parts.push('以下是从该仓库智能挑选的关键文件内容，请参考其结构/实现进行改造：');
    parts.push('');

    for (const p of selected) {
      try {
        const res = await fetch('/api/v1/repo/file', {
          method: 'POST', headers: authHeader(), body: JSON.stringify({ url: url.trim(), path: p }),
          signal: abortController?.signal,
        });
        const data = await res.json();
        if (!res.ok) continue;
        parts.push(`===== 📄 ${p} =====`);
        parts.push(String(data.content || '(空)'));
        parts.push('');
        addedCount++;
      } catch { /* 跳过单个失败文件 */ }
    }
    onAddReference(parts.join('\n'));
    statusMsg = `已将 ${addedCount} 个关键文件加入生成参考`;
    selected = [];
  } catch (e: any) {
    error = e.message || String(e);
  } finally { adding = false; }
}
</script>

{#if show}
  <div class="border-t border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-3">
    <div class="flex items-center gap-2 mb-2">
      <span class="text-xs font-semibold text-[var(--color-text)]">参考仓库</span>
      {#if statusMsg}
        <span class="text-[11px] text-green-500 truncate">{statusMsg}</span>
      {/if}
      <button class="ml-auto p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
        <span class="material-symbols-outlined text-[14px]" style="color: var(--color-text-muted)">close</span>
      </button>
    </div>

    <div class="flex gap-2 mb-2">
      <input class="flex-1 input-field text-xs" placeholder="GitHub 仓库 URL，如 https://github.com/owner/repo"
        value={url} oninput={(e) => url = (e.target as HTMLInputElement).value}
        onkeydown={(e) => { if (e.key === 'Enter') fetchRepo(); }} />
      <button class="text-xs px-2 py-1 rounded-lg bg-primary-600 text-white hover:bg-primary-700 disabled:opacity-50" onclick={fetchRepo} disabled={loading}>
        {loading ? '获取中…' : '获取'}
      </button>
    </div>

    {#if error}
      <div class="text-[11px] text-red-500 mb-2">{error}</div>
    {/if}

    {#if info}
      <div class="text-[11px] text-[var(--color-text-muted)] mb-2 leading-relaxed">
        <span class="font-semibold text-[var(--color-text)]">{info.owner}/{info.name}</span>
        {#if info.stars != null}<span class="ml-1">★{info.stars}</span>{/if}
        {#if info.license}<span class="ml-1">· {info.license}</span>{/if}
        {#if info.topics?.length}<div class="mt-0.5">{info.topics.slice(0,8).join(' · ')}</div>{/if}
      </div>
      <button class="text-xs px-2 py-1 rounded-lg bg-[var(--color-surface)] text-[var(--color-text)] hover:bg-[var(--color-border)] disabled:opacity-50 mb-2" onclick={loadAndSmartSelect} disabled={loadingFiles}>
        {loadingFiles ? '解析中…' : '智能选择关键文件'}
      </button>
    {/if}

    {#if allFiles.length > 0}
      <div class="max-h-40 overflow-y-auto mb-2 rounded-lg bg-[var(--color-surface)] p-1.5">
        {#each selected as p}
          <label class="flex items-center gap-1.5 py-0.5 text-[11px] cursor-pointer hover:bg-[var(--color-border)]/40 rounded px-1">
            <input type="checkbox" checked onchange={(e) => { if (!(e.target as HTMLInputElement).checked) toggleFile(p); }} />
            <span class="truncate font-mono">{p}</span>
          </label>
        {/each}
        {#if selected.length === 0}
          <div class="text-[11px] text-[var(--color-text-muted)] p-1">未选中文件（共 {allFiles.length} 个）</div>
        {/if}
      </div>
      {#if selected.length > 0}
        <button class="text-xs px-2 py-1 rounded-lg bg-primary-600 text-white hover:bg-primary-700 disabled:opacity-50" onclick={addReference} disabled={adding}>
          {adding ? '加入中…' : `加入 ${selected.length} 个文件到参考`}
        </button>
      {/if}
    {/if}
  </div>
{/if}


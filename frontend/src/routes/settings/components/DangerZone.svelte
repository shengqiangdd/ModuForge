<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let { onClear }: { onClear?: () => void } = $props();

  let clearRecycleLoading = $state(false);
  let clearMemoryLoading = $state(false);
  let clearHistoryLoading = $state(false);

  async function clearRecycleBin() {
    clearRecycleLoading = true;
    try {
      const r = await fetch('/api/v1/recycle-bin', { method: 'DELETE', headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) {
        toast('回收站已清空', 'info');
        onClear?.();
      } else {
        toast('操作失败', 'error');
      }
    } catch { toast('操作失败', 'error'); }
    clearRecycleLoading = false;
  }

  async function clearAllMemory() {
    clearMemoryLoading = true;
    try {
      const res = await fetch('/api/v1/ai/memory', {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${getToken()}` },
      });
      if (res.ok) {
        toast('所有记忆已清除', 'info');
        onClear?.();
      }
    } catch {}
    clearMemoryLoading = false;
  }

  async function clearSearchHistory() {
    clearHistoryLoading = true;
    try {
      const r = await fetch('/api/v1/search/history', { method: 'DELETE', headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) {
        toast('搜索历史已清空', 'info');
        onClear?.();
      }
    } catch {}
    clearHistoryLoading = false;
  }
</script>

<section class="card p-6" style="border: 1px solid var(--color-error);">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-error-light)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-error)">warning</span>
    </div>
    <div>
      <h2 class="text-base font-semibold text-[var(--color-error)]">危险操作</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">清除数据不可恢复，请谨慎操作</p>
    </div>
  </div>
  <div class="space-y-3">
    <div class="flex items-center justify-between p-3 rounded-xl" style="background: var(--color-surface)">
      <div>
        <p class="text-sm font-medium text-[var(--color-text)]">清空回收站</p>
        <p class="text-xs" style="color: var(--color-text-muted)">永久删除所有已删除的项目和模块</p>
      </div>
      <button class="px-3 py-1.5 text-xs font-medium rounded-lg" style="background: var(--color-error-light); color: var(--color-error)" onclick={clearRecycleBin} disabled={clearRecycleLoading}>
        {clearRecycleLoading ? '清除中...' : '清空'}
      </button>
    </div>
    <div class="flex items-center justify-between p-3 rounded-xl" style="background: var(--color-surface)">
      <div>
        <p class="text-sm font-medium text-[var(--color-text)]">清除 AI 记忆</p>
        <p class="text-xs" style="color: var(--color-text-muted)">删除 Agent 保存的所有用户偏好和上下文</p>
      </div>
      <button class="px-3 py-1.5 text-xs font-medium rounded-lg" style="background: var(--color-error-light); color: var(--color-error)" onclick={clearAllMemory} disabled={clearMemoryLoading}>
        {clearMemoryLoading ? '清除中...' : '清除'}
      </button>
    </div>
    <div class="flex items-center justify-between p-3 rounded-xl" style="background: var(--color-surface)">
      <div>
        <p class="text-sm font-medium text-[var(--color-text)]">清空搜索历史</p>
        <p class="text-xs" style="color: var(--color-text-muted)">删除所有搜索记录</p>
      </div>
      <button class="px-3 py-1.5 text-xs font-medium rounded-lg" style="background: var(--color-error-light); color: var(--color-error)" onclick={clearSearchHistory} disabled={clearHistoryLoading}>
        {clearHistoryLoading ? '清除中...' : '清空'}
      </button>
    </div>
  </div>
</section>

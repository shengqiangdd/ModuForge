<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let searchHistory = $state<{ id: number; query: string; result_count: number; searched_at: string }[]>([]);

  async function loadSearchHistory() {
    const token = getToken();
    try {
      const r = await fetch('/api/v1/search/history', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) { const d = await r.json(); searchHistory = d.history || []; }
    } catch { searchHistory = []; }
  }

  async function deleteSearchHistoryItem(id: number) {
    const token = getToken();
    try {
      await fetch(`/api/v1/search/history/${id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      searchHistory = searchHistory.filter(h => h.id !== id);
    } catch (e) { console.error('Failed to delete search history item:', e); }
  }

  async function clearSearchHistory() {
    const token = getToken();
    try {
      await fetch('/api/v1/search/history', { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      searchHistory = [];
    } catch (e) { console.error('Failed to clear search history:', e); }
  }

  onMount(() => {
    loadSearchHistory();
  });
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">history</span>
    </div>
    <div class="flex-1">
      <h2 class="text-base font-semibold text-[var(--color-text)]">搜索历史</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">最近 20 条搜索记录（项目 / 文件搜索）</p>
    </div>
    {#if searchHistory.length > 0}
      <button type="button"
        class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors"
        style="color: var(--color-error); background: var(--color-error-light)"
        onclick={clearSearchHistory}>清空</button>
    {/if}
  </div>
  {#if searchHistory.length === 0}
    <p class="text-sm text-center py-4" style="color: var(--color-text-muted)">暂无搜索记录</p>
  {:else}
    <div class="space-y-1.5">
      {#each searchHistory as h (h.id)}
        <div class="flex items-center justify-between p-2.5 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
          <div class="flex items-center gap-2 min-w-0">
            <span class="material-symbols-outlined text-[16px]" style="color: var(--color-text-muted)">search</span>
            <div class="min-w-0">
              <span class="text-sm font-medium text-[var(--color-text)]">{h.query}</span>
              <span class="text-xs ml-1.5" style="color: var(--color-text-muted)">{h.result_count} 条结果</span>
            </div>
          </div>
          <button type="button"
            class="text-[11px] px-2.5 py-1 rounded-lg transition-colors"
            style="color: var(--color-text-muted); background: var(--color-surface-secondary)"
            onclick={() => deleteSearchHistoryItem(h.id)}>删除</button>
        </div>
      {/each}
    </div>
  {/if}
</section>

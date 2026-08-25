<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  interface CacheInfo {
    entries: number;
    ttl: string;
    size?: string;
    keys?: number;
  }
  let cacheData: CacheInfo | null = $state(null);
  let cacheLoading = $state(false);
  let cacheClearing = $state(false);

  async function loadCacheStatus() {
    cacheLoading = true;
    try {
      const r = await fetch('/api/v1/admin/cache/status', {
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) cacheData = await r.json();
    } catch (e) { console.error('Failed to load cache status:', e); }
    cacheLoading = false;
  }

  async function clearCache() {
    cacheClearing = true;
    try {
      const r = await fetch('/api/v1/admin/cache/clear', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) {
        const d = await r.json();
        toast(`缓存已清除，共 ${d.entries} 条`, 'success');
        cacheData = { entries: 0, ttl: cacheData?.ttl || '5m0s' };
      } else {
        toast('清除缓存失败', 'error');
      }
    } catch { toast('清除缓存失败', 'error'); }
    cacheClearing = false;
  }

  onMount(() => {
    loadCacheStatus();
  });
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">cached</span>
    </div>
    <div class="flex-1">
      <h2 class="text-base font-semibold text-[var(--color-text)]">API 缓存</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">只读 API 响应缓存，减少重复请求</p>
    </div>
    <button class="btn-ghost text-sm" onclick={loadCacheStatus} disabled={cacheLoading}>
      <span class="material-symbols-outlined text-[16px] {cacheLoading ? 'animate-spin' : ''}">refresh</span>
      刷新
    </button>
  </div>
  {#if cacheLoading}
    <div class="skeleton h-20 rounded-xl"></div>
  {:else if cacheData}
    <div class="flex items-center gap-4 mb-4">
      <div class="flex-1 p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
        <p class="text-xs" style="color: var(--color-text-muted)">缓存条目</p>
        <p class="text-lg font-bold text-[var(--color-text)]">{cacheData.entries || 0}</p>
      </div>
      <div class="flex-1 p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
        <p class="text-xs" style="color: var(--color-text-muted)">TTL</p>
        <p class="text-lg font-bold text-[var(--color-text)]">{cacheData.ttl || '5m'}</p>
      </div>
    </div>
    <p class="text-xs mb-3" style="color: var(--color-text-muted)">缓存的 API：模板列表、市场模块、热门模块、项目列表、LLM 供应商</p>
    <button class="btn-ghost text-sm border" style="border-color: var(--color-border); color: var(--color-error)" onclick={clearCache} disabled={cacheClearing}>
      <span class="material-symbols-outlined text-[16px] {cacheClearing ? 'animate-spin' : ''}">delete_sweep</span>
      {cacheClearing ? '清除中...' : '清除缓存'}
    </button>
  {:else}
    <button class="btn-primary text-sm" onclick={loadCacheStatus}>加载缓存状态</button>
  {/if}
</section>

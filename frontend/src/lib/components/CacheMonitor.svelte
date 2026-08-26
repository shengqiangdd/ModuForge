<script>
  import { onMount } from 'svelte';

  let stats = $state(null);
  let loading = $state(false);
  let clearing = $state(false);

  async function fetchStats() {
    loading = true;
    try {
      const res = await fetch('/api/cache/stats');
      if (res.ok) stats = await res.json();
    } catch (e) {
      console.error('Failed to fetch cache stats:', e);
    }
    loading = false;
  }

  async function clearCache() {
    clearing = true;
    try {
      const res = await fetch('/api/cache/clear', { method: 'POST' });
      if (res.ok) {
        await fetchStats();
      }
    } catch (e) {
      console.error('Failed to clear cache:', e);
    }
    clearing = false;
  }

  onMount(fetchStats);
</script>

<div class="cache-monitor">
  <div class="flex items-center justify-between mb-4">
    <h2 class="text-xl font-semibold">🗄️ 缓存监控</h2>
    <div class="flex gap-2">
      <button
        onclick={fetchStats}
        disabled={loading}
        class="px-3 py-1 text-sm bg-gray-100 rounded hover:bg-gray-200 disabled:opacity-50"
      >
        刷新
      </button>
      <button
        onclick={clearCache}
        disabled={clearing}
        class="px-3 py-1 text-sm bg-red-100 text-red-600 rounded hover:bg-red-200 disabled:opacity-50"
      >
        {clearing ? '清除中...' : '清除缓存'}
      </button>
    </div>
  </div>

  {#if stats}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
      <div class="card">
        <div class="card-label">命中率</div>
        <div class="card-value text-green-500">
          {stats.Hits + stats.Misses > 0
            ? ((stats.Hits / (stats.Hits + stats.Misses)) * 100).toFixed(1)
            : 0}%
        </div>
      </div>
      <div class="card">
        <div class="card-label">命中次数</div>
        <div class="card-value">{stats.Hits}</div>
      </div>
      <div class="card">
        <div class="card-label">未命中</div>
        <div class="card-value text-red-500">{stats.Misses}</div>
      </div>
      <div class="card">
        <div class="card-label">淘汰次数</div>
        <div class="card-value text-yellow-500">{stats.Evictions}</div>
      </div>
    </div>

    <div class="card">
      <div class="flex items-center justify-between">
        <span class="text-sm text-gray-500">缓存项数量</span>
        <span class="font-mono">{stats.ItemCount}</span>
      </div>
    </div>
  {:else if loading}
    <div class="text-center py-8 text-gray-500">加载中...</div>
  {/if}
</div>

<style>
  .cache-monitor { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-label { font-size: 0.75rem; color: #6b7280; text-transform: uppercase; }
  .card-value { font-size: 1.5rem; font-weight: 600; margin-top: 0.25rem; }
</style>

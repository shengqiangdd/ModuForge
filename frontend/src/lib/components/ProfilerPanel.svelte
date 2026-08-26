<script>
  import { t } from '$lib/i18n';

  let metrics = $state({});
  let memory = $state(null);
  let loading = $state(false);

  async function fetchMetrics() {
    loading = true;
    try {
      const [metricsRes, memoryRes] = await Promise.all([
        fetch('/api/profiler/metrics'),
        fetch('/api/profiler/memory')
      ]);

      if (metricsRes.ok) metrics = await metricsRes.json();
      if (memoryRes.ok) memory = await memoryRes.json();
    } catch (e) {
      console.error('Failed to fetch profiler data:', e);
    }
    loading = false;
  }

  async function resetMetrics() {
    try {
      await fetch('/api/profiler/reset', { method: 'POST' });
      await fetchMetrics();
    } catch (e) {
      console.error('Failed to reset metrics:', e);
    }
  }

  function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  import { onMount } from 'svelte';
  onMount(fetchMetrics);
</script>

<div class="profiler-panel">
  <div class="flex items-center justify-between mb-4">
    <h2 class="text-xl font-semibold">{$t('profiler.title')}</h2>
    <div class="flex gap-2">
      <button onclick={fetchMetrics} disabled={loading} class="px-3 py-1 text-sm bg-gray-100 rounded hover:bg-gray-200">
        {$t('profiler.refresh')}
      </button>
      <button onclick={resetMetrics} class="px-3 py-1 text-sm bg-red-100 text-red-600 rounded hover:bg-red-200">
        {$t('profiler.reset')}
      </button>
    </div>
  </div>

  {#if memory}
    <div class="card mb-4">
      <h3 class="card-title">{$t('profiler.memory')}</h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <div class="text-gray-500">{$t('profiler.memory.alloc')}</div>
          <div class="font-semibold">{formatBytes(memory.alloc)}</div>
        </div>
        <div>
          <div class="text-gray-500">{$t('profiler.memory.total_alloc')}</div>
          <div class="font-semibold">{formatBytes(memory.total_alloc)}</div>
        </div>
        <div>
          <div class="text-gray-500">{$t('profiler.memory.sys')}</div>
          <div class="font-semibold">{formatBytes(memory.sys)}</div>
        </div>
        <div>
          <div class="text-gray-500">{$t('profiler.memory.num_gc')}</div>
          <div class="font-semibold">{memory.num_gc}</div>
        </div>
      </div>
    </div>
  {/if}

  {#if Object.keys(metrics).length > 0}
    <div class="card">
      <h3 class="card-title">{$t('profiler.metrics')}</h3>
      <div class="space-y-2 max-h-64 overflow-y-auto">
        {#each Object.entries(metrics) as [name, metric]}
          <div class="flex items-center justify-between p-2 bg-gray-50 rounded">
            <span class="font-mono text-sm">{name}</span>
            <div class="text-right text-xs text-gray-500">
              <div>{$t('profiler.metrics.count')}: {metric.count}</div>
              <div>{$t('profiler.metrics.avg')}: {metric.avg.toFixed(2)}</div>
              <div>{$t('profiler.metrics.min')}: {metric.min}</div>
              <div>{$t('profiler.metrics.max')}: {metric.max}</div>
            </div>
          </div>
        {/each}
      </div>
    </div>
  {:else if !loading}
    <div class="card text-center py-8 text-gray-500">
      {$t('profiler.no_data')}
    </div>
  {/if}
</div>

<style>
  .profiler-panel { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

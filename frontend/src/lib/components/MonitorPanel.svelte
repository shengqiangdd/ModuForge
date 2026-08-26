<script>
  let alerts = $state([]);
  let logStats = $state(null);
  let loading = $state(false);
  let activeTab = $state('alerts');

  async function fetchAlerts() {
    try {
      const res = await fetch('/api/monitoring/alerts');
      if (res.ok) alerts = await res.json();
    } catch (e) {
      console.error('Failed to fetch alerts:', e);
    }
  }

  async function fetchLogStats() {
    try {
      const res = await fetch('/api/monitoring/logs/stats');
      if (res.ok) logStats = await res.json();
    } catch (e) {
      console.error('Failed to fetch log stats:', e);
    }
  }

  async function refresh() {
    loading = true;
    await Promise.all([fetchAlerts(), fetchLogStats()]);
    loading = false;
  }

  async function resolveAlert(alertId) {
    try {
      const res = await fetch(`/api/monitoring/alerts/${alertId}/resolve`, { method: 'POST' });
      if (res.ok) await fetchAlerts();
    } catch (e) {
      console.error('Failed to resolve alert:', e);
    }
  }

  function getSeverityColor(severity) {
    const colors = {
      critical: 'bg-red-100 text-red-700 border-red-200',
      warning: 'bg-yellow-100 text-yellow-700 border-yellow-200',
      info: 'bg-blue-100 text-blue-700 border-blue-200'
    };
    return colors[severity] || 'bg-gray-100';
  }

  import { onMount } from 'svelte';
  onMount(refresh);
</script>

<div class="monitor-panel">
  <div class="flex items-center justify-between mb-4">
    <h2 class="text-xl font-semibold">📡 监控告警</h2>
    <button onclick={refresh} disabled={loading} class="px-3 py-1 text-sm bg-gray-100 rounded hover:bg-gray-200">
      {loading ? '刷新中...' : '刷新'}
    </button>
  </div>

  <!-- Tabs -->
  <div class="flex gap-2 mb-4">
    <button
      onclick={() => activeTab = 'alerts'}
      class="px-4 py-2 rounded {activeTab === 'alerts' ? 'bg-blue-500 text-white' : 'bg-gray-100'}"
    >
      告警 ({alerts.length})
    </button>
    <button
      onclick={() => activeTab = 'logs'}
      class="px-4 py-2 rounded {activeTab === 'logs' ? 'bg-blue-500 text-white' : 'bg-gray-100'}"
    >
      日志统计
    </button>
  </div>

  {#if activeTab === 'alerts'}
    <!-- Alerts -->
    {#if alerts.length === 0}
      <div class="card text-center py-8 text-gray-500">
        ✅ 没有活跃告警
      </div>
    {:else}
      <div class="space-y-3">
        {#each alerts as alert}
          <div class="card {getSeverityColor(alert.severity)}">
            <div class="flex items-center justify-between">
              <div>
                <div class="font-medium">{alert.message}</div>
                <div class="text-sm opacity-75">
                  {alert.severity.toUpperCase()} • {new Date(alert.created_at).toLocaleString('zh-CN')}
                </div>
              </div>
              <button
                onclick={() => resolveAlert(alert.id)}
                class="px-3 py-1 text-sm bg-white bg-opacity-50 rounded hover:bg-opacity-75"
              >
                解决
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  {:else}
    <!-- Log Stats -->
    {#if logStats}
      <div class="card mb-4">
        <h3 class="card-title">日志概览</h3>
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          <div>
            <div class="text-gray-500">总日志数</div>
            <div class="text-lg font-semibold">{logStats.total}</div>
          </div>
          {#each Object.entries(logStats.by_level) as [level, count]}
            <div>
              <div class="text-gray-500">{level}</div>
              <div class="text-lg font-semibold">{count}</div>
            </div>
          {/each}
        </div>
      </div>

      <div class="card">
        <h3 class="card-title">最近日志</h3>
        <div class="space-y-2 max-h-64 overflow-y-auto font-mono text-xs">
          {#each logStats.recent as log}
            <div class="p-2 bg-gray-50 rounded">
              <span class="{log.level === 'error' ? 'text-red-500' : log.level === 'warn' ? 'text-yellow-500' : 'text-gray-500'}">
                [{log.level.toUpperCase()}]
              </span>
              <span class="text-gray-400">{log.source}</span>
              <span>{log.message}</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .monitor-panel { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

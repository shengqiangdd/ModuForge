<script>
  import { onMount, onDestroy } from 'svelte';
  
  let summary = $state(null);
  let history = $state([]);
  let modelStats = $state(null);
  let loading = $state(true);
  let error = $state(null);
  let refreshInterval = $state(null);
  
  onMount(async () => {
    await fetchData();
    refreshInterval = setInterval(fetchData, 30000);
  });
  
  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });
  
  async function fetchData() {
    try {
      const [summaryRes, historyRes, modelsRes] = await Promise.all([
        fetch('/api/perf/summary'),
        fetch('/api/perf/history'),
        fetch('/api/perf/models')
      ]);
      
      if (summaryRes.ok) summary = await summaryRes.json();
      if (historyRes.ok) history = await historyRes.json();
      if (modelsRes.ok) modelStats = await modelsRes.json();
      
      loading = false;
    } catch (e) {
      error = e.message;
      loading = false;
    }
  }
  
  function formatUptime(seconds) {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    return `${days}d ${hours}h ${mins}m`;
  }
  
  function formatTokens(tokens) {
    if (tokens >= 1000000) return (tokens / 1000000).toFixed(2) + 'M';
    if (tokens >= 1000) return (tokens / 1000).toFixed(1) + 'K';
    return tokens.toString();
  }
</script>

<div class="perf-dashboard">
  <h2 class="text-xl font-semibold mb-4">📊 性能监控仪表盘</h2>
  
  {#if loading}
    <div class="text-center py-8 text-gray-500">加载中...</div>
  {:else if error}
    <div class="text-center py-8 text-red-500">错误: {error}</div>
  {:else}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
      <div class="card">
        <div class="card-label">运行时间</div>
        <div class="card-value">{summary ? formatUptime(summary.uptime_seconds) : 'N/A'}</div>
      </div>
      <div class="card">
        <div class="card-label">总请求数</div>
        <div class="card-value">{summary ? summary.total_requests : 0}</div>
      </div>
      <div class="card">
        <div class="card-label">错误率</div>
        <div class="card-value text-red-500">{summary ? summary.error_rate.toFixed(2) : 0}%</div>
      </div>
      <div class="card">
        <div class="card-label">Token 使用量</div>
        <div class="card-value">{summary ? formatTokens(summary.total_tokens) : '0'}</div>
      </div>
    </div>
    
    {#if summary?.build_stats}
      <div class="card mb-6">
        <h3 class="card-title">🔨 构建统计</h3>
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div>
            <div class="text-sm text-gray-500">总构建数</div>
            <div class="text-lg font-semibold">{summary.build_stats.total_builds}</div>
          </div>
          <div>
            <div class="text-sm text-gray-500">成功构建</div>
            <div class="text-lg font-semibold text-green-500">{summary.build_stats.success_builds}</div>
          </div>
          <div>
            <div class="text-sm text-gray-500">失败构建</div>
            <div class="text-lg font-semibold text-red-500">{summary.build_stats.failed_builds}</div>
          </div>
          <div>
            <div class="text-sm text-gray-500">成功率</div>
            <div class="text-lg font-semibold">{summary.build_stats.success_rate.toFixed(1)}%</div>
          </div>
        </div>
      </div>
    {/if}
    
    {#if modelStats}
      <div class="card mb-6">
        <h3 class="card-title">🤖 模型统计</h3>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b">
                <th class="text-left py-2">模型</th>
                <th class="text-right py-2">成功</th>
                <th class="text-right py-2">失败</th>
                <th class="text-right py-2">成功率</th>
                <th class="text-right py-2">平均延迟</th>
              </tr>
            </thead>
            <tbody>
              {#each Object.entries(modelStats) as [model, stats]}
                <tr class="border-b hover:bg-gray-50">
                  <td class="py-2 font-mono text-xs">{model}</td>
                  <td class="text-right py-2 text-green-500">{stats.success}</td>
                  <td class="text-right py-2 text-red-500">{stats.failure}</td>
                  <td class="text-right py-2">{stats.success_rate.toFixed(1)}%</td>
                  <td class="text-right py-2">{stats.avg_latency}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/if}
    
    {#if summary?.tool_calls}
      <div class="card">
        <h3 class="card-title">🔧 工具调用统计</h3>
        <div class="flex flex-wrap gap-2">
          {#each Object.entries(summary.tool_calls) as [tool, count]}
            <span class="badge">{tool}: {count}</span>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .perf-dashboard { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-label { font-size: 0.75rem; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em; }
  .card-value { font-size: 1.5rem; font-weight: 600; margin-top: 0.25rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
  .badge { background: #f3f4f6; padding: 0.25rem 0.5rem; border-radius: 0.25rem; font-size: 0.75rem; }
</style>

<script>
  let report = $state(null);
  let loading = $state(false);
  let error = $state(null);
  let projectDir = $state('.');
  
  async function analyze() {
    loading = true;
    error = null;
    try {
      const res = await fetch(`/api/arch/analyze?dir=${encodeURIComponent(projectDir)}`);
      if (res.ok) {
        report = await res.json();
      } else {
        error = await res.text();
      }
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }
  
  function getArchColor(arch) {
    const colors = { 'monolith': '#3b82f6', 'microservice': '#10b981', 'library': '#8b5cf6', 'small_project': '#f59e0b', 'unknown': '#6b7280' };
    return colors[arch] || '#6b7280';
  }
</script>

<div class="arch-report">
  <h2 class="text-xl font-semibold mb-4">🏗️ 架构分析</h2>
  
  <div class="flex gap-2 mb-4">
    <input type="text" bind:value={projectDir} placeholder="项目目录路径" class="flex-1 px-3 py-2 border rounded" />
    <button onclick={analyze} disabled={loading} class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50">
      {loading ? '分析中...' : '分析'}
    </button>
  </div>
  
  {#if error}
    <div class="text-red-500 mb-4">{error}</div>
  {/if}
  
  {#if report}
    <div class="card mb-4">
      <div class="flex items-center gap-3 mb-3">
        <span class="px-3 py-1 rounded-full text-white text-sm font-medium" style="background-color: {getArchColor(report.architecture)}">
          {report.architecture}
        </span>
        <span class="text-gray-500">{report.language}</span>
      </div>
      <div class="grid grid-cols-3 gap-4 text-center">
        <div><div class="text-2xl font-bold">{report.total_files}</div><div class="text-sm text-gray-500">文件数</div></div>
        <div><div class="text-2xl font-bold">{report.total_lines.toLocaleString()}</div><div class="text-sm text-gray-500">代码行数</div></div>
        <div><div class="text-2xl font-bold">{report.dependencies.length}</div><div class="text-sm text-gray-500">依赖数</div></div>
      </div>
    </div>
    
    <div class="card mb-4">
      <h3 class="card-title">📏 质量指标</h3>
      <div class="space-y-3">
        <div>
          <div class="flex justify-between text-sm mb-1"><span>耦合度</span><span>{(report.coupling * 100).toFixed(0)}%</span></div>
          <div class="w-full bg-gray-200 rounded-full h-2">
            <div class="h-2 rounded-full" style="width: {report.coupling * 100}%; background-color: {report.coupling > 0.7 ? '#ef4444' : report.coupling > 0.4 ? '#f59e0b' : '#10b981'}"></div>
          </div>
        </div>
        <div>
          <div class="flex justify-between text-sm mb-1"><span>内聚度</span><span>{(report.cohesion * 100).toFixed(0)}%</span></div>
          <div class="w-full bg-gray-200 rounded-full h-2">
            <div class="h-2 rounded-full" style="width: {report.cohesion * 100}%; background-color: {report.cohesion > 0.7 ? '#10b981' : report.cohesion > 0.4 ? '#f59e0b' : '#ef4444'}"></div>
          </div>
        </div>
      </div>
    </div>
    
    {#if report.suggestions?.length > 0}
      <div class="card mb-4">
        <h3 class="card-title">💡 改进建议</h3>
        <ul class="space-y-2">
          {#each report.suggestions as suggestion}
            <li class="flex items-start gap-2"><span class="text-yellow-500">•</span><span class="text-sm">{suggestion}</span></li>
          {/each}
        </ul>
      </div>
    {/if}
    
    {#if report.issues?.length > 0}
      <div class="card">
        <h3 class="card-title">⚠️ 发现的问题 ({report.issues.length})</h3>
        <div class="space-y-2 max-h-64 overflow-y-auto">
          {#each report.issues as issue}
            <div class="flex items-start gap-2 p-2 bg-gray-50 rounded">
              <span class="text-xs px-2 py-0.5 rounded {issue.severity === 'high' ? 'bg-red-100 text-red-700' : issue.severity === 'medium' ? 'bg-yellow-100 text-yellow-700' : 'bg-gray-100 text-gray-700'}">{issue.severity}</span>
              <div class="flex-1">
                <div class="text-sm">{issue.message}</div>
                {#if issue.file}<div class="text-xs text-gray-500 font-mono">{issue.file}</div>{/if}
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .arch-report { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

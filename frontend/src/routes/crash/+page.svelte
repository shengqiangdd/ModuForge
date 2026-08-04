<script lang="ts">
  import { onMount } from 'svelte';

  let logs: any[] = $state([]);
  let stats: any = $state(null);
  let loading = $state(true);
  let filterModule = $state('');
  let filterType = $state('');
  let filterDevice = $state('');
  let selectedLog: any = $state(null);

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }

  async function load() {
    loading = true;
    const token = getToken();
    const headers = { Authorization: `Bearer ${token}` };
    try {
      const [logsR, statsR] = await Promise.all([
        fetch(`/api/v1/crash/logs?module=${filterModule}&type=${filterType}&device=${filterDevice}`, { headers }),
        fetch('/api/v1/crash/stats', { headers }),
      ]);
      if (logsR.ok) { const d = await logsR.json(); logs = d.logs || []; }
      if (statsR.ok) stats = await statsR.json();
    } catch {}
    loading = false;
  }

  async function deleteLog(id: number) {
    await fetch(`/api/v1/crash/logs/${id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${getToken()}` } });
    logs = logs.filter(l => l.id !== id);
  }

  async function clearAll() {
    if (!confirm('确认清空所有崩溃日志？')) return;
    await fetch('/api/v1/crash/logs', { method: 'DELETE', headers: { Authorization: `Bearer ${getToken()}` } });
    logs = []; stats = null;
  }

  onMount(load);

  const severityColor = (type: string) => {
    const t = type.toLowerCase();
    if (t.includes('crash') || t.includes('fatal') || t.includes('panic')) return '#ef4444';
    if (t.includes('error') || t.includes('exception')) return '#f97316';
    if (t.includes('warn')) return '#eab308';
    return '#3b82f6';
  };
</script>

<div class="p-6 max-w-5xl mx-auto space-y-6">
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold text-[var(--color-text)]">崩溃分析</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">设备崩溃日志监控与统计</p>
    </div>
    <button class="px-3 py-1.5 rounded-lg text-sm" style="background: var(--color-danger-light); color: var(--color-danger)" onclick={clearAll}>清空</button>
  </div>

  <!-- Stats Cards -->
  {#if stats}
    <div class="grid grid-cols-3 gap-4">
      <div class="card p-4 text-center">
        <p class="text-2xl font-bold text-[var(--color-text)]">{stats.total || 0}</p>
        <p class="text-xs mt-1" style="color: var(--color-text-muted)">总计</p>
      </div>
      <div class="card p-4 text-center">
        <p class="text-2xl font-bold text-orange-500">{stats.today || 0}</p>
        <p class="text-xs mt-1" style="color: var(--color-text-muted)">今日</p>
      </div>
      <div class="card p-4 text-center">
        <p class="text-2xl font-bold text-blue-500">{stats.affected_modules || 0}</p>
        <p class="text-xs mt-1" style="color: var(--color-text-muted)">影响模块数</p>
      </div>
    </div>
  {/if}

  <!-- Filters -->
  <div class="flex gap-3 flex-wrap">
    <input class="input-field flex-1 min-w-[150px]" placeholder="按模块过滤..." bind:value={filterModule} oninput={load} />
    <input class="input-field flex-1 min-w-[150px]" placeholder="按类型过滤..." bind:value={filterType} oninput={load} />
    <input class="input-field flex-1 min-w-[150px]" placeholder="按设备过滤..." bind:value={filterDevice} oninput={load} />
  </div>

  <!-- Logs -->
  {#if loading}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">加载中...</div>
  {:else if logs.length === 0}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">暂无崩溃日志</div>
  {:else}
    <div class="space-y-3">
      {#each logs as log}
        <div role="button" tabindex="0" class="card p-4 cursor-pointer hover:shadow-md transition-shadow" onclick={() => selectedLog = log} onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); selectedLog = log; } }}>
          <div class="flex items-center gap-3">
            <div class="w-2 h-2 rounded-full flex-shrink-0" style="background: {severityColor(log.error_type)}"></div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-[var(--color-text)] truncate">{log.error_type}</span>
                {#if log.module_slug}
                  <span class="text-xs px-1.5 py-0.5 rounded" style="background: var(--color-primary-light); color: var(--color-primary)">{log.module_slug}</span>
                {/if}
              </div>
              <p class="text-xs text-[var(--color-text-muted)] mt-0.5">
                {log.device_id} · {new Date(log.created_at).toLocaleString()}
              </p>
            </div>
            <button class="text-xs px-2 py-1 rounded flex-shrink-0" style="background: var(--color-danger-light); color: var(--color-danger)" onclick={(e) => { e.stopPropagation(); deleteLog(log.id); }}>删除</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Detail Modal -->
{#if selectedLog}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) selectedLog = null; }}>
    <div class="rounded-2xl max-w-2xl w-full max-h-[80vh] overflow-auto border" style="background: var(--color-bg-elevated); border-color: var(--color-border)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">{selectedLog.error_type}</h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)]" onclick={() => selectedLog = null}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-4">
        <div class="grid grid-cols-2 gap-4 text-sm">
          <div><span class="text-[var(--color-text-muted)]">设备:</span> <span class="text-[var(--color-text)]">{selectedLog.device_id}</span></div>
          <div><span class="text-[var(--color-text-muted)]">模块:</span> <span class="text-[var(--color-text)]">{selectedLog.module_slug || '-'}</span></div>
          <div><span class="text-[var(--color-text-muted)]">版本:</span> <span class="text-[var(--color-text)]">{selectedLog.app_version || '-'}</span></div>
          <div><span class="text-[var(--color-text-muted)]">时间:</span> <span class="text-[var(--color-text)]">{new Date(selectedLog.created_at).toLocaleString()}</span></div>
        </div>
        <div>
          <h4 class="text-sm font-semibold text-[var(--color-text)] mb-2">堆栈信息</h4>
          <pre class="text-xs p-3 rounded-xl overflow-auto max-h-48" style="background: var(--color-surface); color: var(--color-text-secondary); font-family: monospace; white-space: pre-wrap">{selectedLog.stack_trace}</pre>
        </div>
        {#if selectedLog.device_info && selectedLog.device_info !== '{}'}
          <div>
            <h4 class="text-sm font-semibold text-[var(--color-text)] mb-2">设备信息</h4>
            <pre class="text-xs p-3 rounded-xl overflow-auto max-h-32" style="background: var(--color-surface); color: var(--color-text-secondary); font-family: monospace">{JSON.stringify(JSON.parse(selectedLog.device_info || '{}'), null, 2)}</pre>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let { isAdmin }: { isAdmin: boolean } = $props();

  interface CacheInfo {
    entries: number;
    ttl: string;
    size?: string;
    keys?: number;
  }

  interface LogEntry {
    id: number;
    level: string;
    module: string;
    message: string;
    timestamp: string;
    created_at?: string;
    details?: string;
  }

  interface LogStats {
    total: number;
    levels: Record<string, number>;
    total_logs?: number;
  }

  interface HealthInfo {
    uptime: string;
    version: string;
    goroutines: number;
    memory: string;
    status?: string;
    checks?: Record<string, unknown>;
  }

  // ===== Cache =====
  let cacheData: CacheInfo | null = $state(null);
  let cacheLoading = $state(false);
  let cacheClearing = $state(false);

  // ===== Logs =====
  let logs: LogEntry[] = $state([]);
  let logsLoading = $state(false);
  let logsTotal = $state(0);
  let logsPage = $state(1);
  let logsLevel = $state('');
  let logsModule = $state('');
  let logsStats: LogStats | null = $state(null);
  let logsStatsLoading = $state(false);
  let logsCleanupLoading = $state(false);
  let cleanupDays = $state(30);

  // ===== System Health =====
  let healthData: HealthInfo | null = $state(null);
  let healthLoading = $state(false);

  // ===== Functions =====

  async function loadHealth() {
    healthLoading = true;
    try {
      const r = await fetch('/api/v1/health/system', {
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) healthData = await r.json();
    } catch (e) { console.error('Failed to load health data:', e); }
    healthLoading = false;
  }

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

  async function loadLogs() {
    logsLoading = true;
    try {
      const params = new URLSearchParams({ page: String(logsPage), limit: '50' });
      if (logsLevel) params.set('level', logsLevel);
      if (logsModule) params.set('module', logsModule);
      const r = await fetch(`/api/v1/admin/logs?${params}`, {
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) {
        const d = await r.json();
        logs = d.logs || [];
        logsTotal = d.total || 0;
      }
    } catch (e) { console.error('Failed to load logs:', e); }
    logsLoading = false;
  }

  async function loadLogsStats() {
    logsStatsLoading = true;
    try {
      const r = await fetch('/api/v1/admin/logs/stats', {
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) logsStats = await r.json();
    } catch (e) { console.error('Failed to load log stats:', e); }
    logsStatsLoading = false;
  }

  async function cleanupLogs() {
    logsCleanupLoading = true;
    try {
      const r = await fetch(`/api/v1/admin/logs/cleanup?days=${cleanupDays}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) {
        const d = await r.json();
        toast(d.message || '日志已清理', 'success');
        loadLogs();
        loadLogsStats();
      } else {
        toast('清理失败', 'error');
      }
    } catch { toast('清理失败', 'error'); }
    logsCleanupLoading = false;
  }

  function getLevelColor(level: string): string {
    switch (level) {
      case 'error': return 'var(--color-error)';
      case 'warn': case 'warning': return 'var(--color-warning)';
      case 'info': return 'var(--color-success)';
      case 'debug': return 'var(--color-text-muted)';
      default: return 'var(--color-text-secondary)';
    }
  }

  function getLevelBadgeBg(level: string): string {
    switch (level) {
      case 'error': return 'var(--color-error-light)';
      case 'warn': case 'warning': return 'var(--color-warning-light)';
      case 'info': return 'var(--color-success-light)';
      default: return 'var(--color-surface)';
    }
  }

  onMount(() => {
    if (isAdmin) {
      loadCacheStatus();
      loadLogs();
      loadHealth();
    }
  });
</script>

{#if isAdmin}
<!-- System Health -->
<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-success-light)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-success)">monitor_heart</span>
    </div>
    <div class="flex-1">
      <h2 class="text-base font-semibold text-[var(--color-text)]">系统健康</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">各服务运行状态和资源使用</p>
    </div>
    <button class="btn-ghost text-sm" onclick={loadHealth} disabled={healthLoading}>
      <span class="material-symbols-outlined text-[16px] {healthLoading ? 'animate-spin' : ''}">refresh</span>
      刷新
    </button>
  </div>
  {#if healthLoading}
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      {#each Array(4) as _}
        <div class="skeleton h-20 rounded-xl"></div>
      {/each}
    </div>
  {:else if healthData}
    <div class="flex items-center gap-2 mb-4">
      <span class="w-2.5 h-2.5 rounded-full" style="background: {healthData.status === 'healthy' ? 'var(--color-success)' : 'var(--color-error)'}"></span>
      <span class="text-sm font-medium" style="color: {healthData.status === 'healthy' ? 'var(--color-success)' : 'var(--color-error)'}">{healthData.status === 'healthy' ? '健康' : '异常'}</span>
      <span class="text-xs ml-auto" style="color: var(--color-text-muted)">运行时间: {healthData.uptime} · v{healthData.version}</span>
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      {#each Object.entries(healthData.checks || {}) as [key, check]}
        <div class="p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
          <div class="flex items-center justify-between mb-1">
            <span class="text-xs font-medium" style="color: var(--color-text-secondary)">{key}</span>
            <span class="w-2 h-2 rounded-full" style="background: {(check as any).status === 'ok' || (check as any).status === 'healthy' ? 'var(--color-success)' : 'var(--color-error)'}"></span>
          </div>
          {#if (check as any).response_ms != null}
            <p class="text-lg font-bold text-[var(--color-text)]">{(check as any).response_ms}ms</p>
          {:else if (check as any).free_gb != null}
            <p class="text-lg font-bold text-[var(--color-text)]">{(check as any).free_gb}GB / {(check as any).total_gb}GB</p>
          {:else if (check as any).used_mb != null}
            <p class="text-lg font-bold text-[var(--color-text)]">{(check as any).used_mb}MB / {(check as any).total_mb}MB</p>
          {:else}
            <p class="text-lg font-bold text-[var(--color-text)]">{(check as any).status}</p>
          {/if}
        </div>
      {/each}
    </div>
  {:else}
    <button class="btn-primary text-sm" onclick={loadHealth}>加载健康检查</button>
  {/if}
</section>

<!-- Application Logs -->
<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">receipt_long</span>
    </div>
    <div class="flex-1">
      <h2 class="text-base font-semibold text-[var(--color-text)]">应用日志</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">查看和管理系统日志</p>
    </div>
    <button class="btn-ghost text-sm" onclick={loadLogs} disabled={logsLoading}>
      <span class="material-symbols-outlined text-[16px] {logsLoading ? 'animate-spin' : ''}">refresh</span>
      刷新
    </button>
  </div>

  {#if logsStats}
    <div class="flex gap-2 mb-4 flex-wrap">
      {#each Object.entries(logsStats.levels || {}) as [level, count]}
        <span class="text-xs px-2 py-1 rounded-lg" style="background: {getLevelBadgeBg(level)}; color: {getLevelColor(level)}">
          {level}: {count}
        </span>
      {/each}
      <span class="text-xs px-2 py-1 rounded-lg" style="background: var(--color-surface); color: var(--color-text-muted)">
        共 {logsStats.total_logs || 0} 条
      </span>
    </div>
  {/if}

  <div class="flex gap-2 mb-4 flex-wrap">
    <select class="text-sm px-3 py-1.5 rounded-lg border" style="border-color: var(--color-border); background: var(--color-bg); color: var(--color-text)" bind:value={logsLevel} onchange={() => { logsPage = 1; loadLogs(); }}>
      <option value="">所有级别</option>
      <option value="debug">Debug</option>
      <option value="info">Info</option>
      <option value="warn">Warning</option>
      <option value="error">Error</option>
    </select>
    <input type="text" class="text-sm px-3 py-1.5 rounded-lg border flex-1 min-w-[120px]" style="border-color: var(--color-border); background: var(--color-bg); color: var(--color-text)" placeholder="模块筛选..." bind:value={logsModule} oninput={() => { logsPage = 1; loadLogs(); }} />
    <button class="btn-ghost text-xs px-3 py-1.5" onclick={loadLogsStats} disabled={logsStatsLoading}>
      <span class="material-symbols-outlined text-[14px] {logsStatsLoading ? 'animate-spin' : ''}">bar_chart</span>
      统计
    </button>
  </div>

  {#if logsLoading && logs.length === 0}
    <div class="space-y-2">
      {#each Array(5) as _}
        <div class="skeleton h-16 rounded-xl"></div>
      {/each}
    </div>
  {:else if logs.length === 0}
    <p class="text-sm text-center py-6" style="color: var(--color-text-muted)">暂无日志</p>
  {:else}
    <div class="space-y-1.5 max-h-96 overflow-y-auto">
      {#each logs as log}
        <div class="p-3 rounded-lg border text-sm" style="border-color: var(--color-border); background: var(--color-surface)">
          <div class="flex items-center gap-2 mb-1">
            <span class="text-xs px-1.5 py-0.5 rounded font-medium" style="background: {getLevelBadgeBg(log.level)}; color: {getLevelColor(log.level)}">{log.level}</span>
            {#if log.module}
              <span class="text-xs" style="color: var(--color-text-secondary)">{log.module}</span>
            {/if}
            <span class="text-xs ml-auto" style="color: var(--color-text-muted)">{log.created_at ? new Date(log.created_at).toLocaleString() : ''}</span>
          </div>
          <p class="text-sm" style="color: var(--color-text)">{log.message}</p>
          {#if log.details}
            <pre class="mt-1 text-xs p-2 rounded" style="background: var(--color-bg); color: var(--color-text-secondary); white-space: pre-wrap; max-height: 80px; overflow: auto">{log.details}</pre>
          {/if}
        </div>
      {/each}
    </div>

    {#if logsTotal > 50}
      <div class="flex items-center justify-between mt-4 pt-3 border-t" style="border-color: var(--color-border)">
        <span class="text-xs" style="color: var(--color-text-muted)">共 {logsTotal} 条，第 {logsPage} 页</span>
        <div class="flex gap-2">
          <button class="btn-ghost text-xs px-3 py-1" disabled={logsPage <= 1} onclick={() => { logsPage--; loadLogs(); }}>上一页</button>
          <button class="btn-ghost text-xs px-3 py-1" disabled={logsPage * 50 >= logsTotal} onclick={() => { logsPage++; loadLogs(); }}>下一页</button>
        </div>
      </div>
    {/if}
  {/if}

  <div class="flex items-center gap-3 mt-4 pt-3 border-t" style="border-color: var(--color-border)">
    <span class="text-xs" style="color: var(--color-text-secondary)">清理旧日志：</span>
    <select class="text-xs px-2 py-1 rounded border" style="border-color: var(--color-border); background: var(--color-bg); color: var(--color-text)" bind:value={cleanupDays}>
      <option value={7}>7 天前</option>
      <option value={14}>14 天前</option>
      <option value={30}>30 天前</option>
      <option value={60}>60 天前</option>
      <option value={90}>90 天前</option>
    </select>
    <button class="btn-ghost text-xs px-3 py-1 text-[var(--color-error)]" onclick={cleanupLogs} disabled={logsCleanupLoading}>
      <span class="material-symbols-outlined text-[14px] {logsCleanupLoading ? 'animate-spin' : ''}">delete_sweep</span>
      {logsCleanupLoading ? '清理中...' : '清理'}
    </button>
  </div>
</section>
{/if}

<!-- API Cache -->
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

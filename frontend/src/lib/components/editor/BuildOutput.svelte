<script lang="ts">
  let {
    status = '',
    logLines = [],
    building = false,
    buildCached = false,
    triggerMode = 'manual',
    selectedTarget = 'arm64',
    taskId = null,
    incrementalInfo = null,
    cacheStatus = null,
  }: {
    status?: string;
    logLines?: string[];
    building?: boolean;
    buildCached?: boolean;
    triggerMode?: string;
    selectedTarget?: string;
    taskId?: string | null;
    incrementalInfo?: { needs_rebuild: boolean; changed_files: string[]; new_files: string[]; removed_files: string[]; reason: string } | null;
    cacheStatus?: { total_size: number; file_count: number; hit_rate: number; total_builds: number; cache_hits: number } | null;
  } = $props();

  const statusConfig: Record<string, { color: string; bg: string; icon: string }> = {
    pending: { color: 'text-[var(--color-warning)]', bg: 'bg-[var(--color-warning-light)]', icon: 'schedule' },
    running: { color: 'text-[var(--color-info)]', bg: 'bg-[var(--color-info-light)]', icon: 'sync' },
    success: { color: 'text-[var(--color-success)]', bg: 'bg-[var(--color-success-light)]', icon: 'check_circle' },
    failed: { color: 'text-[var(--color-error)]', bg: 'bg-[var(--color-error-light)]', icon: 'error' },
    cancelled: { color: 'text-[var(--color-text-muted)]', bg: 'bg-[var(--color-surface)]', icon: 'cancel' },
  };

  const triggerIcons: Record<string, string> = { manual: 'build', git: 'cloud_upload', webhook: 'webhook', push: 'cloud_upload', schedule: 'schedule' };

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }
</script>

<!-- Cache Status -->
{#if cacheStatus && cacheStatus.file_count > 0}
  <div class="mb-4 p-3 rounded-2xl border flex items-center gap-4" style="border-color: var(--color-border); background: var(--color-bg-elevated)">
    <div class="flex items-center gap-2">
      <span class="material-symbols-outlined text-[18px] text-[var(--color-text-muted)]">database</span>
      <span class="text-xs font-medium text-[var(--color-text-secondary)]">构建缓存</span>
    </div>
    <div class="flex items-center gap-3 text-xs text-[var(--color-text-muted)]">
      <span>{cacheStatus.file_count} 个文件</span>
      <span>{formatBytes(cacheStatus.total_size)}</span>
      <span>命中率 {cacheStatus.hit_rate.toFixed(0)}%</span>
    </div>
  </div>
{/if}

<!-- Status -->
{#if status}
  {@const cfg = statusConfig[status] || statusConfig.pending}
  <div class="mb-4 p-4 rounded-2xl border {cfg.bg} flex flex-wrap items-center gap-2 sm:gap-3 overflow-x-hidden" style="border-color: var(--color-border)">
    <span class="material-symbols-outlined text-[22px] {cfg.color}">{cfg.icon}</span>
    <span class="text-sm font-semibold {cfg.color} uppercase">{status}</span>
    <span class="text-xs px-2 py-0.5 rounded-full flex items-center gap-1 whitespace-nowrap" style="background: var(--color-bg-elevated); color: var(--color-text-muted)">
      <span class="material-symbols-outlined text-[14px]">{triggerIcons[triggerMode] || 'build'}</span>
      {triggerMode === 'manual' ? '手动' : triggerMode === 'git' ? 'Git' : triggerMode === 'schedule' ? '定时' : triggerMode}
    </span>
    <span class="text-xs px-2 py-0.5 rounded-full flex items-center gap-1 whitespace-nowrap" style="background: var(--color-bg-elevated); color: var(--color-text-muted)">
      <span class="material-symbols-outlined text-[14px]">memory</span>
      {selectedTarget}
    </span>
    {#if status === 'running'}
      <div class="ml-auto flex gap-1">
        <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_infinite]"></div>
        <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_0.3s_infinite]"></div>
        <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_0.6s_infinite]"></div>
      </div>
    {/if}
  </div>
{/if}

<!-- Incremental Build Info -->
{#if incrementalInfo}
  <div class="mb-4 p-3 rounded-2xl border flex items-start gap-3 {incrementalInfo.needs_rebuild ? 'bg-amber-500/10' : 'bg-green-500/10'}"
       style="border-color: {incrementalInfo.needs_rebuild ? 'rgba(245,158,11,0.3)' : 'rgba(34,197,94,0.3)'}">
    <span class="material-symbols-outlined text-[20px] mt-0.5 {incrementalInfo.needs_rebuild ? 'text-amber-500' : 'text-green-500'}">
      {incrementalInfo.needs_rebuild ? 'difference' : 'check_circle'}
    </span>
    <div class="flex-1">
      <span class="text-sm font-semibold {incrementalInfo.needs_rebuild ? 'text-amber-500' : 'text-green-500'}">
        {incrementalInfo.needs_rebuild ? '增量编译' : '无变化'}
      </span>
      {#if incrementalInfo.needs_rebuild}
        <p class="text-xs text-[var(--color-text-muted)] mt-1">{incrementalInfo.reason}</p>
        <div class="flex gap-3 mt-1.5 text-xs text-[var(--color-text-muted)]">
          {#if incrementalInfo.changed_files.length > 0}<span>📝 {incrementalInfo.changed_files.length} 个文件变更</span>{/if}
          {#if incrementalInfo.new_files.length > 0}<span>🆕 {incrementalInfo.new_files.length} 个新文件</span>{/if}
          {#if incrementalInfo.removed_files.length > 0}<span>🗑️ {incrementalInfo.removed_files.length} 个已删除</span>{/if}
        </div>
      {:else}
        <p class="text-xs text-[var(--color-text-muted)] mt-1">所有源文件未变化，使用缓存的二进制</p>
      {/if}
    </div>
  </div>
{/if}

<!-- Log -->
{#if logLines.length > 0}
  <div class="rounded-2xl border overflow-hidden min-w-0" style="border-color: rgba(34,197,94,0.2); box-shadow: 0 0 30px rgba(34, 197, 94, 0.1)">
    <div class="px-4 py-2.5 flex items-center gap-2" style="background: rgba(34,197,94,0.1); border-bottom: 1px solid rgba(34,197,94,0.2)">
      <span class="material-symbols-outlined text-[16px]" style="color: #4ade80">terminal</span>
      <span class="text-xs font-medium" style="color: #4ade80">构建日志</span>
      <div class="ml-auto flex items-center gap-2">
        <span class="text-[10px]" style="color: rgba(74,222,128,0.6)">{logLines.length} 行</span>
        <div class="flex gap-1">
          <div class="w-2.5 h-2.5 rounded-full" style="background: #ef4444"></div>
          <div class="w-2.5 h-2.5 rounded-full" style="background: #f59e0b"></div>
          <div class="w-2.5 h-2.5 rounded-full" style="background: #22c55e"></div>
        </div>
      </div>
    </div>
    <div class="p-2 text-[11px] font-mono overflow-x-auto overflow-y-auto max-h-80 min-w-0 log-container" style="background: #0a0a0a; color: #4ade80; white-space: pre-wrap; word-break: break-all">{#each logLines as line, i}<div class="log-line">{String(i + 1).padStart(3, ' ')} <span class:text-red-400={line.startsWith('[ERROR]')} class:text-amber-400={line.startsWith('[WARN]')} class:text-green-300={line.startsWith('[SUCCESS]')} class:text-green-400={!line.startsWith('[') || line.startsWith('[INFO]')}>{line}</span></div>{/each}</div>
  </div>
{:else if building}
  <div class="rounded-2xl border p-8 text-center" style="border-color: var(--color-border); background: var(--color-bg-elevated)">
    <div class="inline-flex items-center gap-2 mb-3">
      <div class="w-6 h-6 border-2 rounded-full animate-spin" style="border-color: var(--color-border); border-top-color: var(--color-primary)"></div>
    </div>
    <p class="text-sm" style="color: var(--color-text-muted)">等待构建日志...</p>
  </div>
{/if}

<!-- Cache Hit -->
{#if buildCached}
  <div class="mb-4 p-3 rounded-2xl border flex items-center gap-3" style="background: rgba(34,197,94,0.1); border-color: rgba(34,197,94,0.3)">
    <span class="material-symbols-outlined text-[20px] text-green-500">cached</span>
    <span class="text-sm font-semibold text-green-500">缓存命中</span>
    <span class="text-xs text-[var(--color-text-muted)]">使用缓存构建产物</span>
  </div>
{/if}

<!-- Download -->
{#if status === 'success' && taskId}
  {@const dlToken = typeof localStorage !== 'undefined' ? localStorage.getItem('moduforge_token') || '' : ''}
  <a
    href="/api/v1/builds/{taskId}/download?token={encodeURIComponent(dlToken)}"
    class="mt-4 w-full py-3 rounded-xl font-semibold text-sm text-center no-underline
      bg-green-500 text-white hover:bg-green-600 transition-all duration-200 flex items-center justify-center gap-2"
    target="_blank"
  >
    <span class="material-symbols-outlined text-[18px]">download</span>
    下载构建产物
  </a>
{/if}

<style>
  .log-container {
    line-height: 1.2;
  }
  .log-line {
    margin: 0;
    padding: 0;
    line-height: 1.2;
    min-height: 1.2em;
  }
</style>

<script lang="ts">
  let {
    open,
    detail,
    loading,
    onClose,
    onRefresh,
  }: {
    open: boolean;
    detail: any;
    loading: boolean;
    onClose: () => void;
    onRefresh: () => void;
  } = $props();

  function getStatusColor(status: string) {
    return status === 'healthy' ? 'var(--color-success)' : status === 'warning' ? 'var(--color-warning)' : 'var(--color-error)';
  }

  function getStatusLabel(status: string) {
    return status === 'healthy' ? '系统健康' : status === 'warning' ? '存在警告' : '系统异常';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }

  function formatValue(prop: string, val: any): string {
    if (typeof val !== 'number') return String(val);
    if (prop.includes('mb')) return val.toFixed(1) + 'MB';
    if (prop.includes('gb')) return val.toFixed(1) + 'GB';
    if (prop.includes('ms')) return Math.round(val) + 'ms';
    return String(val);
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div
    class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]"
    style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
  >
    <div
      class="rounded-2xl max-w-lg w-full border animate-[scaleIn_0.2s_ease-out] max-h-[80vh] overflow-hidden flex flex-col"
      style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <div class="flex items-center gap-3">
          <div class="w-8 h-8 rounded-lg flex items-center justify-center" style="background: var(--color-success-light)">
            <span class="material-symbols-outlined text-[16px]" style="color: var(--color-success)">monitor_heart</span>
          </div>
          <div>
            <h3 class="text-base font-bold text-[var(--color-text)]">健康检查详情</h3>
            <p class="text-xs" style="color: var(--color-text-muted)">系统运行状态和资源使用</p>
          </div>
        </div>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>

      <div class="p-5 overflow-auto flex-1">
        {#if loading}
          <div class="flex items-center justify-center py-8">
            <div class="animate-spin h-8 w-8 border-2 border-primary-500 border-t-transparent rounded-full"></div>
          </div>
        {:else if detail}
          <!-- Overall Status -->
          <div class="flex items-center gap-3 mb-5 p-3 rounded-xl" style="background: {getStatusColor(detail.status)}22">
            <span class="w-3 h-3 rounded-full" style="background: {getStatusColor(detail.status)}"></span>
            <span class="text-sm font-semibold" style="color: {getStatusColor(detail.status)}">
              {getStatusLabel(detail.status)}
            </span>
            <span class="text-xs ml-auto" style="color: var(--color-text-muted)">
              运行 {detail.uptime} · 检查耗时 {detail.check_ms}ms
            </span>
          </div>

          <!-- Check Details -->
          <div class="space-y-3">
            {#each Object.entries(detail.checks || {}) as [key, check]}
              {@const checkStatus = (check as any).status}
              <div class="p-3 rounded-xl border" style="border-color: var(--color-border); background: var(--color-surface)">
                <div class="flex items-center justify-between mb-2">
                  <div class="flex items-center gap-2">
                    <span class="w-2 h-2 rounded-full" style="background: {getStatusColor(checkStatus === 'ok' ? 'healthy' : checkStatus)}"></span>
                    <span class="text-sm font-semibold text-[var(--color-text)]">{key}</span>
                  </div>
                  <span
                    class="text-xs px-2 py-0.5 rounded-full"
                    style="background: {getStatusColor(checkStatus === 'ok' ? 'healthy' : checkStatus)}22; color: {getStatusColor(checkStatus === 'ok' ? 'healthy' : checkStatus)}"
                  >
                    {checkStatus}
                  </span>
                </div>
                <div class="grid grid-cols-2 gap-2 text-xs">
                  {#each Object.entries(check as any) as [prop, val]}
                    {#if prop !== 'status' && prop !== 'error'}
                      <div>
                        <span style="color: var(--color-text-muted)">{prop}:</span>
                        <span class="font-medium text-[var(--color-text)] ml-1">{formatValue(prop, val)}</span>
                      </div>
                    {/if}
                  {/each}
                  {#if (check as any).error}
                    <div class="col-span-2 mt-1">
                      <span class="text-[var(--color-error)]">错误: {(check as any).error}</span>
                    </div>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="text-center py-8">
            <p class="text-sm text-[var(--color-text-muted)]">点击"刷新"加载详细健康信息</p>
          </div>
        {/if}
      </div>

      <div class="p-4 border-t flex justify-end" style="border-color: var(--color-border)">
        <button class="btn-primary text-sm" onclick={onRefresh} disabled={loading}>
          {loading ? '刷新中...' : '刷新'}
        </button>
      </div>
    </div>
  </div>
{/if}

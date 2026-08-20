<script lang="ts">
  let {
    data,
    loading,
    onViewDetail,
  }: {
    data: any;
    loading: boolean;
    onViewDetail: () => void;
  } = $props();

  function getStatusColor(status: string) {
    return status === 'healthy' ? 'var(--color-success)' : status === 'warning' ? 'var(--color-warning)' : 'var(--color-error)';
  }
</script>

{#if data}
  <div class="flex items-center gap-2 mb-3">
    <span class="w-2.5 h-2.5 rounded-full" style="background: {getStatusColor(data.status)}"></span>
    <span class="text-sm font-medium" style="color: {getStatusColor(data.status)}">
      {data.status === 'healthy' ? '健康' : data.status === 'warning' ? '警告' : '异常'}
    </span>
    <span class="text-xs ml-auto" style="color: var(--color-text-muted)">运行 {data.uptime}</span>
  </div>
  <div class="grid grid-cols-2 gap-2 mb-3">
    {#each Object.entries(data.checks || {}) as [key, check]}
      {@const checkStatus = (check as any).status}
      <div
        role="button"
        tabindex="0"
        class="p-2 rounded-lg cursor-pointer hover:opacity-80 transition-opacity"
        style="background: var(--color-surface);"
        onclick={() => onViewDetail()}
        onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onViewDetail(); } }}
      >
        <div class="flex items-center justify-between mb-0.5">
          <span class="text-[10px] font-medium" style="color: var(--color-text-secondary)">{key}</span>
          <span
            class="w-1.5 h-1.5 rounded-full"
            style="background: {checkStatus === 'ok' ? 'var(--color-success)' : checkStatus === 'warning' ? 'var(--color-warning)' : 'var(--color-error)'}"
          ></span>
        </div>
        {#if (check as any).response_ms != null}
          <p class="text-sm font-bold text-[var(--color-text)]">{(check as any).response_ms}ms</p>
        {:else if (check as any).free_gb != null}
          <p class="text-sm font-bold text-[var(--color-text)]">{(check as any).free_gb.toFixed(1)}GB</p>
        {:else if (check as any).used_mb != null}
          <p class="text-sm font-bold text-[var(--color-text)]">{(check as any).used_mb.toFixed(1)}MB</p>
        {:else}
          <p class="text-sm font-bold text-[var(--color-text)]">{(check as any).status}</p>
        {/if}
      </div>
    {/each}
  </div>
  <button
    class="text-xs w-full py-1.5 rounded-lg text-center hover:opacity-80 transition-opacity"
    style="background: var(--color-surface); color: var(--color-text-secondary)"
    onclick={() => onViewDetail()}
  >
    查看详细信息
  </button>
{:else}
  <p class="text-sm text-[var(--color-text-muted)]">加载中...</p>
{/if}

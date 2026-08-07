<script lang="ts">
  import Skeleton from '$lib/components/ui/Skeleton.svelte';

  let { data = [], loading = false }: { data?: { date: string; success: number; failed: number }[]; loading?: boolean } = $props();

  function formatDate(dateStr: string): string {
    try {
      const d = new Date(dateStr);
      return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
    } catch {
      return dateStr;
    }
  }
</script>

<div class="widget">
  <h3 class="widget-title">构建趋势 (30天)</h3>

  {#if loading}
    <Skeleton count={2} lines={[100, 40]} />
  {:else if data.length === 0}
    <div class="empty">暂无数据</div>
  {:else}
    <div class="trends-chart">
      {#each data.slice(-14) as item (item.date)}
        <div class="trend-bar" title="{formatDate(item.date)}: 成功 {item.success}, 失败 {item.failed}">
          <div class="bar-stack">
            {#if item.success > 0}
              <div class="bar success" style="height: {Math.min(item.success * 10, 100)}%"></div>
            {/if}
            {#if item.failed > 0}
              <div class="bar failed" style="height: {Math.min(item.failed * 10, 100)}%"></div>
            {/if}
          </div>
          <span class="bar-label">{formatDate(item.date)}</span>
        </div>
      {/each}
    </div>
    <div class="legend">
      <span class="legend-item"><span class="legend-dot success"></span> 成功</span>
      <span class="legend-item"><span class="legend-dot failed"></span> 失败</span>
    </div>
  {/if}
</div>

<style>
  .widget {
    padding: 1rem;
    background: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: 0.75rem;
  }

  .widget-title {
    margin: 0 0 1rem;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--color-text-secondary);
  }

  .empty {
    text-align: center;
    padding: 1rem;
    color: var(--color-text-muted);
    font-size: 0.875rem;
  }

  .trends-chart {
    display: flex;
    gap: 4px;
    height: 80px;
    align-items: flex-end;
  }

  .trend-bar {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .bar-stack {
    width: 100%;
    height: 60px;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    gap: 1px;
  }

  .bar {
    width: 100%;
    min-height: 2px;
    border-radius: 2px;
  }

  .bar.success { background: var(--color-success); }
  .bar.failed { background: var(--color-danger); }

  .bar-label {
    font-size: 0.5rem;
    color: var(--color-text-muted);
    writing-mode: vertical-rl;
    text-orientation: mixed;
    max-height: 40px;
    overflow: hidden;
  }

  .legend {
    display: flex;
    gap: 1rem;
    margin-top: 0.75rem;
    justify-content: center;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.75rem;
    color: var(--color-text-secondary);
  }

  .legend-dot {
    width: 8px;
    height: 8px;
    border-radius: 2px;
  }

  .legend-dot.success { background: var(--color-success); }
  .legend-dot.failed { background: var(--color-danger); }
</style>

<script lang="ts">
  import Skeleton from '$lib/components/ui/Skeleton.svelte';

  let { data = null, loading = false }: { data?: { total: number; success: number; failed: number; cancelled: number; avg_time: number; success_rate?: number } | null; loading?: boolean } = $props();
</script>

<div class="widget">
  <h3 class="widget-title">构建统计</h3>

  {#if loading}
    <Skeleton count={3} lines={[70, 90, 60]} />
  {:else if !data}
    <div class="empty">暂无数据</div>
  {:else}
    <div class="stats-row">
      <div class="stat">
        <span class="stat-value success">{data.success || 0}</span>
        <span class="stat-label">成功</span>
      </div>
      <div class="stat">
        <span class="stat-value danger">{data.failed || 0}</span>
        <span class="stat-label">失败</span>
      </div>
      <div class="stat">
        <span class="stat-value">{data.total || 0}</span>
        <span class="stat-label">总计</span>
      </div>
    </div>

    {#if data.success_rate !== undefined}
      <div class="rate-bar">
        <div class="rate-fill" style="width: {data.success_rate}%"></div>
      </div>
      <span class="rate-text">成功率: {data.success_rate.toFixed(1)}%</span>
    {/if}
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

  .stats-row {
    display: flex;
    gap: 1.5rem;
  }

  .stat {
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
  }

  .stat-value {
    font-size: 1.25rem;
    font-weight: 600;
  }

  .stat-value.success { color: var(--color-success); }
  .stat-value.danger { color: var(--color-danger); }

  .stat-label {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
  }

  .rate-bar {
    height: 6px;
    background: var(--color-bg);
    border-radius: 3px;
    overflow: hidden;
    margin-top: 1rem;
  }

  .rate-fill {
    height: 100%;
    background: var(--color-success);
    border-radius: 3px;
  }

  .rate-text {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    margin-top: 0.25rem;
    display: block;
  }
</style>

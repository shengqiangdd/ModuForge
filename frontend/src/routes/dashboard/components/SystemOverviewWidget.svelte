<script lang="ts">
  import Skeleton from '$lib/components/ui/Skeleton.svelte';

  let { data = null, loading = false, t = (k: string) => k }: { data?: { projects: number; users: number; total_builds: number; total_modules: number } | null; loading?: boolean; t?: (k: string) => string } = $props();
</script>

<div class="widget">
  <h3 class="widget-title">系统概览</h3>

  {#if loading}
    <Skeleton count={4} lines={[80, 60, 70, 50]} />
  {:else if !data}
    <div class="empty">暂无数据</div>
  {:else}
    <div class="stats-grid">
      {#each [
        { icon: 'folder', label: t('dashboard.projects'), value: data.projects ?? 0, color: 'from-violet-500 to-purple-600' },
        { icon: 'group', label: t('dashboard.users'), value: data.users ?? 0, color: 'from-cyan-500 to-blue-600' },
        { icon: 'build', label: t('dashboard.total_builds'), value: data.total_builds ?? 0, color: 'from-emerald-500 to-green-600' },
        { icon: 'inventory_2', label: t('dashboard.total_modules'), value: data.total_modules ?? 0, color: 'from-amber-500 to-orange-600' },
      ] as card}
        <div class="stat-item">
          <div class="stat-icon" style="background: var(--gradient-brand)">
            <span class="material-symbols-outlined text-white text-[14px]">{card.icon}</span>
          </div>
          <p class="stat-label">{card.label}</p>
          <p class="stat-value">{card.value}</p>
        </div>
      {/each}
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

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
  }

  .stat-item {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .stat-icon {
    width: 2rem;
    height: 2rem;
    border-radius: 0.5rem;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 0.25rem;
  }

  .stat-label {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    margin: 0;
  }

  .stat-value {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
    font-variant-numeric: tabular-nums;
  }
</style>

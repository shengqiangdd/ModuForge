<script lang="ts">
  import type { InstallStat, MarketModule } from './types';

  let {
    installStats = [],
    statsPeriod = 'day',
    statsLoading = false,
    trendingModules = [],
    onPeriodChange,
  }: {
    installStats?: InstallStat[];
    statsPeriod?: 'day' | 'week' | 'month';
    statsLoading?: boolean;
    trendingModules?: MarketModule[];
    onPeriodChange?: (period: 'day' | 'week' | 'month') => void;
  } = $props();

  const maxCount = $derived(Math.max(1, ...installStats.map(s => s.count)));
</script>

<div class="p-6">
  <h3 class="text-sm font-semibold text-[var(--color-text)] mb-4">安装统计</h3>
  <div class="flex items-center gap-2 mb-4">
    {#each ['day', 'week', 'month'] as p}
      <button
        class="px-3 py-1 rounded-lg text-xs transition-colors"
        style={statsPeriod === p ? 'background: var(--color-primary-light); color: var(--color-primary); font-weight: 600' : 'background: var(--color-surface); color: var(--color-text-muted)'}
        onclick={() => onPeriodChange?.(p as 'day' | 'week' | 'month')}
      >{p === 'day' ? '按日' : p === 'week' ? '按周' : '按月'}</button>
    {/each}
  </div>

  {#if statsLoading}
    <p class="text-xs text-[var(--color-text-muted)]">加载中...</p>
  {:else if installStats.length === 0}
    <p class="text-xs text-[var(--color-text-muted)]">暂无安装数据</p>
  {:else}
    <div class="flex items-end gap-1 h-32 mb-4">
      {#each installStats.slice(-20) as pt}
        <div class="flex-1 flex flex-col items-center gap-0.5 group relative">
          <div class="absolute bottom-full mb-1 hidden group-hover:block bg-[var(--color-bg-elevated)] rounded px-2 py-1 text-xs whitespace-nowrap z-10 border border-[var(--color-border)]">
            {pt.period}: {pt.count}
          </div>
          <div class="w-full rounded-t-sm transition-all" style="height: {(pt.count / maxCount) * 100}%; background: var(--gradient-brand)"></div>
          <span class="text-[8px] text-[var(--color-text-muted)] truncate w-full text-center">{pt.period.slice(-5)}</span>
        </div>
      {/each}
    </div>
  {/if}

  <h3 class="text-sm font-semibold text-[var(--color-text)] mb-4 mt-6">热门模块 Top 10</h3>
  {#if trendingModules.length === 0}
    <p class="text-xs text-[var(--color-text-muted)]">加载中...</p>
  {:else}
    <div class="space-y-2">
      {#each trendingModules as mod, i (mod.id)}
        <div class="flex items-center gap-3 py-1.5">
          <span
            class="w-5 h-5 rounded-full flex items-center justify-center text-xs font-bold"
            style="background: {i < 3 ? 'var(--gradient-brand)' : 'var(--color-surface)'}; color: {i < 3 ? 'white' : 'var(--color-text-muted)'}"
          >{i + 1}</span>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-[var(--color-text)] truncate">{mod.title}</p>
            <p class="text-xs text-[var(--color-text-muted)]">{mod.installs} 安装 · {mod.stars} 星</p>
          </div>
          <span class="text-xs text-[var(--color-text-muted)]">{mod.category}</span>
        </div>
      {/each}
    </div>
  {/if}
</div>

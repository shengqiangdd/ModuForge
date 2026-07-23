<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n';

  let systemStats: any = $state(null);
  let buildStats: any = $state(null);
  let buildTrends: any[] = $state([]);
  let moduleStats: any = $state(null);
  let loading = $state(true);

  async function loadAll() {
    loading = true;
    try {
      const [sys, build, trends, mod] = await Promise.allSettled([
        fetch('/api/v1/analytics/system').then(r => r.json()),
        fetch('/api/v1/analytics/build-stats').then(r => r.json()),
        fetch('/api/v1/analytics/build-trends?days=30').then(r => r.json()),
        fetch('/api/v1/analytics/module-stats').then(r => r.json()),
      ]);
      if (sys.status === 'fulfilled') systemStats = sys.value;
      if (build.status === 'fulfilled') buildStats = build.value;
      if (trends.status === 'fulfilled') buildTrends = trends.value?.trends || [];
      if (mod.status === 'fulfilled') moduleStats = mod.value;
    } catch {}
    loading = false;
  }

  onMount(loadAll);

  let maxTrend = $derived(Math.max(1, ...buildTrends.map((t: any) => t.count || 0)));
</script>

<div class="p-4 md:p-6 max-w-7xl mx-auto">
  <!-- Header -->
  <div class="flex items-center justify-between mb-8">
    <div>
      <h1 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">{$t('dashboard.title')}</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">实时监控系统运行状态</p>
    </div>
    <button class="btn-ghost flex items-center gap-2 text-sm" onclick={loadAll} disabled={loading}>
      <span class="material-symbols-outlined text-[18px] {loading ? 'animate-spin' : ''}" style="color: var(--color-text-muted)">refresh</span>
      {$t('dashboard.refresh')}
    </button>
  </div>

  {#if loading}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
      {#each Array(4) as _}
        <div class="card p-5"><div class="skeleton h-4 w-20 mb-2"></div><div class="skeleton h-8 w-16"></div></div>
      {/each}
    </div>
  {:else}
    <!-- Overview Cards -->
    <section class="mb-8">
      <h2 class="text-[11px] font-semibold uppercase tracking-wider mb-4" style="color: var(--color-text-muted)">{$t('dashboard.system_overview')}</h2>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        {#each [
          { icon: 'folder', label: $t('dashboard.projects'), value: systemStats?.projects ?? 0, color: 'from-violet-500 to-purple-600' },
          { icon: 'group', label: $t('dashboard.users'), value: systemStats?.users ?? 0, color: 'from-cyan-500 to-blue-600' },
          { icon: 'build', label: $t('dashboard.total_builds'), value: systemStats?.total_builds ?? 0, color: 'from-emerald-500 to-green-600' },
          { icon: 'inventory_2', label: $t('dashboard.total_modules'), value: systemStats?.total_modules ?? 0, color: 'from-amber-500 to-orange-600' },
        ] as card, i}
          <div class="card p-5 group hover:shadow-lg hover:-translate-y-0.5 transition-all duration-300" style="animation-delay: {i * 100}ms">
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 rounded-xl flex items-center justify-center shadow-md group-hover:scale-110 group-hover:rotate-3 transition-all duration-300" style="background: var(--gradient-brand)">
                <span class="material-symbols-outlined text-white text-xl">{card.icon}</span>
              </div>
              <div>
                <p class="text-xs font-medium" style="color: var(--color-text-muted)">{card.label}</p>
                <p class="text-2xl font-bold tabular-nums count-up" style="color: var(--color-text)">{card.value}</p>
              </div>
            </div>
          </div>
        {/each}
      </div>
    </section>

    <!-- Build Stats -->
    <section class="mb-8">
      <h2 class="text-[11px] font-semibold uppercase tracking-wider mb-4" style="color: var(--color-text-muted)">{$t('dashboard.build_stats')}</h2>
      <div class="card p-5">
        <div class="grid grid-cols-2 md:grid-cols-4 gap-6 mb-5">
          {#each [
            { label: $t('dashboard.total_builds'), value: buildStats?.total_builds ?? 0, color: '' },
            { label: $t('dashboard.successful'), value: buildStats?.successful_builds ?? 0, color: 'text-green-600' },
            { label: $t('dashboard.failed'), value: buildStats?.failed_builds ?? 0, color: 'text-red-500' },
            { label: $t('dashboard.avg_duration'), value: buildStats?.avg_duration_ms ? (buildStats.avg_duration_ms / 1000).toFixed(1) + 's' : '-', color: '' },
          ] as stat}
            <div>
              <p class="text-xs text-[var(--color-text-muted)] mb-1">{stat.label}</p>
              <p class="text-xl font-bold {stat.color} text-[var(--color-text)] tabular-nums">{stat.value}</p>
            </div>
          {/each}
        </div>
        {#if buildStats?.total_builds > 0}
          <div class="pt-4 border-t border-[var(--color-border)]">
            <div class="flex items-center justify-between mb-2">
              <span class="text-sm text-[var(--color-text-secondary)]">{$t('dashboard.success_rate')}</span>
              <span class="text-sm font-semibold text-[var(--color-text)]">{(buildStats?.success_rate ?? 0).toFixed(1)}%</span>
            </div>
            <div class="w-full rounded-full h-2.5" style="background: var(--color-surface)">
              <div class="rounded-full h-2.5 transition-all duration-700" style="width: {buildStats?.success_rate ?? 0}%; background: var(--gradient-brand)"></div>
            </div>
          </div>
        {/if}
      </div>
    </section>

    <!-- Build Trends -->
    <section class="mb-8">
      <h2 class="text-[11px] font-semibold uppercase tracking-wider mb-4" style="color: var(--color-text-muted)">{$t('dashboard.build_trends')}</h2>
      <div class="card p-5">
        {#if buildTrends.length === 0}
          <p class="text-[var(--color-text-muted)] text-center py-10">{$t('dashboard.no_data')}</p>
        {:else}
          <div class="flex items-end gap-0.5 h-36">
            {#each buildTrends as trend}
              <div class="flex-1 flex flex-col items-center gap-0.5 group relative min-w-0">
                <div class="absolute bottom-full mb-2 hidden group-hover:block bg-[var(--color-bg-elevated)] rounded-xl shadow-elevated px-3 py-2 text-xs whitespace-nowrap z-10 border border-[var(--color-border)]">
                  <div class="font-medium text-[var(--color-text)]">{trend.date}</div>
                  <div class="text-green-600">成功: {trend.success}</div>
                  <div class="text-red-500">失败: {trend.failed}</div>
                </div>
                <div class="w-full flex flex-col justify-end" style="height: 100px;">
                  <div class="w-full rounded-t-sm" style="height: {((trend.success || 0) / maxTrend) * 100}%; background: var(--color-primary)"></div>
                  <div class="w-full rounded-t-sm" style="height: {((trend.failed || 0) / maxTrend) * 100}%; background: var(--color-error)"></div>
                </div>
                <span class="text-[9px] text-[var(--color-text-muted)] truncate w-full text-center">{trend.date?.slice(5)}</span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </section>

    <!-- Market + System -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
      <section>
        <h2 class="text-[11px] font-semibold uppercase tracking-wider mb-4" style="color: var(--color-text-muted)">{$t('dashboard.market_stats')}</h2>
        <div class="card p-5 space-y-4">
          <div class="grid grid-cols-3 gap-4">
            {#each [
              { label: $t('dashboard.total_modules'), value: moduleStats?.total_modules ?? 0 },
              { label: $t('dashboard.total_installs'), value: moduleStats?.total_installs ?? 0 },
              { label: $t('dashboard.total_stars'), value: moduleStats?.total_stars ?? 0 },
            ] as s}
              <div>
                <p class="text-xs text-[var(--color-text-muted)] mb-1">{s.label}</p>
                <p class="text-lg font-bold text-[var(--color-text)] tabular-nums">{s.value}</p>
              </div>
            {/each}
          </div>
          {#if moduleStats?.top_categories?.length > 0}
            <div class="pt-3 border-t border-[var(--color-border)]">
              <p class="text-xs text-[var(--color-text-muted)] mb-3">{$t('dashboard.top_categories')}</p>
              <div class="space-y-2.5">
                {#each moduleStats.top_categories as cat}
                  {@const maxC = Math.max(...moduleStats.top_categories.map((c: any) => c.count))}
                  <div>
                    <div class="flex justify-between text-xs mb-1">
                      <span class="text-[var(--color-text-secondary)]">{cat.category}</span>
                      <span class="text-[var(--color-text-muted)]">{cat.count}</span>
                    </div>
                    <div class="w-full rounded-full h-1.5" style="background: var(--color-surface)">
                      <div class="rounded-full h-1.5" style="width: {(cat.count / maxC) * 100}%; background: var(--gradient-brand)"></div>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>
      </section>

      <section>
        <h2 class="text-[11px] font-semibold uppercase tracking-wider mb-4" style="color: var(--color-text-muted)">{$t('dashboard.system_info')}</h2>
        <div class="card p-5 space-y-0">
          {#each [
            { label: $t('dashboard.uptime'), value: systemStats?.uptime ?? '-' },
            { label: $t('dashboard.db_size'), value: systemStats?.db_size ?? '-' },
            { label: $t('dashboard.projects'), value: systemStats?.projects ?? 0 },
            { label: $t('dashboard.users'), value: systemStats?.users ?? 0 },
          ] as item, i}
            <div class="flex justify-between items-center py-3 {i < 3 ? 'border-b border-[var(--color-border)]' : ''}">
              <span class="text-sm text-[var(--color-text-secondary)]">{item.label}</span>
              <span class="text-sm font-medium text-[var(--color-text)]">{item.value}</span>
            </div>
          {/each}
        </div>
      </section>
    </div>
  {/if}
</div>

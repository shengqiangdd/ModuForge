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
        fetch('/api/v1/analytics/system').then((r) => r.json()),
        fetch('/api/v1/analytics/build-stats').then((r) => r.json()),
        fetch('/api/v1/analytics/build-trends?days=30').then((r) => r.json()),
        fetch('/api/v1/analytics/module-stats').then((r) => r.json()),
      ]);

      if (sys.status === 'fulfilled') systemStats = sys.value;
      if (build.status === 'fulfilled') buildStats = build.value;
      if (trends.status === 'fulfilled') buildTrends = trends.value?.trends || [];
      if (mod.status === 'fulfilled') moduleStats = mod.value;
    } catch {}
    loading = false;
  }

  onMount(loadAll);

  let maxTrendCount = $derived(Math.max(1, ...buildTrends.map((t: any) => t.count || 0)));
</script>

<div class="p-6 max-w-7xl mx-auto">
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold text-on-surface">{$t('dashboard.title')}</h1>
    <md-filled-tonal-button onclick={loadAll} disabled={loading}>
      <md-icon slot="start">refresh</md-icon>
      {$t('dashboard.refresh')}
    </md-filled-tonal-button>
  </div>

  {#if loading}
    <div class="flex justify-center p-12">
      <md-circular-progress indeterminate></md-circular-progress>
    </div>
  {:else}
    <!-- System Overview Cards -->
    <section class="mb-8">
      <h2 class="text-lg font-semibold text-on-surface mb-3">{$t('dashboard.system_overview')}</h2>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant">
          <div class="flex items-center gap-3">
            <span class="material-symbols-outlined text-3xl text-primary">folder</span>
            <div>
              <p class="text-body-small text-on-surface-variant">{$t('dashboard.projects')}</p>
              <p class="text-headline-small font-bold text-on-surface">{systemStats?.projects ?? 0}</p>
            </div>
          </div>
        </div>
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant">
          <div class="flex items-center gap-3">
            <span class="material-symbols-outlined text-3xl text-secondary">group</span>
            <div>
              <p class="text-body-small text-on-surface-variant">{$t('dashboard.users')}</p>
              <p class="text-headline-small font-bold text-on-surface">{systemStats?.users ?? 0}</p>
            </div>
          </div>
        </div>
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant">
          <div class="flex items-center gap-3">
            <span class="material-symbols-outlined text-3xl text-tertiary">hammer</span>
            <div>
              <p class="text-body-small text-on-surface-variant">{$t('dashboard.total_builds')}</p>
              <p class="text-headline-small font-bold text-on-surface">{systemStats?.total_builds ?? 0}</p>
            </div>
          </div>
        </div>
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant">
          <div class="flex items-center gap-3">
            <span class="material-symbols-outlined text-3xl text-primary">inventory_2</span>
            <div>
              <p class="text-body-small text-on-surface-variant">{$t('dashboard.total_modules')}</p>
              <p class="text-headline-small font-bold text-on-surface">{systemStats?.total_modules ?? 0}</p>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Build Stats -->
    <section class="mb-8">
      <h2 class="text-lg font-semibold text-on-surface mb-3">{$t('dashboard.build_stats')}</h2>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant">
          <p class="text-body-small text-on-surface-variant mb-1">{$t('dashboard.total_builds')}</p>
          <p class="text-title-large font-bold text-on-surface">{buildStats?.total_builds ?? 0}</p>
        </div>
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant">
          <p class="text-body-small text-on-surface-variant mb-1">{$t('dashboard.successful')}</p>
          <p class="text-title-large font-bold text-green-600">{buildStats?.successful_builds ?? 0}</p>
        </div>
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant">
          <p class="text-body-small text-on-surface-variant mb-1">{$t('dashboard.failed')}</p>
          <p class="text-title-large font-bold text-red-600">{buildStats?.failed_builds ?? 0}</p>
        </div>
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant">
          <p class="text-body-small text-on-surface-variant mb-1">{$t('dashboard.avg_duration')}</p>
          <p class="text-title-large font-bold text-on-surface">{buildStats?.avg_duration_ms ? (buildStats.avg_duration_ms / 1000).toFixed(1) + 's' : '-'}</p>
        </div>
      </div>
      {#if buildStats?.total_builds > 0}
        <div class="mt-3 bg-surface-container rounded-xl p-4 border border-outline-variant">
          <div class="flex items-center justify-between mb-2">
            <span class="text-body-small text-on-surface-variant">{$t('dashboard.success_rate')}</span>
            <span class="text-body-medium font-semibold text-on-surface">{(buildStats?.success_rate ?? 0).toFixed(1)}%</span>
          </div>
          <div class="w-full bg-surface-container-highest rounded-full h-3">
            <div
              class="bg-primary rounded-full h-3 transition-all duration-500"
              style="width: {buildStats?.success_rate ?? 0}%"
            ></div>
          </div>
        </div>
      {/if}
    </section>

    <!-- Build Trends Chart -->
    <section class="mb-8">
      <h2 class="text-lg font-semibold text-on-surface mb-3">{$t('dashboard.build_trends')}</h2>
      <div class="bg-surface-container rounded-xl p-4 border border-outline-variant">
        {#if buildTrends.length === 0}
          <p class="text-on-surface-variant text-center py-8">{$t('dashboard.no_data')}</p>
        {:else}
          <div class="flex items-end gap-1 h-40">
            {#each buildTrends as trend}
              <div class="flex-1 flex flex-col items-center gap-1 group relative">
                <div class="absolute bottom-full mb-2 hidden group-hover:block bg-surface-elevated rounded-lg shadow-lg px-2 py-1 text-xs whitespace-nowrap z-10 border border-outline-variant">
                  <div>{trend.date}</div>
                  <div class="text-green-600">{$t('dashboard.successful')}: {trend.success}</div>
                  <div class="text-red-600">{$t('dashboard.failed')}: {trend.failed}</div>
                </div>
                <div class="w-full flex flex-col justify-end" style="height: 120px;">
                  <div
                    class="w-full bg-primary rounded-t"
                    style="height: {((trend.success || 0) / maxTrendCount) * 100}%"
                  ></div>
                  <div
                    class="w-full bg-red-400 rounded-t"
                    style="height: {((trend.failed || 0) / maxTrendCount) * 100}%"
                  ></div>
                </div>
                <span class="text-[10px] text-on-surface-variant truncate w-full text-center">{trend.date?.slice(5)}</span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </section>

    <!-- Market Stats + System Info -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
      <!-- Market Stats -->
      <section>
        <h2 class="text-lg font-semibold text-on-surface mb-3">{$t('dashboard.market_stats')}</h2>
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant space-y-4">
          <div class="grid grid-cols-2 gap-4">
            <div>
              <p class="text-body-small text-on-surface-variant">{$t('dashboard.total_modules')}</p>
              <p class="text-title-medium font-bold text-on-surface">{moduleStats?.total_modules ?? 0}</p>
            </div>
            <div>
              <p class="text-body-small text-on-surface-variant">{$t('dashboard.total_installs')}</p>
              <p class="text-title-medium font-bold text-on-surface">{moduleStats?.total_installs ?? 0}</p>
            </div>
          </div>
          <div>
            <p class="text-body-small text-on-surface-variant mb-2">{$t('dashboard.total_stars')}</p>
            <p class="text-title-medium font-bold text-on-surface">{moduleStats?.total_stars ?? 0}</p>
          </div>
          {#if moduleStats?.top_categories?.length > 0}
            <div>
              <p class="text-body-small text-on-surface-variant mb-2">{$t('dashboard.top_categories')}</p>
              <div class="space-y-2">
                {#each moduleStats.top_categories as cat}
                  {@const maxCat = Math.max(...moduleStats.top_categories.map((c: any) => c.count))}
                  <div>
                    <div class="flex justify-between text-body-small mb-1">
                      <span class="text-on-surface">{cat.category}</span>
                      <span class="text-on-surface-variant">{cat.count}</span>
                    </div>
                    <div class="w-full bg-surface-container-highest rounded-full h-2">
                      <div
                        class="bg-secondary rounded-full h-2"
                        style="width: {(cat.count / maxCat) * 100}%"
                      ></div>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>
      </section>

      <!-- System Info -->
      <section>
        <h2 class="text-lg font-semibold text-on-surface mb-3">{$t('dashboard.system_info')}</h2>
        <div class="bg-surface-container rounded-xl p-4 border border-outline-variant space-y-3">
          <div class="flex justify-between items-center py-2 border-b border-outline-variant">
            <span class="text-body-medium text-on-surface-variant">{$t('dashboard.uptime')}</span>
            <span class="text-body-medium font-medium text-on-surface">{systemStats?.uptime ?? '-'}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-outline-variant">
            <span class="text-body-medium text-on-surface-variant">{$t('dashboard.db_size')}</span>
            <span class="text-body-medium font-medium text-on-surface">{systemStats?.db_size ?? '-'}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-outline-variant">
            <span class="text-body-medium text-on-surface-variant">{$t('dashboard.projects')}</span>
            <span class="text-body-medium font-medium text-on-surface">{systemStats?.projects ?? 0}</span>
          </div>
          <div class="flex justify-between items-center py-2">
            <span class="text-body-medium text-on-surface-variant">{$t('dashboard.users')}</span>
            <span class="text-body-medium font-medium text-on-surface">{systemStats?.users ?? 0}</span>
          </div>
        </div>
      </section>
    </div>
  {/if}
</div>

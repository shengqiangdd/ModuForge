<script lang="ts">
  import { onMount } from 'svelte';

  interface Overview {
    total_projects: number;
    total_builds: number;
    success_rate: number;
    active_users: number;
  }

  interface BuildTrend {
    date: string;
    builds: number;
    successes: number;
    failures: number;
  }

  interface PopularModule {
    name: string;
    slug: string;
    downloads: number;
    stars?: number;
  }

  interface UsageStat {
    date: string;
    api_calls: number;
    unique_users: number;
  }

  let overview = $state<Overview | null>(null);
  let trends = $state<BuildTrend[]>([]);
  let modules = $state<PopularModule[]>([]);
  let usage = $state<UsageStat[]>([]);
  let loading = $state(true);
  let errorMsg = $state('');

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }

  async function loadAll() {
    loading = true;
    try {
      const headers = { Authorization: `Bearer ${getToken()}` };
      const [ov, tr, md, us] = await Promise.all([
        fetch('/api/v1/analytics/overview', { headers }),
        fetch('/api/v1/analytics/build-trends', { headers }),
        fetch('/api/v1/analytics/popular-modules', { headers }),
        fetch('/api/v1/analytics/usage', { headers }),
      ]);
      if (ov.ok) { const d = await ov.json(); overview = d; }
      if (tr.ok) { const d = await tr.json(); trends = d.trends || d || []; }
      if (md.ok) { const d = await md.json(); modules = d.modules || d || []; }
      if (us.ok) { const d = await us.json(); usage = d.usage || d || []; }
    } catch (e: any) { errorMsg = e?.message || '加载失败'; }
    loading = false;
  }

  const maxBuilds = $derived(Math.max(1, ...trends.map(t => t.builds)));
  const maxUsage = $derived(Math.max(1, ...usage.map(u => u.api_calls)));

  onMount(loadAll);
</script>

<div class="w-full p-6 max-w-5xl mx-auto space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold text-[var(--color-text)]">分析统计</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">项目构建趋势与使用分析</p>
    </div>
    <button class="px-3 py-1.5 rounded-lg text-sm flex items-center gap-1.5" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={loadAll}>
      <span class="material-symbols-outlined text-[16px]">refresh</span>
      刷新
    </button>
  </div>

  {#if errorMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-error-light); color: var(--color-error)">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="text-center py-12 text-sm" style="color: var(--color-text-muted)">加载中...</div>
  {:else}
    <!-- Overview Cards -->
    {#if overview}
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div class="card p-4 text-center">
          <p class="text-2xl font-bold text-[var(--color-text)]">{overview.total_projects}</p>
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">总项目</p>
        </div>
        <div class="card p-4 text-center">
          <p class="text-2xl font-bold text-blue-500">{overview.total_builds}</p>
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">总构建</p>
        </div>
        <div class="card p-4 text-center">
          <p class="text-2xl font-bold" style="color: {(overview.success_rate || 0) >= 80 ? '#22c55e' : '#f97316'}">{overview.success_rate ?? 0}%</p>
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">成功率</p>
        </div>
        <div class="card p-4 text-center">
          <p class="text-2xl font-bold text-purple-500">{overview.active_users}</p>
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">活跃用户</p>
        </div>
      </div>
    {/if}

    <!-- Build Trends -->
    {#if trends.length > 0}
      <div class="card p-5">
        <h3 class="text-sm font-semibold mb-4 text-[var(--color-text)]">构建趋势</h3>
        <div class="flex items-end gap-1" style="height: 120px;">
          {#each trends as t, i (i)}
            <div class="flex-1 flex flex-col items-center justify-end h-full" title="{t.date}: {t.builds} 次构建">
              <div
                class="w-full rounded-t-sm transition-all"
                style="height: {(t.builds / maxBuilds) * 100}%; background: var(--color-primary); min-height: 2px;"
              ></div>
            </div>
          {/each}
        </div>
        <div class="flex justify-between mt-2">
          {#if trends.length > 0}
            <span class="text-[10px] text-[var(--color-text-muted)]">{trends[0]?.date}</span>
            <span class="text-[10px] text-[var(--color-text-muted)]">{trends[trends.length - 1]?.date}</span>
          {/if}
        </div>
        <div class="flex gap-4 mt-2 text-xs" style="color: var(--color-text-muted)">
          <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-sm" style="background: var(--color-primary)"></span> 构建数</span>
          <span>最近 {trends.length} 个周期</span>
        </div>
      </div>
    {/if}

    <!-- Popular Modules -->
    {#if modules.length > 0}
      <div class="card p-5">
        <h3 class="text-sm font-semibold mb-3 text-[var(--color-text)]">热门模块</h3>
        <div class="space-y-2">
          {#each modules as m, i (m.slug || i)}
            <div class="flex items-center gap-3">
              <span class="text-xs font-mono text-[var(--color-text-muted)] w-5 text-right">{i + 1}</span>
              <div class="flex-1 min-w-0">
                <span class="text-sm font-medium text-[var(--color-text)]">{m.name}</span>
                <span class="text-xs text-[var(--color-text-muted)] ml-2">{m.slug}</span>
              </div>
              <span class="text-xs px-2 py-0.5 rounded-full" style="background: var(--color-primary-light); color: var(--color-primary)">
                {m.downloads.toLocaleString()} 下载
              </span>
              {#if m.stars !== undefined}
                <span class="text-xs text-yellow-500">★ {m.stars}</span>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Usage Timeline -->
    {#if usage.length > 0}
      <div class="card p-5">
        <h3 class="text-sm font-semibold mb-4 text-[var(--color-text)]">使用量时间线</h3>
        <div class="flex items-end gap-1" style="height: 100px;">
          {#each usage as u, i (i)}
            <div class="flex-1 flex flex-col items-center justify-end h-full" title="{u.date}: {u.api_calls} 次 API 调用">
              <div
                class="w-full rounded-t-sm"
                style="height: {(u.api_calls / maxUsage) * 100}%; background: #8b5cf6; min-height: 2px;"
              ></div>
            </div>
          {/each}
        </div>
        <div class="flex justify-between mt-2">
          {#if usage.length > 0}
            <span class="text-[10px] text-[var(--color-text-muted)]">{usage[0]?.date}</span>
            <span class="text-[10px] text-[var(--color-text-muted)]">{usage[usage.length - 1]?.date}</span>
          {/if}
        </div>
        <div class="flex gap-4 mt-2 text-xs" style="color: var(--color-text-muted)">
          <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-sm" style="background: #8b5cf6"></span> API 调用</span>
        </div>
      </div>
    {/if}
  {/if}
</div>

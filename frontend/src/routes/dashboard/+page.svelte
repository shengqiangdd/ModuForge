<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n';
  import ActivityFeed from '$lib/components/ActivityFeed.svelte';

  interface Widget {
    id: number; widget_type: string; title: string; config: string;
    position_x: number; position_y: number; width: number; height: number;
    is_visible: boolean; created_at: string;
  }
  interface WidgetType {
    type: string; name: string; desc: string;
  }

  let widgets = $state<Widget[]>([]);
  let widgetTypes = $state<WidgetType[]>([]);
  let loading = $state(true);
  let showAddModal = $state(false);
  let selectedWidgetType = $state('');
  let dragIdx: number | null = null;
  let dropIdx: number | null = null;
  let autoRefresh = $state(true);
  let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;

  // Data stores
  let systemStats: any = $state(null);
  let buildStats: any = $state(null);
  let buildTrends: any[] = $state([]);
  let moduleStats: any = $state(null);
  let activities = $state<any[]>([]);
  let trendingMods: any[] = $state([]);
  let healthData: any = $state(null);
  let showHealthDetail = $state(false);
  let healthDetail: any = $state(null);
  let healthDetailLoading = $state(false);
  let isAdmin = $state(false);

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }

  async function loadWidgets() {
    const token = getToken();
    if (!token) return;
    try {
      const r = await fetch('/api/v1/dashboard/widgets', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const d = await r.json();
        widgets = d.widgets || [];
      }
    } catch {}
  }

  async function loadWidgetTypes() {
    try {
      const token = getToken();
      const headers: Record<string, string> = {};
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const r = await fetch('/api/v1/dashboard/widget-types', { headers });
      if (r.ok) {
        const d = await r.json();
        widgetTypes = d.types || [];
      }
    } catch {}
  }

  async function loadAll() {
    loading = true;
    const token = getToken();
    const headers: Record<string, string> = token ? { Authorization: `Bearer ${token}` } : {};
    try {
      const promises: Promise<any>[] = [
        fetch('/api/v1/analytics/module-stats', { headers }).then(r => r.json()),
        fetch('/api/v1/market/stats/trending', { headers }).then(r => r.json()),
      ];
      // Admin-only endpoints (under /admin prefix)
      if (isAdmin) {
        promises.unshift(
          fetch('/api/v1/admin/analytics/system', { headers }).then(r => r.json()),
          fetch('/api/v1/admin/analytics/build-stats', { headers }).then(r => r.json()),
          fetch('/api/v1/admin/analytics/build-trends?days=30', { headers }).then(r => r.json()),
          fetch('/api/v1/admin/health/detailed', { headers }).then(r => r.json()),
        );
      }
      const results = await Promise.allSettled(promises);
      let idx = 0;
      if (isAdmin) {
        const sys = results[idx];
        if (sys?.status === 'fulfilled') systemStats = sys.value;
        idx++;
        const bs = results[idx];
        if (bs?.status === 'fulfilled') buildStats = bs.value;
        idx++;
        const bt = results[idx];
        if (bt?.status === 'fulfilled') buildTrends = bt.value?.trends || [];
        idx++;
        const hd = results[idx];
        if (hd?.status === 'fulfilled') healthData = hd.value;
        idx++;
      }
      const ms = results[idx];
      if (ms?.status === 'fulfilled') moduleStats = ms.value;
      idx++;
      const tm = results[idx];
      if (tm?.status === 'fulfilled') trendingMods = tm.value?.modules?.slice(0, 5) || [];
    } catch {}
    loading = false;
  }

  async function loadHealthDetail() {
    healthDetailLoading = true;
    try {
      const token = getToken();
      const headers: Record<string, string> = token ? { Authorization: `Bearer ${token}` } : {};
      const r = await fetch('/api/v1/admin/health/detailed', { headers });
      if (r.ok) {
        healthDetail = await r.json();
        showHealthDetail = true;
      }
    } catch {}
    healthDetailLoading = false;
  }

  // Admin-only widget types
  const adminWidgetTypes = new Set(['system_overview', 'system_info', 'health_check', 'build_stats', 'build_trends']);
  // Visible widgets filtered by role
  let visibleWidgets = $derived(widgets.filter(w => w.is_visible && (isAdmin || !adminWidgetTypes.has(w.widget_type))));

  onMount(() => {
    void (async () => {
    // Check admin role first
    const token = getToken();
    if (token) {
      try {
        const r = await fetch('/api/v1/auth/profile', { headers: { Authorization: `Bearer ${token}` } });
        if (r.ok) { const d = await r.json(); isAdmin = d.is_admin || false; }
      } catch {}
    }
    await Promise.all([loadWidgets(), loadWidgetTypes(), loadAll()]);
    if (widgets.length === 0) {
      if (!token) return;
      const defaults = ['market_stats', 'recent_activity'];
      if (isAdmin) defaults.unshift('system_overview', 'build_stats', 'build_trends', 'system_info', 'health_check');
      for (let i = 0; i < defaults.length; i++) {
        try {
          await fetch('/api/v1/dashboard/widgets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
            body: JSON.stringify({ widget_type: defaults[i], title: defaults[i], position_x: 0, position_y: i, width: 2, height: 1 })
          });
        } catch {}
      }
      await loadWidgets();
    }
    if (!systemStats) await loadAll();
    })();

    return () => { if (autoRefreshTimer) clearInterval(autoRefreshTimer); };
  });

  // Single effect for auto-refresh management (replaces duplicate in onMount)
  $effect(() => {
    if (autoRefresh) {
      if (autoRefreshTimer) clearInterval(autoRefreshTimer);
      autoRefreshTimer = setInterval(loadAll, 30000);
    } else {
      if (autoRefreshTimer) { clearInterval(autoRefreshTimer); autoRefreshTimer = null; }
    }
    return () => { if (autoRefreshTimer) { clearInterval(autoRefreshTimer); autoRefreshTimer = null; } };
  });

  async function addWidget() {
    if (!selectedWidgetType) return;
    const token = getToken();
    if (!token) return;
    const maxY = Math.max(0, ...widgets.map(w => w.position_y + w.height));
    try {
      await fetch('/api/v1/dashboard/widgets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ widget_type: selectedWidgetType, title: selectedWidgetType, position_x: 0, position_y: maxY, width: 2, height: 1 })
      });
      await loadWidgets();
      showAddModal = false;
      selectedWidgetType = '';
    } catch {}
  }

  async function removeWidget(id: number) {
    const token = getToken();
    if (!token) return;
    try {
      await fetch(`/api/v1/dashboard/widgets/${id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      widgets = widgets.filter(w => w.id !== id);
    } catch {}
  }

  function handleDragStart(e: DragEvent, i: number) {
    dragIdx = i;
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
  }
  function handleDragOver(e: DragEvent, i: number) {
    e.preventDefault();
    dropIdx = i;
  }
  async function handleDrop() {
    if (dragIdx === null || dropIdx === null || dragIdx === dropIdx) { dragIdx = null; dropIdx = null; return; }
    const arr = [...widgets];
    const [removed] = arr.splice(dragIdx, 1);
    arr.splice(dropIdx, 0, removed);
    const reorder = arr.map((w, i) => ({ id: w.id, position_x: 0, position_y: i }));
    widgets = arr;
    dragIdx = null; dropIdx = null;
    const token = getToken();
    if (!token) return;
    try {
      await fetch('/api/v1/dashboard/widgets/reorder', {
        method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ items: reorder })
      });
    } catch {}
  }

  let maxTrend = $derived(buildTrends.length === 0 ? 1 : buildTrends.reduce((max, t) => Math.max(max, t.count || 0), 1));
  const gridCols = 'grid grid-cols-1 md:grid-cols-2 gap-4';
</script>

<div class="p-4 md:p-6 max-w-7xl mx-auto">
  <div class="flex items-center justify-between mb-8">
    <div>
      <h1 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">{$t('dashboard.title')}</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">实时监控系统运行状态</p>
    </div>
    <div class="flex items-center gap-2">
      <button class="btn-ghost flex items-center gap-2 text-sm" onclick={() => showAddModal = true}>
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-text-muted)">add</span>
        添加 Widget
      </button>
      <button class="btn-ghost flex items-center gap-2 text-sm" onclick={loadAll} disabled={loading}>
        <span class="material-symbols-outlined text-[18px] {loading ? 'animate-spin' : ''}" style="color: var(--color-text-muted)">refresh</span>
        {$t('dashboard.refresh')}
      </button>
      <button
        class="text-xs px-3 py-1.5 rounded-lg"
        style={autoRefresh ? 'background: var(--color-primary); color: white' : 'background: var(--color-surface); color: var(--color-text-muted)'}
        onclick={() => autoRefresh = !autoRefresh}
      >
        {autoRefresh ? '自动刷新 (30s)' : '自动刷新'}
      </button>
    </div>
  </div>

  {#if loading}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
      {#each Array(4) as _}
        <div class="card p-5"><div class="skeleton h-4 w-20 mb-2"></div><div class="skeleton h-8 w-16"></div></div>
      {/each}
    </div>
  {:else}
    <div class={gridCols}>
      {#each visibleWidgets as w, i}
        {#if w.is_visible}
          <div class="card p-5 relative group"
            style="{w.width > 1 ? 'grid-column: span ' + Math.min(w.width, 2) : ''}"
            draggable="true"
            ondragstart={(e) => handleDragStart(e, i)}
            ondragover={(e) => handleDragOver(e, i)}
            ondrop={handleDrop}
            class:opacity-50={dragIdx === i}
            class:ring-2={dropIdx === i}
            class:ring-primary-500={dropIdx === i}
          >
            <div class="flex items-center justify-between mb-4">
              <h3 class="text-sm font-semibold" style="color: var(--color-text)">
                {#if w.widget_type === 'system_overview'}系统概览
                {:else if w.widget_type === 'build_stats'}构建统计
                {:else if w.widget_type === 'build_trends'}构建趋势
                {:else if w.widget_type === 'market_stats'}市场统计
                {:else if w.widget_type === 'system_info'}系统信息
                {:else if w.widget_type === 'recent_activity'}最近活动
                {:else if w.widget_type === 'trending_modules'}热门趋势
                {:else if w.widget_type === 'health_check'}系统健康
                {:else}{w.title}{/if}
              </h3>
              <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => removeWidget(w.id)} title="删除">
                  <span class="material-symbols-outlined text-[14px]" style="color: var(--color-error)">close</span>
                </button>
              </div>
            </div>

            {#if w.widget_type === 'system_overview'}
              <div class="grid grid-cols-2 gap-4">
                {#each [
                  { icon: 'folder', label: $t('dashboard.projects'), value: systemStats?.projects ?? 0, color: 'from-violet-500 to-purple-600' },
                  { icon: 'group', label: $t('dashboard.users'), value: systemStats?.users ?? 0, color: 'from-cyan-500 to-blue-600' },
                  { icon: 'build', label: $t('dashboard.total_builds'), value: systemStats?.total_builds ?? 0, color: 'from-emerald-500 to-green-600' },
                  { icon: 'inventory_2', label: $t('dashboard.total_modules'), value: systemStats?.total_modules ?? 0, color: 'from-amber-500 to-orange-600' },
                ] as card}
                  <div>
                    <div class="flex items-center gap-2 mb-1">
                      <div class="w-8 h-8 rounded-lg flex items-center justify-center" style="background: var(--gradient-brand)">
                        <span class="material-symbols-outlined text-white text-[14px]">{card.icon}</span>
                      </div>
                    </div>
                    <p class="text-xs font-medium" style="color: var(--color-text-muted)">{card.label}</p>
                    <p class="text-xl font-bold tabular-nums" style="color: var(--color-text)">{card.value}</p>
                  </div>
                {/each}
              </div>

            {:else if w.widget_type === 'build_stats'}
              <div class="grid grid-cols-2 gap-4 mb-4">
                {#each [
                  { label: $t('dashboard.total_builds'), value: buildStats?.total_builds ?? 0 },
                  { label: $t('dashboard.successful'), value: buildStats?.successful_builds ?? 0, color: 'text-green-600' },
                  { label: $t('dashboard.failed'), value: buildStats?.failed_builds ?? 0, color: 'text-red-500' },
                  { label: $t('dashboard.avg_duration'), value: buildStats?.avg_duration_ms ? (buildStats.avg_duration_ms / 1000).toFixed(1) + 's' : '-' },
                ] as stat}
                  <div>
                    <p class="text-xs text-[var(--color-text-muted)] mb-1">{stat.label}</p>
                    <p class="text-lg font-bold {stat.color || 'text-[var(--color-text)]'} tabular-nums">{stat.value}</p>
                  </div>
                {/each}
              </div>
              {#if buildStats?.total_builds > 0}
                <div class="pt-3 border-t border-[var(--color-border)]">
                  <div class="flex items-center justify-between mb-1">
                    <span class="text-xs text-[var(--color-text-secondary)]">{$t('dashboard.success_rate')}</span>
                    <span class="text-xs font-semibold text-[var(--color-text)]">{(buildStats?.success_rate ?? 0).toFixed(1)}%</span>
                  </div>
                  <div class="w-full rounded-full h-2" style="background: var(--color-surface)">
                    <div class="rounded-full h-2 transition-all duration-700" style="width: {buildStats?.success_rate ?? 0}%; background: var(--gradient-brand)"></div>
                  </div>
                </div>
              {/if}

            {:else if w.widget_type === 'build_trends'}
              {#if buildTrends.length === 0}
                <p class="text-[var(--color-text-muted)] text-center py-8">{$t('dashboard.no_data')}</p>
              {:else}
                <div class="flex items-end gap-0.5 h-32">
                  {#each buildTrends as trend}
                    <div class="flex-1 flex flex-col items-center gap-0.5 group relative min-w-0">
                      <div class="absolute bottom-full mb-2 hidden group-hover:block bg-[var(--color-bg-elevated)] rounded-xl shadow-elevated px-2 py-1.5 text-[10px] whitespace-nowrap z-10 border border-[var(--color-border)]">
                        <div class="font-medium text-[var(--color-text)]">{trend.date}</div>
                        <div class="text-green-600">成功: {trend.success}</div>
                        <div class="text-red-500">失败: {trend.failed}</div>
                      </div>
                      <div class="w-full flex flex-col justify-end" style="height: 80px;">
                        <div class="w-full rounded-t-sm" style="height: {((trend.success || 0) / maxTrend) * 100}%; background: var(--color-primary)"></div>
                        <div class="w-full rounded-t-sm" style="height: {((trend.failed || 0) / maxTrend) * 100}%; background: var(--color-error)"></div>
                      </div>
                      <span class="text-[8px] text-[var(--color-text-muted)] truncate w-full text-center">{trend.date?.slice(5)}</span>
                    </div>
                  {/each}
                </div>
              {/if}

            {:else if w.widget_type === 'market_stats'}
              <div class="grid grid-cols-3 gap-2 mb-3">
                {#each [
                  { label: $t('dashboard.total_modules'), value: moduleStats?.total_modules ?? 0 },
                  { label: $t('dashboard.total_installs'), value: moduleStats?.total_installs ?? 0 },
                  { label: $t('dashboard.total_stars'), value: moduleStats?.total_stars ?? 0 },
                ] as s}
                  <div>
                    <p class="text-xs text-[var(--color-text-muted)] mb-1">{s.label}</p>
                    <p class="text-base font-bold text-[var(--color-text)] tabular-nums">{s.value}</p>
                  </div>
                {/each}
              </div>
              {#if moduleStats?.top_categories?.length > 0}
                <div class="pt-2 border-t border-[var(--color-border)]">
                  <p class="text-xs text-[var(--color-text-muted)] mb-2">{$t('dashboard.top_categories')}</p>
                  <div class="space-y-2">
                    {#each moduleStats.top_categories.slice(0, 4) as cat}
                      {@const maxC = Math.max(...moduleStats.top_categories.map((c: any) => c.count))}
                      <div class="flex items-center gap-2 text-xs">
                        <span class="w-16 truncate" style="color: var(--color-text-secondary)">{cat.category}</span>
                        <div class="flex-1 rounded-full h-1.5" style="background: var(--color-surface)">
                          <div class="rounded-full h-1.5" style="width: {(cat.count / maxC) * 100}%; background: var(--gradient-brand)"></div>
                        </div>
                        <span class="text-[var(--color-text-muted)] w-6 text-right">{cat.count}</span>
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}

            {:else if w.widget_type === 'system_info'}
              <div class="space-y-0">
                {#each [
                  { label: $t('dashboard.uptime'), value: systemStats?.uptime ?? '-' },
                  { label: $t('dashboard.db_size'), value: systemStats?.db_size ?? '-' },
                  { label: $t('dashboard.projects'), value: systemStats?.projects ?? 0 },
                  { label: $t('dashboard.users'), value: systemStats?.users ?? 0 },
                ] as item, i}
                  <div class="flex justify-between items-center py-2 {i < 3 ? 'border-b border-[var(--color-border)]' : ''}">
                    <span class="text-sm text-[var(--color-text-secondary)]">{item.label}</span>
                    <span class="text-sm font-medium text-[var(--color-text)]">{item.value}</span>
                  </div>
                {/each}
              </div>

            {:else if w.widget_type === 'recent_activity'}
              <ActivityFeed activities={activities.slice(0, 5)} />

            {:else if w.widget_type === 'trending_modules'}
              {#if trendingMods.length === 0}
                <p class="text-[var(--color-text-muted)] text-center py-8">暂无热门模块</p>
              {:else}
                <div class="space-y-2">
                  {#each trendingMods as mod, i}
                    <div class="flex items-center gap-3 py-1.5">
                      <span class="w-5 h-5 rounded-full flex items-center justify-center text-xs font-bold" style="background: {i < 3 ? 'var(--gradient-brand)' : 'var(--color-surface)'}; color: {i < 3 ? 'white' : 'var(--color-text-muted)'}">{i + 1}</span>
                      <div class="flex-1 min-w-0">
                        <p class="text-sm font-medium text-[var(--color-text)] truncate">{mod.title}</p>
                        <p class="text-xs text-[var(--color-text-muted)]">{mod.installs} 安装 · {mod.stars} 星</p>
                      </div>
                      <span class="badge text-[10px]" style="background: var(--color-surface); color: var(--color-text-muted)">{mod.category}</span>
                    </div>
                  {/each}
                </div>
              {/if}
            {:else if w.widget_type === 'health_check'}
              {#if healthData}
                <div class="flex items-center gap-2 mb-3">
                  <span class="w-2.5 h-2.5 rounded-full" style="background: {healthData.status === 'healthy' ? 'var(--color-success)' : healthData.status === 'warning' ? 'var(--color-warning)' : 'var(--color-error)'}"></span>
                  <span class="text-sm font-medium" style="color: {healthData.status === 'healthy' ? 'var(--color-success)' : healthData.status === 'warning' ? 'var(--color-warning)' : 'var(--color-error)'}">{healthData.status === 'healthy' ? '健康' : healthData.status === 'warning' ? '警告' : '异常'}</span>
                  <span class="text-xs ml-auto" style="color: var(--color-text-muted)">运行 {healthData.uptime}</span>
                </div>
                <div class="grid grid-cols-2 gap-2 mb-3">
                  {#each Object.entries(healthData.checks || {}) as [key, check]}
                    {@const checkStatus = (check as any).status}
                    <div class="p-2 rounded-lg cursor-pointer hover:opacity-80 transition-opacity" style="background: var(--color-surface);" onclick={() => loadHealthDetail()}>
                      <div class="flex items-center justify-between mb-0.5">
                        <span class="text-[10px] font-medium" style="color: var(--color-text-secondary)">{key}</span>
                        <span class="w-1.5 h-1.5 rounded-full" style="background: {checkStatus === 'ok' ? 'var(--color-success)' : checkStatus === 'warning' ? 'var(--color-warning)' : 'var(--color-error)'}"></span>
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
                <button class="text-xs w-full py-1.5 rounded-lg text-center hover:opacity-80 transition-opacity" style="background: var(--color-surface); color: var(--color-text-secondary)" onclick={() => loadHealthDetail()}>
                  查看详细信息
                </button>
              {:else}
                <p class="text-sm text-[var(--color-text-muted)]">加载中...</p>
              {/if}
            {:else}
              <p class="text-sm text-[var(--color-text-muted)]">自定义 widget</p>
            {/if}
          </div>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<!-- Add Widget Modal -->
{#if showAddModal}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => showAddModal = false}>
    <div class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">添加 Widget</h3>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => showAddModal = false}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-2 max-h-80 overflow-auto">
        {#each widgetTypes.filter(wt => !widgets.some(w => w.widget_type === wt.type) && (isAdmin || !adminWidgetTypes.has(wt.type))) as wt}
          <button
            class="w-full text-left p-3 rounded-xl border transition-all"
            style="border-color: {selectedWidgetType === wt.type ? 'var(--color-primary)' : 'var(--color-border)'}; background: {selectedWidgetType === wt.type ? 'var(--color-primary-light)' : 'var(--color-surface)'}"
            onclick={() => selectedWidgetType = wt.type}
          >
            <p class="text-sm font-medium text-[var(--color-text)]">{wt.name}</p>
            <p class="text-xs text-[var(--color-text-muted)] mt-0.5">{wt.desc}</p>
          </button>
        {:else}
          <p class="text-sm text-center py-4" style="color: var(--color-text-muted)">所有 Widget 类型都已添加</p>
        {/each}
      </div>
      <div class="p-5 border-t flex justify-end gap-3" style="border-color: var(--color-border)">
        <button class="btn-ghost text-sm" onclick={() => showAddModal = false}>取消</button>
        <button class="btn-primary text-sm" disabled={!selectedWidgetType} onclick={addWidget}>添加</button>
      </div>
    </div>
  </div>
{/if}

<!-- Health Detail Modal -->
{#if showHealthDetail}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => showHealthDetail = false}>
    <div class="rounded-2xl max-w-lg w-full border animate-[scaleIn_0.2s_ease-out] max-h-[80vh] overflow-hidden flex flex-col" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
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
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => showHealthDetail = false}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      
      <div class="p-5 overflow-auto flex-1">
        {#if healthDetailLoading}
          <div class="flex items-center justify-center py-8">
            <div class="animate-spin h-8 w-8 border-2 border-primary-500 border-t-transparent rounded-full"></div>
          </div>
        {:else if healthDetail}
          <!-- Overall Status -->
          <div class="flex items-center gap-3 mb-5 p-3 rounded-xl" style="background: {healthDetail.status === 'healthy' ? 'var(--color-success-light)' : healthDetail.status === 'warning' ? 'var(--color-warning-light)' : 'var(--color-error-light)'}">
            <span class="w-3 h-3 rounded-full" style="background: {healthDetail.status === 'healthy' ? 'var(--color-success)' : healthDetail.status === 'warning' ? 'var(--color-warning)' : 'var(--color-error)'}"></span>
            <span class="text-sm font-semibold" style="color: {healthDetail.status === 'healthy' ? 'var(--color-success)' : healthDetail.status === 'warning' ? 'var(--color-warning)' : 'var(--color-error)'}">
              {healthDetail.status === 'healthy' ? '系统健康' : healthDetail.status === 'warning' ? '存在警告' : '系统异常'}
            </span>
            <span class="text-xs ml-auto" style="color: var(--color-text-muted)">
              运行 {healthDetail.uptime} · 检查耗时 {healthDetail.check_ms}ms
            </span>
          </div>

          <!-- Check Details -->
          <div class="space-y-3">
            {#each Object.entries(healthDetail.checks || {}) as [key, check]}
              {@const checkStatus = (check as any).status}
              <div class="p-3 rounded-xl border" style="border-color: var(--color-border); background: var(--color-surface)">
                <div class="flex items-center justify-between mb-2">
                  <div class="flex items-center gap-2">
                    <span class="w-2 h-2 rounded-full" style="background: {checkStatus === 'ok' ? 'var(--color-success)' : checkStatus === 'warning' ? 'var(--color-warning)' : 'var(--color-error)'}"></span>
                    <span class="text-sm font-semibold text-[var(--color-text)]">{key}</span>
                  </div>
                  <span class="text-xs px-2 py-0.5 rounded-full" style="background: {checkStatus === 'ok' ? 'var(--color-success-light)' : checkStatus === 'warning' ? 'var(--color-warning-light)' : 'var(--color-error-light)'}; color: {checkStatus === 'ok' ? 'var(--color-success)' : checkStatus === 'warning' ? 'var(--color-warning)' : 'var(--color-error)'}">
                    {checkStatus}
                  </span>
                </div>
                <div class="grid grid-cols-2 gap-2 text-xs">
                  {#each Object.entries(check as any) as [prop, val]}
                    {#if prop !== 'status' && prop !== 'error'}
                      <div>
                        <span style="color: var(--color-text-muted)">{prop}:</span>
                        <span class="font-medium text-[var(--color-text)] ml-1">{typeof val === 'number' ? (prop.includes('mb') || prop.includes('gb') ? val.toFixed(1) : prop.includes('ms') ? Math.round(val) : val) : val}{typeof val === 'number' && prop.includes('mb') ? 'MB' : typeof val === 'number' && prop.includes('gb') ? 'GB' : typeof val === 'number' && prop.includes('ms') ? 'ms' : ''}</span>
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
        <button class="btn-primary text-sm" onclick={() => { healthDetail = null; loadHealthDetail(); }} disabled={healthDetailLoading}>
          {healthDetailLoading ? '刷新中...' : '刷新'}
        </button>
      </div>
    </div>
  </div>
{/if}
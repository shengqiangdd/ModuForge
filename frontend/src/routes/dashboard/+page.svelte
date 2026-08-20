<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n';
  import ActivityFeed from '$lib/components/ActivityFeed.svelte';
  import SystemOverviewWidget from './components/SystemOverviewWidget.svelte';
  import BuildStatsWidget from './components/BuildStatsWidget.svelte';
  import BuildTrendsWidget from './components/BuildTrendsWidget.svelte';
  import MarketStatsWidget from './components/MarketStatsWidget.svelte';
  import HealthCheckWidget from './components/HealthCheckWidget.svelte';
  import AddWidgetModal from './components/AddWidgetModal.svelte';
  import HealthDetailModal from './components/HealthDetailModal.svelte';
  import Skeleton from '$lib/components/ui/Skeleton.svelte';

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
  let dragIdx = $state<number | null>(null);
  let dropIdx = $state<number | null>(null);
  let autoRefresh = $state(true);
  let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;

  // Data stores
  let systemStats: any = $state(null);
  let buildStats: any = $state(null);
  let buildTrends: any[] = $state([]);
  let moduleStats: any = $state(null);
  let activities = $state<any[]>([]);
  let trendingMods: { id: string; title: string; slug: string; category: string; installs: number; stars: number }[] = $state([]);
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

  const adminWidgetTypes = new Set(['system_overview', 'system_info', 'health_check', 'build_stats', 'build_trends']);
  let visibleWidgets = $derived(widgets.filter(w => w.is_visible && (isAdmin || !adminWidgetTypes.has(w.widget_type))));
  let existingWidgetTypeSet = $derived(new Set(widgets.map(w => w.widget_type)));

  onMount(() => {
    void (async () => {
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

  $effect(() => {
    if (autoRefresh) {
      if (autoRefreshTimer) clearInterval(autoRefreshTimer);
      autoRefreshTimer = setInterval(loadAll, 30000);
    } else {
      if (autoRefreshTimer) { clearInterval(autoRefreshTimer); autoRefreshTimer = null; }
    }
    return () => { if (autoRefreshTimer) { clearInterval(autoRefreshTimer); autoRefreshTimer = null; } };
  });

  async function addWidget(type: string) {
    const token = getToken();
    if (!token) return;
    const maxY = Math.max(0, ...widgets.map(w => w.position_y + w.height));
    try {
      await fetch('/api/v1/dashboard/widgets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ widget_type: type, title: type, position_x: 0, position_y: maxY, width: 2, height: 1 })
      });
      await loadWidgets();
      showAddModal = false;
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

  const gridCols = 'grid grid-cols-1 md:grid-cols-2 gap-4';

  const widgetTitleMap: Record<string, string> = {
    system_overview: '系统概览', build_stats: '构建统计', build_trends: '构建趋势',
    market_stats: '市场统计', system_info: '系统信息', recent_activity: '最近活动',
    trending_modules: '热门趋势', health_check: '系统健康',
  };
</script>

<div class="w-full p-4 md:p-6 max-w-7xl mx-auto">
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
        <div class="card p-5"><Skeleton count={2} lines={[60, 40]} /></div>
      {/each}
    </div>
  {:else}
    <div class={gridCols}>
      {#each visibleWidgets as w, i (w.id)}
        {#if w.is_visible}
          <div
            role="presentation"
            class="card p-5 relative group"
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
                {widgetTitleMap[w.widget_type] ?? w.title}
              </h3>
              <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => removeWidget(w.id)} title="删除">
                  <span class="material-symbols-outlined text-[14px]" style="color: var(--color-error)">close</span>
                </button>
              </div>
            </div>

            {#if w.widget_type === 'system_overview'}
              <SystemOverviewWidget data={systemStats} loading={loading} t={$t} />
            {:else if w.widget_type === 'build_stats'}
              <BuildStatsWidget data={buildStats} loading={loading} />
            {:else if w.widget_type === 'build_trends'}
              <BuildTrendsWidget data={buildTrends} loading={loading} />
            {:else if w.widget_type === 'market_stats'}
              <MarketStatsWidget data={moduleStats} {loading} />
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
                  {#each trendingMods as mod, i (mod.slug)}
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
              <HealthCheckWidget data={healthData} {loading} onViewDetail={loadHealthDetail} />
            {:else}
              <p class="text-sm text-[var(--color-text-muted)]">自定义 widget</p>
            {/if}
          </div>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<AddWidgetModal
  open={showAddModal}
  {widgetTypes}
  existingWidgetTypes={existingWidgetTypeSet}
  onClose={() => showAddModal = false}
  onAdd={addWidget}
/>

<HealthDetailModal
  open={showHealthDetail}
  detail={healthDetail}
  loading={healthDetailLoading}
  onClose={() => showHealthDetail = false}
  onRefresh={loadHealthDetail}
/>

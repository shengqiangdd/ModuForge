<script lang="ts">
  import { onMount } from 'svelte';

  interface Activity {
    id: string;
    type: 'build' | 'deploy' | 'collab' | 'security' | 'project' | 'settings';
    description: string;
    actor: string;
    project_name: string;
    created_at: string;
  }

  let activities = $state<Activity[]>([]);
  let loading = $state(true);
  let loadingMore = $state(false);
  let errorMsg = $state('');
  let filterType = $state('');
  let offset = $state(0);
  const limit = 50;
  let hasMore = $state(true);

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }

  const typeIcon = (t: string) => {
    if (t === 'build') return 'build';
    if (t === 'deploy') return 'rocket_launch';
    if (t === 'collab') return 'group';
    if (t === 'security') return 'shield';
    if (t === 'project') return 'folder';
    if (t === 'settings') return 'settings';
    return 'article';
  };

  const typeColor = (t: string) => {
    if (t === 'build') return '#3b82f6';
    if (t === 'deploy') return '#22c55e';
    if (t === 'collab') return '#8b5cf6';
    if (t === 'security') return '#f97316';
    if (t === 'project') return '#06b6d4';
    if (t === 'settings') return '#6b7280';
    return '#6b7280';
  };

  const typeLabel = (t: string) => {
    const labels: Record<string, string> = {
      build: '构建', deploy: '部署', collab: '协作', security: '安全', project: '项目', settings: '设置'
    };
    return labels[t] || t;
  };

  async function loadActivities(reset = true) {
    if (reset) { loading = true; offset = 0; } else { loadingMore = true; }
    try {
      const params = new URLSearchParams({ limit: String(limit), format: 'json' });
      if (!reset) params.set('offset', String(offset));
      const r = await fetch(`/api/v1/activity?${params}`, { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) {
        const d = await r.json();
        const items = (d.activities || d || []).map((a: any) => ({
          id: a.id || String(Math.random()),
          type: a.type || 'build',
          description: a.description || a.action || '',
          actor: a.actor || a.user || '',
          project_name: a.project_name || a.project || '',
          created_at: a.created_at || new Date().toISOString(),
        }));
        activities = reset ? items : [...activities, ...items];
        hasMore = items.length >= limit;
        offset += items.length;
      }
    } catch {}
    loading = false;
    loadingMore = false;
  }

  const filteredActivities = $derived(
    activities.filter(a => !filterType || a.type === filterType)
  );

  const stats = $derived.by(() => {
    const s = { build: 0, deploy: 0, collab: 0, security: 0 };
    for (const a of activities) {
      if (a.type in s) s[a.type as keyof typeof s]++;
    }
    return s;
  });

  const filterOptions = [
    { val: '', label: '全部', icon: 'apps' },
    { val: 'build', label: '构建', icon: 'build' },
    { val: 'deploy', label: '部署', icon: 'rocket_launch' },
    { val: 'collab', label: '协作', icon: 'group' },
    { val: 'security', label: '安全', icon: 'shield' },
  ];

  onMount(() => loadActivities());
</script>

<div class="w-full p-4 md:p-6 max-w-5xl mx-auto space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold text-[var(--color-text)]">活动日志</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">查看系统中的所有操作记录</p>
    </div>
    <button class="px-3 py-1.5 rounded-lg text-sm flex items-center gap-1.5" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => loadActivities()}>
      <span class="material-symbols-outlined text-[16px]">refresh</span>
      刷新
    </button>
  </div>

  {#if errorMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-error-light); color: var(--color-error)">{errorMsg}</div>
  {/if}

  <!-- Quick Stats -->
  <div class="grid grid-cols-2 md:grid-cols-4 gap-2 md:gap-3">
    {#each [{ key: 'build', label: '构建', icon: 'build' }, { key: 'deploy', label: '部署', icon: 'rocket_launch' }, { key: 'collab', label: '协作', icon: 'group' }, { key: 'security', label: '安全', icon: 'shield' }] as s}
      <div class="card p-3 text-center">
        <span class="material-symbols-outlined text-[20px]" style="color: {typeColor(s.key)}">{s.icon}</span>
        <p class="text-lg font-bold text-[var(--color-text)]">{stats[s.key as keyof typeof stats]}</p>
        <p class="text-xs" style="color: var(--color-text-muted)">{s.label}</p>
      </div>
    {/each}
  </div>

  <!-- Filter Tabs -->
  <div class="flex gap-2 flex-wrap">
    {#each filterOptions as f}
      <button
        class="px-3 py-1.5 rounded-lg text-xs font-medium flex items-center gap-1 transition-colors"
        style="background: {filterType === f.val ? 'var(--color-primary)' : 'var(--color-surface)'}; color: {filterType === f.val ? 'white' : 'var(--color-text-secondary)'}; border: 1px solid {filterType === f.val ? 'var(--color-primary)' : 'var(--color-border)'}"
        onclick={() => filterType = f.val}
      >
        <span class="material-symbols-outlined text-[14px]">{f.icon}</span>
        {f.label}
      </button>
    {/each}
  </div>

  <!-- Activity Timeline -->
  {#if loading}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">加载中...</div>
  {:else if filteredActivities.length === 0}
    <div class="text-center py-12">
      <span class="material-symbols-outlined text-[48px]" style="color: var(--color-text-muted)">history</span>
      <p class="text-sm mt-3" style="color: var(--color-text-muted)">暂无活动记录</p>
    </div>
  {:else}
    <div class="relative">
      <!-- Timeline line -->
      <div class="absolute left-[11px] top-3 bottom-3 w-px" style="background: var(--color-border)"></div>
      <div class="space-y-1">
        {#each filteredActivities as a (a.id)}
          <div class="flex items-start gap-3 py-3 relative">
            <div class="w-6 h-6 rounded-full flex items-center justify-center flex-shrink-0 z-10" style="background: {typeColor(a.type)}22; border: 2px solid {typeColor(a.type)}">
              <span class="material-symbols-outlined text-[12px]" style="color: {typeColor(a.type)}">{typeIcon(a.type)}</span>
            </div>
            <div class="flex-1 min-w-0 card p-3">
              <p class="text-sm text-[var(--color-text)]">{a.description}</p>
              <div class="flex items-center gap-2 mt-1 flex-wrap">
                <span class="text-xs px-1.5 py-0.5 rounded-full" style="background: {typeColor(a.type)}18; color: {typeColor(a.type)}">{typeLabel(a.type)}</span>
                {#if a.actor}
                  <span class="text-xs" style="color: var(--color-text-muted)">{a.actor}</span>
                {/if}
                {#if a.project_name}
                  <span class="text-xs" style="color: var(--color-primary)">{a.project_name}</span>
                {/if}
                <span class="text-xs text-[var(--color-text-muted)]">{new Date(a.created_at).toLocaleString()}</span>
              </div>
            </div>
          </div>
        {/each}
      </div>
    </div>

    {#if hasMore && !loadingMore}
      <div class="text-center">
        <button class="px-4 py-2 rounded-lg text-sm" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => loadActivities(false)}>
          加载更多
        </button>
      </div>
    {/if}
    {#if loadingMore}
      <div class="text-center py-2 text-sm" style="color: var(--color-text-muted)">加载中...</div>
    {/if}
  {/if}
</div>

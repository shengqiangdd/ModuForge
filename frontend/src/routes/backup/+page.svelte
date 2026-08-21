<script lang="ts">
  import { onMount } from 'svelte';

  interface BackupSchedule {
    id: number;
    name: string;
    cron: string;
    project_ids: number[];
    retention_days: number;
    enabled: boolean;
    next_run?: string;
    last_run?: string;
    created_at: string;
  }

  interface BackupHistoryItem {
    id: number;
    schedule_id: number;
    schedule_name: string;
    status: 'success' | 'failed' | 'running';
    size_bytes?: number;
    started_at: string;
    finished_at?: string;
  }

  let schedules = $state<BackupSchedule[]>([]);
  let history = $state<BackupHistoryItem[]>([]);
  let loading = $state(true);
  let errorMsg = $state('');
  let successMsg = $state('');
  let showForm = $state(false);
  let formName = $state('');
  let formCron = $state('');
  let formProjectIds = $state('');
  let formRetention = $state('30');
  let saving = $state(false);
  let runningId = $state<number | null>(null);
  let activeTab = $state<'schedules' | 'history'>('schedules');

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }

  function msg(err: string, ok?: string) {
    errorMsg = err; successMsg = ok || '';
    setTimeout(() => { errorMsg = ''; successMsg = ''; }, 4000);
  }

  function formatBytes(bytes?: number) {
    if (!bytes || bytes === 0) return '-';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
  }

  async function loadSchedules() {
    loading = true;
    try {
      const r = await fetch('/api/v1/backup/schedules', { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { const d = await r.json(); schedules = d.schedules || d || []; }
    } catch {}
    loading = false;
  }

  async function loadHistory() {
    try {
      const r = await fetch('/api/v1/backup/history', { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { const d = await r.json(); history = d.history || d || []; }
    } catch {}
  }

  async function createSchedule() {
    if (!formName.trim() || !formCron.trim()) { msg('名称和 cron 表达式不能为空'); return; }
    saving = true;
    try {
      const projectIds = formProjectIds.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n));
      const r = await fetch('/api/v1/backup/schedules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({
          name: formName.trim(),
          cron: formCron.trim(),
          project_ids: projectIds,
          retention_days: parseInt(formRetention) || 30,
        }),
      });
      if (r.ok) {
        msg('', '备份计划已创建');
        showForm = false;
        formName = ''; formCron = ''; formProjectIds = ''; formRetention = '30';
        loadSchedules();
      } else {
        const d = await r.json().catch(() => ({}));
        msg(d.error || '创建失败');
      }
    } catch (e: any) { msg(e?.message || '创建失败'); }
    saving = false;
  }

  async function deleteSchedule(s: BackupSchedule) {
    if (!confirm(`确定删除备份计划 "${s.name}"？`)) return;
    try {
      const r = await fetch(`/api/v1/backup/schedules/${s.id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { msg('', '计划已删除'); loadSchedules(); }
    } catch (e: any) { msg(e?.message || '删除失败'); }
  }

  async function runSchedule(id: number) {
    runningId = id;
    try {
      const r = await fetch(`/api/v1/backup/schedules/${id}/run`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      if (r.ok) { msg('', '备份任务已启动'); setTimeout(loadHistory, 1000); }
      else { const d = await r.json().catch(() => ({})); msg(d.error || '执行失败'); }
    } catch (e: any) { msg(e?.message || '执行失败'); }
    runningId = null;
  }

  const statusColor = (s: string) => {
    if (s === 'success') return '#22c55e';
    if (s === 'failed') return '#ef4444';
    return '#f97316';
  };

  onMount(() => { loadSchedules(); loadHistory(); });
</script>

<div class="w-full p-6 max-w-5xl mx-auto space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold text-[var(--color-text)]">备份管理</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">自动备份计划与历史记录</p>
    </div>
    <div class="flex gap-2">
      <button class="px-3 py-1.5 rounded-lg text-sm flex items-center gap-1.5" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => { loadSchedules(); loadHistory(); }}>
        <span class="material-symbols-outlined text-[16px]">refresh</span>
        刷新
      </button>
      <button class="px-3 py-1.5 rounded-lg text-sm font-medium" style="background: var(--color-primary); color: white" onclick={() => showForm = !showForm}>
        <span class="material-symbols-outlined text-[16px] align-middle">add</span>
        新建计划
      </button>
    </div>
  </div>

  {#if errorMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-error-light); color: var(--color-error)">{errorMsg}</div>
  {/if}
  {#if successMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-success-light); color: var(--color-success)">{successMsg}</div>
  {/if}

  <!-- Create Form -->
  {#if showForm}
    <div class="card p-5 space-y-3">
      <h3 class="text-sm font-semibold text-[var(--color-text)]">新建备份计划</h3>
      <input class="input-field w-full" placeholder="计划名称" bind:value={formName} />
      <input class="input-field w-full" placeholder="Cron 表达式 (如 0 2 * * *)" bind:value={formCron} />
      <input class="input-field w-full" placeholder="项目 ID（逗号分隔，可留空备份全部）" bind:value={formProjectIds} />
      <input class="input-field w-full" placeholder="保留天数" type="number" bind:value={formRetention} />
      <div class="flex gap-2 justify-end">
        <button class="px-3 py-1.5 rounded-lg text-sm" style="background: var(--color-surface); color: var(--color-text-secondary)" onclick={() => showForm = false}>取消</button>
        <button class="px-3 py-1.5 rounded-lg text-sm font-medium" style="background: var(--color-primary); color: white" disabled={saving} onclick={createSchedule}>
          {saving ? '创建中...' : '创建'}
        </button>
      </div>
    </div>
  {/if}

  <!-- Tabs -->
  <div class="flex gap-1 p-1 rounded-lg" style="background: var(--color-surface)">
    <button
      class="flex-1 px-3 py-1.5 rounded-md text-sm transition-colors"
      style={activeTab === 'schedules' ? 'background: var(--color-bg-elevated); color: var(--color-text); box-shadow: 0 1px 3px rgba(0,0,0,0.1)' : 'color: var(--color-text-muted)'}
      onclick={() => activeTab = 'schedules'}
    >备份计划</button>
    <button
      class="flex-1 px-3 py-1.5 rounded-md text-sm transition-colors"
      style={activeTab === 'history' ? 'background: var(--color-bg-elevated); color: var(--color-text); box-shadow: 0 1px 3px rgba(0,0,0,0.1)' : 'color: var(--color-text-muted)'}
      onclick={() => activeTab = 'history'}
    >备份历史</button>
  </div>

  {#if loading}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">加载中...</div>
  {:else if activeTab === 'schedules'}
    {#if schedules.length === 0}
      <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">暂无备份计划</div>
    {:else}
      <div class="space-y-3">
        {#each schedules as s (s.id)}
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="w-2 h-2 rounded-full flex-shrink-0" style="background: {s.enabled ? '#22c55e' : '#9ca3af'}"></div>
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-[var(--color-text)]">{s.name}</span>
                  {#if !s.enabled}
                    <span class="text-xs px-1.5 py-0.5 rounded" style="background: var(--color-surface); color: var(--color-text-muted)">已禁用</span>
                  {/if}
                </div>
                <div class="text-xs text-[var(--color-text-muted)] mt-0.5 space-x-3">
                  <span>Cron: <code class="font-mono">{s.cron}</code></span>
                  <span>保留 {s.retention_days} 天</span>
                  {#if s.next_run}
                    <span>下次: {new Date(s.next_run).toLocaleString()}</span>
                  {/if}
                </div>
              </div>
              <div class="flex gap-1 flex-shrink-0">
                <button
                  class="text-xs px-2 py-1 rounded"
                  style="background: var(--color-primary-light); color: var(--color-primary)"
                  disabled={runningId === s.id}
                  onclick={() => runSchedule(s.id)}
                >
                  {runningId === s.id ? '执行中...' : '立即执行'}
                </button>
                <button
                  class="text-xs px-2 py-1 rounded"
                  style="background: var(--color-error-light); color: var(--color-error)"
                  onclick={() => deleteSchedule(s)}
                >删除</button>
              </div>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  {:else}
    {#if history.length === 0}
      <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">暂无备份记录</div>
    {:else}
      <div class="space-y-2">
        {#each history as h (h.id)}
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="w-2 h-2 rounded-full flex-shrink-0" style="background: {statusColor(h.status)}"></div>
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-[var(--color-text)]">{h.schedule_name}</span>
                  <span class="text-xs px-1.5 py-0.5 rounded" style="background: {statusColor(h.status)}22; color: {statusColor(h.status)}">
                    {h.status === 'success' ? '成功' : h.status === 'failed' ? '失败' : '执行中'}
                  </span>
                </div>
                <div class="text-xs text-[var(--color-text-muted)] mt-0.5 space-x-3">
                  <span>{new Date(h.started_at).toLocaleString()}</span>
                  {#if h.finished_at}
                    <span>耗时 {Math.round((new Date(h.finished_at).getTime() - new Date(h.started_at).getTime()) / 1000)}s</span>
                  {/if}
                  {#if h.size_bytes}
                    <span>{formatBytes(h.size_bytes)}</span>
                  {/if}
                </div>
              </div>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .input-field {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 6px 12px;
    color: var(--color-text);
    font-size: 14px;
    outline: none;
  }
  .input-field:focus { border-color: var(--color-primary); }
</style>

<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';

  let { projectId = '', onNavigate }: { projectId?: string; onNavigate?: (route: string, projectId?: string) => void } = $props();

  interface ArchTarget {
    value: string;
    label: string;
    icon: string;
    desc: string;
  }

  let targets: ArchTarget[] = [
    { value: 'arm64', label: 'arm64', icon: 'smartphone', desc: '64-bit ARM (现代设备)' },
    { value: 'arm', label: 'arm', icon: 'phone_android', desc: '32-bit ARM (旧设备)' },
    { value: 'x86_64', label: 'x86_64', icon: 'computer', desc: 'x86_64 (模拟器)' },
  ];

  let selectedTarget = $state('arm64');
  let taskId = $state<string | null>(null);
  let status = $state<string>('');
  let logLines = $state<string[]>([]);
  let building = $state(false);
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null);
  let project = $state<any>(null);
  let triggerMode = $state<'manual' | 'git' | 'schedule'>('manual');
  const triggerModes: ('manual' | 'git' | 'schedule')[] = ['manual', 'git', 'schedule'];
  let buildHistory = $state<any[]>([]);
  let buildCached = $state(false);
  let gitConfig = $state({ url: '', branch: 'main' });
  let scheduleConfig = $state({ cron: '0 2 * * *', target: 'universal', arch: 'arm64' });
  let buildSchedules = $state<any[]>([]);
  let savingGitConfig = $state(false);
  let savingSchedule = $state(false);

  // Feature 1: Incremental compilation
  let incrementalInfo = $state<{ needs_rebuild: boolean; changed_files: string[]; new_files: string[]; removed_files: string[]; reason: string } | null>(null);

  // Feature 2: Build cache status
  let cacheStatus = $state<{ total_size: number; file_count: number; hit_rate: number; total_builds: number; cache_hits: number } | null>(null);

  const statusConfig: Record<string, { color: string; bg: string; icon: string }> = {
    pending: { color: 'text-[var(--color-warning)]', bg: 'bg-[var(--color-warning-light)]', icon: 'schedule' },
    running: { color: 'text-[var(--color-info)]', bg: 'bg-[var(--color-info-light)]', icon: 'sync' },
    success: { color: 'text-[var(--color-success)]', bg: 'bg-[var(--color-success-light)]', icon: 'check_circle' },
    failed: { color: 'text-[var(--color-error)]', bg: 'bg-[var(--color-error-light)]', icon: 'error' },
    cancelled: { color: 'text-[var(--color-text-muted)]', bg: 'bg-[var(--color-surface)]', icon: 'cancel' },
  };

  const triggerIcons: Record<string, string> = {
    manual: 'build',
    git: 'cloud_upload',
    webhook: 'webhook',
    push: 'cloud_upload',
    schedule: 'schedule',
  };

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  async function loadBuildHistory() {
    if (!projectId) return;
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/builds`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) buildHistory = await res.json();
    } catch {}
  }

  async function loadCacheStatus() {
    if (!projectId) return;
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/build/cache`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) cacheStatus = await res.json();
    } catch {}
  }

  async function loadBuildSchedules() {
    if (!projectId) return;
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/build-schedules`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        const data = await res.json();
        buildSchedules = data.schedules || [];
      }
    } catch {}
  }

  async function saveGitConfig() {
    if (!projectId) return;
    savingGitConfig = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ git_url: gitConfig.url, git_branch: gitConfig.branch, auto_build: !!gitConfig.url }),
      });
      if (res.ok) {
        toast('Git 配置已保存', 'success');
        project = { ...project, git_url: gitConfig.url, git_branch: gitConfig.branch, auto_build: !!gitConfig.url };
      }
    } catch { toast('保存失败', 'error'); }
    savingGitConfig = false;
  }

  async function saveSchedule() {
    if (!projectId) return;
    savingSchedule = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}/build-schedules`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ cron_expr: scheduleConfig.cron, target: scheduleConfig.target, arch: scheduleConfig.arch }),
      });
      if (res.ok) {
        toast('定时任务已创建', 'success');
        loadBuildSchedules();
      }
    } catch { toast('创建失败', 'error'); }
    savingSchedule = false;
  }

  async function toggleSchedule(id: string, active: boolean) {
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      await fetch(`/api/v1/projects/${projectId}/build-schedules/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ active }),
      });
      loadBuildSchedules();
    } catch {}
  }

  async function deleteSchedule(id: string) {
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      await fetch(`/api/v1/projects/${projectId}/build-schedules/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      buildSchedules = buildSchedules.filter(s => s.id !== id);
      toast('已删除', 'info');
    } catch {}
  }

  onMount(() => {
    void (async () => {
      if (!projectId) return;
      try {
        const res = await fetch(`/api/v1/projects/${projectId}`, {
          headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
        });
        if (res.ok) {
          project = await res.json();
          gitConfig.url = project.git_url || '';
          gitConfig.branch = project.git_branch || 'main';
        }
      } catch {}
      loadBuildHistory();
      loadCacheStatus();
      loadBuildSchedules();
    })();
    return () => { if (pollTimer) clearInterval(pollTimer); };
  });

  async function clearCache() {
    if (!projectId) return;
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/build-cache`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        toast('构建缓存已清除', 'info');
        cacheStatus = null;
        loadCacheStatus();
      } else {
        toast('清除缓存失败', 'error');
      }
    } catch { toast('清除缓存失败', 'error'); }
  }

  async function startBuild() {
    if (!projectId) return;
    building = true;
    buildCached = false;
    status = 'pending';
    logLines = [];
    taskId = null;
    incrementalInfo = null;
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/build`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}`,
        },
        body: JSON.stringify({ target: selectedTarget, trigger: triggerMode, arch: selectedTarget }),
      });
      if (!res.ok) throw new Error((await res.json()).error || '构建启动失败');
      const data = await res.json();
      if (data.cached) {
        buildCached = true;
        status = 'success';
        logLines = ['[CACHE] 缓存命中，使用缓存产物'];
        taskId = data.task?.id || '';
        building = false;
        toast('缓存命中！', 'success');
        loadCacheStatus();
        return;
      }
      const task = data;
      taskId = task.id;
      status = task.status;
      toast('构建任务已启动', 'info');
      pollStatus();
    } catch (e: any) {
      logLines = [`[ERROR] ${e.message}`];
      status = 'failed';
      building = false;
      toast(e.message, 'error');
    }
  }

  function pollStatus() {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = setInterval(async () => {
      if (!taskId) return;
      try {
        const res = await fetch(`/api/v1/builds/${taskId}`, {
          headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
        });
        if (!res.ok) { building = false; clearInterval(pollTimer!); return; }
        const task = await res.json();
        status = task.status;
        if (task.log) logLines = task.log.split('\n').filter((l: string) => l);
        if (status === 'success') {
          clearInterval(pollTimer!);
          pollTimer = null;
          building = false;
          toast('构建成功！', 'success');
          loadCacheStatus();
        }
        else if (status === 'failed') { clearInterval(pollTimer!); pollTimer = null; building = false; toast('构建失败', 'error'); }
      } catch { clearInterval(pollTimer!); pollTimer = null; building = false; }
    }, 1000);
  }

  async function cancelBuild() {
    if (!taskId) return;
    try {
      const res = await fetch(`/api/v1/builds/${taskId}/cancel`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        status = 'cancelled';
        toast('构建已取消', 'info');
      }
    } catch {}
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = null;
    building = false;
  }

  // Parse incremental info from build log
  $effect(() => {
    const logText = logLines.join('\n');
    if (logText.includes('Checking incremental build')) {
      const changedMatch = logText.match(/Changed: (\d+) file/);
      const newMatch = logText.match(/New: (\d+) file/);
      const removedMatch = logText.match(/Removed: (\d+) file/);
      const reasonMatch = logText.match(/📋 (.+)/);
      incrementalInfo = {
        needs_rebuild: !logText.includes('No changes detected'),
        changed_files: [],
        new_files: [],
        removed_files: [],
        reason: reasonMatch?.[1] || '',
      };
      if (changedMatch) incrementalInfo.changed_files = Array(parseInt(changedMatch[1])).fill('');
      if (newMatch) incrementalInfo.new_files = Array(parseInt(newMatch[1])).fill('');
      if (removedMatch) incrementalInfo.removed_files = Array(parseInt(removedMatch[1])).fill('');
    }
  });
</script>

<style>
  .build-log {
    box-shadow: 0 0 30px rgba(34, 197, 94, 0.1);
  }
  .log-line {
    display: flex;
    gap: 12px;
  }
  .line-number {
    user-select: none;
    flex-shrink: 0;
    width: 24px;
    text-align: right;
  }
  .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid;
    border-top-color: transparent;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>

<div class="p-4 sm:p-6 max-w-3xl mx-auto pb-28">
  {#if !projectId}
    <div class="text-center py-16 text-[var(--color-text-secondary)]">
      <span class="material-symbols-outlined text-5xl mb-3 text-neutral-300">build</span>
      <p>请先选择项目</p>
    </div>
  {:else}
    <!-- Back button -->
    <div class="mb-4">
      <button
        class="flex items-center gap-1.5 px-3 py-2 rounded-xl text-sm font-medium transition-colors cursor-pointer"
        style="color: var(--color-text-secondary); background: var(--color-surface); border: 1px solid var(--color-border)"
        onclick={() => onNavigate?.('editor', projectId)}
      >
        <span class="material-symbols-outlined text-[18px]">arrow_back</span>
        返回项目
      </button>
    </div>
    <div class="mb-6">
      <div class="flex items-center gap-3 mb-1">
        <h1 class="text-xl font-bold text-[var(--color-text)]">构建模块</h1>
        {#if project}
          <span class="badge-primary text-[10px]">{project.name}</span>
        {/if}
      </div>
      <p class="text-sm text-[var(--color-text-secondary)]">选择目标架构并启动构建</p>
    </div>

    <!-- Architecture Selection -->
    <div class="mb-6">
      <span class="text-sm font-medium text-[var(--color-text-secondary)] mb-3 block">目标架构</span>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {#each targets as t}
          <button
            class="relative p-4 rounded-2xl border-2 transition-all duration-200 text-left cursor-pointer
              {selectedTarget === t.value
                ? 'border-primary-500 shadow-glow'
                : 'border-[var(--color-border)] hover:border-[var(--color-text-muted)] hover:bg-[var(--color-surface)]'}"
            style={selectedTarget === t.value ? 'background: color-mix(in srgb, var(--color-primary) 15%, transparent)' : ''}
            onclick={() => selectedTarget = t.value}
            disabled={building}
          >
            {#if selectedTarget === t.value}
              <div class="absolute top-2 right-2 w-5 h-5 rounded-full flex items-center justify-center" style="background: var(--color-primary)">
                <span class="material-symbols-outlined text-white text-[12px]">check</span>
              </div>
            {/if}
            <div class="flex items-center gap-3 mb-2">
              <div class="w-9 h-9 rounded-xl bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center">
                <span class="material-symbols-outlined text-white text-lg">{t.icon}</span>
              </div>
              <span class="text-base font-semibold text-[var(--color-text)]">{t.label}</span>
            </div>
            <p class="text-xs text-[var(--color-text-muted)]">{t.desc}</p>
          </button>
        {/each}
      </div>
    </div>

    <!-- Trigger Mode -->
    <div class="mb-6">
      <span class="text-sm font-medium text-[var(--color-text-secondary)] mb-3 block">触发方式</span>
      <div class="grid grid-cols-3 gap-2">
        {#each triggerModes as mode (mode)}
            {@const icons = { manual: 'build', git: 'cloud_upload', schedule: 'schedule' }}
          <button
            class="py-2.5 px-2 rounded-xl text-sm font-medium transition-all duration-200 flex items-center justify-center gap-1.5 cursor-pointer"
            style={triggerMode === mode
              ? 'background: var(--color-primary); color: white'
              : 'border: 1px solid var(--color-border); color: var(--color-text-secondary); background: var(--color-bg-elevated)'}
            onclick={() => triggerMode = mode}
            disabled={building}
          >
            <span class="material-symbols-outlined text-[18px]">{icons[mode]}</span>
            <span>{mode === 'manual' ? '手动' : mode === 'git' ? '推送' : '定时'}</span>
          </button>
        {/each}
      </div>
    </div>

    <!-- Git Push Config Panel -->
    {#if triggerMode === 'git'}
      <div class="mb-6 p-4 rounded-2xl border" style="border-color: var(--color-border); background: var(--color-bg-elevated)">
        <div class="flex items-center gap-2 mb-3">
          <span class="material-symbols-outlined text-[18px] text-[var(--color-info)]">cloud_upload</span>
          <span class="text-sm font-semibold text-[var(--color-text)]">推送触发配置</span>
        </div>
        <p class="text-xs text-[var(--color-text-muted)] mb-3">配置 Git 仓库地址和分支，Webhook 收到 push 事件时自动触发构建。</p>
        <div class="space-y-3">
          <div>
            <label for="git-url" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">仓库地址</label>
            <input id="git-url" type="text" class="input-field w-full text-sm" placeholder="https://github.com/user/repo.git" bind:value={gitConfig.url} />
          </div>
          <div>
            <label for="git-branch" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">监听分支</label>
            <input id="git-branch" type="text" class="input-field w-full text-sm" placeholder="main" bind:value={gitConfig.branch} />
          </div>
          <div class="flex items-center justify-between">
            <span class="text-xs text-[var(--color-text-muted)]">Webhook URL: <code class="text-[var(--color-primary)]">POST /api/v1/webhook/git</code></span>
            <button class="btn-primary text-xs py-1.5 px-3" onclick={saveGitConfig} disabled={savingGitConfig || !gitConfig.url}>
              {savingGitConfig ? '保存中...' : '保存配置'}
            </button>
          </div>
        </div>
      </div>
    {/if}

    <!-- Schedule Config Panel -->
    {#if triggerMode === 'schedule'}
      <div class="mb-6 p-4 rounded-2xl border" style="border-color: var(--color-border); background: var(--color-bg-elevated)">
        <div class="flex items-center gap-2 mb-3">
          <span class="material-symbols-outlined text-[18px] text-[var(--color-info)]">schedule</span>
          <span class="text-sm font-semibold text-[var(--color-text)]">定时构建配置</span>
        </div>
        <div class="space-y-3">
          <div>
            <label for="schedule-cron" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">Cron 表达式</label>
            <input id="schedule-cron" type="text" class="input-field w-full text-sm" placeholder="0 2 * * *" bind:value={scheduleConfig.cron} />
            <p class="text-[10px] text-[var(--color-text-muted)] mt-1">分 时 日 月 周 | 例: 每天凌晨2点 → 0 2 * * *</p>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="schedule-target" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">目标平台</label>
              <select id="schedule-target" class="input-field w-full text-sm" bind:value={scheduleConfig.target}>
                <option value="universal">通用</option>
              </select>
            </div>
            <div>
              <label for="schedule-arch" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">架构</label>
              <select id="schedule-arch" class="input-field w-full text-sm" bind:value={scheduleConfig.arch}>
                <option value="arm64">arm64</option>
                <option value="arm">arm</option>
                <option value="x86_64">x86_64</option>
              </select>
            </div>
          </div>
          <button class="btn-primary text-xs py-1.5 px-3 w-full" onclick={saveSchedule} disabled={savingSchedule || !scheduleConfig.cron}>
            {savingSchedule ? '创建中...' : '添加定时任务'}
          </button>
        </div>

        {#if buildSchedules.length > 0}
          <div class="mt-3 space-y-2">
            {#each buildSchedules as s}
              <div class="flex items-center justify-between p-2 rounded-lg" style="background: var(--color-surface)">
                <div class="flex items-center gap-2 text-xs">
                  <span class="material-symbols-outlined text-[14px] {s.is_active ? 'text-[var(--color-success)]' : 'text-[var(--color-text-muted)]'}">
                    {s.is_active ? 'check_circle' : 'pause_circle'}
                  </span>
                  <code class="text-[var(--color-text)]">{s.cron_expr}</code>
                  <span class="text-[var(--color-text-muted)]">{s.target}/{s.arch}</span>
                </div>
                <div class="flex items-center gap-1">
                  <button class="p-1 rounded hover:bg-[var(--color-border)]" onclick={() => toggleSchedule(s.id, !s.is_active)} title={s.is_active ? '暂停' : '启用'}>
                    <span class="material-symbols-outlined text-[14px]">{s.is_active ? 'pause' : 'play_arrow'}</span>
                  </button>
                  <button class="p-1 rounded hover:bg-[var(--color-error-light)]" onclick={() => deleteSchedule(s.id)} title="删除">
                    <span class="material-symbols-outlined text-[14px] text-[var(--color-error)]">delete</span>
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    <!-- Build Button -->
    <div class="flex gap-3 mb-6">
      <button
        class="flex-1 py-3 rounded-xl font-semibold text-sm text-white transition-all duration-200 disabled:opacity-50
          bg-gradient-to-r from-primary-600 to-primary-700 hover:from-primary-700 hover:to-primary-800 active:scale-[0.98] shadow-sm hover:shadow-glow flex items-center justify-center gap-2"
        onclick={startBuild}
        disabled={building}
      >
        {#if building}
          <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/></svg>
          构建中...
        {:else}
          <span class="material-symbols-outlined text-[18px]">build</span>
          开始构建
        {/if}
      </button>
      {#if building}
        <button
          class="px-5 py-3 rounded-xl text-sm font-medium transition-colors"
          style="border: 1px solid var(--color-error); color: var(--color-error); background: transparent"
          onclick={cancelBuild}
        >
          取消
        </button>
      {:else}
        <button
          class="px-4 py-3 rounded-xl text-sm font-medium transition-colors flex items-center gap-1.5"
          style="border: 1px solid var(--color-border); color: var(--color-text-secondary); background: transparent"
          onclick={clearCache}
          title="清除构建缓存"
        >
          <span class="material-symbols-outlined text-[16px]">cleaning_services</span>
          清除缓存
        </button>
      {/if}
    </div>

    <!-- Cache Status -->
    {#if cacheStatus && cacheStatus.file_count > 0}
      <div class="mb-4 p-3 rounded-2xl border flex items-center gap-4" style="border-color: var(--color-border); background: var(--color-bg-elevated)">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-[18px] text-[var(--color-text-muted)]">database</span>
          <span class="text-xs font-medium text-[var(--color-text-secondary)]">构建缓存</span>
        </div>
        <div class="flex items-center gap-3 text-xs text-[var(--color-text-muted)]">
          <span>{cacheStatus.file_count} 个文件</span>
          <span>{formatBytes(cacheStatus.total_size)}</span>
          <span>命中率 {cacheStatus.hit_rate.toFixed(0)}%</span>
        </div>
      </div>
    {/if}

    <!-- Status -->
    {#if status}
      {@const cfg = statusConfig[status] || statusConfig.pending}
      <div class="mb-4 p-4 rounded-2xl border {cfg.bg} flex flex-wrap items-center gap-2 sm:gap-3 overflow-hidden" style="border-color: var(--color-border)">
        <span class="material-symbols-outlined text-[22px] {cfg.color}">{cfg.icon}</span>
        <span class="text-sm font-semibold {cfg.color} uppercase">{status}</span>
        <span class="text-xs px-2 py-0.5 rounded-full flex items-center gap-1 whitespace-nowrap" style="background: var(--color-bg-elevated); color: var(--color-text-muted)">
          <span class="material-symbols-outlined text-[14px]">{triggerIcons[triggerMode] || 'build'}</span>
          {triggerMode === 'manual' ? '手动' : triggerMode === 'git' ? 'Git' : triggerMode === 'schedule' ? '定时' : triggerMode}
        </span>
        <span class="text-xs px-2 py-0.5 rounded-full flex items-center gap-1 whitespace-nowrap" style="background: var(--color-bg-elevated); color: var(--color-text-muted)">
          <span class="material-symbols-outlined text-[14px]">memory</span>
          {selectedTarget}
        </span>
        {#if status === 'running'}
          <div class="ml-auto flex gap-1">
            <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_infinite]"></div>
            <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_0.3s_infinite]"></div>
            <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_0.6s_infinite]"></div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- Incremental Build Info -->
    {#if incrementalInfo}
      <div class="mb-4 p-3 rounded-2xl border flex items-start gap-3 {incrementalInfo.needs_rebuild ? 'bg-amber-500/10' : 'bg-green-500/10'}"
           style="border-color: {incrementalInfo.needs_rebuild ? 'rgba(245,158,11,0.3)' : 'rgba(34,197,94,0.3)'}">
        <span class="material-symbols-outlined text-[20px] mt-0.5 {incrementalInfo.needs_rebuild ? 'text-amber-500' : 'text-green-500'}">
          {incrementalInfo.needs_rebuild ? 'difference' : 'check_circle'}
        </span>
        <div class="flex-1">
          <span class="text-sm font-semibold {incrementalInfo.needs_rebuild ? 'text-amber-500' : 'text-green-500'}">
            {incrementalInfo.needs_rebuild ? '增量编译' : '无变化'}
          </span>
          {#if incrementalInfo.needs_rebuild}
            <p class="text-xs text-[var(--color-text-muted)] mt-1">{incrementalInfo.reason}</p>
            <div class="flex gap-3 mt-1.5 text-xs text-[var(--color-text-muted)]">
              {#if incrementalInfo.changed_files.length > 0}
                <span>📝 {incrementalInfo.changed_files.length} 个文件变更</span>
              {/if}
              {#if incrementalInfo.new_files.length > 0}
                <span>🆕 {incrementalInfo.new_files.length} 个新文件</span>
              {/if}
              {#if incrementalInfo.removed_files.length > 0}
                <span>🗑️ {incrementalInfo.removed_files.length} 个已删除</span>
              {/if}
            </div>
          {:else}
            <p class="text-xs text-[var(--color-text-muted)] mt-1">所有源文件未变化，使用缓存的二进制</p>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Log -->
    {#if logLines.length > 0}
      <div class="build-log rounded-2xl border overflow-hidden" style="border-color: rgba(34,197,94,0.2)">
        <div class="px-4 py-2.5 flex items-center gap-2" style="background: rgba(34,197,94,0.1); border-bottom: 1px solid rgba(34,197,94,0.2)">
          <span class="material-symbols-outlined text-[16px]" style="color: #4ade80">terminal</span>
          <span class="text-xs font-medium" style="color: #4ade80">构建日志</span>
          <div class="ml-auto flex items-center gap-2">
            <span class="text-[10px]" style="color: rgba(74,222,128,0.6)">{logLines.length} 行</span>
            <div class="flex gap-1">
              <div class="w-2.5 h-2.5 rounded-full" style="background: #ef4444"></div>
              <div class="w-2.5 h-2.5 rounded-full" style="background: #f59e0b"></div>
              <div class="w-2.5 h-2.5 rounded-full" style="background: #22c55e"></div>
            </div>
          </div>
        </div>
        <pre class="p-4 text-xs font-mono overflow-auto max-h-96 whitespace-pre-wrap leading-relaxed" style="background: #0a0a0a; color: #4ade80">
          {#each logLines as line, i}
            <div class="log-line">
              <span class="line-number" style="color: rgba(74,222,128,0.3)">{String(i + 1).padStart(3, ' ')}</span>
              <span class:text-red-400={line.startsWith('[ERROR]')} class:text-amber-400={line.startsWith('[WARN]')} class:text-green-300={line.startsWith('[SUCCESS]')} class:text-green-400={!line.startsWith('[') || line.startsWith('[INFO]')}>{line}</span>
            </div>
          {/each}
        </pre>
      </div>
    {:else if building}
      <div class="rounded-2xl border p-8 text-center" style="border-color: var(--color-border); background: var(--color-bg-elevated)">
        <div class="inline-flex items-center gap-2 mb-3">
          <div class="spinner" style="border-color: var(--color-border); border-top-color: var(--color-primary)"></div>
        </div>
        <p class="text-sm" style="color: var(--color-text-muted)">等待构建日志...</p>
      </div>
    {/if}

    <!-- Cache Hit -->
    {#if buildCached}
      <div class="mb-4 p-3 rounded-2xl border flex items-center gap-3" style="background: rgba(34,197,94,0.1); border-color: rgba(34,197,94,0.3)">
        <span class="material-symbols-outlined text-[20px] text-green-500">cached</span>
        <span class="text-sm font-semibold text-green-500">缓存命中</span>
        <span class="text-xs text-[var(--color-text-muted)]">使用缓存构建产物</span>
        <button class="ml-auto px-3 py-1 rounded-lg text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary)" onclick={clearCache}>
          清除缓存
        </button>
      </div>
    {/if}

    <!-- Download -->
    {#if status === 'success' && taskId}
      {@const dlToken = localStorage.getItem('moduforge_token') || ''}
      <a
        href="/api/v1/builds/{taskId}/download?token={encodeURIComponent(dlToken)}"
        class="mt-4 w-full py-3 rounded-xl font-semibold text-sm text-center no-underline
          bg-green-500 text-white hover:bg-green-600 transition-all duration-200 flex items-center justify-center gap-2"
        target="_blank"
      >
        <span class="material-symbols-outlined text-[18px]">download</span>
        下载构建产物
      </a>
    {/if}

    <!-- Build History -->
    {#if buildHistory.length > 0}
      <div class="mt-8">
        <div class="flex items-center justify-between mb-3">
          <h3 class="text-sm font-semibold text-[var(--color-text)] flex items-center gap-2">
            <span class="material-symbols-outlined text-[18px]">history</span>
            构建历史
          </h3>
          <button class="btn-ghost border text-xs px-3 py-1 rounded-lg" style="border-color: var(--color-border)" onclick={loadBuildHistory}>刷新</button>
        </div>
        <div class="space-y-2">
          {#each buildHistory as task}
            {@const cfg = statusConfig[task.status] || statusConfig.pending}
            <div
              role="button"
              tabindex="0"
              class="p-3 rounded-xl border cursor-pointer transition-colors"
              style="border-color: var(--color-border); background: var(--color-bg-elevated)"
              onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); (e.currentTarget as HTMLElement).click(); } }}
              onclick={() => {
                taskId = task.id;
                status = task.status;
                logLines = task.log ? task.log.split('\n') : [];
                if (task.status === 'running' || task.status === 'pending') {
                  building = true;
                  pollStatus();
                }
              }}
            >
              <div class="flex items-center justify-between flex-wrap gap-1">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="material-symbols-outlined text-[18px] {cfg.color}">{cfg.icon}</span>
                  <span class="text-xs text-[var(--color-text)]">#{task.id?.slice(0, 8) || ''}</span>
                  <span class="text-xs px-2 py-0.5 rounded-full flex items-center gap-1 whitespace-nowrap" style="background: var(--color-surface); color: var(--color-text-muted)">
                    <span class="material-symbols-outlined text-[12px]">{triggerIcons[task.trigger] || 'build'}</span>
                    {task.trigger === 'manual' ? '手动' : task.trigger === 'git' ? 'Git' : task.trigger === 'schedule' ? '定时' : task.trigger || '手动'}
                  </span>
                </div>
                <div class="flex items-center gap-2 flex-wrap">
                  {#if task.commit_hash}
                    <span class="text-xs font-mono" style="color: var(--color-text-muted)">{task.commit_hash.slice(0, 7)}</span>
                  {/if}
                  <span class="text-xs" style="color: var(--color-text-muted)">{new Date(task.created_at).toLocaleString('zh-CN')}</span>
                  {#if task.status === 'running' || task.status === 'pending'}
                    <button
                      class="p-1 rounded hover:bg-[var(--color-surface)]"
                      onclick={(e) => { e.stopPropagation(); taskId = task.id; cancelBuild(); }}
                    >
                      <span class="material-symbols-outlined text-[16px] text-[var(--color-error)]">cancel</span>
                    </button>
                  {/if}
                </div>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

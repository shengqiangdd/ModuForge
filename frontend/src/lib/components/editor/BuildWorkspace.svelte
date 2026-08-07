<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import BuildConfig from './BuildConfig.svelte';
  import BuildOutput from './BuildOutput.svelte';
  import BuildHistory from './BuildHistory.svelte';

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

  async function deleteBuild(buildId: string, e: Event) {
    e.stopPropagation();
    if (!confirm('确定删除此构建记录？')) return;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}/builds/${buildId}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) {
        buildHistory = buildHistory.filter(b => b.id !== buildId);
        toast('已删除', 'info');
      }
    } catch {}
  }

  async function deleteFailedBuilds() {
    const failedCount = buildHistory.filter(b => b.status === 'failed').length;
    if (failedCount === 0) return;
    if (!confirm(`确定删除全部 ${failedCount} 条失败记录？`)) return;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}/builds/failed`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) {
        buildHistory = buildHistory.filter(b => b.status !== 'failed');
        toast(`已删除 ${failedCount} 条失败记录`, 'info');
      }
    } catch {}
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
          <span class="px-2 py-0.5 rounded-md text-[10px] font-medium" style="background: var(--color-primary-light); color: var(--color-primary)">{project.name}</span>
        {/if}
      </div>
      <p class="text-sm text-[var(--color-text-secondary)]">选择目标架构并启动构建</p>
    </div>

    <BuildConfig
      {projectId}
      bind:selectedTarget
      bind:triggerMode
      {building} {status} {project} {buildCached}
      {savingGitConfig} {savingSchedule}
      {gitConfig} {scheduleConfig} {buildSchedules}
      onStartBuild={startBuild}
      onCancelBuild={cancelBuild}
      onClearCache={clearCache}
      onSaveGitConfig={saveGitConfig}
      onSaveSchedule={saveSchedule}
      onToggleSchedule={toggleSchedule}
      onDeleteSchedule={deleteSchedule}
      onSelectTarget={(t) => selectedTarget = t}
      onSelectTriggerMode={(m) => triggerMode = m}
      onGitConfigChange={(cfg) => gitConfig = cfg}
      onScheduleConfigChange={(cfg) => scheduleConfig = cfg}
    />

    <BuildOutput
      {status} {logLines} {building} {buildCached}
      {triggerMode} {selectedTarget} {taskId}
      {incrementalInfo} {cacheStatus}
    />

    <BuildHistory
      {buildHistory}
      onSelectBuild={(task) => {
        if (task._cancel) { taskId = task.id; cancelBuild(); return; }
        taskId = task.id;
        status = task.status || '';
        logLines = task.log ? task.log.split('\n') : [];
        if (task.status === 'running' || task.status === 'pending') { building = true; pollStatus(); }
      }}
      onDeleteBuild={deleteBuild}
      onDeleteFailedBuilds={deleteFailedBuilds}
      onRefresh={loadBuildHistory}
    />
  {/if}
</div>

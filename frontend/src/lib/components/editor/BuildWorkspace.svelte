<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import BuildConfig from './BuildConfig.svelte';
  import BuildOutput from './BuildOutput.svelte';
  import BuildHistory from './BuildHistory.svelte';

  let { projectId = '', onNavigate }: { projectId?: string; onNavigate?: (route: string, projectId?: string) => void } = $props();

  // Persist active build across page navigation
  const BUILD_STATE_KEY = 'moduforge_active_build';
  function saveBuildState() {
    if (taskId && building) {
      try { localStorage.setItem(BUILD_STATE_KEY, JSON.stringify({ taskId, projectId, status, startTime: Date.now() })); } catch {}
    } else {
      try { localStorage.removeItem(BUILD_STATE_KEY); } catch {}
    }
  }
  function loadBuildState(): { taskId: string; projectId: string } | null {
    try {
      const raw = localStorage.getItem(BUILD_STATE_KEY);
      if (!raw) return null;
      const s = JSON.parse(raw);
      // Expire after 30 minutes
      if (Date.now() - (s.startTime || 0) > 30 * 60 * 1000) { localStorage.removeItem(BUILD_STATE_KEY); return null; }
      return s;
    } catch { return null; }
  }

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
  let gitConfig = $state<{ url: string; branch: string; commitMsg: string; author: string; excludePatterns: string; includePatterns: string; token: string }>({ url: '', branch: 'main', commitMsg: '', author: '', excludePatterns: '', includePatterns: '', token: '' });
  let scheduleConfig = $state({ cron: '0 2 * * *', target: 'universal', arch: 'arm64' });
  let buildSchedules = $state<any[]>([]);
  let savingGitConfig = $state(false);
  let savingSchedule = $state(false);

  // Feature 1: Incremental compilation — declared as $derived.by later in the
  // file (must NOT be an effect-written $state — that froze the render pipeline).

  // Feature 2: Build cache status
  let cacheStatus = $state<{ total_size: number; file_count: number; hit_rate: number; total_builds: number; cache_hits: number } | null>(null);

  async function loadBuildHistory() {
    if (!projectId) return;
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/builds`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) buildHistory = await res.json();
    } catch (e) { console.error('Failed to load build history:', e); }
  }

  async function loadCacheStatus() {
    if (!projectId) return;
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/build/cache`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) cacheStatus = await res.json();
    } catch (e) { console.error('Failed to load cache status:', e); }
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
    } catch (e) { console.error('Failed to load build schedules:', e); }
  }

  async function saveGitConfig() {
    if (!projectId) return;
    savingGitConfig = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ git_url: gitConfig.url, git_branch: gitConfig.branch, git_author: gitConfig.author, git_commit_msg: gitConfig.commitMsg, auto_build: !!gitConfig.url }),
      });
      if (res.ok) {
        toast('Git 配置已保存', 'success', 3000);
        project = { ...project, git_url: gitConfig.url, git_branch: gitConfig.branch, git_author: gitConfig.author, git_commit_msg: gitConfig.commitMsg, auto_build: !!gitConfig.url };
      }
    } catch { toast('保存失败', 'error', 5000); }
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
        toast('定时任务已创建', 'success', 3000);
        loadBuildSchedules();
      }
    } catch { toast('创建失败', 'error', 5000); }
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
    } catch (e) { console.error('Failed to toggle schedule:', e); }
  }

  async function deleteSchedule(id: string) {
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      await fetch(`/api/v1/projects/${projectId}/build-schedules/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      buildSchedules = buildSchedules.filter(s => s.id !== id);
      toast('已删除', 'info', 3000);
    } catch (e) { console.error('Failed to delete schedule:', e); }
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
          gitConfig.author = project.git_author || '';
          gitConfig.commitMsg = project.git_commit_msg || '';
        }
      } catch (e) { console.error('Failed to load project:', e); }
      loadBuildHistory();
      loadCacheStatus();
      loadBuildSchedules();

      // Resume build if we navigated away and came back
      const saved = loadBuildState();
      if (saved && saved.projectId === projectId && saved.taskId) {
        taskId = saved.taskId;
        building = true;
        status = 'running';
        toast('检测到进行中的构建，已恢复轮询', 'info', 4000);
        pollStatus();
      }
    })();
    return () => {
      if (pollTimer) clearInterval(pollTimer);
    };
  });

  async function clearCache() {
    if (!projectId) return;
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/build-cache`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        toast('构建缓存已清除', 'info', 3000);
        cacheStatus = null;
        loadCacheStatus();
      } else {
        toast('清除缓存失败', 'error', 5000);
      }
    } catch { toast('清除缓存失败', 'error', 5000); }
  }

  async function startBuild() {
    if (!projectId) return;
    building = true;
    buildCached = false;
    status = 'pending';
    logLines = [];
    taskId = null;
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
        toast('缓存命中！', 'success', 4000);
        loadCacheStatus();
        return;
      }
      const task = data;
      taskId = task.id;
      status = task.status;
      toast('构建任务已启动', 'info', 4000);
      saveBuildState();
      pollStatus();
    } catch (e: any) {
      logLines = [`[ERROR] ${e.message}`];
      status = 'failed';
      building = false;
      toast(e.message, 'error', 5000);
    }
  }

  function pollStatus() {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = setInterval(async () => {
      if (!taskId) return;
      let consecutiveErrors = 0;
      try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 10000);
        const res = await fetch(`/api/v1/builds/${taskId}`, {
          headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
          signal: controller.signal,
        });
        clearTimeout(timeoutId);
        if (!res.ok) {
          if (++consecutiveErrors >= 6) { building = false; saveBuildState(); clearInterval(pollTimer!); return; }
          return;
        }
        const task = await res.json();
        status = task.status;
        if (task.log) logLines = task.log.split('\n').filter((l: string) => l);
        if (status === 'success') {
          clearInterval(pollTimer!);
          pollTimer = null;
          building = false;
          saveBuildState();
          toast('构建成功！', 'success', 5000);
          loadCacheStatus();
          loadBuildHistory();
        }
        else if (status === 'failed') {
          clearInterval(pollTimer!);
          pollTimer = null;
          building = false;
          saveBuildState();
          toast('构建失败', 'error', 5000);
          loadBuildHistory();
        }
        else {
          // Still running — persist state for page navigation
          saveBuildState();
        }
      } catch {
        // Transient network/timeout errors must NOT kill the polling loop — only give up after many consecutive failures.
        if (++consecutiveErrors >= 6) { clearInterval(pollTimer!); pollTimer = null; building = false; saveBuildState(); }
      }
    }, 500);
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
        toast('构建已取消', 'info', 4000);
      }
    } catch (e) { console.error('Failed to cancel build:', e); }
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = null;
    building = false;
    saveBuildState();
    loadBuildHistory();
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
      // 404 = record already gone (e.g. removed by another tab / auto-cleanup) — treat as success
      if (res.ok || res.status === 404) {
        buildHistory = buildHistory.filter(b => b.id !== buildId);
        toast('已删除', 'info', 3000);
      } else {
        const body = await res.json().catch(() => ({ error: '未知错误' }));
        toast(`删除失败: ${body.error || res.statusText}`, 'error', 5000);
      }
    } catch (e: any) {
      toast(`删除失败: ${e.message || '网络错误'}`, 'error', 5000);
    }
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
        toast(`已删除 ${failedCount} 条失败记录`, 'success', 4000);
      } else {
        const body = await res.json().catch(() => ({ error: '未知错误' }));
        toast(`清除失败: ${body.error || res.statusText}`, 'error', 5000);
      }
    } catch (e: any) {
      toast(`清除失败: ${e.message || '网络错误'}`, 'error', 5000);
    }
  }

  async function publishRelease(buildId: string) {
    if (!projectId) return;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}/builds/${buildId}/release`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({}),
      });
      if (res.ok) {
        const data = await res.json();
        toast(`Release ${data.tag_name} 已发布！`, 'success', 5000);
        if (data.html_url) {
          window.open(data.html_url, '_blank');
        }
      } else {
        const body = await res.json().catch(() => ({ error: '未知错误' }));
        toast(`发布失败: ${body.error || res.statusText}`, 'error', 5000);
      }
    } catch (e: any) {
      toast(`发布失败: ${e.message || '网络错误'}`, 'error', 5000);
    }
  }

  // Parse incremental info from build log.
  // MUST be a $derived, not an $effect that writes $state: the old effect
  // wrote `incrementalInfo` on the first poll tick and silently froze Svelte 5's
  // render pipeline (UI stuck at RUNNING while polling kept running) — confirmed
  // by DEBUG rounds (ticks kept counting in JS/title but the DOM never re-rendered).
  let incrementalInfo = $derived.by(() => {
    const logText = logLines.join('\n');
    if (!logText.includes('Checking incremental build')) return null;
    const changedMatch = logText.match(/Changed: (\d+) file/);
    const newMatch = logText.match(/New: (\d+) file/);
    const removedMatch = logText.match(/Removed: (\d+) file/);
    const reasonMatch = logText.match(/📋 (.+)/);
    return {
      needs_rebuild: !logText.includes('No changes detected'),
      changed_files: changedMatch ? Array(parseInt(changedMatch[1])).fill('') : [],
      new_files: newMatch ? Array(parseInt(newMatch[1])).fill('') : [],
      removed_files: removedMatch ? Array(parseInt(removedMatch[1])).fill('') : [],
      reason: reasonMatch?.[1] || '',
    };
  });
</script>

<style>
</style>

<div class="w-full p-4 sm:p-6 max-w-3xl mx-auto pb-28 overflow-x-hidden min-w-0">
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
        <h1 class="text-xl font-bold text-[var(--color-text)]">构建模块
          <span class="ml-2 px-1.5 py-0.5 rounded-md text-[10px] font-mono align-middle" style="background: var(--color-primary-light); color: var(--color-primary)" title="前端版本标识（用于确认浏览器已加载最新界面，v15 对应 2026-08-15 部署）">UI v15</span>
        </h1>
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
      onGitConfigChange={(cfg) => gitConfig = { url: cfg.url || '', branch: cfg.branch || 'main', commitMsg: cfg.commitMsg || '', author: cfg.author || '', excludePatterns: cfg.excludePatterns || '', includePatterns: cfg.includePatterns || '', token: cfg.token || '' }}
      onScheduleConfigChange={(cfg) => scheduleConfig = cfg}
    />

    <BuildOutput
      {status} {logLines} {building} {buildCached}
      {triggerMode} {selectedTarget} {taskId}
      {incrementalInfo} {cacheStatus}
    />

    <BuildHistory
      {buildHistory}
      {projectId}
      onSelectBuild={(task) => {
        if (task._cancel) { taskId = task.id; cancelBuild(); return; }
        // Stop any running poll before switching to a different build
        if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
        building = false;
        taskId = task.id;
        status = task.status || '';
        logLines = task.log ? task.log.split('\n').filter((l: string) => l) : [];
        if (task.status === 'running' || task.status === 'pending') { building = true; pollStatus(); }
      }}
      onDeleteBuild={deleteBuild}
      onDeleteFailedBuilds={deleteFailedBuilds}
      onRefresh={loadBuildHistory}
      onPublishRelease={publishRelease}
    />
  {/if}
</div>

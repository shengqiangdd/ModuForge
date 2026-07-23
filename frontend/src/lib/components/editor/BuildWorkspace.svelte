<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';

  let { projectId = '' }: { projectId?: string } = $props();

  interface ArchTarget {
    value: string;
    label: string;
    icon: string;
    desc: string;
  }

  let targets: ArchTarget[] = [
    { value: 'arm64-v8a', label: 'arm64', icon: 'smartphone', desc: '64-bit ARM (现代设备)' },
    { value: 'armeabi-v7a', label: 'arm', icon: 'phone_android', desc: '32-bit ARM (旧设备)' },
    { value: 'both', label: '通用', icon: 'devices', desc: 'arm64 + arm 双架构' },
  ];

  let selectedTarget = $state('arm64-v8a');
  let taskId = $state<string | null>(null);
  let status = $state<string>('');
  let logLines = $state<string[]>([]);
  let building = $state(false);
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null);
  let project = $state<any>(null);

  const statusConfig: Record<string, { color: string; bg: string; icon: string }> = {
    pending: { color: 'text-amber-600', bg: 'bg-amber-50', icon: 'schedule' },
    running: { color: 'text-blue-600', bg: 'bg-blue-50', icon: 'sync' },
    success: { color: 'text-green-600', bg: 'bg-green-50', icon: 'check_circle' },
    failed: { color: 'text-red-500', bg: 'bg-red-50', icon: 'error' },
  };

  onMount(async () => {
    if (!projectId) return;
    try {
      const res = await fetch(`/api/v1/projects/${projectId}`);
      if (res.ok) project = await res.json();
    } catch {}
    return () => { if (pollTimer) clearInterval(pollTimer); };
  });

  async function startBuild() {
    if (!projectId) return;
    building = true;
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
        body: JSON.stringify({ target: selectedTarget }),
      });
      if (!res.ok) throw new Error((await res.json()).error || '构建启动失败');
      const task = await res.json();
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
        if (status === 'success') { clearInterval(pollTimer!); pollTimer = null; building = false; toast('构建成功！', 'success'); }
        else if (status === 'failed') { clearInterval(pollTimer!); pollTimer = null; building = false; toast('构建失败', 'error'); }
      } catch { clearInterval(pollTimer!); pollTimer = null; building = false; }
    }, 1000);
  }

  function cancelBuild() {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = null;
    building = false;
    status = 'cancelled';
  }
</script>

<div class="p-6 max-w-3xl mx-auto">
  {#if !projectId}
    <div class="text-center py-16 text-[var(--color-text-secondary)]">
      <span class="material-symbols-outlined text-5xl mb-3 text-neutral-300">build</span>
      <p>请先选择项目</p>
    </div>
  {:else}
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
      <label class="text-sm font-medium text-[var(--color-text-secondary)] mb-3 block">目标架构</label>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {#each targets as t}
          <button
            class="relative p-4 rounded-2xl border-2 transition-all duration-200 text-left cursor-pointer
              {selectedTarget === t.value
                ? 'border-primary-500 bg-primary-50/50 shadow-glow'
                : 'border-[var(--color-border)] hover:border-neutral-300 hover:bg-[var(--color-surface)]'}"
            onclick={() => selectedTarget = t.value}
            disabled={building}
          >
            {#if selectedTarget === t.value}
              <div class="absolute top-2 right-2 w-5 h-5 rounded-full bg-primary-500 flex items-center justify-center">
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
          class="px-5 py-3 rounded-xl text-sm font-medium border border-red-300 text-red-600 hover:bg-red-50 transition-colors"
          onclick={cancelBuild}
        >
          取消
        </button>
      {/if}
    </div>

    <!-- Status -->
    {#if status}
      {@const cfg = statusConfig[status] || statusConfig.pending}
      <div class="mb-4 p-4 rounded-2xl border {cfg.bg} flex items-center gap-3">
        <span class="material-symbols-outlined text-[22px] {cfg.color}">{cfg.icon}</span>
        <span class="text-sm font-semibold {cfg.color} uppercase">{status}</span>
        {#if status === 'running'}
          <div class="ml-auto flex gap-1">
            <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_infinite]"></div>
            <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_0.3s_infinite]"></div>
            <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_0.6s_infinite]"></div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- Log -->
    {#if logLines.length > 0}
      <div class="rounded-2xl border border-[var(--color-border)] overflow-hidden">
        <div class="px-4 py-2.5 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center gap-2">
          <span class="material-symbols-outlined text-[16px] text-[var(--color-text-muted)]">terminal</span>
          <span class="text-xs font-medium text-[var(--color-text-secondary)]">构建日志</span>
          <span class="text-[10px] text-[var(--color-text-muted)] ml-auto">{logLines.length} 行</span>
        </div>
        <pre class="p-4 bg-neutral-950 text-xs font-mono overflow-auto max-h-96 whitespace-pre-wrap leading-relaxed">
          {#each logLines as line}
            <span class:text-red-400={line.startsWith('[ERROR]')} class:text-amber-400={line.startsWith('[WARN]')} class:text-green-300={line.startsWith('[SUCCESS]')} class:text-green-400={!line.startsWith('[') || line.startsWith('[INFO]')}>{line}</span>{'\n'}
          {/each}
        </pre>
      </div>
    {:else if building}
      <div class="rounded-2xl border border-[var(--color-border)] p-8 text-center text-[var(--color-text-muted)]">
        <span class="material-symbols-outlined text-3xl mb-2">pending</span>
        <p class="text-sm">等待构建日志...</p>
      </div>
    {/if}

    <!-- Download -->
    {#if status === 'success' && taskId}
      <a
        href="/api/v1/builds/{taskId}/download"
        class="mt-4 w-full py-3 rounded-xl font-semibold text-sm text-center no-underline
          bg-green-500 text-white hover:bg-green-600 transition-all duration-200 flex items-center justify-center gap-2"
      >
        <span class="material-symbols-outlined text-[18px]">download</span>
        下载构建产物
      </a>
    {/if}
  {/if}
</div>

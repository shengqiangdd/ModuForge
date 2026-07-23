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
    pending: { color: 'text-[var(--color-warning)]', bg: 'bg-[var(--color-warning-light)]', icon: 'schedule' },
    running: { color: 'text-[var(--color-info)]', bg: 'bg-[var(--color-info-light)]', icon: 'sync' },
    success: { color: 'text-[var(--color-success)]', bg: 'bg-[var(--color-success-light)]', icon: 'check_circle' },
    failed: { color: 'text-[var(--color-error)]', bg: 'bg-[var(--color-error-light)]', icon: 'error' },
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
      {/if}
    </div>

    <!-- Status -->
    {#if status}
      {@const cfg = statusConfig[status] || statusConfig.pending}
      <div class="mb-4 p-4 rounded-2xl border {cfg.bg} flex items-center gap-3" style="border-color: var(--color-border)">
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

<script lang="ts">
  import { onMount } from 'svelte';

  const id = window.location.pathname.split('/').filter(Boolean).at(-2) || '';
  let target = $state('magisk');
  let taskId = $state<string | null>(null);
  let status = $state('');
  let log = $state('');
  let building = $state(false);
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null);

  const targets = [
    { value: 'magisk', label: 'Magisk', icon: 'shield', color: 'from-green-500 to-green-600' },
    { value: 'ksu', label: 'KernelSU', icon: 'security', color: 'from-blue-500 to-blue-600' },
    { value: 'apatch', label: 'APatch', icon: 'lock', color: 'from-purple-500 to-purple-600' },
  ];

  onMount(() => () => { if (pollTimer) clearInterval(pollTimer); });

  async function startBuild() {
    building = true; log = ''; status = 'pending';
    try {
      const res = await fetch(`/api/v1/projects/${id}/build`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
        body: JSON.stringify({ target }),
      });
      const task = await res.json();
      taskId = task.id; status = task.status; pollStatus();
    } catch (e: any) { log = `构建失败: ${e.message}`; status = 'failed'; building = false; }
  }

  function pollStatus() {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = setInterval(async () => {
      if (!taskId) return;
      try {
        const task = await (await fetch(`/api/v1/builds/${taskId}`, { headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` } })).json();
        status = task.status; log = task.log || '';
        if (status === 'success' || status === 'failed') { clearInterval(pollTimer!); pollTimer = null; building = false; }
      } catch { clearInterval(pollTimer!); pollTimer = null; building = false; }
    }, 1500);
  }

  const statusConfig: Record<string, { color: string; bg: string; icon: string; label: string }> = {
    pending: { color: 'text-amber-600', bg: 'bg-amber-50', icon: 'schedule', label: '等待中' },
    running: { color: 'text-blue-600', bg: 'bg-blue-50', icon: 'sync', label: '构建中' },
    success: { color: 'text-green-600', bg: 'bg-green-50', icon: 'check_circle', label: '构建成功' },
    failed: { color: 'text-red-500', bg: 'bg-red-50', icon: 'error', label: '构建失败' },
  };
</script>

<div class="p-6 max-w-3xl mx-auto">
  <div class="mb-8">
    <h1 class="text-2xl font-bold text-[var(--color-text)]">构建模块</h1>
    <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">选择目标平台，一键构建 Magisk 模块</p>
  </div>

  <!-- Target Selection -->
  <div class="mb-8">
    <p class="text-sm font-medium text-[var(--color-text-secondary)] mb-3">构建目标</p>
    <div class="grid grid-cols-3 gap-3">
      {#each targets as t}
        <button
          class="relative p-5 rounded-2xl border-2 transition-all duration-200 cursor-pointer text-center group
            {target === t.value ? 'border-primary-500 bg-primary-50/50 shadow-glow' : 'border-[var(--color-border)] hover:border-neutral-300 hover:bg-neutral-50'}"
          onclick={() => target = t.value}
        >
          {#if target === t.value}
            <div class="absolute top-2 right-2 w-5 h-5 rounded-full bg-primary-500 flex items-center justify-center">
              <span class="material-symbols-outlined text-white text-[12px]">check</span>
            </div>
          {/if}
          <div class="w-10 h-10 rounded-xl bg-gradient-to-br {t.color} flex items-center justify-center mx-auto mb-2 group-hover:scale-105 transition-transform">
            <span class="material-symbols-outlined text-white text-xl">{t.icon}</span>
          </div>
          <span class="text-sm font-semibold text-[var(--color-text)]">{t.label}</span>
        </button>
      {/each}
    </div>
  </div>

  <!-- Build Button -->
  <button
    class="w-full py-3.5 rounded-xl font-semibold text-sm text-white transition-all duration-200 disabled:opacity-50
      bg-gradient-to-r from-primary-600 to-primary-700 hover:from-primary-700 hover:to-primary-800 active:scale-[0.98] shadow-sm hover:shadow-glow flex items-center justify-center gap-2"
    onclick={startBuild}
    disabled={building}
  >
    {#if building}
      <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
      构建中...
    {:else}
      <span class="material-symbols-outlined text-[18px]">build</span>
      开始构建
    {/if}
  </button>

  <!-- Status -->
  {#if status}
    {@const cfg = statusConfig[status] || statusConfig.pending}
    <div class="mt-6 card p-4 {cfg.bg} border border-transparent">
      <div class="flex items-center gap-3">
        <span class="material-symbols-outlined text-[22px] {cfg.color}">{cfg.icon}</span>
        <span class="text-sm font-semibold {cfg.color}">{cfg.label}</span>
        {#if status === 'running'}
          <div class="ml-auto flex gap-1">
            <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_infinite]"></div>
            <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_0.3s_infinite]"></div>
            <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-[pulseSoft_1s_0.6s_infinite]"></div>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Log -->
  {#if log}
    <div class="mt-4 rounded-2xl border border-[var(--color-border)] overflow-hidden">
      <div class="px-4 py-2.5 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center gap-2">
        <span class="material-symbols-outlined text-[16px] text-[var(--color-text-muted)]">terminal</span>
        <span class="text-xs font-medium text-[var(--color-text-secondary)]">构建日志</span>
      </div>
      <pre class="p-4 bg-[var(--color-bg)] text-[var(--color-text)] text-xs font-mono overflow-auto max-h-80 whitespace-pre-wrap leading-relaxed">{log}</pre>
    </div>
  {/if}

  <!-- Download -->
  {#if status === 'success'}
    <a
      href="/api/v1/builds/{taskId}/download"
      class="mt-4 w-full py-3 rounded-xl font-semibold text-sm text-center no-underline
        bg-green-500 text-white hover:bg-green-600 transition-all duration-200 flex items-center justify-center gap-2"
    >
      <span class="material-symbols-outlined text-[18px]">download</span>
      下载模块
    </a>
  {/if}
</div>

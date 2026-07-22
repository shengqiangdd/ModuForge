<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '../../../../lib/api/client';

  const id = window.location.pathname.split('/').filter(Boolean).at(-2) || '';

  let target = $state('magisk');
  let taskId = $state<string | null>(null);
  let status = $state('');
  let log = $state('');
  let building = $state(false);
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null);

  const targets = [
    { value: 'magisk', label: 'Magisk', icon: 'shield' },
    { value: 'ksu', label: 'KernelSU', icon: 'security' },
    { value: 'apatch', label: 'APatch', icon: 'lock' },
  ];

  onMount(() => {
    return () => { if (pollTimer) clearInterval(pollTimer); };
  });

  async function startBuild() {
    building = true;
    log = '';
    status = 'pending';
    try {
      const task = await client.post<any>(`/projects/${id}/build`, { target });
      taskId = task.id;
      status = task.status;
      pollStatus();
    } catch(e: any) {
      log = `构建失败: ${e.message}`;
      status = 'failed';
      building = false;
    }
  }

  function pollStatus() {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = setInterval(async () => {
      if (!taskId) return;
      try {
        const task = await client.get<any>(`/builds/${taskId}`);
        status = task.status;
        log = task.log || '';
        if (status === 'success' || status === 'failed') {
          clearInterval(pollTimer!);
          pollTimer = null;
          building = false;
        }
      } catch {
        clearInterval(pollTimer!);
        pollTimer = null;
        building = false;
      }
    }, 1500);
  }

  function downloadArtifact() {
    if (!taskId) return;
    window.open(`/api/v1/builds/${taskId}/download`, '_blank');
  }

  const statusColors: Record<string, string> = {
    pending: 'text-tertiary',
    running: 'text-primary',
    success: 'text-green',
    failed: 'text-error',
  };
</script>

<div class="max-w-3xl mx-auto p-6">
  <h1 class="text-headline-large text-on-surface mb-6">构建模块</h1>

  <!-- 目标选择 -->
  <div class="mb-6">
    <p class="text-body-medium text-on-surface-variant mb-3">选择构建目标</p>
    <div class="flex gap-3">
      {#each targets as t}
        <button
          class="flex items-center gap-2 px-4 py-3 rounded-xl border-2 transition-colors cursor-pointer"
          class:border-primary={target === t.value}
          class:bg-primary-container={target === t.value}
          class:text-on-primary-container={target === t.value}
          class:border-outline={target !== t.value}
          onclick={() => target = t.value}
        >
          <md-icon>{t.icon}</md-icon>
          <span class="text-label-large">{t.label}</span>
        </button>
      {/each}
    </div>
  </div>

  <!-- 构建按钮 -->
  <md-filled-button onclick={startBuild} disabled={building} class="mb-6">
    <md-icon slot="start">{building ? 'hourglass_empty' : 'build'}</md-icon>
    {building ? '构建中...' : '开始构建'}
  </md-filled-button>

  <!-- 构建状态 -->
  {#if status}
    <div class="mb-4">
      <span class="text-label-large {statusColors[status] || ''}">状态: {status}</span>
    </div>
  {/if}

  <!-- 构建日志 -->
  {#if log}
    <div class="border border-outline-variant rounded-xl overflow-hidden">
      <div class="px-4 py-2 bg-surface-container-high border-b border-outline-variant flex items-center gap-2">
        <md-icon class="text-sm">terminal</md-icon>
        <span class="text-label-medium">构建日志</span>
      </div>
      <pre class="p-4 bg-surface-container text-on-surface text-sm font-mono overflow-auto max-h-96 whitespace-pre-wrap">{log}</pre>
    </div>
  {/if}

  <!-- 下载按钮 -->
  {#if status === 'success'}
    <div class="mt-4">
      <md-filled-tonal-button onclick={downloadArtifact}>
        <md-icon slot="start">download</md-icon>
        下载模块
      </md-filled-tonal-button>
    </div>
  {/if}
</div>

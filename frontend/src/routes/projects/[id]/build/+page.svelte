<script lang="ts">
  import { onMount } from 'svelte';

  const id = window.location.pathname.split('/').filter(Boolean).at(-2) || '';
  let taskId = $state<string | null>(null);
  let status = $state('');
  let log = $state('');
  let building = $state(false);
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null);
  let trigger = $state<string>('manual');
  let commitHash = $state<string>('');

  // Architecture selection
  let selectedArch = $state('arm64');
  let architectures = $state<Array<{id: string; name: string; icon: string; desc: string; default: boolean}>>([]);

  // Incremental build info
  let incrementalInfo = $state<{ needs_rebuild: boolean; reason: string; changed_count: number; new_count: number } | null>(null);
  let buildCached = $state(false);

  // Cache status
  let cacheStatus = $state<{ total_size: number; file_count: number; hit_rate: number } | null>(null);

  // Push to device
  let devices = $state<Array<{serial: string; model: string; state: string}>>([]);
  let pushing = $state(false);
  let pushResult = $state('');
  let showDeviceList = $state(false);

  function goBack() {
    window.location.href = '/projects/' + id;
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  onMount(() => {
    loadDevices();
    loadArchitectures();
    loadCacheStatus();
    return () => { if (pollTimer) clearInterval(pollTimer); };
  });

  async function loadArchitectures() {
    try {
      const res = await fetch(`/api/v1/projects/${id}/build/architectures`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` }
      });
      if (res.ok) {
        const data = await res.json();
        architectures = data.architectures || [];
        const def = architectures.find((a: any) => a.default);
        if (def) selectedArch = def.id;
      }
    } catch {
      architectures = [
        { id: 'arm64', name: 'ARM64 (aarch64)', icon: 'smartphone', desc: '64-bit ARM (现代设备)', default: true },
        { id: 'arm', name: 'ARM (armv7)', icon: 'phone_android', desc: '32-bit ARM (旧设备)', default: false },
        { id: 'x86_64', name: 'x86_64 (x86)', icon: 'computer', desc: 'x86_64 (模拟器)', default: false },
      ];
    }
  }

  async function loadCacheStatus() {
    try {
      const res = await fetch(`/api/v1/projects/${id}/build/cache`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` }
      });
      if (res.ok) cacheStatus = await res.json();
    } catch {}
  }

  async function loadDevices() {
    try {
      const res = await fetch('/api/v1/adb/devices', {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` }
      });
      if (res.ok) {
        devices = (await res.json()).filter((d: any) => d.state === 'device');
      }
    } catch {}
  }

  async function pushToDevice(serial: string) {
    if (!taskId || !serial) return;
    pushing = true;
    pushResult = '';
    try {
      const res = await fetch('/api/v1/adb/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
        body: JSON.stringify({ serial, build_id: taskId }),
      });
      const data = await res.json();
      pushResult = res.ok ? `✅ 推送到 ${serial} 成功` : `❌ ${data.error || '推送失败'}`;
    } catch (e: any) {
      pushResult = `❌ ${e.message || '推送失败'}`;
    }
    pushing = false;
    showDeviceList = false;
  }

  async function startBuild() {
    building = true; log = ''; status = 'pending'; buildCached = false; incrementalInfo = null;
    try {
      const res = await fetch(`/api/v1/projects/${id}/build`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
        body: JSON.stringify({ target: 'universal', arch: selectedArch }),
      });
      const data = await res.json();
      if (data.cached) {
        buildCached = true;
        status = 'success';
        log = '[CACHE] 缓存命中，使用缓存产物';
        taskId = data.task?.id || '';
        building = false;
        loadCacheStatus();
        return;
      }
      taskId = data.id; status = data.status; pollStatus();
    } catch (e: any) { log = `构建失败: ${e.message}`; status = 'failed'; building = false; }
  }

  function pollStatus() {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = setInterval(async () => {
      if (!taskId) return;
      try {
        const task = await (await fetch(`/api/v1/builds/${taskId}`, { headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` } })).json();
        status = task.status; log = task.log || ''; trigger = task.trigger || 'manual'; commitHash = task.commit_hash || '';
        // Parse incremental info from log
        if (log.includes('Checking incremental build')) {
          const changedMatch = log.match(/Changed: (\d+) file/);
          const newMatch = log.match(/New: (\d+) file/);
          const reasonMatch = log.match(/📋 (.+)/m);
          incrementalInfo = {
            needs_rebuild: !log.includes('No changes detected'),
            reason: reasonMatch?.[1] || '',
            changed_count: changedMatch ? parseInt(changedMatch[1]) : 0,
            new_count: newMatch ? parseInt(newMatch[1]) : 0,
          };
        }
        if (status === 'success' || status === 'failed' || status === 'cancelled') {
          clearInterval(pollTimer!); pollTimer = null; building = false;
          loadCacheStatus();
        }
      } catch { clearInterval(pollTimer!); pollTimer = null; building = false; }
    }, 1500);
  }

  async function cancelBuild() {
    if (!taskId) return;
    try {
      await fetch(`/api/v1/builds/${taskId}/cancel`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      status = 'cancelled'; building = false;
      if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
    } catch {}
  }

  const triggerIcons: Record<string, string> = {
    manual: 'build',
    git: 'cloud_upload',
    webhook: 'webhook',
    schedule: 'schedule',
  };

  const statusConfig: Record<string, { color: string; bg: string; icon: string; label: string }> = {
    pending: { color: 'text-amber-500', bg: 'bg-amber-500/10', icon: 'schedule', label: '等待中' },
    running: { color: 'text-blue-500', bg: 'bg-blue-500/10', icon: 'sync', label: '构建中' },
    success: { color: 'text-green-500', bg: 'bg-green-500/10', icon: 'check_circle', label: '构建成功' },
    failed: { color: 'text-red-500', bg: 'bg-red-500/10', icon: 'error', label: '构建失败' },
    cancelled: { color: 'text-gray-400', bg: 'bg-gray-500/10', icon: 'cancel', label: '已取消' },
  };
</script>

<div class="min-h-screen bg-[var(--color-bg)]">
  <!-- Top Bar -->
  <div class="sticky top-0 z-30 flex items-center gap-3 px-4 py-3 border-b border-[var(--color-border)]" style="background: var(--color-bg-elevated); backdrop-filter: blur(12px);">
    <button class="flex items-center gap-1.5 px-3 py-2 rounded-xl text-sm font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={goBack}>
      <span class="material-symbols-outlined text-[18px]">arrow_back</span>
      <span class="hidden sm:inline">返回项目</span>
    </button>
    <div class="flex-1 min-w-0">
      <h1 class="text-base font-semibold text-[var(--color-text)] truncate">构建模块</h1>
      <p class="text-[11px] text-[var(--color-text-muted)]">Universal · Magisk + KSU + APatch · {selectedArch}</p>
    </div>
  </div>

  <div class="p-4 max-w-2xl mx-auto space-y-5">

    <!-- Universal Badge -->
    <div class="p-4 rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)]">
      <div class="flex items-center gap-3">
        <div class="w-11 h-11 rounded-xl bg-gradient-to-br from-violet-500 to-violet-600 flex items-center justify-center flex-shrink-0">
          <span class="material-symbols-outlined text-white text-xl">hub</span>
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-sm font-semibold text-[var(--color-text)]">Universal 模块</p>
          <p class="text-[11px] text-[var(--color-text-muted)] mt-0.5">自动兼容 Magisk · KernelSU · APatch</p>
        </div>
        <span class="material-symbols-outlined text-green-500 text-xl">check_circle</span>
      </div>
    </div>

    <!-- Target Architecture -->
    <div>
      <h2 class="text-xs font-semibold text-[var(--color-text-secondary)] mb-3 px-1">目标架构</h2>
      <div class="grid grid-cols-3 gap-2">
        {#each architectures as arch}
          <button
            class="p-3 rounded-xl border flex flex-col items-center gap-2 transition-colors
              {selectedArch === arch.id ? 'border-violet-500 bg-violet-500/10 relative' : 'border-[var(--color-border)] bg-[var(--color-surface)] hover:border-violet-500/50'}"
            onclick={() => selectedArch = arch.id}
            disabled={building}
          >
            <span class="material-symbols-outlined text-xl {selectedArch === arch.id ? 'text-violet-400' : 'text-[var(--color-text-muted)]'}">{arch.icon}</span>
            <span class="text-xs font-medium text-[var(--color-text)]">{arch.id}</span>
            <span class="text-[10px] {selectedArch === arch.id ? 'text-violet-400' : 'text-[var(--color-text-muted)]'}">
              {arch.id === 'arm64' ? '64-bit' : arch.id === 'arm' ? '32-bit' : 'x86'}
            </span>
            {#if selectedArch === arch.id}
              <span class="absolute -top-1.5 -right-1.5 w-5 h-5 rounded-full bg-violet-500 flex items-center justify-center">
                <span class="material-symbols-outlined text-white text-[12px]">check</span>
              </span>
            {/if}
          </button>
        {/each}
      </div>
    </div>

    <!-- Trigger Mode -->
    <div>
      <h2 class="text-xs font-semibold text-[var(--color-text-secondary)] mb-3 px-1">触发方式</h2>
      <div class="grid grid-cols-3 gap-2">
        <button class="p-4 rounded-xl border-2 border-violet-500 bg-violet-500/10 flex flex-col items-center gap-2">
          <span class="material-symbols-outlined text-xl text-violet-400">build</span>
          <span class="text-xs font-medium text-[var(--color-text)]">手动</span>
        </button>
        <button class="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] flex flex-col items-center gap-2 opacity-50 cursor-not-allowed">
          <span class="material-symbols-outlined text-xl text-[var(--color-text-muted)]">cloud_upload</span>
          <span class="text-xs font-medium text-[var(--color-text-secondary)]">Git 推送</span>
        </button>
        <button class="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] flex flex-col items-center gap-2 opacity-50 cursor-not-allowed">
          <span class="material-symbols-outlined text-xl text-[var(--color-text-muted)]">schedule</span>
          <span class="text-xs font-medium text-[var(--color-text-secondary)]">定时</span>
        </button>
      </div>
    </div>

    <!-- Cache Status -->
    {#if cacheStatus && cacheStatus.file_count > 0}
      <div class="p-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] flex items-center gap-3">
        <span class="material-symbols-outlined text-[18px] text-[var(--color-text-muted)]">database</span>
        <span class="text-xs font-medium text-[var(--color-text-secondary)]">构建缓存</span>
        <span class="text-[11px] text-[var(--color-text-muted)]">{cacheStatus.file_count} 个文件 · {formatBytes(cacheStatus.total_size)} · 命中率 {cacheStatus.hit_rate.toFixed(0)}%</span>
      </div>
    {/if}

    <!-- Build Button -->
    <div class="flex gap-3">
      <button
        class="flex-1 py-3.5 rounded-xl font-semibold text-sm text-white transition-all disabled:opacity-50
          bg-gradient-to-r from-violet-600 to-violet-700 hover:from-violet-700 hover:to-violet-800 active:scale-[0.98] shadow-sm flex items-center justify-center gap-2"
        onclick={startBuild}
        disabled={building}
      >
        {#if building}
          <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
          构建中...
        {:else}
          <span class="material-symbols-outlined text-[18px]">rocket_launch</span>
          开始构建
        {/if}
      </button>
      {#if building}
        <button
          class="px-5 py-3.5 rounded-xl text-sm font-medium border border-red-500 text-red-500 hover:bg-red-500/10 transition-colors"
          onclick={cancelBuild}
        >
          取消
        </button>
      {/if}
    </div>

    <!-- Status -->
    {#if status}
      {@const cfg = statusConfig[status] || statusConfig.pending}
      <div class="p-4 rounded-2xl border border-[var(--color-border)] {cfg.bg}">
        <div class="flex items-center gap-3">
          <span class="material-symbols-outlined text-xl {cfg.color}">{cfg.icon}</span>
          <span class="text-sm font-semibold {cfg.color}">{cfg.label}</span>
          <span class="text-[10px] px-2 py-0.5 rounded-full bg-[var(--color-bg-elevated)] text-[var(--color-text-muted)] flex items-center gap-1">
            <span class="material-symbols-outlined text-[12px]">{triggerIcons[trigger] || 'build'}</span>
            {trigger === 'manual' ? '手动' : trigger === 'git' ? 'Git' : trigger}
          </span>
          <span class="text-[10px] px-2 py-0.5 rounded-full bg-[var(--color-bg-elevated)] text-[var(--color-text-muted)] flex items-center gap-1">
            <span class="material-symbols-outlined text-[12px]">microchip</span>
            {selectedArch}
          </span>
          {#if commitHash}
            <span class="text-[10px] font-mono px-2 py-0.5 rounded-full bg-[var(--color-bg-elevated)] text-[var(--color-text-muted)]">{commitHash.slice(0, 7)}</span>
          {/if}
          {#if status === 'running'}
            <div class="ml-auto flex gap-1">
              <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse"></div>
              <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse" style="animation-delay: 0.2s"></div>
              <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse" style="animation-delay: 0.4s"></div>
            </div>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Incremental Build Info -->
    {#if incrementalInfo}
      <div class="p-3 rounded-xl border flex items-center gap-3 {incrementalInfo.needs_rebuild ? 'bg-amber-500/10 border-amber-500/30' : 'bg-green-500/10 border-green-500/30'}">
        <span class="material-symbols-outlined text-[18px] {incrementalInfo.needs_rebuild ? 'text-amber-500' : 'text-green-500'}">
          {incrementalInfo.needs_rebuild ? 'difference' : 'check_circle'}
        </span>
        <div>
          <span class="text-sm font-medium {incrementalInfo.needs_rebuild ? 'text-amber-500' : 'text-green-500'}">
            {incrementalInfo.needs_rebuild ? '增量编译' : '无变化'}
          </span>
          <p class="text-[11px] text-[var(--color-text-muted)]">
            {incrementalInfo.needs_rebuild ? incrementalInfo.reason : '所有源文件未变化，使用缓存的二进制'}
          </p>
        </div>
      </div>
    {/if}

    <!-- Cache Hit -->
    {#if buildCached}
      <div class="p-3 rounded-xl border bg-green-500/10 border-green-500/30 flex items-center gap-3">
        <span class="material-symbols-outlined text-[18px] text-green-500">cached</span>
        <span class="text-sm font-medium text-green-500">缓存命中</span>
        <span class="text-xs text-[var(--color-text-muted)]">使用缓存构建产物</span>
      </div>
    {/if}

    <!-- Log -->
    {#if log}
      <div class="rounded-2xl border border-[var(--color-border)] overflow-hidden">
        <div class="px-4 py-2.5 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center gap-2">
          <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]">terminal</span>
          <span class="text-xs font-medium text-[var(--color-text-secondary)]">构建日志</span>
        </div>
        <pre class="p-4 bg-[var(--color-bg)] text-[var(--color-text)] text-xs font-mono overflow-auto max-h-80 whitespace-pre-wrap leading-tight">{log}</pre>
      </div>
    {/if}

    <!-- Download & Push -->
    {#if status === 'success'}
      <div class="space-y-3">
        <a
          href="/api/v1/builds/{taskId}/download"
          class="w-full py-3 rounded-xl font-semibold text-sm text-center no-underline
            bg-green-500 text-white hover:bg-green-600 transition-all flex items-center justify-center gap-2"
        >
          <span class="material-symbols-outlined text-[18px]">download</span>
          下载模块
        </a>
        {#if devices.length > 0}
          <button
            class="w-full py-3 rounded-xl font-semibold text-sm text-white transition-all
              bg-gradient-to-r from-blue-600 to-blue-700 hover:from-blue-700 hover:to-blue-800
              flex items-center justify-center gap-2 disabled:opacity-50"
            onclick={() => showDeviceList = !showDeviceList}
            disabled={pushing}
          >
            {#if pushing}
              <span class="material-symbols-outlined text-[18px] animate-spin">sync</span>
              推送中...
            {:else}
              <span class="material-symbols-outlined text-[18px]">send</span>
              推送到设备
            {/if}
          </button>
        {/if}
        {#if showDeviceList && devices.length > 0}
          <div class="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-2 space-y-1">
            {#each devices as device}
              <button
                class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-left hover:bg-[var(--color-bg)] transition-colors"
                onclick={() => pushToDevice(device.serial)}
              >
                <span class="material-symbols-outlined text-[18px] text-blue-500">phone_android</span>
                <div class="flex-1 min-w-0">
                  <p class="font-medium text-[var(--color-text)] truncate">{device.model || device.serial}</p>
                  <p class="text-[11px] text-[var(--color-text-muted)]">{device.serial}</p>
                </div>
                <span class="material-symbols-outlined text-[16px] text-[var(--color-text-muted)]">send</span>
              </button>
            {/each}
          </div>
        {/if}
        {#if pushResult}
          <div class="p-3 rounded-xl text-sm text-center {pushResult.startsWith('✅') ? 'bg-green-500/10 text-green-500' : 'bg-red-500/10 text-red-500'}">
            {pushResult}
          </div>
        {/if}
      </div>
    {/if}
  </div>
</div>

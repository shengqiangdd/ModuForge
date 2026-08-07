<script lang="ts">
  import { focusTrap } from '$lib/utils/focusTrap';
  let {
    show = false,
    moduleName = '',
    moduleVersion = '',
    installing = false,
    installSteps = [],
    installError = '',
    loadingDevices = false,
    installableDevices = [],
    selectedDevice = $bindable(''),
    onClose,
    onStartInstall,
  }: {
    show?: boolean;
    moduleName?: string;
    moduleVersion?: string;
    installing?: boolean;
    installSteps?: { label: string; status: 'pending' | 'running' | 'done' | 'error'; detail?: string }[];
    installError?: string;
    loadingDevices?: boolean;
    installableDevices?: { serial: string; model: string; state: string }[];
    selectedDevice?: string;
    onClose?: () => void;
    onStartInstall?: () => void;
  } = $props();
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget && !installing) onClose?.(); }}>
    <div class="rounded-2xl w-full max-w-md border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: 0 25px 50px -12px rgba(0,0,0,0.5)" role="dialog" aria-modal="true" tabindex="-1" use:focusTrap>
      <!-- Header -->
      <div class="p-5 border-b flex items-center gap-3" style="border-color: var(--color-border)">
        <div class="w-10 h-10 rounded-xl flex items-center justify-center" style="background: var(--gradient-brand)">
          <span class="material-symbols-outlined text-white text-[20px]">download</span>
        </div>
        <div class="flex-1 min-w-0">
          <h3 class="text-base font-bold text-[var(--color-text)] truncate">安装 {moduleName}</h3>
          <p class="text-xs text-[var(--color-text-muted)]">v{moduleVersion}</p>
        </div>
        {#if !installing}
          <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
            <span class="material-symbols-outlined text-[18px]" style="color: var(--color-text-muted)">close</span>
          </button>
        {/if}
      </div>

      <!-- Steps -->
      <div class="p-5 space-y-1">
        {#each installSteps as step, i}
          <div class="flex items-start gap-3 py-3 {i < installSteps.length - 1 ? 'border-b' : ''}" style={i < installSteps.length - 1 ? 'border-color: var(--color-border)' : ''}>
            <div class="mt-0.5 flex-shrink-0">
              {#if step.status === 'done'}
                <div class="w-7 h-7 rounded-full flex items-center justify-center" style="background: rgba(34,197,94,0.15)">
                  <span class="material-symbols-outlined text-[16px]" style="color: #22c55e">check_circle</span>
                </div>
              {:else if step.status === 'running'}
                <div class="w-7 h-7 rounded-full flex items-center justify-center" style="background: rgba(59,130,246,0.15)">
                  <div class="animate-spin h-4 w-4 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
                </div>
              {:else if step.status === 'error'}
                <div class="w-7 h-7 rounded-full flex items-center justify-center" style="background: rgba(239,68,68,0.15)">
                  <span class="material-symbols-outlined text-[16px]" style="color: #ef4444">error</span>
                </div>
              {:else}
                <div class="w-7 h-7 rounded-full flex items-center justify-center" style="background: var(--color-surface)">
                  <span class="text-xs font-bold" style="color: var(--color-text-muted)">{i + 1}</span>
                </div>
              {/if}
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium {step.status === 'pending' ? 'text-[var(--color-text-muted)]' : 'text-[var(--color-text)]'}">{step.label}</div>
              {#if step.detail}
                <div class="mt-1 text-xs font-mono {step.status === 'error' ? 'text-[var(--color-error)]' : 'text-[var(--color-text-muted)]'} truncate" title={step.detail}>{step.detail}</div>
              {/if}
            </div>
          </div>
        {/each}
      </div>

      {#if installError}
        <div class="mx-5 mb-3 p-3 rounded-xl text-xs" style="background: rgba(239,68,68,0.1); border: 1px solid rgba(239,68,68,0.2)">
          <span style="color: #ef4444">{installError}</span>
        </div>
      {/if}

      <div class="p-5 border-t flex items-center gap-3" style="border-color: var(--color-border)">
        {#if installing}
          <div class="flex-1 text-xs text-[var(--color-text-muted)]">
            <span class="animate-pulse">▊</span> 正在安装...
          </div>
        {:else if installSteps.every(s => s.status === 'done')}
          <div class="flex-1"></div>
          <button class="px-5 py-2 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--gradient-brand)" onclick={onClose}>完成</button>
        {:else if installSteps.some(s => s.status === 'error')}
          <button class="px-4 py-2 rounded-xl text-sm font-medium transition-colors" style="border: 1px solid var(--color-border); color: var(--color-text-secondary)" onclick={onClose}>关闭</button>
          <button class="px-5 py-2 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--gradient-brand)" onclick={onStartInstall}>重试</button>
        {:else}
          <div class="flex-1">
            {#if loadingDevices}
              <span class="text-xs text-[var(--color-text-muted)]">检测设备中...</span>
            {:else if installableDevices.length === 0}
              <span class="text-xs text-[var(--color-error)]">未发现已连接设备</span>
            {:else}
              <select class="w-full px-3 py-2 rounded-xl text-sm border appearance-none" style="background: var(--color-surface); border-color: var(--color-border); color: var(--color-text)" bind:value={selectedDevice}>
                <option value="">选择设备...</option>
                {#each installableDevices as dev}
                  <option value={dev.serial}>{dev.model || dev.serial} ({dev.serial})</option>
                {/each}
              </select>
            {/if}
          </div>
          <button class="px-5 py-2 rounded-xl text-sm font-medium text-white transition-colors disabled:opacity-50" style="background: var(--gradient-brand)" disabled={!selectedDevice || loadingDevices} onclick={onStartInstall}>
            <span class="material-symbols-outlined text-[14px] align-text-bottom">download</span>
            开始安装
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<script lang="ts">
  import type { Writable } from 'svelte/store';

  let { activeTab, onTabChange }: { activeTab: string; onTabChange: (tab: string) => void } = $props();

  const tabs = [
    { key: 'info', icon: 'phone_android', label: '设备信息' },
    { key: 'modules', icon: 'extension', label: '模块' },
    { key: 'apps', icon: 'apps', label: '应用' },
    { key: 'files', icon: 'folder', label: '文件' },
    { key: 'screen', icon: 'screen_lock_portrait', label: '屏幕' },
    { key: 'logs', icon: 'terminal', label: '日志' },
    { key: 'shell', icon: 'code', label: '终端' },
  ];
</script>

<div class="flex gap-1 mb-6 overflow-x-auto border-b" style="border-color: var(--color-border)">
  {#each tabs as tab}
    <button
      class="flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium transition-colors whitespace-nowrap"
      style={activeTab === tab.key
        ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)'
        : 'color: var(--color-text-secondary)'}
      onclick={() => onTabChange(tab.key)}
      aria-label={tab.label}
      aria-selected={activeTab === tab.key}
      role="tab"
    >
      <span class="material-symbols-outlined text-[18px]">{tab.icon}</span>
      {tab.label}
    </button>
  {/each}
</div>

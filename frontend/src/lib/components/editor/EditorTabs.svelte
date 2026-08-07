<script lang="ts">
  let {
    openTabs = [],
    activeTab = null,
    diffMode = false,
    showDiffList = false,
    diffFiles = [],
    selectedDiffFile = null,
    getFileIcon,
    getFileIconColor,
    onSwitchTab,
    onCloseTab,
    onSelectDiffFile,
  }: {
    openTabs?: string[];
    activeTab?: string | null;
    diffMode?: boolean;
    showDiffList?: boolean;
    diffFiles?: { path: string; current: string; incoming: string }[];
    selectedDiffFile?: string | null;
    getFileIcon?: (path: string) => string;
    getFileIconColor?: (path: string) => string;
    onSwitchTab?: (path: string) => void;
    onCloseTab?: (path: string) => void;
    onSelectDiffFile?: (path: string) => void;
  } = $props();
</script>

{#if openTabs.length > 0 && !diffMode}
  <div class="h-9 flex items-center border-b border-[var(--color-border)] bg-[var(--color-bg)] overflow-x-auto flex-shrink-0">
    {#each openTabs as tab}
      <button
        class="flex items-center gap-1.5 px-3 h-full text-xs border-r border-[var(--color-border)] transition-colors whitespace-nowrap flex-shrink-0
          {activeTab === tab ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text)] font-medium border-t-2 border-t-primary-500' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'}"
        onclick={() => onSwitchTab?.(tab)}
      >
        <span>{tab.split('/').pop()}</span>
        <span
          role="button"
          tabindex="-1"
          class="material-symbols-outlined text-[12px] p-0.5 rounded hover:bg-black/10 transition-colors"
          onclick={(e) => { e.stopPropagation(); onCloseTab?.(tab); }}
          onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); e.stopPropagation(); onCloseTab?.(tab); } }}
        >close</span>
      </button>
    {/each}
  </div>
{/if}

{#if showDiffList && diffFiles.length > 0}
  <div class="h-10 flex items-center px-3 gap-1 border-b border-[var(--color-border)] bg-[var(--color-bg)] overflow-x-auto flex-shrink-0">
    {#each diffFiles as df}
      <button
        class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs whitespace-nowrap transition-all
          {selectedDiffFile === df.path ? 'bg-[var(--gradient-brand-subtle)] text-[var(--color-primary)]' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
        onclick={() => onSelectDiffFile?.(df.path)}
      >
        <span class="material-symbols-outlined text-[12px]" style="color: {getFileIconColor?.(df.path) || 'var(--color-text-muted)'}">{getFileIcon?.(df.path) || 'description'}</span>
        <span>{df.path.split('/').pop()}</span>
        {#if df.current !== df.incoming}
          <span class="w-2 h-2 rounded-full bg-amber-500"></span>
        {:else}
          <span class="w-2 h-2 rounded-full bg-green-500"></span>
        {/if}
      </button>
    {/each}
  </div>
{/if}

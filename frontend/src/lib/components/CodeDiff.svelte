<script lang="ts">
  interface DiffEntry {
    type: 'add' | 'remove' | 'context';
    line: number;
    old?: string;
    new?: string;
  }

  let { diffs = [], filePath = '', viewMode = $bindable<'unified' | 'split'>('unified') }: {
    diffs: DiffEntry[];
    filePath?: string;
    viewMode?: 'unified' | 'split';
  } = $props();

  let copied = $state(false);

  function copyAll() {
    const text = diffs
      .map(d => {
        if (d.type === 'add') return `+ ${d.new || ''}`;
        if (d.type === 'remove') return `- ${d.old || ''}`;
        return `  ${d.old || ''}`;
      })
      .join('\n');
    navigator.clipboard.writeText(text);
    copied = true;
    setTimeout(() => (copied = false), 2000);
  }

  let addCount = $derived(diffs.filter(d => d.type === 'add').length);
  let removeCount = $derived(diffs.filter(d => d.type === 'remove').length);
</script>

<div class="diff-container rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg)]">
  <!-- Header -->
  <div class="flex items-center justify-between px-3 py-2 border-b border-[var(--color-border)]" style="background: var(--color-surface);">
    <div class="flex items-center gap-2">
      <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]">difference</span>
      <span class="text-xs font-medium text-[var(--color-text)]">{filePath || '代码差异'}</span>
      <span class="text-[10px] text-[var(--color-text-muted)]">
        <span class="text-green-500">+{addCount}</span>
        <span class="text-red-500 ml-1">-{removeCount}</span>
      </span>
    </div>
    <div class="flex items-center gap-1">
      <button
        class="px-2 py-0.5 text-[10px] rounded transition-colors {viewMode === 'unified' ? 'bg-primary-500/20 text-primary-400' : 'text-[var(--color-text-muted)] hover:bg-[var(--color-surface)]'}"
        onclick={() => viewMode = 'unified'}
      >统一</button>
      <button
        class="px-2 py-0.5 text-[10px] rounded transition-colors {viewMode === 'split' ? 'bg-primary-500/20 text-primary-400' : 'text-[var(--color-text-muted)] hover:bg-[var(--color-surface)]'}"
        onclick={() => viewMode = 'split'}
      >并排</button>
      <button
        class="ml-1 px-2 py-0.5 text-[10px] rounded text-[var(--color-text-muted)] hover:bg-[var(--color-surface)] transition-colors"
        onclick={copyAll}
      >
        {copied ? '已复制' : '复制'}
      </button>
    </div>
  </div>

  <!-- Diff Content -->
  {#if viewMode === 'unified'}
    <div class="font-mono text-xs leading-5 overflow-x-auto">
      {#each diffs as d}
        <div class="flex {d.type === 'add' ? 'bg-green-500/10' : d.type === 'remove' ? 'bg-red-500/10' : ''} hover:brightness-110 transition-all">
          <span class="w-10 flex-shrink-0 text-right pr-2 text-[var(--color-text-muted)] select-none opacity-50">{d.line}</span>
          <span class="w-5 flex-shrink-0 text-center select-none {d.type === 'add' ? 'text-green-500' : d.type === 'remove' ? 'text-red-500' : 'text-[var(--color-text-muted)]'}">
            {d.type === 'add' ? '+' : d.type === 'remove' ? '-' : ' '}
          </span>
          <span class="flex-1 pr-3 whitespace-pre {d.type === 'add' ? 'text-green-400' : d.type === 'remove' ? 'text-red-400' : 'text-[var(--color-text)]'}">
            {d.type === 'add' ? d.new : d.old}
          </span>
        </div>
      {/each}
    </div>
  {:else}
    <div class="flex font-mono text-xs leading-5 overflow-x-auto">
      <!-- Left (old) -->
      <div class="flex-1 border-r border-[var(--color-border)]">
        {#each diffs as d}
          <div class="flex {d.type === 'remove' ? 'bg-red-500/10' : d.type === 'context' ? '' : 'bg-[var(--color-surface)]/50'}">
            <span class="w-8 flex-shrink-0 text-right pr-1 text-[var(--color-text-muted)] select-none opacity-50">{d.type !== 'add' ? d.line : ''}</span>
            <span class="w-4 flex-shrink-0 text-center text-red-500 select-none">{d.type === 'remove' ? '-' : ' '}</span>
            <span class="flex-1 pr-2 whitespace-pre {d.type === 'remove' ? 'text-red-400' : 'text-[var(--color-text)]'}">
              {d.type !== 'add' ? (d.old || '') : ''}
            </span>
          </div>
        {/each}
      </div>
      <!-- Right (new) -->
      <div class="flex-1">
        {#each diffs as d}
          <div class="flex {d.type === 'add' ? 'bg-green-500/10' : d.type === 'context' ? '' : 'bg-[var(--color-surface)]/50'}">
            <span class="w-8 flex-shrink-0 text-right pr-1 text-[var(--color-text-muted)] select-none opacity-50">{d.type !== 'remove' ? d.line : ''}</span>
            <span class="w-4 flex-shrink-0 text-center text-green-500 select-none">{d.type === 'add' ? '+' : ' '}</span>
            <span class="flex-1 pr-2 whitespace-pre {d.type === 'add' ? 'text-green-400' : 'text-[var(--color-text)]'}">
              {d.type !== 'remove' ? (d.new || '') : ''}
            </span>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if diffs.length === 0}
    <div class="flex items-center justify-center py-8 text-sm text-[var(--color-text-muted)]">
      <span class="material-symbols-outlined text-[20px] mr-2">check_circle</span>
      无差异
    </div>
  {/if}
</div>

<script lang="ts">
  let {
    skillName = '',
    content = '',
    files = [],
    expanded = $bindable(false),
    onToggle,
    onInsertFile,
  }: {
    skillName?: string;
    content?: string;
    files?: { path: string; content: string }[];
    expanded?: boolean;
    onToggle?: () => void;
    onInsertFile?: (file: { path: string; content: string }) => void;
  } = $props();

  let needsExpand = $derived(
    (content && (content.split('\n').length > 5 || content.length > 300))
  );

  function toggleExpand() {
    expanded = !expanded;
    onToggle?.();
  }
</script>

{#if content}
  <div
    class="text-[10px] mt-0.5 whitespace-pre-wrap step-result-content {expanded ? 'expanded' : ''}"
    style="color: var(--color-text-secondary);"
  >
    {content}
  </div>
  {#if needsExpand}
    <button
      class="text-[9px] mt-0.5 hover:underline"
      style="color: var(--color-primary);"
      onclick={toggleExpand}
    >
      {expanded ? '收起' : '展开全部'}
    </button>
  {/if}
{/if}
{#if files && files.length > 0}
  <div class="mt-1 space-y-0.5">
    {#each files as file (file.path)}
      <div class="flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded" style="background: var(--color-surface);">
        <span class="material-symbols-outlined text-[10px] text-primary-500">description</span>
        <span class="font-mono text-[var(--color-text-secondary)] truncate">{file.path}</span>
        {#if onInsertFile}
          <button
            class="ml-auto text-[9px] px-1 py-0.5 rounded hover:bg-[var(--color-bg)] transition-colors"
            style="color: var(--color-primary);"
            onclick={() => onInsertFile(file)}
          >
            插入
          </button>
        {/if}
      </div>
    {/each}
  </div>
{/if}

<script lang="ts">
  import { renderMarkdown } from '$lib/utils/markdown';
  import type { ChangelogEntry } from './types';

  let {
    changelogs = [],
    changelogsLoading = false,
  }: {
    changelogs?: ChangelogEntry[];
    changelogsLoading?: boolean;
  } = $props();
</script>

<div class="p-6">
  <h3 class="text-sm font-semibold text-[var(--color-text)] mb-4">更新日志</h3>
  {#if changelogsLoading}
    <p class="text-xs text-[var(--color-text-muted)]">加载中...</p>
  {:else if changelogs.length === 0}
    <p class="text-xs text-[var(--color-text-muted)]">暂无更新日志</p>
  {:else}
    <div class="space-y-4">
      {#each changelogs as log}
        <div class="relative pl-6 pb-4" style="border-left: 2px solid var(--color-border)">
          <div class="absolute left-[-5px] top-1 w-2 h-2 rounded-full" style="background: var(--color-primary)"></div>
          <div class="flex items-center gap-2 mb-1">
            <span class="text-sm font-semibold text-[var(--color-text)]">{log.version}</span>
            <span class="text-xs text-[var(--color-text-muted)]">{new Date(log.created_at).toLocaleDateString()}</span>
          </div>
          <div class="text-sm text-[var(--color-text-secondary)]" style="line-height: 1.6">{@html renderMarkdown(log.content)}</div>
        </div>
      {/each}
    </div>
  {/if}
</div>

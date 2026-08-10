<script lang="ts">
  export interface Subtask {
    id: string;
    description: string;
    status: 'pending' | 'in_progress' | 'completed' | 'failed' | 'running' | 'done' | 'error' | 'skipped';
    dependencies?: string[];
    files?: string[];
    tools?: string[];
    progress?: number;
    started_at?: number | string;
    completed_at?: number | string;
    retry_count?: number;
  }

  let {
    subtasks = [],
    collapsed = false,
    onToggleCollapse,
  }: {
    subtasks: Subtask[];
    collapsed?: boolean;
    onToggleCollapse?: () => void;
  } = $props();

  const STATUS_ICONS: Record<string, string> = {
    pending: 'radio_button_unchecked',
    in_progress: 'progress_activity',
    completed: 'check_circle',
    failed: 'error',
  };

  const STATUS_COLORS: Record<string, string> = {
    pending: 'text-[var(--color-text-muted)]',
    in_progress: 'text-primary-500',
    completed: 'text-success',
    failed: 'text-error',
  };

  const STATUS_LABELS: Record<string, string> = {
    pending: '等待中',
    in_progress: '执行中',
    completed: '已完成',
    failed: '失败',
  };

  let completedCount = $derived(subtasks.filter(s => s.status === 'completed').length);
  let totalCount = $derived(subtasks.length);
  let overallProgress = $derived(totalCount > 0 ? Math.round((completedCount / totalCount) * 100) : 0);
  let hasActiveTask = $derived(subtasks.some(s => s.status === 'in_progress'));
</script>

{#if subtasks.length > 0}
  <div class="px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] transition-all">
    <button
      class="flex items-center gap-2 w-full text-left group"
      onclick={onToggleCollapse}
    >
      {#if hasActiveTask}
        <span class="material-symbols-outlined text-[14px] text-primary-500 animate-spin">progress_activity</span>
      {:else if completedCount === totalCount}
        <span class="material-symbols-outlined text-[14px] text-success">check_circle</span>
      {:else}
        <span class="material-symbols-outlined text-[14px] text-primary-500">checklist</span>
      {/if}
      <span class="text-xs font-semibold text-[var(--color-text)]">任务清单</span>
      <span class="text-[10px] text-[var(--color-text-muted)] bg-[var(--color-surface)] px-1.5 py-0.5 rounded">
        {completedCount}/{totalCount}
      </span>
      <div class="flex-1 mx-2 h-1 bg-[var(--color-surface)] rounded-full overflow-hidden">
        <div
          class="h-full rounded-full transition-all duration-500 ease-out"
          style="width: {overallProgress}%; background: var(--color-primary)"
        ></div>
      </div>
      <span class="text-[10px] text-[var(--color-text-muted)]">{overallProgress}%</span>
      <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)] transition-transform {collapsed ? '' : 'rotate-180'}">expand_less</span>
    </button>

    {#if !collapsed}
      <div class="mt-2 space-y-1">
        {#each subtasks as subtask, i (subtask.id)}
          <div
            class="flex items-center gap-2 px-2 py-1.5 rounded-lg transition-all duration-300 {
              subtask.status === 'in_progress'
                ? 'bg-primary-500/10 ring-1 ring-primary-500/20'
                : subtask.status === 'completed'
                  ? 'bg-success/5'
                  : subtask.status === 'failed'
                    ? 'bg-error/5'
                    : ''
            }"
          >
            <span class="material-symbols-outlined text-[14px] {STATUS_COLORS[subtask.status]} {
              subtask.status === 'in_progress' ? 'animate-spin' : ''
            }">
              {STATUS_ICONS[subtask.status]}
            </span>

            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-1.5">
                <span class="text-[11px] font-medium {
                  subtask.status === 'completed'
                    ? 'text-success line-through opacity-70'
                    : subtask.status === 'failed'
                      ? 'text-error'
                      : subtask.status === 'in_progress'
                        ? 'text-[var(--color-text)]'
                        : 'text-[var(--color-text-muted)]'
                }">
                  {subtask.description}
                </span>
              </div>

              {#if subtask.files && subtask.files.length > 0}
                <div class="flex flex-wrap gap-1 mt-0.5">
                  {#each subtask.files.slice(0, 3) as file}
                    <span class="text-[9px] px-1 py-0 rounded bg-[var(--color-surface)] text-[var(--color-text-muted)] font-mono">
                      {file.split('/').pop()}
                    </span>
                  {/each}
                  {#if subtask.files.length > 3}
                    <span class="text-[9px] text-[var(--color-text-muted)]">+{subtask.files.length - 3}</span>
                  {/if}
                </div>
              {/if}
              {#if subtask.tools && subtask.tools.length > 0}
                <div class="flex flex-wrap gap-1 mt-0.5">
                  {#each subtask.tools.slice(0, 2) as tool}
                    <span class="text-[9px] px-1 py-0 rounded bg-primary-500/10 text-primary-500 font-mono">
                      🔧 {tool}
                    </span>
                  {/each}
                  {#if subtask.tools.length > 2}
                    <span class="text-[9px] text-[var(--color-text-muted)]">+{subtask.tools.length - 2}</span>
                  {/if}
                </div>
              {/if}

              {#if subtask.status === 'in_progress' && subtask.progress !== undefined && subtask.progress > 0}
                <div class="mt-1 h-1 bg-[var(--color-surface)] rounded-full overflow-hidden">
                  <div
                    class="h-full bg-primary-500 rounded-full transition-all duration-300"
                    style="width: {subtask.progress}%"
                  ></div>
                </div>
              {/if}
            </div>

            <span class="text-[9px] text-[var(--color-text-muted)] flex-shrink-0">
              {STATUS_LABELS[subtask.status]}
            </span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

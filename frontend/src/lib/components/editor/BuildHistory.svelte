<script lang="ts">
  let {
    buildHistory = [],
    onSelectBuild,
    onDeleteBuild,
    onDeleteFailedBuilds,
    onRefresh,
  }: {
    buildHistory?: { id: string; status: string; timestamp: string; branch: string; target: string; version: string; trigger?: string; commit_hash?: string; created_at?: string; _cancel?: boolean }[];
    onSelectBuild?: (task: { id: string; _cancel?: boolean; status?: string; log?: string }) => void;
    onDeleteBuild?: (buildId: string, e: Event) => void;
    onDeleteFailedBuilds?: () => void;
    onRefresh?: () => void;
  } = $props();

  const statusConfig: Record<string, { color: string; bg: string; icon: string }> = {
    pending: { color: 'text-[var(--color-warning)]', bg: 'bg-[var(--color-warning-light)]', icon: 'schedule' },
    running: { color: 'text-[var(--color-info)]', bg: 'bg-[var(--color-info-light)]', icon: 'sync' },
    success: { color: 'text-[var(--color-success)]', bg: 'bg-[var(--color-success-light)]', icon: 'check_circle' },
    failed: { color: 'text-[var(--color-error)]', bg: 'bg-[var(--color-error-light)]', icon: 'error' },
    cancelled: { color: 'text-[var(--color-text-muted)]', bg: 'bg-[var(--color-surface)]', icon: 'cancel' },
  };

  const triggerIcons: Record<string, string> = { manual: 'build', git: 'cloud_upload', push: 'cloud_upload', schedule: 'schedule' };
</script>

{#if buildHistory.length > 0}
  <div class="mt-8">
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-sm font-semibold text-[var(--color-text)] flex items-center gap-2">
        <span class="material-symbols-outlined text-[18px]">history</span>
        构建历史
      </h3>
      <div class="flex items-center gap-2">
        {#if buildHistory.some(b => b.status === 'failed')}
          <button class="px-3 py-1 rounded-lg text-xs font-medium transition-colors text-[var(--color-error)]" style="border: 1px solid var(--color-border); color: var(--color-error)" onclick={onDeleteFailedBuilds}>清除失败</button>
        {/if}
        <button class="px-3 py-1 rounded-lg text-xs font-medium transition-colors" style="border: 1px solid var(--color-border); color: var(--color-text-secondary)" onclick={onRefresh}>刷新</button>
      </div>
    </div>
    <div class="space-y-2">
      {#each buildHistory as task}
        {@const cfg = statusConfig[task.status] || statusConfig.pending}
        <div
          role="button"
          tabindex="0"
          class="p-3 rounded-xl border cursor-pointer transition-colors"
          style="border-color: var(--color-border); background: var(--color-bg-elevated)"
          onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); (e.currentTarget as HTMLElement).click(); } }}
          onclick={() => onSelectBuild?.(task)}
        >
          <div class="flex items-center justify-between flex-wrap gap-1">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="material-symbols-outlined text-[18px] {cfg.color}">{cfg.icon}</span>
              <span class="text-xs text-[var(--color-text)]">#{task.id?.slice(0, 8) || ''}</span>
              <span class="text-xs px-2 py-0.5 rounded-full flex items-center gap-1 whitespace-nowrap" style="background: var(--color-surface); color: var(--color-text-muted)">
                <span class="material-symbols-outlined text-[12px]">{triggerIcons[task.trigger || ''] || 'build'}</span>
                {task.trigger === 'manual' ? '手动' : task.trigger === 'git' ? 'Git' : task.trigger === 'schedule' ? '定时' : task.trigger || '手动'}
              </span>
            </div>
            <div class="flex items-center gap-2 flex-wrap">
              {#if task.commit_hash}
                <span class="text-xs font-mono" style="color: var(--color-text-muted)">{task.commit_hash.slice(0, 7)}</span>
              {/if}
              <span class="text-xs" style="color: var(--color-text-muted)">{new Date(task.created_at || task.timestamp).toLocaleString('zh-CN')}</span>
              {#if task.status === 'running' || task.status === 'pending'}
                <button class="p-1 rounded hover:bg-[var(--color-surface)]" onclick={(e) => { e.stopPropagation(); onSelectBuild?.({ ...task, _cancel: true }); }}>
                  <span class="material-symbols-outlined text-[16px] text-[var(--color-error)]">cancel</span>
                </button>
              {/if}
              {#if task.status !== 'running' && task.status !== 'pending'}
                <button class="p-1 rounded hover:bg-[var(--color-surface)]" onclick={(e) => onDeleteBuild?.(task.id, e)}>
                  <span class="material-symbols-outlined text-[16px] text-[var(--color-text-muted)] hover:text-[var(--color-error)]">delete</span>
                </button>
              {/if}
            </div>
          </div>
        </div>
      {/each}
    </div>
  </div>
{/if}

<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';
  let {
    buildHistory = [],
    projectId = '',
    onSelectBuild,
    onDeleteBuild,
    onDeleteFailedBuilds,
    onRefresh,
    onPublishRelease,
  }: {
    buildHistory?: { id: string; status: string; timestamp: string; branch: string; target: string; version: string; trigger?: string; commit_hash?: string; created_at?: string; artifact_path?: string; _cancel?: boolean }[];
    projectId?: string;
    onSelectBuild?: (task: { id: string; _cancel?: boolean; status?: string; log?: string }) => void;
    onDeleteBuild?: (buildId: string, e: Event) => void;
    onDeleteFailedBuilds?: () => void;
    onRefresh?: () => void;
    onPublishRelease?: (buildId: string) => void;
  } = $props();

  let publishingId = $state<string | null>(null);
  let selectionMode = $state(false);
  let selectedIds = $state<Set<string>>(new Set());
  let deleting = $state(false);

  const statusConfig: Record<string, { color: string; bg: string; icon: string }> = {
    pending: { color: 'text-[var(--color-warning)]', bg: 'bg-[var(--color-warning-light)]', icon: 'schedule' },
    running: { color: 'text-[var(--color-info)]', bg: 'bg-[var(--color-info-light)]', icon: 'sync' },
    success: { color: 'text-[var(--color-success)]', bg: 'bg-[var(--color-success-light)]', icon: 'check_circle' },
    failed: { color: 'text-[var(--color-error)]', bg: 'bg-[var(--color-error-light)]', icon: 'error' },
    cancelled: { color: 'text-[var(--color-text-muted)]', bg: 'bg-[var(--color-surface)]', icon: 'cancel' },
  };

  const triggerIcons: Record<string, string> = { manual: 'build', git: 'cloud_upload', push: 'cloud_upload', schedule: 'schedule' };

  function toggleSelect(id: string) {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selectedIds = next;
  }

  function selectAll() {
    if (selectedIds.size === buildHistory.length) {
      selectedIds = new Set();
    } else {
      selectedIds = new Set(buildHistory.map(b => b.id));
    }
  }

  async function deleteSelected() {
    if (selectedIds.size === 0) return;
    if (!confirm(`确定删除 ${selectedIds.size} 条构建记录？`)) return;
    deleting = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}/builds/delete`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({ ids: Array.from(selectedIds) }),
      });
      if (res.ok) {
        const data = await res.json();
        buildHistory = buildHistory.filter(b => !selectedIds.has(b.id));
        selectedIds = new Set();
        selectionMode = false;
        // @ts-ignore
        toast(`已删除 ${data.deleted || selectedIds.size} 条记录`, 'success', 3000);
      } else {
        const body = await res.json().catch(() => ({ error: '未知错误' }));
        // @ts-ignore
        toast(`删除失败: ${body.error || res.statusText}`, 'error', 5000);
      }
    } catch (e: any) {
      // @ts-ignore
      toast(`删除失败: ${e.message || '网络错误'}`, 'error', 5000);
    } finally {
      deleting = false;
    }
  }
</script>

{#if buildHistory.length > 0}
  <div class="mt-8">
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-sm font-semibold text-[var(--color-text)] flex items-center gap-2">
        <span class="material-symbols-outlined text-[18px]">history</span>
        构建历史
        {#if selectionMode && selectedIds.size > 0}
          <span class="text-xs px-2 py-0.5 rounded-full" style="background: var(--color-primary-light); color: var(--color-primary)">已选 {selectedIds.size}</span>
        {/if}
      </h3>
      <div class="flex items-center gap-2">
        {#if selectionMode}
          <button
            class="px-3 py-1 rounded-lg text-xs font-medium transition-colors"
            style="border: 1px solid var(--color-border); color: var(--color-text-secondary)"
            onclick={selectAll}
          >
            {selectedIds.size === buildHistory.length ? '取消全选' : '全选'}
          </button>
          {#if selectedIds.size > 0}
            <button
              class="px-3 py-1 rounded-lg text-xs font-medium transition-colors text-[var(--color-error)]"
              style="border: 1px solid var(--color-error); color: var(--color-error)"
              onclick={deleteSelected}
              disabled={deleting}
            >
              {deleting ? '删除中...' : `删除 (${selectedIds.size})`}
            </button>
          {/if}
          <button
            class="px-3 py-1 rounded-lg text-xs font-medium transition-colors"
            style="border: 1px solid var(--color-border); color: var(--color-text-secondary)"
            onclick={() => { selectionMode = false; selectedIds = new Set(); }}
          >
            取消
          </button>
        {:else}
          {#if buildHistory.some(b => b.status === 'failed')}
            <button class="px-3 py-1 rounded-lg text-xs font-medium transition-colors text-[var(--color-error)]" style="border: 1px solid var(--color-border); color: var(--color-error)" onclick={onDeleteFailedBuilds}>清除失败</button>
          {/if}
          <button
            class="px-3 py-1 rounded-lg text-xs font-medium transition-colors"
            style="border: 1px solid var(--color-border); color: var(--color-text-secondary)"
            onclick={() => selectionMode = true}
          >
            多选
          </button>
          <button class="px-3 py-1 rounded-lg text-xs font-medium transition-colors" style="border: 1px solid var(--color-border); color: var(--color-text-secondary)" onclick={onRefresh}>刷新</button>
        {/if}
      </div>
    </div>
    <div class="space-y-2">
      {#each buildHistory as task}
        {@const cfg = statusConfig[task.status] || statusConfig.pending}
        <div
          role="button"
          tabindex="0"
          class="p-3 rounded-xl border cursor-pointer transition-colors"
          style="border-color: {selectionMode && selectedIds.has(task.id) ? 'var(--color-primary)' : 'var(--color-border)'}; background: {selectionMode && selectedIds.has(task.id) ? 'var(--color-primary-light)' : 'var(--color-bg-elevated)'}"
          onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); (e.currentTarget as HTMLElement).click(); } }}
          onclick={() => { if (selectionMode) { toggleSelect(task.id); } else { onSelectBuild?.(task); } }}
        >
          <div class="flex items-center justify-between flex-wrap gap-1">
            <div class="flex items-center gap-2 flex-wrap">
              {#if selectionMode}
                <input
                  type="checkbox"
                  checked={selectedIds.has(task.id)}
                  class="w-4 h-4 rounded accent-[var(--color-primary)]"
                  onclick={(e) => e.stopPropagation()}
                  onchange={() => toggleSelect(task.id)}
                />
              {/if}
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
              {#if !selectionMode}
                {#if task.status === 'running' || task.status === 'pending'}
                  <button class="p-1 rounded hover:bg-[var(--color-surface)]" onclick={(e) => { e.stopPropagation(); onSelectBuild?.({ ...task, _cancel: true }); }}>
                    <span class="material-symbols-outlined text-[16px] text-[var(--color-error)]">cancel</span>
                  </button>
                {/if}
                {#if task.status !== 'running' && task.status !== 'pending'}
                  {#if task.status === 'success' && task.artifact_path && onPublishRelease}
                    <button
                      class="p-1 rounded hover:bg-[var(--color-success-light)] transition-colors"
                      title="发布到 GitHub Release"
                      disabled={publishingId === task.id}
                      onclick={(e) => { e.stopPropagation(); publishingId = task.id; onPublishRelease(task.id); setTimeout(() => { publishingId = null; }, 3000); }}
                    >
                      <span class="material-symbols-outlined text-[16px] text-[var(--color-success)]">{publishingId === task.id ? 'sync' : 'rocket_launch'}</span>
                    </button>
                  {/if}
                  <button class="p-1 rounded hover:bg-[var(--color-surface)]" onclick={(e) => onDeleteBuild?.(task.id, e)}>
                    <span class="material-symbols-outlined text-[16px] text-[var(--color-text-muted)] hover:text-[var(--color-error)]">delete</span>
                  </button>
                {/if}
              {/if}
            </div>
          </div>
        </div>
      {/each}
    </div>
  </div>
{/if}

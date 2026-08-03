<script lang="ts">
let {
  projectId = '',
  projectName = '',
  fileCount = 0,
  collapsed = false,
  onToggleCollapse,
}: {
  projectId: string;
  projectName: string;
  fileCount: number;
  collapsed: boolean;
  onToggleCollapse: () => void;
} = $props();

function openProject() {
  window.history.pushState(null, '', '/projects/' + projectId);
  window.dispatchEvent(new PopStateEvent('popstate'));
}
</script>

{#if projectId}
  <div class="px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)]" style="animation: fadeInUp 0.3s ease-out;">
    <button class="flex items-center gap-2 w-full text-left" onclick={onToggleCollapse}>
      <span class="material-symbols-outlined text-[14px] text-primary-500">folder</span>
      <span class="text-xs font-semibold text-[var(--color-text)] flex-1 truncate">{projectName || '未命名项目'}</span>
      <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-primary-500/20 text-primary-600 font-medium">{fileCount} 个文件</span>
      <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)] transition-transform {collapsed ? '' : 'rotate-180'}">expand_less</span>
    </button>
    {#if !collapsed}
      <div class="mt-2 flex gap-2">
        <button class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-[11px] font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors" onclick={openProject}>
          <span class="material-symbols-outlined text-[12px]">edit</span>
          打开编辑器
        </button>
        <button class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-[11px] font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text); border: 1px solid var(--color-border);" onclick={openProject}>
          <span class="material-symbols-outlined text-[12px]">open_in_new</span>
          查看项目
        </button>
      </div>
    {/if}
  </div>
{/if}

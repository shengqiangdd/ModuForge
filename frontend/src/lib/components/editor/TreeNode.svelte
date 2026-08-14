<script lang="ts">
  import TreeNode from './TreeNode.svelte';
  interface TreeNodeData {
    name: string;
    path: string;
    type: 'file' | 'directory';
    size?: number;
    modified?: string;
    children?: TreeNodeData[];
  }

  let {
    node,
    selectedFile = null,
    expandedDirs,
    isExpanded,
    onToggleDir,
    onSelect,
    onDelete,
    getFileIcon,
    getFileIconColor,
  }: {
    node: TreeNode;
    selectedFile?: string | null;
    expandedDirs: Set<string>;
    isExpanded?: boolean;
    onToggleDir?: (path: string) => void;
    onSelect?: (path: string) => void;
    onDelete?: (path: string) => void;
    getFileIcon?: (path: string) => string;
    getFileIconColor?: (path: string) => string;
  } = $props();

  let dirExpanded = $derived(isExpanded ?? expandedDirs.has(node.path));
  let isDir = $derived(node.type === 'directory');

  function formatSize(bytes?: number): string {
    if (bytes == null) return '';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  function getDirFileCount(n: TreeNodeData): number {
    if (!n.children) return 0;
    let count = 0;
    for (const child of n.children) {
      if (child.type === 'file') count++;
      else count += getDirFileCount(child);
    }
    return count;
  }

  function getIcon(n: TreeNodeData): string {
    if (n.type === 'directory') return dirExpanded ? 'folder_open' : 'folder';
    return getFileIcon?.(n.path) || 'description';
  }

  function getIconColor(n: TreeNodeData): string {
    if (n.type === 'directory') return 'var(--color-warning)';
    return getFileIconColor?.(n.path) || 'var(--color-text-muted)';
  }
</script>

{#if isDir}
  <div>
    <button
      class="w-full flex items-center gap-2 px-2 py-1.5 rounded-lg text-sm text-left transition-all duration-150 text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] cursor-pointer"
      onclick={() => onToggleDir?.(node.path)}
    >
      <span class="material-symbols-outlined text-[14px] transition-transform duration-150" style="transform: rotate({dirExpanded ? '90' : '0'}deg); color: var(--color-text-muted)">chevron_right</span>
      <span class="material-symbols-outlined text-[16px] flex-shrink-0" style="color: {getIconColor(node)}">{getIcon(node)}</span>
      <span class="truncate flex-1 text-[var(--color-text)] font-medium text-xs">{node.name}</span>
      <span class="text-[10px] text-[var(--color-text-muted)]">{getDirFileCount(node)}</span>
    </button>
    {#if dirExpanded && node.children}
      <div class="ml-4 pl-2 border-l border-[var(--color-border)]">
        {#each node.children as child (child.path)}
          <TreeNode
            node={child}
            {selectedFile}
            {expandedDirs}
            isExpanded={expandedDirs.has(child.path)}
            {onToggleDir}
            {onSelect}
            {onDelete}
            {getFileIcon}
            getFileIconColor={getFileIconColor}
          />
        {/each}
      </div>
    {/if}
  </div>
{:else}
  <div class="group flex items-center">
    <button
      class="flex-1 flex items-center gap-2 px-2 py-1.5 rounded-lg text-sm transition-all duration-150 text-left {selectedFile === node.path ? 'bg-[var(--gradient-brand-subtle)] text-[var(--color-primary)] font-medium' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'} cursor-pointer"
      onclick={() => { onSelect?.(node.path); }}
    >
      <span class="w-[14px]"></span>
      <span class="material-symbols-outlined text-[16px] flex-shrink-0" style="color: {getIconColor(node)}">{getIcon(node)}</span>
      <span class="truncate text-xs">{node.name}</span>
      {#if node.size != null}
        <span class="ml-auto text-[10px] text-[var(--color-text-muted)]">{formatSize(node.size)}</span>
      {/if}
      {#if selectedFile === node.path}
        <div class="w-1.5 h-1.5 rounded-full" style="background: var(--gradient-brand)"></div>
      {/if}
    </button>
    <button
      class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-[var(--color-error-light)] transition-opacity mr-1 cursor-pointer"
      title="删除文件"
      onclick={(e) => { e.stopPropagation(); onDelete?.(node.path); }}
    >
      <span class="material-symbols-outlined text-[14px] text-[var(--color-error)]">delete</span>
    </button>
  </div>
{/if}

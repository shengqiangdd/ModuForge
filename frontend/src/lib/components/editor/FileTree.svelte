<script lang="ts">
  let {
    files = [],
    selectedFile = null,
    project = null,
    sidebarOpen = true,
    dragOver = false,
    uploadProgress = null,
    showFileSearch = false,
    fileSearchQuery = '',
    fileSearchInput = undefined,
    filteredFiles = [],
    onSelect,
    onDelete,
    onToggleSidebar,
    onOpenFileSearch,
    onFileSearchInput,
    onFileSearchKeydown,
    onSelectFileFromSearch,
    onCloseFileSearch,
    getFileIcon,
    getFileIconColor,
    onDrop,
    onDragOver,
    onDragLeave,
  }: {
    files?: { id?: number; path: string; content?: string }[];
    selectedFile?: string | null;
    project?: { id: string; name: string; path: string } | null;
    sidebarOpen?: boolean;
    dragOver?: boolean;
    uploadProgress?: string | null;
    showFileSearch?: boolean;
    fileSearchQuery?: string;
    fileSearchInput?: HTMLInputElement;
    filteredFiles?: { id?: number; path: string; content?: string }[];
    onSelect?: (path: string) => void;
    onDelete?: (path: string) => void;
    onToggleSidebar?: () => void;
    onOpenFileSearch?: () => void;
    onFileSearchInput?: (e: Event) => void;
    onFileSearchKeydown?: (e: KeyboardEvent) => void;
    onSelectFileFromSearch?: (path: string) => void;
    onCloseFileSearch?: () => void;
    getFileIcon?: (path: string) => string;
    getFileIconColor?: (path: string) => string;
    onDrop?: (e: DragEvent) => void;
    onDragOver?: (e: DragEvent) => void;
    onDragLeave?: () => void;
  } = $props();
</script>

{#if !sidebarOpen}
  <button
    class="fixed left-2 top-20 z-10 w-9 h-9 rounded-xl bg-[var(--color-bg-elevated)] border border-[var(--color-border)] flex items-center justify-center shadow-md hover:bg-[var(--color-surface)] hover:shadow-lg transition-all"
    onclick={onToggleSidebar}
    title="展开侧边栏"
  >
    <span class="material-symbols-outlined text-[18px]">menu</span>
  </button>
{/if}

<aside
  class="border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col flex-shrink-0 transition-all duration-200
    {sidebarOpen ? 'w-60 max-md:fixed max-md:top-0 max-md:bottom-16 max-md:left-0 max-md:z-20 max-md:shadow-elevated-lg' : 'w-0 max-md:hidden overflow-hidden border-r-0'}"
  ondragover={onDragOver}
  ondragleave={onDragLeave}
  ondrop={onDrop}
>
  <div class="px-4 h-12 flex items-center border-b border-[var(--color-border)] min-w-[240px]">
    <h3 class="text-sm font-semibold text-[var(--color-text)] truncate flex-1">{project?.name || '项目'}</h3>
    <button class="flex items-center justify-center w-7 h-7 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={onOpenFileSearch} title="搜索文件 (Ctrl+P)">
      <span class="material-symbols-outlined !text-[16px]">search</span>
    </button>
    <button class="flex items-center justify-center w-7 h-7 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors ml-1" onclick={onToggleSidebar} title="折叠侧边栏">
      <span class="material-symbols-outlined !text-[16px]">menu_open</span>
    </button>
    <span class="text-xs text-[var(--color-text-muted)] ml-1">{files.length}</span>
  </div>
  <div class="flex-1 overflow-y-auto p-2 space-y-0.5 relative" class:drag-over={dragOver}>
    {#if dragOver}
      <div class="absolute inset-0 z-10 flex items-center justify-center rounded-xl pointer-events-none" style="background: color-mix(in srgb, var(--color-primary) 8%, transparent); border: 2px dashed var(--color-primary);">
        <div class="text-center">
          <span class="material-symbols-outlined text-3xl block mb-2" style="color: var(--color-primary)">cloud_upload</span>
          <p class="text-sm font-medium" style="color: var(--color-primary)">拖放到这里上传</p>
        </div>
      </div>
    {/if}
    {#if uploadProgress}
      <div class="px-3 py-2 text-xs text-[var(--color-primary)]">{uploadProgress}</div>
    {/if}
    {#each files as file}
      <div class="group flex items-center">
        <button
          class="flex-1 flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all duration-200 text-left {selectedFile === file.path ? 'bg-[var(--gradient-brand-subtle)] text-[var(--color-primary)] font-medium' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
          onclick={() => { onSelect?.(file.path); if (typeof window !== 'undefined' && window.innerWidth < 768) onToggleSidebar?.(); }}
        >
          <span class="material-symbols-outlined text-[16px] flex-shrink-0" style="color: {getFileIconColor?.(file.path) || 'var(--color-text-muted)'}">{getFileIcon?.(file.path) || 'description'}</span>
          <span class="truncate">{file.path.split('/').pop()}</span>
          {#if selectedFile === file.path}
            <div class="ml-auto w-1.5 h-1.5 rounded-full" style="background: var(--gradient-brand)"></div>
          {/if}
        </button>
        <button
          class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-[var(--color-error-light)] transition-opacity mr-1 cursor-pointer"
          title="删除文件"
          onclick={(e) => { e.stopPropagation(); onDelete?.(file.path); }}
        >
          <span class="material-symbols-outlined text-[14px] text-[var(--color-error)]">delete</span>
        </button>
      </div>
    {:else}
      <div class="text-center py-12">
        <span class="material-symbols-outlined text-4xl mb-2" style="color: var(--color-text-muted)">folder_open</span>
        <p class="text-xs" style="color: var(--color-text-muted)">暂无文件</p>
      </div>
    {/each}
  </div>
  <div class="px-3 py-2 border-t border-[var(--color-border)]">
    <div class="text-[10px] text-[var(--color-text-muted)] flex items-center gap-3">
      <span><kbd class="px-1 py-0.5 rounded bg-[var(--color-surface)]">⌘P</kbd> 搜索</span>
      <span><kbd class="px-1 py-0.5 rounded bg-[var(--color-surface)]">⌘S</kbd> 保存</span>
    </div>
  </div>
</aside>

{#if sidebarOpen}
  <div class="fixed inset-0 bg-black/20 z-10 max-md:block md:hidden" role="presentation" onclick={onToggleSidebar}></div>
{/if}

<!-- File Search Overlay -->
{#if showFileSearch}
  <div class="fixed inset-0 z-50 flex items-start justify-center pt-24" style="background: rgba(0,0,0,0.4); backdrop-filter: blur(4px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onCloseFileSearch?.(); }}>
    <div class="w-96 rounded-xl shadow-2xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg)]" role="presentation" onclick={(e) => e.stopPropagation()}>
      <div class="px-3 py-2 border-b border-[var(--color-border)]">
        <input
          bind:this={fileSearchInput}
          type="text"
          class="w-full px-3 py-2 rounded-lg text-sm bg-[var(--color-bg)] border border-[var(--color-border)] text-[var(--color-text)] outline-none focus:border-primary-500"
          placeholder="搜索文件..."
          value={fileSearchQuery}
          oninput={onFileSearchInput}
          onkeydown={onFileSearchKeydown}
        />
      </div>
      {#if filteredFiles.length > 0}
        <div class="max-h-72 overflow-y-auto p-2 space-y-0.5">
          {#each filteredFiles as file}
            <button
              class="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs text-left transition-colors text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]"
              onclick={() => onSelectFileFromSearch?.(file.path)}
            >
              <span class="material-symbols-outlined text-[14px]" style="color: {getFileIconColor?.(file.path) || 'var(--color-text-muted)'}">{getFileIcon?.(file.path) || 'description'}</span>
              <span class="truncate flex-1">{file.path}</span>
              <span class="text-[10px] text-[var(--color-text-muted)]">{file.path.split('/').pop()}</span>
            </button>
          {/each}
        </div>
        <div class="px-3 py-1.5 border-t border-[var(--color-border)] text-[10px] text-[var(--color-text-muted)] flex items-center justify-between">
          <span>{filteredFiles.length} 个文件</span>
          <span><kbd class="px-1 py-0.5 rounded bg-[var(--color-surface)]">Esc</kbd> 关闭</span>
        </div>
      {:else}
        <div class="px-4 py-6 text-center text-xs text-[var(--color-text-muted)]">未找到匹配文件</div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .drag-over { position: relative; min-height: 100px; }
</style>

<script lang="ts">
  import { onMount } from 'svelte';
  import { client, authFetch } from '$lib/api/client';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Terminal from '$lib/components/Terminal.svelte';
  import UndoHistory from '$lib/components/UndoHistory.svelte';
  import FileComments from '$lib/components/FileComments.svelte';
  import CollaborationCursors from '$lib/components/CollaborationCursors.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { historyStore } from '$lib/stores/history';
  import { loadShortcuts, matchShortcut, shortcutLabel } from '$lib/stores/shortcuts';

  let { projectId = '' }: { projectId?: string } = $props();

  let files = $state<{ id?: number; path: string; content?: string }[]>([]);
  let selectedFile = $state<string | null>(null);
  let editorContent = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let project = $state<any>(null);

  let openTabs = $state<string[]>([]);
  let activeTab = $state<string | null>(null);

  let sidebarOpen = $state(true);

  // File search
  let showFileSearch = $state(false);
  let fileSearchQuery = $state('');
  let fileSearchInput: HTMLInputElement | undefined = $state();
  let filteredFiles = $derived(
    fileSearchQuery
      ? files.filter(f => f.path.toLowerCase().includes(fileSearchQuery.toLowerCase()))
      : files
  );

  function onFileSearchInput(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    // Debounce: update query after 150ms of no typing
    if (_fileSearchTimer) clearTimeout(_fileSearchTimer);
    _fileSearchTimer = setTimeout(() => { fileSearchQuery = val; }, 150);
  }
  let _fileSearchTimer: ReturnType<typeof setTimeout> | null = null;

  // Diff view
  let diffMode = $state(false);
  let diffFiles = $state<{path: string; current: string; incoming: string}[]>([]);
  let selectedDiffFile = $state<string | null>(null);
  let showDiffList = $state(false);

  // Security scan
  let securityScanning = $state(false);
  let securityResult = $state<any>(null);
  let showSecurityPanel = $state(false);

  // Formatting
  let formatting = $state(false);

  interface SecurityIssue {
    severity: string;
    file: string;
    line: number;
    rule: string;
    message: string;
  }
  interface SecurityScanResult {
    safe: boolean;
    issues: SecurityIssue[];
    score: number;
    summary: string;
  }

  async function formatCode() {
    if (!projectId) return;
    formatting = true;
    try {
      const res = await authFetch(`/api/v1/projects/${projectId}/format`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      if (res.ok) {
        const d = await res.json();
        toast(`格式化完成: ${d.success} 成功, ${d.failed} 失败, ${d.skipped} 跳过`, d.failed > 0 ? 'warning' : 'success');
        loadFiles();
      } else {
        toast('格式化失败', 'error');
      }
    } catch (e: any) {
      toast(e.message || '格式化失败', 'error');
    }
    formatting = false;
  }

  async function runSecurityScan() {
    if (!projectId) return;
    securityScanning = true;
    securityResult = null;
    showSecurityPanel = true;
    try {
      const res = await authFetch(`/api/v1/security/scan-project/${projectId}`, { method: 'POST' });
      if (!res.ok) { throw new Error('scan failed'); }
      securityResult = await res.json();
      if (securityResult.safe) {
        toast('安全扫描通过', 'success');
      } else {
        toast(`安全评分: ${securityResult.score}/100`, securityResult.score < 50 ? 'error' : 'warning');
      }
    } catch (e: any) {
      toast(e.message || '安全扫描失败', 'error');
      securityResult = null;
    } finally {
      securityScanning = false;
    }
  }

  function getSecurityIcon(): string {
    if (!securityResult) return 'security';
    return securityResult.safe ? 'verified' : 'warning';
  }
  function getSecurityColor(): string {
    if (!securityResult) return 'var(--color-text-muted)';
    return securityResult.safe ? '#22c55e' : '#ef4444';
  }
  function getIssueIcon(severity: string): string {
    return severity === 'critical' ? 'error' : severity === 'warning' ? 'warning' : 'info';
  }
  function getIssueColor(severity: string): string {
    return severity === 'critical' ? '#ef4444' : severity === 'warning' ? '#f59e0b' : '#6b7280';
  }

  // Terminal
  let showTerminal = $state(false);
  let terminalHeight = $state(200);

  // Drag-and-drop upload
  let dragOver = $state(false);
  let uploadProgress = $state<string | null>(null);

  async function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
    if (!e.dataTransfer?.files.length) return;
    const form = new FormData();
    for (const f of e.dataTransfer.files) {
      form.append('files', f, f.webkitRelativePath || f.name);
    }
    uploadProgress = '上传中...';
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}/files/upload`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
        body: form,
      });
      if (res.ok) {
        const data = await res.json();
        const okCount = data.results?.filter((r: string) => r.endsWith(': ok')).length || 0;
        toast(`上传完成: ${okCount} 个文件`, 'success');
        historyStore.push({ type: 'file_upload', data: { count: okCount } });
        loadFiles();
      } else {
        const err = await res.json().catch(() => ({ error: 'upload failed' }));
        toast(err.error || '上传失败', 'error');
      }
    } catch (e: any) {
      toast(e.message || '上传失败', 'error');
    }
    uploadProgress = null;
  }

  // Undo/Redo
  let showUndoHistory = $state(false);

  function handleUndo() {
    const action = historyStore.undo();
    if (!action) { toast('没有可撤销的操作', 'info'); return; }
    toast(`撤销: ${action.type}`, 'info');
    loadFiles();
  }

  function handleRedo() {
    const action = historyStore.redo();
    if (!action) { toast('没有可重做的操作', 'info'); return; }
    toast(`重做: ${action.type}`, 'info');
    loadFiles();
  }

  async function execTerminalCommand(cmd: string): Promise<string> {
    const serial = localStorage.getItem('moduforge_adb_serial') || '';
    if (!serial) return 'No device selected. Go to Devices page first.';
    try {
      const res = await fetch(`/api/v1/adb/exec`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('moduforge_token')}`,
        },
        body: JSON.stringify({ serial, command: cmd }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        return `Error: ${err.error || res.statusText}`;
      }
      const data = await res.json();
      return data.output || '(empty output)';
    } catch (e: any) {
      return `Error: ${e.message || 'Request failed'}`;
    }
  }

  // Keyboard shortcuts
  let shortcuts = $state(loadShortcuts());

  function handleKeydown(e: KeyboardEvent) {
    for (const sc of shortcuts) {
      if (!matchShortcut(e, sc)) continue;
      e.preventDefault();
      switch (sc.id) {
        case 'save': saveFile(); break;
        case 'search-file': openFileSearch(); break;
        case 'toggle-terminal': showTerminal = !showTerminal; break;
        case 'undo': handleUndo(); break;
        case 'redo': handleRedo(); break;
        case 'format-code': formatCode(); break;
      }
      return;
    }
  }

  onMount(() => {
    document.addEventListener('keydown', handleKeydown);
    void (async () => {
      if (!projectId) { loading = false; return; }
      try {
        const [p, fileData] = await Promise.all([
          client.get<any>(`/projects/${projectId}`),
          client.get<{ id?: number; path: string }[]>(`/projects/${projectId}/files`),
        ]);
        project = p;
        files = (fileData || []).map(f => ({ ...f, path: f.path }));
      } catch (e: any) {
        toast(e.message || '加载项目失败', 'error');
      } finally {
        loading = false;
      }
    })();
    return () => document.removeEventListener('keydown', handleKeydown);
  });

  function openFileSearch() {
    fileSearchQuery = '';
    filteredFiles = files;
    showFileSearch = true;
    setTimeout(() => fileSearchInput?.focus(), 50);
  }

  function selectFileFromSearch(path: string) {
    showFileSearch = false;
    selectFile(path);
  }

  function closeFileSearch() {
    showFileSearch = false;
    fileSearchQuery = '';
  }

  function handleFileSearchKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') closeFileSearch();
    if (e.key === 'Enter' && filteredFiles.length > 0) {
      selectFileFromSearch(filteredFiles[0].path);
    }
  }

  async function selectFile(path: string) {
    if (selectedFile === path && editorContent) return;
    selectedFile = path;
    if (!openTabs.includes(path)) openTabs = [...openTabs, path];
    activeTab = path;
    const existing = files.find(f => f.path === path);
    if (existing?.content !== undefined) {
      editorContent = existing.content;
      return;
    }
    try {
      const file = await client.get<{ path: string; content: string }>(`/projects/${projectId}/files/${encodeURIComponent(path)}`);
      editorContent = file.content;
      const idx = files.findIndex(f => f.path === path);
      if (idx >= 0) files[idx] = { ...files[idx], content: file.content };
    } catch {
      editorContent = '';
    }
  }

  function switchTab(path: string) {
    activeTab = path;
    selectFile(path);
  }

  function closeTab(path: string) {
    openTabs = openTabs.filter(t => t !== path);
    if (activeTab === path) {
      activeTab = openTabs[openTabs.length - 1] || null;
      if (activeTab) selectFile(activeTab);
      else { selectedFile = null; editorContent = ''; }
    }
  }

  async function saveFile() {
    if (!selectedFile || !projectId) return;
    saving = true;
    try {
      const resp = await client.put(`/projects/${projectId}/files/${encodeURIComponent(selectedFile)}`, { content: editorContent });
      const idx = files.findIndex(f => f.path === selectedFile);
      if (idx >= 0) files[idx] = { ...files[idx], content: editorContent };
      historyStore.push({ type: 'file_save', data: { path: selectedFile } });
      toast('文件保存成功', 'success');

      // Show validation warnings if present
      const data = resp as any;
      if (data?.validation) {
        const v = data.validation;
        if (v.errors && v.errors.length > 0) {
          for (const issue of v.errors) {
            toast(`❌ ${issue.message}`, 'error');
          }
        }
        if (v.warnings && v.warnings.length > 0) {
          for (const issue of v.warnings) {
            toast(`⚠️ ${issue.message}`, 'warning');
          }
        }
      }
    } catch (e: any) {
      toast(e.message || '保存失败', 'error');
    } finally {
      saving = false;
    }
  }

  async function validateProject() {
    if (!projectId) return;
    try {
      const resp = await client.post(`/projects/${projectId}/validate`) as any;
      if (resp?.errors && resp.errors.length > 0) {
        for (const issue of resp.errors) {
          toast(`❌ ${issue.message}`, 'error');
        }
      } else if (resp?.warnings && resp.warnings.length > 0) {
        for (const issue of resp.warnings) {
          toast(`⚠️ ${issue.message}`, 'warning');
        }
      } else {
        toast('✅ 项目完整性校验通过，未发现缺失文件', 'success');
      }
    } catch (e: any) {
      toast(e.message || '校验失败', 'error');
    }
  }

  async function loadFiles() {
    if (!projectId) return;
    try {
      const fileData = await client.get<{ id?: number; path: string }[]>(`/projects/${projectId}/files`);
      files = (fileData || []).map(f => ({ ...f, path: f.path }));
    } catch {}
  }

  // File search — memoized icon lookup caches
  const _iconCache = new Map<string, string>();
  const _iconColorCache = new Map<string, string>();

  function detectLanguage(path: string): string {
    const ext = path.split('.').pop()?.toLowerCase() || '';
    const map: Record<string, string> = {
      js: 'javascript', jsx: 'javascript', ts: 'javascript', tsx: 'javascript',
      py: 'python', html: 'html', htm: 'html', css: 'css', scss: 'css',
      json: 'json', xml: 'xml', yaml: 'json', yml: 'json', sh: 'shell', bash: 'shell',
    };
    return map[ext] || 'javascript';
  }

  function getFileIcon(path: string): string {
    const ext = path.split('.').pop()?.toLowerCase() || '';
    if (_iconCache.has(ext)) return _iconCache.get(ext)!;
    const iconMap: Record<string, string> = {
      js: 'javascript', jsx: 'javascript', ts: 'javascript', tsx: 'javascript',
      py: 'python', html: 'html', htm: 'html', css: 'css', scss: 'css',
      json: 'data_object', xml: 'code', yaml: 'code', yml: 'code',
      sh: 'terminal', bash: 'terminal',
      md: 'description', txt: 'description', log: 'description',
      png: 'image', jpg: 'image', jpeg: 'image', gif: 'image', svg: 'image',
      zip: 'folder_zip', tar: 'folder_zip', gz: 'folder_zip',
      prop: 'settings', mk: 'build',
    };
    const icon = iconMap[ext] || 'description';
    _iconCache.set(ext, icon);
    return icon;
  }

  function getFileIconColor(path: string): string {
    const ext = path.split('.').pop()?.toLowerCase() || '';
    if (_iconColorCache.has(ext)) return _iconColorCache.get(ext)!;
    const colorMap: Record<string, string> = {
      js: '#f7df1e', jsx: '#61dafb', ts: '#3178c6', tsx: '#61dafb',
      py: '#3776ab', html: '#e34f26', css: '#1572b6',
      json: '#292929', sh: '#4eaa25', bash: '#4eaa25',
      md: '#ffffff', prop: '#8b5cf6',
    };
    const color = colorMap[ext] || 'var(--color-text-muted)';
    _iconColorCache.set(ext, color);
    return color;
  }

  function handleEditorChange(val: string) {
    editorContent = val;
  }

  // Diff view
  function openDiffView(files: {path: string; current: string; incoming: string}[]) {
    diffFiles = files;
    selectedDiffFile = files.length > 0 ? files[0].path : null;
    diffMode = true;
    showDiffList = true;
  }

  function closeDiffView() {
    diffMode = false;
    showDiffList = false;
    diffFiles = [];
    selectedDiffFile = null;
  }

  function getCurrentDiffContent(): string {
    if (!selectedDiffFile) return '';
    const df = diffFiles.find(f => f.path === selectedDiffFile);
    return df?.current || '';
  }

  function getIncomingDiffContent(): string {
    if (!selectedDiffFile) return '';
    const df = diffFiles.find(f => f.path === selectedDiffFile);
    return df?.incoming || '';
  }
</script>

<style>
  .file-tree-item {
    color: var(--color-text-secondary);
  }
  .file-tree-item:hover {
    background: var(--color-surface);
  }
  .file-tree-item.active {
    background: var(--gradient-brand-subtle);
    color: var(--color-primary);
    font-weight: 500;
  }
  .diff-file-item {
    transition: all 0.15s ease;
  }
  .diff-file-item:hover {
    background: var(--color-surface);
  }
  .diff-file-item.active {
    background: var(--gradient-brand-subtle);
    color: var(--color-primary);
  }
  .terminal-resize-handle {
    height: 3px;
    background: var(--color-border);
    transition: background 0.15s;
    position: relative;
  }
  .terminal-resize-handle:hover,
  .terminal-resize-handle:active {
    background: var(--color-primary);
  }
  .terminal-container {
    flex-shrink: 0;
    overflow: hidden;
  }
  .drag-over {
    position: relative;
    min-height: 100px;
  }
</style>

<div class="flex flex-1 h-full overflow-hidden">
  {#if !projectId}
    <div class="flex-1 flex items-center justify-center text-[var(--color-text-secondary)]">
      <div class="text-center">
        <span class="material-symbols-outlined text-5xl mb-3 text-neutral-300">code_blocks</span>
        <p class="text-lg font-medium">选择或创建一个项目开始编辑</p>
        <p class="text-xs text-[var(--color-text-muted)] mt-1">{(shortcuts.find(s => s.id === 'search-file') ? shortcutLabel(shortcuts.find(s => s.id === 'search-file')!) : 'Ctrl+P')} 快速搜索文件 · {(shortcuts.find(s => s.id === 'save') ? shortcutLabel(shortcuts.find(s => s.id === 'save')!) : 'Ctrl+S')} 保存</p>
      </div>
    </div>
  {:else if loading}
    <div class="flex-1 flex items-center justify-center">
      <div class="flex flex-col items-center gap-3">
        <div class="animate-spin h-6 w-6 border-2 border-primary-500 border-t-transparent rounded-full"></div>
        <span class="text-sm text-[var(--color-text-secondary)]">加载项目中...</span>
      </div>
    </div>
  {:else}
    <!-- Sidebar Toggle -->
    {#if !sidebarOpen}
      <button
        class="fixed left-2 top-20 z-10 w-9 h-9 rounded-xl bg-[var(--color-bg-elevated)] border border-[var(--color-border)] flex items-center justify-center shadow-md hover:bg-[var(--color-surface)] hover:shadow-lg transition-all"
        onclick={() => sidebarOpen = true}
        title="展开侧边栏"
      >
        <span class="material-symbols-outlined text-[18px]">menu</span>
      </button>
    {/if}

    <!-- File Tree Sidebar -->
    <aside
      class="border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col flex-shrink-0 transition-all duration-200
        {sidebarOpen ? 'w-60 max-md:fixed max-md:top-0 max-md:bottom-16 max-md:left-0 max-md:z-20 max-md:shadow-elevated-lg' : 'w-0 max-md:hidden overflow-hidden border-r-0'}"
      ondragover={(e) => { e.preventDefault(); dragOver = true; }}
      ondragleave={() => dragOver = false}
      ondrop={handleDrop}
    >
      <div class="px-4 h-12 flex items-center border-b border-[var(--color-border)] min-w-[240px]">
        <h3 class="text-sm font-semibold text-[var(--color-text)] truncate flex-1">{project?.name || '项目'}</h3>
        <button
          class="flex items-center justify-center w-7 h-7 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors"
          onclick={openFileSearch}
          title="搜索文件 (Ctrl+P)"
        >
          <span class="material-symbols-outlined !text-[16px]">search</span>
        </button>
        <button
          class="flex items-center justify-center w-7 h-7 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors ml-1"
          onclick={() => sidebarOpen = false}
          title="折叠侧边栏"
        >
          <span class="material-symbols-outlined !text-[16px]">menu_open</span>
        </button>
        <span class="text-xs text-[var(--color-text-muted)] ml-1">{files.length}</span>
      </div>
      <div class="flex-1 overflow-y-auto p-2 space-y-0.5" class:drag-over={dragOver}>
        <!-- Drag-over overlay -->
        {#if dragOver}
          <div class="absolute inset-0 z-10 flex items-center justify-center rounded-xl pointer-events-none" style="background: rgba(139,92,246,0.08); border: 2px dashed var(--color-primary);">
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
              class="file-tree-item flex-1 flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all duration-200 text-left
                {selectedFile === file.path ? 'active' : ''}"
              onclick={() => { selectFile(file.path); if (window.innerWidth < 768) sidebarOpen = false; }}
            >
              <span class="material-symbols-outlined text-[16px] flex-shrink-0" style="color: {getFileIconColor(file.path)}">{getFileIcon(file.path)}</span>
              <span class="truncate">{file.path.split('/').pop()}</span>
              {#if selectedFile === file.path}
                <div class="ml-auto w-1.5 h-1.5 rounded-full" style="background: var(--gradient-brand)"></div>
              {/if}
            </button>
            <button
              class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-[var(--color-error-light)] transition-opacity mr-1 cursor-pointer"
              title="删除文件"
              onclick={async (e) => {
                e.stopPropagation();
                if (!confirm(`确定删除 ${file.path}？`)) return;
                try {
                  const token = localStorage.getItem('moduforge_token') || '';
                  const res = await fetch(`/api/v1/projects/${projectId}/files/${encodeURIComponent(file.path)}`, {
                    method: 'DELETE',
                    headers: { 'Authorization': `Bearer ${token}` },
                  });
                  if (res.ok) {
                    files = files.filter(f => f.path !== file.path);
                    if (selectedFile === file.path) { selectedFile = null; editorContent = ''; }
                    toast('文件已删除', 'success');
                  }
                } catch { toast('删除失败', 'error'); }
              }}
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
      <div class="fixed inset-0 bg-black/20 z-10 max-md:block md:hidden" role="presentation" onclick={() => sidebarOpen = false}></div>
    {/if}

    <!-- Main Editor Area + Panels (wrapped in flex column) -->
    <div class="flex-1 flex flex-col overflow-hidden">
      <!-- Editor Content Area -->
      <div class="flex-1 flex flex-col overflow-hidden">
        <!-- Toolbar -->
        <div class="h-12 flex items-center justify-between px-2 sm:px-4 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex-shrink-0">
          <div class="flex items-center gap-2 sm:gap-3 min-w-0">
            <span class="text-sm font-medium text-[var(--color-text)] hidden sm:block truncate">{project?.name}</span>
            <span class="badge-primary text-[10px] hidden sm:inline-flex">Universal</span>
            <CollaborationCursors {projectId} currentUserId={''} />
            {#if showDiffList && diffFiles.length > 0}
              <span class="text-xs text-primary-500 flex items-center gap-1 hidden sm:flex">
                <span class="material-symbols-outlined text-[14px]">compare_arrows</span>
                {diffFiles.length} 待审查
              </span>
            {/if}
          </div>
          <div class="flex items-center gap-1 sm:gap-2 flex-shrink-0 overflow-x-auto">
            {#if showDiffList && diffFiles.length > 0}
              <button
                class="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors whitespace-nowrap"
                onclick={closeDiffView}
              >
                <span class="material-symbols-outlined text-[14px]">check</span>
                <span class="hidden sm:inline">完成审查</span>
              </button>
            {/if}
            <button
              class="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium transition-colors whitespace-nowrap"
              style="background: var(--color-surface); color: var(--color-text-secondary)"
              onclick={formatCode}
              disabled={formatting}
              title="格式化代码 ({shortcutLabel(shortcuts.find(s => s.id === 'format-code')!)})"
            >
              <span class="material-symbols-outlined !text-[14px] {formatting ? 'animate-spin' : ''}">{formatting ? 'progress_activity' : 'align_horizontal_left'}</span>
              <span class="hidden md:inline">{formatting ? '格式化中...' : '格式化'}</span>
            </button>
            <button
              class="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50 whitespace-nowrap"
              style="background: var(--color-surface); color: {getSecurityColor()}"
              onclick={runSecurityScan}
              disabled={securityScanning}
              title="安全扫描"
            >
              <span class="material-symbols-outlined !text-[14px] {securityScanning ? 'animate-spin' : ''}">{securityScanning ? 'progress_activity' : getSecurityIcon()}</span>
              <span class="hidden md:inline">{securityScanning ? '扫描中...' : '安全扫描'}</span>
            </button>
            <button
              class="flex items-center justify-center w-8 h-8 rounded-lg bg-[var(--color-surface)] text-[var(--color-text-secondary)] hover:bg-[var(--color-border)] transition-colors"
              onclick={validateProject}
              title="项目完整性校验"
            >
              <span class="material-symbols-outlined !text-[16px]">checklist</span>
            </button>
            <a href="/projects/{projectId}/build" class="flex items-center justify-center w-8 h-8 rounded-lg bg-[var(--color-surface)] text-[var(--color-text-secondary)] hover:bg-[var(--color-border)] transition-colors no-underline" title="构建模块">
              <span class="material-symbols-outlined !text-[16px]">build</span>
            </a>
            <button
              class="flex items-center justify-center w-8 h-8 rounded-lg transition-colors"
              style={showTerminal ? 'background: var(--color-primary); color: white' : 'background: var(--color-surface); color: var(--color-text-secondary)'}
              onclick={() => showTerminal = !showTerminal}
              title="终端 ({shortcutLabel(shortcuts.find(s => s.id === 'toggle-terminal')!)})"
            >
              <span class="material-symbols-outlined !text-[16px]">terminal</span>
            </button>
            <button
              class="flex items-center justify-center w-8 h-8 rounded-lg transition-colors"
              style="background: var(--color-surface); color: var(--color-text-secondary)"
              onclick={() => showUndoHistory = true}
              title="操作历史"
            >
              <span class="material-symbols-outlined !text-[16px]">history</span>
            </button>
            <button
              class="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50 whitespace-nowrap
                {saving ? 'text-[var(--color-text-muted)]' : 'bg-primary-600 text-white hover:bg-primary-700'}"
              style={saving ? 'background: var(--color-surface)' : ''}
              onclick={saveFile}
              disabled={saving || !selectedFile}
            >
              <span class="material-symbols-outlined !text-[14px]">{saving ? 'hourglass_top' : 'save'}</span>
              <span class="hidden sm:inline">{saving ? '保存中...' : '保存'}</span>
            </button>
          </div>
        </div>

        <!-- Tab Bar -->
        {#if openTabs.length > 0 && !diffMode}
          <div class="h-9 flex items-center border-b border-[var(--color-border)] bg-[var(--color-bg)] overflow-x-auto flex-shrink-0">
            {#each openTabs as tab}
              <button
                class="flex items-center gap-1.5 px-3 h-full text-xs border-r border-[var(--color-border)] transition-colors whitespace-nowrap flex-shrink-0
                  {activeTab === tab ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text)] font-medium border-t-2 border-t-primary-500' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'}"
                onclick={() => switchTab(tab)}
              >
                <span>{tab.split('/').pop()}</span>
                <span
                  role="button"
                  tabindex="-1"
                  class="material-symbols-outlined text-[12px] p-0.5 rounded hover:bg-black/10 transition-colors"
                  onclick={(e) => { e.stopPropagation(); closeTab(tab); }}
                  onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); e.stopPropagation(); closeTab(tab); } }}
                >close</span>
              </button>
            {/each}
          </div>
        {/if}

        <!-- Diff File List (shown when diff view is active) -->
        {#if showDiffList && diffFiles.length > 0}
          <div class="h-10 flex items-center px-3 gap-1 border-b border-[var(--color-border)] bg-[var(--color-bg)] overflow-x-auto flex-shrink-0">
            {#each diffFiles as df}
              <button
                class="diff-file-item flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs whitespace-nowrap
                  {selectedDiffFile === df.path ? 'active' : 'text-[var(--color-text-secondary)]'}"
                onclick={() => selectedDiffFile = df.path}
              >
                <span class="material-symbols-outlined text-[12px]" style="color: {getFileIconColor(df.path)}">{getFileIcon(df.path)}</span>
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

        <!-- Editor -->
        <div class="flex-1 flex flex-col overflow-hidden relative min-h-0" style={showDiffList && diffMode ? 'display: none;' : ''}>
          {#if selectedFile}
            <div class="flex-1 min-h-0 overflow-hidden">
              <CodeEditor
                value={editorContent}
                language={detectLanguage(selectedFile)}
                onChange={handleEditorChange}
                onSave={saveFile}
                onFileSearch={openFileSearch}
                diffMode={false}
              />
            </div>
            <!-- File Comments -->
            <FileComments {projectId} filePath={selectedFile} />
          {:else}
            <div class="h-full flex items-center justify-center text-[var(--color-text-secondary)]">
              <div class="text-center">
                <span class="material-symbols-outlined text-5xl mb-2 text-neutral-300">edit_document</span>
                <p class="text-sm">从左侧选择一个文件开始编辑</p>
                <p class="text-xs text-[var(--color-text-muted)] mt-1">或按 {shortcutLabel(shortcuts.find(s => s.id === 'search-file')!)} 快速搜索文件</p>
              </div>
            </div>
          {/if}
        </div>

        <!-- Diff Editor (side-by-side) -->
        {#if showDiffList && diffMode && selectedDiffFile}
          <div class="flex-1 overflow-hidden">
            <CodeEditor
              value={getCurrentDiffContent()}
              language={detectLanguage(selectedDiffFile)}
              onChange={() => {}}
              diffMode={true}
              diffContent={getIncomingDiffContent()}
              diffLabel={selectedDiffFile}
            />
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<!-- Security Panel Modal -->
{#if showSecurityPanel}
  <div class="fixed inset-0 z-50 flex items-center justify-center" style="background: rgba(0,0,0,0.4); backdrop-filter: blur(4px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showSecurityPanel = false; }}>
    <div class="w-[480px] max-h-[80vh] rounded-xl shadow-2xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg)]" role="dialog" aria-modal="true" tabindex="-1">
      <div class="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]" style="background: var(--color-bg-elevated)">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-[18px]" style="color: {getSecurityColor()}">{getSecurityIcon()}</span>
          <span class="text-sm font-semibold text-[var(--color-text)]">安全扫描</span>
          {#if securityResult}
            <span class="text-xs px-2 py-0.5 rounded-full" style="background: color-mix(in srgb, {getSecurityColor()} 15%, transparent); color: {getSecurityColor()}">{securityResult.score}/100</span>
          {/if}
        </div>
        <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={() => showSecurityPanel = false}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      <div class="p-4 overflow-y-auto" style="max-height: calc(80vh - 60px)">
        {#if securityScanning}
          <div class="flex items-center justify-center py-8">
            <span class="material-symbols-outlined text-[24px] animate-spin" style="color: var(--color-primary)">progress_activity</span>
            <span class="ml-2 text-sm text-[var(--color-text-secondary)]">扫描中...</span>
          </div>
        {:else if securityResult && securityResult.issues.length > 0}
          <div class="space-y-2">
            {#each securityResult.issues as issue}
              <div class="flex items-start gap-2 px-3 py-2 rounded-lg text-xs" style="background: color-mix(in srgb, var(--color-surface) 50%, transparent)">
                <span class="material-symbols-outlined text-[14px] mt-0.5 flex-shrink-0" style="color: {getIssueColor(issue.severity)}">{getIssueIcon(issue.severity)}</span>
                <div class="flex-1 min-w-0">
                  <div class="font-medium text-[var(--color-text)]">{issue.rule}</div>
                  <div class="text-[var(--color-text-muted)]">{issue.file}:{issue.line} — {issue.message}</div>
                </div>
              </div>
            {/each}
          </div>
        {:else if securityResult}
          <div class="text-center py-8">
            <span class="material-symbols-outlined text-[48px] text-green-500">check_circle</span>
            <p class="mt-2 text-sm text-[var(--color-text-secondary)]">未发现安全问题，项目代码安全。</p>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- Terminal Panel (fixed at bottom) -->
{#if showTerminal}
  <div class="fixed bottom-0 left-0 right-0 z-40 border-t border-[var(--color-border)]" style="height: {terminalHeight}px;">
    <div
      role="presentation"
      class="terminal-resize-handle cursor-row-resize flex-shrink-0 h-1 hover:bg-[var(--color-primary)] transition-colors"
      onmousedown={(e) => {
        e.preventDefault();
        const startY = e.clientY;
        const startHeight = terminalHeight;
        const onMove = (ev: MouseEvent) => {
          const delta = startY - ev.clientY;
          terminalHeight = Math.max(100, Math.min(600, startHeight + delta));
        };
        const onUp = () => {
          window.removeEventListener('mousemove', onMove);
          window.removeEventListener('mouseup', onUp);
        };
        window.addEventListener('mousemove', onMove);
        window.addEventListener('mouseup', onUp);
      }}
    ></div>
    <div class="terminal-container" style="height: calc(100% - 4px);">
      <Terminal execCommand={execTerminalCommand} onClose={() => showTerminal = false} />
    </div>
  </div>
{/if}

{#if showUndoHistory}
<UndoHistory onClose={() => showUndoHistory = false} onRollback={(idx) => { loadFiles(); toast('已回滚到历史操作', 'info'); }} />
{/if}

<!-- File Search Overlay (Ctrl+P) -->
{#if showFileSearch}
  <div class="fixed inset-0 z-50 flex items-start justify-center pt-24" style="background: rgba(0,0,0,0.4); backdrop-filter: blur(4px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) closeFileSearch(); }}>
    <div class="w-96 rounded-xl shadow-2xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg)]" role="presentation" onclick={(e) => e.stopPropagation()}>
      <div class="px-3 py-2 border-b border-[var(--color-border)]">
        <input
          bind:this={fileSearchInput}
          type="text"
          class="w-full px-3 py-2 rounded-lg text-sm bg-[var(--color-bg)] border border-[var(--color-border)] text-[var(--color-text)] outline-none focus:border-primary-500"
          placeholder="搜索文件..."
          value={fileSearchQuery}
          oninput={onFileSearchInput}
          onkeydown={handleFileSearchKeydown}
        />
      </div>
      {#if filteredFiles.length > 0}
        <div class="max-h-72 overflow-y-auto p-2 space-y-0.5">
          {#each filteredFiles as file}
            <button
              class="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs text-left transition-colors text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]"
              onclick={() => selectFileFromSearch(file.path)}
            >
              <span class="material-symbols-outlined text-[14px]" style="color: {getFileIconColor(file.path)}">{getFileIcon(file.path)}</span>
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

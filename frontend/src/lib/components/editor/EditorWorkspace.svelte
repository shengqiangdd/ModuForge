<script lang="ts">
  import { onMount } from 'svelte';
  import { client, authFetch } from '$lib/api/client';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Terminal from '$lib/components/Terminal.svelte';
  import UndoHistory from '$lib/components/UndoHistory.svelte';
  import FileComments from '$lib/components/FileComments.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { historyStore } from '$lib/stores/history';
  import { focusTrap } from '$lib/utils/focusTrap';
  import { loadShortcuts, matchShortcut, shortcutLabel } from '$lib/stores/shortcuts';
  import { debounce } from '$lib/utils/performance';
  import FileTree from './FileTree.svelte';
  import EditorToolbar from './EditorToolbar.svelte';
  import EditorTabs from './EditorTabs.svelte';
  import DiffView from './DiffView.svelte';
  import {
    buildTree,
    folderFirstCompare,
    detectLanguage,
    getFileIcon,
    getFileIconColor,
    getSecurityIcon as getSecurityIconUtil,
    getSecurityColor as getSecurityColorUtil,
    getIssueIcon as getIssueIconUtil,
    getIssueColor as getIssueColorUtil,
    type TreeNode,
  } from './editor-utils';

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

  // Tree view
  let viewMode = $state<'flat' | 'tree'>('tree');
  let treeData = $state<TreeNode | null>(null);

  $effect(() => {
    if (viewMode === 'tree' && files.length > 0) {
      treeData = buildTree(files);
    } else {
      treeData = null;
    }
  });

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
    debouncedFileSearch((e.target as HTMLInputElement).value);
  }
  const debouncedFileSearch = debounce((val: string) => { fileSearchQuery = val; }, 150);

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
    return getSecurityIconUtil(securityResult);
  }
  function getSecurityColor(): string {
    return getSecurityColorUtil(securityResult);
  }
  function getIssueIcon(severity: string): string {
    return getIssueIconUtil(severity);
  }
  function getIssueColor(severity: string): string {
    return getIssueColorUtil(severity);
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
        files = (fileData || []).map(f => ({ ...f, path: f.path })).sort(folderFirstCompare);
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
    // ListFiles returns metadata only (content omitted/empty since S3 is the
    // content store). Only reuse cached content when it is non-empty; otherwise
    // always fetch the actual content from GetFile (which reads S3 first).
    if (existing?.content) {
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
      files = (fileData || []).map(f => ({ ...f, path: f.path })).sort(folderFirstCompare);
    } catch {}
  }

  function handleEditorChange(val: string) {
    editorContent = val;
  }

</script>

<style>
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
    <FileTree
      {files} {selectedFile} {project} {sidebarOpen} {dragOver} {uploadProgress}
      {showFileSearch} {fileSearchQuery} {fileSearchInput} {filteredFiles}
      {viewMode} {treeData}
      onSelect={selectFile}
      onDelete={async (path: string) => {
        if (!confirm(`确定删除 ${path}？`)) return;
        try {
          const token = localStorage.getItem('moduforge_token') || '';
          const res = await fetch(`/api/v1/projects/${projectId}/files/${encodeURIComponent(path)}`, { method: 'DELETE', headers: { 'Authorization': `Bearer ${token}` } });
          if (res.ok) { files = files.filter(f => f.path !== path); if (selectedFile === path) { selectedFile = null; editorContent = ''; } toast('文件已删除', 'success'); }
        } catch { toast('删除失败', 'error'); }
      }}
      onToggleSidebar={() => sidebarOpen = !sidebarOpen}
      onOpenFileSearch={openFileSearch}
      onFileSearchInput={onFileSearchInput}
      onFileSearchKeydown={handleFileSearchKeydown}
      onSelectFileFromSearch={selectFileFromSearch}
      onCloseFileSearch={closeFileSearch}
      {getFileIcon} {getFileIconColor}
      onDrop={handleDrop}
      onDragOver={(e) => { e.preventDefault(); dragOver = true; }}
      onDragLeave={() => dragOver = false}
      onRefreshTree={() => { loadFiles(); }}
      onViewModeChange={(mode) => viewMode = mode}
    />

    <!-- Main Editor Area + Panels (wrapped in flex column) -->
    <div class="flex-1 flex flex-col overflow-hidden">
      <!-- Editor Content Area -->
      <div class="flex-1 flex flex-col overflow-hidden">
        <EditorToolbar
          {project} {projectId} {selectedFile} {saving} {formatting}
          {securityScanning} {securityResult} {showTerminal}
          {showDiffList} {diffFiles} {shortcuts}
          onFormatCode={formatCode}
          onRunSecurityScan={runSecurityScan}
          onValidateProject={validateProject}
          onToggleTerminal={() => showTerminal = !showTerminal}
          onSave={saveFile}
          {getSecurityIcon} {getSecurityColor}
        />

        <EditorTabs
          {openTabs} {activeTab} {diffMode}
          {showDiffList} {diffFiles} {selectedDiffFile}
          {getFileIcon} {getFileIconColor}
          onSwitchTab={switchTab}
          onCloseTab={closeTab}
          onSelectDiffFile={(path) => selectedDiffFile = path}
        />

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
          <DiffView
            {selectedDiffFile}
            {diffFiles}
            {detectLanguage}
          />
        {/if}
      </div>
    </div>
  {/if}
</div>

<!-- Security Panel Modal -->
{#if showSecurityPanel}
  <div class="fixed inset-0 z-50 flex items-center justify-center" style="background: rgba(0,0,0,0.4); backdrop-filter: blur(4px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showSecurityPanel = false; }} onkeydown={(e) => { if (e.key === 'Escape') showSecurityPanel = false; }}>
    <div class="w-[480px] max-h-[80vh] rounded-xl shadow-2xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg)]" role="dialog" aria-modal="true" tabindex="-1" use:focusTrap>
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

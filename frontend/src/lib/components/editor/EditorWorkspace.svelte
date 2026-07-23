<script lang="ts">
import { onMount } from 'svelte';
import { client, authFetch } from '$lib/api/client';
import CodeEditor from '$lib/components/CodeEditor.svelte';
import { toast } from '$lib/stores/toast.svelte';

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

  // Security scan
  let securityScanning = $state(false);
  let securityResult = $state<any>(null);
  let showSecurityPanel = $state(false);

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

  onMount(async () => {
    if (!projectId) { loading = false; return; }
    try {
      const [p, fileData] = await Promise.all([
        client.get<any>(`/projects/${projectId}`),
        client.get<{ id?: number; path: string }[]>(`/projects/${projectId}/files`),
      ]);
      project = p;
      files = fileData.map(f => ({ ...f, path: f.path }));
    } catch (e: any) {
      toast(e.message || '加载项目失败', 'error');
    } finally {
      loading = false;
    }
  });

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
      await client.put(`/projects/${projectId}/files/${encodeURIComponent(selectedFile)}`, { content: editorContent });
      const idx = files.findIndex(f => f.path === selectedFile);
      if (idx >= 0) files[idx] = { ...files[idx], content: editorContent };
      toast('文件保存成功', 'success');
    } catch (e: any) {
      toast(e.message || '保存失败', 'error');
    } finally {
      saving = false;
    }
  }

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
    return iconMap[ext] || 'description';
  }

  function getFileIconColor(path: string): string {
    const ext = path.split('.').pop()?.toLowerCase() || '';
    const colorMap: Record<string, string> = {
      js: '#f7df1e', jsx: '#61dafb', ts: '#3178c6', tsx: '#61dafb',
      py: '#3776ab', html: '#e34f26', css: '#1572b6',
      json: '#292929', sh: '#4eaa25', bash: '#4eaa25',
      md: '#ffffff', prop: '#8b5cf6',
    };
    return colorMap[ext] || 'var(--color-text-muted)';
  }

  function handleEditorChange(val: string) {
    editorContent = val;
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
</style>

<div class="flex flex-1 h-full overflow-hidden">
  {#if !projectId}
    <div class="flex-1 flex items-center justify-center text-[var(--color-text-secondary)]">
      <div class="text-center">
        <span class="material-symbols-outlined text-5xl mb-3 text-neutral-300">code_blocks</span>
        <p class="text-lg font-medium">选择或创建一个项目开始编辑</p>
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
    <!-- Sidebar Toggle (mobile) -->
    <button
      class="md:hidden fixed left-2 top-20 z-10 w-8 h-8 rounded-lg bg-[var(--color-bg-elevated)] border border-[var(--color-border)] flex items-center justify-center shadow-sm"
      onclick={() => sidebarOpen = !sidebarOpen}
    >
      <span class="material-symbols-outlined text-[18px]">{sidebarOpen ? 'menu_open' : 'menu'}</span>
    </button>

    <!-- File Tree Sidebar -->
    <aside
      class="w-60 border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col flex-shrink-0 transition-all duration-200
        {sidebarOpen ? 'max-md:fixed max-md:inset-y-0 max-md:left-0 max-md:z-20 max-md:shadow-elevated-lg' : 'max-md:hidden'}"
    >
      <div class="px-4 h-12 flex items-center border-b border-[var(--color-border)]">
        <h3 class="text-sm font-semibold text-[var(--color-text)] truncate flex-1">{project?.name || '项目'}</h3>
        <span class="text-xs text-[var(--color-text-muted)]">{files.length} 文件</span>
      </div>
      <div class="flex-1 overflow-y-auto p-2 space-y-0.5">
        {#each files as file}
          <button
            class="file-tree-item w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all duration-200 text-left
              {selectedFile === file.path ? 'active' : ''}"
            onclick={() => { selectFile(file.path); sidebarOpen = false; }}
          >
            <span class="material-symbols-outlined text-[16px] flex-shrink-0" style="color: {getFileIconColor(file.path)}">{getFileIcon(file.path)}</span>
            <span class="truncate">{file.path.split('/').pop()}</span>
            {#if selectedFile === file.path}
              <div class="ml-auto w-1.5 h-1.5 rounded-full" style="background: var(--gradient-brand)"></div>
            {/if}
          </button>
        {:else}
          <div class="text-center py-12">
            <span class="material-symbols-outlined text-4xl mb-2" style="color: var(--color-text-muted)">folder_open</span>
            <p class="text-xs" style="color: var(--color-text-muted)">暂无文件</p>
          </div>
        {/each}
      </div>
    </aside>

    {#if sidebarOpen && !sidebarOpen}
      <div class="fixed inset-0 bg-black/20 z-10 md:hidden" onclick={() => sidebarOpen = false}></div>
    {/if}

    <!-- Main Editor Area -->
    <div class="flex-1 flex flex-col overflow-hidden">
      <!-- Toolbar -->
      <div class="h-12 flex items-center justify-between px-4 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex-shrink-0">
        <div class="flex items-center gap-3">
          <span class="text-sm font-medium text-[var(--color-text)] hidden sm:block">{project?.name}</span>
          <span class="badge-primary text-[10px]">Universal</span>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50"
            style="background: var(--color-surface); color: {getSecurityColor()}"
            onclick={runSecurityScan}
            disabled={securityScanning}
            title="安全扫描"
          >
            <span class="material-symbols-outlined text-[14px] {securityScanning ? 'animate-spin' : ''}">{securityScanning ? 'progress_activity' : getSecurityIcon()}</span>
            <span class="hidden sm:inline">{securityScanning ? '扫描中...' : '安全扫描'}</span>
          </button>
          <a href="/projects/{projectId}/build" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-[var(--color-surface)] text-[var(--color-text-secondary)] hover:bg-[var(--color-border)] transition-colors no-underline">
            <span class="material-symbols-outlined text-[14px]">build</span>
            <span class="hidden sm:inline">构建</span>
          </a>
          <button
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50
              {saving ? 'text-[var(--color-text-muted)]' : 'bg-primary-600 text-white hover:bg-primary-700'}"
            style={saving ? 'background: var(--color-surface)' : ''}
            onclick={saveFile}
            disabled={saving || !selectedFile}
          >
            <span class="material-symbols-outlined text-[14px]">{saving ? 'hourglass_top' : 'save'}</span>
            <span class="hidden sm:inline">{saving ? '保存中...' : '保存'}</span>
          </button>
        </div>
      </div>

      <!-- Tab Bar -->
      {#if openTabs.length > 0}
        <div class="h-9 flex items-center border-b border-[var(--color-border)] bg-[var(--color-bg)] overflow-x-auto flex-shrink-0">
          {#each openTabs as tab}
            <button
              class="flex items-center gap-1.5 px-3 h-full text-xs border-r border-[var(--color-border)] transition-colors whitespace-nowrap flex-shrink-0
                {activeTab === tab ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text)] font-medium border-t-2 border-t-primary-500' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'}"
              onclick={() => switchTab(tab)}
            >
              <span>{tab.split('/').pop()}</span>
              <span
                class="material-symbols-outlined text-[12px] p-0.5 rounded hover:bg-black/10 transition-colors"
                onclick={(e) => { e.stopPropagation(); closeTab(tab); }}
              >close</span>
            </button>
          {/each}
        </div>
      {/if}

      <!-- Editor -->
      <div class="flex-1 overflow-hidden relative">
        {#if selectedFile}
          <CodeEditor value={editorContent} language={detectLanguage(selectedFile)} onChange={handleEditorChange} />
        {:else}
          <div class="h-full flex items-center justify-center text-[var(--color-text-secondary)]">
            <div class="text-center">
              <span class="material-symbols-outlined text-5xl mb-2 text-neutral-300">edit_document</span>
              <p class="text-sm">从左侧选择一个文件开始编辑</p>
            </div>
          </div>
        {/if}
      </div>
    </div>

    <!-- Security Scan Panel -->
    {#if showSecurityPanel}
      <div class="border-t border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex-shrink-0" style="max-height: 200px; overflow-y: auto;">
        <div class="flex items-center justify-between px-4 py-2">
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-[16px]" style="color: {getSecurityColor()}">{getSecurityIcon()}</span>
            <span class="text-xs font-semibold text-[var(--color-text)]">安全扫描</span>
            {#if securityResult}
              <span class="text-xs" style="color: {getSecurityColor()}">评分：{securityResult.score}/100</span>
            {/if}
          </div>
          <div class="flex items-center gap-2">
            {#if securityScanning}
              <span class="text-xs text-[var(--color-text-muted)]">扫描中...</span>
            {:else if securityResult}
              <span class="text-xs text-[var(--color-text-secondary)] truncate max-w-64">{securityResult.summary}</span>
            {/if}
            <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => showSecurityPanel = false}>
              <span class="material-symbols-outlined text-[14px]">close</span>
            </button>
          </div>
        </div>
        {#if securityResult && securityResult.issues.length > 0}
          <div class="px-4 pb-2 space-y-1 max-h-32 overflow-y-auto">
            {#each securityResult.issues as issue}
              <div class="flex items-start gap-1.5 text-xs px-2 py-1 rounded" style="background: color-mix(in srgb, var(--color-surface) 50%, transparent)">
                <span class="material-symbols-outlined text-[12px] mt-0.5 flex-shrink-0" style="color: {getIssueColor(issue.severity)}">{getIssueIcon(issue.severity)}</span>
                <span style="color: var(--color-text-secondary)">
                  <strong>{issue.rule}</strong> @ {issue.file}:{issue.line} — {issue.message}
                </span>
              </div>
            {/each}
          </div>
        {:else if securityResult}
          <div class="px-4 pb-3">
            <p class="text-xs text-[var(--color-text-secondary)]">未发现安全问题，项目代码安全。</p>
          </div>
        {/if}
      </div>
    {/if}
  {/if}
</div>

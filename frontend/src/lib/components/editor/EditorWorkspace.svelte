<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '$lib/api/client';
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

  function handleEditorChange(val: string) {
    editorContent = val;
  }
</script>

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
            class="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors text-left
              {selectedFile === file.path ? 'bg-primary-50 text-primary-700 font-medium' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
            onclick={() => { selectFile(file.path); sidebarOpen = false; }}
          >
            <span class="material-symbols-outlined text-[16px] flex-shrink-0">{file.path.endsWith('/') ? 'folder' : 'description'}</span>
            <span class="truncate">{file.path}</span>
          </button>
        {:else}
          <p class="text-xs text-[var(--color-text-muted)] text-center py-8">暂无文件</p>
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
          <span class="badge-primary text-[10px]">{project?.module_type || 'magisk'}</span>
        </div>
        <div class="flex items-center gap-2">
          <a href="/projects/{projectId}/build" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-[var(--color-surface)] text-[var(--color-text-secondary)] hover:bg-neutral-200 transition-colors no-underline">
            <span class="material-symbols-outlined text-[14px]">build</span>
            <span class="hidden sm:inline">构建</span>
          </a>
          <button
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50
              {saving ? 'bg-neutral-100 text-neutral-400' : 'bg-primary-600 text-white hover:bg-primary-700'}"
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
  {/if}
</div>

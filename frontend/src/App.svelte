<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n';
  import { client } from '$lib/api/client';
  import LocaleSwitcher from '$lib/components/LocaleSwitcher.svelte';
  import MarketPage from './routes/market/+page.svelte';
  import PublishPage from './routes/market/publish/+page.svelte';
  import AuthPage from './routes/auth/+page.svelte';
  import DashboardPage from './routes/dashboard/+page.svelte';
  import EditorWorkspace from '$lib/components/editor/EditorWorkspace.svelte';
  import BuildWorkspace from '$lib/components/editor/BuildWorkspace.svelte';
  import SettingsPage from './routes/settings/+page.svelte';
  import Toast from '$lib/components/ui/Toast.svelte';
  import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
  import { toast } from '$lib/stores/toast.svelte';

  type Route = 'auth' | 'projects' | 'editor' | 'builds' | 'settings' | 'market' | 'market-publish' | 'dashboard';

  let current = $state<Route>('auth');
  let projectId = $state('');
  let token = $state<string | null>(null);
  let sidebarCollapsed = $state(false);
  let mounted = $state(false);

  // Global loading
  let globalLoading = $state(false);

  // Confirm dialog
  let confirmOpen = $state(false);
  let confirmTitle = $state('');
  let confirmMessage = $state('');
  let confirmVariant = $state<'primary' | 'danger'>('primary');
  let confirmCallback = $state<(() => void) | null>(null);

  // Project state
  interface Project { id: string; name: string; module_type: string; description: string; created_at: string; updated_at: string; }
  let projects = $state<Project[]>([]);
  let showCreateModal = $state(false);
  let newProjectName = $state('');
  let newProjectType = $state('magisk');
  let newProjectDesc = $state('');
  let creatingProject = $state(false);

  function navigate(route: Route, id?: string) {
    current = route;
    if (id) projectId = id;
    if (route === 'market') history.pushState(null, '', '/market');
    else if (route === 'market-publish') history.pushState(null, '', '/market/publish');
    else if (route === 'dashboard') history.pushState(null, '', '/dashboard');
    else if (route === 'projects') history.pushState(null, '', '/projects');
    else if (route === 'settings') history.pushState(null, '', '/settings');
    else if (route === 'editor' && id) history.pushState(null, '', `/projects/${id}`);
    else if (route === 'builds' && id) history.pushState(null, '', `/projects/${id}/build`);
  }

  function handleAuth(newToken: string) {
    token = newToken;
    current = 'projects';
    loadProjects();
    toast('登录成功', 'success');
  }

  function logout() {
    localStorage.removeItem('moduforge_token');
    token = null;
    current = 'auth';
    projects = [];
    toast('已退出登录', 'info');
  }

  async function loadProjects() {
    try {
      projects = await client.get<Project[]>('/projects');
    } catch (e: any) { toast(e.message || '加载项目失败', 'error'); }
  }

  async function createProject() {
    if (!newProjectName.trim()) return;
    creatingProject = true;
    try {
      const p = await client.post<Project>('/projects', {
        name: newProjectName.trim(),
        module_type: newProjectType,
        description: newProjectDesc.trim(),
      });
      projects = [p, ...projects];
      showCreateModal = false;
      newProjectName = '';
      newProjectDesc = '';
      toast('项目创建成功', 'success');
      navigate('editor', p.id);
    } catch (e: any) {
      toast(e.message || '创建失败', 'error');
    } finally {
      creatingProject = false;
    }
  }

  function confirmDelete(id: string) {
    confirmTitle = '删除项目';
    confirmMessage = '确定要删除这个项目吗？此操作不可撤销。';
    confirmVariant = 'danger';
    confirmCallback = () => deleteProject(id);
    confirmOpen = true;
  }

  async function deleteProject(id: string) {
    globalLoading = true;
    try {
      await client.del(`/projects/${id}`);
      projects = projects.filter(p => p.id !== id);
      if (projectId === id) { projectId = ''; navigate('projects'); }
      toast('项目已删除', 'success');
    } catch (e: any) { toast(e.message, 'error'); }
    globalLoading = false;
  }

  function confirmLogout() {
    confirmTitle = '退出登录';
    confirmMessage = '确定要退出登录吗？';
    confirmVariant = 'primary';
    confirmCallback = logout;
    confirmOpen = true;
  }

  onMount(() => {
    mounted = true;
    const saved = localStorage.getItem('moduforge_token');
    if (saved) {
      token = saved;
      current = 'projects';
      loadProjects();
    }

    // Apply saved theme
    const savedTheme = localStorage.getItem('moduforge_theme');
    if (savedTheme) {
      document.documentElement.classList.toggle('dark', savedTheme === 'dark');
    }

    function handlePopState() {
      const path = window.location.pathname;
      if (path === '/market/publish') current = 'market-publish';
      else if (path === '/market') current = 'market';
      else if (path === '/dashboard') current = 'dashboard';
      else if (path === '/settings') current = 'settings';
      else if (path === '/projects') current = 'projects';
      else if (path.startsWith('/projects/') && path.includes('/build')) { current = 'builds'; projectId = path.split('/')[2] || ''; }
      else if (path.startsWith('/projects/')) { current = 'editor'; projectId = path.split('/')[2] || ''; }
    }
    handlePopState();
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  });

  const navItems = $derived([
    { id: 'dashboard', icon: 'monitoring', label: $t('nav.dashboard') },
    { id: 'projects', icon: 'folder', label: $t('nav.projects') },
    ...(projectId ? [
      { id: 'editor', icon: 'code', label: $t('nav.editor') },
      { id: 'builds', icon: 'build', label: $t('nav.builds') },
    ] : []),
    { id: 'market', icon: 'storefront', label: $t('nav.market') },
    { id: 'settings', icon: 'settings', label: $t('nav.settings') },
  ]);

  const bottomNavItems = $derived([
    { id: 'dashboard', icon: 'monitoring', label: $t('nav.dashboard') },
    { id: 'projects', icon: 'folder', label: $t('nav.projects') },
    ...(projectId ? [{ id: 'editor', icon: 'code', label: $t('nav.editor') }] : []),
    { id: 'market', icon: 'storefront', label: $t('nav.market') },
  ]);
</script>

<Toast />

<ConfirmDialog
  open={confirmOpen}
  {confirmTitle}
  message={confirmMessage}
  variant={confirmVariant}
  onConfirm={() => { confirmOpen = false; confirmCallback?.(); }}
  onCancel={() => { confirmOpen = false; confirmCallback = null; }}
/>

{#if !token}
  <AuthPage onAuth={handleAuth} />
{:else}
<div class="flex h-screen overflow-hidden bg-[var(--color-bg)]">
  <!-- Global Loading Overlay -->
  {#if globalLoading}
    <div class="fixed inset-0 bg-black/10 z-50 flex items-center justify-center">
      <div class="bg-[var(--color-bg-elevated)] rounded-2xl px-6 py-4 shadow-elevated-lg flex items-center gap-3">
        <div class="animate-spin h-5 w-5 border-2 border-primary-500 border-t-transparent rounded-full"></div>
        <span class="text-sm text-[var(--color-text-secondary)]">处理中...</span>
      </div>
    </div>
  {/if}

  <!-- Desktop Sidebar -->
  <aside
    class="hidden md:flex flex-col border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] transition-all duration-300 ease-in-out {sidebarCollapsed ? 'w-[72px]' : 'w-64'}"
  >
    <!-- Logo -->
    <div class="flex items-center gap-3 px-5 h-16 border-b border-[var(--color-border)] cursor-pointer" role="button" tabindex="0" onclick={() => navigate('dashboard')} onkeydown={(e) => { if (e.key === 'Enter') navigate('dashboard'); }}>
      <div class="w-8 h-8 rounded-xl bg-gradient-to-br from-primary-500 to-primary-700 flex items-center justify-center flex-shrink-0 cursor-pointer">
        <span class="material-symbols-outlined text-white text-lg">extension</span>
      </div>
      {#if !sidebarCollapsed}
        <div class="overflow-hidden cursor-pointer">
          <h1 class="text-base font-bold text-[var(--color-text)] tracking-tight">ModuForge</h1>
          <p class="text-[11px] text-[var(--color-text-muted)] leading-tight">Module Builder</p>
        </div>
      {/if}
      <button
        class="ml-auto p-1.5 rounded-lg hover:bg-neutral-100 text-neutral-400 transition-colors"
        onclick={() => sidebarCollapsed = !sidebarCollapsed}
      >
        <span class="material-symbols-outlined text-[18px]">{sidebarCollapsed ? 'chevron_right' : 'chevron_left'}</span>
      </button>
    </div>

    <!-- Nav Items -->
    <nav class="flex-1 p-3 space-y-1 overflow-y-auto">
      {#each navItems as item}
        <button
          class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-150 text-[14px] font-medium group
            {current === item.id
              ? 'bg-primary-50 text-primary-700'
              : 'text-[var(--color-text-secondary)] hover:bg-neutral-50 hover:text-[var(--color-text)]'}"
          onclick={() => navigate(item.id as Route, item.id === 'editor' || item.id === 'builds' ? projectId : undefined)}
          title={sidebarCollapsed ? item.label : undefined}
        >
          <span class="material-symbols-outlined text-[20px] flex-shrink-0 {current === item.id ? 'text-primary-600' : 'text-neutral-400 group-hover:text-neutral-600'}">
            {item.icon}
          </span>
          {#if !sidebarCollapsed}
            <span>{item.label}</span>
          {/if}
          {#if current === item.id}
            <div class="ml-auto w-1.5 h-1.5 rounded-full bg-primary-500 flex-shrink-0"></div>
          {/if}
        </button>
      {/each}
    </nav>

    <!-- Bottom section -->
    <div class="p-3 border-t border-[var(--color-border)]">
      <LocaleSwitcher compact={sidebarCollapsed} />
      <button
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-[var(--color-text-secondary)] hover:bg-red-50 hover:text-red-600 transition-all duration-150 text-[14px] font-medium mt-1"
        onclick={confirmLogout}
      >
        <span class="material-symbols-outlined text-[20px]">logout</span>
        {#if !sidebarCollapsed}
          <span>{$t('nav.logout')}</span>
        {/if}
      </button>
    </div>
  </aside>

  <!-- Main Content -->
  <main class="flex-1 flex flex-col overflow-hidden">
    <!-- Mobile Header -->
    <header class="md:hidden flex items-center justify-between px-4 h-14 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex-shrink-0">
      <div class="flex items-center gap-2.5" onclick={() => navigate('dashboard')}>
        <div class="w-7 h-7 rounded-lg bg-gradient-to-br from-primary-500 to-primary-700 flex items-center justify-center">
          <span class="material-symbols-outlined text-white text-sm">extension</span>
        </div>
        <span class="text-sm font-bold text-[var(--color-text)]">ModuForge</span>
      </div>
      <LocaleSwitcher />
    </header>

    <!-- Page Content -->
    {#if current === 'editor'}
      <EditorWorkspace {projectId} />
    {:else if current === 'builds'}
      <div class="flex-1 overflow-y-auto">
        <BuildWorkspace {projectId} />
      </div>
    {:else if current === 'settings'}
      <div class="flex-1 overflow-y-auto"><SettingsPage /></div>
    {:else if current === 'projects'}
      <div class="flex-1 overflow-y-auto page-enter">
        <div class="p-6 max-w-7xl mx-auto">
          <div class="flex items-center gap-3 mb-6">
            <div>
              <h2 class="text-2xl font-bold text-[var(--color-text)]">{$t('nav.projects')}</h2>
              <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">{$t('project.select_or_create')}</p>
            </div>
          </div>
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <!-- New Project Card -->
            <button
              class="border-2 border-dashed border-[var(--color-border)] rounded-2xl p-8 text-center hover:border-primary-400 hover:bg-primary-50/50 transition-all duration-200 cursor-pointer group"
              onclick={() => showCreateModal = true}
            >
              <div class="w-12 h-12 rounded-2xl bg-primary-100 flex items-center justify-center mx-auto mb-3 group-hover:scale-110 transition-transform">
                <span class="material-symbols-outlined text-primary-600 text-2xl">add</span>
              </div>
              <p class="font-semibold text-[var(--color-text)]">{$t('project.new')}</p>
              <p class="text-xs text-[var(--color-text-muted)] mt-1">Create a new Magisk module</p>
            </button>
            <!-- Existing Projects -->
            {#each projects as project (project.id)}
              <div class="bg-[var(--color-bg-elevated)] rounded-2xl border border-[var(--color-border)] p-5 hover:shadow-md transition-all duration-200 cursor-pointer group relative"
                   onclick={() => navigate('editor', project.id)}>
                <div class="flex items-start justify-between mb-3">
                  <div class="w-10 h-10 rounded-xl bg-primary-50 flex items-center justify-center">
                    <span class="material-symbols-outlined text-primary-600">folder</span>
                  </div>
                  <button class="p-1 rounded-lg hover:bg-red-50 text-neutral-400 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-all"
                          onclick={(e) => { e.stopPropagation(); confirmDelete(project.id); }}>
                    <span class="material-symbols-outlined text-[18px]">delete</span>
                  </button>
                </div>
                <h3 class="font-semibold text-[var(--color-text)] mb-1">{project.name}</h3>
                <p class="text-xs text-[var(--color-text-muted)] line-clamp-2">{project.description || 'No description'}</p>
                <div class="flex items-center gap-2 mt-3">
                  <span class="px-2 py-0.5 rounded-full bg-primary-50 text-primary-700 text-[11px] font-medium">{project.module_type}</span>
                  <span class="text-[11px] text-[var(--color-text-muted)]">{new Date(project.updated_at).toLocaleDateString()}</span>
                </div>
              </div>
            {/each}
          </div>
        </div>
      </div>

      <!-- Create Project Modal -->
      {#if showCreateModal}
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-[fadeIn_0.15s_ease-out]" onclick={() => showCreateModal = false}>
          <div class="bg-[var(--color-bg-elevated)] rounded-2xl shadow-elevated-lg w-full max-w-md mx-4 p-6 border border-[var(--color-border)] animate-[scaleIn_0.2s_ease-out]"
               onclick={(e) => e.stopPropagation()}>
            <h3 class="text-lg font-bold text-[var(--color-text)] mb-4">{$t('project.new')}</h3>
            <div class="space-y-4">
              <div>
                <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">项目名称</label>
                <input type="text" bind:value={newProjectName} placeholder="My Awesome Module"
                       class="w-full px-3 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent" />
              </div>
              <div>
                <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">模块类型</label>
                <select bind:value={newProjectType}
                        class="w-full px-3 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] focus:outline-none focus:ring-2 focus:ring-primary-500">
                  <option value="magisk">Magisk</option>
                  <option value="ksu">KernelSU</option>
                  <option value="apatch">APatch</option>
                  <option value="hybrid">Hybrid</option>
                </select>
              </div>
              <div>
                <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">描述</label>
                <textarea bind:value={newProjectDesc} placeholder="Optional description..." rows="3"
                          class="w-full px-3 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"></textarea>
              </div>
            </div>
            <div class="flex justify-end gap-3 mt-6">
              <button class="px-4 py-2 rounded-xl text-sm font-medium text-[var(--color-text-secondary)] hover:bg-neutral-100 transition-colors"
                      onclick={() => showCreateModal = false}>取消</button>
              <button class="px-4 py-2 rounded-xl text-sm font-medium bg-gradient-to-r from-primary-500 to-primary-700 text-white hover:from-primary-600 hover:to-primary-800 transition-all disabled:opacity-50"
                      onclick={createProject} disabled={creatingProject || !newProjectName.trim()}>
                {creatingProject ? '创建中...' : '创建'}
              </button>
            </div>
          </div>
        </div>
      {/if}
    {:else if current === 'market'}
      <div class="flex-1 overflow-y-auto page-enter"><MarketPage /></div>
    {:else if current === 'market-publish'}
      <div class="flex-1 overflow-y-auto page-enter"><PublishPage /></div>
    {:else if current === 'dashboard'}
      <div class="flex-1 overflow-y-auto page-enter"><DashboardPage /></div>
    {/if}
  </main>

  <!-- Mobile Bottom Nav -->
  <nav class="md:hidden fixed bottom-0 left-0 right-0 h-16 glass border-t border-[var(--color-border)] flex items-center justify-around px-2 z-40">
    {#each bottomNavItems as item}
      <button
        class="flex flex-col items-center gap-0.5 py-1.5 px-3 rounded-xl transition-all duration-150 min-w-[60px] {current === item.id ? 'text-primary-600' : 'text-[var(--color-text-muted)]'}"
        onclick={() => navigate(item.id as Route, item.id === 'editor' ? projectId : undefined)}
      >
        <span class="material-symbols-outlined text-[22px] {current === item.id ? 'text-primary-600' : ''}">{item.icon}</span>
        <span class="text-[10px] font-medium leading-tight">{item.label}</span>
      </button>
    {/each}
  </nav>
</div>
{/if}

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
  import AIPage from './routes/ai/+page.svelte';
  import Toast from '$lib/components/ui/Toast.svelte';
  import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
  import { toast } from '$lib/stores/toast.svelte';

  type Route = 'auth' | 'projects' | 'editor' | 'builds' | 'settings' | 'market' | 'market-publish' | 'dashboard' | 'ai';

  let current = $state<Route>('auth');
  let projectId = $state('');
  let token = $state<string | null>(null);
  let sidebarCollapsed = $state(false);
  let mounted = $state(false);
  let globalLoading = $state(false);
  let mobileMenuOpen = $state(false);
  let offline = $state(false);
  let errorCaught = $state<string | null>(null);

  // Theme
  let theme = $state<'dark' | 'light'>('dark');

  // Confirm dialog
  let confirmOpen = $state(false);
  let confirmTitle = $state('');
  let confirmMessage = $state('');
  let confirmVariant = $state<'primary' | 'danger'>('primary');
  let confirmCallback = $state<(() => void) | null>(null);

  // Project state
  interface Project { id: string; name: string; module_type: string; description: string; created_at: string; updated_at: string; }
  let projects = $state<Project[]>([]);
  let projectSearch = $state('');
  let showCreateModal = $state(false);
  let newProjectName = $state('');
  let newProjectType = $state('universal');
  let newProjectDesc = $state('');
  let creatingProject = $state(false);
  let filteredProjects = $derived((projects || []).filter(p =>
    !projectSearch || p.name.toLowerCase().includes(projectSearch.toLowerCase()) || (p.description && p.description.toLowerCase().includes(projectSearch.toLowerCase()))
  ));

  function navigate(route: Route, id?: string) {
    current = route;
    mobileMenuOpen = false;
    if (id) projectId = id;
    if (route === 'market') history.pushState(null, '', '/market');
    else if (route === 'market-publish') history.pushState(null, '', '/market/publish');
    else if (route === 'dashboard') history.pushState(null, '', '/dashboard');
    else if (route === 'projects') history.pushState(null, '', '/projects');
    else if (route === 'settings') history.pushState(null, '', '/settings');
    else if (route === 'ai') history.pushState(null, '', '/ai');
    else if (route === 'editor' && id) history.pushState(null, '', `/projects/${id}`);
    else if (route === 'builds' && id) history.pushState(null, '', `/projects/${id}/build`);
  }

  function handleAuth(newToken: string, action: 'login' | 'register' = 'login') {
    token = newToken;
    localStorage.setItem('moduforge_token', newToken);
    current = 'projects';
    loadProjects();
    if (action === 'register') {
      toast('注册成功！正在跳转...', 'success');
    } else {
      toast('登录成功！欢迎回来', 'success');
    }
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

  function toggleTheme() {
    theme = theme === 'dark' ? 'light' : 'dark';
    document.documentElement.classList.toggle('dark', theme === 'dark');
    document.documentElement.classList.toggle('light', theme === 'light');
    localStorage.setItem('moduforge_theme', theme);
  }

  onMount(() => {
    mounted = true;
    offline = !navigator.onLine;
    const saved = localStorage.getItem('moduforge_token');
    if (saved) {
      token = saved;
      current = 'projects';
      loadProjects();
    }

    // Apply theme — dark by default
    const savedTheme = localStorage.getItem('moduforge_theme') as 'dark' | 'light' | null;
    theme = savedTheme || 'dark';
    document.documentElement.classList.toggle('dark', theme === 'dark');
    document.documentElement.classList.toggle('light', theme === 'light');

    function handlePopState() {
      const path = window.location.pathname;
      if (path === '/market/publish') current = 'market-publish';
      else if (path === '/market') current = 'market';
      else if (path === '/dashboard') current = 'dashboard';
      else if (path === '/settings') current = 'settings';
      else if (path === '/ai') current = 'ai';
      else if (path === '/projects') current = 'projects';
      else if (path.startsWith('/projects/') && path.includes('/build')) { current = 'builds'; projectId = path.split('/')[2] || ''; }
      else if (path.startsWith('/projects/')) { current = 'editor'; projectId = path.split('/')[2] || ''; }
    }
    handlePopState();
    window.addEventListener('popstate', handlePopState);

    const goOnline = () => { offline = false; toast('网络已恢复', 'success'); };
    const goOffline = () => { offline = true; toast('网络已断开，部分功能不可用', 'error'); };
    window.addEventListener('online', goOnline);
    window.addEventListener('offline', goOffline);

    const handleGlobalError = (event: ErrorEvent) => {
      errorCaught = event.message || '未知错误';
      event.preventDefault();
    };
    window.addEventListener('error', handleGlobalError);

    return () => {
      window.removeEventListener('popstate', handlePopState);
      window.removeEventListener('online', goOnline);
      window.removeEventListener('offline', goOffline);
      window.removeEventListener('error', handleGlobalError);
    };
  });

  const navItems = $derived([
    { id: 'dashboard', icon: 'monitoring', label: $t('nav.dashboard') },
    { id: 'projects', icon: 'folder', label: $t('nav.projects') },
    ...(projectId ? [
      { id: 'editor', icon: 'code', label: $t('nav.editor') },
      { id: 'builds', icon: 'build', label: $t('nav.builds') },
    ] : []),
    { id: 'ai', icon: 'psychology', label: 'AI 助手' },
    { id: 'market', icon: 'storefront', label: $t('nav.market') },
    { id: 'settings', icon: 'settings', label: $t('nav.settings') },
  ]);

  const bottomNavItems = $derived([
    { id: 'dashboard', icon: 'monitoring', label: $t('nav.dashboard') },
    { id: 'projects', icon: 'folder', label: $t('nav.projects') },
    ...(projectId ? [{ id: 'editor', icon: 'code', label: $t('nav.editor') }] : []),
    { id: 'ai', icon: 'psychology', label: 'AI' },
    { id: 'market', icon: 'storefront', label: $t('nav.market') },
  ]);
</script>

<style>
  .project-card {
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .project-card:hover {
    border-color: var(--color-primary);
    box-shadow: 0 8px 32px rgba(139,92,246,0.12), 0 0 0 1px rgba(139,92,246,0.08);
    transform: translateY(-4px);
  }
  .nav-item {
    position: relative;
    overflow: hidden;
  }
</style>

<Toast />

{#if offline}
  <div class="fixed top-0 left-0 right-0 z-[100] flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-white" style="background: var(--color-error);">
    <span class="material-symbols-outlined text-[16px]">wifi_off</span>
    <span>网络已断开 — 部分功能不可用</span>
  </div>
{/if}

{#if errorCaught}
  <div class="fixed inset-0 z-[200] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);">
    <div class="rounded-2xl shadow-2xl p-8 max-w-md text-center border" style="background: var(--color-bg-elevated); border-color: var(--color-border);">
      <span class="material-symbols-outlined text-5xl mb-3 block" style="color: var(--color-error)">error_outline</span>
      <h2 class="text-lg font-bold mb-2" style="color: var(--color-text)">出现异常</h2>
      <p class="text-sm mb-4" style="color: var(--color-text-secondary)">{errorCaught}</p>
      <button class="btn-primary" onclick={() => { errorCaught = null; window.location.reload(); }}>刷新页面</button>
    </div>
  </div>
{/if}

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
<div class="flex h-screen overflow-hidden" style="background: var(--color-bg)">
  <!-- Global Loading Overlay -->
  {#if globalLoading}
    <div class="fixed inset-0 z-50 flex items-center justify-center" style="background: rgba(0,0,0,0.5); backdrop-filter: blur(4px)">
      <div class="flex items-center gap-3 px-6 py-4 rounded-2xl border" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)">
        <div class="animate-spin h-5 w-5 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
        <span class="text-sm" style="color: var(--color-text-secondary)">处理中...</span>
      </div>
    </div>
  {/if}

  <!-- ═══ Desktop Sidebar ═══ -->
  <aside
    class="hidden md:flex flex-col border-r transition-all duration-300 ease-in-out {sidebarCollapsed ? 'w-[72px]' : 'w-64'}"
    style="background: color-mix(in srgb, var(--color-bg-elevated) 92%, rgba(139,92,246,0.06)); border-color: var(--color-border)"
  >
    <!-- Logo -->
    <div class="flex items-center gap-3 px-5 h-16 border-b cursor-pointer" style="border-color: var(--color-border)" role="button" tabindex="0" onclick={() => navigate('dashboard')} onkeydown={(e) => { if (e.key === 'Enter') navigate('dashboard'); }}>
      <div class="w-8 h-8 rounded-xl flex items-center justify-center flex-shrink-0" style="background: var(--gradient-brand); box-shadow: var(--shadow-glow)">
        <span class="material-symbols-outlined text-white text-lg">extension</span>
      </div>
      {#if !sidebarCollapsed}
        <div class="overflow-hidden">
          <h1 class="text-base font-bold tracking-tight" style="color: var(--color-text)">ModuForge</h1>
          <p class="text-[11px] leading-tight" style="color: var(--color-text-muted)">Module Builder</p>
        </div>
      {/if}
      <button
        class="ml-auto p-1.5 rounded-lg transition-colors"
        style="color: var(--color-text-muted)"
        onclick={() => sidebarCollapsed = !sidebarCollapsed}
      >
        <span class="material-symbols-outlined text-[18px]">{sidebarCollapsed ? 'chevron_right' : 'chevron_left'}</span>
      </button>
    </div>

    <!-- Nav Items -->
    <nav class="flex-1 p-3 space-y-1 overflow-y-auto">
      {#each navItems as item}
        {@const isActive = current === item.id}
        <button
          class="nav-item w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-200 text-[14px] font-medium group min-h-[44px] relative"
          style={isActive
            ? 'color: var(--color-primary)'
            : 'color: var(--color-text-secondary)'}
          onclick={() => navigate(item.id as Route, item.id === 'editor' || item.id === 'builds' ? projectId : undefined)}
          title={sidebarCollapsed ? item.label : undefined}
        >
          <!-- Active indicator bar -->
          {#if isActive}
            <div class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r-full" style="background: var(--gradient-brand)"></div>
          {/if}
          <!-- Hover background -->
           <div class="absolute inset-0 rounded-xl transition-opacity duration-200 {isActive ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}"
                style="background: {isActive ? 'rgba(139,92,246,0.12)' : 'transparent'}; border: 1px solid {isActive ? 'rgba(139,92,246,0.2)' : 'transparent'}">
          </div>
          <span class="material-symbols-outlined text-[20px] flex-shrink-0 relative z-10" style={isActive ? 'color: var(--color-primary)' : 'color: var(--color-text-muted)'}>
            {item.icon}
          </span>
          {#if !sidebarCollapsed}
            <span class="relative z-10">{item.label}</span>
          {/if}
        </button>
      {/each}
    </nav>

    <!-- Bottom section -->
    <div class="p-3 border-t" style="border-color: var(--color-border)">
      <!-- Theme toggle -->
      <button
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-150 text-[14px] font-medium min-h-[44px] hover:bg-[var(--color-surface)]"
        style="color: var(--color-text-secondary)"
        onclick={toggleTheme}
      >
        <span class="material-symbols-outlined text-[20px]" style="color: var(--color-text-muted)">{theme === 'dark' ? 'light_mode' : 'dark_mode'}</span>
        {#if !sidebarCollapsed}
          <span>{theme === 'dark' ? '浅色模式' : '深色模式'}</span>
        {/if}
      </button>
      <LocaleSwitcher compact={sidebarCollapsed} />
      <button
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-200 text-[14px] font-medium mt-1 min-h-[44px] group/logout hover:bg-[var(--color-surface)]"
        style="color: var(--color-text-secondary)"
        onclick={confirmLogout}
      >
        <span class="material-symbols-outlined text-[20px] transition-colors group-hover/logout:text-[var(--color-error)]" style="color: var(--color-text-muted)">logout</span>
        {#if !sidebarCollapsed}
          <span class="transition-colors group-hover/logout:text-[var(--color-error)]">{$t('nav.logout')}</span>
        {/if}
      </button>
    </div>
  </aside>

  <!-- ═══ Main Content ═══ -->
  <main class="flex-1 flex flex-col overflow-hidden pb-16 md:pb-0" style="background: var(--color-bg)">
    <!-- Mobile Header -->
    <header class="md:hidden flex items-center justify-between px-4 h-14 border-b flex-shrink-0 glass" style="border-color: var(--color-border)">
      <div class="flex items-center gap-2.5" onclick={() => navigate('dashboard')}>
        <div class="w-7 h-7 rounded-lg flex items-center justify-center" style="background: var(--gradient-brand)">
          <span class="material-symbols-outlined text-white text-sm">extension</span>
        </div>
        <span class="text-sm font-bold" style="color: var(--color-text)">ModuForge</span>
      </div>
      <div class="flex items-center gap-2">
        <button class="p-2 rounded-lg min-w-[44px] min-h-[44px] flex items-center justify-center" style="color: var(--color-text-muted)" onclick={toggleTheme}>
          <span class="material-symbols-outlined text-[20px]">{theme === 'dark' ? 'light_mode' : 'dark_mode'}</span>
        </button>
        <LocaleSwitcher />
        <button class="p-2 rounded-lg min-w-[44px] min-h-[44px] flex items-center justify-center" style="color: var(--color-text-muted)" onclick={confirmLogout} title="退出登录">
          <span class="material-symbols-outlined text-[20px]">logout</span>
        </button>
      </div>
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
    {:else if current === 'ai'}
      <div class="flex-1 overflow-hidden"><AIPage /></div>
    {:else if current === 'projects'}
      <div class="flex-1 overflow-y-auto page-enter">
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div class="flex items-center gap-3 mb-4">
            <div class="flex-1">
              <h2 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">{$t('nav.projects')}</h2>
              <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">{$t('project.select_or_create')}</p>
            </div>
            <button class="btn-primary flex items-center gap-2" onclick={() => showCreateModal = true}>
              <span class="material-symbols-outlined text-[18px]">add</span>
              新建
            </button>
          </div>
          <div class="relative mb-4">
            <span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-neutral-400 text-[18px]">search</span>
            <input type="text" placeholder="搜索项目..." class="input-field" style="padding-left: 36px;" bind:value={projectSearch} />
          </div>
          {#if filteredProjects.length === 0 && projectSearch}
            <div class="text-center py-16">
              <span class="material-symbols-outlined text-5xl mb-3 block" style="color: var(--color-text-muted)">search_off</span>
              <p class="text-[var(--color-text-secondary)]">没有找到匹配 "{projectSearch}" 的项目</p>
            </div>
          {:else}
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              <!-- New Project Card -->
              <button
                class="border-2 border-dashed rounded-2xl p-8 text-center transition-all duration-200 cursor-pointer group min-h-[200px] flex flex-col items-center justify-center"
                style="border-color: var(--color-border); color: var(--color-text-secondary)"
                onclick={() => showCreateModal = true}
              >
                <div class="w-12 h-12 rounded-2xl flex items-center justify-center mx-auto mb-3 group-hover:scale-110 transition-transform" style="background: var(--gradient-brand-subtle)">
                  <span class="material-symbols-outlined text-2xl" style="color: var(--color-primary)">add</span>
                </div>
                <p class="font-semibold" style="color: var(--color-text)">{$t('project.new')}</p>
                <p class="text-xs mt-1" style="color: var(--color-text-muted)">Create a new universal module</p>
              </button>
              <!-- Existing Projects -->
              {#each filteredProjects as project (project.id)}
                <div
                  class="project-card rounded-2xl border p-5 cursor-pointer group relative overflow-hidden"
                  style="background: var(--color-bg-elevated); border-color: var(--color-border)"
                  onclick={() => navigate('editor', project.id)}
                >
                  <!-- Hover gradient -->
                  <div class="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-300" style="background: linear-gradient(135deg, rgba(139,92,246,0.05) 0%, rgba(6,182,212,0.03) 100%)"></div>
                  
                  <div class="relative z-10">
                    <div class="flex items-start justify-between mb-3">
                      <div class="w-11 h-11 rounded-xl flex items-center justify-center group-hover:scale-110 transition-transform duration-300" style="background: var(--gradient-brand-subtle)">
                        <span class="material-symbols-outlined" style="color: var(--color-primary)">folder</span>
                      </div>
                      <button class="p-2 rounded-lg opacity-0 group-hover:opacity-100 transition-all duration-200 min-w-[44px] min-h-[44px] flex items-center justify-center hover:bg-[var(--color-error-light)]"
                              style="color: var(--color-text-muted)"
                              onclick={(e) => { e.stopPropagation(); confirmDelete(project.id); }}>
                        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-error)">delete</span>
                      </button>
                    </div>
                    <h3 class="font-semibold mb-1 group-hover:text-[var(--color-primary)] transition-colors" style="color: var(--color-text)">{project.name}</h3>
                    <p class="text-xs line-clamp-2 leading-relaxed" style="color: var(--color-text-muted)">{project.description || 'No description'}</p>
                    <div class="flex items-center gap-2 mt-3">
                      <span class="badge" style="background: var(--color-primary-light); color: var(--color-primary)">Universal</span>
                      <span class="text-[11px]" style="color: var(--color-text-muted)">{new Date(project.updated_at).toLocaleDateString()}</span>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- Create Project Modal -->
      {#if showCreateModal}
        <div class="fixed inset-0 z-50 flex items-center justify-center p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => showCreateModal = false}>
          <div class="w-full max-w-md rounded-2xl shadow-xl p-6 border animate-[scaleIn_0.2s_ease-out]"
               style="background: var(--color-bg-elevated); border-color: var(--color-border)"
               onclick={(e) => e.stopPropagation()}>
            <h3 class="text-lg font-bold mb-4" style="color: var(--color-text)">{$t('project.new')}</h3>
            <div class="space-y-4">
              <div>
                <label class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">项目名称</label>
                <input type="text" bind:value={newProjectName} placeholder="My Awesome Module"
                       class="input-field" />
              </div>
              <div class="flex items-center gap-1.5 px-2 py-1 rounded-md w-fit" style="background: var(--color-primary-light); color: var(--color-primary)">
                <span class="material-symbols-outlined text-[12px]">hub</span>
                <span class="text-[10px] font-medium">Universal · Magisk + KSU + APatch</span>
              </div>
              <div>
                <label class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">描述</label>
                <textarea bind:value={newProjectDesc} placeholder="Optional description..." rows="3"
                          class="input-field resize-none"></textarea>
              </div>
            </div>
            <div class="flex justify-end gap-3 mt-6">
              <button class="btn-ghost" onclick={() => showCreateModal = false}>取消</button>
              <button class="btn-primary disabled:opacity-50"
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

  <!-- ═══ Mobile Bottom Nav ═══ -->
  <nav class="md:hidden fixed bottom-0 left-0 right-0 h-16 glass flex items-center justify-around px-2 z-40" style="border-top: 1px solid var(--color-border)">
    {#each bottomNavItems as item}
      {@const isActive = current === item.id}
      <button
        class="flex flex-col items-center gap-0.5 py-1.5 px-3 rounded-xl transition-all duration-150 min-w-[60px] min-h-[44px]"
        style={isActive ? 'color: var(--color-primary)' : 'color: var(--color-text-muted)'}
        onclick={() => navigate(item.id as Route, item.id === 'editor' ? projectId : undefined)}
      >
        <span class="material-symbols-outlined text-[22px]">{item.icon}</span>
        <span class="text-[10px] font-medium leading-tight">{item.label}</span>
      </button>
    {/each}
  </nav>
</div>
{/if}

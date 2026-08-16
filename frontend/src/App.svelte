<script lang="ts">
  import { onMount } from 'svelte';
  import type { Component } from 'svelte';
  import { t } from '$lib/i18n';
  import { client, getToken, setToken, clearToken, AuthError, hasValidToken, tryRefreshToken } from '$lib/api/client';
  import { globalLoading } from '$lib/stores/loading.svelte';
  import { ws } from '$lib/ws';
  import { debounce } from '$lib/utils/performance';
  // ListTransition removed - using {#each} for Svelte 5 compatibility
  import LocaleSwitcher from '$lib/components/LocaleSwitcher.svelte';
  import NotificationBell from '$lib/components/NotificationBell.svelte';
  import Onboarding from '$lib/components/Onboarding.svelte';
  import SearchPanel from '$lib/components/SearchPanel.svelte';
  import ShortcutsHelp from '$lib/components/ShortcutsHelp.svelte';
  import AuthPage from './routes/auth/+page.svelte';
  import Toast from '$lib/components/ui/Toast.svelte';
  import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
  import QuickActions from '$lib/components/QuickActions.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { initTheme, getTheme, setTheme } from '$lib/stores/theme';
  import PageSkeleton from '$lib/components/ui/PageSkeleton.svelte';

  // Lazy-loaded route components — only imported when navigated to
  const lazyRoutes: Record<string, () => Promise<{ default: Component }>> = {
    'market': () => import('./routes/market/+page.svelte'),
    'market-publish': () => import('./routes/market/publish/+page.svelte'),
    'dashboard': () => import('./routes/dashboard/+page.svelte'),
    'editor': () => import('$lib/components/editor/EditorWorkspace.svelte'),
    'builds': () => import('$lib/components/editor/BuildWorkspace.svelte'),
    'tests': () => import('./routes/projects/[id]/tests/+page.svelte'),
    'settings': () => import('./routes/settings/+page.svelte'),
    'ai': () => import('./routes/ai/+page.svelte'),
    'mcp': () => import('./routes/mcp/+page.svelte'),
    'devices': () => import('./routes/devices/+page.svelte'),
    'glossary': () => import('./routes/glossary/+page.svelte'),
    'crash': () => import('./routes/crash/+page.svelte'),
  };

  let routeComponent = $state<Component | null>(null);
  let routeKey = $state('');
  let routeLoading = $state(false);

  async function loadRoute(route: string) {
    if (route === 'auth' || route === 'projects') { routeComponent = null; routeKey = route; return; }
    const loader = lazyRoutes[route];
    if (!loader) { routeComponent = null; routeKey = route; return; }
    if (routeKey === route && routeComponent) return; // already loaded
    routeLoading = true;
    try {
      const mod = await loader();
      routeComponent = mod.default;
      routeKey = route;
    } catch { routeComponent = null; }
    routeLoading = false;
  }

  type Route = 'auth' | 'projects' | 'editor' | 'builds' | 'tests' | 'settings' | 'market' | 'market-publish' | 'dashboard' | 'ai' | 'mcp' | 'devices' | 'crash' | 'glossary';

  let current = $state<Route>('auth');
  let projectId = $state('');
  let token = $state<string | null>(null);
  let sidebarCollapsed = $state(false);
  let mounted = $state(false);
  let mobileMenuOpen = $state(false);
  let offline = $state(false);
  let errorCaught = $state<string | null>(null);
  let errorDetail = $state<string | null>(null);
  let errorRecoverable = $state(true);
  let showOnboarding = $state(false);

  // Search panel (Ctrl+K)
  let showSearch = $state(false);
  // Shortcuts help (? key)
  let showShortcuts = $state(false);
  // Quick actions (Ctrl+Q)
  let quickActionsOpen = $state(false);

  // Theme
  let themeMode = $state(getTheme());
  function toggleTheme() {
    const next = themeMode === 'light' ? 'dark' : themeMode === 'dark' ? 'system' : 'light';
    themeMode = next;
    setTheme(next);
  }

  // Confirm dialog
  let confirmOpen = $state(false);
  let confirmTitle = $state('');
  let confirmMessage = $state('');
  let confirmVariant = $state<'primary' | 'danger'>('primary');
  let confirmCallback = $state<(() => void) | null>(null);

  // Project state
  interface Project { id: string; name: string; module_type: string; description: string; created_at: string; updated_at: string; }
  let projects = $state<Project[]>([]);
  let projectSearchRaw = $state('');
  let projectSearch = $state('');
  const debouncedProjectSearch = debounce((val: string) => { projectSearch = val; }, 200);
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
    // Preload the route component
    loadRoute(route);
    const paths: Record<string, string> = {
      'market': '/market', 'market-publish': '/market/publish', 'dashboard': '/dashboard',
      'projects': '/projects', 'settings': '/settings', 'ai': '/ai', 'mcp': '/mcp',
      'devices': '/devices', 'glossary': '/glossary',
    };
    if (paths[route]) history.pushState(null, '', paths[route]);
    else if (route === 'editor' && id) history.pushState(null, '', `/projects/${id}`);
    else if (route === 'builds' && id) history.pushState(null, '', `/projects/${id}/build`);
    else if (route === 'tests' && id) history.pushState(null, '', `/projects/${id}/tests`);
  }

  function handleAuth(newToken: string, action: 'login' | 'register' = 'login', user?: {username: string; email: string}, rememberMe = true) {
    token = newToken;
    setToken(newToken, rememberMe);
    if (user) {
      localStorage.setItem('moduforge_username', user.username);
      localStorage.setItem('moduforge_email', user.email);
    }
    ws.connect();
    current = 'projects';
    loadProjects();
    scheduleTokenRefresh();
    // Preload projects route component
    loadRoute('projects');
    if (action === 'register') {
      toast('注册成功！正在跳转...', 'success');
    } else {
      toast('登录成功！欢迎回来', 'success');
    }
  }

  // Auto-refresh JWT before expiry
  let refreshTimer: ReturnType<typeof setTimeout> | null = null;

  function getTokenExpiry(tokenStr: string): number | null {
    try {
      const parts = tokenStr.split('.');
      if (parts.length !== 3) return null;
      const payload = JSON.parse(atob(parts[1]));
      return payload.exp ? payload.exp * 1000 : null; // exp is in seconds
    } catch { return null; }
  }

  async function scheduleTokenRefresh(_initialToken?: string) {
    if (refreshTimer) clearTimeout(refreshTimer);
    
    // Always read the current token from storage (not the captured closure)
    const currentToken = getToken();
    if (!currentToken) return;
    
    const exp = getTokenExpiry(currentToken);
    if (!exp) return;

    const now = Date.now();
    const msUntilExpiry = exp - now;
    // Refresh when 5 minutes left (or immediately if already close to expiry)
    const refreshIn = Math.max(msUntilExpiry - 5 * 60 * 1000, 5000);

    refreshTimer = setTimeout(async () => {
      try {
        const latestToken = getToken();
        if (!latestToken) return;
        const r = await fetch('/api/v1/auth/refresh', {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${latestToken}`,
            'Content-Type': 'application/json',
          },
          body: '{}',
        });
        if (r.ok) {
          const data = await r.json();
          token = data.token;
          setToken(data.token, true);
          scheduleTokenRefresh(); // schedule next refresh with new token
          // Reconnect WS with fresh token
          try { ws.disconnect(); ws.connect(); } catch {}
        }
      } catch {}
    }, refreshIn);
  }

  function logout() {
    clearToken();
    token = null;
    current = 'auth';
    routeComponent = null;
    routeKey = '';
    projects = [];
    ws.disconnect();
    toast('已退出登录', 'info');
  }

  async function loadProjects() {
    try {
      projects = await client.get<Project[]>('/projects');
    } catch (e: any) {
      if (e instanceof AuthError) {
        // Token expired — go to login but DON'T clear token yet
        // (user might refresh to get a new token)
        current = 'auth';
        toast('登录已过期，请重新登录', 'warning');
      } else {
        toast(e.message || '加载项目失败', 'error');
      }
    }
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
    globalLoading.inc();
    try {
      await client.del(`/projects/${id}`);
      projects = projects.filter(p => p.id !== id);
      if (projectId === id) { projectId = ''; navigate('projects'); }
      toast('项目已删除', 'success');
    } catch (e: any) { toast(e.message, 'error'); }
    globalLoading.dec();
  }

  function confirmLogout() {
    confirmTitle = '退出登录';
    confirmMessage = '确定要退出登录吗？';
    confirmVariant = 'primary';
    confirmCallback = logout;
    confirmOpen = true;
  }

  function handleQuickAction(action: string) {
    switch (action) {
      case 'new-project':
        showCreateModal = true;
        break;
      case 'open-project':
        navigate('projects');
        break;
      case 'ai-chat':
        navigate('ai');
        break;
      case 'build':
        if (projectId) navigate('builds', projectId);
        else toast('请先选择一个项目', 'info');
        break;
      case 'settings':
        navigate('settings');
        break;
      case 'help':
        showShortcuts = true;
        break;
    }
  }

  onMount(() => {
    void (async () => {
      mounted = true;
      offline = !navigator.onLine;
      const saved = getToken();
      if (saved && hasValidToken()) {
        token = saved;
        current = 'projects';
        loadProjects();
        scheduleTokenRefresh();
        // Connect WebSocket if we have a token
        try { ws.connect(); } catch {}
      } else if (saved) {
        // Token exists but expired — try refresh
        const refreshed = await tryRefreshToken();
        if (refreshed) {
          token = getToken();
          current = 'projects';
          loadProjects();
          scheduleTokenRefresh();
          try { ws.connect(); } catch {}
        } else {
          // Refresh failed — clear and show login
          clearToken();
        }
      }

      // Show onboarding for first-time users
      if (token && !localStorage.getItem('moduforge_onboarded')) {
        showOnboarding = true;
      }

      // Initialize theme
      initTheme();
      themeMode = getTheme();
    })();

    function handlePopState() {
      const path = window.location.pathname;
      const routeMap: Record<string, Route> = {
        '/market/publish': 'market-publish', '/market': 'market', '/dashboard': 'dashboard',
        '/settings': 'settings', '/ai': 'ai', '/mcp': 'mcp', '/devices': 'devices',
        '/glossary': 'glossary', '/crash': 'crash',
      };
      if (routeMap[path]) { current = routeMap[path]; loadRoute(routeMap[path]); }
      else if (path === '/projects') { current = 'projects'; routeComponent = null; routeKey = 'projects'; }
      else if (path.startsWith('/projects/') && path.includes('/build')) { current = 'builds'; projectId = path.split('/')[2] || ''; loadRoute('builds'); }
      else if (path.startsWith('/projects/') && path.includes('/tests')) { current = 'tests'; projectId = path.split('/')[2] || ''; loadRoute('tests'); }
      else if (path.startsWith('/projects/')) { current = 'editor'; projectId = path.split('/')[2] || ''; loadRoute('editor'); }
    }
    handlePopState();
    window.addEventListener('popstate', handlePopState);

    const goOnline = () => { offline = false; toast('网络已恢复', 'success'); };
    const goOffline = () => { offline = true; toast('网络已断开，部分功能不可用', 'error'); };
    window.addEventListener('online', goOnline);
    window.addEventListener('offline', goOffline);

    const handleGlobalError = (event: ErrorEvent) => {
      errorCaught = event.message || '未知错误';
      errorDetail = event.filename && event.lineno ? `${event.filename}:${event.lineno}` : null;
      errorRecoverable = true;
      event.preventDefault();
    };
    window.addEventListener('error', handleGlobalError);

    const handleRejection = (event: PromiseRejectionEvent) => {
      errorCaught = event.reason?.message || '未处理的 Promise 异常';
      errorDetail = event.reason?.stack ? event.reason.stack.split('\n')[0] : null;
      errorRecoverable = true;
      event.preventDefault();
    };
    window.addEventListener('unhandledrejection', handleRejection);

    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        showSearch = true;
      }
      if ((e.ctrlKey || e.metaKey) && e.key === '/') {
        e.preventDefault();
        showShortcuts = true;
      }
      if (e.key === 'Escape') {
        showSearch = false;
        showShortcuts = false;
      }
      if (e.key === '?' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        showShortcuts = true;
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'q') {
        e.preventDefault();
        quickActionsOpen = !quickActionsOpen;
      }
    };
    window.addEventListener('keydown', handleKeyDown);

    // WS notifications
    const unsubNotif = ws.on('notification', () => {
      // NotificationBell will poll unread count
    });

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      unsubNotif();
      window.removeEventListener('popstate', handlePopState);
      window.removeEventListener('online', goOnline);
      window.removeEventListener('offline', goOffline);
      window.removeEventListener('error', handleGlobalError);
      window.removeEventListener('unhandledrejection', handleRejection);
    };
  });

  const navItems = $derived([
    { id: 'dashboard', icon: 'monitoring', label: $t('nav.dashboard') },
    { id: 'projects', icon: 'folder', label: $t('nav.projects') },
    ...(projectId ? [
      { id: 'editor', icon: 'code', label: $t('nav.editor') },
      { id: 'builds', icon: 'build', label: $t('nav.builds') },
      { id: 'tests', icon: 'bug_report', label: '测试' },
    ] : []),
    { id: 'ai', icon: 'psychology', label: 'AI 助手' },
    { id: 'mcp', icon: 'hub', label: 'MCP' },
    { id: 'glossary', icon: 'menu_book', label: '术语表' },
    { id: 'devices', icon: 'devices', label: $t('nav.devices') },
    { id: 'market', icon: 'storefront', label: $t('nav.market') },
    { id: 'crash', icon: 'bug_report', label: '崩溃分析' },
    { id: 'settings', icon: 'settings', label: $t('nav.settings') },
  ]);

  const bottomNavItems = $derived([
    { id: 'projects', icon: 'folder', label: '项目' },
    { id: 'ai', icon: 'psychology', label: 'AI' },
    { id: 'devices', icon: 'devices', label: '设备' },
    { id: 'market', icon: 'storefront', label: '市场' },
    { id: 'settings', icon: 'settings', label: '设置' },
  ]);
</script>

<style>
  .project-card {
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .project-card:hover {
    border-color: var(--color-primary);
    box-shadow: 0 8px 32px color-mix(in srgb, var(--color-primary) 12%, transparent), 0 0 0 1px color-mix(in srgb, var(--color-primary) 8%, transparent);
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
      <p class="text-sm mb-2" style="color: var(--color-text-secondary)">{errorCaught}</p>
      {#if errorDetail}
        <p class="text-xs mb-4 font-mono" style="color: var(--color-text-muted); word-break: break-all">{errorDetail}</p>
      {/if}
      <div class="flex gap-3 justify-center">
        <button class="btn-primary" onclick={() => { errorCaught = null; errorDetail = null; }}>重试</button>
        <button class="btn-ghost" onclick={() => { errorCaught = null; errorDetail = null; window.location.reload(); }}>刷新页面</button>
      </div>
    </div>
  </div>
{/if}

<ConfirmDialog
  open={confirmOpen}
  title={confirmTitle}
  message={confirmMessage}
  variant={confirmVariant}
  onConfirm={() => { confirmOpen = false; confirmCallback?.(); }}
  onCancel={() => { confirmOpen = false; confirmCallback = null; }}
/>

{#if !token}
  <AuthPage onAuth={handleAuth} />
{:else}
{#if showOnboarding}
  <Onboarding onDone={() => showOnboarding = false} />
{/if}

{#if showSearch}
<SearchPanel onClose={() => showSearch = false} onNavigate={(route, id) => { showSearch = false; navigate(route as Route, id); }} />
{/if}
{#if showShortcuts}
<ShortcutsHelp onClose={() => showShortcuts = false} />
{/if}

{#if current !== 'ai'}
<QuickActions onAction={handleQuickAction} />
{/if}
<!-- AI page: fullscreen mode — no sidebar, no header -->
<div class="flex h-screen overflow-hidden {current === 'ai' ? 'ai-fullscreen' : ''}" style="background: var(--color-bg)">
  <!-- Global Loading Overlay -->
  {#if globalLoading.value}
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
    style="background: color-mix(in srgb, var(--color-bg-elevated) 92%, color-mix(in srgb, var(--color-primary) 6%, transparent)); border-color: var(--color-border)"
  >
    <!-- Logo -->
    <div class="flex items-center gap-3 px-5 h-16 border-b cursor-pointer" style="border-color: var(--color-border)" role="button" tabindex="0" onclick={() => navigate('projects')} onkeydown={(e) => { if (e.key === 'Enter') navigate('projects'); }}>
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
        aria-label={sidebarCollapsed ? '展开侧边栏' : '折叠侧边栏'}
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
                style="background: {isActive ? 'color-mix(in srgb, var(--color-primary) 12%, transparent)' : 'transparent'}; border: 1px solid {isActive ? 'color-mix(in srgb, var(--color-primary) 20%, transparent)' : 'transparent'}">
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
        aria-label={themeMode === 'dark' ? '浅色模式' : themeMode === 'light' ? '深色模式' : '跟随系统'}
      >
        <span class="material-symbols-outlined text-[20px]" style="color: var(--color-text-muted)">{themeMode === 'dark' ? 'dark_mode' : themeMode === 'light' ? 'light_mode' : 'brightness_auto'}</span>
        {#if !sidebarCollapsed}
          <span>{themeMode === 'dark' ? '浅色模式' : themeMode === 'light' ? '深色模式' : '跟随系统'}</span>
        {/if}
      </button>
      <div class="flex items-center justify-center {sidebarCollapsed ? '' : 'px-3'} py-1">
        <NotificationBell />
      </div>
      <button
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-150 text-[14px] font-medium min-h-[44px] hover:bg-[var(--color-surface)]"
        style="color: var(--color-text-secondary)"
        onclick={() => showShortcuts = true}
        aria-label="快捷键"
      >
        <span class="material-symbols-outlined text-[20px]" style="color: var(--color-text-muted)">keyboard</span>
        {#if !sidebarCollapsed}
          <span>快捷键</span>
        {/if}
      </button>
      <LocaleSwitcher compact={sidebarCollapsed} />
      <button
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-200 text-[14px] font-medium mt-1 min-h-[44px] group/logout hover:bg-[var(--color-surface)]"
        style="color: var(--color-text-secondary)"
        onclick={confirmLogout}
        aria-label="{$t('nav.logout')}"
      >
        <span class="material-symbols-outlined text-[20px] transition-colors group-hover/logout:text-[var(--color-error)]" style="color: var(--color-text-muted)">logout</span>
        {#if !sidebarCollapsed}
          <span class="transition-colors group-hover/logout:text-[var(--color-error)]">{$t('nav.logout')}</span>
        {/if}
      </button>
    </div>
  </aside>

  <!-- ═══ Main Content ═══ -->
  <main class="flex-1 flex flex-col overflow-hidden {current === 'ai' ? '' : 'pb-16 md:pb-0'}" style="background: var(--color-bg)">
    <!-- Mobile Header (hidden on AI page for fullscreen) -->
    {#if current !== 'ai'}
    <header class="md:hidden flex items-center justify-between px-4 h-14 border-b flex-shrink-0 glass" style="border-color: var(--color-border)">
      <div class="flex items-center gap-2.5" role="button" tabindex="0" onclick={() => navigate('projects')} onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); navigate('projects'); } }}>
        <div class="w-7 h-7 rounded-lg flex items-center justify-center" style="background: var(--gradient-brand)">
          <span class="material-symbols-outlined text-white text-sm">extension</span>
        </div>
        <span class="text-sm font-bold" style="color: var(--color-text)">ModuForge</span>
      </div>
      <div class="flex items-center gap-1">
        <NotificationBell />
        <button class="p-2 rounded-lg min-w-[44px] min-h-[44px] flex items-center justify-center" style="color: var(--color-text-muted)" onclick={toggleTheme} aria-label="切换主题">
          <span class="material-symbols-outlined text-[20px]">{themeMode === 'dark' ? 'dark_mode' : themeMode === 'light' ? 'light_mode' : 'brightness_auto'}</span>
        </button>
        <LocaleSwitcher />
        <button class="p-2 rounded-lg min-w-[44px] min-h-[44px] flex items-center justify-center" style="color: var(--color-text-muted)" onclick={confirmLogout} title="退出登录">
          <span class="material-symbols-outlined text-[20px]">logout</span>
        </button>
      </div>
    </header>
    {/if}

    <!-- Page Content -->
    {#if current === 'projects'}
      <div class="flex-1 overflow-y-auto page-enter min-w-0">
        <div class="w-full p-4 md:p-6 max-w-7xl mx-auto">
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
            <input type="text" placeholder="搜索项目..." class="input-field" style="padding-left: 36px;" bind:value={projectSearchRaw} oninput={() => debouncedProjectSearch(projectSearchRaw)} />
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
              {#each filteredProjects as project, i (project.id)}
                <div
                  role="button"
                  tabindex="0"
                  class="project-card rounded-2xl border p-5 cursor-pointer group relative overflow-hidden"
                  style="background: var(--color-bg-elevated); border-color: var(--color-border)"
                  onclick={() => navigate('editor', project.id)}
                  onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); navigate('editor', project.id); } }}
                >
                  <!-- Hover gradient -->
                  <div class="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-300" style="background: linear-gradient(135deg, color-mix(in srgb, var(--color-primary) 5%, transparent) 0%, color-mix(in srgb, var(--color-info) 3%, transparent) 100%)"></div>
                  
                  <div class="relative z-10">
                    <div class="flex items-start justify-between mb-3">
                      <div class="w-11 h-11 rounded-xl flex items-center justify-center group-hover:scale-110 transition-transform duration-300" style="background: var(--gradient-brand-subtle)">
                        <span class="material-symbols-outlined" style="color: var(--color-primary)">folder</span>
                      </div>
                      <button class="p-2 rounded-lg opacity-0 group-hover:opacity-100 transition-all duration-200 min-w-[44px] min-h-[44px] flex items-center justify-center hover:bg-[var(--color-error-light)]"
                              style="color: var(--color-text-muted)"
                              onclick={(e) => { e.stopPropagation(); confirmDelete(project.id); }}
                              aria-label="删除项目 {project.name}">
                        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-error)">delete</span>
                      </button>
                    </div>
                    <h3 class="font-semibold mb-1 group-hover:text-[var(--color-primary)] transition-colors" style="color: var(--color-text)">{project.name}</h3>
                    <p class="text-xs leading-relaxed" style="color: var(--color-text-muted); max-height: 10rem; overflow-y: auto; scrollbar-width: thin; white-space: pre-wrap">{project.description || 'No description'}</p>
                    <div class="flex items-center gap-2 mt-3">
                      <span class="badge" style="background: var(--color-primary-light); color: var(--color-primary)">{project.module_type ? project.module_type.charAt(0).toUpperCase() + project.module_type.slice(1) : 'Universal'}</span>
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
        <div class="fixed inset-0 z-50 flex items-center justify-center p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showCreateModal = false; }} onkeydown={(e) => { if (e.key === 'Escape') showCreateModal = false; }}>
          <div class="w-full max-w-md rounded-2xl shadow-xl p-6 border animate-[scaleIn_0.2s_ease-out]"
               style="background: var(--color-bg-elevated); border-color: var(--color-border)"
               role="dialog" aria-modal="true" aria-labelledby="new-project-title" tabindex="-1">
            <h3 id="new-project-title" class="text-lg font-bold mb-4" style="color: var(--color-text)">{$t('project.new')}</h3>
            <div class="space-y-4">
              <div>
                <label for="new-project-name" class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">项目名称</label>
                <input id="new-project-name" type="text" bind:value={newProjectName} placeholder="My Awesome Module"
                       class="input-field" />
              </div>
              <div class="flex items-center gap-1.5 px-2 py-1 rounded-md w-fit" style="background: var(--color-primary-light); color: var(--color-primary)">
                <span class="material-symbols-outlined text-[12px]">hub</span>
                <span class="text-[10px] font-medium">Universal · Magisk + KSU + APatch</span>
              </div>
              <div>
                <label for="new-project-desc" class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">描述</label>
                <textarea id="new-project-desc" bind:value={newProjectDesc} placeholder="Optional description..." rows="3"
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
    {:else}
      <!-- Lazy-loaded route component -->
      <div class="flex-1 overflow-y-auto overflow-x-hidden flex flex-col min-h-0 min-w-0 {current === 'ai' ? '' : 'page-enter'}">
        {#if routeLoading}
          <PageSkeleton variant={current === 'editor' ? 'editor' : current === 'market' ? 'market' : current === 'market-publish' ? 'market' : 'list'} />
        {:else if routeComponent}
          {#key routeKey}
            {@const RouteComp = routeComponent}
            <RouteComp
              {projectId}
              onNavigate={(route: string, id?: string) => navigate(route as Route, id)}
            />
          {/key}
        {/if}
      </div>
    {/if}
  </main>

  <!-- ═══ Mobile Bottom Nav (hidden on AI page) ═══ -->
  {#if current !== 'ai'}
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
    <button
      class="flex flex-col items-center gap-0.5 py-1.5 px-3 rounded-xl transition-all duration-150 min-w-[60px] min-h-[44px]"
      style={mobileMenuOpen ? 'color: var(--color-primary)' : 'color: var(--color-text-muted)'}
      onclick={() => mobileMenuOpen = !mobileMenuOpen}
    >
      <span class="material-symbols-outlined text-[22px]">{mobileMenuOpen ? 'close' : 'more_horiz'}</span>
      <span class="text-[10px] font-medium leading-tight">更多</span>
    </button>
  </nav>
  {/if}

  {#if mobileMenuOpen && current !== 'ai'}
    <div class="fixed inset-0 z-50 md:hidden" role="presentation" onclick={() => mobileMenuOpen = false} onkeydown={(e) => { if (e.key === 'Escape') mobileMenuOpen = false; }}>
      <div class="absolute bottom-16 left-2 right-2 rounded-2xl border shadow-2xl p-4 grid grid-cols-3 gap-3"
           style="background: var(--color-bg-elevated); border-color: var(--color-border)"
           role="presentation"
           onclick={(e) => e.stopPropagation()}>
        {#each [
          { id: 'dashboard', icon: 'monitoring', label: '仪表盘' },
          { id: 'glossary', icon: 'menu_book', label: '术语表' },
          { id: 'crash', icon: 'bug_report', label: '崩溃分析' },
          { id: 'market-publish', icon: 'publish', label: '发布模块' },
        ] as item}
          <button class="flex flex-col items-center gap-1.5 p-3 rounded-xl hover:bg-[var(--color-surface)] transition-colors"
                  style="color: var(--color-text-secondary)"
                  onclick={() => { navigate(item.id as Route); mobileMenuOpen = false; }}>
            <span class="material-symbols-outlined text-[24px]">{item.icon}</span>
            <span class="text-[11px] font-medium">{item.label}</span>
          </button>
        {/each}
      </div>
    </div>
  {/if}
</div>
{/if}

<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { Component } from 'svelte';
  import { client, getToken, setToken, clearToken, AuthError, hasValidToken, tryRefreshToken } from '$lib/api/client';
  import { globalLoading } from '$lib/stores/loading.svelte';
  import { ws } from '$lib/ws';
  import { debounce } from '$lib/utils/performance';
  import LocaleSwitcher from '$lib/components/LocaleSwitcher.svelte';
  import Onboarding from '$lib/components/Onboarding.svelte';
  import SearchPanel from '$lib/components/SearchPanel.svelte';
  import ShortcutsHelp from '$lib/components/ShortcutsHelp.svelte';
  import AuthPage from './routes/auth/+page.svelte';
  import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
  import QuickActions from '$lib/components/QuickActions.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { initTheme, getTheme, setTheme } from '$lib/stores/theme';
  // Layout components
  import AppToast from '$lib/components/layout/AppToast.svelte';
  import AppSidebar from '$lib/components/layout/AppSidebar.svelte';
  import AppHeader from '$lib/components/layout/AppHeader.svelte';
  import AppRouter from '$lib/components/layout/AppRouter.svelte';

  // ─── Lazy route definitions ───
  const lazyRoutes: Record<string, () => Promise<{ default: Component }>> = {
    'market': () => import('./routes/market/+page.svelte'),
    'market-publish': () => import('./routes/market/publish/+page.svelte'),
    'dashboard': () => import('./routes/dashboard/+page.svelte'),
    'editor': () => import('$lib/components/editor/EditorWorkspace.svelte'),
    'builds': () => import('$lib/components/editor/BuildWorkspace.svelte'),
    'tests': () => import('./routes/projects/[id]/tests/+page.svelte'),
    'settings': () => import('./routes/settings/+page.svelte'),
    'ai': () => import('./routes/ai/+page.svelte'),
    'analytics': () => import('./routes/analytics/+page.svelte'),
    'mcp': () => import('./routes/mcp/+page.svelte'),
    'devices': () => import('./routes/devices/+page.svelte'),
    'glossary': () => import('./routes/glossary/+page.svelte'),
    'crash': () => import('./routes/crash/+page.svelte'),
    'perf': () => import('$lib/components/PerfDashboard.svelte'),
    'arch': () => import('$lib/components/ArchReport.svelte'),
    'git-ops': () => import('$lib/components/GitPanel.svelte'),
    'prompts-mgr': () => import('$lib/components/PromptManager.svelte'),
    'ensemble': () => import('$lib/components/EnsemblePanel.svelte'),
    'cache': () => import('$lib/components/CacheMonitor.svelte'),
  };

  type Route = 'auth' | 'projects' | 'editor' | 'builds' | 'tests' | 'settings' | 'market' | 'market-publish' | 'dashboard' | 'ai' | 'analytics' | 'mcp' | 'devices' | 'crash' | 'glossary' | 'perf' | 'arch' | 'git-ops' | 'prompts-mgr' | 'ensemble' | 'cache';
  interface Project { id: string; name: string; module_type: string; description: string; created_at: string; updated_at: string; }

  // ─── Global state ───
  let routeComponent = $state<Component | null>(null);
  let routeKey = $state('');
  let routeLoading = $state(false);
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
  let showSearch = $state(false);
  let showShortcuts = $state(false);
  let quickActionsOpen = $state(false);
  let themeMode = $state(getTheme());

  // Confirm dialog
  let confirmOpen = $state(false);
  let confirmTitle = $state('');
  let confirmMessage = $state('');
  let confirmVariant = $state<'primary' | 'danger'>('primary');
  let confirmCallback = $state<(() => void) | null>(null);

  // Project state
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

  // ─── Route loading ───
  async function loadRoute(route: string) {
    if (route === 'auth' || route === 'projects') { routeComponent = null; routeKey = route; return; }
    const loader = lazyRoutes[route];
    if (!loader) { routeComponent = null; routeKey = route; return; }
    if (routeKey === route && routeComponent) return;
    routeLoading = true;
    try {
      const mod = await loader();
      routeComponent = mod.default;
      routeKey = route;
    } catch { routeComponent = null; }
    routeLoading = false;
  }

  // ─── Navigation ───
  function navigate(route: Route, id?: string) {
    current = route;
    mobileMenuOpen = false;
    if (id) projectId = id;
    loadRoute(route);
    const paths: Record<string, string> = {
      'market': '/market', 'market-publish': '/market/publish', 'dashboard': '/dashboard',
      'projects': '/projects', 'settings': '/settings', 'ai': '/ai', 'analytics': '/analytics',
      'mcp': '/mcp', 'devices': '/devices', 'glossary': '/glossary',
      'perf': '/perf', 'arch': '/arch', 'git-ops': '/git-ops', 'prompts-mgr': '/prompts-mgr', 'ensemble': '/ensemble',
    };
    if (paths[route]) history.pushState(null, '', paths[route]);
    else if (route === 'editor' && id) history.pushState(null, '', `/projects/${id}`);
    else if (route === 'builds' && id) history.pushState(null, '', `/projects/${id}/build`);
    else if (route === 'tests' && id) history.pushState(null, '', `/projects/${id}/tests`);
  }

  // ─── Theme ───
  function toggleTheme() {
    const next = themeMode === 'light' ? 'dark' : themeMode === 'dark' ? 'system' : 'light';
    themeMode = next;
    setTheme(next);
  }

  // ─── Auth ───
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
    loadRoute('projects');
    if (action === 'register') { toast('注册成功！正在跳转...', 'success'); }
    else { toast('登录成功！欢迎回来', 'success'); }
  }

  // ─── Token refresh ───
  let refreshTimer: ReturnType<typeof setTimeout> | null = null;

  function getTokenExpiry(tokenStr: string): number | null {
    try {
      const parts = tokenStr.split('.');
      if (parts.length !== 3) return null;
      const payload = JSON.parse(atob(parts[1]));
      return payload.exp ? payload.exp * 1000 : null;
    } catch (e) { console.warn('parse JWT failed:', e); return null; }
  }

  async function scheduleTokenRefresh(_initialToken?: string) {
    if (refreshTimer) clearTimeout(refreshTimer);
    const currentToken = getToken();
    if (!currentToken) return;
    const exp = getTokenExpiry(currentToken);
    if (!exp) return;
    const now = Date.now();
    const msUntilExpiry = exp - now;
    const refreshIn = Math.max(msUntilExpiry - 5 * 60 * 1000, 5000);
    refreshTimer = setTimeout(async () => {
      try {
        const latestToken = getToken();
        if (!latestToken) return;
        const r = await fetch('/api/v1/auth/refresh', {
          method: 'POST',
          headers: { 'Authorization': `Bearer ${latestToken}`, 'Content-Type': 'application/json' },
          body: '{}',
        });
        if (r.ok) {
          const data = await r.json();
          token = data.token;
          setToken(data.token, true);
          scheduleTokenRefresh();
          try { ws.disconnect(); ws.connect(); } catch (e) { console.error('WS reconnect failed:', e); }
        }
      } catch (e) { console.error('token refresh failed:', e); }
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

  onDestroy(() => {
    if (refreshTimer) clearTimeout(refreshTimer);
  });

  // ─── Projects ───
  async function loadProjects() {
    try {
      projects = await client.get<Project[]>('/projects');
    } catch (e: any) {
      if (e instanceof AuthError) {
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
      case 'new-project': showCreateModal = true; break;
      case 'open-project': navigate('projects'); break;
      case 'ai-chat': navigate('ai'); break;
      case 'build':
        if (projectId) navigate('builds', projectId);
        else toast('请先选择一个项目', 'info');
        break;
      case 'settings': navigate('settings'); break;
      case 'help': showShortcuts = true; break;
    }
  }

  // ─── Lifecycle ───
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
        try { ws.connect(); } catch (e) { console.error('WS connect failed:', e); }
      } else if (saved) {
        const refreshed = await tryRefreshToken();
        if (refreshed) {
          token = getToken();
          current = 'projects';
          loadProjects();
          scheduleTokenRefresh();
          try { ws.connect(); } catch (e) { console.error('WS connect failed:', e); }
        } else {
          clearToken();
        }
      }
      if (token && !localStorage.getItem('moduforge_onboarded')) {
        showOnboarding = true;
      }
      initTheme();
      themeMode = getTheme();
    })();

    function handlePopState() {
      const path = window.location.pathname;
      const routeMap: Record<string, Route> = {
        '/market/publish': 'market-publish', '/market': 'market', '/dashboard': 'dashboard',
        '/settings': 'settings', '/ai': 'ai', '/analytics': 'analytics', '/mcp': 'mcp',
        '/devices': 'devices', '/glossary': 'glossary', '/crash': 'crash',
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
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') { e.preventDefault(); showSearch = true; }
      if ((e.ctrlKey || e.metaKey) && e.key === '/') { e.preventDefault(); showShortcuts = true; }
      if (e.key === 'Escape') { showSearch = false; showShortcuts = false; }
      if (e.key === '?' && !e.ctrlKey && !e.metaKey && !e.altKey) { showShortcuts = true; }
      if ((e.ctrlKey || e.metaKey) && e.key === 'q') { e.preventDefault(); quickActionsOpen = !quickActionsOpen; }
    };
    window.addEventListener('keydown', handleKeyDown);

    const unsubNotif = ws.on('notification', () => {});

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
</style>

<!-- Global overlays -->
<AppToast
  {offline}
  {errorCaught}
  {errorDetail}
  onDismissError={() => { errorCaught = null; errorDetail = null; }}
  onReloadPage={() => { errorCaught = null; errorDetail = null; window.location.reload(); }}
/>

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

  <div class="flex flex-col md:flex-row h-screen overflow-hidden {current === 'ai' ? 'ai-fullscreen' : ''}" style="background: var(--color-bg)">
    <!-- Global Loading Overlay -->
    {#if globalLoading.value}
      <div class="fixed inset-0 z-50 flex items-center justify-center" style="background: rgba(0,0,0,0.5); backdrop-filter: blur(4px)">
        <div class="flex items-center gap-3 px-6 py-4 rounded-2xl border" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)">
          <div class="animate-spin h-5 w-5 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
          <span class="text-sm" style="color: var(--color-text-secondary)">处理中...</span>
        </div>
      </div>
    {/if}

    <AppSidebar
      {current}
      {projectId}
      {themeMode}
      {sidebarCollapsed}
      onNavigate={navigate}
      onToggleTheme={toggleTheme}
      onToggleCollapse={() => sidebarCollapsed = !sidebarCollapsed}
      onShowShortcuts={() => showShortcuts = true}
      onConfirmLogout={confirmLogout}
    />

    <div class="flex flex-col flex-1 min-h-0 min-w-0">
    <AppHeader
      {themeMode}
      onNavigate={navigate}
      onToggleTheme={toggleTheme}
      onConfirmLogout={confirmLogout}
    />

    <AppRouter
      {current}
      {projectId}
      {routeComponent}
      {routeKey}
      {routeLoading}
      {projects}
      {filteredProjects}
      {projectSearchRaw}
      {showCreateModal}
      {newProjectName}
      {newProjectDesc}
      {creatingProject}
      {mobileMenuOpen}
      onNavigate={navigate}
      onSearchInput={(val) => { projectSearchRaw = val; debouncedProjectSearch(val); }}
      onSetShowCreateModal={(v) => showCreateModal = v}
      onSetNewProjectName={(v) => newProjectName = v}
      onSetNewProjectDesc={(v) => newProjectDesc = v}
      onCreateProject={createProject}
      onConfirmDelete={confirmDelete}
      onSetMobileMenuOpen={(v) => mobileMenuOpen = v}
    />
    </div>
  </div>
{/if}

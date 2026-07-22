<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n';
  import LocaleSwitcher from '$lib/components/LocaleSwitcher.svelte';
  import MarketPage from './routes/market/+page.svelte';
  import PublishPage from './routes/market/publish/+page.svelte';
  import AuthPage from './routes/auth/+page.svelte';
  import DashboardPage from './routes/dashboard/+page.svelte';

  type Route = 'auth' | 'projects' | 'editor' | 'builds' | 'settings' | 'market' | 'market-publish' | 'dashboard';

  let current = $state<Route>('auth');
  let projectId = $state('');
  let token = $state<string | null>(null);
  let sidebarCollapsed = $state(false);
  let mobileMenuOpen = $state(false);
  let mounted = $state(false);

  function navigate(route: Route, id?: string) {
    current = route;
    if (id) projectId = id;
    mobileMenuOpen = false;
    // Update URL
    if (route === 'market') history.pushState(null, '', '/market');
    else if (route === 'market-publish') history.pushState(null, '', '/market/publish');
    else if (route === 'dashboard') history.pushState(null, '', '/dashboard');
    else if (route === 'projects') history.pushState(null, '', '/projects');
    else if (route === 'editor' && id) history.pushState(null, '', `/projects/${id}`);
    else if (route === 'builds' && id) history.pushState(null, '', `/projects/${id}/build`);
  }

  function handleAuth(newToken: string) {
    token = newToken;
    current = 'projects';
  }

  function logout() {
    localStorage.removeItem('moduforge_token');
    token = null;
    current = 'auth';
  }

  onMount(() => {
    mounted = true;
    const saved = localStorage.getItem('moduforge_token');
    if (saved) {
      token = saved;
      current = 'projects';
    }

    function handlePopState() {
      const path = window.location.pathname;
      if (path === '/market/publish') current = 'market-publish';
      else if (path === '/market') current = 'market';
      else if (path === '/dashboard') current = 'dashboard';
      else if (path.startsWith('/projects/') && path.includes('/build')) current = 'builds';
      else if (path.startsWith('/projects/')) { current = 'editor'; projectId = path.split('/')[2] || ''; }
      else if (path === '/projects') current = 'projects';
    }
    handlePopState();
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  });

  const navItems = $derived([
    { id: 'projects', icon: 'folder', label: $t('nav.projects') },
    ...(projectId ? [
      { id: 'editor', icon: 'code', label: $t('nav.editor') },
      { id: 'builds', icon: 'build', label: $t('nav.builds') },
    ] : []),
    { id: 'settings', icon: 'settings', label: $t('nav.settings') },
    { id: 'market', icon: 'storefront', label: $t('nav.market') },
    { id: 'dashboard', icon: 'monitoring', label: $t('nav.dashboard') },
  ]);

  const bottomNavItems = $derived([
    { id: 'projects', icon: 'folder', label: $t('nav.projects') },
    ...(projectId ? [{ id: 'editor', icon: 'code', label: $t('nav.editor') }] : []),
    { id: 'market', icon: 'storefront', label: $t('nav.market') },
    { id: 'dashboard', icon: 'monitoring', label: $t('nav.dashboard') },
  ]);
</script>

{#if !token}
  <AuthPage onAuth={handleAuth} />
{:else}
<div class="flex h-screen overflow-hidden bg-[var(--color-bg)]">
  <!-- Desktop Sidebar -->
  <aside
    class="hidden md:flex flex-col border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] transition-all duration-300 ease-in-out {sidebarCollapsed ? 'w-[72px]' : 'w-64'}"
  >
    <!-- Logo -->
    <div class="flex items-center gap-3 px-5 h-16 border-b border-[var(--color-border)]">
      <div class="w-8 h-8 rounded-xl bg-gradient-to-br from-primary-500 to-primary-700 flex items-center justify-center flex-shrink-0">
        <span class="material-symbols-outlined text-white text-lg">extension</span>
      </div>
      {#if !sidebarCollapsed}
        <div class="overflow-hidden">
          <h1 class="text-base font-bold text-[var(--color-text)] tracking-tight">ModuForge</h1>
          <p class="text-[11px] text-[var(--color-text-muted)] leading-tight">Module Builder</p>
        </div>
      {/if}
      <button
        class="ml-auto p-1.5 rounded-lg hover:bg-neutral-100 text-neutral-400 transition-colors hide-mobile"
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
        onclick={logout}
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
    <header class="md:hidden flex items-center justify-between px-4 h-14 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
      <div class="flex items-center gap-2.5">
        <div class="w-7 h-7 rounded-lg bg-gradient-to-br from-primary-500 to-primary-700 flex items-center justify-center">
          <span class="material-symbols-outlined text-white text-sm">extension</span>
        </div>
        <span class="text-sm font-bold text-[var(--color-text)]">ModuForge</span>
      </div>
      <LocaleSwitcher />
    </header>

    <!-- Page Content -->
    <div class="flex-1 overflow-y-auto">
      {#if current === 'projects'}
        <div class="page-enter">
          <!-- projects page placeholder - keep existing -->
          <div class="p-6 max-w-7xl mx-auto">
            <div class="flex items-center gap-3 mb-6">
              <div>
                <h2 class="text-2xl font-bold text-[var(--color-text)]">{$t('nav.projects')}</h2>
                <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">{$t('project.select_or_create')}</p>
              </div>
            </div>
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              <button
                class="border-2 border-dashed border-[var(--color-border)] rounded-2xl p-8 text-center hover:border-primary-400 hover:bg-primary-50/50 transition-all duration-200 cursor-pointer group"
                onclick={() => {/* TODO: create project dialog */}}
              >
                <div class="w-12 h-12 rounded-2xl bg-primary-100 flex items-center justify-center mx-auto mb-3 group-hover:scale-110 transition-transform">
                  <span class="material-symbols-outlined text-primary-600 text-2xl">add</span>
                </div>
                <p class="font-semibold text-[var(--color-text)]">{$t('project.new')}</p>
                <p class="text-xs text-[var(--color-text-muted)] mt-1">Create a new Magisk module</p>
              </button>
            </div>
          </div>
        </div>
      {:else if current === 'editor'}
        <div class="page-enter p-6">
          <h2 class="text-2xl font-bold text-[var(--color-text)]">{$t('nav.editor')}</h2>
          <p class="text-[var(--color-text-secondary)] mt-1">{$t('project.placeholder')}{projectId}</p>
        </div>
      {:else if current === 'builds'}
        <div class="page-enter p-6">
          <h2 class="text-2xl font-bold text-[var(--color-text)]">{$t('nav.builds')}</h2>
          <p class="text-[var(--color-text-secondary)] mt-1">{$t('project.placeholder')}{projectId}</p>
        </div>
      {:else if current === 'settings'}
        <div class="page-enter p-6">
          <h2 class="text-2xl font-bold text-[var(--color-text)]">{$t('nav.settings')}</h2>
          <p class="text-[var(--color-text-secondary)] mt-1">{$t('nav.settings')}</p>
        </div>
      {:else if current === 'market'}
        <div class="page-enter"><MarketPage /></div>
      {:else if current === 'market-publish'}
        <div class="page-enter"><PublishPage /></div>
      {:else if current === 'dashboard'}
        <div class="page-enter"><DashboardPage /></div>
      {/if}
    </div>
  </main>

  <!-- Mobile Bottom Nav -->
  <nav class="md:hidden fixed bottom-0 left-0 right-0 h-16 glass border-t border-[var(--color-border)] flex items-center justify-around px-2 safe-area-bottom z-50">
    {#each bottomNavItems as item}
      <button
        class="flex flex-col items-center gap-0.5 py-1.5 px-3 rounded-xl transition-all duration-150 min-w-[60px]
          {current === item.id ? 'text-primary-600' : 'text-[var(--color-text-muted)]'}"
        onclick={() => navigate(item.id as Route, item.id === 'editor' ? projectId : undefined)}
      >
        <span class="material-symbols-outlined text-[22px] {current === item.id ? 'text-primary-600' : ''}">
          {current === item.id ? item.icon + '_filled' : item.icon}
        </span>
        <span class="text-[10px] font-medium leading-tight">{item.label}</span>
      </button>
    {/each}
  </nav>
</div>
{/if}

<style>
  .safe-area-bottom {
    padding-bottom: env(safe-area-inset-bottom, 0px);
  }
</style>

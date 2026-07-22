<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n';
  import LocaleSwitcher from '$lib/components/LocaleSwitcher.svelte';
  import MarketPage from './routes/market/+page.svelte';
  import PublishPage from './routes/market/publish/+page.svelte';
  import AuthPage from './routes/auth/+page.svelte';
  import DashboardPage from './routes/dashboard/+page.svelte';

  type Route = 'auth' | 'projects' | 'editor' | 'builds' | 'settings' | 'market' | 'market-publish' | 'dashboard';

  let current: Route = $state('auth');
  let projectId: string | $state = $state('');
  let token = $state<string | null>(null);

  function navigate(route: Route, id?: string) {
    current = route;
    if (id) projectId = id;
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
      else if (path.startsWith('/projects/')) current = 'editor';
      else if (path === '/projects') current = 'projects';
    }
    handlePopState();
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  });
</script>

{#if !token}
  <AuthPage onAuth={handleAuth} />
{:else}
<div class="flex h-screen">
  <!-- Sidebar -->
  <aside class="w-64 bg-surface-container-high flex flex-col">
    <header class="p-4 border-b border-outline-variant">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-xl font-bold text-on-surface">ModuForge</h1>
          <p class="text-sm text-on-surface-variant">{$t('nav.home')}</p>
        </div>
        <LocaleSwitcher />
      </div>
    </header>

    <nav class="flex-1 p-2">
      <button
        class="w-full text-left px-3 py-2 rounded-lg mb-1 transition-colors"
        class:bg-primary-container={current === 'projects'}
        class:text-on-primary-container={current === 'projects'}
        onclick={() => navigate('projects')}
      >
        <span class="material-symbols-outlined mr-2 align-middle">folder</span>
        {$t('nav.projects')}
      </button>

      {#if projectId}
        <button
          class="w-full text-left px-3 py-2 rounded-lg mb-1 transition-colors"
          class:bg-primary-container={current === 'editor'}
          class:text-on-primary-container={current === 'editor'}
          onclick={() => navigate('editor')}
        >
          <span class="material-symbols-outlined mr-2 align-middle">code</span>
          {$t('nav.editor')}
        </button>

        <button
          class="w-full text-left px-3 py-2 rounded-lg mb-1 transition-colors"
          class:bg-primary-container={current === 'builds'}
          class:text-on-primary-container={current === 'builds'}
          onclick={() => navigate('builds')}
        >
          <span class="material-symbols-outlined mr-2 align-middle">build</span>
          {$t('nav.builds')}
        </button>
      {/if}

      <button
        class="w-full text-left px-3 py-2 rounded-lg mb-1 transition-colors"
        class:bg-primary-container={current === 'settings'}
        class:text-on-primary-container={current === 'settings'}
        onclick={() => navigate('settings')}
      >
        <span class="material-symbols-outlined mr-2 align-middle">settings</span>
        {$t('nav.settings')}
      </button>

      <button
        class="w-full text-left px-3 py-2 rounded-lg mb-1 transition-colors"
        class:bg-primary-container={current === 'market'}
        class:text-on-primary-container={current === 'market'}
        onclick={() => navigate('market')}
      >
        <span class="material-symbols-outlined mr-2 align-middle">storefront</span>
        {$t('nav.market')}
      </button>

      <button
        class="w-full text-left px-3 py-2 rounded-lg mb-1 transition-colors"
        class:bg-primary-container={current === 'dashboard'}
        class:text-on-primary-container={current === 'dashboard'}
        onclick={() => navigate('dashboard')}
      >
        <span class="material-symbols-outlined mr-2 align-middle">monitoring</span>
        {$t('nav.dashboard')}
      </button>
    </nav>

    <div class="p-3 border-t border-outline-variant">
      <button
        class="w-full text-left px-3 py-2 rounded-lg text-on-surface-variant hover:bg-error-container hover:text-on-error-container transition-colors"
        onclick={logout}
      >
        <span class="material-symbols-outlined mr-2 align-middle">logout</span>
        {$t('nav.logout')}
      </button>
    </div>
  </aside>

  <!-- Main -->
  <main class="flex-1 overflow-auto bg-surface">
    {#if current === 'projects'}
      <div class="p-6">
        <h2 class="text-2xl font-semibold mb-4">{$t('nav.projects')}</h2>
        <p class="text-on-surface-variant">{$t('project.select_or_create')}</p>
        <div class="mt-4 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <button
            class="border-2 border-dashed border-outline rounded-xl p-6 text-center hover:border-primary transition-colors cursor-pointer"
            onclick={() => {/* TODO: create project dialog */}}
          >
            <span class="material-symbols-outlined text-4xl text-primary">add</span>
            <p class="mt-2 text-on-surface">{$t('project.new')}</p>
          </button>
        </div>
      </div>
    {:else if current === 'editor'}
      <div class="p-6">
        <h2 class="text-2xl font-semibold mb-4">{$t('nav.editor')}</h2>
        <p class="text-on-surface-variant">{$t('project.placeholder')}{projectId}</p>
      </div>
    {:else if current === 'builds'}
      <div class="p-6">
        <h2 class="text-2xl font-semibold mb-4">{$t('nav.builds')}</h2>
        <p class="text-on-surface-variant">{$t('project.placeholder')}{projectId}</p>
      </div>
    {:else if current === 'settings'}
      <div class="p-6">
        <h2 class="text-2xl font-semibold mb-4">{$t('nav.settings')}</h2>
        <p class="text-on-surface-variant">{$t('nav.settings')}</p>
      </div>
    {:else if current === 'market'}
      <MarketPage />
    {:else if current === 'market-publish'}
      <PublishPage />
    {:else if current === 'dashboard'}
      <DashboardPage />
    {/if}
  </main>
</div>
{/if}

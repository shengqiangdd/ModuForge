<script lang="ts">
  import NotificationBell from '$lib/components/NotificationBell.svelte';
  import LocaleSwitcher from '$lib/components/LocaleSwitcher.svelte';
  import { t } from '$lib/i18n';

  let {
    current = '',
    projectId = '',
    sidebarCollapsed = false,
    themeMode = 'light',
    navItems = [],
    onNavigate,
    onToggleCollapse,
    onToggleTheme,
    onShowShortcuts,
    onConfirmLogout
  }: {
    current?: string;
    projectId?: string;
    sidebarCollapsed?: boolean;
    themeMode?: string;
    navItems?: Array<{ id: string; icon: string; label: string }>;
    onNavigate?: (route: string, id?: string) => void;
    onToggleCollapse?: () => void;
    onToggleTheme?: () => void;
    onShowShortcuts?: () => void;
    onConfirmLogout?: () => void;
  } = $props();
</script>

<aside
  class="sidebar"
  class:collapsed={sidebarCollapsed}
>
  <!-- Logo -->
  <div class="logo-section" role="button" tabindex="0" onclick={() => onNavigate?.('projects')} onkeydown={(e) => { if (e.key === 'Enter') onNavigate?.('projects'); }}>
    <div class="logo-icon">
      <span class="material-symbols-outlined">extension</span>
    </div>
    {#if !sidebarCollapsed}
      <div class="logo-text">
        <h1>ModuForge</h1>
        <p>Module Builder</p>
      </div>
    {/if}
    <button class="collapse-btn" onclick={onToggleCollapse}>
      <span class="material-symbols-outlined">{sidebarCollapsed ? 'chevron_right' : 'chevron_left'}</span>
    </button>
  </div>

  <!-- Nav Items -->
  <nav class="nav-items">
    {#each navItems as item}
      {@const isActive = current === item.id}
      <button
        class="nav-item"
        class:active={isActive}
        onclick={() => onNavigate?.(item.id, item.id === 'editor' || item.id === 'builds' ? projectId : undefined)}
        title={sidebarCollapsed ? item.label : undefined}
      >
        {#if isActive}
          <div class="active-indicator"></div>
        {/if}
        <span class="material-symbols-outlined nav-icon">{item.icon}</span>
        {#if !sidebarCollapsed}
          <span class="nav-label">{item.label}</span>
        {/if}
      </button>
    {/each}
  </nav>

  <!-- Bottom section -->
  <div class="bottom-section">
    <button class="bottom-btn" onclick={onToggleTheme}>
      <span class="material-symbols-outlined">{themeMode === 'dark' ? 'dark_mode' : themeMode === 'light' ? 'light_mode' : 'brightness_auto'}</span>
      {#if !sidebarCollapsed}
        <span>{themeMode === 'dark' ? '浅色模式' : themeMode === 'light' ? '深色模式' : '跟随系统'}</span>
      {/if}
    </button>

    <div class="notification-wrapper">
      <NotificationBell />
    </div>

    <button class="bottom-btn" onclick={onShowShortcuts}>
      <span class="material-symbols-outlined">keyboard</span>
      {#if !sidebarCollapsed}
        <span>快捷键</span>
      {/if}
    </button>

    <LocaleSwitcher compact={sidebarCollapsed} />

    <button class="bottom-btn logout" onclick={onConfirmLogout}>
      <span class="material-symbols-outlined logout-icon">logout</span>
      {#if !sidebarCollapsed}
        <span class="logout-text">{$t('nav.logout')}</span>
      {/if}
    </button>
  </div>
</aside>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    width: 256px;
    height: 100%;
    border-right: 1px solid var(--color-border);
    background: color-mix(in srgb, var(--color-bg-elevated) 92%, color-mix(in srgb, var(--color-primary) 6%, transparent));
    transition: width 0.3s ease-in-out;
  }

  .sidebar.collapsed {
    width: 72px;
  }

  .logo-section {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0 1.25rem;
    height: 64px;
    border-bottom: 1px solid var(--color-border);
    cursor: pointer;
  }

  .logo-icon {
    width: 32px;
    height: 32px;
    border-radius: 0.75rem;
    background: var(--gradient-brand);
    box-shadow: var(--shadow-glow);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .logo-icon :global(.material-symbols-outlined) {
    color: white;
    font-size: 1.25rem;
  }

  .logo-text {
    overflow: hidden;
  }

  .logo-text h1 {
    margin: 0;
    font-size: 1rem;
    font-weight: 700;
    letter-spacing: -0.025em;
    color: var(--color-text);
  }

  .logo-text p {
    margin: 0;
    font-size: 0.6875rem;
    line-height: 1.2;
    color: var(--color-text-muted);
  }

  .collapse-btn {
    margin-left: auto;
    padding: 0.375rem;
    border-radius: 0.5rem;
    border: none;
    background: transparent;
    color: var(--color-text-muted);
    cursor: pointer;
    transition: color 0.2s;
  }

  .collapse-btn:hover {
    color: var(--color-text);
  }

  .nav-items {
    flex: 1;
    padding: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    overflow-y: auto;
  }

  .nav-item {
    position: relative;
    width: 100%;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.625rem 0.75rem;
    border-radius: 0.75rem;
    border: none;
    background: transparent;
    font-size: 0.875rem;
    font-weight: 500;
    min-height: 44px;
    cursor: pointer;
    transition: all 0.2s;
    color: var(--color-text-secondary);
  }

  .nav-item:hover {
    background: color-mix(in srgb, var(--color-primary) 8%, transparent);
  }

  .nav-item.active {
    color: var(--color-primary);
    background: color-mix(in srgb, var(--color-primary) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-primary) 20%, transparent);
  }

  .active-indicator {
    position: absolute;
    left: 0;
    top: 0.5rem;
    bottom: 0.5rem;
    width: 3px;
    border-radius: 0 9999px 9999px 0;
    background: var(--gradient-brand);
  }

  .nav-icon {
    font-size: 1.25rem;
    flex-shrink: 0;
    position: relative;
    z-index: 10;
  }

  .nav-item.active .nav-icon {
    color: var(--color-primary);
  }

  .nav-label {
    position: relative;
    z-index: 10;
  }

  .bottom-section {
    padding: 0.75rem;
    border-top: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .bottom-btn {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.625rem 0.75rem;
    border-radius: 0.75rem;
    border: none;
    background: transparent;
    font-size: 0.875rem;
    font-weight: 500;
    min-height: 44px;
    cursor: pointer;
    color: var(--color-text-secondary);
    transition: background 0.15s;
  }

  .bottom-btn:hover {
    background: var(--color-surface);
  }

  .bottom-btn :global(.material-symbols-outlined) {
    font-size: 1.25rem;
    color: var(--color-text-muted);
  }

  .notification-wrapper {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.25rem 0.75rem;
  }

  .logout {
    margin-top: 0.25rem;
  }

  .logout:hover .logout-icon {
    color: var(--color-error) !important;
  }

  .logout:hover .logout-text {
    color: var(--color-error);
  }

  @media (max-width: 768px) {
    .sidebar {
      display: none;
    }
  }
</style>

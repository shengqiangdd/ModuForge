<script lang="ts">
  import { t } from '$lib/i18n';
  import NotificationBell from '$lib/components/NotificationBell.svelte';
  import LocaleSwitcher from '$lib/components/LocaleSwitcher.svelte';

  type Route = 'auth' | 'projects' | 'editor' | 'builds' | 'tests' | 'settings' | 'market' | 'market-publish' | 'dashboard' | 'ai' | 'mcp' | 'devices' | 'crash' | 'glossary';

  interface Props {
    current: Route;
    projectId: string;
    themeMode: string;
    collapsed: boolean;
    onNavigate: (route: Route, id?: string) => void;
    onToggleTheme: () => void;
    onToggleCollapse: () => void;
    onShowShortcuts: () => void;
    onConfirmLogout: () => void;
  }

  let {
    current,
    projectId,
    themeMode,
    collapsed,
    onNavigate,
    onToggleTheme,
    onToggleCollapse,
    onShowShortcuts,
    onConfirmLogout,
  }: Props = $props();

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
</script>

<style>
  .nav-item {
    position: relative;
    overflow: hidden;
  }
</style>

<!-- ═══ Desktop Sidebar ═══ -->
<aside
  class="hidden md:flex flex-col border-r transition-all duration-300 ease-in-out {collapsed ? 'w-[72px]' : 'w-64'}"
  style="background: color-mix(in srgb, var(--color-bg-elevated) 92%, color-mix(in srgb, var(--color-primary) 6%, transparent)); border-color: var(--color-border)"
>
  <!-- Logo -->
  <div class="flex items-center gap-3 px-5 h-16 border-b cursor-pointer" style="border-color: var(--color-border)" role="button" tabindex="0" onclick={() => onNavigate('projects')} onkeydown={(e) => { if (e.key === 'Enter') onNavigate('projects'); }}>
    <div class="w-8 h-8 rounded-xl flex items-center justify-center flex-shrink-0" style="background: var(--gradient-brand); box-shadow: var(--shadow-glow)">
      <span class="material-symbols-outlined text-white text-lg">extension</span>
    </div>
    {#if !collapsed}
      <div class="overflow-hidden">
        <h1 class="text-base font-bold tracking-tight" style="color: var(--color-text)">ModuForge</h1>
        <p class="text-[11px] leading-tight" style="color: var(--color-text-muted)">Module Builder</p>
      </div>
    {/if}
    <button
      class="ml-auto p-1.5 rounded-lg transition-colors"
      style="color: var(--color-text-muted)"
      onclick={onToggleCollapse}
      aria-label={collapsed ? '展开侧边栏' : '折叠侧边栏'}
    >
      <span class="material-symbols-outlined text-[18px]">{collapsed ? 'chevron_right' : 'chevron_left'}</span>
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
        onclick={() => onNavigate(item.id as Route, item.id === 'editor' || item.id === 'builds' ? projectId : undefined)}
        title={collapsed ? item.label : undefined}
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
        {#if !collapsed}
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
      onclick={onToggleTheme}
      aria-label={themeMode === 'dark' ? '浅色模式' : themeMode === 'light' ? '深色模式' : '跟随系统'}
    >
      <span class="material-symbols-outlined text-[20px]" style="color: var(--color-text-muted)">{themeMode === 'dark' ? 'dark_mode' : themeMode === 'light' ? 'light_mode' : 'brightness_auto'}</span>
      {#if !collapsed}
        <span>{themeMode === 'dark' ? '浅色模式' : themeMode === 'light' ? '深色模式' : '跟随系统'}</span>
      {/if}
    </button>
    <div class="flex items-center justify-center {collapsed ? '' : 'px-3'} py-1">
      <NotificationBell />
    </div>
    <button
      class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-150 text-[14px] font-medium min-h-[44px] hover:bg-[var(--color-surface)]"
      style="color: var(--color-text-secondary)"
      onclick={onShowShortcuts}
      aria-label="快捷键"
    >
      <span class="material-symbols-outlined text-[20px]" style="color: var(--color-text-muted)">keyboard</span>
      {#if !collapsed}
        <span>快捷键</span>
      {/if}
    </button>
    <LocaleSwitcher compact={collapsed} />
    <button
      class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-200 text-[14px] font-medium mt-1 min-h-[44px] group/logout hover:bg-[var(--color-surface)]"
      style="color: var(--color-text-secondary)"
      onclick={onConfirmLogout}
      aria-label="{$t('nav.logout')}"
    >
      <span class="material-symbols-outlined text-[20px] transition-colors group-hover/logout:text-[var(--color-error)]" style="color: var(--color-text-muted)">logout</span>
      {#if !collapsed}
        <span class="transition-colors group-hover/logout:text-[var(--color-error)]">{$t('nav.logout')}</span>
      {/if}
    </button>
  </div>
</aside>

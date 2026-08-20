<script lang="ts">
  import NotificationBell from '$lib/components/NotificationBell.svelte';
  import LocaleSwitcher from '$lib/components/LocaleSwitcher.svelte';

  type Route = 'auth' | 'projects' | 'editor' | 'builds' | 'tests' | 'settings' | 'market' | 'market-publish' | 'dashboard' | 'ai' | 'mcp' | 'devices' | 'crash' | 'glossary';

  interface Props {
    themeMode: string;
    onNavigate: (route: Route) => void;
    onToggleTheme: () => void;
    onConfirmLogout: () => void;
  }

  let {
    themeMode,
    onNavigate,
    onToggleTheme,
    onConfirmLogout,
  }: Props = $props();
</script>

<!-- Mobile Header (hidden on AI page for fullscreen) -->
<header class="md:hidden flex items-center justify-between px-4 h-14 border-b flex-shrink-0 glass" style="border-color: var(--color-border)">
  <div class="flex items-center gap-2.5" role="button" tabindex="0" onclick={() => onNavigate('projects')} onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onNavigate('projects'); } }}>
    <div class="w-7 h-7 rounded-lg flex items-center justify-center" style="background: var(--gradient-brand)">
      <span class="material-symbols-outlined text-white text-sm">extension</span>
    </div>
    <span class="text-sm font-bold" style="color: var(--color-text)">ModuForge</span>
  </div>
  <div class="flex items-center gap-1">
    <NotificationBell />
    <button class="p-2 rounded-lg min-w-[44px] min-h-[44px] flex items-center justify-center" style="color: var(--color-text-muted)" onclick={onToggleTheme} aria-label="切换主题">
      <span class="material-symbols-outlined text-[20px]">{themeMode === 'dark' ? 'dark_mode' : themeMode === 'light' ? 'light_mode' : 'brightness_auto'}</span>
    </button>
    <LocaleSwitcher />
    <button class="p-2 rounded-lg min-w-[44px] min-h-[44px] flex items-center justify-center" style="color: var(--color-text-muted)" onclick={onConfirmLogout} title="退出登录">
      <span class="material-symbols-outlined text-[20px]">logout</span>
    </button>
  </div>
</header>

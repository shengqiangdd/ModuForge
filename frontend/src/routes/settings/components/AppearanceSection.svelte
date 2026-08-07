<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';
  import { getTheme, setTheme } from '$lib/stores/theme';

  let { themeMode, onThemeChange }: { themeMode: string; onThemeChange?: (mode: 'light' | 'dark' | 'system') => void } = $props();

  function changeTheme(mode: 'light' | 'dark' | 'system') {
    setTheme(mode);
    onThemeChange?.(mode);
    toast(`已切换到${mode === 'dark' ? '深色' : mode === 'light' ? '浅色' : '跟随系统'}模式`, 'info');
  }
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: linear-gradient(135deg, rgba(249,115,22,0.15), rgba(168,85,247,0.15))">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-warning)">palette</span>
    </div>
    <div>
      <h2 class="text-base font-semibold text-[var(--color-text)]">外观</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">自定义界面主题和显示效果</p>
    </div>
  </div>
  <div class="flex items-center justify-between">
    <div>
      <p class="text-sm font-medium text-[var(--color-text)]">主题模式</p>
      <p class="text-xs" style="color: var(--color-text-muted)">当前: {themeMode === 'dark' ? '深色模式' : themeMode === 'light' ? '浅色模式' : '跟随系统'}</p>
    </div>
    <div class="flex gap-1 p-1 rounded-xl" style="background: var(--color-surface)">
      {#each [
        { mode: 'light' as const, icon: 'light_mode', label: '浅色' },
        { mode: 'dark' as const, icon: 'dark_mode', label: '深色' },
        { mode: 'system' as const, icon: 'brightness_auto', label: '系统' },
      ] as opt}
        <button
          class="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
          style={themeMode === opt.mode ? 'background: var(--color-primary); color: white' : 'color: var(--color-text-secondary)'}
          onclick={() => changeTheme(opt.mode)}
          aria-label="切换到{opt.label}主题"
        >
          <span class="material-symbols-outlined text-[14px]">{opt.icon}</span>
          <span class="hidden sm:inline">{opt.label}</span>
        </button>
      {/each}
    </div>
  </div>
</section>

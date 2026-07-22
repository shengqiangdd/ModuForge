<script lang="ts">
  import { currentLocale } from '$lib/i18n';

  let { compact = false }: { compact?: boolean } = $props();
  let open = $state(false);

  const languages = [
    { code: 'en', label: 'EN', flag: '🇺🇸', name: 'English' },
    { code: 'zh', label: '中文', flag: '🇨🇳', name: '中文' },
    { code: 'ja', label: '日本語', flag: '🇯🇵', name: '日本語' },
    { code: 'ko', label: '한국어', flag: '🇰🇷', name: '한국어' },
  ];

  let currentLang = $derived(languages.find(l => l.code === $currentLocale) || languages[0]);

  function select(code: string) {
    $currentLocale = code;
    open = false;
  }

  function handleClickOutside(e: Event) {
    const target = e.target as HTMLElement;
    if (!target.closest('.locale-switcher')) open = false;
  }
</script>

<svelte:window on:click={handleClickOutside} />

<div class="locale-switcher relative">
  <button
    class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-xl text-sm font-medium text-[var(--color-text-secondary)] hover:bg-neutral-100 transition-all duration-150"
    onclick={(e) => { e.stopPropagation(); open = !open; }}
  >
    <span class="text-base">{currentLang.flag}</span>
    {#if !compact}
      <span class="text-xs">{currentLang.label}</span>
    {/if}
    <span class="material-symbols-outlined text-[14px] transition-transform duration-200 {open ? 'rotate-180' : ''}">expand_more</span>
  </button>

  {#if open}
    <div
      class="absolute top-full mt-2 right-0 w-44 bg-[var(--color-bg-elevated)] rounded-xl shadow-elevated-lg border border-[var(--color-border)] py-1.5 z-50 animate-[scaleIn_0.15s_ease-out]"
      onclick={(e) => e.stopPropagation()}
    >
      {#each languages as lang}
        <button
          class="w-full flex items-center gap-3 px-3 py-2 text-sm transition-colors duration-100
            {lang.code === $currentLocale ? 'bg-primary-50 text-primary-700 font-medium' : 'text-[var(--color-text-secondary)] hover:bg-neutral-50 hover:text-[var(--color-text)]'}"
          onclick={() => select(lang.code)}
        >
          <span class="text-base">{lang.flag}</span>
          <span>{lang.name}</span>
          {#if lang.code === $currentLocale}
            <span class="material-symbols-outlined text-[14px] ml-auto text-primary-500">check</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

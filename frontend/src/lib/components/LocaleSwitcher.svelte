<script lang="ts">
  import { currentLocale, locales, type Locale } from '$lib/i18n';

  let open = $state(false);

  function select(code: Locale) {
    $currentLocale = code;
    open = false;
  }

  let current = $derived(locales.find((l) => l.code === $currentLocale) || locales[0]);
</script>

<div class="relative">
  <button
    onclick={() => (open = !open)}
    class="flex items-center gap-1.5 px-2 py-1 rounded-lg hover:bg-surface-container-highest text-sm transition-colors"
  >
    <span>{current.flag}</span>
    <span class="hidden sm:inline">{current.code.toUpperCase()}</span>
    <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
    </svg>
  </button>

  {#if open}
    <div
      class="absolute right-0 top-full mt-1 bg-surface rounded-lg shadow-xl z-50 min-w-[140px] border border-outline-variant"
    >
      {#each locales as locale}
        <button
          onclick={() => select(locale.code)}
          class="w-full flex items-center gap-2 px-3 py-2 hover:bg-surface-container text-sm transition-colors first:rounded-t-lg last:rounded-b-lg"
          class:bg-primary-container={locale.code === $currentLocale}
          class:text-on-primary-container={locale.code === $currentLocale}
        >
          <span>{locale.flag}</span>
          <span>{locale.name}</span>
          {#if locale.code === $currentLocale}
            <span class="ml-auto text-primary">&#10003;</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

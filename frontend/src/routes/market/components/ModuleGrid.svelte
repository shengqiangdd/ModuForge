<script lang="ts">
  interface MarketModule {
    id: string; title: string; slug: string; description: string; category: string;
    tags: string; version: string; version_code: number; author: string;
    license: string; stars: number; installs: number; updated_at: string; created_at: string;
    screenshots?: { url: string }[];
    cover_image?: string;
    dependencies?: { id: string; min_version?: string; optional?: boolean }[];
  }

  let {
    modules = [],
    loading = false,
    favoritedModules = new Set<string>(),
    selectedSlugs = new Set<string>(),
    compareIds = new Set<string>(),
    categoryStyles = {},
    onSelect,
    onFavorite,
    onCompare,
    onOpenDetail,
  }: {
    modules?: MarketModule[];
    loading?: boolean;
    favoritedModules?: Set<string>;
    selectedSlugs?: Set<string>;
    compareIds?: Set<string>;
    categoryStyles?: Record<string, string>;
    onSelect?: (slug: string) => void;
    onFavorite?: (mod: MarketModule) => void;
    onCompare?: (slug: string) => void;
    onOpenDetail?: (mod: MarketModule) => void;
  } = $props();

  function fmt(n: number) { return n >= 1000 ? (n / 1000).toFixed(1) + 'k' : String(n); }
</script>

{#if loading}
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
    {#each Array(8) as _}
      <div class="rounded-2xl border border-[var(--color-border)] p-5">
        <div class="skeleton h-4 w-24 mb-3"></div>
        <div class="skeleton h-5 w-full mb-2"></div>
        <div class="skeleton h-3 w-3/4 mb-4"></div>
        <div class="skeleton h-3 w-full mb-1"></div>
        <div class="skeleton h-3 w-2/3"></div>
      </div>
    {/each}
  </div>
{:else if modules.length === 0}
  <div class="text-center py-16">
    <span class="material-symbols-outlined text-5xl text-neutral-300 mb-3 block">inventory_2</span>
    <p class="text-[var(--color-text-secondary)]">没有找到匹配的模块</p>
  </div>
{:else}
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4" style="animation: fadeIn 0.3s ease-out">
    {#each modules as mod, i (mod.id)}
      <div
        role="button"
        tabindex="0"
        class="text-left p-5 group cursor-pointer relative overflow-hidden rounded-2xl border border-[var(--color-border)] transition-all duration-300 hover:border-[var(--color-primary)] hover:shadow-[0_8px_32px_color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] hover:-translate-y-1"
        style="background: var(--color-bg-elevated); animation-delay: {i * 50}ms"
        onclick={() => onOpenDetail?.(mod)}
        onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onOpenDetail?.(mod); } }}
      >
        <!-- Favorite button -->
        <button class="absolute top-5 left-3 z-10" aria-label="收藏" onclick={(e) => { e.stopPropagation(); onFavorite?.(mod); }}>
          <span class="material-symbols-outlined text-lg p-1 rounded-full cursor-pointer transition-colors" style="color: {favoritedModules.has(mod.id) ? '#ef4444' : 'var(--color-text-muted)'}; background: {favoritedModules.has(mod.id) ? '#ef444420' : 'transparent'}">{favoritedModules.has(mod.id) ? 'favorite' : 'favorite_border'}</span>
        </button>
        <!-- Batch select checkbox -->
        <button class="absolute top-5 right-16 z-10" aria-label="选择" onclick={(e) => { e.stopPropagation(); onSelect?.(mod.slug); }}>
          <div class="w-4 h-4 rounded border-2 flex items-center justify-center transition-colors" style={selectedSlugs.has(mod.slug) ? 'background: var(--color-primary); border-color: var(--color-primary)' : 'border-color: var(--color-border); background: transparent'}>
            {#if selectedSlugs.has(mod.slug)}
              <span class="material-symbols-outlined text-[10px] text-white">check</span>
            {/if}
          </div>
        </button>
        <!-- Compare checkbox -->
        <button class="absolute top-5 right-3 z-10" aria-label="对比" onclick={(e) => { e.stopPropagation(); onCompare?.(mod.slug); }}>
          <div
            class="w-5 h-5 rounded border-2 flex items-center justify-center transition-colors"
            style={compareIds.has(mod.slug) ? 'background: var(--color-primary); border-color: var(--color-primary)' : 'border-color: var(--color-border); background: transparent'}
          >
            {#if compareIds.has(mod.slug)}
              <span class="material-symbols-outlined text-[14px] text-white">check</span>
            {/if}
          </div>
        </button>
        <!-- Hover gradient overlay -->
        <div class="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-300" style="background: linear-gradient(135deg, color-mix(in srgb, var(--color-primary) 5%, transparent) 0%, color-mix(in srgb, var(--color-info) 3%, transparent) 100%)"></div>
        
        <div class="relative z-10 pl-7">
          <div class="flex items-center gap-3 mb-3">
            <span class="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
              <span class="material-symbols-outlined text-[14px] text-amber-500">star</span>
              {mod.stars}
            </span>
            <span class="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
              <span class="material-symbols-outlined text-[14px]">download</span>
              {fmt(mod.installs)}
            </span>
            <span class="ml-auto text-[10px] px-1.5 py-0.5 rounded-md" style={categoryStyles[mod.category] || 'background: var(--color-surface); color: var(--color-text-muted)'}>
              {mod.category}
            </span>
          </div>
          <h3 class="font-semibold text-[var(--color-text)] mb-1 line-clamp-1 group-hover:text-[var(--color-primary)] transition-colors duration-200">{mod.title}</h3>
          <div class="flex items-center gap-2 mb-2">
            <span class="text-xs px-1.5 py-0.5 rounded-md" style="background: var(--color-primary-light); color: var(--color-primary)">{mod.version}</span>
            <span class="text-xs" style="color: var(--color-text-muted)">{mod.author}</span>
          </div>
          <p class="text-sm text-[var(--color-text-secondary)] line-clamp-2 leading-relaxed">{mod.description}</p>
        </div>
      </div>
    {/each}
  </div>
{/if}

<style>
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(8px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>

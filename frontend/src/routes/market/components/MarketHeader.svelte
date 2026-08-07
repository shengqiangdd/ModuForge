<script lang="ts">
  import type { Writable } from 'svelte/store';

  let {
    searchQuery = $bindable(''),
    selectedCategory = $bindable(''),
    sortBy = $bindable('stars'),
    total = 0,
    categories,
    allTags,
    selectedTag = $bindable(null),
    compareIds = $bindable(new Set<string>()),
    selectedSlugs = $bindable(new Set<string>()),
    batchProcessing = false,
    compareLoading = false,
    onSearch,
    onCategoryChange,
    onSortChange,
    onTagChange,
    onCompare,
    onRunBatch,
    onClearSlugs,
    onClearCompare,
  }: {
    searchQuery?: string;
    selectedCategory?: string;
    sortBy?: string;
    total?: number;
    categories: { value: string; label: string; icon: string }[];
    allTags: { id: number; name: string; color: string; usage_count: number }[];
    selectedTag?: number | null;
    compareIds?: Set<string>;
    selectedSlugs?: Set<string>;
    batchProcessing?: boolean;
    compareLoading?: boolean;
    onSearch?: () => void;
    onCategoryChange?: (cat: string) => void;
    onSortChange?: (sort: string) => void;
    onTagChange?: (tag: number | null) => void;
    onCompare?: () => void;
    onRunBatch?: (action: 'install' | 'uninstall' | 'update') => void;
    onClearSlugs?: () => void;
    onClearCompare?: (slug: string) => void;
  } = $props();

  let debounceTimer: ReturnType<typeof setTimeout> | null = $state(null);

  function onSearchInput() {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => onSearch?.(), 300);
  }
</script>

<!-- Search -->
<div class="relative mb-5">
  <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-neutral-400 text-[20px] z-10">search</span>
  <div class="absolute left-[38px] top-2.5 bottom-2.5 w-px pointer-events-none z-10" style="background: var(--color-border)"></div>
  <input
    type="text"
    placeholder="搜索模块名称、描述、标签..."
    class="input-field"
    style="padding-left: 48px;"
    bind:value={searchQuery}
    oninput={onSearchInput}
    onkeydown={(e) => { if (e.key === 'Enter') { if (debounceTimer) clearTimeout(debounceTimer); onSearch?.(); } }}
  />
</div>

<!-- Categories -->
<div class="flex gap-2 flex-wrap mb-4">
  {#each categories as cat}
    <button
      class="flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium min-h-[44px] transition-all active:scale-[0.96]"
      style={selectedCategory === cat.value
        ? 'background: var(--gradient-brand); color: #fff; box-shadow: var(--shadow-glow)'
        : 'background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)'}
      onclick={() => onCategoryChange?.(cat.value)}
    >
      <span class="material-symbols-outlined text-[16px]">{cat.icon}</span>
      {cat.label}
    </button>
  {/each}
</div>

<!-- Tags Filter -->
{#if allTags.length > 0}
  <div class="flex gap-2 flex-wrap mb-4">
    <button
      class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
      style={!selectedTag ? 'background: var(--color-primary-light); color: var(--color-primary)' : 'background: var(--color-surface); color: var(--color-text-muted); border: 1px solid var(--color-border)'}
      onclick={() => onTagChange?.(null)}
    >全部</button>
    {#each allTags as tag}
      <button
        class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
        style={selectedTag === tag.id ? `background: ${tag.color}20; color: ${tag.color}` : 'background: var(--color-surface); color: var(--color-text-muted); border: 1px solid var(--color-border)'}
        onclick={() => onTagChange?.(tag.id)}
      >{tag.name}</button>
    {/each}
  </div>
{/if}

<!-- Sort & Count -->
<div class="flex items-center gap-3 mb-6 text-sm" style="color: var(--color-text-secondary)">
  <span>排序</span>
  {#each [{ id: 'stars', label: '热度' }, { id: 'installs', label: '安装量' }, { id: 'newest', label: '最新' }] as s}
    <button
      class="px-3 py-1 rounded-lg transition-colors min-h-[36px]"
      style={sortBy === s.id ? 'background: var(--color-primary-light); color: var(--color-primary); font-weight: 600' : ''}
      onclick={() => onSortChange?.(s.id)}
    >
      {s.label}
    </button>
  {/each}
  <span class="ml-auto" style="color: var(--color-text-muted)">{total} 个模块</span>
</div>

<!-- Compare bar -->
{#if compareIds.size > 0}
  <div class="flex items-center gap-3 mb-4 px-4 py-3 rounded-xl" style="background: var(--color-primary-light);">
    <span class="text-sm" style="color: var(--color-primary)">已选 {compareIds.size}/2 个模块</span>
    <div class="flex gap-2 flex-1 flex-wrap">
      {#each Array.from(compareIds) as slug}
        <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-lg text-xs" style="background: var(--color-bg-elevated); color: var(--color-text)">
          {slug}
          <button class="p-0.5 hover:text-[var(--color-error)]" onclick={() => onClearCompare?.(slug)}>
            <span class="material-symbols-outlined text-[12px]">close</span>
          </button>
        </span>
      {/each}
    </div>
    {#if compareIds.size === 2}
      <button class="px-3 py-1.5 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--gradient-brand)" onclick={onCompare} disabled={compareLoading}>
        {compareLoading ? '对比中...' : '对比'}
      </button>
    {/if}
  </div>
{/if}

<!-- Batch Operations Bar -->
{#if selectedSlugs.size > 0}
  <div class="flex items-center gap-3 mb-4 px-4 py-3 rounded-xl" style="background: var(--color-primary-light);">
    <span class="text-sm font-medium" style="color: var(--color-primary)">已选 {selectedSlugs.size} 个模块</span>
    <div class="flex gap-2 ml-auto">
      <button class="px-3 py-1.5 rounded-xl text-sm font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" disabled={batchProcessing} onclick={() => onRunBatch?.('install')}>
        {batchProcessing ? '处理中...' : '安装'}
      </button>
      <button class="px-3 py-1.5 rounded-xl text-sm font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" disabled={batchProcessing} onclick={() => onRunBatch?.('uninstall')}>
        卸载
      </button>
      <button class="px-3 py-1.5 rounded-xl text-sm font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" disabled={batchProcessing} onclick={() => onRunBatch?.('update')}>
        更新
      </button>
      <button class="flex items-center gap-1 text-xs px-2 py-1.5 rounded-lg hover:bg-red-50 transition-colors" style="color: var(--color-error)" onclick={onClearSlugs}>
        <span class="material-symbols-outlined text-[14px]">close</span>
        清除
      </button>
    </div>
  </div>
{/if}

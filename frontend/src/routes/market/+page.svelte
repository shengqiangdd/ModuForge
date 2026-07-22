<script lang="ts">
  import { onMount } from 'svelte';

  interface MarketModule {
    id: string; title: string; slug: string; description: string; category: string;
    tags: string; version: string; version_code: number; author: string;
    license: string; stars: number; installs: number; updated_at: string; created_at: string;
  }
  interface Review {
    id: string; module_id: string; uid: string; username: string;
    rating: number; comment: string; created_at: string;
  }

  let modules = $state<MarketModule[]>([]);
  let total = $state(0);
  let loading = $state(true);
  let searchQuery = $state('');
  let selectedCategory = $state('');
  let sortBy = $state('stars');
  let page = $state(1);
  const perPage = 20;

  let selectedModule = $state<MarketModule | null>(null);
  let reviews = $state<Review[]>([]);
  let reviewsLoading = $state(false);
  let newReviewRating = $state(5);
  let newReviewComment = $state('');
  let submittingReview = $state(false);

  const categories = [
    { value: '', label: '全部', icon: 'apps' },
    { value: 'system', label: '系统', icon: 'phone_android' },
    { value: 'ui', label: '界面', icon: 'palette' },
    { value: 'audio', label: '音频', icon: 'headphones' },
    { value: 'display', label: '显示', icon: 'brightness_6' },
    { value: 'utility', label: '工具', icon: 'build' },
  ];

  const categoryColors: Record<string, string> = {
    system: 'bg-blue-100 text-blue-700', ui: 'bg-purple-100 text-purple-700',
    audio: 'bg-green-100 text-green-700', display: 'bg-orange-100 text-orange-700',
    utility: 'bg-neutral-100 text-neutral-600',
  };

  async function loadModules() {
    loading = true;
    try {
      const params = new URLSearchParams({ page: String(page), per_page: String(perPage), sort: sortBy });
      if (searchQuery) params.set('query', searchQuery);
      if (selectedCategory) params.set('category', selectedCategory);
      const res = await fetch(`/api/v1/market/modules?${params}`);
      if (res.ok) { const d = await res.json(); modules = d.modules || []; total = d.total || 0; }
    } catch { modules = []; }
    loading = false;
  }

  async function openDetail(mod: MarketModule) {
    selectedModule = mod;
    reviewsLoading = true;
    try {
      const res = await fetch(`/api/v1/market/module/${mod.slug}/reviews`);
      if (res.ok) { const d = await res.json(); reviews = d.reviews || []; }
    } catch { reviews = []; }
    reviewsLoading = false;
  }

  async function starModule() {
    if (!selectedModule) return;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/market/module/${selectedModule.slug}/star`, { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } });
      if (res.ok) {
        const d = await res.json();
        selectedModule = { ...selectedModule, stars: d.stars };
        const idx = modules.findIndex(m => m.id === selectedModule!.id);
        if (idx >= 0) modules[idx] = { ...modules[idx], stars: d.stars };
      }
    } catch {}
  }

  async function submitReview() {
    if (!selectedModule || !newReviewComment.trim()) return;
    submittingReview = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/market/module/${selectedModule.slug}/review`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ uid: 'anonymous', username: 'Anonymous', rating: newReviewRating, comment: newReviewComment }),
      });
      if (res.ok) {
        newReviewComment = ''; newReviewRating = 5;
        const r = await fetch(`/api/v1/market/module/${selectedModule.slug}/reviews`);
        if (r.ok) { const d = await r.json(); reviews = d.reviews || []; }
      }
    } catch {}
    submittingReview = false;
  }

  function fmt(n: number) { return n >= 1000 ? (n / 1000).toFixed(1) + 'k' : String(n); }
  function handleSearch(e: KeyboardEvent) { if (e.key === 'Enter') { page = 1; loadModules(); } }

  onMount(() => loadModules());
</script>

<div class="p-6 max-w-7xl mx-auto">
  <!-- Header -->
  <div class="flex items-center justify-between mb-8">
    <div>
      <h1 class="text-2xl font-bold text-[var(--color-text)]">ModuForge 市场</h1>
      <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">发现和分享优质 Magisk/KSU 模块</p>
    </div>
    <a href="/market/publish" class="btn-primary flex items-center gap-2 no-underline">
      <span class="material-symbols-outlined text-[18px]">publish</span>
      发布模块
    </a>
  </div>

  <!-- Search -->
  <div class="relative mb-5">
    <span class="material-symbols-outlined absolute left-4 top-1/2 -translate-y-1/2 text-neutral-400 text-[20px]">search</span>
    <input
      type="text"
      placeholder="搜索模块名称、描述、标签..."
      class="input-field pl-12 pr-4"
      bind:value={searchQuery}
      onkeydown={handleSearch}
    />
  </div>

  <!-- Categories -->
  <div class="flex gap-2 flex-wrap mb-4">
    {#each categories as cat}
      <button
        class="flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium transition-all duration-150
          {selectedCategory === cat.value
            ? 'bg-primary-600 text-white shadow-sm'
            : 'bg-[var(--color-surface)] text-[var(--color-text-secondary)] hover:bg-neutral-200'}"
        onclick={() => { selectedCategory = cat.value; page = 1; loadModules(); }}
      >
        <span class="material-symbols-outlined text-[16px]">{cat.icon}</span>
        {cat.label}
      </button>
    {/each}
  </div>

  <!-- Sort & Count -->
  <div class="flex items-center gap-3 mb-6 text-sm text-[var(--color-text-secondary)]">
    <span>排序</span>
    {#each [{ id: 'stars', label: '热度' }, { id: 'installs', label: '安装量' }, { id: 'newest', label: '最新' }] as s}
      <button
        class="px-3 py-1 rounded-lg transition-colors {sortBy === s.id ? 'bg-primary-100 text-primary-700 font-medium' : 'hover:bg-neutral-100'}"
        onclick={() => { sortBy = s.id; page = 1; loadModules(); }}
      >
        {s.label}
      </button>
    {/each}
    <span class="ml-auto text-[var(--color-text-muted)]">{total} 个模块</span>
  </div>

  <!-- Grid -->
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
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      {#each modules as mod}
        <button
          class="text-left card-hover p-5 group cursor-pointer"
          onclick={() => openDetail(mod)}
        >
          <div class="flex items-center gap-3 mb-3">
            <span class="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
              <span class="material-symbols-outlined text-[14px] text-amber-500">star</span>
              {mod.stars}
            </span>
            <span class="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
              <span class="material-symbols-outlined text-[14px]">download</span>
              {fmt(mod.installs)}
            </span>
            <span class="ml-auto badge text-[10px] {categoryColors[mod.category] || 'bg-neutral-100 text-neutral-600'}">
              {mod.category}
            </span>
          </div>
          <h3 class="font-semibold text-[var(--color-text)] mb-1 line-clamp-1 group-hover:text-primary-600 transition-colors">{mod.title}</h3>
          <p class="text-xs text-[var(--color-text-muted)] mb-2">{mod.version} · {mod.author}</p>
          <p class="text-sm text-[var(--color-text-secondary)] line-clamp-2">{mod.description}</p>
        </button>
      {/each}
    </div>
  {/if}
</div>

<!-- Detail Modal -->
{#if selectedModule}
  <div class="fixed inset-0 bg-black/40 backdrop-blur-sm flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" onclick={() => selectedModule = null}>
    <div class="bg-[var(--color-bg-elevated)] rounded-2xl max-w-2xl w-full max-h-[85vh] overflow-auto shadow-elevated-lg border border-[var(--color-border)] animate-[scaleIn_0.2s_ease-out]" onclick={(e) => e.stopPropagation()}>
      <div class="p-6 border-b border-[var(--color-border)]">
        <div class="flex items-start justify-between">
          <div>
            <h2 class="text-xl font-bold text-[var(--color-text)]">{selectedModule.title}</h2>
            <div class="flex flex-wrap items-center gap-2 mt-2 text-sm text-[var(--color-text-muted)]">
              <span>{selectedModule.version}</span>
              <span>·</span>
              <span class="badge {categoryColors[selectedModule.category] || ''}">{selectedModule.category}</span>
              <span>·</span>
              <span>{selectedModule.author}</span>
              <span>·</span>
              <span>{selectedModule.license}</span>
            </div>
          </div>
          <button class="p-2 rounded-xl hover:bg-neutral-100 transition-colors" onclick={() => selectedModule = null}>
            <span class="material-symbols-outlined text-[20px]">close</span>
          </button>
        </div>
        <div class="flex items-center gap-5 mt-4">
          <button class="flex items-center gap-1.5 text-sm font-medium hover:text-primary-600 transition-colors" onclick={starModule}>
            <span class="material-symbols-outlined text-[18px]">star</span>
            {selectedModule.stars} Stars
          </button>
          <span class="flex items-center gap-1.5 text-sm text-[var(--color-text-secondary)]">
            <span class="material-symbols-outlined text-[18px]">download</span>
            {fmt(selectedModule.installs)} 安装
          </span>
        </div>
      </div>

      <div class="p-6 border-b border-[var(--color-border)]">
        <h3 class="text-sm font-semibold text-[var(--color-text)] mb-2">描述</h3>
        <p class="text-sm text-[var(--color-text-secondary)] leading-relaxed">{selectedModule.description}</p>
        {#if selectedModule.tags}
          <div class="flex flex-wrap gap-1.5 mt-3">
            {#each selectedModule.tags.split(',') as tag}
              <span class="px-2.5 py-1 bg-[var(--color-surface)] rounded-lg text-xs text-[var(--color-text-muted)]">{tag.trim()}</span>
            {/each}
          </div>
        {/if}
      </div>

      <div class="p-6">
        <h3 class="text-sm font-semibold text-[var(--color-text)] mb-4">评论</h3>
        {#if reviewsLoading}
          <div class="flex justify-center py-6"><div class="skeleton h-4 w-32"></div></div>
        {:else if reviews.length === 0}
          <p class="text-sm text-[var(--color-text-muted)] mb-4">暂无评论</p>
        {:else}
          <div class="space-y-2 mb-4 max-h-48 overflow-auto">
            {#each reviews as rev}
              <div class="p-3 rounded-xl bg-[var(--color-surface)]">
                <div class="flex items-center gap-2 mb-1">
                  <span class="text-sm font-medium text-[var(--color-text)]">{rev.username}</span>
                  <span class="text-xs text-amber-500">{'★'.repeat(rev.rating)}{'☆'.repeat(5 - rev.rating)}</span>
                </div>
                <p class="text-sm text-[var(--color-text-secondary)]">{rev.comment}</p>
              </div>
            {/each}
          </div>
        {/if}

        <div class="border-t border-[var(--color-border)] pt-4">
          <div class="flex items-center gap-2 mb-3">
            <span class="text-sm font-medium">评分</span>
            {#each [1,2,3,4,5] as star}
              <button class="text-xl transition-colors {star <= newReviewRating ? 'text-amber-500' : 'text-neutral-300'}" onclick={() => newReviewRating = star}>★</button>
            {/each}
          </div>
          <textarea class="input-field resize-none" rows="3" placeholder="写下你的评价..." bind:value={newReviewComment}></textarea>
          <div class="flex justify-end mt-3">
            <button
              class="btn-primary text-sm disabled:opacity-50"
              disabled={submittingReview || !newReviewComment.trim()}
              onclick={submitReview}
            >
              {submittingReview ? '提交中...' : '提交评论'}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

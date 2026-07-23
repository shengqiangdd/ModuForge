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

  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  function onSearchInput() {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      page = 1;
      loadModules();
    }, 300);
  }

  const categories = [
    { value: '', label: '全部', icon: 'apps' },
    { value: 'system', label: '系统', icon: 'phone_android' },
    { value: 'ui', label: '界面', icon: 'palette' },
    { value: 'audio', label: '音频', icon: 'headphones' },
    { value: 'display', label: '显示', icon: 'brightness_6' },
    { value: 'utility', label: '工具', icon: 'build' },
  ];

  const categoryColors: Record<string, string> = {
    system: 'system', ui: 'ui', audio: 'audio', display: 'display', utility: 'utility',
  };

  const categoryStyles: Record<string, string> = {
    system: 'background: rgba(59,130,246,0.15); color: #60a5fa',
    ui: 'background: rgba(168,85,247,0.15); color: #c084fc',
    audio: 'background: rgba(34,197,94,0.15); color: #4ade80',
    display: 'background: rgba(249,115,22,0.15); color: #fb923c',
    utility: 'background: rgba(161,161,170,0.15); color: #a1a1aa',
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

<style>
  .market-card {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .market-card:hover {
    border-color: var(--color-primary);
    box-shadow: 0 8px 32px rgba(139,92,246,0.15), 0 0 0 1px rgba(139,92,246,0.1);
    transform: translateY(-4px);
  }
  .cat-btn {
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .cat-btn:active {
    transform: scale(0.96);
  }
  .module-grid {
    animation: fadeIn 0.3s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(8px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>

<div class="p-4 md:p-6 max-w-7xl mx-auto">
  <!-- Header -->
  <div class="flex items-center justify-between mb-8">
    <div>
      <h1 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">ModuForge 市场</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">发现和分享优质 Magisk/KSU 模块</p>
    </div>
    <a href="/market/publish" class="btn-primary flex items-center gap-2 no-underline">
      <span class="material-symbols-outlined text-[18px]">publish</span>
      发布模块
    </a>
  </div>

  <!-- Search -->
  <div class="relative mb-5">
    <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-neutral-400 text-[20px] z-10">search</span>
    <div class="absolute left-[38px] top-2.5 bottom-2.5 w-px pointer-events-none z-10" style="background: var(--color-border)"></div>
    <input
      type="text"
      placeholder="搜索模块名称、描述、标签..."
      class="input-field market-search-input"
      style="padding-left: 48px;"
      bind:value={searchQuery}
      oninput={onSearchInput}
      onkeydown={(e) => { if (e.key === 'Enter') { if (debounceTimer) clearTimeout(debounceTimer); page = 1; loadModules(); } }}
    />
  </div>

  <!-- Categories -->
  <div class="flex gap-2 flex-wrap mb-4">
    {#each categories as cat}
      <button
        class="cat-btn flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium min-h-[44px]"
        style={selectedCategory === cat.value
          ? 'background: var(--gradient-brand); color: #fff; box-shadow: var(--shadow-glow)'
          : 'background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)'}
        onclick={() => { selectedCategory = cat.value; page = 1; loadModules(); }}
      >
        <span class="material-symbols-outlined text-[16px]">{cat.icon}</span>
        {cat.label}
      </button>
    {/each}
  </div>

  <!-- Sort & Count -->
  <div class="flex items-center gap-3 mb-6 text-sm" style="color: var(--color-text-secondary)">
    <span>排序</span>
    {#each [{ id: 'stars', label: '热度' }, { id: 'installs', label: '安装量' }, { id: 'newest', label: '最新' }] as s}
      <button
        class="px-3 py-1 rounded-lg transition-colors min-h-[36px]"
        style={sortBy === s.id ? 'background: var(--color-primary-light); color: var(--color-primary); font-weight: 600' : ''}
        onclick={() => { sortBy = s.id; page = 1; loadModules(); }}
      >
        {s.label}
      </button>
    {/each}
    <span class="ml-auto" style="color: var(--color-text-muted)">{total} 个模块</span>
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
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 module-grid" key={selectedCategory + sortBy + page}>
      {#each modules as mod, i}
        <button
          class="market-card text-left p-5 group cursor-pointer relative overflow-hidden"
          style="animation-delay: {i * 50}ms"
          onclick={() => openDetail(mod)}
        >
          <!-- Hover gradient overlay -->
          <div class="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-300" style="background: linear-gradient(135deg, rgba(139,92,246,0.05) 0%, rgba(6,182,212,0.03) 100%)"></div>
          
          <div class="relative z-10">
            <div class="flex items-center gap-3 mb-3">
              <span class="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
                <span class="material-symbols-outlined text-[14px] text-amber-500">star</span>
                {mod.stars}
              </span>
              <span class="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
                <span class="material-symbols-outlined text-[14px]">download</span>
                {fmt(mod.installs)}
              </span>
              <span class="ml-auto badge text-[10px]" style={categoryStyles[mod.category] || 'background: var(--color-surface); color: var(--color-text-muted)'}>
                {mod.category}
              </span>
            </div>
            <h3 class="font-semibold text-[var(--color-text)] mb-1 line-clamp-1 group-hover:text-[var(--color-primary)] transition-colors duration-200">{mod.title}</h3>
            <p class="text-xs text-[var(--color-text-muted)] mb-2">{mod.version} · {mod.author}</p>
            <p class="text-sm text-[var(--color-text-secondary)] line-clamp-2 leading-relaxed">{mod.description}</p>
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<!-- Detail Modal -->
{#if selectedModule}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => selectedModule = null}>
    <div class="rounded-2xl max-w-2xl w-full max-h-[85vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
      <div class="p-6 border-b" style="border-color: var(--color-border)">
        <div class="flex items-start justify-between">
          <div>
            <h2 class="text-xl font-bold text-[var(--color-text)]">{selectedModule.title}</h2>
            <div class="flex flex-wrap items-center gap-2 mt-2 text-sm text-[var(--color-text-muted)]">
              <span>{selectedModule.version}</span>
              <span>·</span>
              <span class="badge" style={categoryStyles[selectedModule.category] || ''}>{selectedModule.category}</span>
              <span>·</span>
              <span>{selectedModule.author}</span>
              <span>·</span>
              <span>{selectedModule.license}</span>
            </div>
          </div>
          <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={() => selectedModule = null}>
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
          <button class="ml-auto flex items-center gap-1.5 px-4 py-1.5 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--gradient-brand)" onclick={() => { selectedModule!.installs++; alert('模块下载链接已生成（演示功能）'); }}>
            <span class="material-symbols-outlined text-[16px]">download</span>
            安装
          </button>
        </div>
      </div>

      <div class="p-6 border-b" style="border-color: var(--color-border)">
        <h3 class="text-sm font-semibold text-[var(--color-text)] mb-2">描述</h3>
        <p class="text-sm text-[var(--color-text-secondary)] leading-relaxed">{selectedModule.description}</p>
        {#if selectedModule.tags}
          <div class="flex flex-wrap gap-1.5 mt-3">
            {#each selectedModule.tags.split(',') as tag}
              <span class="px-2.5 py-1 rounded-lg" style="background: var(--color-surface) text-xs text-[var(--color-text-muted)]">{tag.trim()}</span>
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
              <div class="p-3 rounded-xl" style="background: var(--color-surface)">
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

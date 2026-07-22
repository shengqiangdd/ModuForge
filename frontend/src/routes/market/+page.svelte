<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '../../lib/api/client';

  interface MarketModule {
    id: string;
    title: string;
    slug: string;
    description: string;
    category: string;
    tags: string;
    version: string;
    version_code: number;
    author: string;
    license: string;
    stars: number;
    installs: number;
    updated_at: string;
    created_at: string;
  }

  interface Review {
    id: string;
    module_id: string;
    uid: string;
    username: string;
    rating: number;
    comment: string;
    created_at: string;
  }

  let modules = $state<MarketModule[]>([]);
  let total = $state(0);
  let loading = $state(true);
  let searchQuery = $state('');
  let selectedCategory = $state('');
  let sortBy = $state('stars');
  let page = $state(1);
  const perPage = 20;

  // Detail modal
  let selectedModule = $state<MarketModule | null>(null);
  let reviews = $state<Review[]>([]);
  let reviewsLoading = $state(false);
  let newReviewRating = $state(5);
  let newReviewComment = $state('');
  let submittingReview = $state(false);

  const categories = [
    { value: '', label: '全部' },
    { value: 'system', label: '系统' },
    { value: 'ui', label: '界面' },
    { value: 'audio', label: '音频' },
    { value: 'display', label: '显示' },
    { value: 'utility', label: '工具' },
  ];

  const categoryColors: Record<string, string> = {
    system: 'bg-blue-100 text-blue-800',
    ui: 'bg-purple-100 text-purple-800',
    audio: 'bg-green-100 text-green-800',
    display: 'bg-orange-100 text-orange-800',
    utility: 'bg-gray-100 text-gray-800',
  };

  async function loadModules() {
    loading = true;
    try {
      const params = new URLSearchParams({
        page: String(page),
        per_page: String(perPage),
        sort: sortBy,
      });
      if (searchQuery) params.set('query', searchQuery);
      if (selectedCategory) params.set('category', selectedCategory);

      const res = await fetch(`/api/v1/market/modules?${params}`);
      if (res.ok) {
        const data = await res.json();
        modules = data.modules || [];
        total = data.total || 0;
      }
    } catch {
      modules = [];
    }
    loading = false;
  }

  async function openDetail(mod: MarketModule) {
    selectedModule = mod;
    reviewsLoading = true;
    try {
      const res = await fetch(`/api/v1/market/module/${mod.slug}/reviews`);
      if (res.ok) {
        const data = await res.json();
        reviews = data.reviews || [];
      }
    } catch {
      reviews = [];
    }
    reviewsLoading = false;
  }

  async function starModule() {
    if (!selectedModule) return;
    try {
      const res = await fetch(`/api/v1/market/module/${selectedModule.slug}/star`, { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        selectedModule = { ...selectedModule, stars: data.stars };
        // Update in list too
        const idx = modules.findIndex(m => m.id === selectedModule!.id);
        if (idx >= 0) modules[idx] = { ...modules[idx], stars: data.stars };
      }
    } catch {}
  }

  async function submitReview() {
    if (!selectedModule || !newReviewComment.trim()) return;
    submittingReview = true;
    try {
      const res = await fetch(`/api/v1/market/module/${selectedModule.slug}/review`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          uid: 'anonymous',
          username: 'Anonymous',
          rating: newReviewRating,
          comment: newReviewComment,
        }),
      });
      if (res.ok) {
        newReviewComment = '';
        newReviewRating = 5;
        // Reload reviews
        const reviewRes = await fetch(`/api/v1/market/module/${selectedModule.slug}/reviews`);
        if (reviewRes.ok) {
          const data = await reviewRes.json();
          reviews = data.reviews || [];
        }
      }
    } catch {}
    submittingReview = false;
  }

  function formatInstalls(n: number): string {
    if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
    return String(n);
  }

  function handleSearchKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      page = 1;
      loadModules();
    }
  }

  onMount(() => loadModules());
</script>

<div class="p-6 max-w-7xl mx-auto">
  <!-- Header -->
  <div class="flex items-center justify-between mb-6">
    <div>
      <h1 class="text-headline-large text-on-surface">ModuForge 市场</h1>
      <p class="text-body-medium text-on-surface-variant mt-1">发现和分享优质 Magisk/KSU 模块</p>
    </div>
    <md-filled-button href="/market/publish">
      <md-icon slot="icon">publish</md-icon>
      发布模块
    </md-filled-button>
  </div>

  <!-- Search & Filters -->
  <div class="mb-6 space-y-4">
    <div class="flex gap-3">
      <div class="flex-1 relative">
        <md-icon class="absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant">search</md-icon>
        <input
          type="text"
          placeholder="搜索模块..."
          class="w-full pl-10 pr-4 py-3 border border-outline rounded-xl bg-surface text-on-surface text-body-large"
          bind:value={searchQuery}
          onkeydown={handleSearchKeydown}
        />
      </div>
      <md-filled-tonal-button onclick={() => { page = 1; loadModules(); }}>
        搜索
      </md-filled-tonal-button>
    </div>

    <!-- Category tags -->
    <div class="flex gap-2 flex-wrap">
      {#each categories as cat}
        <button
          class="px-4 py-1.5 rounded-full text-label-medium transition-colors
            {selectedCategory === cat.value
              ? 'bg-primary text-on-primary'
              : 'bg-surface-container text-on-surface-variant hover:bg-surface-container-high'}"
          onclick={() => { selectedCategory = cat.value; page = 1; loadModules(); }}
        >
          {cat.label}
        </button>
      {/each}
    </div>

    <!-- Sort -->
    <div class="flex items-center gap-3 text-body-small text-on-surface-variant">
      <span>排序：</span>
      <button
        class="px-3 py-1 rounded-lg transition-colors {sortBy === 'stars' ? 'bg-primary-container text-on-primary-container' : 'hover:bg-surface-container'}"
        onclick={() => { sortBy = 'stars'; page = 1; loadModules(); }}
      >
        热度
      </button>
      <button
        class="px-3 py-1 rounded-lg transition-colors {sortBy === 'installs' ? 'bg-primary-container text-on-primary-container' : 'hover:bg-surface-container'}"
        onclick={() => { sortBy = 'installs'; page = 1; loadModules(); }}
      >
        安装量
      </button>
      <button
        class="px-3 py-1 rounded-lg transition-colors {sortBy === 'newest' ? 'bg-primary-container text-on-primary-container' : 'hover:bg-surface-container'}"
        onclick={() => { sortBy = 'newest'; page = 1; loadModules(); }}
      >
        最新
      </button>
      <span class="ml-auto">{total} 个模块</span>
    </div>
  </div>

  <!-- Module Grid -->
  {#if loading}
    <div class="flex justify-center p-12"><md-circular-progress indeterminate /></div>
  {:else if modules.length === 0}
    <div class="text-center p-12 text-on-surface-variant">
      <md-icon class="text-6xl mb-4">inventory_2</md-icon>
      <p class="text-body-large">没有找到匹配的模块</p>
    </div>
  {:else}
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      {#each modules as mod}
        <button
          class="text-left p-4 rounded-2xl border border-outline hover:border-primary hover:shadow-md transition-all cursor-pointer bg-surface"
          onclick={() => openDetail(mod)}
        >
          <!-- Stats row -->
          <div class="flex items-center gap-4 mb-3 text-body-small text-on-surface-variant">
            <span class="flex items-center gap-1">
              <md-icon class="text-sm">star</md-icon>
              {mod.stars}
            </span>
            <span class="flex items-center gap-1">
              <md-icon class="text-sm">download</md-icon>
              {formatInstalls(mod.installs)}
            </span>
          </div>

          <!-- Title -->
          <h3 class="text-title-medium text-on-surface mb-1 line-clamp-1">{mod.title}</h3>

          <!-- Version & Category -->
          <div class="flex items-center gap-2 mb-2 text-label-small text-on-surface-variant">
            <span>{mod.version}</span>
            <span>·</span>
            <span class="px-2 py-0.5 rounded-full {categoryColors[mod.category] || 'bg-gray-100 text-gray-800'}">
              {mod.category}
            </span>
          </div>

          <!-- Description -->
          <p class="text-body-small text-on-surface-variant line-clamp-2 mb-3">{mod.description}</p>

          <!-- Footer -->
          <div class="flex items-center justify-between text-label-small text-on-surface-variant">
            <span class="px-2 py-0.5 rounded bg-surface-container">{mod.license}</span>
            <span>{mod.author}</span>
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<!-- Detail Modal -->
{#if selectedModule}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4" onclick={() => selectedModule = null}>
    <div
      class="bg-surface rounded-2xl max-w-2xl w-full max-h-[85vh] overflow-auto shadow-2xl"
      onclick={(e) => e.stopPropagation()}
    >
      <!-- Header -->
      <div class="p-6 border-b border-outline-variant">
        <div class="flex items-start justify-between">
          <div>
            <h2 class="text-headline-small text-on-surface">{selectedModule.title}</h2>
            <div class="flex items-center gap-2 mt-2 text-body-small text-on-surface-variant">
              <span>{selectedModule.version}</span>
              <span>·</span>
              <span class="px-2 py-0.5 rounded-full {categoryColors[selectedModule.category] || 'bg-gray-100 text-gray-800'}">
                {selectedModule.category}
              </span>
              <span>·</span>
              <span>{selectedModule.author}</span>
              <span>·</span>
              <span>{selectedModule.license}</span>
            </div>
          </div>
          <button onclick={() => selectedModule = null}>
            <md-icon>close</md-icon>
          </button>
        </div>

        <!-- Stats -->
        <div class="flex items-center gap-6 mt-4">
          <button class="flex items-center gap-1 text-body-medium hover:text-primary transition-colors" onclick={starModule}>
            <md-icon>star</md-icon>
            <span>{selectedModule.stars} Stars</span>
          </button>
          <span class="flex items-center gap-1 text-body-medium text-on-surface-variant">
            <md-icon>download</md-icon>
            {formatInstalls(selectedModule.installs)} 安装
          </span>
        </div>
      </div>

      <!-- Description -->
      <div class="p-6 border-b border-outline-variant">
        <h3 class="text-title-small mb-2">描述</h3>
        <p class="text-body-medium text-on-surface-variant">{selectedModule.description}</p>
        {#if selectedModule.tags}
          <div class="flex flex-wrap gap-2 mt-3">
            {#each selectedModule.tags.split(',') as tag}
              <span class="px-2 py-1 bg-surface-container rounded text-label-small text-on-surface-variant">{tag.trim()}</span>
            {/each}
          </div>
        {/if}
      </div>

      <!-- Reviews -->
      <div class="p-6">
        <h3 class="text-title-small mb-4">评论</h3>

        {#if reviewsLoading}
          <div class="flex justify-center py-4"><md-circular-progress indeterminate /></div>
        {:else if reviews.length === 0}
          <p class="text-body-small text-on-surface-variant mb-4">暂无评论</p>
        {:else}
          <div class="space-y-3 mb-4 max-h-48 overflow-auto">
            {#each reviews as rev}
              <div class="p-3 bg-surface-container rounded-xl">
                <div class="flex items-center gap-2 mb-1">
                  <span class="text-label-medium">{rev.username}</span>
                  <span class="text-label-small text-on-surface-variant">
                    {'★'.repeat(rev.rating)}{'☆'.repeat(5 - rev.rating)}
                  </span>
                </div>
                <p class="text-body-small text-on-surface-variant">{rev.comment}</p>
              </div>
            {/each}
          </div>
        {/if}

        <!-- Add Review -->
        <div class="border-t border-outline-variant pt-4">
          <div class="flex items-center gap-2 mb-3">
            <span class="text-label-medium">评分：</span>
            {#each [1,2,3,4,5] as star}
              <button
                class="text-xl transition-colors {star <= newReviewRating ? 'text-amber-500' : 'text-on-surface-variant'}"
                onclick={() => newReviewRating = star}
              >
                ★
              </button>
            {/each}
          </div>
          <textarea
            class="w-full px-3 py-2 border border-outline rounded-xl bg-surface text-on-surface text-body-small resize-none"
            rows="3"
            placeholder="写下你的评价..."
            bind:value={newReviewComment}
          ></textarea>
          <div class="flex justify-end mt-2">
            <md-filled-tonal-button onclick={submitReview} disabled={submittingReview || !newReviewComment.trim()}>
              {submittingReview ? '提交中...' : '提交评论'}
            </md-filled-tonal-button>
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

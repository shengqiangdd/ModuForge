<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let favoriteItems = $state<{ id: number; item_type: string; item_id: number; created_at: string }[]>([]);
  let favFilter = $state('');

  async function loadFavorites() {
    const token = getToken();
    try {
      const params = favFilter ? `?type=${favFilter}` : '';
      const r = await fetch(`/api/v1/favorites${params}`, { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) { const d = await r.json(); favoriteItems = d.favorites || []; }
    } catch { favoriteItems = []; }
  }

  async function removeFavorite(f: { id: number; item_type: string; item_id: number }) {
    const token = getToken();
    try {
      await fetch(`/api/v1/favorites/${f.item_type}/${f.item_id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      favoriteItems = favoriteItems.filter(i => i.id !== f.id);
    } catch (e) { console.error('Failed to remove favorite:', e); }
  }

  onMount(() => {
    loadFavorites();
  });
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-warning-light)">
      <span class="material-symbols-outlined text-[18px] text-amber-500">star</span>
    </div>
    <div class="flex-1">
      <h2 class="text-base font-semibold text-[var(--color-text)]">收藏</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">你收藏的市场模块和项目</p>
    </div>
    <div class="flex items-center gap-1.5">
      <button type="button"
        class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors"
        style="background: {favFilter === 'module' ? 'var(--color-primary-light)' : 'var(--color-surface)'}; color: {favFilter === 'module' ? 'var(--color-primary)' : 'var(--color-text-secondary)'}"
        onclick={() => { favFilter = favFilter === 'module' ? '' : 'module'; loadFavorites(); }}>模块</button>
      <button type="button"
        class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors"
        style="background: {favFilter === 'project' ? 'var(--color-primary-light)' : 'var(--color-surface)'}; color: {favFilter === 'project' ? 'var(--color-primary)' : 'var(--color-text-secondary)'}"
        onclick={() => { favFilter = favFilter === 'project' ? '' : 'project'; loadFavorites(); }}>项目</button>
    </div>
  </div>
  {#if favoriteItems.length === 0}
    <p class="text-sm text-center py-4" style="color: var(--color-text-muted)">暂无收藏，去市场页把模块加为收藏吧</p>
  {:else}
    <div class="space-y-1.5">
      {#each favoriteItems as f (f.id)}
        <div class="flex items-center justify-between p-2.5 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
          <div class="flex items-center gap-2 min-w-0">
            <span class="material-symbols-outlined text-[16px] text-amber-500">star</span>
            <div class="min-w-0">
              <span class="text-sm font-medium text-[var(--color-text)]">{f.item_type}</span>
              <span class="text-xs ml-1.5" style="color: var(--color-text-muted)">#{f.item_id}</span>
            </div>
          </div>
          <button type="button"
            class="text-[11px] px-2.5 py-1 rounded-lg transition-colors"
            style="color: var(--color-error); background: var(--color-error-light)"
            onclick={() => removeFavorite(f)}>取消收藏</button>
        </div>
      {/each}
    </div>
  {/if}
</section>

<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n';

  interface GlossaryItem {
    id: number; term: string; definition: string;
    category: string; related_terms: string; created_at: string;
  }

  let items = $state<GlossaryItem[]>([]);
  let loading = $state(true);
  let searchQuery = $state('');
  let selectedCategory = $state('');
  let selectedItem = $state<GlossaryItem | null>(null);
  let isAdmin = $state(false);

  // Admin state
  let showForm = $state(false);
  let editingItem = $state<GlossaryItem | null>(null);
  let formTerm = $state('');
  let formDef = $state('');
  let formCat = $state('general');
  let formRelated = $state('');
  let saving = $state(false);

  const categories = [
    { value: '', label: '全部' },
    { value: 'general', label: '通用' },
    { value: 'dev', label: '开发' },
    { value: 'security', label: '安全' },
    { value: 'ai', label: 'AI' },
  ];

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }

  onMount(async () => {
    const token = getToken();
    if (token) {
      try {
        const payload = JSON.parse(atob(token.split('.')[1]));
        isAdmin = payload.role === 'admin';
      } catch { isAdmin = false; }
    }
    await loadItems();
  });

  async function loadItems() {
    loading = true;
    try {
      const params = new URLSearchParams();
      if (selectedCategory) params.set('category', selectedCategory);
      if (searchQuery) params.set('search', searchQuery);
      const token = getToken();
      const headers: Record<string, string> = {};
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const r = await fetch(`/api/v1/glossary?${params}`, { headers });
      if (r.ok) { const d = await r.json(); items = d.items || []; }
    } catch {}
    loading = false;
  }

  $effect(() => {
    searchQuery; selectedCategory;
    loadItems();
  });

  function openEdit(item: GlossaryItem) {
    editingItem = item;
    formTerm = item.term;
    formDef = item.definition;
    formCat = item.category;
    formRelated = item.related_terms || '';
    showForm = true;
  }

  function openNew() {
    editingItem = null;
    formTerm = '';
    formDef = '';
    formCat = 'general';
    formRelated = '';
    showForm = true;
  }

  async function saveItem() {
    if (!formTerm || !formDef) return;
    saving = true;
    const token = getToken();
    const body = { term: formTerm, definition: formDef, category: formCat, related_terms: formRelated };
    try {
      if (editingItem) {
        const r = await fetch(`/api/v1/admin/glossary/${editingItem.id}`, {
          method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify(body),
        });
        if (r.ok) { showForm = false; await loadItems(); }
      } else {
        const r = await fetch('/api/v1/admin/glossary', {
          method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify(body),
        });
        if (r.ok) { showForm = false; await loadItems(); }
        else { const d = await r.json(); alert(d.error || '创建失败'); }
      }
    } catch {}
    saving = false;
  }

  async function deleteItem(id: number) {
    if (!confirm('确定删除此术语？')) return;
    const token = getToken();
    try {
      await fetch(`/api/v1/admin/glossary/${id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      if (selectedItem?.id === id) selectedItem = null;
      await loadItems();
    } catch {}
  }
</script>

<div class="p-4 md:p-6 max-w-5xl mx-auto">
  <div class="flex items-center justify-between mb-6">
    <div>
      <h1 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">术语表</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">浏览和搜索 ModuForge 术语定义</p>
    </div>
    {#if isAdmin}
      <button class="btn-primary text-sm flex items-center gap-1.5" onclick={openNew}>
        <span class="material-symbols-outlined text-[16px]">add</span> 添加术语
      </button>
    {/if}
  </div>

  <!-- Filters -->
  <div class="flex flex-wrap gap-3 mb-6">
    <input type="text" class="input-field flex-1 min-w-[200px]" bind:value={searchQuery} placeholder="搜索术语..." />
    <div class="flex gap-2 flex-wrap">
      {#each categories as cat}
        <button
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
          style={selectedCategory === cat.value ? 'background: var(--color-primary-light); color: var(--color-primary)' : 'background: var(--color-surface); color: var(--color-text-muted)'}
          onclick={() => selectedCategory = cat.value}
        >{cat.label}</button>
      {/each}
    </div>
  </div>

  {#if loading}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
      {#each Array(6) as _}
        <div class="skeleton h-24 rounded-xl"></div>
      {/each}
    </div>
  {:else if items.length === 0}
    <div class="text-center py-16">
      <span class="material-symbols-outlined text-5xl text-neutral-300 mb-3 block">book</span>
      <p class="text-[var(--color-text-secondary)]">没有找到匹配的术语</p>
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
      {#each items as item}
        <div
          role="button"
          tabindex="0"
          class="text-left p-4 rounded-xl border transition-all hover:border-[var(--color-primary)] cursor-pointer"
          style="border-color: {selectedItem?.id === item.id ? 'var(--color-primary)' : 'var(--color-border)'}; background: {selectedItem?.id === item.id ? 'var(--color-primary-light)' : 'var(--color-bg-elevated)'}"
          onclick={() => selectedItem = selectedItem?.id === item.id ? null : item}
          onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); selectedItem = selectedItem?.id === item.id ? null : item; } }}
        >
          <div class="flex items-center justify-between mb-1">
            <h3 class="font-semibold text-[var(--color-text)]">{item.term}</h3>
            <span class="badge text-[10px]" style="background: {item.category === 'security' ? 'var(--color-error-light)' : item.category === 'ai' ? 'var(--color-primary-light)' : 'var(--color-surface)'}; color: {item.category === 'security' ? 'var(--color-error)' : item.category === 'ai' ? 'var(--color-primary)' : 'var(--color-text-muted)'}">
              {item.category === 'general' ? '通用' : item.category === 'dev' ? '开发' : item.category === 'security' ? '安全' : item.category === 'ai' ? 'AI' : item.category}
            </span>
          </div>
          <p class="text-sm text-[var(--color-text-secondary)] line-clamp-2">{item.definition}</p>
          {#if selectedItem?.id === item.id}
            <div class="mt-3 pt-3 border-t border-[var(--color-border)]">
              <p class="text-sm text-[var(--color-text)]">{item.definition}</p>
              {#if item.related_terms}
                <div class="flex flex-wrap gap-1.5 mt-2">
                  {#each item.related_terms.split(',') as rt}
                    <span class="px-2 py-0.5 rounded text-xs" style="background: var(--color-surface); color: var(--color-text-muted)">{rt.trim()}</span>
                  {/each}
                </div>
              {/if}
              {#if isAdmin}
                <div class="flex gap-2 mt-3">
                  <button class="text-xs px-2 py-1 rounded-lg" style="background: var(--color-surface); color: var(--color-text-secondary)" onclick={(e) => { e.stopPropagation(); openEdit(item); }}>编辑</button>
                  <button class="text-xs px-2 py-1 rounded-lg" style="background: var(--color-danger-light); color: var(--color-error)" onclick={(e) => { e.stopPropagation(); deleteItem(item.id); }}>删除</button>
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Edit/Create Modal -->
{#if showForm}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showForm = false; }}>
    <div class="card p-6 w-full max-w-lg" role="dialog" tabindex="-1">
      <div class="flex items-center gap-3 mb-5">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-info)">menu_book</span>
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--color-text)]">{editingItem ? '编辑' : '添加'}术语</h3>
        </div>
      </div>
      <div class="space-y-4">
        <div>
          <label for="glossary-term" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">术语</label>
          <input id="glossary-term" type="text" class="input-field" bind:value={formTerm} placeholder="e.g., ADB" />
        </div>
        <div>
          <label for="glossary-def" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">定义</label>
          <textarea id="glossary-def" class="input-field resize-none" rows="3" bind:value={formDef} placeholder="定义..."></textarea>
        </div>
        <div>
          <label for="glossary-cat" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">分类</label>
          <select id="glossary-cat" class="input-field" bind:value={formCat}>
            <option value="general">通用</option>
            <option value="dev">开发</option>
            <option value="security">安全</option>
            <option value="ai">AI</option>
          </select>
        </div>
        <div>
          <label for="glossary-related" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">相关术语（逗号分隔）</label>
          <input id="glossary-related" type="text" class="input-field" bind:value={formRelated} placeholder="ADB, Shell, Debug" />
        </div>
      </div>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button class="btn-ghost text-sm" onclick={() => showForm = false}>取消</button>
        <button class="btn-primary text-sm" onclick={saveItem} disabled={saving || !formTerm || !formDef}>
          {saving ? '保存中...' : '保存'}
        </button>
      </div>
    </div>
  </div>
{/if}
<script lang="ts">
  import { onMount } from 'svelte';

  interface SearchResult {
    type: string;
    id: number;
    name: string;
    description?: string;
    score?: number;
  }

  interface SearchHistoryItem {
    id: number;
    query: string;
    created_at: string;
  }

  let query = $state('');
  let results = $state<SearchResult[]>([]);
  let history = $state<SearchHistoryItem[]>([]);
  let loading = $state(false);
  let searched = $state(false);

  let grouped = $derived(() => {
    const groups: Record<string, SearchResult[]> = {};
    for (const r of results) {
      const key = r.type || '其他';
      if (!groups[key]) groups[key] = [];
      groups[key].push(r);
    }
    return groups;
  });

  function typeIcon(type: string) {
    switch (type) {
      case 'project': return '📁';
      case 'module': return '📦';
      case 'build': return '🔨';
      case 'file': return '📄';
      default: return '🔍';
    }
  }

  function typeLabel(type: string) {
    const labels: Record<string, string> = {
      project: '项目', module: '模块', build: '构建', file: '文件'
    };
    return labels[type] || type;
  }

  function highlight(text: string, q: string) {
    if (!q || !text) return text;
    const re = new RegExp(`(${q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
    return text.replace(re, '<mark>$1</mark>');
  }

  async function search() {
    if (!query.trim()) return;
    loading = true;
    searched = true;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/search?q=${encodeURIComponent(query)}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) results = await res.json();
      // Save to history
      await fetch('/api/v1/search/history', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ query })
      });
    } catch (e) { console.error(e); }
    loading = false;
    loadHistory();
  }

  async function loadHistory() {
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/search/history', {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) history = await res.json();
    } catch (e) { console.error(e); }
  }

  async function deleteHistory(id: number) {
    const token = localStorage.getItem('token');
    await fetch(`/api/v1/search/history/${id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` }
    });
    history = history.filter(h => h.id !== id);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') search();
  }

  onMount(loadHistory);
</script>

<div class="page">
  <h1>全局搜索</h1>

  <div class="search-bar">
    <input
      type="text"
      bind:value={query}
      onkeydown={handleKeydown}
      placeholder="搜索项目、模块、构建、文件..."
      class="search-input"
    />
    <button class="btn-primary" onclick={search} disabled={loading}>
      {loading ? '搜索中...' : '搜索'}
    </button>
  </div>

  {#if !searched && history.length > 0}
    <div class="section">
      <h3>搜索历史</h3>
      <div class="history-list">
        {#each history as h}
          <div class="history-item">
            <span class="history-query" onclick={() => { query = h.query; search(); }}>{h.query}</span>
            <button class="delete" onclick={() => deleteHistory(h.id)}>×</button>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if searched}
    {#if results.length === 0}
      <div class="empty">
        <span class="empty-icon">🔍</span>
        <p>没有找到匹配的结果</p>
      </div>
    {:else}
      <div class="results-info">找到 {results.length} 个结果</div>
      {#each Object.entries(grouped()) as [type, items]}
        <div class="section">
          <h3>{typeIcon(type)} {typeLabel(type)} ({items.length})</h3>
          <div class="result-list">
            {#each items as item}
              <div class="result-item">
                <div class="result-name">{highlight(item.name, query)}</div>
                {#if item.description}
                  <div class="result-desc">{highlight(item.description, query)}</div>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/each}
    {/if}
  {/if}
</div>

<style>
  .page { padding: 1.5rem; max-width: 800px; margin: 0 auto; }
  h1 { font-size: 1.5rem; color: var(--text-primary); margin-bottom: 1.5rem; }
  .search-bar { display: flex; gap: 0.75rem; margin-bottom: 1.5rem; }
  .search-input { flex: 1; padding: 0.7rem 1rem; border: 1px solid var(--border); border-radius: 10px; background: var(--bg-card); color: var(--text-primary); font-size: 1rem; outline: none; }
  .search-input:focus { border-color: var(--primary); }
  .btn-primary { padding: 0.7rem 1.5rem; border: none; border-radius: 10px; background: var(--primary); color: white; cursor: pointer; font-weight: 600; }
  .btn-primary:disabled { opacity: 0.6; }
  .section { margin-bottom: 1.5rem; }
  .section h3 { font-size: 1rem; color: var(--text-secondary); margin-bottom: 0.75rem; }
  .results-info { color: var(--text-tertiary); font-size: 0.85rem; margin-bottom: 1rem; }
  .result-list { display: flex; flex-direction: column; gap: 0.5rem; }
  .result-item { padding: 0.8rem 1rem; border: 1px solid var(--border); border-radius: 8px; background: var(--bg-card); }
  .result-name { font-weight: 600; color: var(--text-primary); }
  .result-desc { color: var(--text-secondary); font-size: 0.85rem; margin-top: 4px; }
  :global(mark) { background: #fef08a; color: inherit; border-radius: 2px; padding: 0 2px; }
  .history-list { display: flex; flex-direction: column; gap: 0.3rem; }
  .history-item { display: flex; justify-content: space-between; align-items: center; padding: 0.5rem 0.75rem; border: 1px solid var(--border); border-radius: 6px; }
  .history-query { cursor: pointer; color: var(--primary); }
  .history-query:hover { text-decoration: underline; }
  .delete { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 1.1rem; }
  .delete:hover { color: #ef4444; }
  .empty { text-align: center; padding: 3rem; color: var(--text-secondary); }
  .empty-icon { font-size: 3rem; display: block; margin-bottom: 1rem; }
</style>

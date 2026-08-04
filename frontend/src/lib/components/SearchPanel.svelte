<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '$lib/api/client';

  let { onClose, onNavigate }: { onClose: () => void; onNavigate: (route: string, id?: string) => void } = $props();

  let query = $state('');
  let results = $state<any[]>([]);
  let searching = $state(false);
  let selectedIndex = $state(0);
  let inputRef: HTMLInputElement | undefined = $state();

  interface SearchResult {
    project_id: string;
    project_name: string;
    file_path: string;
    context: string;
  }

  async function doSearch() {
    if (!query.trim()) { results = []; return; }
    searching = true;
    try {
      const data = await client.get<{ results: SearchResult[] }>(`/search?q=${encodeURIComponent(query)}`);
      results = data.results || [];
      selectedIndex = 0;
    } catch { results = []; }
    searching = false;
  }

  function selectItem(index: number) {
    const item = results[index];
    if (!item) return;
    if (item.file_path) {
      onNavigate('editor', item.project_id);
    } else {
      onNavigate('editor', item.project_id);
    }
    onClose();
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      selectedIndex = Math.min(selectedIndex + 1, results.length - 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      selectedIndex = Math.max(selectedIndex - 1, 0);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      selectItem(selectedIndex);
    }
  }

  onMount(() => {
    setTimeout(() => inputRef?.focus(), 50);
  });
</script>

<div
  class="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]"
  style="background: rgba(0,0,0,0.5); backdrop-filter: blur(4px)"
  role="presentation"
  onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
>
  <div
    class="w-full max-w-xl rounded-2xl shadow-2xl border overflow-hidden"
    style="background: var(--color-bg-elevated); border-color: var(--color-border);"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
  >
    <div class="flex items-center gap-3 px-4 py-3 border-b" style="border-color: var(--color-border);">
      <span class="material-symbols-outlined text-[20px]" style="color: var(--color-text-muted)">search</span>
      <input
        bind:this={inputRef}
        type="text"
        bind:value={query}
        oninput={doSearch}
        onkeydown={handleKeyDown}
        placeholder="搜索项目、文件..."
        class="flex-1 bg-transparent border-none outline-none text-sm"
        style="color: var(--color-text)"
      />
      {#if searching}
        <div class="animate-spin h-4 w-4 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
      {/if}
      <button class="text-xs px-2 py-1 rounded-md" style="background: var(--color-surface); color: var(--color-text-muted)" onclick={onClose}>
        ESC
      </button>
    </div>
    {#if results.length > 0}
      <div class="max-h-80 overflow-y-auto p-2">
        {#each results as item, i}
          <button
            class="w-full flex items-start gap-3 px-3 py-2.5 rounded-xl text-left transition-colors"
            style={i === selectedIndex ? 'background: var(--color-primary-light);' : ''}
            onclick={() => selectItem(i)}
            onmouseenter={() => selectedIndex = i}
          >
            <span class="material-symbols-outlined text-[18px] mt-0.5 flex-shrink-0" style="color: var(--color-text-muted)">
              {item.file_path ? 'description' : 'folder'}
            </span>
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium truncate" style="color: var(--color-text)">
                {item.project_name}
                {#if item.file_path}
                  <span class="text-xs font-normal" style="color: var(--color-text-muted)">/ {item.file_path}</span>
                {/if}
              </div>
              <div class="text-xs truncate mt-0.5" style="color: var(--color-text-muted)">{item.context}</div>
            </div>
          </button>
        {/each}
      </div>
    {:else if query && !searching}
      <div class="px-4 py-8 text-center text-sm" style="color: var(--color-text-muted)">
        没有找到匹配的结果
      </div>
    {/if}
    <div class="px-4 py-2 border-t flex items-center gap-4 text-xs" style="border-color: var(--color-border); color: var(--color-text-muted)">
      <span><kbd class="px-1 py-0.5 rounded" style="background: var(--color-surface);">↑↓</kbd> 导航</span>
      <span><kbd class="px-1 py-0.5 rounded" style="background: var(--color-surface);">Enter</kbd> 打开</span>
      <span><kbd class="px-1 py-0.5 rounded" style="background: var(--color-surface);">ESC</kbd> 关闭</span>
    </div>
  </div>
</div>

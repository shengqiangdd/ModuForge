<script>
  let templates = $state([]);
  let snippets = $state([]);
  let activeTab = $state('templates');
  let searchQuery = $state('');
  let selectedLanguage = $state('');
  let loading = $state(false);

  async function loadTemplates() {
    loading = true;
    try {
      const res = await fetch('/api/templates');
      if (res.ok) {
        const data = await res.json();
        templates = data.templates || [];
      }
    } catch (e) {
      console.error('Failed to load templates:', e);
    }
    loading = false;
  }

  async function loadSnippets() {
    loading = true;
    try {
      const url = searchQuery
        ? `/api/snippets/search?q=${encodeURIComponent(searchQuery)}&language=${selectedLanguage}`
        : '/api/snippets';
      const res = await fetch(url);
      if (res.ok) {
        const data = await res.json();
        snippets = data.snippets || [];
      }
    } catch (e) {
      console.error('Failed to load snippets:', e);
    }
    loading = false;
  }

  function switchTab(tab) {
    activeTab = tab;
    if (tab === 'templates') loadTemplates();
    else loadSnippets();
  }

  function getLanguageColor(lang) {
    const colors = {
      go: 'bg-cyan-100 text-cyan-700',
      python: 'bg-yellow-100 text-yellow-700',
      javascript: 'bg-orange-100 text-orange-700',
      typescript: 'bg-blue-100 text-blue-700'
    };
    return colors[lang] || 'bg-gray-100 text-gray-700';
  }

  $effect(() => {
    loadTemplates();
  });
</script>

<div class="template-gallery">
  <h2 class="text-xl font-semibold mb-4">项目模板与代码片段</h2>

  <!-- 标签页 -->
  <div class="flex gap-2 mb-4">
    <button
      onclick={() => switchTab('templates')}
      class="px-4 py-2 rounded {activeTab === 'templates' ? 'bg-blue-500 text-white' : 'bg-gray-100'}"
    >
      模板
    </button>
    <button
      onclick={() => switchTab('snippets')}
      class="px-4 py-2 rounded {activeTab === 'snippets' ? 'bg-blue-500 text-white' : 'bg-gray-100'}"
    >
      代码片段
    </button>
  </div>

  {#if activeTab === 'snippets'}
    <!-- 搜索栏 -->
    <div class="flex gap-4 mb-4">
      <input
        bind:value={searchQuery}
        placeholder="搜索代码片段..."
        class="flex-1 px-3 py-2 border rounded"
        oninput={loadSnippets}
      />
      <select bind:value={selectedLanguage} class="px-3 py-2 border rounded" onchange={loadSnippets}>
        <option value="">所有语言</option>
        <option value="go">Go</option>
        <option value="python">Python</option>
        <option value="javascript">JavaScript</option>
      </select>
    </div>
  {/if}

  {#if loading}
    <div class="text-center py-8 text-gray-500">加载中...</div>
  {:else if activeTab === 'templates'}
    <!-- 模板列表 -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each templates as template}
        <div class="card hover:shadow-lg transition-shadow cursor-pointer">
          <div class="flex items-center gap-2 mb-2">
            <span class="text-xs px-2 py-0.5 rounded {getLanguageColor(template.language)}">
              {template.language}
            </span>
            <span class="text-xs px-2 py-0.5 bg-gray-100 rounded">
              {template.category}
            </span>
          </div>
          <h3 class="font-semibold">{template.name}</h3>
          <p class="text-sm text-gray-500 mt-1">{template.description}</p>
          <div class="text-xs text-gray-400 mt-2">
            {template.files?.length || 0} 个文件
          </div>
        </div>
      {/each}
    </div>
  {:else}
    <!-- 代码片段列表 -->
    <div class="space-y-4">
      {#each snippets as snippet}
        <div class="card">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <span class="text-xs px-2 py-0.5 rounded {getLanguageColor(snippet.language)}">
                {snippet.language}
              </span>
              <span class="text-xs px-2 py-0.5 bg-gray-100 rounded">
                {snippet.category}
              </span>
            </div>
            <span class="text-xs text-gray-400">
              使用 {snippet.usage_count} 次
            </span>
          </div>
          <h3 class="font-semibold">{snippet.title}</h3>
          <p class="text-sm text-gray-500 mt-1">{snippet.description}</p>
          {#if snippet.tags?.length > 0}
            <div class="flex flex-wrap gap-1 mt-2">
              {#each snippet.tags as tag}
                <span class="text-xs px-2 py-0.5 bg-blue-50 text-blue-600 rounded">
                  {tag}
                </span>
              {/each}
            </div>
          {/if}
          <pre class="mt-3 p-3 bg-gray-50 rounded text-xs font-mono overflow-x-auto max-h-32">{snippet.code}</pre>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .template-gallery { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
</style>

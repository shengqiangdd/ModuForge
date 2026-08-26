<script>
  let files = $state({});
  let language = $state('go');
  let graph = $state(null);
  let stats = $state(null);
  let loading = $state(false);
  let error = $state(null);
  let fileInput = $state('');

  async function buildGraph() {
    if (Object.keys(files).length === 0) {
      error = '请先添加文件';
      return;
    }

    loading = true;
    error = null;
    graph = null;
    stats = null;

    try {
      const res = await fetch('/api/code/knowledge-graph', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files, language })
      });

      if (res.ok) {
        const data = await res.json();
        if (data.valid) {
          graph = data.graph;
          stats = data.stats;
        } else {
          error = data.error;
        }
      } else {
        error = await res.text();
      }
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }

  function addFile() {
    const parts = fileInput.split(':');
    if (parts.length === 2) {
      files[parts[0]] = parts[1];
      fileInput = '';
    }
  }

  function removeFile(name) {
    delete files[name];
    files = { ...files };
  }

  function getNodeTypeColor(type) {
    const colors = {
      package: 'bg-blue-100 text-blue-700',
      function: 'bg-green-100 text-green-700',
      struct: 'bg-purple-100 text-purple-700',
      interface: 'bg-orange-100 text-orange-700',
      variable: 'bg-gray-100 text-gray-700'
    };
    return colors[type] || 'bg-gray-100 text-gray-700';
  }

  function getNodeTypeIcon(type) {
    const icons = {
      package: '\u{1F4E6}',
      function: '\u{0192}',
      struct: '\u{1F3D7}\u{FE0F}',
      interface: '\u{1F50C}',
      variable: '\u{1F4E6}'
    };
    return icons[type] || '\u{2022}';
  }
</script>

<div class="knowledge-graph">
  <h2 class="text-xl font-semibold mb-4">代码知识图谱</h2>

  <div class="card mb-4">
    <div class="flex gap-4 mb-3">
      <div>
        <label class="text-sm text-gray-500 mb-1 block">语言</label>
        <select bind:value={language} class="px-3 py-2 border rounded">
          <option value="go">Go</option>
          <option value="javascript">JavaScript</option>
          <option value="python">Python</option>
        </select>
      </div>
    </div>

    <div class="mb-3">
      <label class="text-sm text-gray-500 mb-1 block">添加文件 (文件名:内容)</label>
      <div class="flex gap-2">
        <input
          bind:value={fileInput}
          placeholder="main.go:package main..."
          class="flex-1 px-3 py-2 border rounded"
        />
        <button onclick={addFile} class="px-4 py-2 bg-gray-200 rounded">添加</button>
      </div>
    </div>

    {#if Object.keys(files).length > 0}
      <div class="mb-3">
        <label class="text-sm text-gray-500 mb-1 block">已添加文件</label>
        <div class="flex flex-wrap gap-2">
          {#each Object.keys(files) as name}
            <span class="px-2 py-1 bg-gray-100 rounded text-sm flex items-center gap-1">
              {name}
              <button onclick={() => removeFile(name)} class="text-red-500 hover:text-red-700">&times;</button>
            </span>
          {/each}
        </div>
      </div>
    {/if}

    <button
      onclick={buildGraph}
      disabled={loading || Object.keys(files).length === 0}
      class="px-4 py-2 bg-teal-500 text-white rounded hover:bg-teal-600 disabled:opacity-50"
    >
      {loading ? '构建中...' : '构建图谱'}
    </button>
  </div>

  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}

  {#if graph && stats}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
      <div class="card text-center">
        <div class="text-2xl font-bold">{stats.total_nodes}</div>
        <div class="text-xs text-gray-500">节点总数</div>
      </div>
      <div class="card text-center">
        <div class="text-2xl font-bold">{stats.total_edges}</div>
        <div class="text-xs text-gray-500">关系总数</div>
      </div>
      <div class="card text-center">
        <div class="text-2xl font-bold">{stats.functions}</div>
        <div class="text-xs text-gray-500">函数</div>
      </div>
      <div class="card text-center">
        <div class="text-2xl font-bold">{stats.structs}</div>
        <div class="text-xs text-gray-500">结构体</div>
      </div>
    </div>

    <div class="card mb-4">
      <h3 class="card-title">节点列表 ({graph.nodes?.length || 0})</h3>
      <div class="space-y-2 max-h-64 overflow-y-auto">
        {#each graph.nodes || [] as node}
          <div class="p-2 bg-gray-50 rounded flex items-center justify-between">
            <div class="flex items-center gap-2">
              <span>{getNodeTypeIcon(node.type)}</span>
              <span class="font-mono text-sm">{node.label}</span>
              <span class="text-xs px-2 py-0.5 rounded {getNodeTypeColor(node.type)}">
                {node.type}
              </span>
            </div>
            {#if node.parent}
              <span class="text-xs text-gray-400">{node.parent}</span>
            {/if}
          </div>
        {/each}
      </div>
    </div>

    <div class="card">
      <h3 class="card-title">关系列表 ({graph.edges?.length || 0})</h3>
      <div class="space-y-2 max-h-64 overflow-y-auto">
        {#each graph.edges || [] as edge}
          <div class="p-2 bg-gray-50 rounded flex items-center gap-2 text-sm">
            <span class="font-mono">{edge.source.split(':').pop()}</span>
            <span class="text-gray-400">&rarr;</span>
            <span class="font-mono">{edge.target.split(':').pop()}</span>
            <span class="text-xs px-2 py-0.5 bg-blue-100 text-blue-700 rounded">
              {edge.type}
            </span>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .knowledge-graph { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

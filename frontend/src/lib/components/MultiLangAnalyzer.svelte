<script>
  let code = $state('');
  let language = $state('go');
  let result = $state(null);
  let loading = $state(false);
  let error = $state(null);

  const languages = [
    { value: 'go', label: 'Go' },
    { value: 'javascript', label: 'JavaScript' },
    { value: 'typescript', label: 'TypeScript' },
    { value: 'python', label: 'Python' },
  ];

  async function analyze() {
    if (!code.trim()) return;

    loading = true;
    error = null;
    result = null;

    try {
      const res = await fetch('/api/code/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code, language })
      });

      if (res.ok) {
        result = await res.json();
      } else {
        error = await res.text();
      }
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }

  function getComplexityColor(complexity) {
    if (complexity <= 5) return 'text-green-500';
    if (complexity <= 10) return 'text-yellow-500';
    return 'text-red-500';
  }
</script>

<div class="multi-lang-analyzer">
  <h2 class="text-xl font-semibold mb-4">🌐 多语言代码分析</h2>

  <div class="card mb-4">
    <div class="flex gap-4 mb-3">
      <div>
        <label class="text-sm text-gray-500 mb-1 block">语言</label>
        <select bind:value={language} class="px-3 py-2 border rounded">
          {#each languages as lang}
            <option value={lang.value}>{lang.label}</option>
          {/each}
        </select>
      </div>
    </div>

    <div class="mb-3">
      <label class="text-sm text-gray-500 mb-1 block">代码</label>
      <textarea
        bind:value={code}
        placeholder="粘贴代码进行分析..."
        rows="10"
        class="w-full px-3 py-2 border rounded font-mono text-sm"
      ></textarea>
    </div>

    <button
      onclick={analyze}
      disabled={loading || !code.trim()}
      class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50"
    >
      {loading ? '分析中...' : '开始分析'}
    </button>
  </div>

  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}

  {#if result?.valid && result.result}
    <div class="card mb-4">
      <h3 class="card-title">📊 分析结果</h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <div class="text-gray-500">语言</div>
          <div class="font-semibold">{result.result.language}</div>
        </div>
        <div>
          <div class="text-gray-500">代码行数</div>
          <div class="font-semibold">{result.result.lines}</div>
        </div>
        <div>
          <div class="text-gray-500">函数数量</div>
          <div class="font-semibold">{result.result.functions?.length || 0}</div>
        </div>
        <div>
          <div class="text-gray-500">复杂度</div>
          <div class="font-semibold {getComplexityColor(result.result.complexity)}">
            {result.result.complexity || 0}
          </div>
        </div>
      </div>
    </div>

    {#if result.result.imports?.length > 0}
      <div class="card mb-4">
        <h3 class="card-title">📦 导入 ({result.result.imports.length})</h3>
        <div class="flex flex-wrap gap-2">
          {#each result.result.imports as imp}
            <span class="px-2 py-1 bg-gray-100 rounded text-xs font-mono">{imp}</span>
          {/each}
        </div>
      </div>
    {/if}

    {#if result.result.functions?.length > 0}
      <div class="card mb-4">
        <h3 class="card-title">🔧 函数 ({result.result.functions.length})</h3>
        <div class="space-y-2 max-h-64 overflow-y-auto">
          {#each result.result.functions as func}
            <div class="p-2 bg-gray-50 rounded flex items-center justify-between">
              <span class="font-mono text-sm">{func.name}</span>
              <span class="text-xs text-gray-500">{func.exported ? '导出' : '私有'}</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if result.result.classes?.length > 0}
      <div class="card mb-4">
        <h3 class="card-title">🏗️ 类 ({result.result.classes.length})</h3>
        <div class="space-y-2 max-h-48 overflow-y-auto">
          {#each result.result.classes as cls}
            <div class="p-2 bg-gray-50 rounded">
              <span class="font-mono text-sm">{cls.name}</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if result.result.metrics && Object.keys(result.result.metrics).length > 0}
      <div class="card mb-4">
        <h3 class="card-title">📈 度量指标</h3>
        <div class="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
          {#each Object.entries(result.result.metrics) as [key, value]}
            <div>
              <div class="text-gray-500">{key}</div>
              <div class="font-semibold">{value}</div>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if result.result.warnings?.length > 0}
      <div class="card">
        <h3 class="card-title">⚠️ 警告 ({result.result.warnings.length})</h3>
        <ul class="space-y-1">
          {#each result.result.warnings as warning}
            <li class="flex items-center gap-2 text-sm text-yellow-600">
              <span>⚠️</span>
              <span>{warning}</span>
            </li>
          {/each}
        </ul>
      </div>
    {/if}
  {:else if result && !result.valid}
    <div class="card text-red-500">
      <div class="font-medium">分析错误</div>
      <div class="text-sm">{result.error}</div>
    </div>
  {/if}
</div>

<style>
  .multi-lang-analyzer { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

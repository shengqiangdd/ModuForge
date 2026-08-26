<script>
  let code = $state('');
  let language = $state('go');
  let result = $state(null);
  let loading = $state(false);
  let error = $state(null);
  
  async function analyze() {
    if (!code.trim()) return;
    
    loading = true;
    error = null;
    result = null;
    
    try {
      const res = await fetch('/api/code/refactor', {
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
  
  function getSeverityColor(severity) {
    const colors = {
      high: 'bg-red-100 text-red-700',
      medium: 'bg-yellow-100 text-yellow-700',
      low: 'bg-blue-100 text-blue-700'
    };
    return colors[severity] || 'bg-gray-100';
  }
  
  function getScoreColor(score) {
    if (score >= 80) return 'text-green-500';
    if (score >= 60) return 'text-yellow-500';
    return 'text-red-500';
  }
</script>

<div class="refactor-panel">
  <h2 class="text-xl font-semibold mb-4">代码重构建议</h2>
  
  <div class="card mb-4">
    <div class="flex gap-4 mb-3">
      <div>
        <label class="text-sm text-gray-500 mb-1 block">语言</label>
        <select bind:value={language} class="px-3 py-2 border rounded">
          <option value="go">Go</option>
          <option value="javascript">JavaScript</option>
          <option value="typescript">TypeScript</option>
          <option value="python">Python</option>
        </select>
      </div>
    </div>
    
    <div class="mb-3">
      <label class="text-sm text-gray-500 mb-1 block">代码</label>
      <textarea
        bind:value={code}
        placeholder="粘贴代码获取重构建议..."
        rows="10"
        class="w-full px-3 py-2 border rounded font-mono text-sm"
      ></textarea>
    </div>
    
    <button
      onclick={analyze}
      disabled={loading || !code.trim()}
      class="px-4 py-2 bg-purple-500 text-white rounded hover:bg-purple-600 disabled:opacity-50"
    >
      {loading ? '分析中...' : '获取建议'}
    </button>
  </div>
  
  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}
  
  {#if result?.valid && result.result}
    <!-- 质量评分 -->
    <div class="card mb-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="card-title">代码质量评分</h3>
          <p class="text-sm text-gray-500">{result.result.summary}</p>
        </div>
        <div class="text-center">
          <div class="text-4xl font-bold {getScoreColor(result.result.score)}">
            {result.result.score}
          </div>
          <div class="text-xs text-gray-500">/100</div>
        </div>
      </div>
    </div>
    
    <!-- 建议列表 -->
    {#if result.result.suggestions?.length > 0}
      <div class="card">
        <h3 class="card-title">改进建议 ({result.result.suggestions.length})</h3>
        <div class="space-y-3 max-h-96 overflow-y-auto">
          {#each result.result.suggestions as suggestion}
            <div class="p-3 bg-gray-50 rounded">
              <div class="flex items-center gap-2 mb-2">
                <span class="text-xs px-2 py-0.5 rounded {getSeverityColor(suggestion.severity)}">
                  {suggestion.severity}
                </span>
                <span class="text-xs px-2 py-0.5 bg-gray-200 rounded">
                  {suggestion.type}
                </span>
              </div>
              <div class="font-medium">{suggestion.title}</div>
              <div class="text-sm text-gray-600 mt-1">{suggestion.description}</div>
              {#if suggestion.location}
                <div class="text-xs text-gray-400 mt-1">位置: {suggestion.location}</div>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {:else}
      <div class="card text-center py-8 text-gray-500">
        代码质量很好，暂无改进建议
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
  .refactor-panel { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

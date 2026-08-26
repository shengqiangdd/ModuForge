<script>
  let prompt = $state('');
  let models = $state(['deepseek-v4-flash', 'qwen3.8-max']);
  let availableModels = $state(['deepseek-v4-flash', 'qwen3.8-max', 'xiaomi/mimo-v2.5', 'gpt-4o-mini']);
  let result = $state(null);
  let loading = $state(false);
  let error = $state(null);
  
  async function generate() {
    if (!prompt.trim() || models.length === 0) return;
    loading = true;
    error = null;
    result = null;
    try {
      const res = await fetch('/api/ensemble/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prompt, models })
      });
      if (res.ok) result = await res.json();
      else error = await res.text();
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  function toggleModel(model) {
    if (models.includes(model)) models = models.filter(m => m !== model);
    else models = [...models, model];
  }
</script>

<div class="ensemble-panel">
  <h2 class="text-xl font-semibold mb-4">🤖 多模型协同生成</h2>
  
  <div class="card mb-4">
    <h3 class="card-title">选择模型</h3>
    <div class="flex flex-wrap gap-2">
      {#each availableModels as model}
        <button onclick={() => toggleModel(model)} class="px-3 py-1 text-sm rounded {models.includes(model) ? 'bg-blue-500 text-white' : 'bg-gray-100 hover:bg-gray-200'}">{model}</button>
      {/each}
    </div>
    <div class="text-xs text-gray-500 mt-2">已选择 {models.length} 个模型</div>
  </div>
  
  <div class="card mb-4">
    <h3 class="card-title">输入提示</h3>
    <textarea bind:value={prompt} placeholder="输入你的提示..." rows="4" class="w-full px-3 py-2 border rounded resize-none"></textarea>
    <button onclick={generate} disabled={loading || !prompt.trim() || models.length === 0} class="mt-3 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50">{loading ? '生成中...' : '开始生成'}</button>
  </div>
  
  {#if error}<div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>{/if}
  
  {#if result}
    <div class="card">
      <h3 class="card-title">生成结果</h3>
      <div class="grid grid-cols-2 gap-4 mb-4 text-sm">
        <div><span class="text-gray-500">最佳模型:</span><span class="font-mono font-medium ml-2">{result.model}</span></div>
        <div><span class="text-gray-500">质量评分:</span><span class="ml-2">{(result.quality * 100).toFixed(1)}%</span></div>
        <div><span class="text-gray-500">耗时:</span><span class="ml-2">{result.duration_ms}ms</span></div>
      </div>
      <div class="bg-gray-50 p-3 rounded font-mono text-sm whitespace-pre-wrap">{result.content}</div>
    </div>
  {/if}
</div>

<style>
  .ensemble-panel { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

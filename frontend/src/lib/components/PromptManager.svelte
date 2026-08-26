<script>
  import { onMount } from 'svelte';
  
  let templates = $state([]);
  let selectedTemplate = $state(null);
  let taskDescription = $state('');
  let loading = $state(false);
  let error = $state(null);
  
  async function fetchTemplates(category = '') {
    try {
      const url = category ? `/api/prompts/list?category=${category}` : '/api/prompts/list';
      const res = await fetch(url);
      if (res.ok) templates = await res.json();
    } catch (e) { console.error('Failed to fetch templates:', e); }
  }
  
  async function selectTemplate() {
    if (!taskDescription.trim()) return;
    loading = true;
    error = null;
    try {
      const res = await fetch('/api/prompts/select', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ task: taskDescription })
      });
      if (res.ok) {
        const data = await res.json();
        selectedTemplate = templates.find(t => t.name === data.template);
      } else { error = await res.text(); }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  onMount(() => fetchTemplates());
</script>

<div class="prompt-manager">
  <h2 class="text-xl font-semibold mb-4">🎯 提示词模板</h2>
  
  <div class="card mb-4">
    <h3 class="card-title">智能选择</h3>
    <div class="flex gap-2">
      <input type="text" bind:value={taskDescription} placeholder="描述你的任务..." class="flex-1 px-3 py-2 border rounded" onkeydown={(e) => e.key === 'Enter' && selectTemplate()} />
      <button onclick={selectTemplate} disabled={loading || !taskDescription.trim()} class="px-4 py-2 bg-purple-500 text-white rounded hover:bg-purple-600 disabled:opacity-50">{loading ? '选择中...' : '选择模板'}</button>
    </div>
    {#if error}<div class="text-red-500 text-sm mt-2">{error}</div>{/if}
    {#if selectedTemplate}
      <div class="mt-4 p-3 bg-purple-50 rounded">
        <div class="font-medium text-purple-700">{selectedTemplate.name}</div>
        <div class="text-sm text-gray-600 mt-1">{selectedTemplate.description}</div>
        <div class="text-xs text-gray-500 mt-2">分类: {selectedTemplate.category} | 变量: {selectedTemplate.variables?.join(', ')}</div>
      </div>
    {/if}
  </div>
  
  <div class="card">
    <h3 class="card-title">所有模板 ({templates.length})</h3>
    <div class="space-y-2">
      {#each templates as template}
        <div class="p-3 border rounded cursor-pointer hover:bg-gray-50 {selectedTemplate?.name === template.name ? 'border-purple-500 bg-purple-50' : ''}" onclick={() => selectedTemplate = template}>
          <div class="flex items-center gap-2"><span class="font-medium">{template.name}</span><span class="text-xs px-2 py-0.5 bg-gray-100 rounded">{template.category}</span></div>
          <div class="text-sm text-gray-600 mt-1">{template.description}</div>
        </div>
      {/each}
      {#if templates.length === 0}<div class="text-gray-500 text-sm text-center py-4">暂无模板</div>{/if}
    </div>
  </div>
</div>

<style>
  .prompt-manager { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

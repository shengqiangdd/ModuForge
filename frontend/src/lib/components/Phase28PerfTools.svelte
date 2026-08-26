<script>
  let activeTab = $state('cache');
  
  // Cache state
  let cacheKey = $state('');
  let cacheValue = $state('');
  let cacheStats = $state(null);
  let cacheMessage = $state('');
  
  // Bloom filter state
  let bloomItem = $state('');
  let bloomItems = $state('');
  let bloomResult = $state(null);
  
  // Trie state
  let triePrefix = $state('');
  let trieKeywords = $state('');
  let trieResult = $state(null);
  
  // Pool state
  let poolTasks = $state(10);
  let poolResult = $state(null);
  
  let loading = $state(false);
  let error = $state(null);
  
  async function cachePut() {
    if (!cacheKey.trim()) return;
    loading = true; error = null;
    try {
      const res = await fetch('/api/code/cache/put', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: cacheKey, value: cacheValue })
      });
      if (res.ok) {
        const data = await res.json();
        cacheStats = data.stats;
        cacheMessage = '✅ 已缓存';
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function cacheGet() {
    if (!cacheKey.trim()) return;
    loading = true; error = null;
    try {
      const res = await fetch(`/api/code/cache/get?key=${encodeURIComponent(cacheKey)}`);
      if (res.ok) {
        const data = await res.json();
        cacheValue = data.value || '';
        cacheStats = data.stats;
        cacheMessage = data.found ? `✅ 找到: ${data.value}` : '❌ 未找到';
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function bloomCheck() {
    if (!bloomItem.trim()) return;
    loading = true; error = null;
    try {
      const res = await fetch('/api/code/bloom', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'check', item: bloomItem })
      });
      if (res.ok) {
        const data = await res.json();
        bloomResult = { exists: data.found, item: data.item };
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function bloomBatchAdd() {
    const items = bloomItems.split('\n').map(s => s.trim()).filter(s => s);
    if (items.length === 0) return;
    loading = true; error = null;
    try {
      const res = await fetch('/api/code/bloom', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'batch-add', items })
      });
      if (res.ok) {
        const data = await res.json();
        bloomResult = { approximateCount: data.approximateCount, added: items.length };
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function trieSearch() {
    const keywords = {};
    trieKeywords.split('\n').forEach(line => {
      const parts = line.split(':');
      if (parts.length === 2) {
        keywords[parts[0].trim()] = parts[1].trim();
      }
    });
    if (Object.keys(keywords).length === 0 || !triePrefix.trim()) return;
    
    loading = true; error = null;
    try {
      const res = await fetch('/api/code/trie', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'batch', keywords, prefix: triePrefix })
      });
      if (res.ok) {
        const data = await res.json();
        trieResult = { results: data.results, count: data.count, size: data.size };
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function runPool() {
    loading = true; error = null;
    try {
      const res = await fetch('/api/code/pool', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tasks: poolTasks })
      });
      if (res.ok) {
        const data = await res.json();
        poolResult = data;
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
</script>

<div class="perf-tools">
  <h2 class="text-xl font-semibold mb-4">⚡ Phase 28 - 性能优化</h2>
  
  <div class="flex gap-2 mb-4 flex-wrap">
    <button onclick={() => activeTab='cache'} class="px-4 py-2 rounded {activeTab==='cache'?'bg-green-500 text-white':'bg-gray-100'}">💾 LRU缓存</button>
    <button onclick={() => activeTab='bloom'} class="px-4 py-2 rounded {activeTab==='bloom'?'bg-purple-500 text-white':'bg-gray-100'}">🌸 Bloom Filter</button>
    <button onclick={() => activeTab='trie'} class="px-4 py-2 rounded {activeTab==='trie'?'bg-blue-500 text-white':'bg-gray-100'}">🌳 Trie前缀树</button>
    <button onclick={() => activeTab='pool'} class="px-4 py-2 rounded {activeTab==='pool'?'bg-orange-500 text-white':'bg-gray-100'}">🔄 协程池</button>
  </div>
  
  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}
  
  <!-- Cache Tab -->
  {#if activeTab === 'cache'}
    <div class="card mb-4">
      <h3 class="card-title">LRU Cache (容量1000)</h3>
      <div class="flex gap-2 mb-3">
        <input bind:value={cacheKey} class="flex-1 px-3 py-2 border rounded" placeholder="Key" />
        <input bind:value={cacheValue} class="flex-1 px-3 py-2 border rounded" placeholder="Value" />
      </div>
      <div class="flex gap-2">
        <button onclick={cachePut} disabled={loading || !cacheKey.trim()} class="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600 disabled:opacity-50">PUT</button>
        <button onclick={cacheGet} disabled={loading || !cacheKey.trim()} class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50">GET</button>
      </div>
      {#if cacheMessage}
        <div class="mt-3 p-2 bg-gray-50 rounded text-sm">{cacheMessage}</div>
      {/if}
    </div>
    
    {#if cacheStats}
      <div class="card">
        <h3 class="card-title">缓存统计</h3>
        <div class="grid grid-cols-4 gap-4 text-center text-sm">
          <div><div class="text-xl font-bold">{cacheStats.hits}</div><div class="text-xs text-gray-500">命中</div></div>
          <div><div class="text-xl font-bold">{cacheStats.misses}</div><div class="text-xs text-gray-500">未命中</div></div>
          <div><div class="text-xl font-bold text-green-500">{cacheStats.hit_rate?.toFixed(1)}%</div><div class="text-xs text-gray-500">命中率</div></div>
          <div><div class="text-xl font-bold">{cacheStats.size}/{cacheStats.capacity}</div><div class="text-xs text-gray-500">使用率</div></div>
        </div>
      </div>
    {/if}
  {/if}
  
  <!-- Bloom Filter Tab -->
  {#if activeTab === 'bloom'}
    <div class="card mb-4">
      <h3 class="card-title">布隆过滤器</h3>
      <div class="mb-3">
        <textarea bind:value={bloomItems} rows="3" class="w-full px-3 py-2 border rounded font-mono text-sm mb-2" placeholder="批量添加（每行一个）"></textarea>
        <button onclick={bloomBatchAdd} disabled={loading || !bloomItems.trim()} class="px-4 py-2 bg-purple-500 text-white rounded hover:bg-purple-600 disabled:opacity-50">
          批量添加
        </button>
      </div>
      <div class="flex gap-2">
        <input bind:value={bloomItem} class="flex-1 px-3 py-2 border rounded" placeholder="检查元素是否存在" />
        <button onclick={bloomCheck} disabled={loading || !bloomItem.trim()} class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50">检查</button>
      </div>
    </div>
    
    {#if bloomResult}
      <div class="card">
        {#if bloomResult.approximateCount !== undefined}
          <div class="text-center py-4">
            <div class="text-3xl font-bold text-purple-500">{bloomResult.approximateCount?.toFixed(0)}</div>
            <div class="text-sm text-gray-500">近似元素数量</div>
            <div class="text-xs text-gray-400 mt-1">已添加 {bloomResult.added} 个元素</div>
          </div>
        {:else}
          <div class="text-center py-4">
            <div class="text-3xl font-bold {bloomResult.exists ? 'text-green-500' : 'text-red-500'}">{bloomResult.exists ? '✅ 可能存在' : '❌ 一定不存在'}</div>
            <div class="text-sm text-gray-500">"{bloomResult.item}"</div>
            <div class="text-xs text-gray-400 mt-1">假阳性率: 1%</div>
          </div>
        {/if}
      </div>
    {/if}
  {/if}
  
  <!-- Trie Tab -->
  {#if activeTab === 'trie'}
    <div class="card mb-4">
      <h3 class="card-title">Trie 前缀树</h3>
      <div class="mb-3">
        <textarea bind:value={trieKeywords} rows="3" class="w-full px-3 py-2 border rounded font-mono text-sm mb-2" placeholder="关键字列表（每行 key:category）"></textarea>
        <div class="flex gap-2">
          <input bind:value={triePrefix} class="flex-1 px-3 py-2 border rounded" placeholder="前缀" />
          <button onclick={trieSearch} disabled={loading || !triePrefix.trim()} class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50">前缀搜索</button>
        </div>
      </div>
    </div>
    
    {#if trieResult}
      <div class="card">
        <div class="text-sm text-gray-500 mb-2">前缀 "{triePrefix}" 找到 {trieResult.count} 个匹配，树中共 {trieResult.size} 个关键字</div>
        {#if trieResult.results?.length > 0}
          <div class="space-y-1">
            {#each trieResult.results as result}
              <div class="p-2 bg-blue-50 rounded text-sm">{result}</div>
            {/each}
          </div>
        {:else}
          <div class="text-center py-4 text-gray-400">无匹配结果</div>
        {/if}
      </div>
    {/if}
  {/if}
  
  <!-- Pool Tab -->
  {#if activeTab === 'pool'}
    <div class="card mb-4">
      <h3 class="card-title">协程池 (Worker: 8, Queue: 100)</h3>
      <div class="flex gap-2 mb-3">
        <input type="number" bind:value={poolTasks} min="1" max="1000" class="w-32 px-3 py-2 border rounded" />
        <span class="self-center text-sm text-gray-500">任务数量</span>
      </div>
      <button onclick={runPool} disabled={loading} class="px-4 py-2 bg-orange-500 text-white rounded hover:bg-orange-600 disabled:opacity-50">
        {loading ? '执行中...' : '执行任务'}
      </button>
    </div>
    
    {#if poolResult}
      <div class="card">
        <div class="grid grid-cols-2 gap-4 text-center">
          <div><div class="text-3xl font-bold text-green-500">{poolResult.completed}</div><div class="text-xs text-gray-500">完成任务数</div></div>
          <div><div class="text-3xl font-bold text-orange-500">{poolResult.tasks}</div><div class="text-xs text-gray-500">提交任务数</div></div>
        </div>
        <div class="text-center mt-2 text-sm text-gray-500">耗时: 所有 {poolResult.tasks} 个任务在 8 个 worker 上并行执行</div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .perf-tools { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

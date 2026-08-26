<script>
  let activeTab = $state('complexity');
  
  // Complexity state
  let complexityFiles = $state([{name: 'main.go', code: ''}]);
  let complexityResult = $state(null);
  
  // Dependencies state
  let rootDir = $state('.');
  let depResult = $state(null);
  
  // Custom Rules state
  let customRules = $state([{id: 'no-print', name: '禁止Println', pattern: 'fmt\\.Println', message: '生产代码不应使用 Println', severity: 'warning', language: 'go'}]);
  let ruleFiles = $state([{name: 'main.go', code: ''}]);
  let ruleLanguage = $state('go');
  let ruleResults = $state(null);

  
  let loading = $state(false);
  let error = $state(null);
  
  async function analyzeComplexity() {
    const files = {};
    complexityFiles.forEach(f => { if (f.code.trim()) files[f.name] = f.code; });
    if (Object.keys(files).length === 0) return;
    
    loading = true; error = null; complexityResult = null;
    try {
      const res = await fetch('/api/code/project/complexity', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files })
      });
      if (res.ok) {
        const data = await res.json();
        complexityResult = data.result;
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function analyzeDependencies() {
    loading = true; error = null; depResult = null;
    try {
      const res = await fetch('/api/code/project/dependencies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ root_dir: rootDir })
      });
      if (res.ok) {
        const data = await res.json();
        depResult = data.result;
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function runCustomRules() {
    const files = {};
    ruleFiles.forEach(f => { if (f.code.trim()) files[f.name] = f.code; });
    if (Object.keys(files).length === 0) return;
    
    loading = true; error = null; ruleResults = null;
    try {
      const res = await fetch('/api/code/project/custom-rules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files, language: ruleLanguage, rules: customRules })
      });
      if (res.ok) {
        const data = await res.json();
        ruleResults = data.results;
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  function addComplexityFile() {
    complexityFiles = [...complexityFiles, { name: `file${complexityFiles.length+1}.go`, code: '' }];
  }
  
  function addRuleFile() {
    ruleFiles = [...ruleFiles, { name: `file${ruleFiles.length+1}.go`, code: '' }];
  }
  
  function addCustomRule() {
    customRules = [...customRules, { id: `rule-${Date.now()}`, name: 'New Rule', pattern: '', message: '', severity: 'warning', language: '' }];
  }
  
  function getRiskColor(level) {
    return { low:'text-green-500', medium:'text-yellow-500', high:'text-orange-500', critical:'text-red-500' }[level] || 'text-gray-500';
  }
</script>

<div class="phase27-tools">
  <h2 class="text-xl font-semibold mb-4">📊 Phase 27 - 依赖/复杂度/规则</h2>
  
  <div class="flex gap-2 mb-4 flex-wrap">
    <button onclick={() => activeTab='complexity'} class="px-4 py-2 rounded {activeTab==='complexity'?'bg-blue-500 text-white':'bg-gray-100'}">📈 复杂度分析</button>
    <button onclick={() => activeTab='deps'} class="px-4 py-2 rounded {activeTab==='deps'?'bg-purple-500 text-white':'bg-gray-100'}">📦 依赖分析</button>
    <button onclick={() => activeTab='rules'} class="px-4 py-2 rounded {activeTab==='rules'?'bg-orange-500 text-white':'bg-gray-100'}">📏 自定义规则</button>
  </div>
  
  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}
  
  <!-- Complexity Tab -->
  {#if activeTab === 'complexity'}
    <div class="card mb-4">
      <div class="flex justify-between items-center mb-3">
        <h3 class="card-title">代码文件</h3>
        <button onclick={addComplexityFile} class="text-sm text-blue-500">+ 添加文件</button>
      </div>
      <div class="space-y-3 max-h-64 overflow-y-auto mb-3">
        {#each complexityFiles as file, i}
          <div class="flex gap-2">
            <input bind:value={file.name} class="px-2 py-1 border rounded text-sm w-1/4" placeholder="文件名" />
            <textarea bind:value={file.code} rows="2" class="flex-1 px-3 py-2 border rounded font-mono text-sm" placeholder="粘贴代码..."></textarea>
          </div>
        {/each}
      </div>
      <button onclick={analyzeComplexity} disabled={loading} class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50">
        {loading ? '分析中...' : '分析复杂度'}
      </button>
    </div>
    
    {#if complexityResult}
      <div class="card mb-4">
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-center">
          <div><div class="text-2xl font-bold">{complexityResult.total_files}</div><div class="text-xs text-gray-500">文件数</div></div>
          <div><div class="text-2xl font-bold">{complexityResult.total_lines}</div><div class="text-xs text-gray-500">总行数</div></div>
          <div><div class="text-2xl font-bold text-yellow-500">{complexityResult.avg_complexity?.toFixed(1)}</div><div class="text-xs text-gray-500">平均复杂度</div></div>
          <div><div class="text-2xl font-bold text-red-500">{complexityResult.max_complexity}</div><div class="text-xs text-gray-500">最大复杂度</div></div>
        </div>
      </div>
      
      {#if complexityResult.files?.length > 0}
        <div class="card">
          <h3 class="card-title">文件详情</h3>
          <div class="space-y-2 max-h-64 overflow-y-auto">
            {#each complexityResult.files as file}
              <div class="flex items-center justify-between p-2 bg-gray-50 rounded text-sm">
                <span class="font-mono">{file.file_name}</span>
                <div class="flex gap-4">
                  <span>{file.lines} 行</span>
                  <span class="{getRiskColor(file.risk_level)}">复杂度: {file.complexity} ({file.risk_level})</span>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/if}
  {/if}
  
  <!-- Dependencies Tab -->
  {#if activeTab === 'deps'}
    <div class="card mb-4">
      <div class="flex gap-2 mb-3">
        <input bind:value={rootDir} class="flex-1 px-3 py-2 border rounded" placeholder="项目根目录路径 (默认当前目录)" />
        <button onclick={analyzeDependencies} disabled={loading} class="px-4 py-2 bg-purple-500 text-white rounded hover:bg-purple-600 disabled:opacity-50">
          {loading ? '分析中...' : '分析依赖'}
        </button>
      </div>
    </div>
    
    {#if depResult}
      <div class="card">
        <h3 class="card-title">{depResult.language.toUpperCase()} 依赖分析</h3>
        <div class="mb-2 text-sm text-gray-500">文件: {depResult.file_name}</div>
        <div class="text-sm mb-4">共 {depResult.dependencies?.length || 0} 个依赖</div>
        
        {#if depResult.dependencies?.length > 0}
          <div class="grid grid-cols-1 md:grid-cols-2 gap-2 max-h-64 overflow-y-auto">
            {#each depResult.dependencies as dep}
              <div class="p-2 bg-gray-50 rounded text-sm flex justify-between">
                <span class="font-mono truncate">{dep.name}</span>
                <span class="text-gray-500 text-xs">{dep.version}</span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  {/if}
  
  <!-- Rules Tab -->
  {#if activeTab === 'rules'}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
      <div class="card">
        <div class="flex justify-between items-center mb-3">
          <h3 class="card-title">自定义规则</h3>
          <button onclick={addCustomRule} class="text-sm text-orange-500">+ 添加规则</button>
        </div>
        <div class="space-y-3 max-h-64 overflow-y-auto">
          {#each customRules as rule, i}
            <div class="p-2 bg-gray-50 rounded text-sm">
              <input bind:value={rule.name} class="w-full px-2 py-1 border rounded mb-1 text-xs" placeholder="规则名称" />
              <input bind:value={rule.pattern} class="w-full px-2 py-1 border rounded mb-1 text-xs font-mono" placeholder="正则表达式" />
              <div class="flex gap-2">
                <select bind:value={rule.severity} class="px-2 py-1 border rounded text-xs">
                  <option value="error">Error</option>
                  <option value="warning">Warning</option>
                  <option value="info">Info</option>
                </select>
                <input bind:value={rule.message} class="flex-1 px-2 py-1 border rounded text-xs" placeholder="告警信息" />
              </div>
            </div>
          {/each}
        </div>
      </div>
      
      <div class="card">
        <div class="flex justify-between items-center mb-3">
          <h3 class="card-title">待检查代码</h3>
          <button onclick={addRuleFile} class="text-sm text-orange-500">+ 添加文件</button>
        </div>
        <select bind:value={ruleLanguage} class="w-full px-3 py-2 border rounded mb-3 text-sm">
          <option value="go">Go</option>
          <option value="javascript">JavaScript</option>
          <option value="python">Python</option>
        </select>
        <div class="space-y-3 max-h-48 overflow-y-auto">
          {#each ruleFiles as file}
            <input bind:value={file.name} class="w-full px-2 py-1 border rounded text-sm mb-1" placeholder="文件名" />
            <textarea bind:value={file.code} rows="3" class="w-full px-3 py-2 border rounded font-mono text-xs" placeholder="粘贴代码..."></textarea>
          {/each}
        </div>
      </div>
    </div>
    
    <button onclick={runCustomRules} disabled={loading} class="w-full px-4 py-2 bg-orange-500 text-white rounded hover:bg-orange-600 disabled:opacity-50 mb-4">
      {loading ? '检查中...' : '执行规则检查'}
    </button>
    
    {#if ruleResults}
      <div class="card">
        <h3 class="card-title">检查结果 ({ruleResults.length})</h3>
        {#if ruleResults.length === 0}
          <div class="text-center py-4 text-green-500">✅ 全部通过</div>
        {:else}
          <div class="space-y-2 max-h-64 overflow-y-auto">
            {#each ruleResults as res}
              <div class="p-2 bg-red-50 rounded text-sm border-l-4 {res.severity==='error'?'border-red-500':'border-yellow-500'}">
                <div class="flex items-center gap-2">
                  <span class="px-1 rounded text-xs {res.severity==='error'?'bg-red-100 text-red-700':'bg-yellow-100 text-yellow-700'}">{res.severity.toUpperCase()}</span>
                  <span class="font-mono text-xs text-gray-500">{res.file_name}:{res.line}</span>
                </div>
                <div class="mt-1">{res.message}</div>
                <div class="text-xs text-gray-400 font-mono mt-1">`{res.matched_code}`</div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  {/if}
</div>

<style>
  .phase27-tools { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

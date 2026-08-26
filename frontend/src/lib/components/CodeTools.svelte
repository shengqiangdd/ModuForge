<script>
  let activeTab = $state('diff');
  
  // Diff state
  let oldCode = $state('');
  let newCode = $state('');
  let diffResult = $state(null);
  
  // Security scan state
  let scanCode = $state('');
  let scanLanguage = $state('go');
  let scanResult = $state(null);
  
  // Performance state
  let profileResult = $state(null);
  
  let loading = $state(false);
  let error = $state(null);
  
  async function runDiff() {
    loading = true; error = null; diffResult = null;
    try {
      const res = await fetch('/api/code/diff', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ old_code: oldCode, new_code: newCode })
      });
      if (res.ok) {
        const data = await res.json();
        diffResult = data.result;
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function runSecurityScan() {
    loading = true; error = null; scanResult = null;
    try {
      const res = await fetch('/api/code/security-scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: scanCode, language: scanLanguage })
      });
      if (res.ok) {
        const data = await res.json();
        scanResult = data.result;
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function runProfile() {
    loading = true; error = null; profileResult = null;
    try {
      const res = await fetch('/api/code/runtime-profile');
      if (res.ok) {
        const data = await res.json();
        profileResult = data.result;
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  function getSeverityColor(sev) {
    return { critical:'bg-red-100 text-red-700', high:'bg-orange-100 text-orange-700', medium:'bg-yellow-100 text-yellow-700', low:'bg-blue-100 text-blue-700' }[sev] || 'bg-gray-100';
  }
  
  function getRiskColor(level) {
    return { critical:'text-red-500', high:'text-orange-500', medium:'text-yellow-500', low:'text-green-500' }[level] || 'text-gray-500';
  }
</script>

<div class="code-tools">
  <h2 class="text-xl font-semibold mb-4">🛠️ 代码工具箱</h2>
  
  <div class="flex gap-2 mb-4">
    <button onclick={() => activeTab='diff'} class="px-4 py-2 rounded {activeTab==='diff'?'bg-blue-500 text-white':'bg-gray-100'}">📝 版本对比</button>
    <button onclick={() => activeTab='security'} class="px-4 py-2 rounded {activeTab==='security'?'bg-red-500 text-white':'bg-gray-100'}">🔒 安全扫描</button>
    <button onclick={() => activeTab='profile'} class="px-4 py-2 rounded {activeTab==='profile'?'bg-green-500 text-white':'bg-gray-100'}">⚡ 性能分析</button>
  </div>
  
  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}
  
  <!-- Diff Tab -->
  {#if activeTab === 'diff'}
    <div class="card mb-4">
      <div class="grid grid-cols-2 gap-4 mb-3">
        <div>
          <label class="text-sm text-gray-500 mb-1 block">旧版本</label>
          <textarea bind:value={oldCode} rows="8" class="w-full px-3 py-2 border rounded font-mono text-sm" placeholder="粘贴旧代码..."></textarea>
        </div>
        <div>
          <label class="text-sm text-gray-500 mb-1 block">新版本</label>
          <textarea bind:value={newCode} rows="8" class="w-full px-3 py-2 border rounded font-mono text-sm" placeholder="粘贴新代码..."></textarea>
        </div>
      </div>
      <button onclick={runDiff} disabled={loading} class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50">
        {loading ? '对比中...' : '开始对比'}
      </button>
    </div>
    
    {#if diffResult}
      <div class="card mb-4">
        <div class="grid grid-cols-3 gap-4 text-center">
          <div><div class="text-2xl font-bold text-green-500">+{diffResult.lines_added}</div><div class="text-xs text-gray-500">新增行</div></div>
          <div><div class="text-2xl font-bold text-red-500">-{diffResult.lines_removed}</div><div class="text-xs text-gray-500">删除行</div></div>
          <div><div class="text-2xl font-bold">{diffResult.changes?.length || 0}</div><div class="text-xs text-gray-500">变更数</div></div>
        </div>
        <div class="text-sm text-gray-500 text-center mt-3">{diffResult.summary}</div>
      </div>
      
      {#if diffResult.changes?.length > 0}
        <div class="card">
          <h3 class="card-title">变更详情</h3>
          <div class="space-y-2 max-h-80 overflow-y-auto font-mono text-sm">
            {#each diffResult.changes as change}
              <div class="p-2 rounded {change.type==='added'?'bg-green-50':change.type==='removed'?'bg-red-50':'bg-yellow-50'}">
                <span class="text-xs text-gray-400">行 {change.line}</span>
                <span class="text-xs ml-2 {change.type==='added'?'text-green-600':change.type==='removed'?'text-red-600':'text-yellow-600'}">
                  {change.type === 'added' ? '+ ' : change.type === 'removed' ? '- ' : '~ '}
                </span>
                <pre class="whitespace-pre-wrap">{change.content}</pre>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/if}
  {/if}
  
  <!-- Security Tab -->
  {#if activeTab === 'security'}
    <div class="card mb-4">
      <div class="flex gap-4 mb-3">
        <div>
          <label class="text-sm text-gray-500 mb-1 block">语言</label>
          <select bind:value={scanLanguage} class="px-3 py-2 border rounded">
            <option value="go">Go</option>
            <option value="javascript">JavaScript</option>
            <option value="python">Python</option>
          </select>
        </div>
      </div>
      <div class="mb-3">
        <label class="text-sm text-gray-500 mb-1 block">代码</label>
        <textarea bind:value={scanCode} rows="8" class="w-full px-3 py-2 border rounded font-mono text-sm" placeholder="粘贴代码进行安全扫描..."></textarea>
      </div>
      <button onclick={runSecurityScan} disabled={loading || !scanCode.trim()} class="px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600 disabled:opacity-50">
        {loading ? '扫描中...' : '开始扫描'}
      </button>
    </div>
    
    {#if scanResult}
      <div class="card mb-4">
        <div class="grid grid-cols-2 gap-4">
          <div class="text-center">
            <div class="text-4xl font-bold {getRiskColor(scanResult.risk_level)}">{scanResult.score}</div>
            <div class="text-xs text-gray-500">安全评分 /100</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold {getRiskColor(scanResult.risk_level)}">{scanResult.risk_level}</div>
            <div class="text-xs text-gray-500">风险等级</div>
          </div>
        </div>
        <div class="grid grid-cols-4 gap-2 mt-4 text-center text-sm">
          <div><div class="font-bold text-red-500">{scanResult.stats?.critical_count || 0}</div><div class="text-xs text-gray-500">严重</div></div>
          <div><div class="font-bold text-orange-500">{scanResult.stats?.high_count || 0}</div><div class="text-xs text-gray-500">高</div></div>
          <div><div class="font-bold text-yellow-500">{scanResult.stats?.medium_count || 0}</div><div class="text-xs text-gray-500">中</div></div>
          <div><div class="font-bold text-blue-500">{scanResult.stats?.low_count || 0}</div><div class="text-xs text-gray-500">低</div></div>
        </div>
      </div>
      
      {#if scanResult.vulnerabilities?.length > 0}
        <div class="card">
          <h3 class="card-title">漏洞详情 ({scanResult.vulnerabilities.length})</h3>
          <div class="space-y-3 max-h-96 overflow-y-auto">
            {#each scanResult.vulnerabilities as vuln}
              <div class="p-3 bg-gray-50 rounded">
                <div class="flex items-center gap-2 mb-1">
                  <span class="text-xs px-2 py-0.5 rounded {getSeverityColor(vuln.severity)}">{vuln.severity}</span>
                  <span class="font-medium">{vuln.title}</span>
                  {#if vuln.cwe}
                    <span class="text-xs px-2 py-0.5 bg-gray-200 rounded">{vuln.cwe}</span>
                  {/if}
                </div>
                <div class="text-sm text-gray-600">{vuln.description}</div>
                {#if vuln.suggestion}
                  <div class="text-sm text-green-600 mt-1">💡 {vuln.suggestion}</div>
                {/if}
                {#if vuln.location}
                  <div class="text-xs text-gray-400 mt-1">位置: {vuln.location}</div>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {:else}
        <div class="card text-center py-6 text-green-500">✅ 未发现安全漏洞</div>
      {/if}
    {/if}
  {/if}
  
  <!-- Profile Tab -->
  {#if activeTab === 'profile'}
    <div class="card mb-4">
      <p class="text-sm text-gray-500 mb-3">采集运行时性能快照（内存、GC、Goroutine）</p>
      <button onclick={runProfile} disabled={loading} class="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600 disabled:opacity-50">
        {loading ? '采集中...' : '采集快照'}
      </button>
    </div>
    
    {#if profileResult}
      <div class="card mb-4">
        <h3 class="card-title">性能摘要</h3>
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          <div><div class="text-gray-500">平均内存</div><div class="font-semibold">{profileResult.summary?.avg_memory_mb?.toFixed(1)} MB</div></div>
          <div><div class="text-gray-500">最大内存</div><div class="font-semibold">{profileResult.summary?.max_memory_mb?.toFixed(1)} MB</div></div>
          <div><div class="text-gray-500">平均Goroutine</div><div class="font-semibold">{profileResult.summary?.avg_goroutines?.toFixed(0)}</div></div>
          <div><div class="text-gray-500">GC次数</div><div class="font-semibold">{profileResult.summary?.gc_total_count}</div></div>
        </div>
      </div>
      
      {#if profileResult.snapshots?.length > 0}
        <div class="card">
          <h3 class="card-title">快照列表 ({profileResult.snapshots.length})</h3>
          <div class="space-y-2 max-h-64 overflow-y-auto">
            {#each profileResult.snapshots as snap, i}
              <div class="p-2 bg-gray-50 rounded text-sm flex items-center justify-between">
                <span class="text-gray-400">#{i+1}</span>
                <span>内存: {(snap.memory.alloc/1024/1024).toFixed(1)} MB</span>
                <span>Goroutine: {snap.goroutines}</span>
                <span>GC: {snap.gc.num_gc}</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/if}
  {/if}
</div>

<style>
  .code-tools { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

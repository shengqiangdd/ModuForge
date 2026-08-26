<script>
  let activeTab = $state('collab');
  
  // Collaboration state
  let sessions = $state([]);
  let currentSession = $state(null);
  let newFileName = $state('');
  let sessionId = $state('');
  let userId = $state('user-' + Math.random().toString(36).substr(2, 9));
  let username = $state('');
  
  // API Doc state
  let apiCode = $state('');
  let apiLanguage = $state('go');
  let apiTitle = $state('');
  let apiVersion = $state('1.0.0');
  let apiDocResult = $state(null);
  
  // Duplication state
  let dupFiles = $state([{name: 'file1.go', code: ''}, {name: 'file2.go', code: ''}]);
  let dupResult = $state(null);
  
  let loading = $state(false);
  let error = $state(null);
  
  async function createSession() {
    if (!newFileName.trim()) return;
    loading = true; error = null;
    try {
      const res = await fetch('/api/code/collaboration/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ file_name: newFileName })
      });
      if (res.ok) {
        const data = await res.json();
        currentSession = data.session;
        sessionId = data.session.id;
        await loadSessions();
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function joinSession() {
    if (!sessionId.trim() || !username.trim()) return;
    loading = true; error = null;
    try {
      const res = await fetch('/api/code/collaboration/join', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_id: sessionId, user_id: userId, username })
      });
      if (res.ok) {
        await loadSessions();
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function loadSessions() {
    try {
      const res = await fetch('/api/code/collaboration/sessions');
      if (res.ok) {
        const data = await res.json();
        sessions = data.sessions || [];
      }
    } catch (e) { console.error(e); }
  }
  
  async function generateAPIDoc() {
    if (!apiCode.trim()) return;
    loading = true; error = null; apiDocResult = null;
    try {
      const res = await fetch('/api/code/api-docs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: apiCode, language: apiLanguage, title: apiTitle, version: apiVersion })
      });
      if (res.ok) {
        const data = await res.json();
        apiDocResult = data.result;
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function detectDuplication() {
    const files = {};
    dupFiles.forEach(f => { if (f.code.trim()) files[f.name] = f.code; });
    if (Object.keys(files).length === 0) return;
    
    loading = true; error = null; dupResult = null;
    try {
      const res = await fetch('/api/code/duplication', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files })
      });
      if (res.ok) {
        const data = await res.json();
        dupResult = data.result;
      }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  function addDupFile() {
    dupFiles = [...dupFiles, { name: `file${dupFiles.length+1}.go`, code: '' }];
  }
  
  function removeDupFile(index) {
    dupFiles = dupFiles.filter((_, i) => i !== index);
  }
  
  function getScoreColor(score) {
    if (score < 10) return 'text-green-500';
    if (score < 30) return 'text-yellow-500';
    return 'text-red-500';
  }
  
  // 初始加载会话
  loadSessions();
</script>

<div class="phase26-tools">
  <h2 class="text-xl font-semibold mb-4">🔄 Phase 26 - 协作/文档/重复检测</h2>
  
  <div class="flex gap-2 mb-4 flex-wrap">
    <button onclick={() => activeTab='collab'} class="px-4 py-2 rounded {activeTab==='collab'?'bg-blue-500 text-white':'bg-gray-100'}">👥 实时协作</button>
    <button onclick={() => activeTab='apidoc'} class="px-4 py-2 rounded {activeTab==='apidoc'?'bg-purple-500 text-white':'bg-gray-100'}">📄 API文档</button>
    <button onclick={() => activeTab='dup'} class="px-4 py-2 rounded {activeTab==='dup'?'bg-orange-500 text-white':'bg-gray-100'}">🔍 重复检测</button>
  </div>
  
  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}
  
  <!-- Collaboration Tab -->
  {#if activeTab === 'collab'}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
      <div class="card">
        <h3 class="card-title">创建会话</h3>
        <input bind:value={newFileName} placeholder="文件名" class="w-full px-3 py-2 border rounded mb-2" />
        <input bind:value={username} placeholder="你的用户名" class="w-full px-3 py-2 border rounded mb-2" />
        <button onclick={createSession} disabled={loading || !newFileName.trim() || !username.trim()} class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50">
          创建新会话
        </button>
      </div>
      
      <div class="card">
        <h3 class="card-title">加入会话</h3>
        <input bind:value={sessionId} placeholder="会话ID" class="w-full px-3 py-2 border rounded mb-2" />
        <input bind:value={username} placeholder="你的用户名" class="w-full px-3 py-2 border rounded mb-2" />
        <button onclick={joinSession} disabled={loading || !sessionId.trim() || !username.trim()} class="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600 disabled:opacity-50">
          加入会话
        </button>
      </div>
    </div>
    
    {#if currentSession}
      <div class="card mb-4">
        <h3 class="card-title">当前会话</h3>
        <div class="text-sm">
          <div><span class="text-gray-500">ID:</span> {currentSession.id}</div>
          <div><span class="text-gray-500">文件:</span> {currentSession.file_name}</div>
          <div><span class="text-gray-500">参与者:</span> {currentSession.participants?.length || 0}</div>
        </div>
      </div>
    {/if}
    
    {#if sessions.length > 0}
      <div class="card">
        <h3 class="card-title">活跃会话 ({sessions.length})</h3>
        <div class="space-y-2 max-h-64 overflow-y-auto">
          {#each sessions as session}
            <div class="p-2 bg-gray-50 rounded text-sm flex items-center justify-between">
              <div>
                <span class="font-mono text-xs text-gray-400">{session.id?.slice(0, 12)}...</span>
                <span class="ml-2">{session.file_name}</span>
              </div>
              <span class="text-xs text-gray-500">{session.participants?.filter(p => p.is_active)?.length || 0} 人</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
  
  <!-- API Doc Tab -->
  {#if activeTab === 'apidoc'}
    <div class="card mb-4">
      <div class="grid grid-cols-3 gap-4 mb-3">
        <input bind:value={apiTitle} placeholder="API标题" class="px-3 py-2 border rounded" />
        <input bind:value={apiVersion} placeholder="版本号" class="px-3 py-2 border rounded" />
        <select bind:value={apiLanguage} class="px-3 py-2 border rounded">
          <option value="go">Go</option>
          <option value="javascript">JavaScript</option>
          <option value="python">Python</option>
        </select>
      </div>
      <textarea bind:value={apiCode} rows="10" class="w-full px-3 py-2 border rounded font-mono text-sm mb-3" placeholder="粘贴代码生成API文档..."></textarea>
      <button onclick={generateAPIDoc} disabled={loading || !apiCode.trim()} class="px-4 py-2 bg-purple-500 text-white rounded hover:bg-purple-600 disabled:opacity-50">
        {loading ? '生成中...' : '生成文档'}
      </button>
    </div>
    
    {#if apiDocResult}
      <div class="card mb-4">
        <h3 class="card-title">{apiDocResult.title || 'API文档'} v{apiDocResult.version}</h3>
        <p class="text-sm text-gray-500 mb-4">{apiDocResult.description}</p>
        
        {#if apiDocResult.endpoints?.length > 0}
          <div class="mb-4">
            <h4 class="font-medium mb-2">端点 ({apiDocResult.endpoints.length})</h4>
            <div class="space-y-2">
              {#each apiDocResult.endpoints as endpoint}
                <div class="p-2 bg-gray-50 rounded text-sm">
                  <span class="px-2 py-0.5 rounded text-xs font-bold {endpoint.method==='GET'?'bg-green-100 text-green-700':endpoint.method==='POST'?'bg-blue-100 text-blue-700':endpoint.method==='PUT'?'bg-yellow-100 text-yellow-700':'bg-red-100 text-red-700'}">{endpoint.method}</span>
                  <span class="font-mono ml-2">{endpoint.path}</span>
                  {#if endpoint.description}
                    <span class="text-gray-500 ml-2">- {endpoint.description}</span>
                  {/if}
                </div>
              {/each}
            </div>
          </div>
        {/if}
        
        {#if apiDocResult.schemas?.length > 0}
          <div>
            <h4 class="font-medium mb-2">数据模型 ({apiDocResult.schemas.length})</h4>
            <div class="space-y-2">
              {#each apiDocResult.schemas as schema}
                <div class="p-2 bg-gray-50 rounded text-sm">
                  <span class="font-mono font-bold">{schema.name}</span>
                  <span class="text-gray-500 ml-2">({schema.type})</span>
                  {#if schema.properties}
                    <div class="ml-4 mt-1 text-xs text-gray-600">
                      {#each Object.entries(schema.properties) as [key, value]}
                        <div>{key}: {value}</div>
                      {/each}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    {/if}
  {/if}
  
  <!-- Duplication Tab -->
  {#if activeTab === 'dup'}
    <div class="card mb-4">
      <div class="flex items-center justify-between mb-3">
        <h3 class="card-title">文件列表</h3>
        <button onclick={addDupFile} class="px-3 py-1 bg-gray-200 rounded text-sm hover:bg-gray-300">+ 添加文件</button>
      </div>
      
      <div class="space-y-3 max-h-96 overflow-y-auto">
        {#each dupFiles as file, i}
          <div class="p-3 bg-gray-50 rounded">
            <div class="flex items-center gap-2 mb-2">
              <input bind:value={file.name} class="px-2 py-1 border rounded text-sm w-40" placeholder="文件名" />
              <button onclick={() => removeDupFile(i)} class="text-red-500 hover:text-red-700 text-sm">删除</button>
            </div>
            <textarea bind:value={file.code} rows="4" class="w-full px-3 py-2 border rounded font-mono text-sm" placeholder="粘贴代码..."></textarea>
          </div>
        {/each}
      </div>
      
      <button onclick={detectDuplication} disabled={loading} class="mt-3 px-4 py-2 bg-orange-500 text-white rounded hover:bg-orange-600 disabled:opacity-50">
        {loading ? '检测中...' : '检测重复'}
      </button>
    </div>
    
    {#if dupResult}
      <div class="card mb-4">
        <div class="grid grid-cols-3 gap-4 text-center">
          <div>
            <div class="text-2xl font-bold">{dupResult.total_lines}</div>
            <div class="text-xs text-gray-500">总行数</div>
          </div>
          <div>
            <div class="text-2xl font-bold text-orange-500">{dupResult.duplicate_lines}</div>
            <div class="text-xs text-gray-500">重复行数</div>
          </div>
          <div>
            <div class="text-2xl font-bold {getScoreColor(dupResult.score)}">{dupResult.score?.toFixed(1)}%</div>
            <div class="text-xs text-gray-500">重复率</div>
          </div>
        </div>
        <div class="text-sm text-gray-500 text-center mt-3">{dupResult.summary}</div>
      </div>
      
      {#if dupResult.duplicates?.length > 0}
        <div class="card">
          <h3 class="card-title">重复详情 ({dupResult.duplicates.length})</h3>
          <div class="space-y-3 max-h-80 overflow-y-auto">
            {#each dupResult.duplicates as dup}
              <div class="p-3 bg-gray-50 rounded">
                <div class="flex items-center gap-2 mb-2">
                  <span class="px-2 py-0.5 bg-orange-100 text-orange-700 rounded text-xs">#{dup.id}</span>
                  <span class="font-medium">{dup.lines} 行重复</span>
                  <span class="text-xs text-gray-500">相似度: {dup.similarity}%</span>
                </div>
                {#each dup.locations as loc}
                  <div class="text-xs text-gray-600 ml-4">
                    <span class="font-mono">{loc.file}</span>
                    <span class="ml-2">行 {loc.start_line}-{loc.end_line}</span>
                  </div>
                {/each}
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/if}
  {/if}
</div>

<style>
  .phase26-tools { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

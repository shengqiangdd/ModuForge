<script>
  let code = $state('');
  let language = $state('go');
  let result = $state(null);
  let loading = $state(false);
  let error = $state(null);

  async function scan() {
    if (!code.trim()) return;

    loading = true;
    error = null;
    result = null;

    try {
      const res = await fetch('/api/code/security-scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code, language })
      });

      if (res.ok) {
        const data = await res.json();
        if (data.valid) {
          result = data.result;
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

  function getSeverityColor(severity) {
    const colors = {
      critical: 'bg-red-100 text-red-700 border-red-300',
      high: 'bg-orange-100 text-orange-700 border-orange-300',
      medium: 'bg-yellow-100 text-yellow-700 border-yellow-300',
      low: 'bg-blue-100 text-blue-700 border-blue-300'
    };
    return colors[severity] || 'bg-gray-100 text-gray-700';
  }

  function getRiskColor(riskLevel) {
    const colors = {
      '低风险': 'text-green-500',
      '中风险': 'text-yellow-500',
      '高风险': 'text-orange-500',
      '严重风险': 'text-red-500'
    };
    return colors[riskLevel] || 'text-gray-500';
  }

  function getCategoryIcon(category) {
    const icons = {
      injection: '💉',
      xss: '🌐',
      secrets: '🔑',
      path_traversal: '📁',
      command_injection: '⚡',
      deserialization: '📦',
      weak_crypto: '🔓',
      memory_safety: '💾'
    };
    return icons[category] || '⚠️';
  }
</script>

<div class="security-scanner">
  <h2 class="text-xl font-semibold mb-4">🛡️ 安全漏洞扫描</h2>

  <div class="card mb-4">
    <div class="flex gap-4 mb-3">
      <div>
        <label class="text-sm text-gray-500 mb-1 block">语言</label>
        <select bind:value={language} class="px-3 py-2 border rounded">
          <option value="go">Go</option>
          <option value="javascript">JavaScript</option>
          <option value="python">Python</option>
          <option value="c">C</option>
          <option value="cpp">C++</option>
        </select>
      </div>
    </div>

    <div class="mb-3">
      <label class="text-sm text-gray-500 mb-1 block">代码</label>
      <textarea
        bind:value={code}
        placeholder="粘贴代码进行安全扫描..."
        rows="10"
        class="w-full px-3 py-2 border rounded font-mono text-sm"
      ></textarea>
    </div>

    <button
      onclick={scan}
      disabled={loading || !code.trim()}
      class="px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600 disabled:opacity-50"
    >
      {loading ? '扫描中...' : '开始扫描'}
    </button>
  </div>

  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}

  {#if result}
    <!-- 扫描摘要 -->
    <div class="card mb-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="card-title">扫描结果</h3>
          <p class="text-sm text-gray-500">{result.summary}</p>
        </div>
        <div class="text-center">
          <div class="text-4xl font-bold {getRiskColor(result.risk_level)}">
            {result.score}
          </div>
          <div class="text-xs text-gray-500">安全评分</div>
          <div class="text-sm font-semibold {getRiskColor(result.risk_level)}">
            {result.risk_level}
          </div>
        </div>
      </div>

      <!-- 统计 -->
      <div class="grid grid-cols-4 gap-4 mt-4 text-center">
        <div>
          <div class="text-lg font-bold text-red-500">{result.stats.critical_count}</div>
          <div class="text-xs text-gray-500">严重</div>
        </div>
        <div>
          <div class="text-lg font-bold text-orange-500">{result.stats.high_count}</div>
          <div class="text-xs text-gray-500">高</div>
        </div>
        <div>
          <div class="text-lg font-bold text-yellow-500">{result.stats.medium_count}</div>
          <div class="text-xs text-gray-500">中</div>
        </div>
        <div>
          <div class="text-lg font-bold text-blue-500">{result.stats.low_count}</div>
          <div class="text-xs text-gray-500">低</div>
        </div>
      </div>
    </div>

    <!-- 漏洞列表 -->
    {#if result.vulnerabilities?.length > 0}
      <div class="card">
        <h3 class="card-title">发现的漏洞 ({result.vulnerabilities.length})</h3>
        <div class="space-y-3 max-h-96 overflow-y-auto">
          {#each result.vulnerabilities as vuln}
            <div class="p-3 bg-gray-50 rounded border-l-4 {getSeverityColor(vuln.severity)}">
              <div class="flex items-center gap-2 mb-2">
                <span>{getCategoryIcon(vuln.category)}</span>
                <span class="text-xs px-2 py-0.5 rounded {getSeverityColor(vuln.severity)}">
                  {vuln.severity}
                </span>
                <span class="text-xs px-2 py-0.5 bg-gray-200 rounded">
                  {vuln.cwe}
                </span>
              </div>
              <div class="font-medium text-sm">{vuln.title}</div>
              <div class="text-xs text-gray-500 mt-1">{vuln.description}</div>
              <div class="text-xs text-gray-400 mt-1">位置: {vuln.location}</div>
              <div class="mt-2 text-xs">
                <span class="text-gray-500">建议：</span>
                <span class="text-green-600">{vuln.suggestion}</span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {:else}
      <div class="card text-center py-8 text-gray-500">
        ✅ 未发现安全漏洞
      </div>
    {/if}
  {/if}
</div>

<script>
  let auditResult = $state(null);
  let loading = $state(false);
  let error = $state(null);
  let code = $state('');
  let language = $state('go');

  async function runAudit() {
    if (!code.trim()) return;

    loading = true;
    error = null;
    try {
      const res = await fetch('/api/quality/validate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code, language })
      });

      if (res.ok) {
        auditResult = await res.json();
      } else {
        error = await res.text();
      }
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }

  function getScoreColor(score) {
    if (score >= 80) return 'text-green-500';
    if (score >= 60) return 'text-yellow-500';
    return 'text-red-500';
  }

  function getSeverityColor(severity) {
    const colors = {
      critical: 'bg-red-100 text-red-700',
      high: 'bg-orange-100 text-orange-700',
      medium: 'bg-yellow-100 text-yellow-700',
      low: 'bg-gray-100 text-gray-700'
    };
    return colors[severity] || 'bg-gray-100';
  }
</script>

<div class="security-audit">
  <h2 class="text-xl font-semibold mb-4">🔒 安全审计</h2>

  <div class="card mb-4">
    <h3 class="card-title">代码安全检查</h3>

    <div class="mb-3">
      <label class="text-sm text-gray-500 mb-1 block">语言</label>
      <select bind:value={language} class="px-3 py-2 border rounded">
        <option value="go">Go</option>
        <option value="javascript">JavaScript</option>
        <option value="python">Python</option>
      </select>
    </div>

    <div class="mb-3">
      <label class="text-sm text-gray-500 mb-1 block">代码</label>
      <textarea
        bind:value={code}
        placeholder="粘贴代码进行安全检查..."
        rows="8"
        class="w-full px-3 py-2 border rounded font-mono text-sm"
      ></textarea>
    </div>

    <button
      onclick={runAudit}
      disabled={loading || !code.trim()}
      class="px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600 disabled:opacity-50"
    >
      {loading ? '检查中...' : '运行安全检查'}
    </button>
  </div>

  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}

  {#if auditResult}
    <!-- Score -->
    <div class="card mb-4">
      <div class="flex items-center gap-4">
        <div class="text-center">
          <div class="text-4xl font-bold {getScoreColor(auditResult.score)}">
            {auditResult.score.toFixed(0)}
          </div>
          <div class="text-sm text-gray-500">安全评分</div>
        </div>
        <div class="flex-1">
          <div class="text-sm text-gray-500">安全分数</div>
          <div class="w-full bg-gray-200 rounded-full h-3">
            <div
              class="h-3 rounded-full {auditResult.score >= 80 ? 'bg-green-500' : auditResult.score >= 60 ? 'bg-yellow-500' : 'bg-red-500'}"
              style="width: {auditResult.score}%"
            ></div>
          </div>
        </div>
      </div>
    </div>

    <!-- Metrics -->
    <div class="card mb-4">
      <h3 class="card-title">质量指标</h3>
      <div class="grid grid-cols-2 gap-4 text-sm">
        <div>
          <div class="text-gray-500">代码行数</div>
          <div class="font-semibold">{auditResult.metrics.lines_of_code}</div>
        </div>
        <div>
          <div class="text-gray-500">圈复杂度</div>
          <div class="font-semibold">{auditResult.metrics.complexity.toFixed(1)}</div>
        </div>
        <div>
          <div class="text-gray-500">可维护性</div>
          <div class="font-semibold">{auditResult.metrics.maintainability.toFixed(1)}%</div>
        </div>
        <div>
          <div class="text-gray-500">安全分数</div>
          <div class="font-semibold {getScoreColor(auditResult.metrics.security_score)}">
            {auditResult.metrics.security_score.toFixed(0)}%
          </div>
        </div>
      </div>
    </div>

    <!-- Issues -->
    {#if auditResult.issues?.length > 0}
      <div class="card mb-4">
        <h3 class="card-title">⚠️ 发现的问题 ({auditResult.issues.length})</h3>
        <div class="space-y-2 max-h-64 overflow-y-auto">
          {#each auditResult.issues as issue}
            <div class="flex items-start gap-2 p-2 bg-gray-50 rounded">
              <span class="text-xs px-2 py-0.5 rounded {getSeverityColor(issue.severity)}">
                {issue.severity}
              </span>
              <div class="flex-1">
                <div class="text-sm">{issue.message}</div>
                <div class="text-xs text-gray-500 font-mono">{issue.rule}</div>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Suggestions -->
    {#if auditResult.suggestions?.length > 0}
      <div class="card">
        <h3 class="card-title">💡 改进建议</h3>
        <ul class="space-y-2">
          {#each auditResult.suggestions as suggestion}
            <li class="flex items-start gap-2">
              <span class="text-blue-500">•</span>
              <span class="text-sm">{suggestion}</span>
            </li>
          {/each}
        </ul>
      </div>
    {/if}
  {/if}
</div>

<style>
  .security-audit { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

<script>
  let code = $state('');
  let language = $state('go');
  let result = $state(null);
  let loading = $state(false);
  let error = $state(null);

  async function review() {
    if (!code.trim()) return;

    loading = true;
    error = null;
    result = null;

    try {
      const res = await fetch('/api/code/review', {
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
      critical: 'bg-red-100 text-red-700',
      high: 'bg-orange-100 text-orange-700',
      medium: 'bg-yellow-100 text-yellow-700',
      low: 'bg-blue-100 text-blue-700'
    };
    return colors[severity] || 'bg-gray-100';
  }

  function getCategoryIcon(category) {
    const icons = {
      security: '\u{1F512}',
      performance: '\u26A1',
      style: '\u{1F3A8}',
      bug: '\u{1F41B}',
      best_practice: '\u2728'
    };
    return icons[category] || '\u{1F4DD}';
  }

  function getScoreColor(score) {
    if (score >= 80) return 'text-green-500';
    if (score >= 60) return 'text-yellow-500';
    return 'text-red-500';
  }
</script>

<div class="code-review">
  <h2 class="text-xl font-semibold mb-4">Code Review</h2>

  <div class="card mb-4">
    <div class="flex gap-4 mb-3">
      <div>
        <label class="text-sm text-gray-500 mb-1 block">Language</label>
        <select bind:value={language} class="px-3 py-2 border rounded">
          <option value="go">Go</option>
          <option value="javascript">JavaScript</option>
          <option value="typescript">TypeScript</option>
          <option value="python">Python</option>
        </select>
      </div>
    </div>

    <div class="mb-3">
      <label class="text-sm text-gray-500 mb-1 block">Code</label>
      <textarea
        bind:value={code}
        placeholder="Paste code to review..."
        rows="10"
        class="w-full px-3 py-2 border rounded font-mono text-sm"
      ></textarea>
    </div>

    <button
      onclick={review}
      disabled={loading || !code.trim()}
      class="px-4 py-2 bg-indigo-500 text-white rounded hover:bg-indigo-600 disabled:opacity-50"
    >
      {loading ? 'Reviewing...' : 'Start Review'}
    </button>
  </div>

  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}

  {#if result?.valid && result.result}
    <div class="card mb-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="card-title">Review Result</h3>
          <p class="text-sm text-gray-500">{result.result.summary}</p>
        </div>
        <div class="text-center">
          <div class="text-4xl font-bold {getScoreColor(result.result.score)}">
            {result.result.score}
          </div>
          <div class="text-xs text-gray-500">/100</div>
        </div>
      </div>

      <div class="grid grid-cols-4 gap-4 mt-4 text-center">
        <div>
          <div class="text-lg font-bold text-red-500">{result.result.stats.critical_issues}</div>
          <div class="text-xs text-gray-500">Critical</div>
        </div>
        <div>
          <div class="text-lg font-bold text-orange-500">{result.result.stats.high_issues}</div>
          <div class="text-xs text-gray-500">High</div>
        </div>
        <div>
          <div class="text-lg font-bold text-yellow-500">{result.result.stats.medium_issues}</div>
          <div class="text-xs text-gray-500">Medium</div>
        </div>
        <div>
          <div class="text-lg font-bold text-blue-500">{result.result.stats.low_issues}</div>
          <div class="text-xs text-gray-500">Low</div>
        </div>
      </div>
    </div>

    {#if result.result.issues?.length > 0}
      <div class="card">
        <h3 class="card-title">Issues Found ({result.result.issues.length})</h3>
        <div class="space-y-3 max-h-96 overflow-y-auto">
          {#each result.result.issues as issue}
            <div class="p-3 bg-gray-50 rounded">
              <div class="flex items-center gap-2 mb-2">
                <span>{getCategoryIcon(issue.category)}</span>
                <span class="text-xs px-2 py-0.5 rounded {getSeverityColor(issue.severity)}">
                  {issue.severity}
                </span>
                <span class="text-xs px-2 py-0.5 bg-gray-200 rounded">
                  {issue.category}
                </span>
              </div>
              <div class="font-medium">{issue.title}</div>
              <div class="text-sm text-gray-600 mt-1">{issue.description}</div>
              {#if issue.suggestion}
                <div class="text-sm text-green-600 mt-2">
                  {issue.suggestion}
                </div>
              {/if}
              {#if issue.location}
                <div class="text-xs text-gray-400 mt-1">Location: {issue.location}</div>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {:else}
      <div class="card text-center py-8 text-gray-500">
        Code review passed, no issues found
      </div>
    {/if}
  {:else if result && !result.valid}
    <div class="card text-red-500">
      <div class="font-medium">Review Error</div>
      <div class="text-sm">{result.error}</div>
    </div>
  {/if}
</div>

<style>
  .code-review { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

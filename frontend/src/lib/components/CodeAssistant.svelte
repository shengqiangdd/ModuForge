<script>
  let code = $state('');
  let language = $state('go');
  let completions = $state([]);
  let loading = $state(false);
  let error = $state(null);
  let cursorPos = $state({ line: 0, col: 0 });

  async function getCompletions() {
    if (!code.trim()) return;

    loading = true;
    error = null;

    try {
      const res = await fetch('/api/code/completions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          code,
          language,
          cursor_line: cursorPos.line,
          cursor_col: cursorPos.col,
          context: 'function'
        })
      });

      if (res.ok) {
        const data = await res.json();
        completions = data.completions || [];
      } else {
        error = await res.text();
      }
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }

  function insertCompletion(completion) {
    const lines = code.split('\n');
    const line = lines[cursorPos.line] || '';
    const before = line.slice(0, cursorPos.col);
    const after = line.slice(cursorPos.col);

    lines[cursorPos.line] = before + completion.insert_text + after;
    code = lines.join('\n');

    completions = [];
  }

  function getKindIcon(kind) {
    const icons = {
      function: 'ƒ',
      keyword: '🔑',
      snippet: '📝',
      variable: '📦',
      type: '🏷️'
    };
    return icons[kind] || '•';
  }

  function getKindColor(kind) {
    const colors = {
      function: 'text-blue-500',
      keyword: 'text-purple-500',
      snippet: 'text-green-500',
      variable: 'text-orange-500',
      type: 'text-cyan-500'
    };
    return colors[kind] || 'text-gray-500';
  }
</script>

<div class="code-assistant">
  <h2 class="text-xl font-semibold mb-4">🤖 AI代码助手</h2>

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
        placeholder="输入代码获取智能补全..."
        rows="12"
        class="w-full px-3 py-2 border rounded font-mono text-sm"
      ></textarea>
    </div>

    <button
      onclick={getCompletions}
      disabled={loading || !code.trim()}
      class="px-4 py-2 bg-violet-500 text-white rounded hover:bg-violet-600 disabled:opacity-50"
    >
      {loading ? '分析中...' : '获取补全建议'}
    </button>
  </div>

  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}

  {#if completions.length > 0}
    <div class="card">
      <h3 class="card-title">💡 补全建议 ({completions.length})</h3>
      <div class="space-y-2 max-h-96 overflow-y-auto">
        {#each completions as completion}
          <button
            onclick={() => insertCompletion(completion)}
            class="w-full text-left p-3 bg-gray-50 rounded hover:bg-gray-100 transition-colors"
          >
            <div class="flex items-center gap-2 mb-1">
              <span class="{getKindColor(completion.kind)}">{getKindIcon(completion.kind)}</span>
              <span class="font-mono font-semibold">{completion.label}</span>
              <span class="text-xs px-2 py-0.5 bg-gray-200 rounded">{completion.kind}</span>
            </div>
            <div class="text-sm text-gray-500">{completion.detail}</div>
            {#if completion.description}
              <div class="text-xs text-gray-400 mt-1">{completion.description}</div>
            {/if}
            <pre class="mt-2 p-2 bg-white rounded text-xs font-mono overflow-x-auto border">{completion.insert_text}</pre>
          </button>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .code-assistant { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

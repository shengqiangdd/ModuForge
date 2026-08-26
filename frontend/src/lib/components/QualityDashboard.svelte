<script>
  let files = $state({});
  let language = $state('go');
  let report = $state(null);
  let loading = $state(false);
  let error = $state(null);
  let fileInput = $state('');

  async function analyze() {
    if (Object.keys(files).length === 0) {
      error = '请先添加文件';
      return;
    }

    loading = true;
    error = null;
    report = null;

    try {
      const res = await fetch('/api/code/quality-report', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files, language })
      });

      if (res.ok) {
        const data = await res.json();
        if (data.valid) {
          report = data.report;
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

  function addFile() {
    const parts = fileInput.split(':');
    if (parts.length === 2) {
      files[parts[0]] = parts[1];
      fileInput = '';
    }
  }

  function removeFile(name) {
    delete files[name];
    files = { ...files };
  }

  function getGradeColor(grade) {
    const colors = {
      'A': 'text-green-500',
      'B': 'text-blue-500',
      'C': 'text-yellow-500',
      'D': 'text-orange-500',
      'F': 'text-red-500'
    };
    return colors[grade] || 'text-gray-500';
  }

  function getScoreColor(score) {
    if (score >= 80) return 'text-green-500';
    if (score >= 60) return 'text-yellow-500';
    return 'text-red-500';
  }
</script>

<div class="quality-dashboard">
  <h2 class="text-xl font-semibold mb-4">代码质量仪表板</h2>

  <!-- 输入区域 -->
  <div class="card mb-4">
    <div class="flex gap-4 mb-3">
      <div>
        <label class="text-sm text-gray-500 mb-1 block">语言</label>
        <select bind:value={language} class="px-3 py-2 border rounded">
          <option value="go">Go</option>
          <option value="javascript">JavaScript</option>
          <option value="python">Python</option>
        </select>
      </div>
    </div>

    <div class="mb-3">
      <label class="text-sm text-gray-500 mb-1 block">添加文件 (文件名:内容)</label>
      <div class="flex gap-2">
        <input
          bind:value={fileInput}
          placeholder="main.go:package main..."
          class="flex-1 px-3 py-2 border rounded"
        />
        <button onclick={addFile} class="px-4 py-2 bg-gray-200 rounded">添加</button>
      </div>
    </div>

    <!-- 文件列表 -->
    {#if Object.keys(files).length > 0}
      <div class="mb-3">
        <label class="text-sm text-gray-500 mb-1 block">已添加文件</label>
        <div class="flex flex-wrap gap-2">
          {#each Object.keys(files) as name}
            <span class="px-2 py-1 bg-gray-100 rounded text-sm flex items-center gap-1">
              {name}
              <button onclick={() => removeFile(name)} class="text-red-500 hover:text-red-700">×</button>
            </span>
          {/each}
        </div>
      </div>
    {/if}

    <button
      onclick={analyze}
      disabled={loading || Object.keys(files).length === 0}
      class="px-4 py-2 bg-emerald-500 text-white rounded hover:bg-emerald-600 disabled:opacity-50"
    >
      {loading ? '分析中...' : '生成报告'}
    </button>
  </div>

  {#if error}
    <div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>
  {/if}

  {#if report}
    <!-- 总览卡片 -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
      <div class="card text-center">
        <div class="text-4xl font-bold {getScoreColor(report.score)}">{report.score}</div>
        <div class="text-sm text-gray-500">质量评分</div>
      </div>
      <div class="card text-center">
        <div class="text-4xl font-bold {getGradeColor(report.grade)}">{report.grade}</div>
        <div class="text-sm text-gray-500">等级</div>
      </div>
      <div class="card text-center">
        <div class="text-4xl font-bold">{report.summary.total_files}</div>
        <div class="text-sm text-gray-500">文件数</div>
      </div>
      <div class="card text-center">
        <div class="text-4xl font-bold">{report.summary.total_lines}</div>
        <div class="text-sm text-gray-500">代码行数</div>
      </div>
    </div>

    <!-- 指标详情 -->
    <div class="card mb-4">
      <h3 class="card-title">质量指标</h3>
      <div class="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
        <div>
          <div class="text-gray-500">平均复杂度</div>
          <div class="font-semibold">{report.metrics.average_complexity?.toFixed(1) || 'N/A'}</div>
        </div>
        <div>
          <div class="text-gray-500">最大复杂度</div>
          <div class="font-semibold">{report.metrics.max_complexity || 0}</div>
        </div>
        <div>
          <div class="text-gray-500">函数总数</div>
          <div class="font-semibold">{report.summary.total_functions}</div>
        </div>
        <div>
          <div class="text-gray-500">问题总数</div>
          <div class="font-semibold text-red-500">{report.summary.total_issues}</div>
        </div>
      </div>
    </div>

    <!-- 问题统计 -->
    {#if report.issues?.length > 0}
      <div class="card mb-4">
        <h3 class="card-title">问题统计</h3>
        <div class="space-y-2">
          {#each report.issues as issue}
            <div class="flex items-center justify-between p-2 bg-gray-50 rounded">
              <div class="flex items-center gap-2">
                <span class="text-xs px-2 py-0.5 rounded {issue.severity === 'critical' ? 'bg-red-100 text-red-700' : issue.severity === 'high' ? 'bg-orange-100 text-orange-700' : 'bg-yellow-100 text-yellow-700'}">
                  {issue.severity}
                </span>
                <span>{issue.title}</span>
              </div>
              <span class="font-semibold">{issue.count}</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- 重构建议 -->
    {#if report.suggestions?.length > 0}
      <div class="card">
        <h3 class="card-title">重构建议</h3>
        <div class="space-y-2 max-h-64 overflow-y-auto">
          {#each report.suggestions as suggestion}
            <div class="p-2 bg-gray-50 rounded">
              <div class="flex items-center gap-2 mb-1">
                <span class="text-xs px-2 py-0.5 bg-blue-100 text-blue-700 rounded">
                  {suggestion.type}
                </span>
                <span class="font-medium">{suggestion.title}</span>
              </div>
              <div class="text-sm text-gray-600">{suggestion.description}</div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .quality-dashboard { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

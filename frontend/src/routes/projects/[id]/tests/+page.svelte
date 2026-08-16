<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';

  const id = window.location.pathname.split('/').filter(Boolean).at(-2) || '';

  let project = $state<any>(null);
  let files = $state<Record<string, string>>({});
  let testType = $state<'shell' | 'unit' | 'integration'>('shell');
  let testResults = $state<any>(null);
  let testing = $state(false);
  let showReport = $state(false);
  let historyData = $state<any[]>([]);

  onMount(async () => {
    await loadProject();
    await loadHistory();
  });

  async function loadProject() {
    if (!id) return;
    try {
      const res = await fetch(`/api/v1/projects/${id}`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) project = await res.json();
    } catch {}
  }

  async function loadHistory() {
    if (!id) return;
    try {
      const res = await fetch(`/api/v1/projects/${id}/builds`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        const builds = await res.json();
        historyData = builds.slice(-20).filter((b: any) => b.status === 'success' || b.status === 'failed');
      }
    } catch {}
  }

  async function loadFiles() {
    if (!id) return;
    try {
      const res = await fetch(`/api/v1/projects/${id}/files`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        const fileList: string[] = await res.json();
        const fileMap: Record<string, string> = {};
        for (const f of fileList) {
          try {
            const fres = await fetch(`/api/v1/projects/${id}/files/${encodeURIComponent(f)}`, {
              headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
            });
            if (fres.ok) fileMap[f] = await fres.text();
          } catch {}
        }
        files = fileMap;
      }
    } catch {}
  }

  async function runTests() {
    testing = true;
    testResults = null;
    showReport = true;

    await loadFiles();

    try {
      const res = await fetch('/api/v1/agent/run', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}`,
        },
        body: JSON.stringify({
          task: `Run ${testType} tests on the project files`,
          messages: [],
          session_id: `test_${id}_${Date.now()}`,
        }),
      });

      if (!res.ok) throw new Error('测试请求失败');

      const reader = res.body?.getReader();
      if (!reader) throw new Error('无法读取响应流');

      let buffer = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += new TextDecoder().decode(value);
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6));
              testResults = data;
            } catch {}
          }
        }
      }
      toast(`${testType === 'shell' ? 'Shell' : testType === 'unit' ? '单元' : '集成'}测试完成`, 'success');
    } catch (e: any) {
      toast(e.message || '测试失败', 'error');
    } finally {
      testing = false;
    }
  }

  async function runLocalTests() {
    testing = true;
    testResults = null;
    showReport = true;

    await loadFiles();

    try {
      const res = await fetch('/api/v1/validate', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}`,
        },
        body: JSON.stringify({ files }),
      });

      if (res.ok) {
        const data = await res.json();
        testResults = {
          test_type: 'shell',
          total: (data.issues || []).length,
          passed: (data.issues || []).filter((i: any) => i.severity === 'info').length,
          failed: (data.issues || []).filter((i: any) => i.severity === 'critical' || i.severity === 'error').length,
          skipped: 0,
          cases: (data.issues || []).map((i: any) => ({
            name: `${i.file}: ${i.message}`,
            status: i.severity === 'critical' || i.severity === 'error' ? 'failed' : i.severity === 'warning' ? 'skipped' : 'passed',
            detail: i.rule || '',
          })),
        };
      }
    } catch (e: any) {
      toast(e.message || '本地测试失败', 'error');
    } finally {
      testing = false;
    }
  }

  const statusColor = (s: string) => {
    if (s === 'passed') return 'var(--color-success)';
    if (s === 'failed') return 'var(--color-error)';
    return 'var(--color-text-muted)';
  };

  const statusIcon = (s: string) => {
    if (s === 'passed') return 'check_circle';
    if (s === 'failed') return 'error';
    return 'skip_next';
  };
</script>

<div class="w-full p-6 max-w-4xl mx-auto">
  <div class="mb-6">
    <a href="/projects/{id}" class="inline-flex items-center gap-1 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] transition-colors mb-3 no-underline" onclick={(e) => { e.preventDefault(); window.history.pushState(null, '', '/projects/' + id); window.dispatchEvent(new PopStateEvent('popstate')); }}>
      <span class="material-symbols-outlined text-[14px]">arrow_back</span>
      返回编辑器
    </a>
    <div class="flex items-center gap-3 mb-1">
      <h1 class="text-xl font-bold text-[var(--color-text)]">自动化测试</h1>
      {#if project}
        <span class="badge-primary text-[10px]">{project.name}</span>
      {/if}
    </div>
    <p class="text-sm text-[var(--color-text-secondary)]">对模块脚本执行 Shell 测试、单元测试或集成测试</p>
  </div>

  <!-- Test Type Selection -->
  <div class="mb-6">
    <span class="text-sm font-medium text-[var(--color-text-secondary)] mb-3 block">测试类型</span>
    <div class="flex gap-2">
      {#each ['shell', 'unit', 'integration'] as type}
        <button
          class="flex-1 py-2.5 rounded-xl text-sm font-medium transition-all duration-200 flex items-center justify-center gap-2 cursor-pointer"
          style={testType === type
            ? 'background: var(--color-primary); color: white'
            : 'border: 1px solid var(--color-border); color: var(--color-text-secondary); background: var(--color-bg-elevated)'}
          onclick={() => testType = type as 'shell' | 'unit' | 'integration'}
          disabled={testing}
        >
          <span class="material-symbols-outlined text-[18px]">
            {type === 'shell' ? 'terminal' : type === 'unit' ? 'function' : 'integration_instructions'}
          </span>
          {type === 'shell' ? 'Shell 测试' : type === 'unit' ? '单元测试' : '集成测试'}
        </button>
      {/each}
    </div>
  </div>

  <!-- Run Buttons -->
  <div class="flex gap-3 mb-6">
    <button
      class="flex-1 py-3 rounded-xl font-semibold text-sm text-white transition-all duration-200 disabled:opacity-50
        bg-gradient-to-r from-primary-600 to-primary-700 hover:from-primary-700 hover:to-primary-800 active:scale-[0.98] shadow-sm hover:shadow-glow flex items-center justify-center gap-2"
      onclick={runTests}
      disabled={testing || !id}
    >
      {#if testing}
        <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/></svg>
        测试中...
      {:else}
        <span class="material-symbols-outlined text-[18px]">play_arrow</span>
        生成测试
      {/if}
    </button>
    <button
      class="px-5 py-3 rounded-xl text-sm font-medium transition-colors"
      style="border: 1px solid var(--color-border); color: var(--color-text-secondary); background: var(--color-bg-elevated)"
      onclick={runLocalTests}
      disabled={testing || !id}
    >
      快速检查
    </button>
  </div>

  <!-- Results -->
  {#if showReport}
    <div class="mb-6 p-4 rounded-2xl border" style="border-color: var(--color-border); background: var(--color-bg-elevated)">
      {#if testing}
        <div class="flex items-center gap-3">
          <div class="spinner w-5 h-5 rounded-full" style="border: 2px solid var(--color-border); border-top-color: var(--color-primary); animation: spin 0.8s linear infinite"></div>
          <span class="text-sm text-[var(--color-text-secondary)]">正在执行测试...</span>
        </div>
      {:else if testResults}
        <div class="space-y-4">
          <!-- Summary -->
          <div class="flex items-center gap-4">
            <div class="flex items-center gap-2">
              <span class="text-xs font-medium" style="color: var(--color-text-secondary)">共 {testResults.total || 0} 项</span>
              <span class="text-xs font-medium" style="color: var(--color-success)}">{testResults.passed || 0} 通过</span>
              <span class="text-xs font-medium" style="color: var(--color-error)}">{testResults.failed || 0} 失败</span>
              <span class="text-xs font-medium" style="color: var(--color-text-muted)}">{testResults.skipped || 0} 跳过</span>
            </div>
          </div>

          <!-- Test Cases -->
          {#if testResults.cases && testResults.cases.length > 0}
            <div class="space-y-1 max-h-64 overflow-auto">
              {#each testResults.cases as c}
                <div class="flex items-center gap-2 p-2 rounded-lg" style="background: var(--color-surface)">
                  <span class="material-symbols-outlined text-[16px]" style="color: {statusColor(c.status)}">{statusIcon(c.status)}</span>
                  <span class="text-xs flex-1" style="color: var(--color-text)">{c.name}</span>
                  {#if c.detail}
                    <span class="text-[10px]" style="color: var(--color-text-muted)">{c.detail}</span>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}

          <!-- Generated Code -->
          {#if testResults.code}
            <div>
              <button
                class="text-xs flex items-center gap-1 px-3 py-1 rounded-lg"
                style="border: 1px solid var(--color-border)"
                onclick={() => { const el = document.getElementById('test-code'); if (el) el.hidden = !el.hidden; }}
              >
                <span class="material-symbols-outlined text-[14px]">code</span>
                查看测试代码
              </button>
              <pre id="test-code" hidden class="mt-2 p-3 rounded-xl text-xs font-mono overflow-auto max-h-48" style="background: #0a0a0a; color: #4ade80">{[testResults.code]}</pre>
            </div>
          {/if}
        </div>
      {:else}
        <p class="text-sm text-[var(--color-text-secondary)]">点击"生成测试"开始</p>
      {/if}
    </div>
  {/if}

  <!-- History Trend -->
  {#if historyData.length > 0}
    <div class="mt-8">
      <h3 class="text-sm font-semibold text-[var(--color-text)] mb-3 flex items-center gap-2">
        <span class="material-symbols-outlined text-[18px]">trending_up</span>
        构建历史趋势
      </h3>
      <div class="flex items-end gap-1 h-20" style="border-bottom: 1px solid var(--color-border)">
        {#each historyData as b, i}
          {@const h = b.status === 'success' ? 60 + Math.random() * 40 : 20 + Math.random() * 30}
          <div
            class="flex-1 rounded-t transition-all duration-300"
            style="height: {h}%; background: {b.status === 'success' ? 'var(--color-success)' : 'var(--color-error)'}; opacity: {0.5 + (i / historyData.length) * 0.5}"
            title="Build {b.id?.slice(0, 8) || ''}: {b.status}"
          ></div>
        {/each}
      </div>
      <div class="flex justify-between mt-1">
        <span class="text-[10px]" style="color: var(--color-text-muted)">旧</span>
        <span class="text-[10px]" style="color: var(--color-text-muted)">新</span>
      </div>
    </div>
  {/if}
</div>

<style>
  .spinner {
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>

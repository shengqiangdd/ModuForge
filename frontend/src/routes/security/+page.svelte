<script lang="ts">
  import { onMount } from 'svelte';

  interface Vuln {
    id: number;
    file: string;
    line: number;
    severity: 'critical' | 'high' | 'medium' | 'low';
    rule: string;
    message: string;
    project_id?: number;
    created_at: string;
  }

  interface VulnHistory {
    id: number;
    scan_type: string;
    total: number;
    critical: number;
    high: number;
    medium: number;
    low: number;
    created_at: string;
  }

  let vulns = $state<Vuln[]>([]);
  let history = $state<VulnHistory[]>([]);
  let loading = $state(true);
  let scanning = $state(false);
  let errorMsg = $state('');
  let successMsg = $state('');
  let filterSeverity = $state('');
  let filterFile = $state('');
  let scanFiles = $state('');
  let scanProjectId = $state('');
  let showScanDialog = $state(false);

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }

  const severityColor = (s: string) => {
    if (s === 'critical') return '#dc2626';
    if (s === 'high') return '#f97316';
    if (s === 'medium') return '#eab308';
    return '#3b82f6';
  };

  const severityLabel = (s: string) => {
    if (s === 'critical') return '严重';
    if (s === 'high') return '高危';
    if (s === 'medium') return '中等';
    return '低危';
  };

  function msg(err: string, ok?: string) {
    errorMsg = err; successMsg = ok || '';
    setTimeout(() => { errorMsg = ''; successMsg = ''; }, 4000);
  }

  async function loadHistory() {
    loading = true;
    try {
      const r = await fetch('/api/v1/projects/0/vuln-history', { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { const d = await r.json(); history = d.history || d || []; }
    } catch (e) { console.error('load vuln history failed:', e); }
    loading = false;
  }

  async function loadVulns() {
    loading = true;
    try {
      const r = await fetch('/api/v1/projects/0/vuln-history', { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) {
        const d = await r.json();
        vulns = (d.vulns || d.items || []).map((v: any, i: number) => ({
          id: v.id || i,
          file: v.file || v.filename || '',
          line: v.line || 0,
          severity: v.severity || 'low',
          rule: v.rule || v.rule_id || '',
          message: v.message || v.description || '',
          project_id: v.project_id,
          created_at: v.created_at || new Date().toISOString(),
        }));
      }
    } catch (e) { console.error('load vulns failed:', e); }
    loading = false;
  }

  async function runScan() {
    scanning = true;
    try {
      const files = scanFiles.split(',').map(f => f.trim()).filter(Boolean);
      const body: any = { files };
      if (scanProjectId) body.project_id = Number(scanProjectId);
      const r = await fetch('/api/v1/security/scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify(body),
      });
      if (r.ok) {
        msg('', '扫描完成');
        showScanDialog = false;
        scanFiles = '';
        scanProjectId = '';
        loadHistory();
      } else {
        const d = await r.json().catch(() => ({}));
        msg(d.error || '扫描失败');
      }
    } catch (e: any) {
      msg(e?.message || '扫描失败');
    }
    scanning = false;
  }

  const totalVulns = $derived(history.reduce((s, h) => s + h.total, 0));
  const totalCritical = $derived(history.reduce((s, h) => s + h.critical, 0));
  const totalHigh = $derived(history.reduce((s, h) => s + h.high, 0));
  const totalMedium = $derived(history.reduce((s, h) => s + h.medium, 0));
  const totalLow = $derived(history.reduce((s, h) => s + h.low, 0));

  const filteredVulns = $derived(
    vulns.filter(v =>
      (!filterSeverity || v.severity === filterSeverity) &&
      (!filterFile || v.file.toLowerCase().includes(filterFile.toLowerCase()))
    )
  );

  onMount(() => { loadHistory(); });
</script>

<div class="w-full p-4 md:p-6 max-w-5xl mx-auto space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold text-[var(--color-text)]">安全扫描</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">漏洞检测与安全审计</p>
    </div>
    <div class="flex gap-2">
      <button class="px-3 py-1.5 rounded-lg text-sm flex items-center gap-1.5" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => { loadHistory(); }}>
        <span class="material-symbols-outlined text-[16px]">refresh</span>
        刷新
      </button>
      <button class="px-3 py-1.5 rounded-lg text-sm font-medium" style="background: var(--color-primary); color: white" onclick={() => showScanDialog = !showScanDialog}>
        <span class="material-symbols-outlined text-[16px] align-middle">shield</span>
        新扫描
      </button>
    </div>
  </div>

  {#if errorMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-error-light); color: var(--color-error)">{errorMsg}</div>
  {/if}
  {#if successMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-success-light); color: var(--color-success)">{successMsg}</div>
  {/if}

  <!-- Scan Dialog -->
  {#if showScanDialog}
    <div class="card p-5 space-y-3">
      <h3 class="text-sm font-semibold text-[var(--color-text)]">手动扫描</h3>
      <input class="input-field w-full" placeholder="文件路径，逗号分隔" bind:value={scanFiles} />
      <input class="input-field w-full" placeholder="项目 ID（可选）" bind:value={scanProjectId} />
      <div class="flex gap-2 justify-end">
        <button class="px-3 py-1.5 rounded-lg text-sm" style="background: var(--color-surface); color: var(--color-text-secondary)" onclick={() => showScanDialog = false}>取消</button>
        <button class="px-3 py-1.5 rounded-lg text-sm font-medium" style="background: var(--color-primary); color: white" disabled={scanning || !scanFiles.trim()} onclick={runScan}>
          {scanning ? '扫描中...' : '开始扫描'}
        </button>
      </div>
    </div>
  {/if}

  <!-- Stats -->
  <div class="grid grid-cols-2 md:grid-cols-4 gap-2 md:gap-4">
    <div class="card p-4 text-center">
      <p class="text-2xl font-bold text-[var(--color-text)]">{totalVulns}</p>
      <p class="text-xs mt-1" style="color: var(--color-text-muted)">总漏洞</p>
    </div>
    <div class="card p-4 text-center">
      <p class="text-2xl font-bold" style="color: {severityColor('critical')}">{totalCritical}</p>
      <p class="text-xs mt-1" style="color: var(--color-text-muted)">严重</p>
    </div>
    <div class="card p-4 text-center">
      <p class="text-2xl font-bold" style="color: {severityColor('high')}">{totalHigh}</p>
      <p class="text-xs mt-1" style="color: var(--color-text-muted)">高危</p>
    </div>
    <div class="card p-4 text-center">
      <p class="text-2xl font-bold" style="color: {severityColor('medium')}">{totalMedium}</p>
      <p class="text-xs mt-1" style="color: var(--color-text-muted)">中等</p>
    </div>
  </div>

  <!-- History Timeline -->
  {#if history.length > 0}
    <div class="card p-5">
      <h3 class="text-sm font-semibold mb-3 text-[var(--color-text)]">扫描历史</h3>
      <div class="space-y-2">
        {#each history as h, i (h.id || i)}
          <div class="flex items-center gap-3 text-sm">
            <div class="w-2 h-2 rounded-full flex-shrink-0" style="background: {h.total > 0 ? severityColor('high') : '#22c55e'}"></div>
            <div class="flex-1">
              <span class="text-[var(--color-text)]">{h.scan_type}</span>
              <span class="text-[var(--color-text-muted)] ml-2">— {h.total} 个漏洞</span>
            </div>
            <span class="text-xs text-[var(--color-text-muted)]">{new Date(h.created_at).toLocaleString()}</span>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Filters -->
  <div class="flex gap-3 flex-wrap">
    <select class="input-field" bind:value={filterSeverity}>
      <option value="">全部级别</option>
      <option value="critical">严重</option>
      <option value="high">高危</option>
      <option value="medium">中等</option>
      <option value="low">低危</option>
    </select>
    <input class="input-field flex-1 min-w-[150px]" placeholder="按文件名过滤..." bind:value={filterFile} />
  </div>

  <!-- Vuln List -->
  {#if loading}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">加载中...</div>
  {:else if filteredVulns.length === 0}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">暂无漏洞数据，点击"新扫描"开始检测</div>
  {:else}
    <div class="space-y-2">
      {#each filteredVulns as v (v.id)}
        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="w-2 h-2 rounded-full flex-shrink-0" style="background: {severityColor(v.severity)}"></div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="text-xs px-2 py-0.5 rounded-full font-medium" style="background: {severityColor(v.severity)}22; color: {severityColor(v.severity)}">
                  {severityLabel(v.severity)}
                </span>
                <span class="text-sm font-medium text-[var(--color-text)] truncate">{v.rule}</span>
                {#if v.file}
                  <span class="text-xs text-[var(--color-text-muted)]">{v.file}:{v.line}</span>
                {/if}
              </div>
              <p class="text-xs text-[var(--color-text-secondary)] mt-1">{v.message}</p>
            </div>
            <span class="text-xs text-[var(--color-text-muted)] flex-shrink-0">{new Date(v.created_at).toLocaleDateString()}</span>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .input-field {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 6px 12px;
    color: var(--color-text);
    font-size: 14px;
    outline: none;
  }
  .input-field:focus { border-color: var(--color-primary); }
</style>

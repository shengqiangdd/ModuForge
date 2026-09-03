<script lang="ts">
  import { onMount } from 'svelte';

  interface CompatibilityCheck {
    name: string;
    category: string;
    status: string;
    message: string;
    fix: string;
  }

  interface PlatformCompat {
    compatible: boolean;
    score: number;
    issues: string[];
  }

  interface CompatibilityResult {
    module_id: string;
    module_name: string;
    module_type: string;
    checks: CompatibilityCheck[];
    score: number;
    summary: string;
    recommendations: string[];
    platforms: Record<string, PlatformCompat>;
  }

  let projects = $state<any[]>([]);
  let selectedProjectId = $state('');
  let checking = $state(false);
  let result = $state<CompatibilityResult | null>(null);
  let files = $state<Record<string, string>>({});
  let activeTab = $state('overview');

  const categoryIcons: Record<string, string> = {
    structure: 'folder',
    script: 'terminal',
    config: 'settings',
    security: 'shield',
    api: 'api',
  };

  const categoryLabels: Record<string, string> = {
    structure: '模块结构',
    script: '脚本检查',
    config: '配置检查',
    security: '安全检查',
    api: 'API 兼容性',
  };

  async function loadProjects() {
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/projects', {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        projects = Array.isArray(data) ? data : (data.projects || []);
      }
    } catch (e) {
      console.error('Failed to load projects:', e);
    }
  }

  async function loadProjectFiles(projectId: string) {
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/projects/${projectId}/files`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        files = {};
        const fileList = Array.isArray(data) ? data : (data.files || []);
        for (const f of fileList) {
          if (f.path && f.content) {
            files[f.path] = f.content;
          }
        }
      }
    } catch (e) {
      console.error('Failed to load files:', e);
    }
  }

  async function runCheck() {
    checking = true;
    result = null;
    try {
      const token = localStorage.getItem('token');
      if (selectedProjectId) {
        await loadProjectFiles(selectedProjectId);
      }
      const res = await fetch('/api/v1/compat-check/check', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          project_id: selectedProjectId || undefined,
          files,
        })
      });
      if (res.ok) {
        result = await res.json();
      }
    } catch (e) {
      console.error('Failed to run check:', e);
    }
    checking = false;
  }

  function getScoreColor(score: number): string {
    if (score >= 80) return '#10b981';
    if (score >= 60) return '#f59e0b';
    return '#ef4444';
  }

  function getScoreLabel(score: number): string {
    if (score >= 90) return '优秀';
    if (score >= 70) return '良好';
    if (score >= 50) return '一般';
    return '需改进';
  }

  function getStatusIcon(status: string): string {
    switch (status) {
      case 'pass': return 'check_circle';
      case 'warn': return 'warning';
      case 'fail': return 'error';
      default: return 'help';
    }
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case 'pass': return '#10b981';
      case 'warn': return '#f59e0b';
      case 'fail': return '#ef4444';
      default: return '#6b7280';
    }
  }

  let groupedChecks = $derived(() => {
    if (!result) return {};
    const groups: Record<string, CompatibilityCheck[]> = {};
    for (const check of result.checks) {
      if (!groups[check.category]) groups[check.category] = [];
      groups[check.category].push(check);
    }
    return groups;
  });

  let passCount = $derived(() => result?.checks.filter(c => c.status === 'pass').length || 0);
  let warnCount = $derived(() => result?.checks.filter(c => c.status === 'warn').length || 0);
  let failCount = $derived(() => result?.checks.filter(c => c.status === 'fail').length || 0);

  onMount(loadProjects);
</script>

<div class="page">
  <div class="header">
    <h1>模块兼容性检查</h1>
    <p class="subtitle">检查模块在 Magisk / KernelSU / APatch 上的兼容性</p>
  </div>

  <!-- Config Bar -->
  <div class="config-bar">
    <div class="config-left">
      <label class="config-label">
        选择项目
        <select bind:value={selectedProjectId}>
          <option value="">手动上传文件</option>
          {#each projects as p}
            <option value={p.id}>{p.name}</option>
          {/each}
        </select>
      </label>
    </div>
    <button class="btn-primary" onclick={runCheck} disabled={checking}>
      {#if checking}
        <span class="spinner"></span> 检查中...
      {:else}
        <span class="material-symbols-outlined" style="font-size:18px;">verified</span>
        运行检查
      {/if}
    </button>
  </div>

  <!-- Results -->
  {#if result}
    <!-- Score Overview -->
    <div class="score-overview">
      <div class="score-card main-score">
        <div class="score-ring" style="--score-color: {getScoreColor(result.score)}">
          <svg viewBox="0 0 100 100">
            <circle cx="50" cy="50" r="42" fill="none" stroke="var(--color-border)" stroke-width="6" />
            <circle cx="50" cy="50" r="42" fill="none" stroke={getScoreColor(result.score)} stroke-width="6"
              stroke-dasharray="{result.score * 2.64} 264" stroke-linecap="round"
              transform="rotate(-90 50 50)" />
          </svg>
          <div class="score-text">
            <span class="score-num">{result.score}</span>
            <span class="score-label">{getScoreLabel(result.score)}</span>
          </div>
        </div>
        <div class="score-meta">
          <h3>兼容性评分</h3>
          <p>{result.summary}</p>
        </div>
      </div>

      <div class="score-card">
        <div class="stat-row">
          <span class="stat-icon" style="color: #10b981;"><span class="material-symbols-outlined">check_circle</span></span>
          <div>
            <span class="stat-num">{passCount()}</span>
            <span class="stat-label">通过</span>
          </div>
        </div>
      </div>
      <div class="score-card">
        <div class="stat-row">
          <span class="stat-icon" style="color: #f59e0b;"><span class="material-symbols-outlined">warning</span></span>
          <div>
            <span class="stat-num">{warnCount()}</span>
            <span class="stat-label">警告</span>
          </div>
        </div>
      </div>
      <div class="score-card">
        <div class="stat-row">
          <span class="stat-icon" style="color: #ef4444;"><span class="material-symbols-outlined">error</span></span>
          <div>
            <span class="stat-num">{failCount()}</span>
            <span class="stat-label">失败</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Platform Compat -->
    <div class="platform-section">
      <h2>平台兼容性</h2>
      <div class="platform-grid">
        {#each Object.entries(result.platforms) as [name, compat]}
          <div class="platform-card" class:compatible={compat.compatible} class:incompatible={!compat.compatible}>
            <div class="platform-header">
              <span class="platform-name">{name.toUpperCase()}</span>
              <span class="platform-badge" style="background: {compat.compatible ? '#10b98120' : '#ef444420'}; color: {compat.compatible ? '#10b981' : '#ef4444'}">
                {compat.compatible ? '兼容' : '不兼容'}
              </span>
            </div>
            <div class="platform-score-bar">
              <div class="platform-score-fill" style="width: {compat.score}%; background: {getScoreColor(compat.score)}"></div>
            </div>
            <span class="platform-score-text">{compat.score}/100</span>
            {#if compat.issues.length > 0}
              <ul class="issue-list">
                {#each compat.issues as issue}
                  <li>{issue}</li>
                {/each}
              </ul>
            {/if}
          </div>
        {/each}
      </div>
    </div>

    <!-- Check Details -->
    <div class="details-section">
      <h2>详细检查结果</h2>
      <div class="detail-tabs">
        {#each Object.entries(groupedChecks()) as [category, checks]}
          <button class="detail-tab" class:active={activeTab === category} onclick={() => activeTab = category}>
            <span class="material-symbols-outlined" style="font-size:16px;">{categoryIcons[category] || 'check'}</span>
            {categoryLabels[category] || category}
            <span class="tab-badge">{checks.length}</span>
          </button>
        {/each}
      </div>

      {#each Object.entries(groupedChecks()) as [category, checks]}
        {#if activeTab === category}
          <div class="check-list">
            {#each checks as check}
              <div class="check-item" style="border-left-color: {getStatusColor(check.status)}">
                <div class="check-header">
                  <span class="material-symbols-outlined" style="color: {getStatusColor(check.status)}; font-size: 20px;">{getStatusIcon(check.status)}</span>
                  <span class="check-name">{check.name}</span>
                </div>
                <p class="check-message">{check.message}</p>
                {#if check.fix}
                  <div class="check-fix">
                    <span class="material-symbols-outlined" style="font-size: 14px;">lightbulb</span>
                    建议: {check.fix}
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      {/each}
    </div>

    <!-- Recommendations -->
    {#if result.recommendations.length > 0}
      <div class="recommendations">
        <h2>改进建议</h2>
        <ul>
          {#each result.recommendations as rec}
            <li>
              <span class="material-symbols-outlined" style="color: var(--color-primary); font-size: 18px;">tips_and_updates</span>
              {rec}
            </li>
          {/each}
        </ul>
      </div>
    {/if}
  {:else if !checking}
    <div class="empty-state">
      <span class="material-symbols-outlined" style="font-size: 4rem; color: var(--color-text-muted);">verified</span>
      <p>选择项目后点击「运行检查」分析模块兼容性</p>
    </div>
  {/if}
</div>

<style>
  .page { padding: 1.5rem; max-width: 1200px; margin: 0 auto; }
  .header { margin-bottom: 1.5rem; }
  .header h1 { font-size: 1.5rem; font-weight: 700; color: var(--color-text); margin: 0; }
  .subtitle { color: var(--color-text-muted); font-size: 0.9rem; margin-top: 0.25rem; }

  .config-bar { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1rem 1.25rem; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-bg-card); margin-bottom: 1.5rem; flex-wrap: wrap; }
  .config-left { flex: 1; }
  .config-label { display: flex; align-items: center; gap: 0.75rem; font-size: 0.85rem; font-weight: 500; color: var(--color-text-secondary); }
  .config-label select { padding: 0.45rem 0.7rem; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-bg); color: var(--color-text); font-size: 0.85rem; outline: none; min-width: 200px; }
  .config-label select:focus { border-color: var(--color-primary); }

  .btn-primary { display: inline-flex; align-items: center; gap: 6px; padding: 0.55rem 1.2rem; border: none; border-radius: 10px; background: var(--color-primary); color: white; font-weight: 600; font-size: 0.9rem; cursor: pointer; }
  .btn-primary:hover { opacity: 0.9; }
  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
  .spinner { width: 16px; height: 16px; border: 2px solid rgba(255,255,255,0.3); border-top-color: white; border-radius: 50%; animation: spin 0.6s linear infinite; display: inline-block; }
  @keyframes spin { to { transform: rotate(360deg); } }

  .score-overview { display: grid; grid-template-columns: 1.5fr 1fr 1fr 1fr; gap: 1rem; margin-bottom: 1.5rem; }
  @media (max-width: 800px) { .score-overview { grid-template-columns: 1fr 1fr; } }
  .score-card { border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-bg-card); padding: 1.25rem; }
  .main-score { display: flex; align-items: center; gap: 1.25rem; grid-column: 1; }
  .score-ring { position: relative; width: 100px; height: 100px; flex-shrink: 0; }
  .score-ring svg { width: 100%; height: 100%; }
  .score-text { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; }
  .score-num { font-size: 1.75rem; font-weight: 800; color: var(--color-text); }
  .score-label { font-size: 0.7rem; color: var(--color-text-muted); }
  .score-meta h3 { margin: 0; font-size: 1rem; color: var(--color-text); }
  .score-meta p { margin: 0.25rem 0 0; font-size: 0.8rem; color: var(--color-text-muted); }

  .stat-row { display: flex; align-items: center; gap: 0.75rem; }
  .stat-icon .material-symbols-outlined { font-size: 1.5rem; }
  .stat-num { font-size: 1.5rem; font-weight: 700; color: var(--color-text); display: block; }
  .stat-label { font-size: 0.75rem; color: var(--color-text-muted); }

  .platform-section { margin-bottom: 1.5rem; }
  .platform-section h2, .details-section h2, .recommendations h2 { font-size: 1.1rem; font-weight: 700; color: var(--color-text); margin: 0 0 1rem; }
  .platform-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 1rem; }
  .platform-card { border: 1px solid var(--color-border); border-radius: 12px; padding: 1rem; background: var(--color-bg-card); }
  .platform-card.compatible { border-left: 3px solid #10b981; }
  .platform-card.incompatible { border-left: 3px solid #ef4444; }
  .platform-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem; }
  .platform-name { font-weight: 700; font-size: 0.9rem; color: var(--color-text); }
  .platform-badge { padding: 2px 8px; border-radius: 10px; font-size: 0.7rem; font-weight: 600; }
  .platform-score-bar { height: 6px; background: var(--color-border); border-radius: 3px; margin-bottom: 0.3rem; overflow: hidden; }
  .platform-score-fill { height: 100%; border-radius: 3px; transition: width 0.5s; }
  .platform-score-text { font-size: 0.75rem; color: var(--color-text-muted); }
  .issue-list { margin: 0.5rem 0 0; padding-left: 1.2rem; }
  .issue-list li { font-size: 0.8rem; color: var(--color-text-secondary); margin-bottom: 0.2rem; }

  .details-section { margin-bottom: 1.5rem; }
  .detail-tabs { display: flex; gap: 0.3rem; flex-wrap: wrap; margin-bottom: 1rem; }
  .detail-tab { display: inline-flex; align-items: center; gap: 4px; padding: 0.4rem 0.75rem; border: 1px solid var(--color-border); border-radius: 8px; background: transparent; color: var(--color-text-secondary); font-size: 0.8rem; cursor: pointer; transition: all 0.2s; }
  .detail-tab.active { background: color-mix(in srgb, var(--color-primary) 12%, transparent); border-color: var(--color-primary); color: var(--color-primary); font-weight: 600; }
  .tab-badge { font-size: 0.65rem; background: var(--color-surface); padding: 1px 5px; border-radius: 8px; }

  .check-list { display: flex; flex-direction: column; gap: 0.5rem; }
  .check-item { border-left: 3px solid; padding: 0.75rem 1rem; border-radius: 0 8px 8px 0; background: var(--color-bg-card); }
  .check-header { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.3rem; }
  .check-name { font-weight: 600; font-size: 0.9rem; color: var(--color-text); }
  .check-message { margin: 0; font-size: 0.82rem; color: var(--color-text-secondary); }
  .check-fix { display: flex; align-items: center; gap: 0.3rem; margin-top: 0.4rem; font-size: 0.78rem; color: var(--color-primary); background: color-mix(in srgb, var(--color-primary) 6%, transparent); padding: 0.3rem 0.6rem; border-radius: 6px; }

  .recommendations ul { list-style: none; padding: 0; }
  .recommendations li { display: flex; align-items: flex-start; gap: 0.5rem; padding: 0.5rem 0; border-bottom: 1px solid var(--color-border); font-size: 0.85rem; color: var(--color-text-secondary); }
  .recommendations li:last-child { border-bottom: none; }

  .empty-state { text-align: center; padding: 4rem 2rem; }
  .empty-state p { color: var(--color-text-secondary); margin-top: 0.75rem; }
</style>

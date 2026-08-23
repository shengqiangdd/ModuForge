<script lang="ts">
import { onMount } from 'svelte';
import { client } from '$lib/api/client';

let { onNavigate }: { onNavigate?: (route: string, id?: string) => void } = $props();

// State
let loading = $state(true);
let isAdmin = $state(false);
let overview = $state<any>(null);
let users = $state<any[]>([]);
let models = $state<any[]>([]);
let modes = $state<any[]>([]);
let timeline = $state<any[]>([]);

// Filters
let selectedUserId = $state('');
let timelineDays = $state(30);
let activeTab = $state<'overview' | 'users' | 'models' | 'modes' | 'timeline'>('overview');

// Check admin status and load data
onMount(async () => {
  const token = localStorage.getItem('moduforge_token');
  if (token) {
    try {
      const r = await client.get<any>('/auth/profile');
      isAdmin = r?.is_admin || r?.role === 'admin' || false;
    } catch {}
  }
  loadAnalytics();
});

// Load data
async function loadAnalytics() {
  loading = true;
  try {
    const params = new URLSearchParams();
    // Only admin can filter by specific user
    if (isAdmin && selectedUserId) params.set('user_id', selectedUserId);
    const qs = params.toString();
    const base = '/analytics';

    const [ov, us, mo, tl, md] = await Promise.all([
      client.get<any>(`${base}/overview${qs ? '?' + qs : ''}`).catch(() => null),
      client.get<any>(`${base}/users?limit=50`).catch(() => ({ users: [] })),
      client.get<any>(`${base}/models${qs ? '?' + qs : ''}`).catch(() => ({ models: [] })),
      client.get<any>(`${base}/timeline?days=${timelineDays}${qs ? '&' + qs : ''}`).catch(() => ({ timeline: [] })),
      client.get<any>(`${base}/modes${qs ? '?' + qs : ''}`).catch(() => ({ modes: [] })),
    ]);

    overview = ov;
    users = us?.users || [];
    models = mo?.models || [];
    modes = md?.modes || [];
    timeline = tl?.timeline || [];
  } catch (e: any) {
    console.error('Failed to load analytics:', e);
  } finally {
    loading = false;
  }
}

onMount(loadAnalytics);

function formatTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
  return String(n);
}

function formatDate(d: string): string {
  const parts = d.split('-');
  return `${parts[1]}/${parts[2]}`;
}

// Timeline chart: compute max for bar height
let maxTokenUsage = $derived(Math.max(1, ...timeline.map((t: any) => t.token_usage || 0)));

const modeLabels: Record<string, string> = {
  generate: '生成模块',
  chat: 'AI 对话',
  repair: '修复构建',
  gather: '需求收集',
  agent: 'Agent',
  'auto-build': '智能构建',
};

const modeIcons: Record<string, string> = {
  generate: 'auto_fix_high',
  chat: 'chat',
  repair: 'build_circle',
  gather: 'checklist',
  agent: 'smart_toy',
  'auto-build': 'rocket_launch',
};
</script>

<div class="analytics-page">
  <!-- Header -->
  <div class="header">
    <div class="header-left">
      <button class="back-btn" onclick={() => onNavigate?.('projects')} title="返回">
        <span class="material-symbols-outlined text-[18px]">arrow_back</span>
      </button>
      <div>
        <h1 class="title">AI 统计</h1>
        <p class="subtitle">Token 消耗 · 模型使用 · 模式分析</p>
      </div>
    </div>
    <div class="header-actions">
      <select class="filter-select" bind:value={timelineDays} onchange={() => loadAnalytics()}>
        <option value={7}>近 7 天</option>
        <option value={30}>近 30 天</option>
        <option value={90}>近 90 天</option>
        <option value={180}>近半年</option>
        <option value={365}>近一年</option>
      </select>
      <button class="refresh-btn" onclick={loadAnalytics()} disabled={loading}>
        <span class="material-symbols-outlined text-[18px] {loading ? 'animate-spin' : ''}">refresh</span>
      </button>
    </div>
  </div>

  {#if loading && !overview}
    <div class="loading-state">
      <div class="spinner-lg"></div>
      <p>加载统计数据...</p>
    </div>
  {:else}
    <!-- Tabs -->
    <div class="tabs">
      {#each [
        { id: 'overview', label: '概览', icon: 'monitoring', adminOnly: false },
        { id: 'users', label: '用户', icon: 'group', adminOnly: true },
        { id: 'models', label: '模型', icon: 'psychology', adminOnly: false },
        { id: 'modes', label: '模式', icon: 'tune', adminOnly: false },
        { id: 'timeline', label: '趋势', icon: 'show_chart', adminOnly: false },
      ] as tab}
        {#if !tab.adminOnly || isAdmin}
          <button
            class="tab-btn"
            class:active={activeTab === tab.id}
            onclick={() => activeTab = tab.id}
          >
            <span class="material-symbols-outlined text-[16px]">{tab.icon}</span>
            <span>{tab.label}</span>
          </button>
        {/if}
      {/each}
    </div>

    <!-- Content -->
    <div class="content">
      {#if activeTab === 'overview'}
        <!-- Overview Cards -->
        <div class="overview-grid">
          <div class="stat-card">
            <div class="stat-icon" style="background: var(--color-primary-light); color: var(--color-primary)">
              <span class="material-symbols-outlined">group</span>
            </div>
            <div class="stat-info">
              <span class="stat-value">{overview?.total_users ?? 0}</span>
              <span class="stat-label">活跃用户</span>
            </div>
          </div>
          <div class="stat-card">
            <div class="stat-icon" style="background: var(--color-info-light); color: var(--color-info)">
              <span class="material-symbols-outlined">chat</span>
            </div>
            <div class="stat-info">
              <span class="stat-value">{overview?.total_conversations ?? 0}</span>
              <span class="stat-label">总对话数</span>
            </div>
          </div>
          <div class="stat-card">
            <div class="stat-icon" style="background: var(--color-success-light); color: var(--color-success)">
              <span class="material-symbols-outlined">token</span>
            </div>
            <div class="stat-info">
              <span class="stat-value">{formatTokens(overview?.total_tokens ?? 0)}</span>
              <span class="stat-label">总 Token 消耗</span>
            </div>
          </div>
          <div class="stat-card">
            <div class="stat-icon" style="background: var(--color-warning-light); color: var(--color-warning)">
              <span class="material-symbols-outlined">smart_toy</span>
            </div>
            <div class="stat-info">
              <span class="stat-value">{overview?.total_models ?? 0}</span>
              <span class="stat-label">使用模型数</span>
            </div>
          </div>
        </div>

        <!-- Build Stats -->
        <div class="section">
          <h2 class="section-title">构建统计</h2>
          <div class="overview-grid">
            <div class="stat-card wide">
              <div class="stat-icon" style="background: var(--color-success-light); color: var(--color-success)">
                <span class="material-symbols-outlined">build</span>
              </div>
              <div class="stat-info">
                <span class="stat-value">{overview?.total_builds ?? 0}</span>
                <span class="stat-label">总构建次数</span>
              </div>
            </div>
            <div class="stat-card wide">
              <div class="stat-icon" style="background: var(--color-info-light); color: var(--color-info)">
                <span class="material-symbols-outlined">check_circle</span>
              </div>
              <div class="stat-info">
                <span class="stat-value">{(overview?.build_success_rate ?? 0).toFixed(1)}%</span>
                <span class="stat-label">构建成功率</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Quick Mode & Model Preview -->
        <div class="two-col">
          <div class="section">
            <h2 class="section-title">模式使用 TOP 3</h2>
            <div class="mini-list">
              {#each modes.slice(0, 3) as m}
                <div class="mini-item">
                  <span class="material-symbols-outlined text-[16px]" style="color: var(--color-primary)">{modeIcons[m.mode] || 'help'}</span>
                  <span class="mini-name">{modeLabels[m.mode] || m.mode}</span>
                  <span class="mini-value">{formatTokens(m.total_tokens)}</span>
                </div>
              {/each}
            </div>
          </div>
          <div class="section">
            <h2 class="section-title">模型使用 TOP 3</h2>
            <div class="mini-list">
              {#each models.slice(0, 3) as m}
                <div class="mini-item">
                  <span class="material-symbols-outlined text-[16px]" style="color: var(--color-info)">psychology</span>
                  <span class="mini-name">{m.model}</span>
                  <span class="mini-value">{formatTokens(m.total_tokens)}</span>
                </div>
              {/each}
            </div>
          </div>
        </div>

      {:else if activeTab === 'users'}
        <div class="section">
          <h2 class="section-title">用户 Token 消耗排名</h2>
          <div class="table-wrap">
            <table class="data-table">
              <thead>
                <tr>
                  <th>排名</th>
                  <th>用户</th>
                  <th>Token 消耗</th>
                  <th>对话数</th>
                  <th>最常使用模型</th>
                </tr>
              </thead>
              <tbody>
                {#each users as u, i}
                  <tr>
                    <td>
                      <span class="rank" class:rank-top={i < 3}>{i + 1}</span>
                    </td>
                    <td class="cell-user">
                      <div class="user-avatar">{u.username?.charAt(0)?.toUpperCase() || '?'}</div>
                      <span>{u.username || u.id}</span>
                    </td>
                    <td class="cell-tokens">{formatTokens(u.total_tokens)}</td>
                    <td>{u.conversation_count}</td>
                    <td class="cell-model">{u.favorite_model || '-'}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
          {#if users.length === 0}
            <div class="empty-state">暂无用户数据</div>
          {/if}
        </div>

      {:else if activeTab === 'models'}
        <div class="section">
          <h2 class="section-title">模型使用统计</h2>
          <div class="table-wrap">
            <table class="data-table">
              <thead>
                <tr>
                  <th>模型</th>
                  <th>调用次数</th>
                  <th>Token 消耗</th>
                  <th>使用用户数</th>
                  <th>平均 Token/次</th>
                </tr>
              </thead>
              <tbody>
                {#each models as m}
                  <tr>
                    <td class="cell-model">{m.model}</td>
                    <td>{m.call_count}</td>
                    <td class="cell-tokens">{formatTokens(m.total_tokens)}</td>
                    <td>{m.user_count}</td>
                    <td>{m.call_count > 0 ? formatTokens(Math.round(m.total_tokens / m.call_count)) : '-'}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
          {#if models.length === 0}
            <div class="empty-state">暂无模型数据</div>
          {/if}
        </div>

      {:else if activeTab === 'modes'}
        <div class="section">
          <h2 class="section-title">模式使用统计</h2>
          <div class="mode-grid">
            {#each modes as m}
              <div class="mode-card">
                <div class="mode-header">
                  <span class="material-symbols-outlined text-[24px]" style="color: var(--color-primary)">{modeIcons[m.mode] || 'help'}</span>
                  <span class="mode-name">{modeLabels[m.mode] || m.mode}</span>
                </div>
                <div class="mode-stats">
                  <div class="mode-stat">
                    <span class="mode-stat-value">{m.conversation_count}</span>
                    <span class="mode-stat-label">对话数</span>
                  </div>
                  <div class="mode-stat">
                    <span class="mode-stat-value">{formatTokens(m.total_tokens)}</span>
                    <span class="mode-stat-label">Token</span>
                  </div>
                  <div class="mode-stat">
                    <span class="mode-stat-value">{m.model_count}</span>
                    <span class="mode-stat-label">模型数</span>
                  </div>
                </div>
                <!-- Token bar -->
                <div class="mode-bar-wrap">
                  <div class="mode-bar" style="width: {maxTokenUsage > 0 ? (m.total_tokens / maxTokenUsage * 100) : 0}%"></div>
                </div>
              </div>
            {/each}
          </div>
          {#if modes.length === 0}
            <div class="empty-state">暂无模式数据</div>
          {/if}
        </div>

      {:else if activeTab === 'timeline'}
        <div class="section">
          <h2 class="section-title">使用量趋势（近 {timelineDays} 天）</h2>
          {#if timeline.length > 0}
            <div class="timeline-chart">
              {#each timeline as entry}
                <div class="timeline-bar-col">
                  <div class="timeline-bar-wrap">
                    <div
                      class="timeline-bar"
                      style="height: {maxTokenUsage > 0 ? ((entry.token_usage || 0) / maxTokenUsage * 100) : 0}%"
                      title="{formatTokens(entry.token_usage || 0)} tokens"
                    ></div>
                  </div>
                  <span class="timeline-date">{formatDate(entry.date)}</span>
                  <span class="timeline-value">{formatTokens(entry.token_usage || 0)}</span>
                </div>
              {/each}
            </div>
          {:else}
            <div class="empty-state">暂无趋势数据</div>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .analytics-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--color-bg);
    overflow-y: auto;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 24px;
    border-bottom: 1px solid var(--color-border);
    background: var(--color-bg-elevated);
    flex-shrink: 0;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .back-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: 10px;
    background: transparent;
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: all 0.15s;
    border: 1px solid var(--color-border);
  }
  .back-btn:hover {
    background: var(--color-surface);
    color: var(--color-text);
  }

  .title {
    font-size: 18px;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }
  .subtitle {
    font-size: 12px;
    color: var(--color-text-muted);
    margin: 2px 0 0;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .filter-select {
    padding: 6px 10px;
    border-radius: 8px;
    border: 1px solid var(--color-border);
    background: var(--color-surface);
    color: var(--color-text);
    font-size: 13px;
    cursor: pointer;
    outline: none;
  }
  .filter-select:focus {
    border-color: var(--color-primary);
  }

  .refresh-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: 10px;
    border: 1px solid var(--color-border);
    background: transparent;
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: all 0.15s;
  }
  .refresh-btn:hover { background: var(--color-surface); color: var(--color-text); }
  .refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 80px 0;
    color: var(--color-text-muted);
    font-size: 14px;
  }

  .spinner-lg {
    width: 32px;
    height: 32px;
    border: 3px solid var(--color-border);
    border-top-color: var(--color-primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  /* Tabs */
  .tabs {
    display: flex;
    gap: 2px;
    padding: 12px 24px 0;
    border-bottom: 1px solid var(--color-border);
    flex-shrink: 0;
    overflow-x: auto;
  }

  .tab-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 10px 16px;
    border: none;
    border-bottom: 2px solid transparent;
    background: transparent;
    color: var(--color-text-muted);
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
    white-space: nowrap;
    min-height: 42px;
  }
  .tab-btn:hover {
    color: var(--color-text-secondary);
    background: var(--color-surface);
  }
  .tab-btn.active {
    color: var(--color-primary);
    border-bottom-color: var(--color-primary);
  }

  /* Content */
  .content {
    flex: 1;
    padding: 20px 24px;
    overflow-y: auto;
  }

  .section {
    margin-bottom: 28px;
  }
  .section-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--color-text-secondary);
    margin: 0 0 12px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  /* Overview grid */
  .overview-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 12px;
    margin-bottom: 20px;
  }
  .two-col {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
  }
  @media (max-width: 768px) {
    .two-col { grid-template-columns: 1fr; }
  }

  .stat-card {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 18px 16px;
    border-radius: 14px;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    transition: border-color 0.2s;
  }
  .stat-card:hover {
    border-color: color-mix(in srgb, var(--color-primary) 30%, transparent);
  }
  .stat-card.wide {
    min-width: 0;
  }

  .stat-icon {
    width: 44px;
    height: 44px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .stat-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .stat-value {
    font-size: 22px;
    font-weight: 700;
    color: var(--color-text);
    line-height: 1.2;
  }
  .stat-label {
    font-size: 12px;
    color: var(--color-text-muted);
  }

  /* Mini list */
  .mini-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .mini-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
  }
  .mini-name {
    flex: 1;
    font-size: 13px;
    color: var(--color-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mini-value {
    font-size: 13px;
    font-weight: 600;
    color: var(--color-primary);
    flex-shrink: 0;
  }

  /* Table */
  .table-wrap {
    overflow-x: auto;
    border-radius: 12px;
    border: 1px solid var(--color-border);
    background: var(--color-bg-elevated);
  }

  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  .data-table th {
    text-align: left;
    padding: 12px 16px;
    font-weight: 600;
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-text-muted);
    border-bottom: 1px solid var(--color-border);
    white-space: nowrap;
  }
  .data-table td {
    padding: 12px 16px;
    border-bottom: 1px solid var(--color-border-subtle);
    color: var(--color-text-secondary);
    white-space: nowrap;
  }
  .data-table tr:last-child td {
    border-bottom: none;
  }
  .data-table tr:hover td {
    background: var(--color-surface);
  }

  .rank {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 700;
    color: var(--color-text-muted);
    background: var(--color-surface);
  }
  .rank-top {
    background: var(--color-primary-light);
    color: var(--color-primary);
  }

  .cell-user {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .user-avatar {
    width: 28px;
    height: 28px;
    border-radius: 8px;
    background: var(--gradient-brand-subtle);
    color: var(--color-primary);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 700;
    flex-shrink: 0;
  }

  .cell-tokens {
    font-weight: 600;
    color: var(--color-primary);
  }

  .cell-model {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px;
  }

  .empty-state {
    text-align: center;
    padding: 48px 0;
    color: var(--color-text-muted);
    font-size: 14px;
  }

  /* Mode grid */
  .mode-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 12px;
  }

  .mode-card {
    padding: 18px 16px;
    border-radius: 14px;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    transition: border-color 0.2s;
  }
  .mode-card:hover {
    border-color: color-mix(in srgb, var(--color-primary) 30%, transparent);
  }

  .mode-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 14px;
  }
  .mode-name {
    font-size: 14px;
    font-weight: 600;
    color: var(--color-text);
  }

  .mode-stats {
    display: flex;
    gap: 20px;
    margin-bottom: 12px;
  }
  .mode-stat {
    display: flex;
    flex-direction: column;
  }
  .mode-stat-value {
    font-size: 16px;
    font-weight: 700;
    color: var(--color-text);
  }
  .mode-stat-label {
    font-size: 11px;
    color: var(--color-text-muted);
  }

  .mode-bar-wrap {
    height: 4px;
    border-radius: 2px;
    background: var(--color-surface);
    overflow: hidden;
  }
  .mode-bar {
    height: 100%;
    border-radius: 2px;
    background: var(--gradient-brand);
    transition: width 0.5s cubic-bezier(0.4, 0, 0.2, 1);
    min-width: 2px;
  }

  /* Timeline chart */
  .timeline-chart {
    display: flex;
    align-items: flex-end;
    gap: 2px;
    height: 220px;
    padding: 0 4px;
    border-radius: 12px;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    padding: 16px 12px 40px;
    position: relative;
    overflow-x: auto;
  }

  .timeline-bar-col {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    min-width: 28px;
    flex: 1;
  }

  .timeline-bar-wrap {
    width: 100%;
    height: 140px;
    display: flex;
    align-items: flex-end;
    justify-content: center;
  }

  .timeline-bar {
    width: 70%;
    max-width: 24px;
    border-radius: 4px 4px 0 0;
    background: var(--gradient-brand);
    transition: height 0.5s cubic-bezier(0.4, 0, 0.2, 1);
    min-height: 2px;
    cursor: pointer;
  }
  .timeline-bar:hover {
    opacity: 0.85;
  }

  .timeline-date {
    font-size: 10px;
    color: var(--color-text-muted);
    white-space: nowrap;
    position: absolute;
    bottom: 12px;
  }

  .timeline-value {
    font-size: 10px;
    color: var(--color-text-muted);
    white-space: nowrap;
  }

  /* Responsive */
  @media (max-width: 768px) {
    .header { padding: 12px 16px; }
    .content { padding: 16px; }
    .tabs { padding: 8px 16px 0; }
    .overview-grid { grid-template-columns: repeat(2, 1fr); }
    .mode-grid { grid-template-columns: 1fr; }
    .data-table { font-size: 12px; }
    .data-table th, .data-table td { padding: 8px 10px; }
  }
</style>

<script lang="ts">
  let {
    inputPricePerM = 0,
    outputPricePerM = 0,
  }: {
    inputPricePerM: number;
    outputPricePerM: number;
  } = $props();

  let collapsed = $state(true);
  let metrics: any = $state(null);
  let daily: any[] = $state([]);
  let loading = $state(false);

  async function refresh() {
    if (loading) return;
    loading = true;
    try {
      const res = await fetch('/api/v1/agent/metrics', { headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` } });
      if (res.ok) {
        const data = await res.json();
        metrics = data.metrics || {};
        daily = data.daily || [];
      }
    } catch { /* silent */ }
    loading = false;
  }

  $effect(() => { refresh(); });

  function fmtMs(ms: number | undefined): string {
    if (ms == null) return '-';
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  }

  function fmtTokens(t: number | undefined): string {
    if (t == null) return '-';
    if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(2)}M`;
    if (t >= 1000) return `${(t / 1000).toFixed(1)}K`;
    return `${t}`;
  }

  // Estimated cost: avg(input, output) price applied to total tokens (approx).
  function estimatedCost(tokens?: number): string {
    const t = tokens ?? metrics?.llm_token_usage;
    if (!t) return '-';
    const avgPrice = (inputPricePerM + outputPricePerM) / 2;
    if (avgPrice <= 0) return '免费模型';
    const cost = (t / 1_000_000) * avgPrice;
    return `$${cost.toFixed(4)}`;
  }

  function todayTokens(): number {
    if (daily.length === 0) return 0;
    const last = daily[daily.length - 1];
    const today = new Date().toISOString().slice(0, 10);
    return last.date === today ? (last.llm_token_usage || 0) : 0;
  }

  function todayCalls(): number {
    if (daily.length === 0) return 0;
    const last = daily[daily.length - 1];
    const today = new Date().toISOString().slice(0, 10);
    return last.date === today ? (last.llm_call_count || 0) : 0;
  }

  function maxDailyTokens(): number {
    let m = 0;
    for (const d of daily) m = Math.max(m, d.llm_token_usage || 0);
    return m || 1;
  }

  function shortDate(date: string): string {
    return date.slice(5); // MM-DD
  }
</script>

<div class="metrics-wrap">
  <button class="metrics-toggle" onclick={() => { if (collapsed) refresh(); collapsed = !collapsed; }}>
    <span class="icon">📊</span>
    <span>AI 统计</span>
    {#if daily.length > 0}
      <span class="badge">今日 {todayCalls()} 次 · {fmtTokens(todayTokens())} tokens · {estimatedCost(todayTokens())}</span>
    {/if}
    <span class="chevron">{collapsed ? '▸' : '▾'}</span>
  </button>
  {#if !collapsed}
    <div class="metrics-body">
      <div class="metric-grid">
        <div class="metric"><span class="label">LLM 调用</span><span class="value">{metrics?.llm_call_count ?? '-'}</span></div>
        <div class="metric"><span class="label">Token 用量</span><span class="value">{fmtTokens(metrics?.llm_token_usage)}</span></div>
        <div class="metric"><span class="label">估算成本</span><span class="value">{estimatedCost()}</span></div>
        <div class="metric"><span class="label">LLM 总耗时</span><span class="value">{fmtMs(metrics?.llm_total_duration)}</span></div>
        <div class="metric"><span class="label">平均耗时</span><span class="value">{fmtMs(metrics?.llm_avg_duration)}</span></div>
        <div class="metric"><span class="label">P95 耗时</span><span class="value">{fmtMs(metrics?.llm_p95_duration)}</span></div>
        <div class="metric"><span class="label">工具调用</span><span class="value">{metrics?.tool_call_count ?? '-'}</span></div>
        <div class="metric"><span class="label">迭代次数</span><span class="value">{metrics?.iteration_count ?? '-'}</span></div>
        <div class="metric"><span class="label">错误</span><span class="value err">{metrics?.error_count ?? '-'}</span></div>
        <div class="metric"><span class="label">重试</span><span class="value retry">{metrics?.retry_count ?? '-'}</span></div>
      </div>
      {#if daily.length > 0}
        <div class="daily-block">
          <div class="daily-title">近 {daily.length} 天用量（持久化，重启保留）</div>
          <div class="daily-chart">
            {#each daily as d (d.date)}
              <div class="daily-bar-wrap" title="{d.date} · {d.llm_call_count} 次调用 · {fmtTokens(d.llm_token_usage)} tokens">
                <div class="daily-bar" style="height: {Math.max(6, Math.round((d.llm_token_usage || 0) / maxDailyTokens() * 48))}px"></div>
                <span class="daily-label">{shortDate(d.date)}</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}
      <div class="metrics-foot">
        <span class="hint">进程累计统计（重启清零）</span>
        <button class="refresh-btn" onclick={() => refresh()}>{loading ? '刷新中…' : '刷新'}</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .metrics-wrap { margin: 0 16px 4px; }
  .metrics-toggle {
    display: flex; align-items: center; gap: 8px;
    width: 100%; padding: 6px 12px; border-radius: 8px;
    background: var(--color-bg-elevated, rgba(127,127,127,0.08));
    border: 1px solid var(--color-border, rgba(127,127,127,0.18));
    color: var(--color-text, inherit);
    font-size: 12px; cursor: pointer;
  }
  .metrics-toggle:hover { background: var(--color-bg-hover, rgba(127,127,127,0.14)); }
  .badge { color: var(--color-text-muted, #888); font-size: 11px; }
  .chevron { margin-left: auto; color: var(--color-text-muted, #888); }
  .metrics-body {
    margin-top: 4px; padding: 10px 12px; border-radius: 8px;
    background: var(--color-bg-elevated, rgba(127,127,127,0.06));
    border: 1px solid var(--color-border, rgba(127,127,127,0.15));
  }
  .metric-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(110px, 1fr)); gap: 8px; }
  .metric { display: flex; flex-direction: column; gap: 2px; padding: 6px 8px; border-radius: 6px; background: rgba(127,127,127,0.06); }
  .label { font-size: 10px; color: var(--color-text-muted, #888); }
  .value { font-size: 13px; font-weight: 600; }
  .value.err { color: #e5484d; }
  .value.retry { color: #f5a623; }
  .metrics-foot { display: flex; align-items: center; justify-content: space-between; margin-top: 8px; }
  .hint { font-size: 10px; color: var(--color-text-muted, #888); }
  .refresh-btn {
    font-size: 11px; padding: 2px 10px; border-radius: 6px; cursor: pointer;
    background: var(--color-bg-hover, rgba(127,127,127,0.12)); border: 1px solid var(--color-border, rgba(127,127,127,0.2));
    color: var(--color-text, inherit);
  }
  .daily-block { margin-top: 10px; }
  .daily-title { font-size: 10px; color: var(--color-text-muted, #888); margin-bottom: 6px; }
  .daily-chart { display: flex; align-items: flex-end; gap: 6px; overflow-x: auto; padding-bottom: 2px; }
  .daily-bar-wrap { display: flex; flex-direction: column; align-items: center; gap: 2px; flex-shrink: 0; }
  .daily-bar {
    width: 22px; border-radius: 4px 4px 0 0;
    background: linear-gradient(180deg, var(--color-primary, #4f8cff), color-mix(in srgb, var(--color-primary, #4f8cff) 45%, transparent));
    min-height: 4px;
  }
  .daily-label { font-size: 9px; color: var(--color-text-muted, #888); }
</style>

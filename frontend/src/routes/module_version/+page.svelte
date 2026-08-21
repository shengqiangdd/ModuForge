<script lang="ts">
  import { onMount } from 'svelte';

  interface ModuleVersion {
    id: number;
    module_id: number;
    module_name: string;
    version: string;
    changelog: string;
    status: string;
    created_at: string;
  }

  let versions = $state<ModuleVersion[]>([]);
  let loading = $state(true);
  let selected = $state<ModuleVersion | null>(null);
  let rollingBack = $state(false);

  async function loadVersions() {
    loading = true;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/module-versions', {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) versions = await res.json();
    } catch (e) { console.error(e); }
    loading = false;
  }

  async function rollback(v: ModuleVersion) {
    if (!confirm(`确定回滚到版本 ${v.version}？`)) return;
    rollingBack = true;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/module-versions/rollback/${v.id}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) {
        alert('回滚成功');
        await loadVersions();
        selected = null;
      }
    } catch (e) { console.error(e); }
    rollingBack = false;
  }

  function statusColor(status: string) {
    switch (status) {
      case 'active': return '#22c55e';
      case 'deprecated': return '#f59e0b';
      case 'yanked': return '#ef4444';
      default: return '#6b7280';
    }
  }

  function timeAgo(date: string) {
    const d = new Date(date);
    const now = new Date();
    const diff = Math.floor((now.getTime() - d.getTime()) / 1000);
    if (diff < 60) return '刚刚';
    if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`;
    if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`;
    return `${Math.floor(diff / 86400)} 天前`;
  }

  onMount(loadVersions);
</script>

<div class="page">
  <div class="header">
    <h1>版本管理</h1>
    <button class="btn-secondary" onclick={loadVersions}>刷新</button>
  </div>

  {#if loading}
    <div class="empty">加载中...</div>
  {:else if versions.length === 0}
    <div class="empty">
      <span class="empty-icon">📦</span>
      <p>暂无版本记录</p>
    </div>
  {:else}
    <div class="layout">
      <div class="list">
        {#each versions as v}
          <div
            class="item"
            class:active={selected?.id === v.id}
            onclick={() => selected = v}
          >
            <div class="item-header">
              <span class="version">{v.version}</span>
              <span class="status" style="color: {statusColor(v.status)}">{v.status}</span>
            </div>
            <div class="item-meta">
              <span>{v.module_name}</span>
              <span>{timeAgo(v.created_at)}</span>
            </div>
          </div>
        {/each}
      </div>

      <div class="detail">
        {#if selected}
          <div class="detail-header">
            <h2>{selected.version}</h2>
            <span class="status-badge" style="background: {statusColor(selected.status)}20; color: {statusColor(selected.status)}">{selected.status}</span>
          </div>
          <div class="detail-meta">
            <div><strong>模块：</strong>{selected.module_name}</div>
            <div><strong>发布时间：</strong>{new Date(selected.created_at).toLocaleString()}</div>
          </div>
          <div class="changelog">
            <h3>变更日志</h3>
            <p>{selected.changelog || '暂无变更日志'}</p>
          </div>
          {#if selected.status === 'active'}
            <button class="btn-danger" onclick={() => rollback(selected)} disabled={rollingBack}>
              {rollingBack ? '回滚中...' : '回滚到此版本'}
            </button>
          {/if}
        {:else}
          <div class="empty-detail">选择一个版本查看详情</div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .page { padding: 1.5rem; max-width: 1200px; margin: 0 auto; }
  .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; }
  .header h1 { font-size: 1.5rem; color: var(--text-primary); }
  .btn-secondary { padding: 0.4rem 1rem; border: 1px solid var(--border); border-radius: 8px; background: transparent; cursor: pointer; color: var(--text-secondary); }
  .btn-danger { padding: 0.5rem 1.2rem; border: none; border-radius: 8px; background: #ef4444; color: white; cursor: pointer; font-weight: 600; margin-top: 1rem; }
  .btn-danger:disabled { opacity: 0.6; }
  .layout { display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; min-height: 400px; }
  .list { border: 1px solid var(--border); border-radius: 10px; overflow: hidden; }
  .item { padding: 0.8rem 1rem; border-bottom: 1px solid var(--border); cursor: pointer; transition: background 0.15s; }
  .item:last-child { border-bottom: none; }
  .item:hover { background: var(--bg-hover); }
  .item.active { background: var(--primary)10; border-left: 3px solid var(--primary); }
  .item-header { display: flex; justify-content: space-between; align-items: center; }
  .version { font-weight: 700; color: var(--text-primary); font-family: monospace; }
  .status { font-size: 0.8rem; font-weight: 600; }
  .item-meta { display: flex; justify-content: space-between; font-size: 0.8rem; color: var(--text-tertiary); margin-top: 4px; }
  .detail { border: 1px solid var(--border); border-radius: 10px; padding: 1.5rem; background: var(--bg-card); }
  .detail-header { display: flex; justify-content: space-between; align-items: center; }
  .detail-header h2 { margin: 0; color: var(--text-primary); font-family: monospace; }
  .status-badge { padding: 0.2rem 0.6rem; border-radius: 12px; font-size: 0.8rem; font-weight: 600; }
  .detail-meta { margin-top: 1rem; display: flex; flex-direction: column; gap: 0.4rem; font-size: 0.9rem; color: var(--text-secondary); }
  .changelog { margin-top: 1.5rem; }
  .changelog h3 { font-size: 0.9rem; color: var(--text-secondary); margin-bottom: 0.5rem; }
  .changelog p { color: var(--text-primary); white-space: pre-wrap; }
  .empty-detail { display: flex; align-items: center; justify-content: center; height: 100%; color: var(--text-tertiary); }
  .empty { text-align: center; padding: 3rem; color: var(--text-secondary); }
  .empty-icon { font-size: 3rem; display: block; margin-bottom: 1rem; }

  @media (max-width: 768px) {
    .layout { grid-template-columns: 1fr; }
  }
</style>

<script>
  import { onMount } from 'svelte';
  
  let status = $state(null);
  let logs = $state([]);
  let loading = $state(false);
  let error = $state(null);
  let commitMessage = $state('');
  let selectedFiles = $state(new Set());
  
  async function fetchStatus() {
    try {
      const res = await fetch('/api/git/status');
      if (res.ok) status = await res.json();
    } catch (e) { console.error('Failed to fetch git status:', e); }
  }
  
  async function fetchLogs() {
    try {
      const res = await fetch('/api/git/log?n=20');
      if (res.ok) logs = await res.json();
    } catch (e) { console.error('Failed to fetch git logs:', e); }
  }
  
  async function refresh() {
    loading = true;
    await Promise.all([fetchStatus(), fetchLogs()]);
    loading = false;
  }
  
  async function commit() {
    if (!commitMessage.trim()) return;
    loading = true;
    error = null;
    try {
      const res = await fetch('/api/git/commit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: commitMessage, files: Array.from(selectedFiles) })
      });
      if (res.ok) { commitMessage = ''; selectedFiles = new Set(); await refresh(); }
      else { error = await res.text(); }
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  async function rollback(commitHash, hard = false) {
    if (!confirm(hard ? '确定要硬回滚吗？' : '确定要软回滚吗？')) return;
    loading = true;
    try {
      const res = await fetch('/api/git/rollback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ commit: commitHash, hard })
      });
      if (res.ok) await refresh();
      else error = await res.text();
    } catch (e) { error = e.message; }
    loading = false;
  }
  
  function toggleFile(file) {
    if (selectedFiles.has(file)) selectedFiles.delete(file);
    else selectedFiles.add(file);
    selectedFiles = selectedFiles;
  }
  
  function formatDate(dateStr) { return new Date(dateStr).toLocaleString('zh-CN'); }
  
  onMount(refresh);
</script>

<div class="git-panel">
  <div class="flex items-center justify-between mb-4">
    <h2 class="text-xl font-semibold">📝 Git 操作</h2>
    <button onclick={refresh} disabled={loading} class="px-3 py-1 text-sm bg-gray-100 rounded hover:bg-gray-200 disabled:opacity-50">刷新</button>
  </div>
  
  {#if error}<div class="text-red-500 mb-4 p-2 bg-red-50 rounded">{error}</div>{/if}
  
  {#if status}
    <div class="card mb-4">
      <h3 class="card-title">提交更改</h3>
      {#if status.clean}
        <div class="text-gray-500 text-sm">工作区干净，没有更改</div>
      {:else}
        {#if status.staged?.length > 0}
          <div class="mb-3">
            <div class="text-sm font-medium text-green-600 mb-2">已暂存 ({status.staged.length})</div>
            {#each status.staged as file}
              <label class="flex items-center gap-2 py-1 cursor-pointer"><input type="checkbox" checked onchange={() => toggleFile(file.file)} /><span class="text-sm font-mono">{file.file}</span><span class="text-xs text-gray-500">{file.status}</span></label>
            {/each}
          </div>
        {/if}
        {#if status.unstaged?.length > 0}
          <div class="mb-3">
            <div class="text-sm font-medium text-yellow-600 mb-2">未暂存 ({status.unstaged.length})</div>
            {#each status.unstaged as file}
              <label class="flex items-center gap-2 py-1 cursor-pointer"><input type="checkbox" onchange={() => toggleFile(file.file)} /><span class="text-sm font-mono">{file.file}</span><span class="text-xs text-gray-500">{file.status}</span></label>
            {/each}
          </div>
        {/if}
        {#if status.untracked?.length > 0}
          <div class="mb-3">
            <div class="text-sm font-medium text-gray-500 mb-2">未跟踪 ({status.untracked.length})</div>
            {#each status.untracked as file}
              <label class="flex items-center gap-2 py-1 cursor-pointer"><input type="checkbox" onchange={() => toggleFile(file)} /><span class="text-sm font-mono">{file}</span></label>
            {/each}
          </div>
        {/if}
        <div class="flex gap-2 mt-4">
          <input type="text" bind:value={commitMessage} placeholder="提交消息..." class="flex-1 px-3 py-2 border rounded" onkeydown={(e) => e.key === 'Enter' && commit()} />
          <button onclick={commit} disabled={loading || !commitMessage.trim()} class="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600 disabled:opacity-50">提交</button>
        </div>
      {/if}
    </div>
    
    <div class="card mb-4">
      <div class="flex items-center gap-2"><span class="text-sm text-gray-500">当前分支:</span><span class="font-mono text-sm font-medium">{status.branch}</span></div>
    </div>
  {/if}
  
  <div class="card">
    <h3 class="card-title">提交历史</h3>
    <div class="space-y-2 max-h-96 overflow-y-auto">
      {#each logs as commit}
        <div class="p-2 bg-gray-50 rounded hover:bg-gray-100">
          <div class="flex items-center gap-2 mb-1"><span class="font-mono text-xs text-gray-500">{commit.short_hash}</span><span class="text-sm font-medium">{commit.message}</span></div>
          <div class="flex items-center gap-4 text-xs text-gray-500">
            <span>{commit.author}</span><span>{formatDate(commit.date)}</span>
            <button onclick={() => rollback(commit.hash, false)} class="text-blue-500 hover:underline">软回滚</button>
            <button onclick={() => rollback(commit.hash, true)} class="text-red-500 hover:underline">硬回滚</button>
          </div>
        </div>
      {/each}
      {#if logs.length === 0}<div class="text-gray-500 text-sm text-center py-4">暂无提交历史</div>{/if}
    </div>
  </div>
</div>

<style>
  .git-panel { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

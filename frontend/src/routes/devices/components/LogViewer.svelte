<script lang="ts">
  import { apiGet, apiPost } from '../device-api';

  let { serial }: { serial: string } = $props();

  let logs = $state('');
  let logFilter = $state('');
  let logLevel = $state('');

  async function loadLogs() {
    if (!serial) return;
    const params = new URLSearchParams({ serial });
    if (logFilter) params.set('filter', logFilter);
    if (logLevel) params.set('level', logLevel);
    const d = await apiGet(`/api/v1/adb/logcat?${params}`);
    logs = d.logs || '';
  }

  async function clearLogs() {
    await apiPost('/api/v1/adb/logcat/clear', { serial });
    logs = '';
  }

  $effect(() => {
    if (serial) loadLogs();
  });
</script>

<div class="info-card overflow-hidden">
  <div class="p-4 border-b flex flex-wrap items-center gap-3" style="border-color: var(--color-border)">
    <input type="text" class="input-field text-xs flex-1" placeholder="Tag 过滤" bind:value={logFilter} />
    <select class="input-field text-xs py-1" bind:value={logLevel}>
      <option value="">全部</option>
      <option value="v">Verbose</option>
      <option value="d">Debug</option>
      <option value="i">Info</option>
      <option value="w">Warn</option>
      <option value="e">Error</option>
    </select>
    <button class="btn-primary text-xs" onclick={loadLogs}>查询</button>
    <button class="btn-ghost text-xs" onclick={clearLogs}>清空</button>
  </div>
  <pre class="p-4 text-xs overflow-auto max-h-[500px] font-mono" style="background: var(--color-surface); color: var(--color-text); white-space: pre-wrap; word-break: break-all">{logs || '无日志'}</pre>
</div>

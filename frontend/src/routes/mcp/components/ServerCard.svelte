<script lang="ts">
  import ToolItem from './ToolItem.svelte';

  interface MCPTool {
    name: string;
    description: string;
    inputSchema: Record<string, unknown>;
    writes: boolean;
  }

  interface MCPServer {
    id: number;
    name: string;
    url: string;
    headers: Record<string, string>;
    enabled: boolean;
    managed: boolean;
    ready: boolean;
    tool_count: number;
    tools: MCPTool[];
    server_name: string;
    connected_at: string;
    last_error: string;
  }

  interface Props {
    server: MCPServer;
    expanded: boolean;
    expandedTool: string | null;
    search: string;
    policies: Record<string, string>;
    policyBusy: boolean;
    onToggle: (name: string) => void;
    onToggleTool: (key: string) => void;
    onReconnect: (server: MCPServer) => void;
    onEdit: (server: MCPServer) => void;
    onDelete: (server: MCPServer) => void;
    onSetPolicy: (server: string, tool: MCPTool, mode: string) => void;
    onStartTest: (server: string, tool: string) => void;
  }

  let {
    server,
    expanded,
    expandedTool,
    search,
    policies,
    policyBusy,
    onToggle,
    onToggleTool,
    onReconnect,
    onEdit,
    onDelete,
    onSetPolicy,
    onStartTest
  }: Props = $props();

  function policyMode(serverName: string, tool: MCPTool): string {
    return policies[`${serverName}/${tool.name}`] || 'deny';
  }

  function filteredTools(s: MCPServer): MCPTool[] {
    const q = search.trim().toLowerCase();
    if (!q) return s.tools || [];
    return (s.tools || []).filter(t =>
      t.name.toLowerCase().includes(q) || (t.description || '').toLowerCase().includes(q)
    );
  }

  function fmtTime(iso: string): string {
    if (!iso || iso === '0001-01-01T00:00:00Z') return '—';
    try {
      return new Date(iso).toLocaleString();
    } catch {
      return iso;
    }
  }

  const toolList = $derived(filteredTools(server));
</script>

<div class="rounded-2xl border overflow-hidden transition-all" style="background: var(--color-bg-elevated); border-color: var(--color-border)">
  <!-- Server header -->
  <div
    class="w-full flex items-center gap-3 p-4 text-left"
    role="button"
    tabindex="0"
    onclick={() => onToggle(server.name)}
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onToggle(server.name); } }}
  >
    <div
      class="w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0"
      style="background: {server.ready ? 'var(--gradient-brand-subtle)' : 'color-mix(in srgb, var(--color-text-muted) 12%, transparent)'}"
    >
      <span class="material-symbols-outlined" style="color: {server.ready ? 'var(--color-primary)' : 'var(--color-error)'}">
        {server.ready ? 'check_circle' : 'error'}
      </span>
    </div>
    <div class="flex-1 min-w-0">
      <div class="flex items-center gap-2 flex-wrap">
        <span class="font-semibold" style="color: var(--color-text)">{server.name}</span>
        {#if !server.managed}
          <span class="badge text-[10px]" style="background: color-mix(in srgb, var(--color-text-muted) 12%, transparent); color: var(--color-text-muted)">环境变量</span>
        {/if}
        {#if server.ready}
          <span class="badge text-[10px]" style="background: var(--color-success-light); color: var(--color-success)">已连接</span>
        {:else}
          <span class="badge text-[10px]" style="background: var(--color-error-light); color: var(--color-error)">未连接</span>
        {/if}
        {#if server.server_name}
          <span class="badge text-[10px]" style="background: var(--color-primary-light); color: var(--color-primary)">{server.server_name}</span>
        {/if}
      </div>
      <p class="text-xs mt-0.5 truncate" style="color: var(--color-text-muted)">
        {server.tool_count} 个工具 · {server.url}
        {#if server.last_error}
          <span class="ml-1" style="color: var(--color-error)">⚠ {server.last_error}</span>
        {/if}
      </p>
    </div>
    <div class="flex items-center gap-1 flex-shrink-0" onclick={(e) => e.stopPropagation()}>
      {#if server.managed}
        <button
          class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] min-w-[30px] min-h-[30px] flex items-center justify-center"
          style="color: var(--color-text-muted)"
          title="重连"
          onclick={() => onReconnect(server)}
        >
          <span class="material-symbols-outlined text-[17px]">sync</span>
        </button>
        <button
          class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] min-w-[30px] min-h-[30px] flex items-center justify-center"
          style="color: var(--color-text-muted)"
          title="编辑"
          onclick={() => onEdit(server)}
        >
          <span class="material-symbols-outlined text-[17px]">edit</span>
        </button>
        <button
          class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] min-w-[30px] min-h-[30px] flex items-center justify-center"
          style="color: var(--color-error)"
          title="删除"
          onclick={() => onDelete(server)}
        >
          <span class="material-symbols-outlined text-[17px]">delete</span>
        </button>
      {/if}
    </div>
    <span class="material-symbols-outlined text-[20px] transition-transform duration-200" style="color: var(--color-text-muted)">
      {expanded ? 'expand_less' : 'expand_more'}
    </span>
  </div>

  <!-- Server detail -->
  {#if expanded}
    <div class="border-t px-4 py-4" style="border-color: var(--color-border)">
      {#if !server.ready}
        <div class="rounded-xl border p-3 mb-3 text-sm" style="border-color: var(--color-error); background: var(--color-error-light)">
          <p class="font-medium" style="color: var(--color-error)">连接失败</p>
          {#if server.last_error}
            <p class="text-xs mt-1 font-mono break-all" style="color: var(--color-error)">{server.last_error}</p>
          {/if}
        </div>
      {:else if server.connected_at}
        <p class="text-xs mb-3" style="color: var(--color-text-muted)">连接时间：{fmtTime(server.connected_at)}</p>
      {/if}

      {#if toolList.length === 0}
        <p class="text-sm" style="color: var(--color-text-muted)">
          {search ? `没有匹配 "${search}" 的工具` : '该服务器没有暴露任何工具'}
        </p>
      {:else}
        <div class="space-y-2">
          {#each toolList as tool}
            <ToolItem
              {tool}
              serverName={server.name}
              expanded={expandedTool === server.name + '/' + tool.name}
              policyMode={policyMode(server.name, tool)}
              {policyBusy}
              onToggle={onToggleTool}
              {onStartTest}
              {onSetPolicy}
            />
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

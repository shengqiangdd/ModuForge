<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { client } from '$lib/api/client';
  import McpHeader from './components/McpHeader.svelte';
  import McpConfigInfo from './components/McpConfigInfo.svelte';
  import ServerCard from './components/ServerCard.svelte';
  import ServerEditorModal from './components/ServerEditorModal.svelte';
  import TestToolModal from './components/TestToolModal.svelte';

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

  // ---- State ----
  let servers = $state<MCPServer[]>([]);
  let loading = $state(true);
  let error = $state('');
  let expandedServer = $state<string | null>(null);
  let expandedTool = $state<string | null>(null);
  let search = $state('');

  // Server editor modal
  let showEditor = $state(false);
  let editServer = $state<MCPServer | null>(null);
  let saving = $state(false);

  // Test tool modal
  let testServer = $state('');
  let testTool = $state('');

  // Permission policies
  let policies = $state<Record<string, string>>({});
  let policyBusy = $state(false);

  // ---- Data loading ----
  async function loadStatus() {
    loading = true;
    error = '';
    try {
      const data = await client.get<{ servers: MCPServer[] }>('/agent/mcp/servers');
      servers = data.servers || [];
    } catch (e: any) {
      error = e.message || '加载 MCP 状态失败';
    } finally {
      loading = false;
    }
  }

  async function loadPolicies() {
    try {
      const data = await client.get<{ policies: { server: string; tool: string; allow_auto: boolean; mode: string }[] }>('/agent/mcp/policies');
      const map: Record<string, string> = {};
      for (const p of data.policies || []) map[`${p.server}/${p.tool}`] = p.mode || (p.allow_auto ? 'allow' : 'deny');
      policies = map;
    } catch (e) { console.warn('Failed to load MCP policies:', e); }
  }

  // ---- Server management ----
  function openAdd() {
    editServer = null;
    showEditor = true;
  }

  function openEdit(s: MCPServer) {
    editServer = s;
    showEditor = true;
  }

  async function saveServer(data: { name: string; url: string; headers: Record<string, string>; enabled: boolean }) {
    saving = true;
    try {
      if (editServer) {
        await client.put(`/agent/mcp/servers/${encodeURIComponent(editServer.name)}`, data);
        toast('服务器已更新', 'success');
      } else {
        await client.post('/agent/mcp/servers', data);
        toast('服务器已添加', 'success');
      }
      showEditor = false;
      loadStatus();
    } catch (e: any) {
      toast(e.message || '保存失败', 'error');
    } finally {
      saving = false;
    }
  }

  async function deleteServer(s: MCPServer) {
    if (!confirm(`确定删除 MCP 服务器 "${s.name}"？`)) return;
    try {
      await client.del(`/agent/mcp/servers/${encodeURIComponent(s.name)}`);
      toast('服务器已删除', 'success');
      showEditor = false;
      loadStatus();
    } catch (e: any) {
      toast(e.message || '删除失败', 'error');
    }
  }

  async function reconnectServer(s: MCPServer) {
    try {
      const d = await client.post<{ ready: boolean; tool_count: number }>(`/agent/mcp/servers/${encodeURIComponent(s.name)}/reconnect`, {});
      toast(`重连成功：${d.tool_count} 个工具`, 'success');
      loadStatus();
    } catch (e: any) {
      toast(e.message || '重连失败', 'error');
      loadStatus();
    }
  }

  // ---- Tool management ----
  async function setPolicy(server: string, tool: MCPTool, mode: string) {
    policyBusy = true;
    try {
      await client.put(`/agent/mcp/policies/${encodeURIComponent(server)}/${encodeURIComponent(tool.name)}`, { mode });
      policies = { ...policies, [`${server}/${tool.name}`]: mode };
      const label = mode === 'allow' ? '自动允许' : mode === 'ask' ? '每次询问' : '拒绝';
      toast(`${tool.name}：${label}`, 'success');
    } catch (e: any) {
      toast(e.message || '策略保存失败', 'error');
    } finally {
      policyBusy = false;
    }
  }

  // ---- Expand/collapse ----
  function toggleServer(name: string) {
    expandedServer = expandedServer === name ? null : name;
    expandedTool = null;
  }

  function toggleTool(key: string) {
    expandedTool = expandedTool === key ? null : key;
  }

  // ---- Test modal ----
  function startTest(server: string, tool: string) {
    testServer = server;
    testTool = tool;
  }

  function closeTest() {
    testServer = '';
    testTool = '';
  }

  // ---- Lifecycle ----
  onMount(() => {
    loadStatus();
    loadPolicies();
  });
</script>

<svelte:head><title>MCP 服务器 · ModuForge</title></svelte:head>

<div class="p-4 md:p-6 max-w-7xl mx-auto page-enter w-full">
  <McpHeader
    {search}
    {loading}
    onSearch={(v) => search = v}
    onRefresh={loadStatus}
    onAdd={openAdd}
  />

  <McpConfigInfo />

  {#if loading}
    <div class="flex items-center justify-center py-16">
      <div class="animate-spin h-6 w-6 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
    </div>
  {:else if error}
    <div class="rounded-2xl border p-8 text-center" style="border-color: var(--color-error); background: var(--color-error-light)">
      <span class="material-symbols-outlined text-4xl mb-2 block" style="color: var(--color-error)">error_outline</span>
      <p class="font-medium" style="color: var(--color-error)">{error}</p>
    </div>
  {:else if servers.length === 0}
    <div class="text-center py-16">
      <span class="material-symbols-outlined text-5xl mb-3 block" style="color: var(--color-text-muted)">hub_off</span>
      <p class="font-medium" style="color: var(--color-text-secondary)">未配置 MCP 服务器</p>
      <p class="text-sm mt-1 mb-4" style="color: var(--color-text-muted)">点击「添加服务器」接入外部工具，或在环境中配置 MCP_SERVERS 后重启</p>
      <button class="btn-primary" onclick={openAdd}>添加服务器</button>
    </div>
  {:else}
    <div class="space-y-4">
      {#each servers as server (server.id)}
        <ServerCard
          {server}
          expanded={expandedServer === server.name}
          {expandedTool}
          {search}
          {policies}
          {policyBusy}
          onToggle={toggleServer}
          onToggleTool={toggleTool}
          onReconnect={reconnectServer}
          onEdit={openEdit}
          onDelete={deleteServer}
          onSetPolicy={setPolicy}
          onStartTest={startTest}
        />
      {/each}
    </div>
  {/if}
</div>

<ServerEditorModal
  show={showEditor}
  {editServer}
  {saving}
  onClose={() => showEditor = false}
  onSave={saveServer}
  onDelete={deleteServer}
/>

{#if testServer && testTool}
  <TestToolModal
    server={testServer}
    tool={testTool}
    onClose={closeTest}
  />
{/if}

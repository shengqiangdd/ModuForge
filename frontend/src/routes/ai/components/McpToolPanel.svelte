<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { client } from '$lib/api/client';

  interface MCPTool {
    name: string;
    description: string;
    inputSchema: Record<string, unknown>;
  }

  interface MCPServer {
    name: string;
    url: string;
    server_name: string;
    ready: boolean;
    tool_count: number;
    tools: MCPTool[];
  }

  let {
    show = false,
    onClose,
    onInsertTool,
    onToolCountChange,
    onNavigate,
  }: {
    show: boolean;
    onClose: () => void;
    onInsertTool: (text: string) => void;
    onToolCountChange?: (count: number) => void;
    onNavigate?: (route: string) => void;
  } = $props();

  let servers = $state<MCPServer[]>([]);
  let loading = $state(false);
  let loaded = $state(false);

  async function load() {
    if (!show) return;
    if (!loaded) loading = true;
    try {
      const data = await client.get<{ servers: MCPServer[] }>('/agent/mcp/status');
      servers = data.servers || [];
      const total = servers.reduce((acc, s) => acc + (s.tools?.length || 0), 0);
      onToolCountChange?.(total);
    } catch (e: any) {
      toast(e.message || '加载 MCP 工具失败', 'error');
    } finally {
      loading = false;
      loaded = true;
    }
  }

  function insertTool(server: string, tool: MCPTool) {
    const fullName = `mcp__${server}__${tool.name}`;
    onInsertTool(`请使用 MCP 工具 \`${fullName}\`（${tool.description || '无描述'}）\n\n调用时以 function-calling 形式使用该工具。`);
    toast(`已插入 ${fullName} 提示`, 'success');
  }

  $effect(() => {
    if (show) load();
  });

  onMount(() => { if (show) load(); });
</script>

{#if show}
  <div class="fixed inset-0 z-40" style="background: rgba(0,0,0,0.4); backdrop-filter: blur(2px)" role="presentation"
       onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
  </div>
  <aside class="fixed top-0 right-0 bottom-0 z-50 w-[360px] max-w-[92vw] flex flex-col shadow-2xl animate-[slideInRight_0.2s_ease-out]"
         style="background: var(--color-bg-elevated); border-left: 1px solid var(--color-border)">
    <div class="flex items-center gap-2 px-4 h-14 border-b flex-shrink-0" style="border-color: var(--color-border)">
      <span class="material-symbols-outlined" style="color: var(--color-primary)">hub</span>
      <h3 class="font-bold flex-1" style="color: var(--color-text)">MCP 工具</h3>
      <button class="p-2 rounded-lg hover:bg-[var(--color-surface)] min-w-[36px] min-h-[36px] flex items-center justify-center" style="color: var(--color-text-muted)" onclick={load} title="刷新">
        <span class="material-symbols-outlined text-[18px]">refresh</span>
      </button>
      <button class="p-2 rounded-lg hover:bg-[var(--color-surface)] min-w-[36px] min-h-[36px] flex items-center justify-center" style="color: var(--color-text-muted)" onclick={onClose} title="关闭">
        <span class="material-symbols-outlined text-[18px]">close</span>
      </button>
    </div>

    <div class="flex-1 overflow-y-auto p-3 space-y-3">
      {#if loading && !loaded}
        <div class="flex items-center justify-center py-10">
          <div class="animate-spin h-5 w-5 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
        </div>
      {:else if servers.length === 0}
        <div class="text-center py-10">
          <span class="material-symbols-outlined text-4xl mb-2 block" style="color: var(--color-text-muted)">hub_off</span>
          <p class="text-sm font-medium" style="color: var(--color-text-secondary)">未配置 MCP 服务器</p>
          <p class="text-xs mt-1 mb-3" style="color: var(--color-text-muted)">通过 MCP_SERVERS 环境变量接入外部工具</p>
          {#if onNavigate}
            <button class="btn-primary text-xs" onclick={() => { onClose(); onNavigate('mcp'); }}>
              前往 MCP 页面
            </button>
          {/if}
        </div>
      {:else}
        {#each servers as server}
          <div class="rounded-xl border overflow-hidden" style="border-color: var(--color-border)">
            <div class="flex items-center gap-2 px-3 py-2.5" style="background: var(--color-surface)">
              <span class="material-symbols-outlined text-[16px]" style="color: {server.ready ? 'var(--color-success)' : 'var(--color-error)'}">
                {server.ready ? 'check_circle' : 'error'}
              </span>
              <span class="text-sm font-semibold flex-1 truncate" style="color: var(--color-text)">{server.name}</span>
              <span class="badge text-[10px]" style="background: var(--color-primary-light); color: var(--color-primary)">{server.tool_count} 工具</span>
            </div>
            <div class="p-1.5 space-y-1">
              {#if server.tools.length === 0}
                <p class="text-xs px-2 py-1" style="color: var(--color-text-muted)">无工具</p>
              {:else}
                {#each server.tools as tool}
                  <button class="w-full flex items-start gap-2 px-2 py-1.5 rounded-lg text-left hover:bg-[var(--color-surface)] transition-colors group"
                          onclick={() => insertTool(server.name, tool)}>
                    <span class="material-symbols-outlined text-[16px] mt-0.5" style="color: var(--color-primary)">bolt</span>
                    <span class="min-w-0 flex-1">
                      <span class="block font-mono text-xs font-medium truncate" style="color: var(--color-text)">{tool.name}</span>
                      {#if tool.description}
                        <span class="block text-[11px] mt-0.5 line-clamp-2" style="color: var(--color-text-muted)">{tool.description}</span>
                      {/if}
                    </span>
                    <span class="material-symbols-outlined text-[14px] opacity-0 group-hover:opacity-100 transition-opacity" style="color: var(--color-text-muted)">add_circle</span>
                  </button>
                {/each}
              {/if}
            </div>
          </div>
        {/each}
      {/if}
    </div>

    <div class="p-3 border-t flex-shrink-0" style="border-color: var(--color-border)">
      <p class="text-[11px] leading-relaxed" style="color: var(--color-text-muted)">
        点击工具插入调用提示，AI 会以 function-calling 形式调用。工具名格式：<code>mcp__服务器__工具</code>
      </p>
    </div>
  </aside>
{/if}

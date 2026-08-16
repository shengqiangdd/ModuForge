<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { client, getToken } from '$lib/api/client';

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

  let servers = $state<MCPServer[]>([]);
  let loading = $state(true);
  let error = $state('');
  let expandedServer = $state<string | null>(null);
  let expandedTool = $state<string | null>(null);

  // Test call state
  let testServer = $state('');
  let testTool = $state('');
  let testArgs = $state('{}');
  let testResult = $state('');
  let testing = $state(false);

  async function loadStatus() {
    loading = true;
    error = '';
    try {
      const data = await client.get<{ servers: MCPServer[] }>('/agent/mcp/status');
      servers = data.servers || [];
    } catch (e: any) {
      error = e.message || '加载 MCP 状态失败';
    } finally {
      loading = false;
    }
  }

  function toggleServer(name: string) {
    expandedServer = expandedServer === name ? null : name;
    expandedTool = null;
  }

  function toggleTool(name: string) {
    expandedTool = expandedTool === name ? null : name;
  }

  function startTest(server: string, tool: string) {
    testServer = server;
    testTool = tool;
    testArgs = '{}';
    testResult = '';
  }

  async function runTest() {
    if (!testServer || !testTool) return;
    testing = true;
    testResult = '';
    let parsed: Record<string, unknown> = {};
    try {
      parsed = JSON.parse(testArgs || '{}');
    } catch {
      testResult = '❌ 参数不是合法 JSON';
      testing = false;
      return;
    }
    try {
      const data = await client.post<{ result: string }>('/agent/mcp/test', {
        server: testServer,
        tool: testTool,
        arguments: parsed,
      });
      testResult = data.result || '(空结果)';
    } catch (e: any) {
      testResult = '❌ ' + (e.message || '调用失败');
    } finally {
      testing = false;
    }
  }

  function formatSchema(schema: Record<string, unknown>): string {
    try {
      return JSON.stringify(schema, null, 2);
    } catch {
      return '(无 schema)';
    }
  }

  // Render schema properties as friendly parameter chips
  function schemaProps(schema: Record<string, unknown>): { name: string; type: string; required: boolean }[] {
    const props = (schema as any)?.properties || {};
    const required = (schema as any)?.required || [];
    return Object.entries(props).map(([name, v]: [string, any]) => ({
      name,
      type: typeof v === 'object' && v ? (v.type || 'any') : 'any',
      required: required.includes(name),
    }));
  }

  onMount(() => {
    loadStatus();
  });
</script>

<svelte:head><title>MCP 服务器 · ModuForge</title></svelte:head>

<div class="p-4 md:p-6 max-w-7xl mx-auto page-enter">
  <!-- Header -->
  <div class="flex items-center gap-3 mb-6">
    <div class="w-11 h-11 rounded-2xl flex items-center justify-center flex-shrink-0" style="background: var(--gradient-brand-subtle)">
      <span class="material-symbols-outlined" style="color: var(--color-primary)">hub</span>
    </div>
    <div class="flex-1">
      <h1 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">MCP 服务器</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">
        Model Context Protocol — 通过 MCP_SERVERS 环境变量接入外部工具
      </p>
    </div>
    <button class="btn-ghost flex items-center gap-1.5" onclick={loadStatus} disabled={loading}>
      <span class="material-symbols-outlined text-[18px]">refresh</span>
      刷新
    </button>
  </div>

  <!-- 配置说明 -->
  <div class="rounded-2xl border p-4 mb-6 text-sm" style="background: var(--color-bg-elevated); border-color: var(--color-border)">
    <div class="flex items-start gap-3">
      <span class="material-symbols-outlined text-[20px] mt-0.5" style="color: var(--color-primary)">info</span>
      <div class="space-y-1.5 min-w-0">
        <p class="font-medium" style="color: var(--color-text)">如何配置 MCP 服务器</p>
        <p style="color: var(--color-text-secondary)">通过环境变量注入（需重启服务生效）：</p>
        <pre class="rounded-xl p-3 overflow-x-auto text-xs font-mono" style="background: var(--color-surface); color: var(--color-text-secondary)"><code>MCP_SERVERS='[&#123;"name":"github","url":"http://host:8000/mcp","headers":&#123;"Authorization":"Bearer x"&#125;&#125;]'
MCP_SERVERS_FILE=/path/to/servers.json   # 或使用配置文件</code></pre>
        <p class="text-xs" style="color: var(--color-text-muted)">配置后 MCP 工具会自动注册为 <code>mcp__&lt;server&gt;__&lt;tool&gt;</code> 技能，AI 助手可直接调用。</p>
      </div>
    </div>
  </div>

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
      <p class="text-sm mt-1" style="color: var(--color-text-muted)">配置 MCP_SERVERS 环境变量后重启服务即可看到服务器与工具</p>
    </div>
  {:else}
    <div class="space-y-4">
      {#each servers as server}
        <div class="rounded-2xl border overflow-hidden transition-all" style="background: var(--color-bg-elevated); border-color: var(--color-border)">
          <!-- Server header -->
          <button class="w-full flex items-center gap-3 p-4 text-left hover:bg-[var(--color-surface)] transition-colors"
                  onclick={() => toggleServer(server.name)}>
            <div class="w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0"
                 style="background: {server.ready ? 'var(--gradient-brand-subtle)' : 'color-mix(in srgb, var(--color-text-muted) 12%, transparent)'}">
              <span class="material-symbols-outlined" style="color: {server.ready ? 'var(--color-primary)' : 'var(--color-text-muted)'}">
                {server.ready ? 'check_circle' : 'error'}
              </span>
            </div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="font-semibold" style="color: var(--color-text)">{server.name}</span>
                {#if server.ready}
                  <span class="badge text-[10px]" style="background: var(--color-success-light); color: var(--color-success)">已连接</span>
                {:else}
                  <span class="badge text-[10px]" style="background: var(--color-error-light); color: var(--color-error)">未连接</span>
                {/if}
              </div>
              <p class="text-xs mt-0.5 truncate" style="color: var(--color-text-muted)">
                {server.server_name} · {server.tool_count} 个工具 · {server.url}
              </p>
            </div>
            <span class="material-symbols-outlined text-[20px] transition-transform duration-200" style="color: var(--color-text-muted)">
              {expandedServer === server.name ? 'expand_less' : 'expand_more'}
            </span>
          </button>

          <!-- Server detail -->
          {#if expandedServer === server.name}
            <div class="border-t px-4 py-4" style="border-color: var(--color-border)">
              {#if server.tools.length === 0}
                <p class="text-sm" style="color: var(--color-text-muted)">该服务器没有暴露任何工具</p>
              {:else}
                <div class="space-y-2">
                  {#each server.tools as tool}
                    <div class="rounded-xl border overflow-hidden" style="border-color: var(--color-border)">
                      <div class="w-full flex items-center gap-2 p-3 text-left cursor-pointer hover:bg-[var(--color-surface)] transition-colors"
                              role="button" tabindex="0"
                              onclick={() => toggleTool(server.name + '/' + tool.name)}
                              onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleTool(server.name + '/' + tool.name); } }}>
                        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">bolt</span>
                        <span class="font-mono text-sm font-medium flex-1" style="color: var(--color-text)">{tool.name}</span>
                        <button class="btn-ghost text-xs px-2 py-1" onclick={(e) => { e.stopPropagation(); startTest(server.name, tool.name); }}>
                          测试
                        </button>
                        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-text-muted)">
                          {expandedTool === server.name + '/' + tool.name ? 'expand_less' : 'expand_more'}
                        </span>
                      </div>
                      {#if expandedTool === server.name + '/' + tool.name}
                        <div class="px-3 pb-3 space-y-3">
                          {#if tool.description}
                            <p class="text-sm" style="color: var(--color-text-secondary)">{tool.description}</p>
                          {/if}
                          {#if (schemaProps(tool.inputSchema || {})).length > 0}
                            <div>
                              <p class="text-xs font-medium mb-1.5" style="color: var(--color-text-muted)">参数</p>
                              <div class="flex flex-wrap gap-1.5">
                                {#each schemaProps(tool.inputSchema || {}) as p}
                                  <span class="badge text-[11px] font-mono" style="background: var(--color-primary-light); color: var(--color-primary)">
                                    {p.name}{p.required ? ' *' : ''}
                                  </span>
                                {/each}
                              </div>
                            </div>
                          {/if}
                          <details>
                            <summary class="text-xs cursor-pointer" style="color: var(--color-text-muted)">查看 JSON Schema</summary>
                            <pre class="mt-2 rounded-lg p-2 overflow-x-auto text-[11px] font-mono" style="background: var(--color-surface); color: var(--color-text-secondary)">{formatSchema(tool.inputSchema || {})}</pre>
                          </details>
                        </div>
                      {/if}
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Test tool modal -->
{#if testServer && testTool}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)"
       role="presentation" onclick={(e) => { if (e.target === e.currentTarget) { testServer = ''; testTool = ''; } }}>
    <div class="w-full max-w-lg rounded-2xl shadow-xl p-6 border animate-[scaleIn_0.2s_ease-out]"
         style="background: var(--color-bg-elevated); border-color: var(--color-border)" role="dialog" aria-modal="true">
      <div class="flex items-center gap-2 mb-4">
        <span class="material-symbols-outlined" style="color: var(--color-primary)">terminal</span>
        <h3 class="text-lg font-bold flex-1" style="color: var(--color-text)">测试工具调用</h3>
        <button class="p-2 rounded-lg hover:bg-[var(--color-surface)]" style="color: var(--color-text-muted)" onclick={() => { testServer = ''; testTool = ''; }}>✕</button>
      </div>
      <div class="space-y-3 mb-4">
        <div>
          <label class="block text-xs font-medium mb-1" style="color: var(--color-text-muted)">服务器 / 工具</label>
          <div class="flex items-center gap-2 font-mono text-sm" style="color: var(--color-text)">
            <span class="badge" style="background: var(--color-primary-light); color: var(--color-primary)">{testServer}</span>
            <span style="color: var(--color-text-muted)">→</span>
            <span class="badge" style="background: var(--color-success-light); color: var(--color-success)">{testTool}</span>
          </div>
        </div>
        <div>
          <label for="mcp-test-args" class="block text-xs font-medium mb-1" style="color: var(--color-text-muted)">参数 (JSON)</label>
          <textarea id="mcp-test-args" bind:value={testArgs} rows="4" class="input-field font-mono text-xs resize-none" placeholder={'{"owner":"octocat","repo":"hello-world"}'}></textarea>
        </div>
        {#if testResult}
          <div>
            <label class="block text-xs font-medium mb-1" style="color: var(--color-text-muted)">结果</label>
            <pre class="rounded-xl p-3 overflow-x-auto text-xs font-mono whitespace-pre-wrap" style="background: var(--color-surface); color: var(--color-text); max-height: 240px; overflow-y: auto">{testResult}</pre>
          </div>
        {/if}
      </div>
      <div class="flex justify-end gap-3">
        <button class="btn-ghost" onclick={() => { testServer = ''; testTool = ''; }}>关闭</button>
        <button class="btn-primary disabled:opacity-50" onclick={runTest} disabled={testing}>
          {testing ? '调用中...' : '调用'}
        </button>
      </div>
    </div>
  </div>
{/if}

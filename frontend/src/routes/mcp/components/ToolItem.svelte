<script lang="ts">
  interface MCPTool {
    name: string;
    description: string;
    inputSchema: Record<string, unknown>;
    writes: boolean;
  }

  interface Props {
    tool: MCPTool;
    serverName: string;
    expanded: boolean;
    policyMode: string;
    policyBusy: boolean;
    onToggle: (key: string) => void;
    onStartTest: (server: string, tool: string) => void;
    onSetPolicy: (server: string, tool: MCPTool, mode: string) => void;
  }

  let {
    tool,
    serverName,
    expanded,
    policyMode: mode,
    policyBusy,
    onToggle,
    onStartTest,
    onSetPolicy
  }: Props = $props();

  const toggleKey = $derived(serverName + '/' + tool.name);

  function formatSchema(schema: Record<string, unknown>): string {
    try {
      return JSON.stringify(schema, null, 2);
    } catch {
      return '(无 schema)';
    }
  }

  function schemaProps(schema: Record<string, unknown>): { name: string; type: string; required: boolean }[] {
    const props = (schema as any)?.properties || {};
    const required = (schema as any)?.required || [];
    return Object.entries(props).map(([name, v]: [string, any]) => ({
      name,
      type: typeof v === 'object' && v ? (v.type || 'any') : 'any',
      required: required.includes(name),
    }));
  }

  const permissionOptions = [
    { v: 'allow', label: '自动允许' },
    { v: 'ask', label: '每次询问' },
    { v: 'deny', label: '拒绝' }
  ];
</script>

<div class="rounded-xl border overflow-hidden" style="border-color: var(--color-border)">
  <div
    class="w-full flex items-center gap-2 p-3 text-left cursor-pointer hover:bg-[var(--color-surface)] transition-colors"
    role="button"
    tabindex="0"
    onclick={() => onToggle(toggleKey)}
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onToggle(toggleKey); } }}
  >
    <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">bolt</span>
    <span class="font-mono text-sm font-medium flex-1 truncate" style="color: var(--color-text)">{tool.name}</span>
    {#if tool.writes}
      <span class="badge text-[10px] flex-shrink-0" style="background: color-mix(in srgb, var(--color-warning) 14%, transparent); color: var(--color-warning)">写</span>
    {/if}
    <button
      class="btn-ghost text-xs px-2 py-1 flex-shrink-0"
      onclick={(e) => { e.stopPropagation(); onStartTest(serverName, tool.name); }}
    >
      测试
    </button>
    <span class="material-symbols-outlined text-[18px]" style="color: var(--color-text-muted)">
      {expanded ? 'expand_less' : 'expand_more'}
    </span>
  </div>

  {#if expanded}
    <div class="px-3 pb-3 space-y-3">
      {#if tool.description}
        <p class="text-sm" style="color: var(--color-text-secondary)">{tool.description}</p>
      {/if}

      {#if tool.writes}
        <div
          class="rounded-lg border p-2.5 space-y-2"
          style="border-color: color-mix(in srgb, var(--color-warning) 30%, transparent); background: color-mix(in srgb, var(--color-warning) 6%, transparent)"
        >
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-[16px]" style="color: var(--color-warning)">warning</span>
            <div class="flex-1 min-w-0">
              <p class="text-xs font-medium" style="color: var(--color-warning)">写操作 — AI 自动调用需授权</p>
              <p class="text-[11px] mt-0.5" style="color: var(--color-text-secondary)">「自动允许」AI 直接调用；「每次询问」AI 调用时弹出确认；「拒绝」阻止调用</p>
            </div>
          </div>
          <div class="flex gap-1.5 flex-wrap" role="radiogroup" aria-label="权限模式">
            {#each permissionOptions as opt}
              <button
                type="button"
                role="radio"
                aria-checked={mode === opt.v}
                disabled={policyBusy}
                onclick={() => onSetPolicy(serverName, tool, opt.v)}
                class="text-[11px] px-2.5 py-1 rounded-full border transition-colors"
                style={mode === opt.v
                  ? 'background: color-mix(in srgb, var(--color-warning) 18%, transparent); border-color: var(--color-warning); color: var(--color-warning); font-weight: 600;'
                  : 'border-color: var(--color-border); color: var(--color-text-secondary);'}
              >{opt.label}</button>
            {/each}
          </div>
        </div>
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
        <pre
          class="mt-2 rounded-lg p-2 overflow-x-auto text-[11px] font-mono"
          style="background: var(--color-surface); color: var(--color-text-secondary)"
        >{formatSchema(tool.inputSchema || {})}</pre>
      </details>
    </div>
  {/if}
</div>

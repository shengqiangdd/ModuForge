<script lang="ts">
  interface MCPServer {
    id: number;
    name: string;
    url: string;
    headers: Record<string, string>;
    enabled: boolean;
    managed: boolean;
    ready: boolean;
    tool_count: number;
    tools: any[];
    server_name: string;
    connected_at: string;
    last_error: string;
  }

  interface Props {
    show: boolean;
    editServer: MCPServer | null;
    saving: boolean;
    onClose: () => void;
    onSave: (data: { name: string; url: string; headers: Record<string, string>; enabled: boolean }) => void;
    onDelete: (server: MCPServer) => void;
  }

  let { show, editServer, saving, onClose, onSave, onDelete }: Props = $props();

  let formName = $state('');
  let formURL = $state('');
  let formHeaders = $state('{}');
  let formEnabled = $state(true);

  // Initialize form when editServer changes
  $effect(() => {
    if (editServer) {
      formName = editServer.name;
      formURL = editServer.url;
      formHeaders = JSON.stringify(editServer.headers || {}, null, 2);
      formEnabled = editServer.enabled;
    } else {
      formName = '';
      formURL = '';
      formHeaders = '{}';
      formEnabled = true;
    }
  });

  function handleSave() {
    if (!formName.trim() || !formURL.trim()) return;

    let headers: Record<string, string> = {};
    try {
      headers = JSON.parse(formHeaders || '{}');
    } catch {
      return;
    }

    onSave({
      name: formName.trim(),
      url: formURL.trim(),
      headers,
      enabled: formEnabled
    });
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)"
    role="presentation"
    onclick={handleBackdropClick}
  >
    <div
      class="w-full max-w-lg rounded-2xl shadow-xl p-6 border animate-[scaleIn_0.2s_ease-out]"
      style="background: var(--color-bg-elevated); border-color: var(--color-border)"
      role="dialog"
      aria-modal="true"
    >
      <div class="flex items-center gap-2 mb-4">
        <span class="material-symbols-outlined" style="color: var(--color-primary)">hub</span>
        <h3 class="text-lg font-bold flex-1" style="color: var(--color-text)">{editServer ? '编辑服务器' : '添加服务器'}</h3>
        <button class="p-2 rounded-lg hover:bg-[var(--color-surface)]" style="color: var(--color-text-muted)" onclick={onClose}>✕</button>
      </div>

      <div class="space-y-3 mb-4">
        <div>
          <label for="mcp-form-name" class="block text-xs font-medium mb-1" style="color: var(--color-text-muted)">名称 *</label>
          <input id="mcp-form-name" bind:value={formName} placeholder="github" class="input-field" />
          <p class="text-[11px] mt-1" style="color: var(--color-text-muted)">工具将注册为 <code>mcp__{formName || 'name'}__tool</code></p>
        </div>
        <div>
          <label for="mcp-form-url" class="block text-xs font-medium mb-1" style="color: var(--color-text-muted)">URL *（MCP 端点，Streamable HTTP）</label>
          <input id="mcp-form-url" bind:value={formURL} placeholder="http://localhost:8000/mcp" class="input-field font-mono" />
        </div>
        <div>
          <label for="mcp-form-headers" class="block text-xs font-medium mb-1" style="color: var(--color-text-muted)">Headers（JSON 对象，可选）</label>
          <textarea id="mcp-form-headers" bind:value={formHeaders} rows="3" class="input-field font-mono text-xs resize-none" placeholder='&#123;"Authorization":"Bearer xxx"&#125;'></textarea>
        </div>
        <label class="flex items-center gap-2 cursor-pointer select-none">
          <input type="checkbox" bind:checked={formEnabled} class="accent-[var(--color-primary)]" />
          <span class="text-sm" style="color: var(--color-text)">启用（保存后立即连接）</span>
        </label>
      </div>

      <div class="flex justify-end gap-3">
        {#if editServer}
          <button class="btn-ghost text-[var(--color-error)]" onclick={() => onDelete(editServer)} disabled={saving}>删除</button>
        {/if}
        <button class="btn-ghost" onclick={onClose} disabled={saving}>取消</button>
        <button
          class="btn-primary disabled:opacity-50"
          onclick={handleSave}
          disabled={saving || !formName.trim() || !formURL.trim()}
        >
          {saving ? '保存中...' : '保存'}
        </button>
      </div>
    </div>
  </div>
{/if}

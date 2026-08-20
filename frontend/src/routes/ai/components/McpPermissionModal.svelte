<!-- McpPermissionModal.svelte — MCP write-tool permission confirmation (ask mode) -->
<script lang="ts">
interface Props {
  permission: {
    request_id: string;
    server: string;
    tool: string;
    args: Record<string, unknown>;
    timeout_s: number;
  } | null;
  busy: boolean;
  argsPreview: string;
  onResolve: (allow: boolean) => void;
  onClose: () => void;
}

let { permission, busy, argsPreview, onResolve, onClose }: Props = $props();
</script>

{#if permission}
  <div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.55); backdrop-filter: blur(6px)">
    <div class="w-full max-w-md rounded-2xl border p-5 shadow-2xl" style="background: var(--color-surface); border-color: var(--color-border)">
      <div class="flex items-start gap-3">
        <span class="material-symbols-outlined text-[28px] flex-shrink-0" style="color: var(--color-warning)">shield_person</span>
        <div class="min-w-0 flex-1">
          <h3 class="text-base font-semibold" style="color: var(--color-text)">MCP 写操作确认</h3>
          <p class="text-xs mt-1" style="color: var(--color-text-secondary)">
            AI 请求调用 <span class="font-mono font-semibold" style="color: var(--color-warning)">{permission.tool}</span>
            {#if permission.server}（{permission.server}）{/if}，
            该工具会<strong>变更远端状态</strong>。
          </p>
        </div>
        <button class="btn-ghost flex-shrink-0" onclick={onClose} aria-label="关闭">✕</button>
      </div>
      <div class="mt-3 rounded-lg p-3 overflow-auto max-h-48 font-mono text-[11px]" style="background: var(--color-bg-elevated, rgba(127,127,127,0.07)); color: var(--color-text-secondary)">
        {argsPreview}
      </div>
      <div class="mt-4 flex gap-2 justify-end">
        <button class="px-4 py-2 rounded-lg text-sm font-medium border" disabled={busy}
                style="border-color: var(--color-border); color: var(--color-text-secondary)"
                onclick={() => onResolve(false)}>拒绝</button>
        <button class="px-4 py-2 rounded-lg text-sm font-semibold" disabled={busy}
                style="background: var(--color-warning); color: #fff"
                onclick={() => onResolve(true)}>允许本次调用</button>
      </div>
      <p class="text-[10px] mt-2 text-center" style="color: var(--color-text-muted)">{permission.timeout_s} 秒内未确认将自动拒绝；可在 MCP 页面设置「自动允许」</p>
    </div>
  </div>
{/if}

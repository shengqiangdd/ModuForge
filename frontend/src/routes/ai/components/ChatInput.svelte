<script lang="ts">
  import { onMount } from 'svelte';

  let {
    input = '',
    streaming = false,
    mode = 'generate' as string,
    messages = [] as any[],
    buildLog = '',
    analysisModes = [] as { id: string; label: string; icon: string; prompt: string }[],
    mcpToolCount = 0,
    onSend,
    onStop,
    onSendAnalysis,
    onBuildLogChange,
    onInputChange,
    onOpenMcpTools,
  }: {
    input?: string;
    streaming?: boolean;
    mode?: string;
    messages?: any[];
    buildLog?: string;
    analysisModes?: { id: string; label: string; icon: string; prompt: string }[];
    mcpToolCount?: number;
    onSend?: () => void;
    onStop?: () => void;
    onSendAnalysis?: (text: string, modeId: string) => void;
    onBuildLogChange?: (value: string) => void;
    onInputChange?: (value: string) => void;
    onOpenMcpTools?: () => void;
  } = $props();

  let textareaEl: HTMLTextAreaElement | undefined = $state();

  onMount(() => {
    // Focus the input on load so the user can start typing right away
    textareaEl?.focus();
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey) {
      e.preventDefault();
      onSend?.();
    }
  }
</script>

<div class="border-t border-[var(--color-border)] p-3 bg-[var(--color-bg-elevated)] ai-input-area">
  {#if mode === 'generate'}
    <div class="flex items-center gap-2 mb-2">
      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium" style="background: var(--color-primary-light); color: var(--color-primary)">
        <span class="material-symbols-outlined text-[12px]">hub</span>
        Universal · Magisk + KSU + APatch
      </span>
    </div>
  {/if}
  {#if mode === 'chat' && messages.length > 0}
    <div class="flex flex-wrap gap-1 mb-2">
      {#each analysisModes as am}
        <button
          class="flex items-center gap-1 px-2 py-1 rounded-lg text-[10px] font-medium transition-colors"
          style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)"
          onclick={() => {
            const lastUserMsg = [...messages].reverse().find((m: any) => m.role === 'user');
            onSendAnalysis?.(lastUserMsg?.content || '', am.id);
          }}
        >
          <span class="material-symbols-outlined text-[12px]">{am.icon}</span>
          {am.label}
        </button>
      {/each}
    </div>
  {/if}
  {#if mode === 'repair'}
    <textarea
      class="input-field text-xs font-mono resize-none mb-2"
      rows="2"
      placeholder="粘贴构建日志（可选）"
      value={buildLog}
      oninput={(e) => onBuildLogChange?.((e.target as HTMLTextAreaElement).value)}
    ></textarea>
  {/if}
  <div class="flex items-center gap-2 input-row">
    <button
      class="p-1.5 rounded-lg transition-colors relative flex-shrink-0"
      style="color: var(--color-text-secondary); background: var(--color-surface); border: 1px solid var(--color-border);"
      onclick={onOpenMcpTools}
      title="MCP 工具面板"
      aria-label="打开 MCP 工具面板"
    >
      <span class="material-symbols-outlined text-[16px]">hub</span>
      {#if mcpToolCount > 0}
        <span class="absolute -top-1 -right-1 text-[8px] font-bold px-1 py-0.5 rounded-full min-w-[14px] text-center"
              style="background: var(--color-success); color: white;">{mcpToolCount > 99 ? '99+' : mcpToolCount}</span>
      {/if}
    </button>
    <textarea
      class="flex-1 input-field resize-none"
      rows="2"
      style="min-height: 48px;"
      placeholder={mode === 'auto-build' ? '描述你想要创建的模块，AI 将自动完成开发全流程...' : mode === 'generate' ? '描述你的通用模块功能...' : mode === 'repair' ? '描述问题...' : mode === 'gather' ? '描述你的模块想法（如：我想做个电量管理模块）...' : '输入消息...'}
      value={input}
      oninput={(e) => onInputChange?.((e.target as HTMLTextAreaElement).value)}
      onkeydown={handleKeydown}
      bind:this={textareaEl}
    ></textarea>
    {#if streaming}
      <button class="p-1.5 rounded-lg transition-colors" onclick={onStop} style="background: var(--color-error-light); color: var(--color-error)" title="停止生成">
        <span class="material-symbols-outlined text-[16px]">stop_circle</span>
      </button>
    {:else}
      <button class="p-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={() => onSend?.()} disabled={!input.trim()} title="发送 (Enter)">
        <span class="material-symbols-outlined text-[16px]">send</span>
      </button>
    {/if}
  </div>
  <div class="flex items-center justify-between mt-1 px-0.5">
    <span class="text-[9px] text-[var(--color-text-muted)] opacity-60">Enter 发送 · Shift+Enter 换行</span>
    <span class="text-[9px] text-[var(--color-text-muted)] opacity-60" style="opacity: 0.45;">{input.length} 字符</span>
  </div>
</div>

<script lang="ts">
  let {
    input = '',
    streaming = false,
    mode = 'generate' as string,
    messages = [] as any[],
    buildLog = '',
    analysisModes = [] as { id: string; label: string; icon: string; prompt: string }[],
    onSend,
    onStop,
    onSendAnalysis,
    onBuildLogChange,
    onInputChange,
  }: {
    input?: string;
    streaming?: boolean;
    mode?: string;
    messages?: any[];
    buildLog?: string;
    analysisModes?: { id: string; label: string; icon: string; prompt: string }[];
    onSend?: () => void;
    onStop?: () => void;
    onSendAnalysis?: (text: string, modeId: string) => void;
    onBuildLogChange?: (value: string) => void;
    onInputChange?: (value: string) => void;
  } = $props();

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
    <textarea
      class="flex-1 input-field resize-none"
      rows="2"
      style="min-height: 48px;"
      placeholder={mode === 'auto-build' ? '描述你想要创建的模块，AI 将自动完成开发全流程...' : mode === 'generate' ? '描述你的通用模块功能...' : mode === 'repair' ? '描述问题...' : mode === 'gather' ? '描述你的模块想法（如：我想做个电量管理模块）...' : '输入消息...'}
      value={input}
      oninput={(e) => onInputChange?.((e.target as HTMLTextAreaElement).value)}
      onkeydown={handleKeydown}
    ></textarea>
    {#if streaming}
      <button class="p-1.5 rounded-lg transition-colors" onclick={onStop} style="background: var(--color-error-light); color: var(--color-error)">
        <span class="material-symbols-outlined text-[16px]">stop_circle</span>
      </button>
    {:else}
      <button class="p-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={() => onSend?.()} disabled={!input.trim()}>
        <span class="material-symbols-outlined text-[16px]">send</span>
      </button>
    {/if}
  </div>
</div>

<script lang="ts">
import { focusTrap } from '$lib/utils/focusTrap';
import type { ComparisonResult } from '../../lib/types';

let {
  show = false,
  results = [],
  running = false,
  input = '',
  onClose,
  onInputChange,
  onRun,
}: {
  show: boolean;
  results: ComparisonResult[];
  running: boolean;
  input: string;
  onClose: () => void;
  onInputChange: (v: string) => void;
  onRun: () => void;
} = $props();
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center" role="dialog" aria-modal="true" tabindex="-1" style="background: rgba(0,0,0,0.4)" onkeydown={(e) => { if (e.key === 'Escape') onClose(); }}>
    <div class="w-[90vw] max-w-2xl max-h-[80vh] rounded-xl shadow-2xl border border-[var(--color-border)] bg-[var(--color-bg)] overflow-hidden flex flex-col" tabindex="-1" use:focusTrap>
      <div class="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
        <span class="text-sm font-semibold text-[var(--color-text)]">多模型对比</span>
        <button class="p-1 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={onClose} aria-label="关闭">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-text-muted)">close</span>
        </button>
      </div>
      <div class="p-4 space-y-3 overflow-y-auto flex-1">
        <textarea class="input-field text-xs resize-none w-full" rows="3" placeholder="输入对比查询..." value={input} oninput={(e) => onInputChange((e.target as HTMLTextAreaElement).value)}></textarea>
        <button class="w-full px-4 py-2 rounded-lg text-sm font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={onRun} disabled={!input.trim() || running}>
          {running ? '运行中...' : '运行对比'}
        </button>
        {#each results as result}
          <div class="rounded-lg border border-[var(--color-border)] overflow-hidden">
            <div class="flex items-center gap-2 px-3 py-1.5 text-xs font-medium" style="background: var(--color-surface); border-bottom: 1px solid var(--color-border);">
              <span class="material-symbols-outlined text-[12px] text-primary-500">model_training</span>
              {result.model}
              <span class="ml-auto text-[var(--color-text-muted)]">{result.time}ms</span>
            </div>
            <pre class="p-3 text-xs leading-relaxed overflow-x-auto whitespace-pre-wrap" style="color: var(--color-text);">{result.response}</pre>
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}

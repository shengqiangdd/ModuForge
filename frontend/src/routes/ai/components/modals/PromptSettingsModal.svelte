<script lang="ts">
import type { Mode, AIPrompt } from '../../lib/types';
import { PROMPT_MODES } from '../../lib/types';

let {
  show = false,
  promptTab = 'generate',
  promptDraft = '',
  promptLoading = false,
  promptSaving = false,
  onClose,
  onTabChange,
  onDraftChange,
  onSave,
  onReset,
}: {
  show: boolean;
  promptTab: Mode;
  promptDraft: string;
  promptLoading: boolean;
  promptSaving: boolean;
  onClose: () => void;
  onTabChange: (mode: Mode) => void;
  onDraftChange: (v: string) => void;
  onSave: () => void;
  onReset: () => void;
} = $props();
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm prompt-modal-overlay" onclick={onClose}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-3xl max-h-[80vh] flex flex-col border border-[var(--color-border)]" onclick={(e) => e.stopPropagation()}>
      <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)]">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-primary-600">tune</span>
          <h2 class="text-lg font-semibold text-[var(--color-text)]">AI 提示词设置</h2>
        </div>
        <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <div class="flex items-center gap-1 px-6 pt-4 overflow-x-auto flex-nowrap" style="-webkit-overflow-scrolling: touch;">
        {#each PROMPT_MODES as m}
          <button
            class="flex items-center gap-1.5 px-3 py-2 rounded-xl text-sm font-medium transition-all whitespace-nowrap shrink-0
              {promptTab === m.value ? 'text-[var(--color-primary)]' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
            style={promptTab === m.value ? 'background: var(--color-primary-light)' : ''}
            onclick={() => onTabChange(m.value)}
          >
            <span class="material-symbols-outlined text-[14px]">{m.icon}</span>
            {m.label}
          </button>
        {/each}
      </div>

      <div class="flex-1 overflow-y-auto px-6 py-4">
        <p class="text-xs text-[var(--color-text-muted)] mb-3">自定义此模式下 AI 的系统提示词。留空则使用内置默认提示词。</p>
        {#key promptTab}
          {#if promptLoading}
            <div class="flex items-center justify-center py-8">
              <span class="material-symbols-outlined animate-spin text-primary-500">progress_activity</span>
            </div>
          {:else}
            <textarea
              class="w-full h-64 px-4 py-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] font-mono text-xs leading-relaxed resize-y focus:outline-none focus:ring-2 focus:ring-primary-500/30 focus:border-primary-500"
              value={promptDraft}
              oninput={(e) => onDraftChange((e.target as HTMLTextAreaElement).value)}
              placeholder="输入自定义提示词..."
            ></textarea>
          {/if}
        {/key}
      </div>

      <div class="flex items-center justify-between px-6 py-4 border-t border-[var(--color-border)]">
        <button class="flex items-center gap-1.5 px-3 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={onReset}>
          <span class="material-symbols-outlined text-[14px]">restart_alt</span>
          恢复默认
        </button>
        <div class="flex items-center gap-2">
          <button class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>取消</button>
          <button class="flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={onSave} disabled={promptSaving}>
            {#if promptSaving}
              <span class="material-symbols-outlined text-[14px] animate-spin">progress_activity</span>
            {:else}
              <span class="material-symbols-outlined text-[14px]">save</span>
            {/if}
            保存
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

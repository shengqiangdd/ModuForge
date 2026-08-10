<script lang="ts">
import type { Mode, AnalysisMode } from '../lib/types';
import { MODES } from '../lib/types';

let {
  mode = 'generate',
  streaming = false,
  showComparison = false,
  showProjectContext = false,
  showHistorySidebar = false,
  showCapability = false,
  onModeChange,
  onToggleComparison,
  onToggleProjectContext,
  onToggleHistory,
  onLoadCapability,
  onOpenPromptSettings,
  onOpenMDPrompts,
  onNavigate,
}: {
  mode: Mode;
  streaming: boolean;
  showComparison: boolean;
  showProjectContext: boolean;
  showHistorySidebar: boolean;
  showCapability: boolean;
  onModeChange: (m: Mode) => void;
  onToggleComparison: () => void;
  onToggleProjectContext: () => void;
  onToggleHistory: () => void;
  onLoadCapability: () => void;
  onOpenPromptSettings: () => void;
  onOpenMDPrompts: () => void;
  onNavigate?: (route: string) => void;
} = $props();
</script>

<div class="px-2 py-1.5 border-b border-[var(--color-border)] bg-[var(--color-surface)]">
  <div class="flex items-center gap-0.5 overflow-x-auto" style="-webkit-overflow-scrolling: touch; scrollbar-width: none;">
    <button class="md:hidden flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={() => onNavigate?.('projects')} title="返回">
      <span class="material-symbols-outlined text-[18px]">arrow_back</span>
    </button>
    <div class="md:hidden w-px h-4 bg-[var(--color-border)] mx-0.5 flex-shrink-0"></div>
    {#each MODES as m}
      <button
        class="flex-shrink-0 flex items-center justify-center gap-1 px-2 py-1 rounded-lg text-xs font-medium transition-all duration-150 min-h-[32px]
          {mode === m.value ? 'bg-primary-600 text-white shadow-sm' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
        onclick={() => onModeChange(m.value)}
        title={m.label}
      >
        <span class="material-symbols-outlined text-[16px]">{m.icon}</span>
        <span class="hidden sm:inline mode-label">{m.label}</span>
      </button>
    {/each}
    <div class="w-px h-4 bg-[var(--color-border)] mx-0.5 flex-shrink-0"></div>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={onToggleComparison} title="多模型对比">
      <span class="material-symbols-outlined text-[16px]">compare_arrows</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={onToggleProjectContext} title="项目上下文">
      <span class="material-symbols-outlined text-[16px]">folder</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg transition-all disabled:opacity-50 min-h-[32px] {showHistorySidebar ? 'bg-primary-500/10 text-primary-500' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}" onclick={onToggleHistory} title="历史记录">
      <span class="material-symbols-outlined text-[16px]">history</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={onLoadCapability} title="AI 能力评分">
      <span class="material-symbols-outlined text-[16px]">speed</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={onOpenPromptSettings} title="提示词设置">
      <span class="material-symbols-outlined text-[16px]">edit_note</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={onOpenMDPrompts} title="MD 提示词编辑器">
      <span class="material-symbols-outlined text-[16px]">code</span>
    </button>
  </div>
</div>

<script lang="ts">
import type { Mode } from '../lib/types';
import { MODES } from '../lib/types';

let {
  mode = 'generate',
  streaming = false,
  showComparison = false,
  showRepoReference = false,
  showHistorySidebar = false,
  showMcpTools = false,
  onModeChange,
  onToggleComparison,
  onToggleRepoReference,
  onToggleHistory,
  onToggleMcpTools,
  onOpenPromptSettings,
  onOpenMDPrompts,
  onNavigate,
  showSearch = false,
  onToggleSearch,
}: {
  mode: Mode;
  streaming: boolean;
  showComparison: boolean;
  showRepoReference: boolean;
  showHistorySidebar: boolean;
  showMcpTools: boolean;
  onModeChange: (m: Mode) => void;
  onToggleComparison: () => void;
  onToggleRepoReference: () => void;
  onToggleHistory: () => void;
  onToggleMcpTools: () => void;
  onOpenPromptSettings: () => void;
  onOpenMDPrompts: () => void;
  onNavigate?: (route: string) => void;
  showSearch?: boolean;
  onToggleSearch?: () => void;
} = $props();
</script>

<div class="flex items-center gap-1 px-2 py-1.5 border-b border-[var(--color-border)] bg-[var(--color-surface)]">
  <!-- Mobile back button -->
  <button class="md:hidden flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={() => onNavigate?.('projects')} title="返回">
    <span class="material-symbols-outlined text-[18px]">arrow_back</span>
  </button>
  <div class="md:hidden w-px h-4 bg-[var(--color-border)] mx-0.5 flex-shrink-0"></div>

  <!-- Mode pills -->
  <div class="flex items-center gap-0.5 overflow-x-auto flex-shrink-0" style="scrollbar-width: none;">
    {#each MODES as m}
      <button
        class="flex-shrink-0 flex items-center justify-center gap-1 px-2 py-1 rounded-lg text-xs font-medium transition-all duration-150 min-h-[32px]
          {mode === m.value ? 'bg-primary-600 text-white shadow-sm' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
        onclick={() => onModeChange(m.value)}
        title={m.label}
      >
        <span class="material-symbols-outlined text-[15px]">{m.icon}</span>
        <span class="hidden sm:inline mode-label">{m.label}</span>
      </button>
    {/each}
  </div>

  <!-- Separator -->
  <div class="w-px h-4 bg-[var(--color-border)] mx-0.5 flex-shrink-0"></div>

  <!-- Action buttons (right side) -->
  <div class="flex items-center gap-0.5 ml-auto flex-shrink-0">
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={onToggleComparison} title="多模型对比">
      <span class="material-symbols-outlined text-[16px]">compare_arrows</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg transition-all min-h-[32px] {showRepoReference ? 'bg-primary-500/10 text-primary-500' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}" onclick={onToggleRepoReference} title="参考仓库">
      <span class="material-symbols-outlined text-[16px]">link</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg transition-all disabled:opacity-50 min-h-[32px] {showHistorySidebar ? 'bg-primary-500/10 text-primary-500' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}" onclick={onToggleHistory} title="历史记录">
      <span class="material-symbols-outlined text-[16px]">history</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg transition-all disabled:opacity-50 min-h-[32px] {showMcpTools ? 'bg-primary-500/10 text-primary-500' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}" onclick={onToggleMcpTools} title="MCP 工具">
      <span class="material-symbols-outlined text-[16px]">hub</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={onOpenPromptSettings} title="提示词设置">
      <span class="material-symbols-outlined text-[16px]">edit_note</span>
    </button>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all min-h-[32px]" onclick={onOpenMDPrompts} title="MD 提示词编辑器">
      <span class="material-symbols-outlined text-[16px]">code</span>
    </button>
    <div class="w-px h-4 bg-[var(--color-border)] mx-0.5 flex-shrink-0"></div>
    <button class="flex-shrink-0 flex items-center justify-center p-1.5 rounded-lg transition-all min-h-[32px] {showSearch ? 'bg-primary-500/10 text-primary-500' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}" onclick={onToggleSearch} title="搜索会话">
      <span class="material-symbols-outlined text-[16px]">search</span>
    </button>
  </div>
</div>

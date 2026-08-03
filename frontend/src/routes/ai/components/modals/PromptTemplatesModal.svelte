<script lang="ts">
import { PROMPT_TEMPLATES } from '../../lib/types';

let {
  show = false,
  onClose,
  onSelect,
}: {
  show: boolean;
  onClose: () => void;
  onSelect: (prompt: string) => void;
} = $props();
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.4)" onclick={onClose}>
    <div class="w-full max-w-sm max-h-[70vh] rounded-xl shadow-2xl border border-[var(--color-border)] bg-[var(--color-bg)] flex flex-col" onclick={(e) => e.stopPropagation()}>
      <div class="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)] shrink-0">
        <span class="text-sm font-semibold text-[var(--color-text)]">提示词模板</span>
        <button class="p-1 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-text-muted)">close</span>
        </button>
      </div>
      <div class="p-2 space-y-1 overflow-y-auto flex-1 min-h-0">
        {#each PROMPT_TEMPLATES as template}
          <button
            class="flex items-center gap-2 w-full px-3 py-2.5 rounded-lg text-sm transition-colors hover:bg-[var(--color-surface)]"
            onclick={() => onSelect(template.prompt)}
          >
            <span class="material-symbols-outlined text-[16px] text-primary-500">description</span>
            <span class="font-medium text-[var(--color-text)]">{template.name}</span>
          </button>
        {/each}
      </div>
    </div>
  </div>
{/if}

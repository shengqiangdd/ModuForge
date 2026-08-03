<script lang="ts">
import type { ProgressStepDetail } from '../lib/types';
import { PROGRESS_LABELS } from '../lib/types';

let {
  show = false,
  streaming = false,
  currentStepIndex = -1,
  progressStepDetails = [],
  stepElapsed = '0s',
  progressCollapsed = false,
  onToggleCollapse,
}: {
  show: boolean;
  streaming: boolean;
  currentStepIndex: number;
  progressStepDetails: ProgressStepDetail[];
  stepElapsed: string;
  progressCollapsed: boolean;
  onToggleCollapse: () => void;
} = $props();
</script>

{#if show}
  <div class="px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
    <button class="flex items-center gap-2 w-full text-left" onclick={onToggleCollapse}>
      <span class="material-symbols-outlined text-[14px] text-primary-500 animate-spin">progress_activity</span>
      <span class="text-xs font-medium text-[var(--color-text)] flex-1 truncate">
        {progressStepDetails.length > 0 ? progressStepDetails[progressStepDetails.length - 1].message : '正在准备...'}
      </span>
      <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)] transition-transform {progressCollapsed ? '' : 'rotate-180'}">expand_less</span>
    </button>
    {#if !progressCollapsed}
      <div class="mt-2 space-y-1">
        {#each ['start', 'structure', 'script', 'system', 'optimize', 'done'] as step, si}
          <div class="flex items-center gap-2 px-2 py-0.5 rounded-lg {progressStepDetails.some(d => d.step === step) ? 'bg-primary-500/10' : si === currentStepIndex ? 'bg-primary-500/5' : ''}">
            {#if progressStepDetails.some(d => d.step === step)}
              <span class="material-symbols-outlined text-[12px] text-primary-500">check_circle</span>
            {:else if si === currentStepIndex}
              <span class="material-symbols-outlined text-[12px] text-primary-500 animate-pulse">radio_button_checked</span>
            {:else}
              <span class="material-symbols-outlined text-[12px] text-[var(--color-text-muted)]">radio_button_unchecked</span>
            {/if}
            <span class="text-[11px] {progressStepDetails.some(d => d.step === step) ? 'text-primary-600' : si === currentStepIndex ? 'text-[var(--color-text)]' : 'text-[var(--color-text-muted)]'}">
              {PROGRESS_LABELS[step] || step}
            </span>
            {#if progressStepDetails.some(d => d.step === step)}
              {@const _detail = progressStepDetails.find(d => d.step === step)}
              {#if _detail}
                <span class="text-[10px] text-[var(--color-text-muted)] ml-auto">
                  {new Date(_detail.time).toLocaleTimeString('zh-CN', {hour: '2-digit', minute: '2-digit', second: '2-digit'})}
                </span>
              {/if}
            {:else if si === currentStepIndex && streaming}
              <span class="text-[10px] text-amber-500 ml-auto animate-pulse">⏱ {stepElapsed}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

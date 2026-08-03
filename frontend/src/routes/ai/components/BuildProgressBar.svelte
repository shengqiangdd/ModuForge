<script lang="ts">
let {
  show = false,
  progress = null,
}: {
  show: boolean;
  progress: { stage: string; progress: number; message: string } | null;
} = $props();
</script>

{#if show && progress}
  <div class="fixed bottom-0 left-0 right-0 z-50 px-4 pb-4 pointer-events-none">
    <div class="max-w-xl mx-auto pointer-events-auto bg-[var(--color-bg)] rounded-2xl shadow-2xl border border-[var(--color-border)] p-4">
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-[16px] animate-spin text-primary-500">progress_activity</span>
          <span class="text-sm font-medium text-[var(--color-text)]">{progress.message}</span>
        </div>
        <span class="text-xs font-mono text-[var(--color-text-muted)]">{progress.progress}%</span>
      </div>
      <div class="w-full h-2 rounded-full overflow-hidden" style="background: var(--color-surface);">
        <div class="h-full rounded-full transition-all duration-500 ease-out" style="width: {progress.progress}%; background: linear-gradient(90deg, var(--color-primary), var(--color-info, #06b6d4));"></div>
      </div>
      <div class="flex items-center justify-between mt-2">
        {#each ['compile', 'test', 'package'] as stage}
          <div class="flex items-center gap-1 text-[10px] {progress.stage === stage || ['compile', 'test', 'package'].indexOf(progress.stage) > ['compile', 'test', 'package'].indexOf(stage) ? 'text-primary-400' : 'text-[var(--color-text-muted)]'}">
            <span class="material-symbols-outlined text-[12px]">
              {progress.stage === stage ? 'radio_button_checked' : (['compile', 'test', 'package'].indexOf(progress.stage) > ['compile', 'test', 'package'].indexOf(stage) ? 'check_circle' : 'radio_button_unchecked')}
            </span>
            {stage === 'compile' ? '编译' : stage === 'test' ? '测试' : '打包'}
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}

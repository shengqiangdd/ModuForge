<script lang="ts">
import type { SecurityScanResult } from '../../lib/types';

let {
  show = false,
  scanResult = null,
  onClose,
  onContinue,
}: {
  show: boolean;
  scanResult: SecurityScanResult | null;
  onClose: () => void;
  onContinue: () => void;
} = $props();
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onclick={onClose}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-md border border-[var(--color-border)]" onclick={(e) => e.stopPropagation()}>
      <div class="px-6 py-4 border-b border-[var(--color-border)] flex items-center gap-2">
        <span class="material-symbols-outlined text-error-500">warning</span>
        <h3 class="text-lg font-semibold text-[var(--color-text)]">安全警告</h3>
      </div>
      <div class="px-6 py-4 space-y-3">
        <p class="text-sm text-[var(--color-text-secondary)]">发现严重安全问题，导入可能存在风险：</p>
        {#if scanResult}
          {#each (scanResult?.issues || []).filter(i => i.severity === 'critical') as issue}
            <div class="flex items-start gap-2 p-2 rounded-lg text-xs" style="background: color-mix(in srgb, #ef4444 10%, var(--color-bg))">
              <span class="material-symbols-outlined text-[14px] text-error-500 flex-shrink-0">error</span>
              <div>
                <p class="font-medium" style="color: var(--color-error)">{issue.rule}</p>
                <p class="text-[var(--color-text-secondary)]">{issue.file}:{issue.line} — {issue.message}</p>
              </div>
            </div>
          {/each}
        {/if}
      </div>
      <div class="flex justify-end gap-2 px-6 py-4 border-t border-[var(--color-border)]">
        <button class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>取消导入</button>
        <button class="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--color-error, #ef4444)" onclick={onContinue}>
          <span class="material-symbols-outlined text-[14px]">download</span>
          忽略风险并导入
        </button>
      </div>
    </div>
  </div>
{/if}

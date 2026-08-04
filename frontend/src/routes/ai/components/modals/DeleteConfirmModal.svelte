<script lang="ts">
let {
  show = false,
  onClose,
  onConfirm,
}: {
  show: boolean;
  onClose: () => void;
  onConfirm: () => void;
} = $props();
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-sm border border-[var(--color-border)] p-6" role="dialog" aria-modal="true" tabindex="-1">
      <div class="flex items-center gap-2 mb-4">
        <span class="material-symbols-outlined text-error-500 text-[20px]">warning</span>
        <h3 class="text-base font-semibold text-[var(--color-text)]">确认删除</h3>
      </div>
      <p class="text-sm text-[var(--color-text-secondary)] mb-6">确定要删除这条消息吗？此操作不可撤销。</p>
      <div class="flex justify-end gap-2">
        <button class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>取消</button>
        <button class="px-4 py-2 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--color-error, #ef4444)" onclick={onConfirm}>删除</button>
      </div>
    </div>
  </div>
{/if}

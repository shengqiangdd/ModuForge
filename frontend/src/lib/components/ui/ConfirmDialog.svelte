<script lang="ts">
  let { open = false, title = '确认', message = '', confirmText = '确认', cancelText = '取消', variant = 'primary', onConfirm = () => {}, onCancel = () => {} }: {
    open?: boolean;
    title?: string;
    message?: string;
    confirmText?: string;
    cancelText?: string;
    variant?: 'primary' | 'danger';
    onConfirm?: () => void;
    onCancel?: () => void;
  } = $props();

  let visible = $state(open);
  $effect(() => { visible = open; });

  function handleConfirm() {
    visible = false;
    onConfirm();
  }

  function handleCancel() {
    visible = false;
    onCancel();
  }
</script>

{#if visible}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-[fadeIn_0.15s_ease-out]" onclick={handleCancel}>
    <div class="bg-[var(--color-bg-elevated)] rounded-2xl shadow-elevated-lg w-full max-w-sm mx-4 p-6 border border-[var(--color-border)] animate-[scaleIn_0.2s_ease-out]" onclick={(e) => e.stopPropagation()}>
      <div class="flex items-center gap-3 mb-4">
        <span class="material-symbols-outlined text-2xl {variant === 'danger' ? 'text-red-500' : 'text-primary-600'}">
          {variant === 'danger' ? 'warning' : 'help'}
        </span>
        <h3 class="text-lg font-bold text-[var(--color-text)]">{title}</h3>
      </div>
      <p class="text-sm text-[var(--color-text-secondary)] mb-6">{message}</p>
      <div class="flex justify-end gap-3">
        <button
          class="px-4 py-2 rounded-xl text-sm font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors"
          onclick={handleCancel}
        >
          {cancelText}
        </button>
        <button
          class="px-4 py-2 rounded-xl text-sm font-medium text-white transition-all {variant === 'danger' ? 'bg-red-500 hover:bg-red-600' : 'bg-primary-600 hover:bg-primary-700'}"
          onclick={handleConfirm}
        >
          {confirmText}
        </button>
      </div>
    </div>
  </div>
{/if}

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
  <div class="confirm-overlay fixed inset-0 z-50 flex items-center justify-center p-4" onclick={handleCancel} role="dialog" aria-modal="true" aria-labelledby="confirm-title">
    <div class="confirm-backdrop absolute inset-0"></div>
    <div class="confirm-dialog relative w-full max-w-md rounded-2xl shadow-2xl border overflow-hidden animate-[scaleIn_0.2s_ease-out]" onclick={(e) => e.stopPropagation()}>
      <!-- Header -->
      <div class="confirm-header px-6 pt-6 pb-4" class:danger={variant === 'danger'}>
        <div class="flex items-center gap-3">
          <div class="confirm-icon w-10 h-10 rounded-xl flex items-center justify-center" class:danger-icon={variant === 'danger'}>
            <span class="material-symbols-outlined text-[22px]">
              {variant === 'danger' ? 'warning' : 'help'}
            </span>
          </div>
          <h3 id="confirm-title" class="text-lg font-bold">{title}</h3>
        </div>
      </div>
      
      <!-- Body -->
      <div class="confirm-body px-6 py-4">
        <p class="text-sm leading-relaxed">{message}</p>
      </div>
      
      <!-- Footer -->
      <div class="confirm-footer px-6 py-4 flex justify-end gap-3">
        <button
          class="confirm-btn px-5 py-2.5 rounded-xl text-sm font-medium transition-all duration-200"
          onclick={handleCancel}
        >
          {cancelText}
        </button>
        <button
          class="confirm-btn confirm-btn-primary px-5 py-2.5 rounded-xl text-sm font-medium text-white transition-all duration-200 active:scale-[0.98]"
          class:danger-btn={variant === 'danger'}
          onclick={handleConfirm}
        >
          {confirmText}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .confirm-overlay {
    animation: fadeIn 0.15s ease-out;
  }
  
  .confirm-backdrop {
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
  }
  
  .confirm-dialog {
    background: var(--color-bg-elevated);
    border-color: var(--color-border);
    box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
  }
  
  .confirm-header {
    border-bottom: 1px solid var(--color-border);
  }
  .confirm-header.danger {
    background: linear-gradient(135deg, rgba(239,68,68,0.1) 0%, rgba(239,68,68,0.05) 100%);
  }
  
  .confirm-icon {
    background: var(--gradient-brand-subtle);
    color: var(--color-primary);
  }
  .confirm-icon.danger-icon {
    background: rgba(239,68,68,0.15);
    color: #ef4444;
  }
  
  .confirm-body {
    color: var(--color-text-secondary);
  }
  
  .confirm-footer {
    background: var(--color-surface);
    border-top: 1px solid var(--color-border);
  }
  
  .confirm-btn {
    background: transparent;
    color: var(--color-text-secondary);
    border: 1px solid var(--color-border);
  }
  .confirm-btn:hover {
    background: var(--color-bg-elevated);
    color: var(--color-text);
    border-color: var(--color-text-muted);
  }
  
  .confirm-btn-primary {
    background: var(--gradient-brand);
    border: none;
  }
  .confirm-btn-primary:hover {
    box-shadow: 0 0 20px rgba(139,92,246,0.3);
    transform: translateY(-1px);
  }
  
  .danger-btn {
    background: linear-gradient(135deg, #ef4444 0%, #dc2626 100%);
  }
  .danger-btn:hover {
    box-shadow: 0 0 20px rgba(239,68,68,0.3);
  }
</style>

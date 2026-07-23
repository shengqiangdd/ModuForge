<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getToasts, subscribe, dismiss } from '$lib/stores/toast.svelte.ts';

  let toasts = $state<Toast[]>([]);

  onMount(() => {
    toasts = getToasts();
    const unsub = subscribe(() => { toasts = getToasts(); });
    return unsub;
  });

  const icons: Record<string, string> = {
    success: 'check_circle',
    error: 'error',
    info: 'info',
    warning: 'warning',
  };
</script>

{#if toasts.length > 0}
  <div class="toast-container fixed bottom-6 right-6 z-[9999] flex flex-col gap-3 max-w-sm pointer-events-none">
    {#each toasts as t (t.id)}
      <div class="toast-item flex items-start gap-3 px-4 py-3.5 rounded-xl border shadow-lg pointer-events-auto backdrop-blur-xl {t.type}">
        <span class="material-symbols-outlined text-[20px] flex-shrink-0 mt-0.5">{icons[t.type]}</span>
        <p class="text-sm flex-1 leading-relaxed">{t.message}</p>
        <button class="toast-close p-0.5 rounded-lg transition-colors" onclick={() => dismiss(t.id)} aria-label="关闭">
          <span class="material-symbols-outlined text-[16px]">close</span>
        </button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toast-container {
    animation: slideInRight 0.3s ease-out;
  }

  .toast-item {
    animation: toastSlideIn 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    transition: all 0.2s ease;
  }
  .toast-item:hover {
    transform: translateX(-4px);
  }

  /* Success */
  .toast-item.success {
    background: rgba(34, 197, 94, 0.15);
    border-color: rgba(34, 197, 94, 0.3);
    color: #4ade80;
  }
  :global(.light) .toast-item.success {
    background: rgba(34, 197, 94, 0.1);
    border-color: rgba(34, 197, 94, 0.2);
    color: #16a34a;
  }

  /* Error */
  .toast-item.error {
    background: rgba(239, 68, 68, 0.15);
    border-color: rgba(239, 68, 68, 0.3);
    color: #f87171;
  }
  :global(.light) .toast-item.error {
    background: rgba(239, 68, 68, 0.1);
    border-color: rgba(239, 68, 68, 0.2);
    color: #dc2626;
  }

  /* Info */
  .toast-item.info {
    background: rgba(6, 182, 212, 0.15);
    border-color: rgba(6, 182, 212, 0.3);
    color: #22d3ee;
  }
  :global(.light) .toast-item.info {
    background: rgba(6, 182, 212, 0.1);
    border-color: rgba(6, 182, 212, 0.2);
    color: #0891b2;
  }

  /* Warning */
  .toast-item.warning {
    background: rgba(245, 158, 11, 0.15);
    border-color: rgba(245, 158, 11, 0.3);
    color: #fbbf24;
  }
  :global(.light) .toast-item.warning {
    background: rgba(245, 158, 11, 0.1);
    border-color: rgba(245, 158, 11, 0.2);
    color: #d97706;
  }

  /* Close button */
  .toast-close {
    color: inherit;
    opacity: 0.6;
  }
  .toast-close:hover {
    opacity: 1;
    background: rgba(255, 255, 255, 0.1);
  }
  :global(.light) .toast-close:hover {
    background: rgba(0, 0, 0, 0.05);
  }

  @keyframes slideInRight {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }

  @keyframes toastSlideIn {
    from {
      opacity: 0;
      transform: translateX(20px) scale(0.95);
    }
    to {
      opacity: 1;
      transform: translateX(0) scale(1);
    }
  }
</style>

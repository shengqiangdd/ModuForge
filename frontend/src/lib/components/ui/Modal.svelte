<script lang="ts">
  let { open = false, title = '', onclose }: {
    open?: boolean;
    title?: string;
    onclose?: () => void;
  } = $props();

  let visible = $state(open);

  $effect(() => { visible = open; });

  function handleKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && visible) {
      visible = false;
      onclose?.();
    }
  }

  function handleBackdrop() {
    visible = false;
    onclose?.();
  }
</script>

<svelte:window onkeydown={handleKey} />

{#if visible}
  <div class="modal-overlay fixed inset-0 z-50 flex items-center justify-center p-4" onclick={handleBackdrop} role="dialog" aria-modal="true" aria-labelledby="modal-title">
    <div class="modal-backdrop absolute inset-0"></div>
    <div class="modal-dialog relative w-full max-w-lg rounded-2xl shadow-2xl border overflow-hidden" onclick={(e) => e.stopPropagation()}>
      {#if title}
        <div class="modal-header px-6 pt-6 pb-4 flex items-center justify-between">
          <h3 id="modal-title" class="text-lg font-bold">{title}</h3>
          <button class="modal-close p-1 rounded-lg transition-colors" onclick={handleBackdrop} aria-label="关闭">
            <span class="material-symbols-outlined text-[20px]">close</span>
          </button>
        </div>
      {/if}
      <div class="modal-body px-6 py-4">
        <slot />
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    animation: fadeIn 0.15s ease-out;
  }

  .modal-backdrop {
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
  }

  .modal-dialog {
    background: var(--color-bg-elevated);
    border-color: var(--color-border);
    box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
    max-height: 85vh;
    display: flex;
    flex-direction: column;
  }

  .modal-header {
    border-bottom: 1px solid var(--color-border);
  }

  .modal-body {
    flex: 1;
    overflow-y: auto;
    color: var(--color-text-secondary);
  }

  .modal-close {
    color: var(--color-text-secondary);
    background: transparent;
    border: none;
    cursor: pointer;
  }
  .modal-close:hover {
    background: var(--color-bg-elevated);
    color: var(--color-text);
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
</style>

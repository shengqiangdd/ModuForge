<script lang="ts">
  let { variant = 'primary', size = 'md', disabled = false, loading = false, type = 'button', onclick }: {
    variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
    size?: 'sm' | 'md' | 'lg';
    disabled?: boolean;
    loading?: boolean;
    type?: string;
    onclick?: () => void;
  } = $props();
</script>

<button
  {type}
  class="btn btn-{variant} btn-{size}"
  class:loading
  {disabled}
  {onclick}
>
  {#if loading}
    <span class="spinner material-symbols-outlined">progress_activity</span>
  {/if}
  <slot />
</button>

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    border-radius: 0.75rem;
    font-weight: 500;
    transition: all 0.2s ease;
    cursor: pointer;
    border: 1px solid transparent;
    white-space: nowrap;
    user-select: none;
  }
  .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    pointer-events: none;
  }
  .btn:active:not(:disabled) {
    transform: scale(0.98);
  }

  .btn-sm { padding: 0.375rem 0.75rem; font-size: 0.75rem; }
  .btn-md { padding: 0.5rem 1rem; font-size: 0.875rem; }
  .btn-lg { padding: 0.75rem 1.5rem; font-size: 1rem; }

  .btn-primary {
    background: var(--gradient-brand);
    color: #fff;
    border: none;
  }
  .btn-primary:hover:not(:disabled) {
    box-shadow: 0 0 20px rgba(139,92,246,0.3);
    transform: translateY(-1px);
  }

  .btn-secondary {
    background: transparent;
    color: var(--color-text-secondary);
    border-color: var(--color-border);
  }
  .btn-secondary:hover:not(:disabled) {
    background: var(--color-bg-elevated);
    color: var(--color-text);
    border-color: var(--color-text-muted);
  }

  .btn-danger {
    background: linear-gradient(135deg, #ef4444 0%, #dc2626 100%);
    color: #fff;
    border: none;
  }
  .btn-danger:hover:not(:disabled) {
    box-shadow: 0 0 20px rgba(239,68,68,0.3);
    transform: translateY(-1px);
  }

  .btn-ghost {
    background: transparent;
    color: var(--color-text-secondary);
    border: none;
  }
  .btn-ghost:hover:not(:disabled) {
    background: var(--color-bg-elevated);
    color: var(--color-text);
  }

  .spinner {
    animation: spin 1s linear infinite;
    font-size: 1.1em;
  }

  .loading {
    pointer-events: none;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>

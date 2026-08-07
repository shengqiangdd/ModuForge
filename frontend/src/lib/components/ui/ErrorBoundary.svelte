<script lang="ts">
  import { onMount } from 'svelte';

  let { children }: { children?: import('svelte').Snippet } = $props();

  let error = $state<string | null>(null);
  let errorDetails = $state<string | null>(null);

  function handleError(e: ErrorEvent) {
    e.preventDefault();
    error = e.message || '未知错误';
    errorDetails = e.filename && e.lineno ? `${e.filename}:${e.lineno}` : null;
  }

  function handleRejection(e: PromiseRejectionEvent) {
    e.preventDefault();
    error = e.reason?.message || '未处理的 Promise 异常';
  }

  onMount(() => {
    window.addEventListener('error', handleError);
    window.addEventListener('unhandledrejection', handleRejection);
    return () => {
      window.removeEventListener('error', handleError);
      window.removeEventListener('unhandledrejection', handleRejection);
    };
  });
</script>

{#if error}
  <div class="error-boundary" role="alert">
    <div class="error-icon">
      <span class="material-symbols-outlined">error_outline</span>
    </div>
    <h3 class="error-title">组件异常</h3>
    <p class="error-message">{error}</p>
    {#if errorDetails}
      <p class="error-details">{errorDetails}</p>
    {/if}
    <div class="error-actions">
      <button class="btn-primary" onclick={() => { error = null; errorDetails = null; }}>
        重试
      </button>
      <button class="btn-ghost" onclick={() => { error = null; errorDetails = null; window.location.reload(); }}>
        刷新页面
      </button>
    </div>
  </div>
{:else}
  {#key error}
    {@render children?.()}
  {/key}
{/if}

<style>
  .error-boundary {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 3rem 1.5rem;
    text-align: center;
    border-radius: 1rem;
    border: 1px solid var(--color-border);
    background: var(--color-bg-elevated);
    margin: 1rem;
  }
  .error-icon {
    margin-bottom: 1rem;
  }
  .error-icon :global(.material-symbols-outlined) {
    font-size: 3rem;
    color: var(--color-error);
  }
  .error-title {
    font-size: 1.125rem;
    font-weight: 700;
    margin-bottom: 0.5rem;
    color: var(--color-text);
  }
  .error-message {
    font-size: 0.875rem;
    color: var(--color-text-secondary);
    margin-bottom: 0.5rem;
    max-width: 400px;
  }
  .error-details {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    margin-bottom: 1rem;
    font-family: monospace;
  }
  .error-actions {
    display: flex;
    gap: 0.75rem;
    margin-top: 0.5rem;
  }
</style>
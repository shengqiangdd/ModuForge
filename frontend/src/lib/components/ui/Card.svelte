<script lang="ts">
  let { title = '', subtitle = '', padding = true, variant = 'default' }: {
    title?: string;
    subtitle?: string;
    padding?: boolean;
    variant?: 'default' | 'elevated' | 'outlined';
  } = $props();
</script>

<div class="card card-{variant}" class:no-padding={!padding}>
  {#if title || subtitle}
    <div class="card-header">
      {#if title}
        <h3 class="card-title">{title}</h3>
      {/if}
      {#if subtitle}
        <p class="card-subtitle">{subtitle}</p>
      {/if}
    </div>
  {/if}
  <div class="card-body">
    <slot />
  </div>
</div>

<style>
  .card {
    border-radius: 1rem;
    overflow: hidden;
    transition: all 0.2s ease;
  }

  .card.default {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
  }

  .card.elevated {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
  }

  .card.outlined {
    background: transparent;
    border: 1px solid var(--color-border);
  }

  .card-header {
    padding: 1rem 1.25rem 0.5rem;
    border-bottom: 1px solid var(--color-border);
  }

  .card-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--color-text);
    margin: 0;
  }

  .card-subtitle {
    font-size: 0.8rem;
    color: var(--color-text-secondary);
    margin: 0.25rem 0 0;
  }

  .card-body {
    padding: 1.25rem;
  }

  .no-padding .card-body {
    padding: 0;
  }
</style>

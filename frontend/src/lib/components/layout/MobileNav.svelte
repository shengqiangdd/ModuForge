<script lang="ts">
  let {
    current = '',
    items = [],
    onNavigate
  }: {
    current?: string;
    items?: Array<{ id: string; icon: string; label: string }>;
    onNavigate?: (route: string) => void;
  } = $props();
</script>

<nav class="mobile-nav">
  {#each items as item}
    <button
      class="nav-item"
      class:active={current === item.id}
      onclick={() => onNavigate?.(item.id)}
    >
      <span class="material-symbols-outlined">{item.icon}</span>
      <span class="nav-label">{item.label}</span>
    </button>
  {/each}
</nav>

<style>
  .mobile-nav {
    display: none;
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: 64px;
    background: var(--color-bg-elevated);
    border-top: 1px solid var(--color-border);
    z-index: 40;
    padding: 0.5rem;
  }

  @media (max-width: 768px) {
    .mobile-nav {
      display: flex;
      align-items: center;
      justify-content: space-around;
    }
  }

  .nav-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.125rem;
    padding: 0.375rem 0.75rem;
    border: none;
    background: transparent;
    border-radius: 0.5rem;
    cursor: pointer;
    color: var(--color-text-secondary);
    transition: color 0.2s;
    min-width: 48px;
  }

  .nav-item.active {
    color: var(--color-primary);
  }

  .nav-item :global(.material-symbols-outlined) {
    font-size: 1.25rem;
  }

  .nav-label {
    font-size: 0.625rem;
    font-weight: 500;
  }
</style>

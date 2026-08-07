<script lang="ts">
  let { count = 3, lines = [100, 80, 60] }: { count?: number; lines?: number[] } = $props();
</script>

<div class="skeleton-container" class:animate-pulse={true}>
  {#each Array(count) as _, i}
    <div class="skeleton-line" style="width: {lines[i % lines.length]}%"></div>
  {/each}
</div>

<style>
  .skeleton-container {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    padding: 1rem;
  }

  .skeleton-line {
    height: 1rem;
    background: linear-gradient(90deg, var(--color-gray-200) 25%, var(--color-gray-100) 50%, var(--color-gray-200) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: 0.25rem;
  }

  :global(.dark) .skeleton-line {
    background: linear-gradient(90deg, var(--color-gray-700) 25%, var(--color-gray-600) 50%, var(--color-gray-700) 75%);
    background-size: 200% 100%;
  }

  @keyframes shimmer {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
  }
</style>

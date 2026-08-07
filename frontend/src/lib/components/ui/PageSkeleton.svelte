<script lang="ts">
  let { variant = 'default' }: { variant?: 'default' | 'editor' | 'market' | 'list' } = $props();
</script>

{#if variant === 'editor'}
  <div class="flex flex-col h-full gap-2 p-4">
    <div class="flex gap-4 h-10">
      <div class="skeleton-line w-48 h-10 rounded-lg"></div>
      <div class="skeleton-line w-32 h-10 rounded-lg"></div>
      <div class="skeleton-line w-24 h-10 rounded-lg ml-auto"></div>
    </div>
    <div class="flex flex-1 gap-4 mt-2">
      <div class="flex flex-col gap-2 w-56">
        <div class="skeleton-line w-full h-8 rounded-lg"></div>
        <div class="skeleton-line w-full h-8 rounded-lg"></div>
        <div class="skeleton-line w-full h-8 rounded-lg"></div>
        <div class="skeleton-line w-3/4 h-8 rounded-lg"></div>
      </div>
      <div class="flex-1 skeleton-line rounded-xl"></div>
    </div>
  </div>
{:else if variant === 'market'}
  <div class="flex flex-col gap-4 p-4 md:p-6 max-w-7xl mx-auto">
    <div class="skeleton-line w-64 h-8 rounded-lg mb-2"></div>
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each Array(6) as _}
        <div class="rounded-2xl border p-4" style="background: var(--color-bg-elevated); border-color: var(--color-border)">
          <div class="flex items-center gap-3 mb-3">
            <div class="skeleton-line w-12 h-12 rounded-xl"></div>
            <div class="flex-1">
              <div class="skeleton-line w-32 h-4 rounded mb-1"></div>
              <div class="skeleton-line w-20 h-3 rounded"></div>
            </div>
          </div>
          <div class="skeleton-line w-full h-3 rounded mb-1"></div>
          <div class="skeleton-line w-3/4 h-3 rounded mb-3"></div>
          <div class="flex gap-2">
            <div class="skeleton-line w-16 h-6 rounded-full"></div>
            <div class="skeleton-line w-12 h-6 rounded-full"></div>
          </div>
        </div>
      {/each}
    </div>
  </div>
{:else if variant === 'list'}
  <div class="flex flex-col gap-3 p-4 md:p-6 max-w-7xl mx-auto">
    <div class="skeleton-line w-48 h-8 rounded-lg mb-2"></div>
    <div class="skeleton-line w-full h-12 rounded-lg mb-2"></div>
    {#each Array(5) as _}
      <div class="rounded-2xl border p-4 flex items-center gap-3" style="background: var(--color-bg-elevated); border-color: var(--color-border)">
        <div class="skeleton-line w-10 h-10 rounded-xl flex-shrink-0"></div>
        <div class="flex-1">
          <div class="skeleton-line w-48 h-4 rounded mb-1"></div>
          <div class="skeleton-line w-24 h-3 rounded"></div>
        </div>
        <div class="skeleton-line w-20 h-8 rounded-lg"></div>
      </div>
    {/each}
  </div>
{:else}
  <div class="flex flex-col gap-3 p-4 md:p-6 max-w-7xl mx-auto">
    <div class="skeleton-line w-48 h-8 rounded-lg mb-2"></div>
    <div class="skeleton-line w-full h-3 rounded mb-1"></div>
    <div class="skeleton-line w-3/4 h-3 rounded mb-1"></div>
    <div class="skeleton-line w-2/3 h-3 rounded mb-3"></div>
    <div class="skeleton-line w-full h-32 rounded-xl"></div>
  </div>
{/if}

<style>
  .skeleton-line {
    background: linear-gradient(90deg, var(--color-gray-200) 25%, var(--color-gray-100) 50%, var(--color-gray-200) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
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
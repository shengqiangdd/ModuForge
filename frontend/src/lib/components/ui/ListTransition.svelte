<script lang="ts">
  let { items = [], key = 'id', duration = 200, children }: {
    items?: Record<string, unknown>[];
    key?: string;
    duration?: number;
    children?: (...args: unknown[]) => unknown;
  } = $props();

  let prevItems = $state<Record<string, unknown>[]>([]);
  let transitioning = $state(new Set<string>());

  $effect(() => {
    const newKeys = new Set(items.map(i => i[key]));
    const oldKeys = new Set(prevItems.map(i => i[key]));

    // Find removed items
    for (const oldKey of oldKeys) {
      if (!newKeys.has(oldKey)) {
        transitioning.add(oldKey as string);
      }
    }

    prevItems = [...items];
  });
</script>

<div class="list-transition">
  {#each items as item, i (item[key])}
    <div
      class="list-item"
      class:entering={!transitioning.has(item[key])}
      style="transition: opacity {duration}ms ease-out, transform {duration}ms ease-out; animation-delay: {i * 30}ms"
    >
      {@render children(item, i)}
    </div>
  {/each}
</div>

<style>
  .list-transition {
    display: flex;
    flex-direction: column;
  }

  .list-item {
    animation: slideIn 0.3s ease-out forwards;
    opacity: 0;
    transform: translateY(10px);
  }

  @keyframes slideIn {
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>

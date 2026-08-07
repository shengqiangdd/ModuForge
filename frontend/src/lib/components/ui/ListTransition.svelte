<script lang="ts" generics="T extends Record<string, any>">
  let { items = [], key = 'id', duration = 200, children }: {
    items?: T[];
    key?: string;
    duration?: number;
    children?: import('svelte').Snippet<[T, number]>;
  } = $props();

  let prevItems = $state<T[]>([]);
  let transitioning = $state(new Set<string>());

  $effect(() => {
    const newKeys = new Set(items.map(i => String(i[key])));
    const oldKeys = new Set(prevItems.map(i => String(i[key])));

    for (const oldKey of oldKeys) {
      if (!newKeys.has(oldKey)) {
        transitioning.add(oldKey);
      }
    }

    prevItems = [...items];
  });
</script>

<div class="list-transition">
  {#each items as item, i (item[key])}
    <div
      class="list-item"
      class:entering={!transitioning.has(String(item[key]))}
      style="transition: opacity {duration}ms ease-out, transform {duration}ms ease-out; animation-delay: {i * 30}ms"
    >
      {#if children}{@render children(item, i)}{/if}
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

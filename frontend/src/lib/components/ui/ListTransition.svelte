<script lang="ts" generics="T extends Record<string, any>">
  // Svelte 5 compatibility: previous implementation mutated $state (prevItems)
  // inside $effect while also reading it, which triggers `state_unsafe_mutation`
  // and corrupts the global flush scheduler — killing ALL subsequent $state
  // updates in the whole component tree (symptom: page renders initially, then
  // tab clicks / toggles never re-render). Replaced with a plain {#each} that
  // keeps the enter animation via CSS.
  let { items = [], key = 'id', duration = 200, children }: {
    items?: T[];
    key?: string;
    duration?: number;
    children?: import('svelte').Snippet<[T, number]>;
  } = $props();
</script>

<div class="list-transition">
  {#each items as item, i (item[key])}
    <div
      class="list-item"
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

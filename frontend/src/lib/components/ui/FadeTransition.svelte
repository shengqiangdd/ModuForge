<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  let { show = false, duration = 200, direction = 'up' }: {
    show?: boolean;
    duration?: number;
    direction?: 'up' | 'down' | 'left' | 'right' | 'fade';
  } = $props();

  let visible = $state(false);
  let animating = $state(false);
  let exitTimer: ReturnType<typeof setTimeout> | null = null;

  onMount(() => {
    if (show) {
      visible = true;
      animating = true;
    }
  });

  onDestroy(() => {
    if (exitTimer) clearTimeout(exitTimer);
  });

  $effect(() => {
    if (show) {
      visible = true;
      requestAnimationFrame(() => {
        animating = true;
      });
    } else {
      animating = false;
      exitTimer = setTimeout(() => {
        visible = false;
        exitTimer = null;
      }, duration);
    }
  });

  function getTransform(): string {
    if (!animating) {
      switch (direction) {
        case 'up': return 'translateY(20px)';
        case 'down': return 'translateY(-20px)';
        case 'left': return 'translateX(20px)';
        case 'right': return 'translateX(-20px)';
        case 'fade': return 'none';
      }
    }
    return 'none';
  }

  function getOpacity(): number {
    if (direction === 'fade') {
      return animating ? 1 : 0;
    }
    return animating ? 1 : 0.5;
  }
</script>

{#if visible}
  <div
    class="transition-wrapper"
    style="
      transition: transform {duration}ms ease-out, opacity {duration}ms ease-out;
      transform: {getTransform()};
      opacity: {getOpacity()};
    "
  >
    <slot />
  </div>
{/if}

<style>
  .transition-wrapper {
    will-change: transform, opacity;
  }
</style>

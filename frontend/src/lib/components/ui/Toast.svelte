<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getToasts, subscribe, dismiss } from '$lib/stores/toast.svelte.ts';

  let toasts = $state<Toast[]>([]);

  onMount(() => {
    toasts = getToasts();
    const unsub = subscribe(() => { toasts = getToasts(); });
    return unsub;
  });

  const icons: Record<string, string> = {
    success: 'check_circle',
    error: 'error',
    info: 'info',
    warning: 'warning',
  };

  const colors: Record<string, string> = {
    success: 'bg-green-50 border-green-400 text-green-800',
    error: 'bg-red-50 border-red-400 text-red-800',
    info: 'bg-blue-50 border-blue-400 text-blue-800',
    warning: 'bg-amber-50 border-amber-400 text-amber-800',
  };
</script>

{#if toasts.length > 0}
  <div class="fixed bottom-6 right-6 z-[9999] flex flex-col gap-2 max-w-sm pointer-events-none">
    {#each toasts as t (t.id)}
      <div class="flex items-start gap-3 px-4 py-3 rounded-xl border shadow-elevated-lg animate-[slideUp_0.2s_ease-out] pointer-events-auto {colors[t.type]}">
        <span class="material-symbols-outlined text-[20px] flex-shrink-0 mt-0.5">{icons[t.type]}</span>
        <p class="text-sm flex-1">{t.message}</p>
        <button class="p-0.5 rounded hover:bg-black/5 transition-colors" onclick={() => dismiss(t.id)}>
          <span class="material-symbols-outlined text-[16px]">close</span>
        </button>
      </div>
    {/each}
  </div>
{/if}

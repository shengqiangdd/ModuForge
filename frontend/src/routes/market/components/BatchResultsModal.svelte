<script lang="ts">
  let { show, results, onClose }: {
    show: boolean;
    results: { slug: string; status: string; error?: string }[];
    onClose: () => void;
  } = $props();
</script>

{#if show}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
    <div class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">批量操作结果</h3>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-2 max-h-60 overflow-auto">
        {#each results as res}
          <div class="flex items-center gap-2 text-sm">
            <span class="material-symbols-outlined text-[14px]" style="color: {res.status === 'ok' ? 'var(--color-success)' : 'var(--color-error)'}">{res.status === 'ok' ? 'check_circle' : 'error'}</span>
            <span style="color: var(--color-text)">{res.slug}</span>
            {#if res.error}
              <span class="text-xs text-[var(--color-error)] ml-auto">{res.error}</span>
            {:else}
              <span class="text-xs text-[var(--color-success)] ml-auto">成功</span>
            {/if}
          </div>
        {/each}
      </div>
      <div class="p-5 border-t flex justify-end" style="border-color: var(--color-border)">
        <button class="btn-primary text-sm" onclick={onClose}>关闭</button>
      </div>
    </div>
  </div>
{/if}
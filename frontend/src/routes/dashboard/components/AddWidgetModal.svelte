<script lang="ts">
  let {
    open,
    widgetTypes,
    existingWidgetTypes,
    onClose,
    onAdd,
  }: {
    open: boolean;
    widgetTypes: { type: string; name: string; desc: string }[];
    existingWidgetTypes: Set<string>;
    onClose: () => void;
    onAdd: (type: string) => void;
  } = $props();

  let selectedType = $state('');

  function handleAdd() {
    if (selectedType) {
      onAdd(selectedType);
      selectedType = '';
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div
    class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]"
    style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
  >
    <div
      class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]"
      style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">添加 Widget</h3>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-2 max-h-80 overflow-auto">
        {#each widgetTypes.filter(wt => !existingWidgetTypes.has(wt.type)) as wt}
          <button
            class="w-full text-left p-3 rounded-xl border transition-all"
            style="border-color: {selectedType === wt.type ? 'var(--color-primary)' : 'var(--color-border)'}; background: {selectedType === wt.type ? 'var(--color-primary-light)' : 'var(--color-surface)'}"
            onclick={() => selectedType = wt.type}
          >
            <p class="text-sm font-medium text-[var(--color-text)]">{wt.name}</p>
            <p class="text-xs text-[var(--color-text-muted)] mt-0.5">{wt.desc}</p>
          </button>
        {:else}
          <p class="text-sm text-center py-4" style="color: var(--color-text-muted)">所有 Widget 类型都已添加</p>
        {/each}
      </div>
      <div class="p-5 border-t flex justify-end gap-3" style="border-color: var(--color-border)">
        <button class="btn-ghost text-sm" onclick={onClose}>取消</button>
        <button class="btn-primary text-sm" disabled={!selectedType} onclick={handleAdd}>添加</button>
      </div>
    </div>
  </div>
{/if}

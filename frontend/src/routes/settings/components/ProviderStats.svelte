<script lang="ts">
  interface Provider {
    id: string;
    name: string;
    models: { id: string; name: string; max_tokens: number }[];
    requires_key: boolean;
    is_free: boolean;
    tier: string;
  }

  let {
    currentProvider,
    currentModelId,
    presetProviders,
    customProviders,
    userModelsMap,
  }: {
    currentProvider: string;
    currentModelId: string;
    presetProviders: Provider[];
    customProviders: Provider[];
    userModelsMap: Record<string, Array<{ id: string; name: string }>>;
  } = $props();

  let currentName = $derived(
    presetProviders.find((p: Provider) => p.id === currentProvider)?.name || currentProvider
  );

  let totalModels = $derived(
    presetProviders.reduce((sum, p) => sum + (p.models?.length || 0) + (userModelsMap[p.id]?.length || 0), 0)
  );

  let configuredCount = $derived(
    presetProviders.filter((p) => p.tier === 'free' || p.is_free).length
  );
</script>

<!-- Current Provider Info -->
<div class="current-provider">
  <span class="material-symbols-outlined text-[var(--color-primary)] text-lg">check_circle</span>
  <div class="flex-1 min-w-0">
    <p class="text-sm font-medium text-[var(--color-text)] truncate">
      当前: {currentName}
      {#if currentModelId}
        <span class="text-[var(--color-text-muted)]">/ {currentModelId}</span>
      {/if}
    </p>
  </div>
</div>

<!-- Stats Summary -->
<div class="flex gap-3 mb-5">
  <div class="flex-1 p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
    <p class="text-xs" style="color: var(--color-text-muted)">预设供应商</p>
    <p class="text-lg font-bold text-[var(--color-text)]">{presetProviders.length}</p>
  </div>
  <div class="flex-1 p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
    <p class="text-xs" style="color: var(--color-text-muted)">自定义供应商</p>
    <p class="text-lg font-bold text-[var(--color-text)]">{customProviders.length}</p>
  </div>
  <div class="flex-1 p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
    <p class="text-xs" style="color: var(--color-text-muted)">可用模型</p>
    <p class="text-lg font-bold text-[var(--color-text)]">{totalModels}</p>
  </div>
  <div class="flex-1 p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
    <p class="text-xs" style="color: var(--color-text-muted)">免费可用</p>
    <p class="text-lg font-bold text-[var(--color-text)]">{configuredCount}</p>
  </div>
</div>

<style>
  .current-provider {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    border-radius: 0.75rem;
    border: 1px solid var(--color-border);
    background: var(--gradient-brand-subtle);
    margin-bottom: 1rem;
  }
</style>

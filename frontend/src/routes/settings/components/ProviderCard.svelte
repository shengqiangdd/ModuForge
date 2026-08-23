<script lang="ts">
  interface Provider {
    id: string;
    name: string;
    endpoint: string;
    models: { id: string; name: string; max_tokens: number }[];
    requires_key: boolean;
    is_free: boolean;
    tier: string;
    models_json?: string;
    api_key?: string;
  }

  let {
    provider,
    isCurrent = false,
    config,
    userModels = [],
    status,
    onOpenModels,
    onOpenConfig,
    onResetConfig,
  }: {
    provider: Provider;
    isCurrent?: boolean;
    config?: { endpoint: string; api_key: string; models_json?: string };
    userModels?: Array<{ id: string; name: string }>;
    status: string;
    onOpenModels: () => void;
    onOpenConfig: () => void;
    onResetConfig: () => void;
  } = $props();

  let totalModels = $derived((provider.models?.length || 0) + userModels.length);

  function badgeClass(status: string) {
    switch (status) {
      case 'current': return 'bg-primary/20 text-primary';
      case 'free': return 'bg-green-500/20 text-green-400';
      case 'subscription': return 'bg-violet-500/20 text-violet-400';
      case 'configured': return 'bg-green-500/20 text-green-400';
      case 'needs_key': return 'bg-amber-500/20 text-amber-400';
      case 'ready': return 'bg-zinc-500/20 text-zinc-400';
      default: return 'bg-zinc-500/20 text-zinc-400';
    }
  }

  function badgeLabel(status: string) {
    switch (status) {
      case 'current': return '使用中';
      case 'free': return '免费';
      case 'subscription': return '订阅';
      case 'configured': return '已配置';
      case 'needs_key': return '需配置';
      case 'ready': return '就绪';
      default: return '-';
    }
  }
</script>

<tr class="border-t border-[var(--color-border)]">
  <td class="py-3 pr-4">
    <span class="font-medium text-[var(--color-text)]">{provider.name}</span>
  </td>
  <td class="py-3 pr-4 text-[var(--color-text-secondary)] hidden sm:table-cell">
    <button class="hover:text-[var(--color-primary)] transition-colors cursor-pointer" onclick={onOpenModels}>
      {totalModels}
      {#if userModels.length > 0}
        <span class="text-xs text-[var(--color-primary)]">(+{userModels.length})</span>
      {/if}
    </button>
  </td>
  <td class="py-3 pr-4">
    <span class="badge text-xs {badgeClass(status)}">
      {badgeLabel(status)}
    </span>
  </td>
  <td class="py-3 pr-4 max-w-[200px] truncate text-[var(--color-text-muted)] text-xs hidden md:table-cell" title={provider.endpoint}>
    {config?.endpoint || provider.endpoint || '-'}
  </td>
  <td class="py-3 text-right">
    <div class="flex items-center justify-end gap-1">
      <button class="btn-ghost text-xs px-2 sm:px-2.5 py-1.5 min-h-0" onclick={onOpenModels}>
        模型
      </button>
      <button class="btn-ghost text-xs px-2 sm:px-2.5 py-1.5 min-h-0" onclick={onOpenConfig}>配置</button>
      {#if config}
        <button class="btn-ghost text-xs px-2 sm:px-2.5 py-1.5 min-h-0 text-[var(--color-error)]" onclick={onResetConfig}>
          重置
        </button>
      {/if}
    </div>
  </td>
</tr>

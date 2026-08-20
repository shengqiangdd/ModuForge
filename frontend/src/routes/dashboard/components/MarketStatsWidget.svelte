<script lang="ts">
  import { t } from '$lib/i18n';

  let { data, loading }: { data: any; loading: boolean } = $props();
</script>

<div class="grid grid-cols-3 gap-2 mb-3">
  {#each [
    { label: $t('dashboard.total_modules'), value: data?.total_modules ?? 0 },
    { label: $t('dashboard.total_installs'), value: data?.total_installs ?? 0 },
    { label: $t('dashboard.total_stars'), value: data?.total_stars ?? 0 },
  ] as s}
    <div>
      <p class="text-xs text-[var(--color-text-muted)] mb-1">{s.label}</p>
      <p class="text-base font-bold text-[var(--color-text)] tabular-nums">{s.value}</p>
    </div>
  {/each}
</div>
{#if data?.top_categories?.length > 0}
  <div class="pt-2 border-t border-[var(--color-border)]">
    <p class="text-xs text-[var(--color-text-muted)] mb-2">{$t('dashboard.top_categories')}</p>
    <div class="space-y-2">
      {#each data.top_categories.slice(0, 4) as cat}
        {@const maxC = Math.max(...data.top_categories.map((c: any) => c.count))}
        <div class="flex items-center gap-2 text-xs">
          <span class="w-16 truncate" style="color: var(--color-text-secondary)">{cat.category}</span>
          <div class="flex-1 rounded-full h-1.5" style="background: var(--color-surface)">
            <div class="rounded-full h-1.5" style="width: {(cat.count / maxC) * 100}%; background: var(--gradient-brand)"></div>
          </div>
          <span class="text-[var(--color-text-muted)] w-6 text-right">{cat.count}</span>
        </div>
      {/each}
    </div>
  </div>
{/if}

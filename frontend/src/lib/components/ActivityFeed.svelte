<script lang="ts">
  import { t } from '$lib/i18n';

  let { activities = [], compact = false, loading = false }: {
    activities?: any[];
    compact?: boolean;
    loading?: boolean;
  } = $props();

  const activityIcons: Record<string, string> = {
    build_started: 'construction',
    build_completed: 'check_circle',
    build_failed: 'error',
    file_edited: 'edit_note',
    module_published: 'publish',
    module_reviewed: 'rate_review',
    member_added: 'person_add',
    member_removed: 'person_remove',
  };

  const activityColors: Record<string, string> = {
    build_started: 'var(--color-primary)',
    build_completed: 'var(--color-success)',
    build_failed: 'var(--color-error)',
    file_edited: 'var(--color-info)',
    module_published: 'var(--color-primary)',
    module_reviewed: 'var(--color-warning)',
    member_added: 'var(--color-success)',
    member_removed: 'var(--color-error)',
  };

  function timeAgo(dateStr: string) {
    const diff = Date.now() - new Date(dateStr).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return '刚刚';
    if (mins < 60) return `${mins} 分钟前`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs} 小时前`;
    const days = Math.floor(hrs / 24);
    if (days < 7) return `${days} 天前`;
    return new Date(dateStr).toLocaleDateString();
  }
</script>

<div class="space-y-1">
  {#if loading}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">{$t('common.loading')}</div>
  {:else if activities.length === 0}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">{$t('activity.empty')}</div>
  {:else}
    <div class="relative">
      <!-- Timeline vertical line -->
      <div class="absolute left-[11px] top-2 bottom-2 w-0.5" style="background: var(--color-border)"></div>

      <div class="space-y-0">
        {#each activities as act, i (act.id || i)}
          <div class="flex items-start gap-3 py-2.5">
            <!-- Timeline dot -->
            <div class="relative z-10 flex-shrink-0 w-6 h-6 rounded-full flex items-center justify-center"
                 style="background: {activityColors[act.activity_type] || 'var(--color-text-muted)'};">
              <span class="material-symbols-outlined text-white text-[12px]">
                {activityIcons[act.activity_type] || 'circle'}
              </span>
            </div>
            <!-- Content -->
            <div class="flex-1 min-w-0 pt-0.5">
              <p class="text-sm" style="color: var(--color-text)">{act.description}</p>
              <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">{timeAgo(act.created_at)}</p>
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

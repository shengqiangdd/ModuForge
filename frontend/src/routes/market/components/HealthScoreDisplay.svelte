<script lang="ts">
  import type { HealthScore } from './types';

  let { score }: { score: HealthScore | null } = $props();

  let color = $derived(
    score
      ? score.score >= 80 ? '#22c55e' : score.score >= 60 ? '#eab308' : '#ef4444'
      : 'var(--color-success)'
  );
</script>

<div class="p-6 border-b" style="border-color: var(--color-border)">
  <h3 class="text-sm font-semibold text-[var(--color-text)] mb-3">健康度评分</h3>
  {#if score}
    <div class="flex items-center gap-4">
      <div class="relative w-20 h-20 flex-shrink-0">
        <svg viewBox="0 0 36 36" class="w-full h-full -rotate-90">
          <circle cx="18" cy="18" r="15.5" fill="none" stroke="var(--color-surface)" stroke-width="3"/>
          <circle
            cx="18" cy="18" r="15.5" fill="none"
            stroke={color} stroke-width="3"
            stroke-dasharray={`${(score.score / 100) * 97.4} 97.4`}
            stroke-linecap="round"
          />
        </svg>
        <div class="absolute inset-0 flex items-center justify-center">
          <span class="text-xl font-bold" style="color: {color}">{score.score}</span>
        </div>
      </div>
      <div class="flex-1 space-y-1.5">
        {#each score.details as detail}
          <div class="flex items-center gap-2 text-xs">
            <span style="color: var(--color-text)">{detail.label}</span>
            <div class="flex-1 h-1.5 rounded-full" style="background: var(--color-surface)">
              <div
                class="h-full rounded-full transition-all"
                style="width: {(detail.score / detail.max) * 100}%; background: {detail.score >= detail.max / 2 ? 'var(--color-success)' : 'var(--color-warning)'}"
              ></div>
            </div>
            <span style="color: var(--color-text-muted)">{detail.score}/{detail.max}</span>
          </div>
        {/each}
      </div>
    </div>
  {:else}
    <p class="text-xs text-[var(--color-text-muted)]">加载中...</p>
  {/if}
</div>

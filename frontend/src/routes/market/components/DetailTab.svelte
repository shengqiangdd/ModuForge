<script lang="ts">
  import HealthScoreDisplay from './HealthScoreDisplay.svelte';
  import ReviewsSection from './ReviewsSection.svelte';
  import type { HealthScore, ModuleTag, Review, MarketModule } from './types';

  let {
    module: mod,
    healthScore,
    moduleTags = [],
    reviews = [],
    reviewsLoading = false,
    newReviewRating = $bindable(5),
    newReviewComment = $bindable(''),
    submittingReview = false,
    onSubmitReview,
  }: {
    module: MarketModule;
    healthScore: HealthScore | null;
    moduleTags?: ModuleTag[];
    reviews?: Review[];
    reviewsLoading?: boolean;
    newReviewRating?: number;
    newReviewComment?: string;
    submittingReview?: boolean;
    onSubmitReview?: () => void;
  } = $props();
</script>

<!-- Health Score -->
<HealthScoreDisplay score={healthScore} />

<!-- Description & Tags -->
<div class="p-6 border-b" style="border-color: var(--color-border)">
  <h3 class="text-sm font-semibold text-[var(--color-text)] mb-2">描述</h3>
  <p class="text-sm text-[var(--color-text-secondary)] leading-relaxed">{mod.description}</p>
  {#if moduleTags.length > 0}
    <div class="flex flex-wrap gap-1.5 mt-3">
      {#each moduleTags as tag}
        <span class="px-2.5 py-1 rounded-lg text-xs" style="background: {tag.color}20; color: {tag.color}">{tag.name}</span>
      {/each}
    </div>
  {:else if mod.tags}
    <div class="flex flex-wrap gap-1.5 mt-3">
      {#each mod.tags.split(',') as tag}
        <span class="px-2.5 py-1 rounded-lg text-xs" style="background: var(--color-surface); color: var(--color-text-muted)">{tag.trim()}</span>
      {/each}
    </div>
  {/if}
</div>

<!-- Reviews -->
<ReviewsSection
  {reviews}
  {reviewsLoading}
  bind:newReviewRating
  bind:newReviewComment
  {submittingReview}
  onSubmit={onSubmitReview}
/>

<script lang="ts">
  import type { Review } from './types';

  let {
    reviews = [],
    reviewsLoading = false,
    newReviewRating = $bindable(5),
    newReviewComment = $bindable(''),
    submittingReview = false,
    onSubmit,
  }: {
    reviews?: Review[];
    reviewsLoading?: boolean;
    newReviewRating?: number;
    newReviewComment?: string;
    submittingReview?: boolean;
    onSubmit?: () => void;
  } = $props();
</script>

<div class="p-6">
  <h3 class="text-sm font-semibold text-[var(--color-text)] mb-4">评论</h3>
  {#if reviewsLoading}
    <div class="flex justify-center py-6"><div class="skeleton h-4 w-32"></div></div>
  {:else if reviews.length === 0}
    <p class="text-sm text-[var(--color-text-muted)] mb-4">暂无评论</p>
  {:else}
    <div class="space-y-2 mb-4 max-h-48 overflow-auto">
      {#each reviews as rev (rev.id)}
        <div class="p-3 rounded-xl" style="background: var(--color-surface)">
          <div class="flex items-center gap-2 mb-1">
            <span class="text-sm font-medium text-[var(--color-text)]">{rev.username}</span>
            <span class="text-xs text-amber-500">{'★'.repeat(rev.rating)}{'☆'.repeat(5 - rev.rating)}</span>
          </div>
          <p class="text-sm text-[var(--color-text-secondary)]">{rev.comment}</p>
        </div>
      {/each}
    </div>
  {/if}

  <div class="border-t border-[var(--color-border)] pt-4">
    <div class="flex items-center gap-2 mb-3">
      <span class="text-sm font-medium">评分</span>
      {#each [1, 2, 3, 4, 5] as star}
        <button
          class="text-xl transition-colors {star <= newReviewRating ? 'text-amber-500' : 'text-neutral-300'}"
          onclick={() => newReviewRating = star}
        >★</button>
      {/each}
    </div>
    <textarea
      class="input-field resize-none"
      rows="3"
      placeholder="写下你的评价..."
      bind:value={newReviewComment}
    ></textarea>
    <div class="flex justify-end mt-3">
      <button
        class="btn-primary text-sm disabled:opacity-50"
        disabled={submittingReview || !newReviewComment.trim()}
        onclick={onSubmit}
      >
        {submittingReview ? '提交中...' : '提交评论'}
      </button>
    </div>
  </div>
</div>

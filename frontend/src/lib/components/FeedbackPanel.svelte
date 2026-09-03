<script>
  import { onMount } from 'svelte';

  let rating = $state(0);
  let comment = $state('');
  let submitted = $state(false);
  let stats = $state(null);
  let loading = $state(false);

  async function submitFeedback() {
    if (rating === 0) return;

    loading = true;
    try {
      const res = await fetch('/api/feedback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          rating,
          comment,
          session_id: 'current',
          accepted: true,
          modified: false
        })
      });

      if (res.ok) {
        submitted = true;
        await fetchStats();
      }
    } catch (e) {
      console.error('Failed to submit feedback:', e);
    }
    loading = false;
  }

  async function fetchStats() {
    try {
      const res = await fetch('/api/feedback/stats');
      if (res.ok) stats = await res.json();
    } catch (e) {
      console.error('Failed to fetch stats:', e);
    }
  }

  onMount(fetchStats);
</script>

<div class="feedback-panel">
  <h2 class="text-xl font-semibold mb-4">💬 用户反馈</h2>

  {#if submitted}
    <div class="card mb-4 bg-green-50">
      <div class="text-green-700">感谢您的反馈！</div>
    </div>
  {:else}
    <div class="card mb-4">
      <h3 class="card-title">评价 AI 生成的代码</h3>

      <!-- Rating -->
      <div class="mb-4">
        <div class="text-sm text-gray-500 mb-2">评分</div>
        <div class="flex gap-2">
          {#each [1, 2, 3, 4, 5] as star}
            <button
              onclick={() => rating = star}
              class="text-2xl {rating >= star ? 'text-yellow-500' : 'text-gray-300'}"
            >
              ★
            </button>
          {/each}
        </div>
      </div>

      <!-- Comment -->
      <div class="mb-4">
        <div class="text-sm text-gray-500 mb-2">评论（可选）</div>
        <textarea
          bind:value={comment}
          placeholder="分享您的想法..."
          rows="3"
          class="w-full px-3 py-2 border rounded"
        ></textarea>
      </div>

      <button
        onclick={submitFeedback}
        disabled={loading || rating === 0}
        class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50"
      >
        {loading ? '提交中...' : '提交反馈'}
      </button>
    </div>
  {/if}

  <!-- Stats -->
  {#if stats && stats.total > 0}
    <div class="card">
      <h3 class="card-title">反馈统计</h3>
      <div class="grid grid-cols-2 gap-4 text-sm">
        <div>
          <div class="text-gray-500">总反馈数</div>
          <div class="text-lg font-semibold">{stats.total}</div>
        </div>
        <div>
          <div class="text-gray-500">平均评分</div>
          <div class="text-lg font-semibold">{stats.avg_rating.toFixed(1)} ★</div>
        </div>
        <div>
          <div class="text-gray-500">接受率</div>
          <div class="text-lg font-semibold">{stats.acceptance_rate.toFixed(1)}%</div>
        </div>
        <div>
          <div class="text-gray-500">修改率</div>
          <div class="text-lg font-semibold">{stats.modification_rate.toFixed(1)}%</div>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .feedback-panel { padding: 1rem; }
  .card { background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 0.5rem; padding: 1rem; }
  .card-title { font-size: 1rem; font-weight: 600; margin-bottom: 0.75rem; }
</style>

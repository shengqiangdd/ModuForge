<script lang="ts">
let {
  show = false,
  capability = null,
  loading = false,
  onClose,
}: {
  show: boolean;
  capability: any;
  loading: boolean;
  onClose: () => void;
} = $props();
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-lg border border-[var(--color-border)]" role="dialog" aria-modal="true" tabindex="-1">
      <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)]">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-primary-600">speed</span>
          <h2 class="text-lg font-semibold text-[var(--color-text)]">AI 能力评分</h2>
        </div>
        <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="px-6 py-5">
        {#if loading}
          <div class="flex items-center justify-center py-8">
            <span class="material-symbols-outlined animate-spin text-primary-500">progress_activity</span>
          </div>
        {:else if capability}
          <div class="flex items-center justify-center mb-5">
            <div class="relative">
              <div class="w-24 h-24 rounded-full flex items-center justify-center text-4xl font-bold
                {capability.grade === 'S' ? 'bg-gradient-to-br from-yellow-400 to-amber-500 text-white' :
                 capability.grade === 'A' ? 'bg-gradient-to-br from-green-400 to-emerald-500 text-white' :
                 capability.grade === 'B' ? 'bg-gradient-to-br from-blue-400 to-blue-500 text-white' :
                 capability.grade === 'C' ? 'bg-gradient-to-br from-orange-400 to-orange-500 text-white' :
                 'bg-gradient-to-br from-red-400 to-red-500 text-white'}">
                {capability.grade}
              </div>
              <div class="absolute -bottom-1 left-1/2 -translate-x-1/2 px-2 py-0.5 rounded-full text-[10px] font-medium bg-[var(--color-bg)] border border-[var(--color-border)] text-[var(--color-text-secondary)]">
                {capability.total_score}/100
              </div>
            </div>
          </div>

          <div class="text-center mb-5">
            <p class="text-sm text-[var(--color-text-secondary)]">当前：{capability.current_provider || '未配置'} / {capability.current_model || '未配置'}</p>
          </div>

          <div class="space-y-3 mb-5">
            {#each [
              { label: '模型能力', score: capability.model_score, max: 40, icon: 'smart_toy' },
              { label: '响应速度', score: capability.speed_score, max: 20, icon: 'bolt' },
              { label: '成本效率', score: capability.cost_score, max: 20, icon: 'savings' },
              { label: '功能完整度', score: capability.feature_score, max: 20, icon: 'extension' }
            ] as dim}
              <div>
                <div class="flex items-center justify-between mb-1">
                  <div class="flex items-center gap-1.5">
                    <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]">{dim.icon}</span>
                    <span class="text-xs text-[var(--color-text-secondary)]">{dim.label}</span>
                  </div>
                  <span class="text-xs font-medium text-[var(--color-text)]">{dim.score}/{dim.max}</span>
                </div>
                <div class="h-2 rounded-full bg-[var(--color-surface)] overflow-hidden">
                  <div class="h-full rounded-full transition-all duration-500
                    {dim.score / dim.max >= 0.75 ? 'bg-green-500' : dim.score / dim.max >= 0.5 ? 'bg-blue-500' : dim.score / dim.max >= 0.25 ? 'bg-orange-500' : 'bg-red-500'}"
                    style="width: {Math.max(4, (dim.score / dim.max) * 100)}%"></div>
                </div>
              </div>
            {/each}
          </div>

          {#if capability.suggestions && capability.suggestions.length > 0}
            <div class="bg-[var(--color-surface)] rounded-xl p-4">
              <p class="text-xs font-medium text-[var(--color-text-secondary)] mb-2">💡 优化建议</p>
              <ul class="space-y-1.5">
                {#each capability.suggestions as s}
                  <li class="text-xs text-[var(--color-text)] flex items-start gap-1.5">
                    <span class="material-symbols-outlined text-[12px] text-primary-500 mt-0.5 shrink-0">arrow_forward</span>
                    {s}
                  </li>
                {/each}
              </ul>
            </div>
          {/if}
        {:else}
          <div class="text-center py-8 text-sm text-[var(--color-text-muted)]">暂无数据</div>
        {/if}
      </div>
    </div>
  </div>
{/if}

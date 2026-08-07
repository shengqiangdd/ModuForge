<script lang="ts">
  let { show, loading, data, visibleLines, onClose, onInstall }: {
    show: boolean;
    loading: boolean;
    data: { error?: string; simulated_output?: string; props?: { path: string; prop: string; before: string; after: string }[]; files?: string[] } | null;
    visibleLines: number;
    onClose: () => void;
    onInstall: () => void;
  } = $props();
</script>

{#if show}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
    <div class="rounded-2xl max-w-lg w-full max-h-[90vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)] flex items-center gap-2">
          <span class="material-symbols-outlined text-[20px] text-purple-500">preview</span>
          试用预览
        </h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      {#if loading}
        <div class="flex justify-center py-12">
          <div class="animate-spin h-8 w-8 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
        </div>
      {:else if data?.error}
        <div class="p-8 text-center text-[var(--color-error)]">{data.error}</div>
      {:else}
        <div class="p-5 space-y-5">
          <!-- Simulated Install Output -->
          <div>
            <h4 class="text-sm font-semibold text-[var(--color-text)] mb-2">模拟安装过程</h4>
            <pre class="p-3 rounded-xl text-xs font-mono leading-relaxed overflow-auto max-h-40 whitespace-pre-wrap" style="background: #0a0a0a; color: #4ade80">
              {#each (data?.simulated_output || '').split('\n').slice(0, visibleLines) as line}
                {line}
                <br>
              {/each}
              {#if visibleLines < (data?.simulated_output || '').split('\n').length}
                <span class="animate-pulse">▊</span>
              {:else}
                <span class="text-green-300">✓ 模拟完成</span>
              {/if}
            </pre>
          </div>
          <!-- Props Comparison -->
          {#if data?.props?.length}
            <div>
              <h4 class="text-sm font-semibold text-[var(--color-text)] mb-2">修改的系统属性</h4>
              <div class="space-y-2">
                {#each data.props as prop}
                  <div class="p-3 rounded-xl" style="background: var(--color-surface)">
                    <div class="text-xs font-mono text-[var(--color-text-muted)] mb-1">{prop.path}</div>
                    <div class="flex items-center gap-2 text-sm">
                      <span class="font-medium text-[var(--color-text)]">{prop.prop}</span>
                      <span class="text-xs text-red-500 line-through">{prop.before}</span>
                      <span class="text-xs text-neutral-500">→</span>
                      <span class="text-xs text-green-500 font-semibold">{prop.after}</span>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          {/if}
          <!-- Affected Files -->
          {#if data?.files?.length}
            <div>
              <h4 class="text-sm font-semibold text-[var(--color-text)] mb-2">影响的文件路径</h4>
              <div class="space-y-1">
                {#each data.files as file}
                  <div class="flex items-center gap-2 p-2 rounded-lg text-xs font-mono" style="background: var(--color-surface); color: var(--color-text-secondary)">
                    <span class="material-symbols-outlined text-[14px]">description</span>
                    {file}
                  </div>
                {/each}
              </div>
            </div>
          {/if}
          <!-- Install Button -->
          <div class="flex gap-3 pt-2">
            <button class="flex-1 py-2.5 rounded-xl font-semibold text-sm text-white transition-all flex items-center justify-center gap-2" style="background: var(--gradient-brand)" onclick={onInstall}>
              <span class="material-symbols-outlined text-[16px]">smartphone</span>
              实际安装
            </button>
            <button class="py-2.5 px-5 rounded-xl text-sm font-medium transition-colors" style="border: 1px solid var(--color-border); color: var(--color-text-secondary)" onclick={onClose}>
              关闭
            </button>
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}
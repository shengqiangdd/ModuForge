<script lang="ts">
let {
  show = false,
  spec = null,
  onClose,
  onGenerate,
}: {
  show: boolean;
  spec: any;
  onClose: () => void;
  onGenerate: () => void;
} = $props();
</script>

{#if show && spec}
  <div class="border-t border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-4 py-3" style="animation: fadeInUp 0.3s ease-out;">
    <div class="rounded-xl border border-primary-500/30 overflow-hidden" style="background: color-mix(in srgb, var(--color-primary) 5%, var(--color-bg))">
      <div class="flex items-center justify-between px-4 py-2.5 border-b border-primary-500/20">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-[16px] text-primary-500">checklist</span>
          <span class="text-sm font-semibold text-[var(--color-text)]">需求规格</span>
          <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-primary-500/20 text-primary-600 font-medium">已收集</span>
        </div>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[14px]">close</span>
        </button>
      </div>
      <div class="px-4 py-3 space-y-3">
        <div class="flex items-center gap-2">
          <span class="text-xs font-medium text-[var(--color-text-secondary)] w-20 shrink-0">模块名称</span>
          <span class="text-sm font-semibold text-[var(--color-text)]">{spec.module_name || '未命名'}</span>
        </div>
        {#if spec.description}
          <div class="flex items-start gap-2">
            <span class="text-xs font-medium text-[var(--color-text-secondary)] w-20 shrink-0 mt-0.5">描述</span>
            <span class="text-xs text-[var(--color-text-secondary)]">{spec.description}</span>
          </div>
        {/if}
        {#if spec.target_frameworks && spec.target_frameworks.length > 0}
          <div class="flex items-start gap-2">
            <span class="text-xs font-medium text-[var(--color-text-secondary)] w-20 shrink-0 mt-0.5">目标框架</span>
            <div class="flex flex-wrap gap-1">
              {#each spec.target_frameworks as fw}
                <span class="text-[10px] px-2 py-0.5 rounded-full font-medium" style="background: color-mix(in srgb, var(--color-primary) 15%, transparent); color: var(--color-primary)">{fw}</span>
              {/each}
            </div>
          </div>
        {/if}
        {#if spec.features && spec.features.length > 0}
          <div class="flex items-start gap-2">
            <span class="text-xs font-medium text-[var(--color-text-secondary)] w-20 shrink-0 mt-0.5">功能</span>
            <div class="space-y-1">
              {#each spec.features as feat}
                <div class="flex items-start gap-1.5">
                  <span class="material-symbols-outlined text-[12px] text-primary-500 mt-0.5">check_circle</span>
                  <span class="text-xs text-[var(--color-text)]">{feat}</span>
                </div>
              {/each}
            </div>
          </div>
        {/if}
        {#if spec.ui_required}
          <div class="flex items-center gap-2">
            <span class="text-xs font-medium text-[var(--color-text-secondary)] w-20 shrink-0">WebUI</span>
            <span class="text-xs px-2 py-0.5 rounded-full bg-primary-500/20 text-primary-600">需要 WebUI</span>
          </div>
        {/if}
        {#if spec.special_requirements}
          <div class="flex items-start gap-2">
            <span class="text-xs font-medium text-[var(--color-text-secondary)] w-20 shrink-0 mt-0.5">特殊需求</span>
            <span class="text-xs text-[var(--color-text-secondary)]">{spec.special_requirements}</span>
          </div>
        {/if}
      </div>
      <div class="px-4 py-2.5 border-t border-primary-500/20 flex justify-end gap-2">
        <button class="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-xs font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors" onclick={onGenerate}>
          <span class="material-symbols-outlined text-[14px]">auto_fix_high</span>
          一键生成模块
        </button>
      </div>
    </div>
  </div>
{/if}

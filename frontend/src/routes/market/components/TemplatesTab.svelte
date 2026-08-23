<script lang="ts">
  import type { TemplateItem, TemplateCategory } from './types';

  let {
    templateList = [],
    templateTotal = 0,
    templateLoading = false,
    templateSearch = $bindable(''),
    templateCategory = $bindable(''),
    templateSort = $bindable('downloads'),
    templatePage = $bindable(1),
    templateCategories = [],
    onShowPublish,
    onUseTemplate,
    onRateTemplate,
    onLoadTemplates,
  }: {
    templateList?: TemplateItem[];
    templateTotal?: number;
    templateLoading?: boolean;
    templateSearch?: string;
    templateCategory?: string;
    templateSort?: string;
    templatePage?: number;
    templateCategories?: TemplateCategory[];
    onShowPublish?: () => void;
    onUseTemplate?: (t: TemplateItem) => void;
    onRateTemplate?: (t: TemplateItem, rating: number) => void;
    onLoadTemplates?: () => void;
  } = $props();
</script>

<div class="p-6">
  <div class="flex items-center justify-between mb-4">
    <h3 class="text-sm font-semibold text-[var(--color-text)]">模板市场</h3>
    <button
      class="flex items-center gap-1 px-3 py-1.5 rounded-xl text-xs font-medium text-white transition-colors"
      style="background: var(--gradient-brand)"
      onclick={onShowPublish}
    >
      <span class="material-symbols-outlined text-[14px]">add</span>
      发布模板
    </button>
  </div>

  <!-- Search and Filter -->
  <div class="flex flex-col sm:flex-row gap-2 mb-4">
    <input
      type="text"
      placeholder="搜索模板..."
      class="flex-1 min-w-0 px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] text-sm"
      bind:value={templateSearch}
      oninput={() => { templatePage = 1; onLoadTemplates?.(); }}
    />
    <div class="flex gap-2">
      <select
        class="flex-1 sm:flex-none min-w-0 px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] text-sm"
        bind:value={templateCategory}
        onchange={() => { templatePage = 1; onLoadTemplates?.(); }}
      >
        <option value="">全部分类</option>
        {#each templateCategories as cat}
          <option value={cat.name}>{cat.name} ({cat.count})</option>
        {/each}
      </select>
      <select
        class="flex-1 sm:flex-none min-w-0 px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] text-sm"
        bind:value={templateSort}
        onchange={() => onLoadTemplates?.()}
      >
        <option value="downloads">最多下载</option>
        <option value="rating">最高评分</option>
        <option value="newest">最新发布</option>
        <option value="name">名称排序</option>
      </select>
    </div>
  </div>

  {#if templateLoading}
    <p class="text-xs text-[var(--color-text-muted)]">加载中...</p>
  {:else if templateList.length === 0}
    <p class="text-xs text-[var(--color-text-muted)] text-center py-8">暂无模板</p>
  {:else}
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      {#each templateList as t (t.id)}
        <div class="p-3 border border-[var(--color-border)] rounded-lg hover:border-primary/50 transition-colors">
          <div class="flex items-start justify-between mb-2">
            <div class="flex-1 min-w-0">
              <h4 class="text-sm font-medium text-[var(--color-text)] truncate">{t.name}</h4>
              <p class="text-xs text-[var(--color-text-muted)] mt-0.5 line-clamp-2">{t.description || '暂无描述'}</p>
            </div>
          </div>
          <div class="flex items-center gap-2 text-xs text-[var(--color-text-muted)] mb-2">
            <span class="material-symbols-outlined text-xs">person</span>
            <span>{t.author}</span>
            <span>·</span>
            <span class="material-symbols-outlined text-xs">download</span>
            <span>{t.downloads}</span>
            <span>·</span>
            <span class="material-symbols-outlined text-xs">star</span>
            <span>{t.rating.toFixed(1)}</span>
          </div>
          <div class="flex items-center gap-2">
            <button class="btn-ghost text-xs flex-1" onclick={() => onUseTemplate?.(t)}>使用</button>
            <button class="btn-ghost text-xs" onclick={() => onRateTemplate?.(t, 5)}>⭐</button>
          </div>
        </div>
      {/each}
    </div>

    {#if templateTotal > 20}
      <div class="flex justify-center gap-2 mt-4">
        <button class="btn-ghost text-xs" onclick={() => { templatePage--; onLoadTemplates?.(); }} disabled={templatePage <= 1}>上一页</button>
        <span class="text-xs text-[var(--color-text-muted)] py-1">第 {templatePage} 页 / 共 {Math.ceil(templateTotal / 20)} 页</span>
        <button class="btn-ghost text-xs" onclick={() => { templatePage++; onLoadTemplates?.(); }} disabled={templatePage * 20 >= templateTotal}>下一页</button>
      </div>
    {/if}
  {/if}
</div>

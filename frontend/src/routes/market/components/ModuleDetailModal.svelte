<script lang="ts">
  import ScreenshotGallery from './ScreenshotGallery.svelte';
  import DetailTab from './DetailTab.svelte';
  import ChangelogsTab from './ChangelogsTab.svelte';
  import StatsTab from './StatsTab.svelte';
  import TemplatesTab from './TemplatesTab.svelte';
  import type {
    MarketModule, HealthScore, ModuleTag, Review, ChangelogEntry,
    InstallStat, TemplateItem, TemplateCategory,
  } from './types';
  import { fmt, categoryStyles } from './types';

  let {
    module: mod,
    healthScore,
    moduleTags = [],
    reviews = [],
    reviewsLoading = false,
    newReviewRating = $bindable(5),
    newReviewComment = $bindable(''),
    submittingReview = false,
    changelogs = [],
    changelogsLoading = false,
    installStats = [],
    statsPeriod = 'day',
    statsLoading = false,
    trendingModules = [],
    templateList = [],
    templateTotal = 0,
    templateLoading = false,
    templateSearch = $bindable(''),
    templateCategory = $bindable(''),
    templateSort = $bindable('downloads'),
    templatePage = $bindable(1),
    templateCategories = [],
    onClose,
    onStar,
    onSubmitReview,
    onLoadVersions,
    onLoadChangelogs,
    onLoadInstallStats,
    onStatsPeriodChange,
    onLoadTemplates,
    onLoadTemplateCategories,
    onShowPublishTemplate,
    onUseTemplate,
    onRateTemplate,
    onOpenDemo,
    onOpenInstallModal,
    onFullscreenScreenshot,
  }: {
    module: MarketModule | null;
    healthScore: HealthScore | null;
    moduleTags?: ModuleTag[];
    reviews?: Review[];
    reviewsLoading?: boolean;
    newReviewRating?: number;
    newReviewComment?: string;
    submittingReview?: boolean;
    changelogs?: ChangelogEntry[];
    changelogsLoading?: boolean;
    installStats?: InstallStat[];
    statsPeriod?: 'day' | 'week' | 'month';
    statsLoading?: boolean;
    trendingModules?: MarketModule[];
    templateList?: TemplateItem[];
    templateTotal?: number;
    templateLoading?: boolean;
    templateSearch?: string;
    templateCategory?: string;
    templateSort?: string;
    templatePage?: number;
    templateCategories?: TemplateCategory[];
    onClose?: () => void;
    onStar?: () => void;
    onSubmitReview?: () => void;
    onLoadVersions?: (slug: string) => void;
    onLoadChangelogs?: (slug: string) => void;
    onLoadInstallStats?: (slug: string) => void;
    onStatsPeriodChange?: (period: 'day' | 'week' | 'month') => void;
    onLoadTemplates?: () => void;
    onLoadTemplateCategories?: () => void;
    onShowPublishTemplate?: () => void;
    onUseTemplate?: (t: TemplateItem) => void;
    onRateTemplate?: (t: TemplateItem, rating: number) => void;
    onOpenDemo?: (slug: string) => void;
    onOpenInstallModal?: () => void;
    onFullscreenScreenshot?: (url: string) => void;
  } = $props();

  let detailTab = $state<'detail' | 'changelogs' | 'stats' | 'templates'>('detail');
</script>

{#if mod}
  <div
    class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]"
    style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget) onClose?.(); }}
  >
    <div
      class="rounded-2xl max-w-2xl w-full max-h-[85vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]"
      style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <!-- Header -->
      <div class="p-6 border-b" style="border-color: var(--color-border)">
        <div class="flex items-start justify-between">
          <div>
            <h2 class="text-xl font-bold text-[var(--color-text)]">{mod.title}</h2>
            <div class="flex flex-wrap items-center gap-2 mt-2 text-sm text-[var(--color-text-muted)]">
              <span>{mod.version}</span>
              <span>·</span>
              <span class="badge" style={categoryStyles[mod.category] || ''}>{mod.category}</span>
              <span>·</span>
              <span>{mod.author}</span>
              <span>·</span>
              <span>{mod.license}</span>
            </div>
          </div>
          <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={() => onClose?.()}>
            <span class="material-symbols-outlined text-[20px]">close</span>
          </button>
        </div>

        <!-- Screenshot Gallery -->
        <ScreenshotGallery
          screenshots={mod.screenshots || []}
          coverImage={mod.cover_image || ''}
          onFullscreen={onFullscreenScreenshot}
        />

        <!-- Dependencies -->
        {#if mod.dependencies && mod.dependencies.length > 0}
          <div class="mt-4 p-3 rounded-xl" style="background: var(--color-surface)">
            <p class="text-xs font-medium text-[var(--color-text-secondary)] mb-2">依赖</p>
            <div class="flex flex-wrap gap-1.5">
              {#each mod.dependencies as dep}
                <span class="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-lg" style="background: var(--color-primary-light); color: var(--color-primary)">
                  {dep.id}
                  {#if dep.min_version}
                    <span class="opacity-60">&ge; {dep.min_version}</span>
                  {/if}
                  {#if dep.optional}
                    <span class="text-[10px] opacity-50">(可选)</span>
                  {/if}
                </span>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Action buttons -->
        <div class="flex flex-wrap items-center gap-3 mt-4">
          <button class="flex items-center gap-1.5 text-sm font-medium hover:text-primary-600 transition-colors" onclick={onStar}>
            <span class="material-symbols-outlined text-[18px]">star</span>
            {mod.stars} Stars
          </button>
          <span class="flex items-center gap-1.5 text-sm text-[var(--color-text-secondary)]">
            <span class="material-symbols-outlined text-[18px]">download</span>
            {fmt(mod.installs)} 安装
          </span>
          <button class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-sm font-medium whitespace-nowrap transition-colors" style="color: var(--color-text-secondary); background: var(--color-surface)" onclick={() => onLoadVersions?.(mod.slug)}>
            <span class="material-symbols-outlined text-[16px]">history</span>
            版本历史
          </button>
          <button class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-sm font-medium whitespace-nowrap transition-colors" style="color: var(--color-text-secondary); background: var(--color-surface)" onclick={() => onOpenDemo?.(mod.slug)}>
            <span class="material-symbols-outlined text-[16px]">preview</span>
            试用预览
          </button>
          <button class="flex-1 sm:flex-none ml-auto flex items-center justify-center gap-1.5 px-4 py-1.5 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--gradient-brand)" onclick={onOpenInstallModal}>
            <span class="material-symbols-outlined text-[16px]">download</span>
            安装
          </button>
        </div>
      </div>

      <!-- Tab Navigation -->
      <div class="flex border-b" style="border-color: var(--color-border)">
        <button class="flex-1 py-3 text-sm font-medium text-center transition-colors" style={detailTab === 'detail' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)' : 'color: var(--color-text-muted)'} onclick={() => detailTab = 'detail'}>详情</button>
        <button class="flex-1 py-3 text-sm font-medium text-center transition-colors" style={detailTab === 'changelogs' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)' : 'color: var(--color-text-muted)'} onclick={() => { detailTab = 'changelogs'; onLoadChangelogs?.(mod.slug); }}>更新日志</button>
        <button class="flex-1 py-3 text-sm font-medium text-center transition-colors" style={detailTab === 'stats' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)' : 'color: var(--color-text-muted)'} onclick={() => { detailTab = 'stats'; onLoadInstallStats?.(mod.slug); }}>统计</button>
        <button class="flex-1 py-3 text-sm font-medium text-center transition-colors" style={detailTab === 'templates' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)' : 'color: var(--color-text-muted)'} onclick={() => { detailTab = 'templates'; onLoadTemplates?.(); onLoadTemplateCategories?.(); }}>模板市场</button>
      </div>

      <!-- Tab Content -->
      {#if detailTab === 'detail'}
        <DetailTab
          module={mod}
          {healthScore}
          {moduleTags}
          {reviews}
          {reviewsLoading}
          bind:newReviewRating
          bind:newReviewComment
          {submittingReview}
          {onSubmitReview}
        />
      {:else if detailTab === 'changelogs'}
        <ChangelogsTab {changelogs} {changelogsLoading} />
      {:else if detailTab === 'stats'}
        <StatsTab
          {installStats}
          {statsPeriod}
          {statsLoading}
          {trendingModules}
          onPeriodChange={onStatsPeriodChange}
        />
      {:else if detailTab === 'templates'}
        <TemplatesTab
          {templateList}
          {templateTotal}
          {templateLoading}
          bind:templateSearch
          bind:templateCategory
          bind:templateSort
          bind:templatePage
          {templateCategories}
          onShowPublish={onShowPublishTemplate}
          {onUseTemplate}
          {onRateTemplate}
          {onLoadTemplates}
        />
      {/if}
    </div>
  </div>
{/if}

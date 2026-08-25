<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import MarketHeader from './components/MarketHeader.svelte';
  import ModuleGrid from './components/ModuleGrid.svelte';
  import ModuleDetailModal from './components/ModuleDetailModal.svelte';
  import InstallModal from './components/InstallModal.svelte';
  import PublishTemplateModal from './components/PublishTemplateModal.svelte';
  import BatchResultsModal from './components/BatchResultsModal.svelte';
  import VersionHistoryModal from './components/VersionHistoryModal.svelte';
  import CompareModal from './components/CompareModal.svelte';
  import DemoModal from './components/DemoModal.svelte';
  import { categoryStyles, type MarketModule, type ModuleVersion, type HealthScore, type ModuleTag, type ChangelogEntry, type InstallStat, type TemplateItem, type TemplateCategory } from './components/types';
  import * as api from './components/store';

  // ─── List state ───
  let modules = $state<MarketModule[]>([]);
  let total = $state(0);
  let loading = $state(true);
  let searchQuery = $state('');
  let selectedCategory = $state('');
  let sortBy = $state('stars');
  let page = $state(1);
  const perPage = 20;

  // ─── Detail state ───
  let selectedModule = $state<MarketModule | null>(null);
  let reviews = $state<any[]>([]);
  let reviewsLoading = $state(false);
  let newReviewRating = $state(5);
  let newReviewComment = $state('');
  let submittingReview = $state(false);
  let healthScore = $state<HealthScore | null>(null);
  let moduleTags = $state<ModuleTag[]>([]);

  // ─── Versions ───
  let showVersions = $state(false);
  let versions = $state<ModuleVersion[]>([]);
  let versionsLoading = $state(false);
  let rollingBack = $state<string | null>(null);
  let showVersionUpdate = $state(false);
  let newVersionCode = $state('');
  let newChangelog = $state('');
  let updatingVersion = $state(false);

  // ─── Favorites ───
  let favoritedModules = $state<Set<string>>(new Set());

  // ─── Tags ───
  let allTags = $state<{ id: number; name: string; color: string; usage_count: number }[]>([]);
  let selectedTag = $state<number | null>(null);

  // ─── Changelogs & Stats ───
  let changelogs = $state<ChangelogEntry[]>([]);
  let changelogsLoading = $state(false);
  let installStats = $state<InstallStat[]>([]);
  let statsPeriod = $state<'day' | 'week' | 'month'>('day');
  let trendingModules = $state<MarketModule[]>([]);
  let statsLoading = $state(false);

  // ─── Batch ───
  let selectedSlugs = $state<Set<string>>(new Set());
  let batchProcessing = $state(false);
  let batchResults = $state<{ slug: string; status: string; error?: string }[]>([]);
  let showBatchResults = $state(false);

  // ─── Install ───
  let showInstallModal = $state(false);
  let installSteps = $state<Array<{ label: string; status: 'pending' | 'running' | 'done' | 'error'; detail?: string }>>([]);
  let installDevice = $state('');
  let installableDevices = $state<Array<{ serial: string; model: string; state: string }>>([]);
  let loadingDevices = $state(false);
  let installing = $state(false);
  let installError = $state('');

  // ─── Templates ───
  let templateList = $state<TemplateItem[]>([]);
  let templateTotal = $state(0);
  let templateLoading = $state(false);
  let templateSearch = $state('');
  let templateCategory = $state('');
  let templateSort = $state('downloads');
  let templatePage = $state(1);
  let showPublishTemplate = $state(false);
  let publishName = $state('');
  let publishDesc = $state('');
  let publishCategory = $state('');
  let publishing = $state(false);
  let templateCategories = $state<TemplateCategory[]>([]);

  // ─── Misc modals ───
  let fullscreenScreenshot = $state<string | null>(null);
  let showDemo = $state(false);
  let demoLoading = $state(false);
  let demoData = $state<any>(null);
  let demoVisibleLines = $state(0);
  let demoInterval = $state<ReturnType<typeof setInterval> | null>(null);
  let compareIds = $state<Set<string>>(new Set());
  let compareLoading = $state(false);
  let compareResult = $state<any>(null);

  const categories = [
    { value: '', label: '全部', icon: 'apps' },
    { value: 'system', label: '系统', icon: 'phone_android' },
    { value: 'ui', label: '界面', icon: 'palette' },
    { value: 'audio', label: '音频', icon: 'headphones' },
    { value: 'display', label: '显示', icon: 'brightness_6' },
    { value: 'utility', label: '工具', icon: 'build' },
  ];

  // ═══════════════════════════════════════════
  //  Callbacks
  // ═══════════════════════════════════════════

  async function loadModules() {
    loading = true;
    const r = await api.fetchModules({ page, perPage, sort: sortBy, query: searchQuery || undefined, category: selectedCategory || undefined, tag: selectedTag });
    modules = r.modules; total = r.total; loading = false;
  }

  async function openDetail(mod: MarketModule) {
    selectedModule = mod; reviewsLoading = true; healthScore = null; moduleTags = [];
    const [revs, hs, tags] = await Promise.all([api.fetchReviews(mod.slug), api.fetchHealthScore(mod.slug), api.fetchModuleTags(mod.slug)]);
    reviews = revs; healthScore = hs; moduleTags = tags; reviewsLoading = false;
  }

  async function toggleFav(mod: MarketModule) {
    const isFav = favoritedModules.has(mod.id);
    const r = await api.toggleFavoriteApi(mod.id, isFav);
    if (r !== null) { if (isFav) favoritedModules.delete(mod.id); else favoritedModules.add(mod.id); favoritedModules = new Set(favoritedModules); }
  }

  async function doStar() {
    if (!selectedModule) return;
    const stars = await api.starModuleApi(selectedModule.slug);
    if (stars !== null) {
      selectedModule = { ...selectedModule, stars };
      const idx = modules.findIndex(m => m.id === selectedModule!.id);
      if (idx >= 0) modules[idx] = { ...modules[idx], stars };
    }
  }

  async function doSubmitReview() {
    if (!selectedModule || !newReviewComment.trim()) return;
    submittingReview = true;
    const ok = await api.submitReviewApi(selectedModule.slug, newReviewRating, newReviewComment);
    if (ok) { newReviewComment = ''; newReviewRating = 5; reviews = await api.fetchReviews(selectedModule.slug); }
    submittingReview = false;
  }

  async function loadVersions(slug: string) {
    versionsLoading = true; showVersions = true;
    versions = await api.fetchVersions(slug); versionsLoading = false;
  }

  async function doRollback(versionId: string) {
    if (!selectedModule) return;
    rollingBack = versionId;
    const ok = await api.rollbackApi(selectedModule.slug, versionId);
    if (ok) { await loadVersions(selectedModule.slug); await loadModules(); }
    rollingBack = null;
  }

  async function doUpdateVersion() {
    if (!selectedModule || !newVersionCode.trim()) return;
    updatingVersion = true;
    const ok = await api.updateVersionApi(selectedModule.slug, newVersionCode, newChangelog);
    if (ok) { showVersionUpdate = false; newVersionCode = ''; newChangelog = ''; await loadVersions(selectedModule.slug); await loadModules(); }
    updatingVersion = false;
  }

  async function loadChangelogs(slug: string) { changelogsLoading = true; changelogs = await api.fetchChangelogs(slug); changelogsLoading = false; }

  async function loadStats(slug: string) {
    statsLoading = true;
    const [stats, trending] = await Promise.all([api.fetchInstallStats(slug, statsPeriod), api.fetchTrending()]);
    installStats = stats; trendingModules = trending; statsLoading = false;
  }

  function toggleSelect(slug: string) { const n = new Set(selectedSlugs); n.has(slug) ? n.delete(slug) : n.add(slug); selectedSlugs = n; }

  async function runBatch(action: string) {
    if (selectedSlugs.size === 0) return;
    batchProcessing = true;
    const r = await api.runBatchApi(action, Array.from(selectedSlugs));
    if (r) { batchResults = r; showBatchResults = true; selectedSlugs = new Set(); }
    batchProcessing = false;
  }

  async function openInstallModal() {
    if (!selectedModule) return;
    showInstallModal = true; installError = ''; installDevice = ''; loadingDevices = true;
    installSteps = [{ label: '下载模块', status: 'pending' }, { label: '推送到设备', status: 'pending' }, { label: '在设备上安装', status: 'pending' }];
    installableDevices = await api.fetchDevices();
    if (installableDevices.length === 1) installDevice = installableDevices[0].serial;
    loadingDevices = false;
  }

  async function startInstall() {
    if (!selectedModule || !installDevice) return;
    installing = true; installError = '';
    const slug = selectedModule.slug;
    try {
      installSteps = installSteps.map((s, i) => i === 0 ? { ...s, status: 'running' } : s);
      const dl = await api.downloadModule(slug);
      if (!dl) throw new Error('下载失败');
      installSteps = installSteps.map((s, i) => i === 0 ? { ...s, status: 'done', detail: `${dl.filename} (${(dl.blob.size / 1024).toFixed(1)} KB)` } : s);
      installSteps = installSteps.map((s, i) => i === 1 ? { ...s, status: 'running' } : s);
      const output = await api.pushToDevice(installDevice, dl.blob, dl.filename);
      installSteps = installSteps.map((s, i) => i === 1 ? { ...s, status: 'done', detail: output } : s);
      installSteps = installSteps.map((s, i) => i === 2 ? { ...s, status: 'done', detail: '模块已安装到设备' } : s);
      selectedModule!.installs++;
    } catch (err: any) {
      const idx = installSteps.findIndex(s => s.status === 'running');
      if (idx >= 0) installSteps = installSteps.map((s, i) => i === idx ? { ...s, status: 'error', detail: err.message } : s);
      installError = err.message;
    }
    installing = false;
  }

  async function loadTemplates() {
    templateLoading = true;
    const r = await api.fetchTemplates({ page: templatePage, sort: templateSort, query: templateSearch || undefined, category: templateCategory || undefined });
    templateList = r.templates; templateTotal = r.total; templateLoading = false;
  }

  async function doUseTemplate(t: any) {
    if (!confirm(`使用模板 "${t.name}" 创建新项目？`)) return;
    const data = await api.useTemplateApi(t.id);
    if (data) window.location.href = `/projects/new?template=${encodeURIComponent(data)}`;
  }

  async function doRateTemplate(t: any, rating: number) {
    const r = await api.rateTemplateApi(t.id, rating);
    if (r !== null) { t.rating = r; templateList = [...templateList]; }
  }

  async function doPublishTemplate() {
    if (!publishName.trim()) return;
    publishing = true;
    const ok = await api.publishTemplateApi(publishName, publishDesc, publishCategory);
    if (ok) { showPublishTemplate = false; publishName = ''; publishDesc = ''; publishCategory = ''; await loadTemplates(); }
    publishing = false;
  }

  async function openDemo(slug: string) {
    showDemo = true; demoLoading = true; demoData = null; demoVisibleLines = 0;
    if (demoInterval) clearInterval(demoInterval);
    demoData = await api.fetchDemo(slug); demoLoading = false;
    if (demoData && !demoData.error) {
      const lines = (demoData.simulated_output || '').split('\n');
      let i = 0;
      demoInterval = setInterval(() => { i++; if (i <= lines.length) demoVisibleLines = i; else { clearInterval(demoInterval!); demoInterval = null; } }, 150);
    }
  }

  function closeDemo() { if (demoInterval) clearInterval(demoInterval); demoInterval = null; showDemo = false; demoData = null; demoVisibleLines = 0; }

  function toggleCompare(slug: string) {
    const n = new Set(compareIds); if (n.has(slug)) n.delete(slug); else if (n.size < 2) n.add(slug); compareIds = n;
  }

  async function doCompare() {
    const ids = Array.from(compareIds); if (ids.length !== 2) return;
    compareLoading = true; compareResult = await api.runCompareApi(ids[0], ids[1]); compareLoading = false;
  }

  function compareWinner(a: number, b: number): 'a' | 'b' | 'tie' { return a > b ? 'a' : b > a ? 'b' : 'tie'; }

  onMount(() => { loadModules(); api.fetchFavoriteIds().then(ids => favoritedModules = ids); api.fetchAllTags().then(t => allTags = t); });

  onDestroy(() => {
    if (demoInterval) clearInterval(demoInterval);
  });
</script>

<div class="w-full p-4 md:p-6 max-w-7xl mx-auto">
  <div class="market-header flex items-center justify-between mb-8">
    <div>
      <h1 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">ModuForge 市场</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">发现和分享优质 Magisk/KSU 模块</p>
    </div>
    <a href="/market/publish" class="btn-primary flex items-center gap-2 no-underline"><span class="material-symbols-outlined text-[18px]">publish</span>发布模块</a>
  </div>

  <MarketHeader bind:searchQuery bind:selectedCategory bind:sortBy {total} {categories} {allTags} bind:selectedTag bind:compareIds bind:selectedSlugs {batchProcessing} {compareLoading}
    onSearch={() => { page = 1; loadModules(); }} onCategoryChange={(c) => { selectedCategory = c; page = 1; loadModules(); }} onSortChange={(s) => { sortBy = s; page = 1; loadModules(); }} onTagChange={(t) => { selectedTag = t; page = 1; loadModules(); }}
    onCompare={doCompare} onRunBatch={runBatch} onClearSlugs={() => selectedSlugs = new Set()} onClearCompare={(s) => { const n = new Set(compareIds); n.delete(s); compareIds = n; }} />

  <PublishTemplateModal show={showPublishTemplate} onClose={() => showPublishTemplate = false} onPublish={doPublishTemplate} {publishing} bind:publishName bind:publishDesc bind:publishCategory />

  <ModuleGrid {modules} {loading} {favoritedModules} {selectedSlugs} {compareIds} {categoryStyles} onSelect={toggleSelect} onFavorite={toggleFav} onCompare={toggleCompare} onOpenDetail={openDetail} />
</div>

<ModuleDetailModal module={selectedModule} {healthScore} {moduleTags} {reviews} {reviewsLoading} bind:newReviewRating bind:newReviewComment {submittingReview} {changelogs} {changelogsLoading} {installStats} {statsPeriod} {statsLoading} {trendingModules}
  {templateList} {templateTotal} {templateLoading} bind:templateSearch bind:templateCategory bind:templateSort bind:templatePage {templateCategories}
  onClose={() => selectedModule = null} onStar={doStar} onSubmitReview={doSubmitReview} onLoadVersions={loadVersions} onLoadChangelogs={loadChangelogs}
  onLoadInstallStats={loadStats} onStatsPeriodChange={(p) => { statsPeriod = p; if (selectedModule) loadStats(selectedModule.slug); }}
  onLoadTemplates={loadTemplates} onLoadTemplateCategories={() => api.fetchTemplateCategories().then(c => templateCategories = c)}
  onShowPublishTemplate={() => showPublishTemplate = true} onUseTemplate={doUseTemplate} onRateTemplate={doRateTemplate}
  onOpenDemo={openDemo} onOpenInstallModal={openInstallModal} onFullscreenScreenshot={(u) => fullscreenScreenshot = u} />

<BatchResultsModal show={showBatchResults} results={batchResults} onClose={() => showBatchResults = false} />

{#if fullscreenScreenshot}
  <div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.85)" role="presentation" onclick={() => fullscreenScreenshot = null}>
    <img src={fullscreenScreenshot} alt="截图" class="max-w-full max-h-full object-contain" role="presentation" onclick={(e) => e.stopPropagation()} />
  </div>
{/if}

<VersionHistoryModal show={showVersions} {versions} {versionsLoading} selectedModuleSlug={selectedModule?.slug || ''} {rollingBack} bind:showVersionUpdate bind:newVersionCode bind:newChangelog {updatingVersion}
  onClose={() => showVersions = false} onRollback={(v) => doRollback(v)} onPublishVersion={doUpdateVersion} onShowUpdate={() => showVersionUpdate = true} onHideUpdate={() => showVersionUpdate = false} />

<CompareModal show={compareResult !== null} result={compareResult} onClose={() => compareResult = null} {compareWinner} />

<InstallModal show={showInstallModal} moduleName={selectedModule?.title || selectedModule?.slug || ''} moduleVersion={selectedModule?.version || ''} {installing} {installSteps} {installError} {loadingDevices} {installableDevices} bind:selectedDevice={installDevice} onClose={() => showInstallModal = false} onStartInstall={startInstall} />

<DemoModal show={showDemo} loading={demoLoading} data={demoData} visibleLines={demoVisibleLines} onClose={closeDemo} onInstall={() => { closeDemo(); window.location.href = '/devices'; }} />

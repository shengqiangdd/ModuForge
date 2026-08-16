<script lang="ts">
  import { onMount } from 'svelte';
  import MarketHeader from './components/MarketHeader.svelte';
  import ModuleGrid from './components/ModuleGrid.svelte';
  import InstallModal from './components/InstallModal.svelte';
  import PublishTemplateModal from './components/PublishTemplateModal.svelte';
  import BatchResultsModal from './components/BatchResultsModal.svelte';
  import VersionHistoryModal from './components/VersionHistoryModal.svelte';
  import CompareModal from './components/CompareModal.svelte';
  import DemoModal from './components/DemoModal.svelte';

  interface MarketModule {
    id: string; title: string; slug: string; description: string; category: string;
    tags: string; version: string; version_code: number; author: string;
    license: string; stars: number; installs: number; updated_at: string; created_at: string;
    screenshots?: { url: string }[];
    cover_image?: string;
    dependencies?: { id: string; min_version?: string; optional?: boolean }[];
  }
  interface Review {
    id: string; module_id: string; uid: string; username: string;
    rating: number; comment: string; created_at: string;
  }

  let modules = $state<MarketModule[]>([]);
  let total = $state(0);
  let loading = $state(true);
  let searchQuery = $state('');
  let selectedCategory = $state('');
  let sortBy = $state('stars');
  let page = $state(1);
  const perPage = 20;

  interface ModuleVersion {
    id: string; module_id: string; version: string; version_code: string;
    changelog: string; created_at: string;
  }

  let selectedModule = $state<MarketModule | null>(null);
  let reviews = $state<Review[]>([]);
  let reviewsLoading = $state(false);
  let newReviewRating = $state(5);
  let newReviewComment = $state('');
  let submittingReview = $state(false);

  let showVersions = $state(false);
  let versions = $state<ModuleVersion[]>([]);
  let versionsLoading = $state(false);
  let rollingBack = $state<string | null>(null);
  let showVersionUpdate = $state(false);
  let newVersionCode = $state('');
  let newChangelog = $state('');
  let updatingVersion = $state(false);

  // Favorites
  let favoritedModules = $state<Set<string>>(new Set());
  let favoriteSlugs = $state<Set<string>>(new Set());

  // Health Score
  let healthScore = $state<{ score: number; level: string; details: { name: string; label: string; score: number; max: number }[] } | null>(null);
  let healthColor = $derived(healthScore ? (healthScore.score >= 80 ? '#22c55e' : healthScore.score >= 60 ? '#eab308' : '#ef4444') : 'var(--color-success)');
  let healthLoading = $state(false);

  // Tags
  let allTags: { id: number; name: string; color: string; usage_count: number }[] = $state([]);
  let selectedTag = $state<number | null>(null);
  let moduleTags: { id: number; name: string; color: string }[] = $state([]);
  let moduleTagsLoading = $state(false);

  // Changelogs
  let changelogs: { id: number; version: string; content: string; created_at: string }[] = $state([]);
  let changelogsLoading = $state(false);
  let detailTab = $state<'detail' | 'changelogs' | 'stats' | 'templates'>('detail');

  // Batch operations
  let selectedSlugs = $state<Set<string>>(new Set());
  let batchProcessing = $state(false);
  let batchResults = $state<{slug: string; status: string; error?: string}[]>([]);
  let showBatchResults = $state(false);

  // Install stats
  let installStats: {period: string; count: number}[] = $state([]);
  let statsPeriod = $state<'day' | 'week' | 'month'>('day');
  let trendingModules: MarketModule[] = $state([]);
  let statsLoading = $state(false);

  // Install to device modal
  let showInstallModal = $state(false);
  let installSteps = $state<Array<{label: string; status: 'pending' | 'running' | 'done' | 'error'; detail?: string}>>([]);
  let installDevice = $state('');
  let installableDevices = $state<Array<{serial: string; model: string; state: string}>>([]);
  let loadingDevices = $state(false);
  let installing = $state(false);
  let installError = $state('');

  // Feature 2: Template Marketplace
  let templateList = $state<Array<{id: number, name: string, description: string, category: string, author: string, downloads: number, rating: number, module_data: string, created_at: string}>>([]);
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
  let templateCategories = $state<Array<{name: string, count: number}>>([]);

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }
  function getUserId() { return localStorage.getItem('moduforge_uid') || ''; }

  async function toggleFavorite(mod: MarketModule) {
    const token = getToken();
    if (!token) return;
    const isFav = favoritedModules.has(mod.id);
    try {
      let r;
      if (isFav) {
        r = await fetch(`/api/v1/favorites/module/${mod.id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      } else {
        r = await fetch('/api/v1/favorites', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }, body: JSON.stringify({ type: 'module', id: parseInt(mod.id.replace('mod_', '')) || 0 }) });
      }
      if (r.status === 401) { localStorage.removeItem('moduforge_token'); return; }
      if (isFav) favoritedModules.delete(mod.id);
      else favoritedModules.add(mod.id);
      favoritedModules = new Set(favoritedModules);
    } catch {}
  }

  async function loadFavorites() {
    const token = getToken();
    if (!token) return;
    try {
      const r = await fetch('/api/v1/favorites?type=module', { headers: { Authorization: `Bearer ${token}` } });
      if (r.status === 401) { localStorage.removeItem('moduforge_token'); return; }
      if (r.ok) {
        const d = await r.json();
        for (const f of (d.favorites || [])) {
          const slug = `mod_${String(f.item_id).padStart(4, '0')}`;
          favoritedModules.add(slug);
        }
        favoritedModules = new Set(favoritedModules);
      }
    } catch {}
  }

  async function loadHealthScore(slug: string) {
    healthLoading = true;
    try {
      const r = await fetch(`/api/v1/market/module/${slug}/health`);
      if (r.ok) {
        healthScore = await r.json();
      }
    } catch { healthScore = null; }
    healthLoading = false;
  }

  async function loadModuleTags(slug: string) {
    moduleTagsLoading = true;
    try {
      const r = await fetch(`/api/v1/market/module/${slug}/tags`);
      if (r.ok) { const d = await r.json(); moduleTags = d.tags || []; }
    } catch { moduleTags = []; }
    moduleTagsLoading = false;
  }

  // Screenshot gallery
  let galleryIndex = $state(0);
  let fullscreenScreenshot = $state<string | null>(null);
  let compareScreenshot = $state<{ before: number; after: number } | null>(null);

  async function loadChangelogs(slug: string) {
    changelogsLoading = true;
    try {
      const r = await fetch(`/api/v1/market/module/${slug}/changelogs`);
      if (r.ok) { const d = await r.json(); changelogs = d.changelogs || []; }
    } catch { changelogs = []; }
    changelogsLoading = false;
  }

  function toggleSelect(slug: string) {
    const next = new Set(selectedSlugs);
    if (next.has(slug)) next.delete(slug); else next.add(slug);
    selectedSlugs = next;
  }

  async function runBatch(action: 'install' | 'uninstall' | 'update') {
    if (selectedSlugs.size === 0) return;
    batchProcessing = true;
    batchResults = [];
    const token = getToken();
    if (!token) { batchProcessing = false; return; }
    try {
      const r = await fetch(`/api/v1/market/batch/${action}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ slugs: Array.from(selectedSlugs) })
      });
      if (r.status === 401) { localStorage.removeItem('moduforge_token'); }
      else if (r.ok) {
        const d = await r.json();
        batchResults = (d.results || []).map((res: any) => ({ ...res, status: res.error ? 'failed' : 'ok' }));
        showBatchResults = true;
        selectedSlugs = new Set();
      }
    } catch {}
    batchProcessing = false;
  }

  async function loadInstallStats(slug: string) {
    statsLoading = true;
    try {
      const r = await fetch(`/api/v1/market/module/${slug}/install-stats?period=${statsPeriod}&days=30`);
      if (r.ok) { const d = await r.json(); installStats = d.stats || []; }
    } catch { installStats = []; }
    statsLoading = false;
  }

  async function loadTrending() {
    try {
      const r = await fetch('/api/v1/market/stats/trending');
      if (r.ok) { const d = await r.json(); trendingModules = d.modules?.slice(0, 10) || []; }
    } catch {}
  }

  // ─── Install to Device ───
  async function openInstallModal() {
    if (!selectedModule) return;
    showInstallModal = true;
    installError = '';
    installSteps = [
      { label: '下载模块', status: 'pending' },
      { label: '推送到设备', status: 'pending' },
      { label: '在设备上安装', status: 'pending' },
    ];
    installDevice = '';
    loadingDevices = true;
    try {
      const r = await fetch('/api/v1/adb/devices');
      if (r.ok) {
        const d = await r.json();
        installableDevices = (d.devices || []).filter((dev: any) => dev.state === 'device');
        if (installableDevices.length === 1) installDevice = installableDevices[0].serial;
      }
    } catch { installableDevices = []; }
    loadingDevices = false;
  }

  async function startInstall() {
    if (!selectedModule || !installDevice) return;
    installing = true;
    installError = '';
    const slug = selectedModule.slug;

    try {
      // Step 1: Download
      installSteps = installSteps.map((s, i) => i === 0 ? { ...s, status: 'running' } : s);
      const token = getToken();
      const downloadRes = await fetch(`/api/v1/market/module/${slug}/download`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {}
      });
      if (!downloadRes.ok) throw new Error(`下载失败 (${downloadRes.status})`);
      const blob = await downloadRes.blob();
      const filename = `${slug}.zip`;
      installSteps = installSteps.map((s, i) => i === 0 ? { ...s, status: 'done', detail: `${filename} (${(blob.size / 1024).toFixed(1)} KB)` } : s);

      // Step 2: Push to device
      installSteps = installSteps.map((s, i) => i === 1 ? { ...s, status: 'running' } : s);
      const formData = new FormData();
      formData.append('serial', installDevice);
      formData.append('file', blob, filename);
      const pushRes = await fetch('/api/v1/adb/module/upload', {
        method: 'POST',
        body: formData,
      });
      if (!pushRes.ok) {
        const errData = await pushRes.json().catch(() => ({}));
        throw new Error(errData.error || `推送失败 (${pushRes.status})`);
      }
      const pushData = await pushRes.json();
      installSteps = installSteps.map((s, i) => i === 1 ? { ...s, status: 'done', detail: pushData.output || '推送完成' } : s);

      // Step 3: Show success
      installSteps = installSteps.map((s, i) => i === 2 ? { ...s, status: 'done', detail: '模块已安装到设备' } : s);
      selectedModule!.installs++;
    } catch (err: any) {
      const errIdx = installSteps.findIndex(s => s.status === 'running');
      if (errIdx >= 0) {
        installSteps = installSteps.map((s, i) => i === errIdx ? { ...s, status: 'error', detail: err.message } : s);
      }
      installError = err.message;
    }
    installing = false;
  }

  // Template Marketplace functions
  async function loadTemplates() {
    templateLoading = true;
    try {
      const params = new URLSearchParams({
        page: String(templatePage),
        per_page: '20',
        sort: templateSort,
      });
      if (templateSearch) params.set('query', templateSearch);
      if (templateCategory) params.set('category', templateCategory);

      const r = await fetch(`/api/v1/templates/market?${params}`);
      if (r.ok) {
        const d = await r.json();
        templateList = d.templates || [];
        templateTotal = d.total || 0;
      }
    } catch {}
    templateLoading = false;
  }

  async function loadTemplateCategories() {
    try {
      const r = await fetch('/api/v1/templates/market/categories');
      if (r.ok) {
        const d = await r.json();
        templateCategories = d.categories || [];
      }
    } catch {}
  }

  async function useTemplate(t: any) {
    if (!confirm(`使用模板 "${t.name}" 创建新项目？`)) return;
    try {
      const r = await fetch(`/api/v1/templates/market/${t.id}/use`, { method: 'POST' });
      if (r.ok) {
        const d = await r.json();
        // Redirect to new project page with template data
        window.location.href = `/projects/new?template=${encodeURIComponent(d.module_data)}`;
      }
    } catch {}
  }

  async function rateTemplate(t: any, rating: number) {
    const token = getToken();
    if (!token) return;
    try {
      const r = await fetch(`/api/v1/templates/market/${t.id}/rate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ rating }),
      });
      if (r.ok) {
        const d = await r.json();
        t.rating = d.rating;
        templateList = [...templateList];
      }
    } catch {}
  }

  async function publishTemplate() {
    if (!publishName.trim()) return;
    publishing = true;
    const token = getToken();
    if (!token) { publishing = false; return; }
    try {
      const r = await fetch('/api/v1/templates/market', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          name: publishName,
          description: publishDesc,
          category: publishCategory,
          module_data: JSON.stringify({ name: publishName, description: publishDesc }),
        }),
      });
      if (r.ok) {
        showPublishTemplate = false;
        publishName = '';
        publishDesc = '';
        publishCategory = '';
        await loadTemplates();
      }
    } catch {}
    publishing = false;
  }

  import { renderMarkdown } from '$lib/utils/markdown';

  async function loadAllTags() {
    try {
      const token = getToken();
      const headers: Record<string, string> = {};
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const r = await fetch('/api/v1/tags', { headers });
      if (r.status === 401) { allTags = []; return; }
      if (r.ok) { const d = await r.json(); allTags = d.tags || []; }
    } catch { allTags = []; }
  }

  const categories = [
    { value: '', label: '全部', icon: 'apps' },
    { value: 'system', label: '系统', icon: 'phone_android' },
    { value: 'ui', label: '界面', icon: 'palette' },
    { value: 'audio', label: '音频', icon: 'headphones' },
    { value: 'display', label: '显示', icon: 'brightness_6' },
    { value: 'utility', label: '工具', icon: 'build' },
  ];

  const categoryColors: Record<string, string> = {
    system: 'system', ui: 'ui', audio: 'audio', display: 'display', utility: 'utility',
  };

  const categoryStyles: Record<string, string> = {
    system: 'background: rgba(59,130,246,0.15); color: #60a5fa',
    ui: 'background: rgba(168,85,247,0.15); color: #c084fc',
    audio: 'background: rgba(34,197,94,0.15); color: #4ade80',
    display: 'background: rgba(249,115,22,0.15); color: #fb923c',
    utility: 'background: rgba(161,161,170,0.15); color: #a1a1aa',
  };

  async function loadModules() {
    loading = true;
    try {
      const params = new URLSearchParams({ page: String(page), per_page: String(perPage), sort: sortBy });
      if (searchQuery) params.set('query', searchQuery);
      if (selectedCategory) params.set('category', selectedCategory);
      if (selectedTag) params.set('tag', String(selectedTag));
      const token = getToken();
      const headers: Record<string, string> = {};
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const res = await fetch(`/api/v1/market/modules?${params}`, { headers });
      if (res.ok) { const d = await res.json(); modules = d.modules || []; total = d.total || 0; }
    } catch { modules = []; }
    loading = false;
  }

  async function openDetail(mod: MarketModule) {
    selectedModule = mod;
    reviewsLoading = true;
    healthScore = null;
    moduleTags = [];
    try {
      const res = await fetch(`/api/v1/market/module/${mod.slug}/reviews`);
      if (res.ok) { const d = await res.json(); reviews = d.reviews || []; }
    } catch { reviews = []; }
    reviewsLoading = false;
    loadHealthScore(mod.slug);
    loadModuleTags(mod.slug);
  }

  async function loadVersions(slug: string) {
    versionsLoading = true;
    showVersions = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/market/module/${slug}/versions`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.status === 401) { localStorage.removeItem('moduforge_token'); versions = []; }
      else if (res.ok) { const d = await res.json(); versions = d.versions || []; }
    } catch { versions = []; }
    versionsLoading = false;
  }

  async function rollback(slug: string, versionId: string) {
    rollingBack = versionId;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/market/module/${slug}/rollback`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ version_id: versionId }),
      });
      if (res.status === 401) { localStorage.removeItem('moduforge_token'); }
      else if (res.ok) {
        loadVersions(slug);
        loadModules();
      }
    } catch {}
    rollingBack = null;
  }

  async function updateVersion(slug: string) {
    if (!newVersionCode.trim()) return;
    updatingVersion = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/market/module/${slug}/version`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ version_code: newVersionCode.trim(), changelog: newChangelog.trim() }),
      });
      if (res.status === 401) { localStorage.removeItem('moduforge_token'); }
      else if (res.ok) {
        showVersionUpdate = false;
        newVersionCode = '';
        newChangelog = '';
        loadVersions(slug);
        loadModules();
      }
    } catch {}
    updatingVersion = false;
  }

  async function starModule() {
    if (!selectedModule) return;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/market/module/${selectedModule.slug}/star`, { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } });
      if (res.status === 401) { localStorage.removeItem('moduforge_token'); }
      else if (res.ok) {
        const d = await res.json();
        selectedModule = { ...selectedModule, stars: d.stars };
        const idx = modules.findIndex(m => m.id === selectedModule!.id);
        if (idx >= 0) modules[idx] = { ...modules[idx], stars: d.stars };
      }
    } catch {}
  }

  async function submitReview() {
    if (!selectedModule || !newReviewComment.trim()) return;
    submittingReview = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/market/module/${selectedModule.slug}/review`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ uid: 'anonymous', username: 'Anonymous', rating: newReviewRating, comment: newReviewComment }),
      });
      if (res.status === 401) { localStorage.removeItem('moduforge_token'); }
      else if (res.ok) {
        newReviewComment = ''; newReviewRating = 5;
        const r = await fetch(`/api/v1/market/module/${selectedModule.slug}/reviews`);
        if (r.ok) { const d = await r.json(); reviews = d.reviews || []; }
      }
    } catch {}
    submittingReview = false;
  }

  // ===== Module Demo =====
  let showDemo = $state(false);
  let demoLoading = $state(false);
  let demoData = $state<any>(null);
  let demoVisibleLines = $state(0);
  let demoInterval = $state<ReturnType<typeof setInterval> | null>(null);

  async function openDemo(slug: string) {
    showDemo = true;
    demoLoading = true;
    demoData = null;
    demoVisibleLines = 0;
    if (demoInterval) clearInterval(demoInterval);
    try {
      const res = await fetch(`/api/v1/market/module/${slug}/demo`);
      if (res.ok) demoData = await res.json();
      else demoData = { error: '无法加载演示数据' };
    } catch { demoData = { error: '网络错误' }; }
    demoLoading = false;
    if (demoData && !demoData.error) {
      const lines = (demoData.simulated_output || '').split('\n');
      let i = 0;
      demoInterval = setInterval(() => {
        i++;
        if (i <= lines.length) { demoVisibleLines = i; }
        else { if (demoInterval) clearInterval(demoInterval); demoInterval = null; }
      }, 150);
    }
  }

  function closeDemo() {
    if (demoInterval) clearInterval(demoInterval);
    demoInterval = null;
    showDemo = false;
    demoData = null;
    demoVisibleLines = 0;
  }

  // ===== Comparison =====
  let compareIds = $state<Set<string>>(new Set());
  let compareLoading = $state(false);
  let compareResult = $state<any>(null);

  function toggleCompare(slug: string) {
    const next = new Set(compareIds);
    if (next.has(slug)) next.delete(slug);
    else if (next.size < 2) next.add(slug);
    compareIds = next;
  }

  async function runCompare() {
    const ids = Array.from(compareIds);
    if (ids.length !== 2) return;
    compareLoading = true;
    compareResult = null;
    try {
      const res = await fetch('/api/v1/market/compare', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ slug1: ids[0], slug2: ids[1] }),
      });
      if (res.ok) compareResult = await res.json();
      else compareResult = { error: '对比失败' };
    } catch { compareResult = { error: '网络错误' }; }
    compareLoading = false;
  }

  function compareWinner(a: number, b: number): 'a' | 'b' | 'tie' {
    if (a > b) return 'a';
    if (b > a) return 'b';
    return 'tie';
  }

  function fmt(n: number) { return n >= 1000 ? (n / 1000).toFixed(1) + 'k' : String(n); }
  function handleSearch(e: KeyboardEvent) { if (e.key === 'Enter') { page = 1; loadModules(); } }

  onMount(() => { loadModules(); loadFavorites(); loadAllTags(); });
</script>

<div class="w-full p-4 md:p-6 max-w-7xl mx-auto">
  <!-- Header -->
  <div class="market-header flex items-center justify-between mb-8">
    <div>
      <h1 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">ModuForge 市场</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">发现和分享优质 Magisk/KSU 模块</p>
    </div>
    <a href="/market/publish" class="btn-primary flex items-center gap-2 no-underline">
      <span class="material-symbols-outlined text-[18px]">publish</span>
      发布模块
    </a>
  </div>

  <!-- Search, Categories, Tags, Sort, Compare, Batch -->
  <MarketHeader
    bind:searchQuery
    bind:selectedCategory
    bind:sortBy
    {total}
    {categories}
    {allTags}
    bind:selectedTag
    bind:compareIds
    bind:selectedSlugs
    {batchProcessing}
    {compareLoading}
    onSearch={() => { page = 1; loadModules(); }}
    onCategoryChange={(cat) => { selectedCategory = cat; page = 1; loadModules(); }}
    onSortChange={(s) => { sortBy = s; page = 1; loadModules(); }}
    onTagChange={(t) => { selectedTag = t; page = 1; loadModules(); }}
    onCompare={runCompare}
    onRunBatch={runBatch}
    onClearSlugs={() => selectedSlugs = new Set()}
    onClearCompare={(slug) => { const next = new Set(compareIds); next.delete(slug); compareIds = next; }}
  />

<PublishTemplateModal
    show={showPublishTemplate}
    onClose={() => showPublishTemplate = false}
    onPublish={publishTemplate}
    {publishing}
    bind:publishName
    bind:publishDesc
    bind:publishCategory
  />

  <!-- Module Grid -->
  <ModuleGrid
    {modules}
    {loading}
    {favoritedModules}
    {selectedSlugs}
    {compareIds}
    {categoryStyles}
    onSelect={toggleSelect}
    onFavorite={toggleFavorite}
    onCompare={toggleCompare}
    onOpenDetail={openDetail}
  />
</div>

<!-- Detail Modal -->
{#if selectedModule}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) selectedModule = null; }}>
    <div class="rounded-2xl max-w-2xl w-full max-h-[85vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-6 border-b" style="border-color: var(--color-border)">
        <div class="flex items-start justify-between">
          <div>
            <h2 class="text-xl font-bold text-[var(--color-text)]">{selectedModule.title}</h2>
            <div class="flex flex-wrap items-center gap-2 mt-2 text-sm text-[var(--color-text-muted)]">
              <span>{selectedModule.version}</span>
              <span>·</span>
              <span class="badge" style={categoryStyles[selectedModule.category] || ''}>{selectedModule.category}</span>
              <span>·</span>
              <span>{selectedModule.author}</span>
              <span>·</span>
              <span>{selectedModule.license}</span>
            </div>
          </div>
          <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={() => selectedModule = null}>
            <span class="material-symbols-outlined text-[20px]">close</span>
          </button>
        </div>
        <!-- Screenshot Gallery -->
        {#if selectedModule.screenshots && selectedModule.screenshots.length > 0}
          <div class="mt-4">
            <div class="relative rounded-xl overflow-hidden">
                <button class="block w-full" onclick={() => fullscreenScreenshot = selectedModule!.screenshots![galleryIndex]?.url}>
                  <img src={selectedModule.screenshots[galleryIndex]?.url} alt="截图" class="w-full h-48 object-cover cursor-pointer" />
                </button>
                {#if selectedModule.screenshots.length > 1}
                  <button class="absolute left-2 top-1/2 -translate-y-1/2 w-8 h-8 rounded-full flex items-center justify-center bg-black/40 text-white" onclick={(e) => { e.stopPropagation(); galleryIndex = galleryIndex > 0 ? galleryIndex - 1 : selectedModule!.screenshots!.length - 1; }}><span class="material-symbols-outlined text-[18px]">chevron_left</span></button>
                  <button class="absolute right-2 top-1/2 -translate-y-1/2 w-8 h-8 rounded-full flex items-center justify-center bg-black/40 text-white" onclick={(e) => { e.stopPropagation(); galleryIndex = galleryIndex < selectedModule!.screenshots!.length - 1 ? galleryIndex + 1 : 0; }}><span class="material-symbols-outlined text-[18px]">chevron_right</span></button>
              {/if}
            </div>
            {#if selectedModule.screenshots.length > 1}
              <div class="flex gap-1.5 mt-2 overflow-x-auto pb-1">
                {#each selectedModule.screenshots as ss, i}
                  <div role="button" tabindex="0" class="w-12 h-8 rounded overflow-hidden flex-shrink-0 cursor-pointer border-2 transition-colors" style="border-color: {i === galleryIndex ? 'var(--color-primary)' : 'transparent'}" onclick={() => galleryIndex = i} onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); galleryIndex = i; } }}>
                    <img src={ss.url} alt="" class="w-full h-full object-cover" />
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {:else if selectedModule.cover_image}
          <div class="mt-4 rounded-xl overflow-hidden">
            <img src={selectedModule.cover_image} alt="封面" class="w-full h-48 object-cover" />
          </div>
        {/if}
        {#if selectedModule.dependencies && selectedModule.dependencies.length > 0}
          <div class="mt-4 p-3 rounded-xl" style="background: var(--color-surface)">
            <p class="text-xs font-medium text-[var(--color-text-secondary)] mb-2">依赖</p>
            <div class="flex flex-wrap gap-1.5">
              {#each selectedModule.dependencies as dep}
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
        <div class="flex flex-wrap items-center gap-3 mt-4">
          <button class="flex items-center gap-1.5 text-sm font-medium hover:text-primary-600 transition-colors" onclick={starModule}>
            <span class="material-symbols-outlined text-[18px]">star</span>
            {selectedModule.stars} Stars
          </button>
          <span class="flex items-center gap-1.5 text-sm text-[var(--color-text-secondary)]">
            <span class="material-symbols-outlined text-[18px]">download</span>
            {fmt(selectedModule.installs)} 安装
          </span>
          <button class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-sm font-medium whitespace-nowrap transition-colors" style="color: var(--color-text-secondary); background: var(--color-surface)" onclick={() => loadVersions(selectedModule!.slug)}>
            <span class="material-symbols-outlined text-[16px]">history</span>
            版本历史
          </button>
          <button class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-sm font-medium whitespace-nowrap transition-colors" style="color: var(--color-text-secondary); background: var(--color-surface)" onclick={() => openDemo(selectedModule!.slug)}>
            <span class="material-symbols-outlined text-[16px]">preview</span>
            试用预览
          </button>
          <button class="flex-1 sm:flex-none ml-auto flex items-center justify-center gap-1.5 px-4 py-1.5 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--gradient-brand)" onclick={openInstallModal}>
            <span class="material-symbols-outlined text-[16px]">download</span>
            安装
          </button>
        </div>
      </div>

      <!-- Tab Navigation -->
      <div class="flex border-b" style="border-color: var(--color-border)">
        <button class="flex-1 py-3 text-sm font-medium text-center transition-colors" style={detailTab === 'detail' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)' : 'color: var(--color-text-muted)'} onclick={() => detailTab = 'detail'}>详情</button>
        <button class="flex-1 py-3 text-sm font-medium text-center transition-colors" style={detailTab === 'changelogs' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)' : 'color: var(--color-text-muted)'} onclick={() => { detailTab = 'changelogs'; loadChangelogs(selectedModule!.slug); }}>更新日志</button>
        <button class="flex-1 py-3 text-sm font-medium text-center transition-colors" style={detailTab === 'stats' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)' : 'color: var(--color-text-muted)'} onclick={() => { detailTab = 'stats'; loadInstallStats(selectedModule!.slug); loadTrending(); }}>统计</button>
        <button class="flex-1 py-3 text-sm font-medium text-center transition-colors" style={detailTab === 'templates' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)' : 'color: var(--color-text-muted)'} onclick={() => { detailTab = 'templates'; loadTemplates(); loadTemplateCategories(); }}>模板市场</button>
      </div>

      {#if detailTab === 'detail'}
        <!-- Health Score -->
        <div class="p-6 border-b" style="border-color: var(--color-border)">
          <h3 class="text-sm font-semibold text-[var(--color-text)] mb-3">健康度评分</h3>
          {#if healthScore}
            <div class="flex items-center gap-4">
              <div class="relative w-20 h-20 flex-shrink-0">
                <svg viewBox="0 0 36 36" class="w-full h-full -rotate-90">
                  <circle cx="18" cy="18" r="15.5" fill="none" stroke="var(--color-surface)" stroke-width="3"/>
                  <circle cx="18" cy="18" r="15.5" fill="none" stroke={healthColor} stroke-width="3" stroke-dasharray={`${(healthScore.score / 100) * 97.4} 97.4`} stroke-linecap="round"/>
                </svg>
                <div class="absolute inset-0 flex items-center justify-center">
                  <span class="text-xl font-bold" style="color: {healthColor}">{healthScore.score}</span>
                </div>
              </div>
              <div class="flex-1 space-y-1.5">
                {#each healthScore.details as detail}
                  <div class="flex items-center gap-2 text-xs">
                    <span style="color: var(--color-text)">{detail.label}</span>
                    <div class="flex-1 h-1.5 rounded-full" style="background: var(--color-surface)">
                      <div class="h-full rounded-full transition-all" style="width: {(detail.score / detail.max) * 100}%; background: {detail.score >= detail.max / 2 ? 'var(--color-success)' : 'var(--color-warning)'}"></div>
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

        <div class="p-6 border-b" style="border-color: var(--color-border)">
          <h3 class="text-sm font-semibold text-[var(--color-text)] mb-2">描述</h3>
          <p class="text-sm text-[var(--color-text-secondary)] leading-relaxed">{selectedModule.description}</p>
          {#if moduleTags.length > 0}
            <div class="flex flex-wrap gap-1.5 mt-3">
              {#each moduleTags as tag}
                <span class="px-2.5 py-1 rounded-lg text-xs" style="background: {tag.color}20; color: {tag.color}">{tag.name}</span>
              {/each}
            </div>
          {:else if selectedModule.tags}
            <div class="flex flex-wrap gap-1.5 mt-3">
              {#each selectedModule.tags.split(',') as tag}
                <span class="px-2.5 py-1 rounded-lg text-xs" style="background: var(--color-surface); color: var(--color-text-muted)">{tag.trim()}</span>
              {/each}
            </div>
          {/if}
        </div>

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
            {#each [1,2,3,4,5] as star}
              <button class="text-xl transition-colors {star <= newReviewRating ? 'text-amber-500' : 'text-neutral-300'}" onclick={() => newReviewRating = star}>★</button>
            {/each}
          </div>
          <textarea class="input-field resize-none" rows="3" placeholder="写下你的评价..." bind:value={newReviewComment}></textarea>
          <div class="flex justify-end mt-3">
            <button
              class="btn-primary text-sm disabled:opacity-50"
              disabled={submittingReview || !newReviewComment.trim()}
              onclick={submitReview}
            >
              {submittingReview ? '提交中...' : '提交评论'}
            </button>
          </div>
        </div>
      </div>
    {:else if detailTab === 'changelogs'}
      <div class="p-6">
        <h3 class="text-sm font-semibold text-[var(--color-text)] mb-4">更新日志</h3>
        {#if changelogsLoading}
          <p class="text-xs text-[var(--color-text-muted)]">加载中...</p>
        {:else if changelogs.length === 0}
          <p class="text-xs text-[var(--color-text-muted)]">暂无更新日志</p>
        {:else}
          <div class="space-y-4">
            {#each changelogs as log}
              <div class="relative pl-6 pb-4" style="border-left: 2px solid var(--color-border)">
                <div class="absolute left-[-5px] top-1 w-2 h-2 rounded-full" style="background: var(--color-primary)"></div>
                <div class="flex items-center gap-2 mb-1">
                  <span class="text-sm font-semibold text-[var(--color-text)]">{log.version}</span>
                  <span class="text-xs text-[var(--color-text-muted)]">{new Date(log.created_at).toLocaleDateString()}</span>
                </div>
                <div class="text-sm text-[var(--color-text-secondary)]" style="line-height: 1.6">{@html renderMarkdown(log.content)}</div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {:else if detailTab === 'stats'}
      <div class="p-6">
        <h3 class="text-sm font-semibold text-[var(--color-text)] mb-4">安装统计</h3>
        <div class="flex items-center gap-2 mb-4">
          {#each ['day', 'week', 'month'] as p}
            <button
              class="px-3 py-1 rounded-lg text-xs transition-colors"
              style={statsPeriod === p ? 'background: var(--color-primary-light); color: var(--color-primary); font-weight: 600' : 'background: var(--color-surface); color: var(--color-text-muted)'}
              onclick={() => { statsPeriod = p as 'day' | 'week' | 'month'; loadInstallStats(selectedModule!.slug); }}
            >{p === 'day' ? '按日' : p === 'week' ? '按周' : '按月'}</button>
          {/each}
        </div>
        {#if statsLoading}
          <p class="text-xs text-[var(--color-text-muted)]">加载中...</p>
        {:else if installStats.length === 0}
          <p class="text-xs text-[var(--color-text-muted)]">暂无安装数据</p>
        {:else}
          {@const maxC = Math.max(1, ...installStats.map(s => s.count))}
          <div class="flex items-end gap-1 h-32 mb-4">
            {#each installStats.slice(-20) as pt}
              <div class="flex-1 flex flex-col items-center gap-0.5 group relative">
                <div class="absolute bottom-full mb-1 hidden group-hover:block bg-[var(--color-bg-elevated)] rounded px-2 py-1 text-xs whitespace-nowrap z-10 border border-[var(--color-border)]">
                  {pt.period}: {pt.count}
                </div>
                <div class="w-full rounded-t-sm transition-all" style="height: {(pt.count / maxC) * 100}%; background: var(--gradient-brand)"></div>
                <span class="text-[8px] text-[var(--color-text-muted)] truncate w-full text-center">{pt.period.slice(-5)}</span>
              </div>
            {/each}
          </div>
        {/if}

        <h3 class="text-sm font-semibold text-[var(--color-text)] mb-4 mt-6">热门模块 Top 10</h3>
        {#if trendingModules.length === 0}
          <p class="text-xs text-[var(--color-text-muted)]">加载中...</p>
        {:else}
          <div class="space-y-2">
            {#each trendingModules as mod, i (mod.id)}
              <div class="flex items-center gap-3 py-1.5">
                <span class="w-5 h-5 rounded-full flex items-center justify-center text-xs font-bold" style="background: {i < 3 ? 'var(--gradient-brand)' : 'var(--color-surface)'}; color: {i < 3 ? 'white' : 'var(--color-text-muted)'}">{i + 1}</span>
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-[var(--color-text)] truncate">{mod.title}</p>
                  <p class="text-xs text-[var(--color-text-muted)]">{mod.installs} 安装 · {mod.stars} 星</p>
                </div>
                <span class="text-xs text-[var(--color-text-muted)]">{mod.category}</span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {:else if detailTab === 'templates'}
      <div class="p-6">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-sm font-semibold text-[var(--color-text)]">模板市场</h3>
          <button class="flex items-center gap-1 px-3 py-1.5 rounded-xl text-xs font-medium text-white transition-colors" style="background: var(--gradient-brand)" onclick={() => showPublishTemplate = true}>
            <span class="material-symbols-outlined text-[14px]">add</span>
            发布模板
          </button>
        </div>

        <!-- Search and Filter -->
        <div class="flex flex-col sm:flex-row gap-2 mb-4">
          <input type="text" placeholder="搜索模板..." class="flex-1 min-w-0 px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] text-sm" bind:value={templateSearch} oninput={() => { templatePage = 1; loadTemplates(); }} />
          <div class="flex gap-2">
            <select class="flex-1 sm:flex-none min-w-0 px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] text-sm" bind:value={templateCategory} onchange={() => { templatePage = 1; loadTemplates(); }}>
              <option value="">全部分类</option>
              {#each templateCategories as cat}
                <option value={cat.name}>{cat.name} ({cat.count})</option>
              {/each}
            </select>
            <select class="flex-1 sm:flex-none min-w-0 px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] text-sm" bind:value={templateSort} onchange={() => loadTemplates()}>
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
          <div class="grid grid-cols-2 gap-3">
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
                  <button class="btn-ghost text-xs flex-1" onclick={() => useTemplate(t)}>使用</button>
                  <button class="btn-ghost text-xs" onclick={() => rateTemplate(t, 5)}>⭐</button>
                </div>
              </div>
            {/each}
          </div>

          {#if templateTotal > 20}
            <div class="flex justify-center gap-2 mt-4">
              <button class="btn-ghost text-xs" onclick={() => { templatePage--; loadTemplates(); }} disabled={templatePage <= 1}>上一页</button>
              <span class="text-xs text-[var(--color-text-muted)] py-1">第 {templatePage} 页 / 共 {Math.ceil(templateTotal / 20)} 页</span>
              <button class="btn-ghost text-xs" onclick={() => { templatePage++; loadTemplates(); }} disabled={templatePage * 20 >= templateTotal}>下一页</button>
            </div>
          {/if}
        {/if}
      </div>
    {/if}
    </div>
  </div>
{/if}

<BatchResultsModal
    show={showBatchResults}
    results={batchResults}
    onClose={() => showBatchResults = false}
  />

<!-- Fullscreen Screenshot -->
{#if fullscreenScreenshot}
  <div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.85)" role="presentation" onclick={() => fullscreenScreenshot = null}>
    <img src={fullscreenScreenshot} alt="截图" class="max-w-full max-h-full object-contain" role="presentation" onclick={(e) => e.stopPropagation()} />
  </div>
{/if}

<VersionHistoryModal
    show={showVersions}
    {versions}
    {versionsLoading}
    selectedModuleSlug={selectedModule!.slug}
    {rollingBack}
    bind:showVersionUpdate
    bind:newVersionCode
    bind:newChangelog
    {updatingVersion}
    onClose={() => showVersions = false}
    onRollback={(versionId) => rollback(selectedModule!.slug, versionId)}
    onPublishVersion={() => updateVersion(selectedModule!.slug)}
    onShowUpdate={() => showVersionUpdate = true}
    onHideUpdate={() => showVersionUpdate = false}
  />

<CompareModal
    show={compareResult !== null}
    result={compareResult}
    onClose={() => compareResult = null}
    {fmt}
    {compareWinner}
  />

  <!-- Install to Device Modal -->
<InstallModal
  show={showInstallModal}
  moduleName={selectedModule?.title || selectedModule?.slug || ''}
  moduleVersion={selectedModule?.version || ''}
  {installing}
  {installSteps}
  {installError}
  {loadingDevices}
  {installableDevices}
  bind:selectedDevice={installDevice}
  onClose={() => showInstallModal = false}
  onStartInstall={startInstall}
/>

<DemoModal
    show={showDemo}
    loading={demoLoading}
    data={demoData}
    visibleLines={demoVisibleLines}
    onClose={closeDemo}
    onInstall={() => { closeDemo(); window.location.href = '/devices'; }}
  />

<script lang="ts">
  import { onMount } from 'svelte';

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

  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  // Favorites
  let favoritedModules = $state<Set<string>>(new Set());
  let favoriteSlugs = $state<Set<string>>(new Set());

  // Health Score
  let healthScore: { score: number; level: string; details: { name: string; label: string; score: number; max: number }[] } | null = $state(null);
  let healthColor = $state('var(--color-success)');
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
  let trendingModules: any[] = $state([]);
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
        const score = healthScore?.score ?? 0;
        if (score >= 80) healthColor = '#22c55e';
        else if (score >= 60) healthColor = '#eab308';
        else healthColor = '#ef4444';
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

  function renderMarkdown(text: string) {
    return text
      .replace(/### (.+)/g, '<strong class="text-sm block mt-2 mb-1">$1</strong>')
      .replace(/## (.+)/g, '<strong class="text-base block mt-3 mb-1">$1</strong>')
      .replace(/# (.+)/g, '<strong class="text-lg block mt-3 mb-1">$1</strong>')
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" target="_blank" class="text-primary-600 underline">$1</a>')
      .replace(/\n/g, '<br>');
  }

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

  function onSearchInput() {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      page = 1;
      loadModules();
    }, 300);
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

<style>
  .market-card {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .market-card:hover {
    border-color: var(--color-primary);
    box-shadow: 0 8px 32px rgba(139,92,246,0.15), 0 0 0 1px rgba(139,92,246,0.1);
    transform: translateY(-4px);
  }
  .cat-btn {
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .cat-btn:active {
    transform: scale(0.96);
  }
  .module-grid {
    animation: fadeIn 0.3s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(8px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .compare-table {
    width: 100%;
    border-collapse: collapse;
  }
  .compare-table th,
  .compare-table td {
    padding: 10px 12px;
    text-align: left;
    border-bottom: 1px solid var(--color-border);
  }
  .compare-table th {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--color-text-muted);
  }
  .compare-label {
    width: 80px;
    font-size: 12px;
    font-weight: 500;
    color: var(--color-text-secondary);
  }
  .compare-value {
    font-size: 13px;
    color: var(--color-text);
  }
  .compare-value.winner {
    color: #22c55e;
    font-weight: 600;
  }
  .compare-value.winner::after {
    content: ' ✓';
    font-size: 11px;
  }
  @media (max-width: 640px) {
    .market-header { flex-direction: column !important; align-items: flex-start !important; gap: 0.75rem !important; }
    .module-grid { grid-template-columns: 1fr !important; }
  }
</style>

<div class="p-4 md:p-6 max-w-7xl mx-auto">
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

  <!-- Search -->
  <div class="relative mb-5">
    <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-neutral-400 text-[20px] z-10">search</span>
    <div class="absolute left-[38px] top-2.5 bottom-2.5 w-px pointer-events-none z-10" style="background: var(--color-border)"></div>
    <input
      type="text"
      placeholder="搜索模块名称、描述、标签..."
      class="input-field market-search-input"
      style="padding-left: 48px;"
      bind:value={searchQuery}
      oninput={onSearchInput}
      onkeydown={(e) => { if (e.key === 'Enter') { if (debounceTimer) clearTimeout(debounceTimer); page = 1; loadModules(); } }}
    />
  </div>

  <!-- Categories -->
  <div class="flex gap-2 flex-wrap mb-4">
    {#each categories as cat}
      <button
        class="cat-btn flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium min-h-[44px]"
        style={selectedCategory === cat.value
          ? 'background: var(--gradient-brand); color: #fff; box-shadow: var(--shadow-glow)'
          : 'background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)'}
        onclick={() => { selectedCategory = cat.value; page = 1; loadModules(); }}
      >
        <span class="material-symbols-outlined text-[16px]">{cat.icon}</span>
        {cat.label}
      </button>
    {/each}
  </div>

  <!-- Tags Filter -->
  {#if allTags.length > 0}
    <div class="flex gap-2 flex-wrap mb-4">
      <button
        class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
        style={!selectedTag ? 'background: var(--color-primary-light); color: var(--color-primary)' : 'background: var(--color-surface); color: var(--color-text-muted); border: 1px solid var(--color-border)'}
        onclick={() => { selectedTag = null; page = 1; loadModules(); }}
      >全部</button>
      {#each allTags as tag}
        <button
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
          style={selectedTag === tag.id ? `background: ${tag.color}20; color: ${tag.color}` : 'background: var(--color-surface); color: var(--color-text-muted); border: 1px solid var(--color-border)'}
          onclick={() => { selectedTag = tag.id; page = 1; loadModules(); }}
        >{tag.name}</button>
      {/each}
    </div>
  {/if}

  <!-- Sort & Count -->
  <div class="flex items-center gap-3 mb-6 text-sm" style="color: var(--color-text-secondary)">
    <span>排序</span>
    {#each [{ id: 'stars', label: '热度' }, { id: 'installs', label: '安装量' }, { id: 'newest', label: '最新' }] as s}
      <button
        class="px-3 py-1 rounded-lg transition-colors min-h-[36px]"
        style={sortBy === s.id ? 'background: var(--color-primary-light); color: var(--color-primary); font-weight: 600' : ''}
        onclick={() => { sortBy = s.id; page = 1; loadModules(); }}
      >
        {s.label}
      </button>
    {/each}
    <span class="ml-auto" style="color: var(--color-text-muted)">{total} 个模块</span>
  </div>

  <!-- Compare bar -->
  {#if compareIds.size > 0}
    <div class="flex items-center gap-3 mb-4 px-4 py-3 rounded-xl" style="background: var(--color-primary-light);">
      <span class="text-sm" style="color: var(--color-primary)">已选 {compareIds.size}/2 个模块</span>
      <div class="flex gap-2 flex-1 flex-wrap">
        {#each Array.from(compareIds) as slug}
          <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-lg text-xs" style="background: var(--color-bg-elevated); color: var(--color-text)">
            {slug}
            <button class="p-0.5 hover:text-[var(--color-error)]" onclick={() => toggleCompare(slug)}>
              <span class="material-symbols-outlined text-[12px]">close</span>
            </button>
          </span>
        {/each}
      </div>
      {#if compareIds.size === 2}
        <button class="btn-primary text-sm px-3 py-1.5" onclick={runCompare} disabled={compareLoading}>
          {compareLoading ? '对比中...' : '对比'}
        </button>
      {/if}
    </div>
  {/if}

  <!-- Batch Operations Bar -->
  {#if selectedSlugs.size > 0}
    <div class="flex items-center gap-3 mb-4 px-4 py-3 rounded-xl" style="background: var(--color-primary-light);">
      <span class="text-sm font-medium" style="color: var(--color-primary)">已选 {selectedSlugs.size} 个模块</span>
      <div class="flex gap-2 ml-auto">
        <button class="btn-ghost text-xs px-3 py-1.5" disabled={batchProcessing} onclick={() => runBatch('install')}>
          {batchProcessing ? '处理中...' : '安装'}
        </button>
        <button class="btn-ghost text-xs px-3 py-1.5" disabled={batchProcessing} onclick={() => runBatch('uninstall')}>
          卸载
        </button>
        <button class="btn-ghost text-xs px-3 py-1.5" disabled={batchProcessing} onclick={() => runBatch('update')}>
          更新
        </button>
        <button class="flex items-center gap-1 text-xs px-2 py-1.5 rounded-lg hover:bg-red-50 transition-colors" style="color: var(--color-error)" onclick={() => selectedSlugs = new Set()}>
          <span class="material-symbols-outlined text-[14px]">close</span>
          清除
        </button>
    </div>
  </div>
{/if}

{#if showPublishTemplate}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => showPublishTemplate = false}>
    <div class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">发布模板</h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={() => showPublishTemplate = false}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-3">
        <div>
          <label class="block text-sm font-medium mb-1">模板名称</label>
          <input type="text" placeholder="e.g. System Prop Tweaks" class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={publishName} />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1">描述</label>
          <textarea placeholder="描述此模板的功能..." class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] h-20 resize-none" bind:value={publishDesc}></textarea>
        </div>
        <div>
          <label class="block text-sm font-medium mb-1">分类</label>
          <select class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={publishCategory}>
            <option value="">选择分类</option>
            <option value="system">系统</option>
            <option value="ui">界面</option>
            <option value="audio">音频</option>
            <option value="display">显示</option>
            <option value="utility">工具</option>
          </select>
        </div>
      </div>
      <div class="p-5 border-t flex justify-end gap-2" style="border-color: var(--color-border)">
        <button class="px-4 py-2 rounded-xl text-sm font-medium transition-colors" style="border: 1px solid var(--color-border); color: var(--color-text-secondary)" onclick={() => showPublishTemplate = false}>取消</button>
        <button class="px-4 py-2 rounded-xl text-sm font-semibold text-white transition-all" style="background: var(--gradient-brand)" onclick={publishTemplate} disabled={publishing || !publishName.trim()}>
          {publishing ? '发布中...' : '发布'}
        </button>
      </div>
    </div>
  </div>
{/if}

  <!-- Grid -->
  {#if loading}
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      {#each Array(8) as _}
        <div class="rounded-2xl border border-[var(--color-border)] p-5">
          <div class="skeleton h-4 w-24 mb-3"></div>
          <div class="skeleton h-5 w-full mb-2"></div>
          <div class="skeleton h-3 w-3/4 mb-4"></div>
          <div class="skeleton h-3 w-full mb-1"></div>
          <div class="skeleton h-3 w-2/3"></div>
        </div>
      {/each}
    </div>
  {:else if modules.length === 0}
    <div class="text-center py-16">
      <span class="material-symbols-outlined text-5xl text-neutral-300 mb-3 block">inventory_2</span>
      <p class="text-[var(--color-text-secondary)]">没有找到匹配的模块</p>
    </div>
  {:else}
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 module-grid">
      {#each modules as mod, i}
        <button
          class="market-card text-left p-5 group cursor-pointer relative overflow-hidden"
          style="animation-delay: {i * 50}ms"
          onclick={() => openDetail(mod)}
        >
          <!-- Favorite button -->
          <div class="absolute top-5 left-3 z-10" onclick={(e) => { e.stopPropagation(); toggleFavorite(mod); }}>
            <span class="material-symbols-outlined text-lg p-1 rounded-full cursor-pointer transition-colors" style="color: {favoritedModules.has(mod.id) ? '#ef4444' : 'var(--color-text-muted)'}; background: {favoritedModules.has(mod.id) ? '#ef444420' : 'transparent'}" role="button" tabindex="-1">{favoritedModules.has(mod.id) ? 'favorite' : 'favorite_border'}</span>
          </div>
          <!-- Batch select checkbox -->
          <div class="absolute top-5 right-16 z-10" onclick={(e) => { e.stopPropagation(); toggleSelect(mod.slug); }}>
            <div class="w-4 h-4 rounded border-2 flex items-center justify-center transition-colors" style={selectedSlugs.has(mod.slug) ? 'background: var(--color-primary); border-color: var(--color-primary)' : 'border-color: var(--color-border); background: transparent'}>
              {#if selectedSlugs.has(mod.slug)}
                <span class="material-symbols-outlined text-[10px] text-white">check</span>
              {/if}
            </div>
          </div>
          <!-- Compare checkbox -->
          <div class="absolute top-5 right-3 z-10" onclick={(e) => { e.stopPropagation(); toggleCompare(mod.slug); }}>
            <div
              class="w-5 h-5 rounded border-2 flex items-center justify-center transition-colors"
              style={compareIds.has(mod.slug) ? 'background: var(--color-primary); border-color: var(--color-primary)' : 'border-color: var(--color-border); background: transparent'}
            >
              {#if compareIds.has(mod.slug)}
                <span class="material-symbols-outlined text-[14px] text-white">check</span>
              {/if}
            </div>
          </div>
          <!-- Hover gradient overlay -->
          <div class="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-300" style="background: linear-gradient(135deg, rgba(139,92,246,0.05) 0%, rgba(6,182,212,0.03) 100%)"></div>
          
          <div class="relative z-10 pl-7">
            <div class="flex items-center gap-3 mb-3">
              <span class="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
                <span class="material-symbols-outlined text-[14px] text-amber-500">star</span>
                {mod.stars}
              </span>
              <span class="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
                <span class="material-symbols-outlined text-[14px]">download</span>
                {fmt(mod.installs)}
              </span>
              <span class="ml-auto badge text-[10px]" style={categoryStyles[mod.category] || 'background: var(--color-surface); color: var(--color-text-muted)'}>
                {mod.category}
              </span>
            </div>
            <h3 class="font-semibold text-[var(--color-text)] mb-1 line-clamp-1 group-hover:text-[var(--color-primary)] transition-colors duration-200">{mod.title}</h3>
            <div class="flex items-center gap-2 mb-2">
              <span class="text-xs px-1.5 py-0.5 rounded-md" style="background: var(--color-primary-light); color: var(--color-primary)">{mod.version}</span>
              <span class="text-xs" style="color: var(--color-text-muted)">{mod.author}</span>
            </div>
            <p class="text-sm text-[var(--color-text-secondary)] line-clamp-2 leading-relaxed">{mod.description}</p>
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<!-- Detail Modal -->
{#if selectedModule}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => selectedModule = null}>
    <div class="rounded-2xl max-w-2xl w-full max-h-[85vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
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
                <img src={selectedModule.screenshots[galleryIndex]?.url} alt="截图" class="w-full h-48 object-cover cursor-pointer" onclick={() => fullscreenScreenshot = selectedModule!.screenshots![galleryIndex]?.url} />
                {#if selectedModule.screenshots.length > 1}
                  <button class="absolute left-2 top-1/2 -translate-y-1/2 w-8 h-8 rounded-full flex items-center justify-center bg-black/40 text-white" onclick={(e) => { e.stopPropagation(); galleryIndex = galleryIndex > 0 ? galleryIndex - 1 : selectedModule!.screenshots!.length - 1; }}><span class="material-symbols-outlined text-[18px]">chevron_left</span></button>
                  <button class="absolute right-2 top-1/2 -translate-y-1/2 w-8 h-8 rounded-full flex items-center justify-center bg-black/40 text-white" onclick={(e) => { e.stopPropagation(); galleryIndex = galleryIndex < selectedModule!.screenshots!.length - 1 ? galleryIndex + 1 : 0; }}><span class="material-symbols-outlined text-[18px]">chevron_right</span></button>
              {/if}
            </div>
            {#if selectedModule.screenshots.length > 1}
              <div class="flex gap-1.5 mt-2 overflow-x-auto pb-1">
                {#each selectedModule.screenshots as ss, i}
                  <div class="w-12 h-8 rounded overflow-hidden flex-shrink-0 cursor-pointer border-2 transition-colors" style="border-color: {i === galleryIndex ? 'var(--color-primary)' : 'transparent'}" onclick={() => galleryIndex = i}>
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
            {#each reviews as rev}
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
            {#each trendingModules as mod, i}
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
            {#each templateList as t}
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

<!-- Batch Results Modal -->
{#if showBatchResults}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => showBatchResults = false}>
    <div class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">批量操作结果</h3>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => showBatchResults = false}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-2 max-h-60 overflow-auto">
        {#each batchResults as res}
          <div class="flex items-center gap-2 text-sm">
            <span class="material-symbols-outlined text-[14px]" style="color: {res.status === 'ok' ? 'var(--color-success)' : 'var(--color-error)'}">{res.status === 'ok' ? 'check_circle' : 'error'}</span>
            <span style="color: var(--color-text)">{res.slug}</span>
            {#if res.error}
              <span class="text-xs text-[var(--color-error)] ml-auto">{res.error}</span>
            {:else}
              <span class="text-xs text-[var(--color-success)] ml-auto">成功</span>
            {/if}
          </div>
        {/each}
      </div>
      <div class="p-5 border-t flex justify-end" style="border-color: var(--color-border)">
        <button class="btn-primary text-sm" onclick={() => showBatchResults = false}>关闭</button>
      </div>
    </div>
  </div>
{/if}

<!-- Fullscreen Screenshot -->
{#if fullscreenScreenshot}
  <div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.85)" onclick={() => fullscreenScreenshot = null}>
    <img src={fullscreenScreenshot} alt="截图" class="max-w-full max-h-full object-contain" onclick={(e) => e.stopPropagation()} />
  </div>
{/if}

<!-- Version History Modal -->
{#if showVersions}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => { showVersions = false; }}>
    <div class="rounded-2xl max-w-lg w-full max-h-[70vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">版本历史</h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={() => showVersions = false}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="p-4">
        <button class="w-full flex items-center gap-2 px-3 py-2 rounded-xl text-sm font-medium mb-3 transition-colors" style="background: var(--color-primary-light); color: var(--color-primary)" onclick={() => { showVersions = false; showVersionUpdate = true; }}>
          <span class="material-symbols-outlined text-[16px]">add</span>
          发布新版本
        </button>
        {#if versionsLoading}
          <div class="flex justify-center py-8"><div class="animate-spin h-6 w-6 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div></div>
        {:else if versions.length === 0}
          <p class="text-center py-8 text-sm text-[var(--color-text-muted)]">暂无版本记录</p>
        {:else}
          <div class="space-y-3">
            {#each versions as ver}
              <div class="p-4 rounded-xl" style="background: var(--color-surface)">
                <div class="flex items-start justify-between">
                  <div>
                    <div class="flex items-center gap-2 mb-1">
                      <span class="text-sm font-semibold text-[var(--color-text)]">{ver.version}</span>
                      <span class="text-xs text-[var(--color-text-muted)]">{ver.version_code}</span>
                    </div>
                    {#if ver.changelog}
                      <p class="text-xs text-[var(--color-text-secondary)] mt-1">{ver.changelog}</p>
                    {/if}
                    <p class="text-[11px] text-[var(--color-text-muted)] mt-1">{ver.created_at?.slice(0, 10)}</p>
                  </div>
                  <button
                    class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50"
                    style="background: var(--color-warning-light); color: var(--color-warning)"
                    disabled={rollingBack === ver.id}
                    onclick={() => rollback(selectedModule!.slug, ver.id)}
                  >
                    {rollingBack === ver.id ? '回滚中...' : '回滚到此版本'}
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- Version Update Dialog -->
{#if showVersionUpdate}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => { showVersionUpdate = false; }}>
    <div class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">发布新版本</h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={() => showVersionUpdate = false}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">版本代码</label>
          <input type="text" class="input-field" placeholder="1.1.0" bind:value={newVersionCode} />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">更新日志</label>
          <textarea class="input-field resize-none" rows="3" placeholder="描述本次更新内容..." bind:value={newChangelog}></textarea>
        </div>
        <div class="flex justify-end gap-3 pt-2">
          <button class="btn-ghost" onclick={() => showVersionUpdate = false}>取消</button>
          <button
            class="btn-primary disabled:opacity-50"
            disabled={updatingVersion || !newVersionCode.trim()}
            onclick={() => updateVersion(selectedModule!.slug)}
          >
            {updatingVersion ? '发布中...' : '发布'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- Comparison Modal -->
{#if compareResult}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => compareResult = null}>
    <div class="rounded-2xl max-w-3xl w-full max-h-[85vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">模块对比</h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={() => compareResult = null}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      {#if compareResult.error}
        <div class="p-8 text-center text-[var(--color-error)]">{compareResult.error}</div>
      {:else}
        <div class="p-5">
          <table class="compare-table">
            <thead>
              <tr>
                <th class="compare-label">字段</th>
                <th class="compare-value" style="color: var(--color-primary)">{compareResult.title_a}</th>
                <th class="compare-value" style="color: var(--color-primary)">{compareResult.title_b}</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td class="compare-label">描述</td>
                <td class="compare-value">{compareResult.description_a || '-'}</td>
                <td class="compare-value">{compareResult.description_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">版本</td>
                <td class="compare-value">{compareResult.version_a || '-'}</td>
                <td class="compare-value">{compareResult.version_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">类别</td>
                <td class="compare-value">{compareResult.category_a || '-'}</td>
                <td class="compare-value">{compareResult.category_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">作者</td>
                <td class="compare-value">{compareResult.author_a || '-'}</td>
                <td class="compare-value">{compareResult.author_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">许可证</td>
                <td class="compare-value">{compareResult.license_a || '-'}</td>
                <td class="compare-value">{compareResult.license_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">评分</td>
                <td class="compare-value" class:winner={compareWinner(compareResult.rating_a, compareResult.rating_b) === 'a'}>{compareResult.rating_a?.toFixed(1) || '-'}</td>
                <td class="compare-value" class:winner={compareWinner(compareResult.rating_a, compareResult.rating_b) === 'b'}>{compareResult.rating_b?.toFixed(1) || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">Stars</td>
                <td class="compare-value" class:winner={compareWinner(compareResult.stars_a, compareResult.stars_b) === 'a'}>{fmt(compareResult.stars_a)}</td>
                <td class="compare-value" class:winner={compareWinner(compareResult.stars_a, compareResult.stars_b) === 'b'}>{fmt(compareResult.stars_b)}</td>
              </tr>
              <tr>
                <td class="compare-label">安装量</td>
                <td class="compare-value" class:winner={compareWinner(compareResult.installs_a, compareResult.installs_b) === 'a'}>{fmt(compareResult.installs_a)}</td>
                <td class="compare-value" class:winner={compareWinner(compareResult.installs_a, compareResult.installs_b) === 'b'}>{fmt(compareResult.installs_b)}</td>
              </tr>
              <tr>
                <td class="compare-label">依赖数</td>
                <td class="compare-value" class:winner={compareWinner(compareResult.dep_count_b, compareResult.dep_count_a) === 'a'}>{compareResult.dep_count_a}</td>
                <td class="compare-value" class:winner={compareWinner(compareResult.dep_count_b, compareResult.dep_count_a) === 'b'}>{compareResult.dep_count_b}</td>
              </tr>
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Install to Device Modal -->
{#if showInstallModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={() => { if (!installing) showInstallModal = false; }}>
    <div class="rounded-2xl w-full max-w-md border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: 0 25px 50px -12px rgba(0,0,0,0.5)" onclick={(e) => e.stopPropagation()}>
      <!-- Header -->
      <div class="p-5 border-b flex items-center gap-3" style="border-color: var(--color-border)">
        <div class="w-10 h-10 rounded-xl flex items-center justify-center" style="background: var(--gradient-brand)">
          <span class="material-symbols-outlined text-white text-[20px]">download</span>
        </div>
        <div class="flex-1 min-w-0">
          <h3 class="text-base font-bold text-[var(--color-text)] truncate">安装 {selectedModule?.title || selectedModule?.slug}</h3>
          <p class="text-xs text-[var(--color-text-muted)]">v{selectedModule?.version}</p>
        </div>
        {#if !installing}
          <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={() => showInstallModal = false}>
            <span class="material-symbols-outlined text-[18px]" style="color: var(--color-text-muted)">close</span>
          </button>
        {/if}
      </div>

      <!-- Steps -->
      <div class="p-5 space-y-1">
        {#each installSteps as step, i}
          <div class="flex items-start gap-3 py-3 {i < installSteps.length - 1 ? 'border-b' : ''}" style={i < installSteps.length - 1 ? 'border-color: var(--color-border)' : ''}>
            <!-- Step Icon -->
            <div class="mt-0.5 flex-shrink-0">
              {#if step.status === 'done'}
                <div class="w-7 h-7 rounded-full flex items-center justify-center" style="background: var(--color-success-light, rgba(34,197,94,0.15))">
                  <span class="material-symbols-outlined text-[16px]" style="color: var(--color-success, #22c55e)">check_circle</span>
                </div>
              {:else if step.status === 'running'}
                <div class="w-7 h-7 rounded-full flex items-center justify-center" style="background: var(--color-primary-light, rgba(59,130,246,0.15))">
                  <div class="animate-spin h-4 w-4 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
                </div>
              {:else if step.status === 'error'}
                <div class="w-7 h-7 rounded-full flex items-center justify-center" style="background: var(--color-error-light, rgba(239,68,68,0.15))">
                  <span class="material-symbols-outlined text-[16px]" style="color: var(--color-error, #ef4444)">error</span>
                </div>
              {:else}
                <div class="w-7 h-7 rounded-full flex items-center justify-center" style="background: var(--color-surface)">
                  <span class="text-xs font-bold" style="color: var(--color-text-muted)">{i + 1}</span>
                </div>
              {/if}
            </div>
            <!-- Step Content -->
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium {step.status === 'pending' ? 'text-[var(--color-text-muted)]' : 'text-[var(--color-text)]'}">{step.label}</div>
              {#if step.detail}
                <div class="mt-1 text-xs font-mono {step.status === 'error' ? 'text-[var(--color-error)]' : 'text-[var(--color-text-muted)]'} truncate" title={step.detail}>{step.detail}</div>
              {/if}
            </div>
          </div>
        {/each}
      </div>

      <!-- Error Banner -->
      {#if installError}
        <div class="mx-5 mb-3 p-3 rounded-xl text-xs" style="background: var(--color-error-light, rgba(239,68,68,0.1)); border: 1px solid var(--color-error-border, rgba(239,68,68,0.2))">
          <span style="color: var(--color-error, #ef4444)">{installError}</span>
        </div>
      {/if}

      <!-- Footer -->
      <div class="p-5 border-t flex items-center gap-3" style="border-color: var(--color-border)">
        {#if installing}
          <div class="flex-1 text-xs text-[var(--color-text-muted)]">
            <span class="animate-pulse">▊</span> 正在安装...
          </div>
        {:else if installSteps.every(s => s.status === 'done')}
          <div class="flex-1"></div>
          <button class="px-5 py-2 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--gradient-brand)" onclick={() => showInstallModal = false}>完成</button>
        {:else if installSteps.some(s => s.status === 'error')}
          <button class="px-4 py-2 rounded-xl text-sm font-medium transition-colors" style="border: 1px solid var(--color-border); color: var(--color-text-secondary)" onclick={() => showInstallModal = false}>关闭</button>
          <button class="px-5 py-2 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--gradient-brand)" onclick={startInstall}>重试</button>
        {:else}
          <!-- Device Selector -->
          <div class="flex-1">
            {#if loadingDevices}
              <span class="text-xs text-[var(--color-text-muted)]">检测设备中...</span>
            {:else if installableDevices.length === 0}
              <span class="text-xs text-[var(--color-error)]">未发现已连接设备</span>
            {:else}
              <select class="w-full px-3 py-2 rounded-xl text-sm border appearance-none" style="background: var(--color-surface); border-color: var(--color-border); color: var(--color-text)" bind:value={installDevice}>
                <option value="">选择设备...</option>
                {#each installableDevices as dev}
                  <option value={dev.serial}>{dev.model || dev.serial} ({dev.serial})</option>
                {/each}
              </select>
            {/if}
          </div>
          <button class="px-5 py-2 rounded-xl text-sm font-medium text-white transition-colors disabled:opacity-50" style="background: var(--gradient-brand)" disabled={!installDevice || loadingDevices} onclick={startInstall}>
            <span class="material-symbols-outlined text-[14px] align-text-bottom">download</span>
            开始安装
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- Module Demo Modal -->
{#if showDemo}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" onclick={closeDemo}>
    <div class="rounded-2xl max-w-lg w-full max-h-[90vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" onclick={(e) => e.stopPropagation()}>
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)] flex items-center gap-2">
          <span class="material-symbols-outlined text-[20px] text-purple-500">preview</span>
          试用预览
        </h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={closeDemo}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      {#if demoLoading}
        <div class="flex justify-center py-12">
          <div class="animate-spin h-8 w-8 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
        </div>
      {:else if demoData?.error}
        <div class="p-8 text-center text-[var(--color-error)]">{demoData.error}</div>
      {:else}
        <div class="p-5 space-y-5">
          <!-- Simulated Install Output -->
          <div>
            <h4 class="text-sm font-semibold text-[var(--color-text)] mb-2">模拟安装过程</h4>
            <pre class="p-3 rounded-xl text-xs font-mono leading-relaxed overflow-auto max-h-40 whitespace-pre-wrap" style="background: #0a0a0a; color: #4ade80">
              {#each (demoData?.simulated_output || '').split('\n').slice(0, demoVisibleLines) as line}
                {line}
                <br>
              {/each}
              {#if demoVisibleLines < (demoData?.simulated_output || '').split('\n').length}
                <span class="animate-pulse">▊</span>
              {:else}
                <span class="text-green-300">✓ 模拟完成</span>
              {/if}
            </pre>
          </div>
          <!-- Props Comparison -->
          {#if demoData?.props?.length}
            <div>
              <h4 class="text-sm font-semibold text-[var(--color-text)] mb-2">修改的系统属性</h4>
              <div class="space-y-2">
                {#each demoData.props as prop}
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
          {#if demoData?.files?.length}
            <div>
              <h4 class="text-sm font-semibold text-[var(--color-text)] mb-2">影响的文件路径</h4>
              <div class="space-y-1">
                {#each demoData.files as file}
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
            <button class="flex-1 py-2.5 rounded-xl font-semibold text-sm text-white transition-all flex items-center justify-center gap-2" style="background: var(--gradient-brand)" onclick={() => { closeDemo(); window.location.href = '/devices'; }}>
              <span class="material-symbols-outlined text-[16px]">smartphone</span>
              实际安装
            </button>
            <button class="py-2.5 px-5 rounded-xl text-sm font-medium transition-colors" style="border: 1px solid var(--color-border); color: var(--color-text-secondary)" onclick={closeDemo}>
              关闭
            </button>
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

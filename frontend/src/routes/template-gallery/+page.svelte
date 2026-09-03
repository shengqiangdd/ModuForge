<script lang="ts">
  import { onMount } from 'svelte';

  interface TemplateFile {
    path: string;
    content: string;
  }

  interface Template {
    name: string;
    description: string;
    category: string;
    tags: string[];
    files: TemplateFile[];
  }

  let templates = $state<Template[]>([]);
  let loading = $state(true);
  let selectedCategory = $state('all');
  let searchQuery = $state('');
  let selectedTemplate = $state<Template | null>(null);
  let showPreview = $state(false);
  let creatingProject = $state(false);
  let projectName = $state('');
  let projectDesc = $state('');
  let showCreateModal = $state(false);

  const categoryIcons: Record<string, string> = {
    system: 'settings',
    ui: 'palette',
    module: 'extension',
    performance: 'speed',
    battery: 'battery_charging_full',
    network: 'wifi',
    utility: 'build',
  };

  const categoryLabels: Record<string, string> = {
    system: '系统',
    ui: '界面',
    module: '服务',
    performance: '性能',
    battery: '电池',
    network: '网络',
    utility: '工具',
  };

  let categories = $derived(() => {
    const cats = new Set(templates.map(t => t.category));
    return ['all', ...Array.from(cats)];
  });

  let filtered = $derived(() => {
    let result = templates;
    if (selectedCategory !== 'all') {
      result = result.filter(t => t.category === selectedCategory);
    }
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      result = result.filter(t =>
        t.name.toLowerCase().includes(q) ||
        t.description.toLowerCase().includes(q) ||
        t.tags.some(tag => tag.toLowerCase().includes(q))
      );
    }
    return result;
  });

  async function loadTemplates() {
    loading = true;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/templates/list', {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        templates = Array.isArray(data) ? data : (data.templates || []);
      }
    } catch (e) {
      console.error('Failed to load templates:', e);
    }
    loading = false;
  }

  function previewTemplate(t: Template) {
    selectedTemplate = t;
    showPreview = true;
  }

  function openCreateFromTemplate(t: Template) {
    selectedTemplate = t;
    projectName = t.name;
    projectDesc = t.description;
    showCreateModal = true;
  }

  async function createProjectFromTemplate() {
    if (!selectedTemplate || !projectName.trim()) return;
    creatingProject = true;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/templates/use', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          template_name: selectedTemplate.name,
          project_name: projectName,
          description: projectDesc,
        })
      });
      if (res.ok) {
        const data = await res.json();
        showCreateModal = false;
        showPreview = false;
        alert(`项目已创建：${data.name || projectName}`);
      }
    } catch (e) {
      console.error('Failed to create project:', e);
    }
    creatingProject = false;
  }

  function getFileCount(t: Template): number {
    return t.files?.length || 0;
  }

  function getLanguage(t: Template): string {
    const file = t.files?.[0];
    if (!file) return 'unknown';
    if (file.path.endsWith('.go')) return 'Go';
    if (file.path.endsWith('.rs')) return 'Rust';
    if (file.path.endsWith('.sh')) return 'Shell';
    if (file.path.endsWith('.kt')) return 'Kotlin';
    if (file.path.endsWith('.c')) return 'C';
    return 'Shell';
  }

  onMount(loadTemplates);
</script>

<div class="page">
  <!-- Header -->
  <div class="header">
    <div>
      <h1>模块模板库</h1>
      <p class="subtitle">选择预配置模板，快速创建 Android 模块项目</p>
    </div>
  </div>

  <!-- Search & Filters -->
  <div class="filters">
    <div class="search-box">
      <span class="material-symbols-outlined search-icon">search</span>
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="搜索模板名称、描述或标签..."
        class="search-input"
      />
    </div>
    <div class="category-tabs">
      {#each categories() as cat}
        <button
          class="category-tab"
          class:active={selectedCategory === cat}
          onclick={() => selectedCategory = cat}
        >
          {#if cat !== 'all'}
            <span class="material-symbols-outlined tab-icon">{categoryIcons[cat] || 'folder'}</span>
          {/if}
          {cat === 'all' ? '全部' : (categoryLabels[cat] || cat)}
        </button>
      {/each}
    </div>
  </div>

  <!-- Template Grid -->
  {#if loading}
    <div class="loading-grid">
      {#each Array(6) as _}
        <div class="skeleton-card">
          <div class="skeleton-line w60"></div>
          <div class="skeleton-line w80"></div>
          <div class="skeleton-line w40"></div>
        </div>
      {/each}
    </div>
  {:else if filtered().length === 0}
    <div class="empty-state">
      <span class="material-symbols-outlined empty-icon">widgets</span>
      <p>没有找到匹配的模板</p>
      <button class="btn-outline" onclick={() => { searchQuery = ''; selectedCategory = 'all'; }}>清除筛选</button>
    </div>
  {:else}
    <div class="template-grid">
      {#each filtered() as t}
        <div class="template-card" role="button" tabindex="0" onclick={() => previewTemplate(t)} onkeydown={(e) => { if (e.key === 'Enter') previewTemplate(t); }}>
          <div class="card-badge" style="background: color-mix(in srgb, var(--color-primary) 15%, transparent); color: var(--color-primary);">
            <span class="material-symbols-outlined" style="font-size: 16px;">{categoryIcons[t.category] || 'folder'}</span>
            {categoryLabels[t.category] || t.category}
          </div>
          <h3 class="card-title">{t.name}</h3>
          <p class="card-desc">{t.description}</p>
          <div class="card-meta">
            <span class="meta-item">
              <span class="material-symbols-outlined" style="font-size: 14px;">folder_open</span>
              {getFileCount(t)} 个文件
            </span>
            <span class="meta-item">
              <span class="material-symbols-outlined" style="font-size: 14px;">code</span>
              {getLanguage(t)}
            </span>
          </div>
          <div class="card-tags">
            {#each t.tags.slice(0, 3) as tag}
              <span class="tag">{tag}</span>
            {/each}
          </div>
          <div class="card-actions">
            <button class="btn-sm primary" onclick={(e) => { e.stopPropagation(); openCreateFromTemplate(t); }}>
              <span class="material-symbols-outlined" style="font-size: 16px;">add</span>
              使用模板
            </button>
            <button class="btn-sm outline" onclick={(e) => { e.stopPropagation(); previewTemplate(t); }}>
              <span class="material-symbols-outlined" style="font-size: 16px;">visibility</span>
              预览
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Preview Modal -->
{#if showPreview && selectedTemplate}
  <div class="overlay" onclick={() => showPreview = false} role="dialog" aria-modal="true">
    <div class="modal preview-modal" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <div>
          <h2>{selectedTemplate.name}</h2>
          <p class="modal-subtitle">{selectedTemplate.description}</p>
        </div>
        <button class="btn-icon" onclick={() => showPreview = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="modal-body">
        <div class="preview-info">
          <div class="info-row">
            <span class="info-label">分类</span>
            <span class="info-value">{categoryLabels[selectedTemplate.category] || selectedTemplate.category}</span>
          </div>
          <div class="info-row">
            <span class="info-label">文件数</span>
            <span class="info-value">{getFileCount(selectedTemplate)}</span>
          </div>
          <div class="info-row">
            <span class="info-label">标签</span>
            <div class="info-tags">
              {#each selectedTemplate.tags as tag}
                <span class="tag">{tag}</span>
              {/each}
            </div>
          </div>
        </div>

        <h3 class="section-title">文件结构</h3>
        <div class="file-tree">
          {#each selectedTemplate.files as file}
            <div class="file-item">
              <span class="material-symbols-outlined file-icon">description</span>
              <span class="file-path">{file.path}</span>
              <span class="file-size">{file.content.length} bytes</span>
            </div>
          {/each}
        </div>

        {#if selectedTemplate.files.length > 0}
          <h3 class="section-title">代码预览</h3>
          <div class="code-preview">
            <pre><code>{selectedTemplate.files[0].content}</code></pre>
          </div>
        {/if}
      </div>

      <div class="modal-footer">
        <button class="btn-outline" onclick={() => showPreview = false}>取消</button>
        <button class="btn-primary" onclick={() => { showPreview = false; openCreateFromTemplate(selectedTemplate!); }}>
          <span class="material-symbols-outlined" style="font-size: 18px;">add</span>
          使用此模板创建项目
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Create Project Modal -->
{#if showCreateModal && selectedTemplate}
  <div class="overlay" onclick={() => showCreateModal = false} role="dialog" aria-modal="true">
    <div class="modal create-modal" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <h2>从模板创建项目</h2>
        <button class="btn-icon" onclick={() => showCreateModal = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="modal-body">
        <div class="form-group">
          <label for="project-name">项目名称</label>
          <input id="project-name" type="text" bind:value={projectName} placeholder="输入项目名称" />
        </div>
        <div class="form-group">
          <label for="project-desc">项目描述</label>
          <textarea id="project-desc" bind:value={projectDesc} rows="3" placeholder="输入项目描述"></textarea>
        </div>
        <div class="template-summary">
          <span class="material-symbols-outlined" style="color: var(--color-primary);">extension</span>
          <span>将使用模板 <strong>{selectedTemplate.name}</strong> 创建项目，包含 {getFileCount(selectedTemplate)} 个预配置文件</span>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn-outline" onclick={() => showCreateModal = false}>取消</button>
        <button class="btn-primary" onclick={createProjectFromTemplate} disabled={creatingProject || !projectName.trim()}>
          {#if creatingProject}
            <span class="spinner"></span> 创建中...
          {:else}
            <span class="material-symbols-outlined" style="font-size: 18px;">rocket_launch</span>
            创建项目
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .page { padding: 1.5rem; max-width: 1200px; margin: 0 auto; }
  .header { margin-bottom: 1.5rem; }
  .header h1 { font-size: 1.5rem; font-weight: 700; color: var(--color-text); margin: 0; }
  .subtitle { color: var(--color-text-muted); font-size: 0.9rem; margin-top: 0.25rem; }

  .filters { margin-bottom: 1.5rem; }
  .search-box { position: relative; margin-bottom: 1rem; }
  .search-icon { position: absolute; left: 12px; top: 50%; transform: translateY(-50%); color: var(--color-text-muted); font-size: 20px; }
  .search-input { width: 100%; padding: 0.7rem 1rem 0.7rem 2.5rem; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-bg-card); color: var(--color-text); font-size: 0.9rem; outline: none; transition: border-color 0.2s; }
  .search-input:focus { border-color: var(--color-primary); }

  .category-tabs { display: flex; gap: 0.5rem; flex-wrap: wrap; }
  .category-tab { display: flex; align-items: center; gap: 4px; padding: 0.4rem 0.8rem; border: 1px solid var(--color-border); border-radius: 20px; background: transparent; color: var(--color-text-secondary); font-size: 0.8rem; cursor: pointer; transition: all 0.2s; }
  .category-tab:hover { border-color: var(--color-primary); color: var(--color-primary); }
  .category-tab.active { background: color-mix(in srgb, var(--color-primary) 12%, transparent); border-color: var(--color-primary); color: var(--color-primary); font-weight: 600; }
  .tab-icon { font-size: 16px; }

  .template-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 1rem; }
  .template-card { border: 1px solid var(--color-border); border-radius: 14px; padding: 1.2rem; background: var(--color-bg-card); cursor: pointer; transition: all 0.2s; position: relative; }
  .template-card:hover { border-color: var(--color-primary); box-shadow: 0 4px 20px color-mix(in srgb, var(--color-primary) 10%, transparent); transform: translateY(-2px); }
  .card-badge { display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; border-radius: 12px; font-size: 0.75rem; font-weight: 600; margin-bottom: 0.75rem; }
  .card-title { font-size: 1.05rem; font-weight: 700; color: var(--color-text); margin: 0 0 0.4rem; }
  .card-desc { font-size: 0.85rem; color: var(--color-text-secondary); margin: 0 0 0.75rem; line-height: 1.4; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .card-meta { display: flex; gap: 1rem; margin-bottom: 0.75rem; }
  .meta-item { display: flex; align-items: center; gap: 4px; font-size: 0.8rem; color: var(--color-text-muted); }
  .card-tags { display: flex; gap: 0.3rem; flex-wrap: wrap; margin-bottom: 0.75rem; }
  .tag { padding: 2px 8px; border-radius: 10px; font-size: 0.7rem; background: color-mix(in srgb, var(--color-text-muted) 10%, transparent); color: var(--color-text-muted); }
  .card-actions { display: flex; gap: 0.5rem; }

  .btn-sm { display: inline-flex; align-items: center; gap: 4px; padding: 0.35rem 0.7rem; border-radius: 8px; font-size: 0.8rem; font-weight: 500; cursor: pointer; transition: all 0.2s; border: none; }
  .btn-sm.primary { background: var(--color-primary); color: white; }
  .btn-sm.primary:hover { opacity: 0.9; }
  .btn-sm.outline { border: 1px solid var(--color-border); background: transparent; color: var(--color-text-secondary); }
  .btn-sm.outline:hover { border-color: var(--color-primary); color: var(--color-primary); }

  .overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; backdrop-filter: blur(4px); }
  .modal { background: var(--color-bg-card); border-radius: 16px; width: 90%; max-height: 85vh; display: flex; flex-direction: column; box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
  .preview-modal { max-width: 700px; }
  .create-modal { max-width: 480px; }
  .modal-header { display: flex; justify-content: space-between; align-items: flex-start; padding: 1.2rem 1.5rem; border-bottom: 1px solid var(--color-border); }
  .modal-header h2 { margin: 0; font-size: 1.15rem; color: var(--color-text); }
  .modal-subtitle { margin: 0.25rem 0 0; font-size: 0.85rem; color: var(--color-text-muted); }
  .btn-icon { padding: 6px; border: none; background: transparent; border-radius: 8px; cursor: pointer; color: var(--color-text-muted); }
  .btn-icon:hover { background: color-mix(in srgb, var(--color-text-muted) 10%, transparent); }
  .modal-body { padding: 1.5rem; overflow-y: auto; flex: 1; }
  .modal-footer { display: flex; justify-content: flex-end; gap: 0.5rem; padding: 1rem 1.5rem; border-top: 1px solid var(--color-border); }

  .preview-info { margin-bottom: 1.5rem; }
  .info-row { display: flex; align-items: center; gap: 1rem; padding: 0.5rem 0; border-bottom: 1px solid var(--color-border); }
  .info-row:last-child { border-bottom: none; }
  .info-label { font-size: 0.8rem; color: var(--color-text-muted); min-width: 60px; }
  .info-value { font-size: 0.85rem; color: var(--color-text); }
  .info-tags { display: flex; gap: 0.3rem; flex-wrap: wrap; }

  .section-title { font-size: 0.9rem; font-weight: 600; color: var(--color-text); margin: 1rem 0 0.5rem; }

  .file-tree { background: var(--color-bg); border-radius: 10px; padding: 0.75rem; }
  .file-item { display: flex; align-items: center; gap: 8px; padding: 0.4rem 0.5rem; border-radius: 6px; font-size: 0.85rem; }
  .file-item:hover { background: color-mix(in srgb, var(--color-text-muted) 5%, transparent); }
  .file-icon { font-size: 18px; color: var(--color-primary); }
  .file-path { color: var(--color-text); flex: 1; font-family: monospace; font-size: 0.8rem; }
  .file-size { color: var(--color-text-muted); font-size: 0.75rem; }

  .code-preview { background: #1e1e2e; border-radius: 10px; padding: 1rem; max-height: 300px; overflow: auto; }
  .code-preview pre { margin: 0; }
  .code-preview code { color: #cdd6f4; font-family: 'JetBrains Mono', monospace; font-size: 0.8rem; line-height: 1.5; white-space: pre-wrap; }

  .form-group { margin-bottom: 1rem; }
  .form-group label { display: block; font-size: 0.85rem; font-weight: 500; color: var(--color-text); margin-bottom: 0.3rem; }
  .form-group input, .form-group textarea { width: 100%; padding: 0.6rem 0.8rem; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-bg); color: var(--color-text); font-size: 0.9rem; outline: none; font-family: inherit; }
  .form-group input:focus, .form-group textarea:focus { border-color: var(--color-primary); }

  .template-summary { display: flex; align-items: center; gap: 8px; padding: 0.75rem; border-radius: 8px; background: color-mix(in srgb, var(--color-primary) 8%, transparent); color: var(--color-text-secondary); font-size: 0.85rem; }

  .btn-primary { display: inline-flex; align-items: center; gap: 6px; padding: 0.55rem 1.2rem; border: none; border-radius: 10px; background: var(--color-primary); color: white; font-weight: 600; font-size: 0.9rem; cursor: pointer; transition: opacity 0.2s; }
  .btn-primary:hover { opacity: 0.9; }
  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-outline { padding: 0.55rem 1.2rem; border: 1px solid var(--color-border); border-radius: 10px; background: transparent; color: var(--color-text-secondary); font-size: 0.9rem; cursor: pointer; }

  .spinner { width: 16px; height: 16px; border: 2px solid rgba(255,255,255,0.3); border-top-color: white; border-radius: 50%; animation: spin 0.6s linear infinite; display: inline-block; }
  @keyframes spin { to { transform: rotate(360deg); } }

  .loading-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 1rem; }
  .skeleton-card { border: 1px solid var(--color-border); border-radius: 14px; padding: 1.2rem; background: var(--color-bg-card); }
  .skeleton-line { height: 14px; border-radius: 6px; background: color-mix(in srgb, var(--color-text-muted) 10%, transparent); margin-bottom: 0.75rem; }
  .w60 { width: 60%; } .w80 { width: 80%; } .w40 { width: 40%; }

  .empty-state { text-align: center; padding: 4rem 2rem; }
  .empty-icon { font-size: 4rem; color: var(--color-text-muted); display: block; margin-bottom: 1rem; }
  .empty-state p { color: var(--color-text-secondary); margin-bottom: 1rem; }
</style>

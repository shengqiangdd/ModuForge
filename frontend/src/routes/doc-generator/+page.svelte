<script lang="ts">
  import { onMount } from 'svelte';

  let projects = $state<any[]>([]);
  let selectedProjectId = $state('');
  let generating = $state(false);
  let generatedDocs = $state<any[]>([]);
  let activeDoc = $state('readme');

  // Form fields
  let projectName = $state('');
  let projectDesc = $state('');
  let author = $state('');
  let version = $state('v1.0');
  let moduleType = $state('magisk');
  let licenseType = $state('GPL-3.0');
  let tags = $state('');
  let minApi = $state(26);
  let architectures = $state(['arm64']);
  let hasDaemon = $state(false);
  let hasWebUI = $state(false);
  let hasService = $state(true);

  async function loadProjects() {
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/projects', {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        projects = Array.isArray(data) ? data : (data.projects || []);
      }
    } catch (e) {
      console.error('Failed to load projects:', e);
    }
  }

  async function selectProject(id: string) {
    selectedProjectId = id;
    if (!id) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/projects/${id}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        projectName = data.name || '';
        projectDesc = data.description || '';
        author = data.author || '';
        version = data.version || 'v1.0';
        moduleType = data.module_type || 'magisk';
      }
    } catch (e) {
      console.error('Failed to load project:', e);
    }
  }

  async function generateDocs() {
    generating = true;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/doc-generator/generate', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          project_id: selectedProjectId || undefined,
          project_name: projectName,
          description: projectDesc,
          author,
          version,
          module_type: moduleType,
          license: licenseType,
          tags: tags.split(',').map(t => t.trim()).filter(Boolean),
          min_api: minApi,
          architectures,
          has_daemon: hasDaemon,
          has_webui: hasWebUI,
          has_service: hasService,
        })
      });
      if (res.ok) {
        const data = await res.json();
        generatedDocs = Array.isArray(data) ? data : (data.docs || []);
        if (generatedDocs.length > 0) activeDoc = generatedDocs[0].type;
      }
    } catch (e) {
      console.error('Failed to generate docs:', e);
    }
    generating = false;
  }

  async function downloadDoc(doc: any) {
    const blob = new Blob([doc.content], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = doc.filename;
    a.click();
    URL.revokeObjectURL(url);
  }

  async function saveDocsToProject() {
    if (!selectedProjectId || generatedDocs.length === 0) return;
    try {
      const token = localStorage.getItem('token');
      for (const doc of generatedDocs) {
        await fetch(`/api/v1/projects/${selectedProjectId}/files/${doc.filename}`, {
          method: 'PUT',
          headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
          body: JSON.stringify({ content: doc.content })
        });
      }
      alert('文档已保存到项目');
    } catch (e) {
      console.error('Failed to save docs:', e);
    }
  }

  let activeDocContent = $derived(() => {
    const doc = generatedDocs.find(d => d.type === activeDoc);
    return doc?.content || '';
  });

  let activeDocFilename = $derived(() => {
    const doc = generatedDocs.find(d => d.type === activeDoc);
    return doc?.filename || '';
  });

  onMount(loadProjects);
</script>

<div class="page">
  <div class="header">
    <h1>模块文档生成</h1>
    <p class="subtitle">自动生成专业的模块文档：README、使用指南、API 文档、更新日志</p>
  </div>

  <div class="layout">
    <!-- Left: Config -->
    <div class="config-panel">
      <div class="panel-section">
        <h3>项目选择</h3>
        <select bind:value={selectedProjectId} onchange={() => selectProject(selectedProjectId)}>
          <option value="">手动填写</option>
          {#each projects as p}
            <option value={p.id}>{p.name}</option>
          {/each}
        </select>
      </div>

      <div class="panel-section">
        <h3>基本信息</h3>
        <label>
          模块名称 *
          <input type="text" bind:value={projectName} placeholder="MyModule" />
        </label>
        <label>
          描述
          <textarea bind:value={projectDesc} rows="2" placeholder="模块功能描述"></textarea>
        </label>
        <label>
          作者
          <input type="text" bind:value={author} placeholder="Author Name" />
        </label>
        <div class="row">
          <label>
            版本
            <input type="text" bind:value={version} placeholder="v1.0" />
          </label>
          <label>
            最低 API
            <input type="number" bind:value={minApi} min="21" max="35" />
          </label>
        </div>
      </div>

      <div class="panel-section">
        <h3>模块配置</h3>
        <label>
          模块类型
          <select bind:value={moduleType}>
            <option value="magisk">Magisk</option>
            <option value="ksu">KernelSU</option>
            <option value="apatch">APatch</option>
            <option value="universal">Universal</option>
          </select>
        </label>
        <label>
          许可证
          <select bind:value={licenseType}>
            <option value="GPL-3.0">GPL-3.0</option>
            <option value="MIT">MIT</option>
            <option value="Apache-2.0">Apache-2.0</option>
            <option value="LGPL-3.0">LGPL-3.0</option>
          </select>
        </label>
        <label>
          标签（逗号分隔）
          <input type="text" bind:value={tags} placeholder="magisk, performance, android" />
        </label>
      </div>

      <div class="panel-section">
        <h3>功能特性</h3>
        <div class="checkbox-group">
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={hasDaemon} />
            <span>守护进程 (Daemon)</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={hasWebUI} />
            <span>Web 界面 (WebUI)</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={hasService} />
            <span>后台服务 (service.sh)</span>
          </label>
        </div>
        <label>
          支持架构
          <div class="arch-options">
            {#each ['arm64', 'arm', 'x86_64', 'x86'] as arch}
              <label class="chip" class:selected={architectures.includes(arch)}>
                <input type="checkbox" value={arch}
                  checked={architectures.includes(arch)}
                  onchange={() => {
                    if (architectures.includes(arch)) {
                      architectures = architectures.filter(a => a !== arch);
                    } else {
                      architectures = [...architectures, arch];
                    }
                  }}
                />
                {arch}
              </label>
            {/each}
          </div>
        </label>
      </div>

      <button class="btn-primary full-width" onclick={generateDocs} disabled={generating || !projectName.trim()}>
        {#if generating}
          <span class="spinner"></span> 生成中...
        {:else}
          <span class="material-symbols-outlined" style="font-size: 18px;">auto_awesome</span>
          生成文档
        {/if}
      </button>
    </div>

    <!-- Right: Preview -->
    <div class="preview-panel">
      {#if generatedDocs.length === 0}
        <div class="empty-preview">
          <span class="material-symbols-outlined" style="font-size: 4rem; color: var(--color-text-muted);">description</span>
          <p>配置左侧选项后点击「生成文档」</p>
          <p class="hint">将自动生成 README.md、USAGE.md、API.md、CHANGELOG.md</p>
        </div>
      {:else}
        <div class="doc-tabs">
          {#each generatedDocs as doc}
            <button class="doc-tab" class:active={activeDoc === doc.type} onclick={() => activeDoc = doc.type}>
              <span class="material-symbols-outlined" style="font-size: 16px;">
                {doc.type === 'readme' ? 'book' : doc.type === 'usage' ? 'help' : doc.type === 'api' ? 'api' : 'history'}
              </span>
              {doc.filename}
            </button>
          {/each}
          <div class="doc-actions">
            <button class="btn-sm outline" onclick={() => { const doc = generatedDocs.find(d => d.type === activeDoc); if (doc) downloadDoc(doc); }}>
              <span class="material-symbols-outlined" style="font-size: 16px;">download</span>
              下载
            </button>
            {#if selectedProjectId}
              <button class="btn-sm primary" onclick={saveDocsToProject}>
                <span class="material-symbols-outlined" style="font-size: 16px;">save</span>
                保存到项目
              </button>
            {/if}
          </div>
        </div>
        <div class="doc-content">
          <pre><code>{activeDocContent()}</code></pre>
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .page { padding: 1.5rem; max-width: 1400px; margin: 0 auto; }
  .header { margin-bottom: 1.5rem; }
  .header h1 { font-size: 1.5rem; font-weight: 700; color: var(--color-text); margin: 0; }
  .subtitle { color: var(--color-text-muted); font-size: 0.9rem; margin-top: 0.25rem; }

  .layout { display: grid; grid-template-columns: 340px 1fr; gap: 1.5rem; align-items: start; }
  @media (max-width: 900px) { .layout { grid-template-columns: 1fr; } }

  .config-panel, .preview-panel { border: 1px solid var(--color-border); border-radius: 14px; background: var(--color-bg-card); }
  .config-panel { padding: 1.25rem; }

  .panel-section { margin-bottom: 1.25rem; padding-bottom: 1.25rem; border-bottom: 1px solid var(--color-border); }
  .panel-section:last-of-type { border-bottom: none; }
  .panel-section h3 { font-size: 0.85rem; font-weight: 600; color: var(--color-text); margin: 0 0 0.75rem; }
  .panel-section label { display: block; margin-bottom: 0.75rem; font-size: 0.8rem; color: var(--color-text-secondary); }
  .panel-section input, .panel-section select, .panel-section textarea {
    width: 100%; margin-top: 0.25rem; padding: 0.45rem 0.6rem; border: 1px solid var(--color-border);
    border-radius: 8px; background: var(--color-bg); color: var(--color-text); font-size: 0.85rem; outline: none; font-family: inherit;
  }
  .panel-section input:focus, .panel-section select:focus, .panel-section textarea:focus { border-color: var(--color-primary); }
  .row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }

  .checkbox-group { display: flex; flex-direction: column; gap: 0.4rem; }
  .checkbox-label { display: flex; align-items: center; gap: 0.5rem; font-size: 0.85rem; color: var(--color-text); cursor: pointer; }
  .checkbox-label input { width: auto; margin: 0; }

  .arch-options { display: flex; flex-wrap: wrap; gap: 0.4rem; margin-top: 0.3rem; }
  .chip { display: inline-flex; align-items: center; gap: 4px; padding: 0.25rem 0.6rem; border: 1px solid var(--color-border); border-radius: 16px; font-size: 0.75rem; cursor: pointer; color: var(--color-text-secondary); transition: all 0.2s; }
  .chip input { display: none; }
  .chip.selected { background: color-mix(in srgb, var(--color-primary) 12%, transparent); border-color: var(--color-primary); color: var(--color-primary); font-weight: 600; }

  .btn-primary { display: inline-flex; align-items: center; gap: 6px; padding: 0.6rem 1.2rem; border: none; border-radius: 10px; background: var(--color-primary); color: white; font-weight: 600; font-size: 0.9rem; cursor: pointer; }
  .btn-primary:hover { opacity: 0.9; }
  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
  .full-width { width: 100%; justify-content: center; }

  .empty-preview { display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 4rem 2rem; text-align: center; }
  .empty-preview p { color: var(--color-text-secondary); margin: 0.5rem 0 0; }
  .hint { font-size: 0.8rem; color: var(--color-text-muted); }

  .doc-tabs { display: flex; align-items: center; gap: 0.3rem; padding: 0.75rem; border-bottom: 1px solid var(--color-border); overflow-x: auto; flex-wrap: wrap; }
  .doc-tab { display: inline-flex; align-items: center; gap: 4px; padding: 0.35rem 0.7rem; border: 1px solid var(--color-border); border-radius: 8px; background: transparent; font-size: 0.8rem; cursor: pointer; color: var(--color-text-secondary); white-space: nowrap; transition: all 0.2s; }
  .doc-tab.active { background: color-mix(in srgb, var(--color-primary) 12%, transparent); border-color: var(--color-primary); color: var(--color-primary); font-weight: 600; }
  .doc-actions { display: flex; gap: 0.4rem; margin-left: auto; }

  .btn-sm { display: inline-flex; align-items: center; gap: 4px; padding: 0.3rem 0.6rem; border-radius: 8px; font-size: 0.78rem; font-weight: 500; cursor: pointer; border: none; transition: all 0.2s; }
  .btn-sm.primary { background: var(--color-primary); color: white; }
  .btn-sm.outline { border: 1px solid var(--color-border); background: transparent; color: var(--color-text-secondary); }

  .doc-content { padding: 1rem; max-height: 600px; overflow: auto; }
  .doc-content pre { margin: 0; white-space: pre-wrap; }
  .doc-content code { font-family: 'JetBrains Mono', monospace; font-size: 0.82rem; line-height: 1.6; color: var(--color-text); }

  .spinner { width: 16px; height: 16px; border: 2px solid rgba(255,255,255,0.3); border-top-color: white; border-radius: 50%; animation: spin 0.6s linear infinite; display: inline-block; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>

<script lang="ts">
  let title = $state('');
  let description = $state('');
  let category = $state('system');
  let tags = $state('');
  let version = $state('v1.0');
  let versionCode = $state('1.0.0');
  let changelog = $state('');
  let license = $state('MIT');
  let publishing = $state(false);
  let published = $state(false);
  let dependencies = $state<{id: string; min_version: string; optional: boolean}[]>([]);
  let depSearch = $state('');
  let searchResults = $state<any[]>([]);
  let searchingDeps = $state(false);
  let coverImage = $state('');
  let coverPreview = $state('');
  let screenshots = $state<{url: string; file?: File; preview: string; caption: string}[]>([]);
  let dragIdx = $state<number | null>(null);
  let dropIdx = $state<number | null>(null);

  function handleDragStart(e: DragEvent, i: number) {
    dragIdx = i;
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
  }
  function handleDragOver(e: DragEvent, i: number) {
    e.preventDefault();
    dropIdx = i;
  }
  function handleDrop(e: DragEvent) {
    e.preventDefault();
    if (dragIdx === null || dropIdx === null || dragIdx === dropIdx) { dragIdx = null; dropIdx = null; return; }
    const arr = [...screenshots];
    const [removed] = arr.splice(dragIdx, 1);
    arr.splice(dropIdx, 0, removed);
    screenshots = arr;
    dragIdx = null; dropIdx = null;
  }

  function handleCoverUpload(e: Event) {
    const input = e.target as HTMLInputElement;
    if (input.files && input.files[0]) {
      const reader = new FileReader();
      reader.onload = () => {
        coverPreview = reader.result as string;
        coverImage = reader.result as string;
      };
      reader.readAsDataURL(input.files[0]);
    }
  }

  function handleScreenshotUpload(e: Event) {
    const input = e.target as HTMLInputElement;
    if (!input.files) return;
    const remaining = 5 - screenshots.length;
    for (let i = 0; i < Math.min(input.files.length, remaining); i++) {
      const file = input.files[i];
      const reader = new FileReader();
      reader.onload = () => {
        screenshots = [...screenshots, { url: '', file, preview: reader.result as string, caption: '' }];
      };
      reader.readAsDataURL(file);
    }
  }

  function removeScreenshot(idx: number) {
    screenshots = screenshots.filter((_, i) => i !== idx);
  }

  async function searchModules(q: string) {
    if (!q.trim()) { searchResults = []; return; }
    searchingDeps = true;
    try {
      const res = await fetch(`/api/v1/market/modules?query=${encodeURIComponent(q)}&per_page=5`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        const data = await res.json();
        searchResults = (data.modules || []).filter((m: any) => !dependencies.some(d => d.id === m.slug));
      }
    } catch {}
    searchingDeps = false;
  }

  function addDep(slug: string) {
    if (dependencies.some(d => d.id === slug)) return;
    dependencies = [...dependencies, { id: slug, min_version: '', optional: false }];
    depSearch = '';
    searchResults = [];
  }

  function removeDep(idx: number) {
    dependencies = dependencies.filter((_, i) => i !== idx);
  }

  const categoryOptions = [
    { value: 'system', label: '系统', icon: 'phone_android' },
    { value: 'ui', label: '界面', icon: 'palette' },
    { value: 'audio', label: '音频', icon: 'headphones' },
    { value: 'display', label: '显示', icon: 'brightness_6' },
    { value: 'utility', label: '工具', icon: 'build' },
  ];
  const licenseOptions = ['MIT', 'Apache-2.0', 'GPL-3.0', 'CC-BY-4.0'];

  async function publish() {
    if (!title.trim() || !description.trim()) return;
    publishing = true;
    try {
      const res = await fetch('/api/v1/market/publish', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: title.trim(), description: description.trim(), category, tags: tags.trim(), version, version_code: versionCode, changelog: changelog.trim(), cover_image: coverImage, license, author: 'Anonymous', dependencies }),
      });
      if (res.ok) published = true;
    } catch {}
    publishing = false;
  }
</script>

<div class="p-6 max-w-2xl mx-auto">
  <!-- Back -->
  <a href="/market" class="inline-flex items-center gap-1.5 text-sm text-[var(--color-text-secondary)] hover:text-primary-600 transition-colors mb-6 no-underline">
    <span class="material-symbols-outlined text-[16px]">arrow_back</span>
    返回市场
  </a>

  {#if published}
    <div class="text-center py-16 animate-[scaleIn_0.3s_ease-out]">
      <div class="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4" style="background: var(--color-success-light)">
        <span class="material-symbols-outlined text-3xl" style="color: var(--color-success)">check_circle</span>
      </div>
      <h2 class="text-xl font-bold text-[var(--color-text)] mb-2">发布成功！</h2>
      <p class="text-sm text-[var(--color-text-secondary)] mb-6">你的模块已发布到 ModuForge 市场。</p>
      <a href="/market" class="btn-primary inline-flex items-center gap-2 no-underline">返回市场</a>
    </div>
  {:else}
    <h1 class="text-2xl font-bold text-[var(--color-text)] mb-6">发布模块</h1>

    <div class="space-y-5">
      <div>
        <label for="pub-title" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">标题 *</label>
        <input id="pub-title" type="text" class="input-field" placeholder="My Awesome Module" bind:value={title} />
      </div>

      <div>
        <label for="pub-desc" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">描述 *</label>
        <textarea id="pub-desc" class="input-field resize-none" rows="4" placeholder="详细描述你的模块功能..." bind:value={description}></textarea>
      </div>

      <div>
        <span class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">分类</span>
        <div class="grid grid-cols-5 gap-2">
          {#each categoryOptions as opt}
            <button
              class="flex flex-col items-center gap-1 p-3 rounded-xl border-2 transition-all duration-150 text-center
                {category === opt.value ? 'border-primary-500' : 'border-[var(--color-border)] hover:border-[var(--color-text-muted)]'}"
              style={category === opt.value ? 'background: var(--color-primary-light)' : ''}
              onclick={() => category = opt.value}
            >
              <span class="material-symbols-outlined text-[18px] {category === opt.value ? 'text-primary-600' : 'text-[var(--color-text-muted)]'}">{opt.icon}</span>
              <span class="text-xs font-medium {category === opt.value ? 'text-primary-700' : 'text-[var(--color-text-secondary)]'}">{opt.label}</span>
            </button>
          {/each}
        </div>
      </div>

      <div>
        <label for="pub-tags" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">标签</label>
        <input id="pub-tags" type="text" class="input-field" placeholder="tag1, tag2, tag3" bind:value={tags} />
        <p class="text-xs text-[var(--color-text-muted)] mt-1">用逗号分隔多个标签</p>
      </div>

      <div>
        <label for="pub-version" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">版本号（显示用）</label>
        <input id="pub-version" type="text" class="input-field" placeholder="v1.0" bind:value={version} />
      </div>

      <div>
        <label for="pub-version-code" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">版本代码（数字）</label>
        <input id="pub-version-code" type="text" class="input-field" placeholder="1.0.0" bind:value={versionCode} />
        <p class="text-xs text-[var(--color-text-muted)] mt-1">语义化版本号，用于版本比较和回滚</p>
      </div>

      <div>
        <label for="pub-changelog" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">更新日志</label>
        <textarea id="pub-changelog" class="input-field resize-none" rows="3" placeholder="描述本次更新的内容..." bind:value={changelog}></textarea>
      </div>

      <div>
        <label for="pub-cover-url" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">封面图</label>
        <div class="flex gap-3 items-start">
          <div class="flex-1">
            <input id="pub-cover-url" type="text" class="input-field mb-2" placeholder="封面图片 URL" bind:value={coverImage} oninput={() => coverPreview = coverImage} />
            <input type="file" accept="image/*" class="text-xs" onchange={handleCoverUpload} />
          </div>
          {#if coverPreview}
            <div class="w-24 h-24 rounded-xl overflow-hidden flex-shrink-0 border" style="border-color: var(--color-border)">
              <img src={coverPreview} alt="封面预览" class="w-full h-full object-cover" />
            </div>
          {/if}
        </div>
      </div>

      <div>
        <span class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">截图（最多 5 张，每张 ≤ 2MB）</span>
        <div class="flex flex-wrap gap-2 mb-2">
          {#each screenshots as ss, i}
            <div class="relative group" role="presentation"
                 draggable="true"
                 ondragstart={(e) => handleDragStart(e, i)}
                 ondragover={(e) => handleDragOver(e, i)}
                 ondrop={handleDrop}
                 class:opacity-50={dragIdx === i}
                 style="border: 2px solid {dropIdx === i ? 'var(--color-primary)' : 'transparent'}; border-radius: 0.75rem;">
              <div class="w-20 h-20 rounded-xl overflow-hidden border" style="border-color: var(--color-border)">
                <img src={ss.preview} alt="截图 {i+1}" class="w-full h-full object-cover" />
              </div>
              <button class="absolute top-0.5 right-0.5 w-5 h-5 rounded-full bg-black/60 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity"
                      onclick={() => removeScreenshot(i)}>
                <span class="material-symbols-outlined text-white text-[12px]">close</span>
              </button>
              <input type="text" class="mt-1 w-full text-[10px] px-1 py-0.5 rounded" style="background: var(--color-surface); color: var(--color-text-muted)" placeholder="说明..." bind:value={ss.caption} />
            </div>
          {/each}
          {#if screenshots.length < 5}
            <label class="w-20 h-20 rounded-xl border-2 border-dashed flex items-center justify-center cursor-pointer hover:border-[var(--color-primary)] transition-colors" style="border-color: var(--color-border)"
                   ondragover={(e) => e.preventDefault()}
                   ondrop={(e) => { e.preventDefault(); const files = e.dataTransfer?.files; if (files) { const input = document.createElement('input'); input.type = 'file'; input.accept = 'image/*'; input.multiple = true; input.onchange = (ev: any) => { const ft = ev.target as HTMLInputElement; if (ft.files) handleScreenshotUpload({ target: ft } as any); }; input.click(); } }}>
              <span class="material-symbols-outlined text-[24px]" style="color: var(--color-text-muted)">add</span>
              <input type="file" accept="image/*" multiple class="hidden" onchange={handleScreenshotUpload} />
            </label>
          {/if}
        </div>
        <p class="text-xs text-[var(--color-text-muted)]">拖拽排序 · 发布后可在模块详情页继续上传截图</p>
      </div>

      <div>
        <label for="pub-license" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">License</label>
        <select id="pub-license" class="input-field" bind:value={license}>
          {#each licenseOptions as opt}<option value={opt}>{opt}</option>{/each}
        </select>
      </div>

      <div>
        <label for="pub-dep-search" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">依赖模块</label>
        <div class="relative mb-2">
          <input id="pub-dep-search" type="text" class="input-field" placeholder="搜索模块添加依赖..." bind:value={depSearch} oninput={() => searchModules(depSearch)} />
          {#if searchResults.length > 0}
            <div class="absolute z-10 left-0 right-0 top-full mt-1 rounded-xl border" style="background: var(--color-surface); border-color: var(--color-border); box-shadow: 0 4px 16px rgba(0,0,0,0.15)">
              {#each searchResults as mod}
                <button class="w-full flex items-center gap-2 px-4 py-2.5 text-sm hover:bg-[var(--color-bg)] transition-colors text-left" onclick={() => addDep(mod.slug)}>
                  <span class="material-symbols-outlined text-[16px] text-[var(--color-text-muted)]">add_circle</span>
                  <span class="font-medium text-[var(--color-text)]">{mod.title || mod.slug}</span>
                  <span class="text-xs text-[var(--color-text-muted)]">v{mod.version}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
        {#if dependencies.length > 0}
          <div class="space-y-1.5">
            {#each dependencies as dep, i}
              <div class="flex items-center gap-2 p-2 rounded-xl" style="background: var(--color-surface)">
                <span class="text-sm font-medium text-[var(--color-text)] flex-1">{dep.id}</span>
                <input type="text" class="w-24 text-xs px-2 py-1 rounded-lg border" style="border-color: var(--color-border)" placeholder="最低版本" bind:value={dep.min_version} />
                <label class="flex items-center gap-1 text-xs text-[var(--color-text-secondary)] cursor-pointer">
                  <input type="checkbox" bind:checked={dep.optional} />可选
                </label>
                <button class="p-1 rounded-lg hover:bg-[var(--color-surface)]" onclick={() => removeDep(i)}>
                  <span class="material-symbols-outlined text-[16px]" style="color: var(--color-error)">remove_circle</span>
                </button>
              </div>
            {/each}
          </div>
        {:else}
          <p class="text-xs text-[var(--color-text-muted)]">（可选）添加本模块依赖的其他市场模块</p>
        {/if}
      </div>

      <div class="flex justify-end pt-2">
        <button
          class="btn-primary flex items-center gap-2 disabled:opacity-50"
          disabled={publishing || !title.trim() || !description.trim()}
          onclick={publish}
        >
          <span class="material-symbols-outlined text-[18px]">publish</span>
          {publishing ? '发布中...' : '发布模块'}
        </button>
      </div>
    </div>
  {/if}
</div>

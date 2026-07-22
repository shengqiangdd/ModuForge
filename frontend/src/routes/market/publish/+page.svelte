<script lang="ts">
  let title = $state('');
  let description = $state('');
  let category = $state('system');
  let tags = $state('');
  let version = $state('v1.0');
  let license = $state('MIT');
  let publishing = $state(false);
  let published = $state(false);

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
        body: JSON.stringify({ title: title.trim(), description: description.trim(), category, tags: tags.trim(), version, license, author: 'Anonymous' }),
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
      <div class="w-16 h-16 rounded-2xl bg-green-100 flex items-center justify-center mx-auto mb-4">
        <span class="material-symbols-outlined text-green-600 text-3xl">check_circle</span>
      </div>
      <h2 class="text-xl font-bold text-[var(--color-text)] mb-2">发布成功！</h2>
      <p class="text-sm text-[var(--color-text-secondary)] mb-6">你的模块已发布到 ModuForge 市场。</p>
      <a href="/market" class="btn-primary inline-flex items-center gap-2 no-underline">返回市场</a>
    </div>
  {:else}
    <h1 class="text-2xl font-bold text-[var(--color-text)] mb-6">发布模块</h1>

    <div class="space-y-5">
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">标题 *</label>
        <input type="text" class="input-field" placeholder="My Awesome Module" bind:value={title} />
      </div>

      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">描述 *</label>
        <textarea class="input-field resize-none" rows="4" placeholder="详细描述你的模块功能..." bind:value={description}></textarea>
      </div>

      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">分类</label>
        <div class="grid grid-cols-5 gap-2">
          {#each categoryOptions as opt}
            <button
              class="flex flex-col items-center gap-1 p-3 rounded-xl border-2 transition-all duration-150 text-center
                {category === opt.value ? 'border-primary-500 bg-primary-50' : 'border-[var(--color-border)] hover:border-neutral-300'}"
              onclick={() => category = opt.value}
            >
              <span class="material-symbols-outlined text-[18px] {category === opt.value ? 'text-primary-600' : 'text-[var(--color-text-muted)]'}">{opt.icon}</span>
              <span class="text-xs font-medium {category === opt.value ? 'text-primary-700' : 'text-[var(--color-text-secondary)]'}">{opt.label}</span>
            </button>
          {/each}
        </div>
      </div>

      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">标签</label>
        <input type="text" class="input-field" placeholder="tag1, tag2, tag3" bind:value={tags} />
        <p class="text-xs text-[var(--color-text-muted)] mt-1">用逗号分隔多个标签</p>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">版本号</label>
          <input type="text" class="input-field" placeholder="v1.0" bind:value={version} />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">License</label>
          <select class="input-field" bind:value={license}>
            {#each licenseOptions as opt}<option value={opt}>{opt}</option>{/each}
          </select>
        </div>
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

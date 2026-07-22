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
    { value: 'system', label: '系统 (System)' },
    { value: 'ui', label: '界面 (UI)' },
    { value: 'audio', label: '音频 (Audio)' },
    { value: 'display', label: '显示 (Display)' },
    { value: 'utility', label: '工具 (Utility)' },
  ];

  const licenseOptions = [
    { value: 'MIT', label: 'MIT' },
    { value: 'Apache-2.0', label: 'Apache-2.0' },
    { value: 'GPL-3.0', label: 'GPL-3.0' },
    { value: 'CC-BY-4.0', label: 'CC-BY-4.0' },
  ];

  async function publish() {
    if (!title.trim() || !description.trim()) return;
    publishing = true;
    try {
      const res = await fetch('/api/v1/market/publish', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: title.trim(),
          description: description.trim(),
          category,
          tags: tags.trim(),
          version,
          license,
          author: 'Anonymous',
        }),
      });
      if (res.ok) {
        published = true;
      }
    } catch {}
    publishing = false;
  }
</script>

<div class="p-6 max-w-2xl mx-auto">
  <div class="mb-6">
    <a href="/market" class="text-primary text-body-medium hover:underline flex items-center gap-1">
      <md-icon class="text-sm">arrow_back</md-icon>
      返回市场
    </a>
  </div>

  <h1 class="text-headline-large text-on-surface mb-6">发布模块</h1>

  {#if published}
    <div class="text-center py-12">
      <md-icon class="text-6xl text-green-500 mb-4">check_circle</md-icon>
      <h2 class="text-headline-small text-on-surface mb-2">发布成功！</h2>
      <p class="text-body-medium text-on-surface-variant mb-6">你的模块已发布到 ModuForge 市场。</p>
      <md-filled-button href="/market">返回市场</md-filled-button>
    </div>
  {:else}
    <div class="space-y-6">
      <!-- Title -->
      <div>
        <label class="block text-label-medium text-on-surface mb-2">标题 *</label>
        <input
          type="text"
          class="w-full px-4 py-3 border border-outline rounded-xl bg-surface text-on-surface text-body-large"
          placeholder="My Awesome Module"
          bind:value={title}
        />
      </div>

      <!-- Description -->
      <div>
        <label class="block text-label-medium text-on-surface mb-2">描述 *</label>
        <textarea
          class="w-full px-4 py-3 border border-outline rounded-xl bg-surface text-on-surface text-body-medium resize-none"
          rows="4"
          placeholder="详细描述你的模块功能..."
          bind:value={description}
        ></textarea>
      </div>

      <!-- Category -->
      <div>
        <label class="block text-label-medium text-on-surface mb-2">分类</label>
        <select
          class="w-full px-4 py-3 border border-outline rounded-xl bg-surface text-on-surface text-body-medium"
          bind:value={category}
        >
          {#each categoryOptions as opt}
            <option value={opt.value}>{opt.label}</option>
          {/each}
        </select>
      </div>

      <!-- Tags -->
      <div>
        <label class="block text-label-medium text-on-surface mb-2">标签</label>
        <input
          type="text"
          class="w-full px-4 py-3 border border-outline rounded-xl bg-surface text-on-surface text-body-medium"
          placeholder="tag1, tag2, tag3"
          bind:value={tags}
        />
        <p class="text-label-small text-on-surface-variant mt-1">用逗号分隔多个标签</p>
      </div>

      <!-- Version & License row -->
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-label-medium text-on-surface mb-2">版本号</label>
          <input
            type="text"
            class="w-full px-4 py-3 border border-outline rounded-xl bg-surface text-on-surface text-body-medium"
            placeholder="v1.0"
            bind:value={version}
          />
        </div>
        <div>
          <label class="block text-label-medium text-on-surface mb-2">License</label>
          <select
            class="w-full px-4 py-3 border border-outline rounded-xl bg-surface text-on-surface text-body-medium"
            bind:value={license}
          >
            {#each licenseOptions as opt}
              <option value={opt.value}>{opt.label}</option>
            {/each}
          </select>
        </div>
      </div>

      <!-- Submit -->
      <div class="flex justify-end pt-4">
        <md-filled-button onclick={publish} disabled={publishing || !title.trim() || !description.trim()}>
          <md-icon slot="icon">publish</md-icon>
          {publishing ? '发布中...' : '发布模块'}
        </md-filled-button>
      </div>
    </div>
  {/if}
</div>

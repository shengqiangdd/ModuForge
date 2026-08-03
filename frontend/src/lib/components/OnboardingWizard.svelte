<script lang="ts">
  import { client } from '$lib/api/client';
  import { toast } from '$lib/stores/toast.svelte';

  let { onDone }: { onDone?: () => void } = $props();

  let step = $state(0);
  let moduleType = $state('universal');
  let moduleName = $state('');
  let moduleDesc = $state('');
  let selectedTemplate = $state('');
  let templates = $state<any[]>([]);
  let creating = $state(false);

  const steps = [
    { icon: 'waving_hand', title: '欢迎使用 ModuForge', desc: '一站式 Magisk/KernelSU/APatch 模块开发平台，让模块开发变得简单高效。', gradient: 'from-violet-500 to-purple-600' },
    { icon: 'category', title: '选择模块类型', desc: '选择你要开发的模块类型。Universal 类型同时兼容 Magisk、KernelSU 和 APatch。', gradient: 'from-cyan-500 to-blue-600' },
    { icon: 'edit_note', title: '命名你的模块', desc: '为你的模块起一个简洁明了的名字，并添加描述。', gradient: 'from-emerald-500 to-green-600' },
    { icon: 'style', title: '选择模板', desc: '从预设模板快速开始，也可以跳过此步骤从空白项目开始。', gradient: 'from-amber-500 to-orange-600' },
    { icon: 'celebration', title: '一切就绪！', desc: '项目已创建成功，现在可以开始编写你的模块了。', gradient: 'from-pink-500 to-rose-600' },
  ];

  const moduleTypes = [
    { value: 'universal', label: 'Universal', desc: '兼容 Magisk + KSU + APatch', icon: 'hub', color: 'from-violet-500 to-purple-600' },
    { value: 'magisk', label: 'Magisk', desc: '仅 Magisk 框架', icon: 'extension', color: 'from-emerald-500 to-green-600' },
    { value: 'ksu', label: 'KernelSU', desc: '仅 KernelSU 框架', icon: 'security', color: 'from-cyan-500 to-blue-600' },
    { value: 'apatch', label: 'APatch', desc: '仅 APatch 框架', icon: 'shield', color: 'from-amber-500 to-orange-600' },
  ];

  $effect(() => {
    if (step === 3) {
      loadTemplates();
    }
  });

  async function loadTemplates() {
    try {
      const data = await client.get<any[]>('/templates');
      templates = data || [];
    } catch {
      templates = [];
    }
  }

  function next() {
    if (step === 1 && !moduleName.trim()) {
      toast('请输入模块名称', 'warning');
      return;
    }
    if (step < steps.length - 1) {
      step++;
    } else {
      createProject();
    }
  }

  function prev() {
    if (step > 0) step--;
  }

  function skip() {
    finish();
  }

  async function createProject() {
    if (!moduleName.trim()) {
      toast('请输入模块名称', 'warning');
      return;
    }
    creating = true;
    try {
      const project = await client.post<any>('/projects', {
        name: moduleName.trim(),
        module_type: moduleType,
        description: moduleDesc.trim(),
      });
      toast('项目创建成功！', 'success');
      finish();
      window.location.href = `/projects/${project.id}`;
    } catch (e: any) {
      toast(e.message || '创建失败', 'error');
    } finally {
      creating = false;
    }
  }

  function finish() {
    localStorage.setItem('moduforge_onboarded', 'true');
    onDone?.();
  }
</script>

<div class="fixed inset-0 z-[100] flex items-center justify-center p-4 animate-[fadeIn_0.3s_ease-out]"
     style="background: rgba(0,0,0,0.7); backdrop-filter: blur(12px)">
  <div class="w-full max-w-lg animate-[scaleIn_0.3s_ease-out]">
    <!-- Step indicator -->
    <div class="flex items-center justify-center gap-2 mb-8">
      {#each steps as _, i}
        <div class="h-1.5 rounded-full transition-all duration-300"
             style="width: {i === step ? '24px' : '8px'}; background: {i <= step ? 'var(--color-primary)' : 'var(--color-border)'}">
        </div>
      {/each}
    </div>

    <!-- Card -->
    <div class="rounded-3xl p-8 text-center border animate-[scaleIn_0.3s_ease-out]"
         style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: 0 32px 64px rgba(0,0,0,0.3)">
      <!-- Icon -->
      <div class="w-20 h-20 rounded-2xl flex items-center justify-center mx-auto mb-6"
           style="background: linear-gradient(135deg, {steps[step].gradient})">
        <span class="material-symbols-outlined text-white text-4xl">{steps[step].icon}</span>
      </div>

      <!-- Title -->
      <h2 class="text-2xl font-bold mb-3" style="color: var(--color-text)">{steps[step].title}</h2>

      <!-- Description -->
      <p class="text-sm leading-relaxed mb-6" style="color: var(--color-text-secondary)">{steps[step].desc}</p>

      <!-- Step-specific content -->
      {#if step === 1}
        <!-- Module Type Selection -->
        <div class="grid grid-cols-2 gap-3 mb-6">
          {#each moduleTypes as type}
            <button
              class="p-3 rounded-xl border-2 text-left transition-all duration-200"
              style="border-color: {moduleType === type.value ? 'var(--color-primary)' : 'var(--color-border)'}; background: {moduleType === type.value ? 'var(--color-primary-light)' : 'var(--color-surface)'}"
              onclick={() => moduleType = type.value}
            >
              <div class="flex items-center gap-2 mb-1">
                <span class="material-symbols-outlined text-lg" style="color: {moduleType === type.value ? 'var(--color-primary)' : 'var(--color-text-muted)'}">{type.icon}</span>
                <span class="text-sm font-semibold" style="color: var(--color-text)">{type.label}</span>
              </div>
              <p class="text-xs" style="color: var(--color-text-muted)">{type.desc}</p>
            </button>
          {/each}
        </div>
      {:else if step === 2}
        <!-- Module Name & Description -->
        <div class="space-y-4 text-left mb-6">
          <div>
            <label class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">模块名称 *</label>
            <input type="text" bind:value={moduleName} placeholder="例如：My Awesome Module"
                   class="input-field w-full" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">描述（可选）</label>
            <textarea bind:value={moduleDesc} placeholder="简要描述你的模块功能..." rows="3"
                      class="input-field w-full resize-none"></textarea>
          </div>
        </div>
      {:else if step === 3}
        <!-- Template Selection -->
        <div class="space-y-2 mb-6 max-h-48 overflow-y-auto text-left">
          <button
            class="w-full p-3 rounded-xl border text-left transition-all duration-200"
            style="border-color: {selectedTemplate === '' ? 'var(--color-primary)' : 'var(--color-border)'}; background: {selectedTemplate === '' ? 'var(--color-primary-light)' : 'var(--color-surface)'}"
            onclick={() => selectedTemplate = ''}
          >
            <div class="flex items-center gap-2">
              <span class="material-symbols-outlined text-lg" style="color: var(--color-primary)">add_circle_outline</span>
              <span class="text-sm font-semibold" style="color: var(--color-text)">空白项目</span>
            </div>
            <p class="text-xs mt-1" style="color: var(--color-text-muted)">从头开始创建模块</p>
          </button>
          {#each templates as tmpl}
            <button
              class="w-full p-3 rounded-xl border text-left transition-all duration-200"
              style="border-color: {selectedTemplate === tmpl.name ? 'var(--color-primary)' : 'var(--color-border)'}; background: {selectedTemplate === tmpl.name ? 'var(--color-primary-light)' : 'var(--color-surface)'}"
              onclick={() => selectedTemplate = tmpl.name}
            >
              <div class="flex items-center gap-2">
                <span class="material-symbols-outlined text-lg" style="color: {selectedTemplate === tmpl.name ? 'var(--color-primary)' : 'var(--color-text-muted)'}">description</span>
                <span class="text-sm font-semibold" style="color: var(--color-text)">{tmpl.name}</span>
              </div>
              <p class="text-xs mt-1" style="color: var(--color-text-muted)">{tmpl.description || '无描述'}</p>
            </button>
          {/each}
          {#if templates.length === 0}
            <p class="text-sm text-center py-4" style="color: var(--color-text-muted)">暂无可用模板</p>
          {/if}
        </div>
      {:else if step === 4}
        <!-- Completion -->
        <div class="space-y-3 mb-6 text-sm" style="color: var(--color-text-secondary)">
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-green-500 text-lg">check_circle</span>
            <span>模块类型: <strong style="color: var(--color-text)">{moduleTypes.find(t => t.value === moduleType)?.label}</strong></span>
          </div>
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-green-500 text-lg">check_circle</span>
            <span>模块名称: <strong style="color: var(--color-text)">{moduleName}</strong></span>
          </div>
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-green-500 text-lg">check_circle</span>
            <span>模板: <strong style="color: var(--color-text)">{selectedTemplate || '空白项目'}</strong></span>
          </div>
        </div>
      {/if}

      <!-- Step number -->
      <p class="text-xs mb-6" style="color: var(--color-text-muted)">
        第 {step + 1}/{steps.length} 步
      </p>

      <!-- Actions -->
      <div class="flex gap-3">
        {#if step > 0}
          <button class="flex-1 py-2.5 rounded-xl text-sm font-medium transition-colors"
                  style="border: 1px solid var(--color-border); color: var(--color-text-secondary)"
                  onclick={prev}>
            上一步
          </button>
        {:else}
          <button class="flex-1 py-2.5 rounded-xl text-sm font-medium transition-colors"
                  style="border: 1px solid var(--color-border); color: var(--color-text-secondary)"
                  onclick={skip}>
            跳过
          </button>
        {/if}
        <button class="flex-1 py-2.5 rounded-xl text-sm font-medium text-white transition-all hover:shadow-lg disabled:opacity-50"
                style="background: var(--gradient-brand)"
                onclick={next}
                disabled={creating || (step === 1 && !moduleName.trim())}>
          {#if creating}
            <span class="inline-flex items-center gap-2">
              <span class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></span>
              创建中...
            </span>
          {:else if step < steps.length - 1}
            下一步
          {:else}
            创建项目
          {/if}
        </button>
      </div>
    </div>
  </div>
</div>

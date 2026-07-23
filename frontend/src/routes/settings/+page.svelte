<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';

  let llmEndpoint = $state('https://api.openai.com/v1');
  let llmApiKey = $state('');
  let llmModel = $state('gpt-4o-mini');
  let llmProvider = $state('openai');
  let llmModelId = $state('');

  let providers = [
    { id: 'openai', name: 'OpenAI', models: 'gpt-4o, gpt-4o-mini, gpt-4-turbo' },
    { id: 'anthropic', name: 'Anthropic', models: 'claude-3.5-sonnet, claude-3-haiku' },
    { id: 'google', name: 'Google AI', models: 'gemini-1.5-pro, gemini-1.5-flash' },
    { id: 'deepseek', name: 'DeepSeek', models: 'deepseek-chat, deepseek-coder' },
    { id: 'qwen', name: '通义千问', models: 'qwen-turbo, qwen-plus, qwen-max' },
    { id: 'ollama', name: 'Ollama (本地)', models: 'llama3, qwen2, mistral' },
    { id: 'xai', name: 'xAI / Grok', models: 'grok-beta' },
  ];

  let theme = $state('light');
  let username = $state('');
  let email = $state('');

  let savingLLM = $state(false);
  let llmTesting = $state(false);

  onMount(async () => {
    const savedTheme = localStorage.getItem('moduforge_theme');
    if (savedTheme) theme = savedTheme;
    else theme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    applyTheme(theme);

    username = localStorage.getItem('moduforge_username') || '';
    email = localStorage.getItem('moduforge_email') || '';

    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch('/api/v1/llm/config', {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (res.ok) {
        const cfg = await res.json();
        llmEndpoint = cfg.endpoint || llmEndpoint;
        llmApiKey = cfg.api_key || '';
        llmModel = cfg.model || llmModel;
        llmProvider = cfg.provider || llmProvider;
        llmModelId = cfg.model_id || '';
      }
    } catch {}
  });

  function applyTheme(t: string) {
    document.documentElement.classList.toggle('dark', t === 'dark');
  }

  function toggleTheme() {
    theme = theme === 'light' ? 'dark' : 'light';
    applyTheme(theme);
    localStorage.setItem('moduforge_theme', theme);
    toast(`已切换到${theme === 'dark' ? '深色' : '浅色'}模式`, 'info');
  }

  async function saveLLMConfig() {
    savingLLM = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch('/api/v1/llm/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          endpoint: llmEndpoint,
          api_key: llmApiKey,
          model: llmModel,
          provider: llmProvider,
          model_id: llmModelId || llmModel,
        }),
      });
      if (res.ok) toast('LLM 配置保存成功', 'success');
      else toast((await res.json()).error || '保存失败', 'error');
    } catch {
      toast('保存失败，请检查网络', 'error');
    } finally {
      savingLLM = false;
    }
  }

  async function testLLM() {
    llmTesting = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch('/api/v1/ai/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ prompt: 'Say hello in one word.' }),
      });
      if (res.ok) {
        const data = await res.json();
        toast(`连接成功: ${data.response || data.message || 'OK'}`, 'success');
      } else {
        toast((await res.json()).error || '连接失败', 'error');
      }
    } catch {
      toast('连接测试失败', 'error');
    } finally {
      llmTesting = false;
    }
  }
</script>

<div class="p-6 max-w-3xl mx-auto space-y-8">
  <div>
    <h1 class="text-2xl font-bold text-[var(--color-text)]">设置</h1>
    <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">管理你的 ModuForge 配置</p>
  </div>

  <!-- Profile -->
  <section class="card p-6">
    <h2 class="text-base font-semibold text-[var(--color-text)] mb-4 flex items-center gap-2">
      <span class="material-symbols-outlined text-[20px] text-primary-500">person</span>
      个人信息
    </h2>
    <div class="space-y-4">
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">用户名</label>
        <input type="text" class="input-field" bind:value={username} disabled />
      </div>
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">邮箱</label>
        <input type="email" class="input-field" bind:value={email} disabled />
      </div>
    </div>
  </section>

  <!-- AI Provider Config -->
  <section class="card p-6">
    <h2 class="text-base font-semibold text-[var(--color-text)] mb-4 flex items-center gap-2">
      <span class="material-symbols-outlined text-[20px] text-primary-500">psychology</span>
      AI 提供商配置
    </h2>
    <div class="space-y-4">
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">提供商</label>
        <select class="input-field" bind:value={llmProvider}>
          {#each providers as p}
            <option value={p.id}>{p.name}</option>
          {/each}
        </select>
        <p class="text-xs text-[var(--color-text-muted)] mt-1">支持模型: {providers.find(p => p.id === llmProvider)?.models || '-'}</p>
      </div>
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Endpoint</label>
        <input type="text" class="input-field" bind:value={llmEndpoint} placeholder="https://api.openai.com/v1" />
      </div>
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Key</label>
        <input type="password" class="input-field" bind:value={llmApiKey} placeholder="sk-..." />
        <p class="text-xs text-[var(--color-text-muted)] mt-1">密钥仅存储在本地浏览器，不会上传到服务器</p>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">模型</label>
          <input type="text" class="input-field" bind:value={llmModel} placeholder="gpt-4o-mini" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">模型 ID (可选)</label>
          <input type="text" class="input-field" bind:value={llmModelId} placeholder="留空使用默认" />
        </div>
      </div>
      <div class="flex gap-3">
        <button class="btn-primary text-sm" onclick={saveLLMConfig} disabled={savingLLM}>
          {savingLLM ? '保存中...' : '保存配置'}
        </button>
        <button class="btn-ghost border border-[var(--color-border)] text-sm" onclick={testLLM} disabled={llmTesting}>
          {llmTesting ? '测试中...' : '测试连接'}
        </button>
      </div>
    </div>
  </section>

  <!-- Theme -->
  <section class="card p-6">
    <h2 class="text-base font-semibold text-[var(--color-text)] mb-4 flex items-center gap-2">
      <span class="material-symbols-outlined text-[20px] text-primary-500">palette</span>
      外观
    </h2>
    <div class="flex items-center justify-between">
      <div>
        <p class="text-sm font-medium text-[var(--color-text)]">主题模式</p>
        <p class="text-xs text-[var(--color-text-muted)]">当前: {theme === 'dark' ? '深色模式' : '浅色模式'}</p>
      </div>
      <button
        class="w-12 h-7 rounded-full transition-colors relative {theme === 'dark' ? 'bg-primary-600' : 'bg-neutral-300'}"
        onclick={toggleTheme}
      >
        <div class="w-5 h-5 rounded-full bg-white shadow-sm absolute top-1 transition-all {theme === 'dark' ? 'left-6' : 'left-1'} flex items-center justify-center">
          <span class="material-symbols-outlined text-[12px] {theme === 'dark' ? 'text-primary-600' : 'text-amber-500'}">
            {theme === 'dark' ? 'dark_mode' : 'light_mode'}
          </span>
        </div>
      </button>
    </div>
  </section>

  <!-- About -->
  <section class="card p-6">
    <h2 class="text-base font-semibold text-[var(--color-text)] mb-4 flex items-center gap-2">
      <span class="material-symbols-outlined text-[20px] text-primary-500">info</span>
      关于
    </h2>
    <div class="space-y-2 text-sm text-[var(--color-text-secondary)]">
      <div class="flex justify-between py-1"><span>版本</span><span class="font-medium text-[var(--color-text)]">2.0-lite</span></div>
      <div class="flex justify-between py-1"><span>前端框架</span><span class="font-medium text-[var(--color-text)]">Svelte 5 + UnoCSS</span></div>
      <div class="flex justify-between py-1"><span>后端框架</span><span class="font-medium text-[var(--color-text)]">Go + Fiber + SQLite</span></div>
    </div>
  </section>
</div>

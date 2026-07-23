<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';

  // ===== Profile =====
  let username = $state('');
  let email = $state('');

  // ===== Current Provider =====
  let currentProvider = $state('opencode-zen');
  let currentModelId = $state('');

  // ===== Preset Providers =====
  let presetProviders: any[] = [];
  let providerConfigs: Record<string, {endpoint: string, api_key: string}> = {};
  let configModalProvider = $state<any>(null);
  let configEndpoint = $state('');
  let configApiKey = $state('');

  // ===== Custom Providers =====
  let customProviders: any[] = [];
  let showCustomModal = $state(false);
  let editingCustom = $state<any>(null);
  let customForm = $state({ name: '', endpoint: '', api_key: '', models_json: '' });
  let deletingCustomId = $state('');

  // ===== Theme =====
  let theme = $state('light');

  // ===== Loading =====
  let loading = $state(true);

  function getToken() {
    return localStorage.getItem('moduforge_token') || '';
  }

  async function loadAll() {
    loading = true;
    const token = getToken();

    // Load current config
    try {
      const r = await fetch('/api/v1/llm/config', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const cfg = await r.json();
        currentProvider = cfg.provider || 'opencode-zen';
        currentModelId = cfg.model_id || '';
      }
    } catch {}

    // Load providers
    try {
      const r = await fetch('/api/v1/llm/providers', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        presetProviders = data.providers || [];
      }
    } catch {}

    // Load provider configs (DB overrides)
    try {
      const r = await fetch('/api/v1/llm/provider-configs', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        for (const c of data.configs || []) {
          providerConfigs[c.id] = { endpoint: c.endpoint, api_key: c.api_key };
        }
      }
    } catch {}

    // Load custom providers
    try {
      const r = await fetch('/api/v1/llm/custom-providers', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        customProviders = data.providers || [];
      }
    } catch {}

    loading = false;
  }

  onMount(async () => {
    const savedTheme = localStorage.getItem('moduforge_theme');
    if (savedTheme) theme = savedTheme;
    else theme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    applyTheme(theme);

    username = localStorage.getItem('moduforge_username') || '';
    email = localStorage.getItem('moduforge_email') || '';

    await loadAll();
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

  // ===== Provider Config =====
  function openConfigModal(p: any) {
    configModalProvider = p;
    const existing = providerConfigs[p.id];
    configEndpoint = existing?.endpoint || p.endpoint || '';
    configApiKey = existing?.api_key || '';
  }

  function closeConfigModal() {
    configModalProvider = null;
    configEndpoint = '';
    configApiKey = '';
  }

  async function saveProviderConfig() {
    if (!configModalProvider) return;
    const token = getToken();
    try {
      const r = await fetch('/api/v1/llm/provider-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          id: configModalProvider.id,
          endpoint: configEndpoint,
          api_key: configApiKey,
        }),
      });
      if (r.ok) {
        providerConfigs[configModalProvider.id] = { endpoint: configEndpoint, api_key: configApiKey };
        toast('配置已保存', 'success');
        closeConfigModal();
      } else {
        toast((await r.json()).error || '保存失败', 'error');
      }
    } catch {
      toast('保存失败', 'error');
    }
  }

  async function resetProviderConfig(providerId: string) {
    const token = getToken();
    try {
      const r = await fetch(`/api/v1/llm/provider-config/${providerId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (r.ok) {
        delete providerConfigs[providerId];
        providerConfigs = { ...providerConfigs };
        toast('已恢复默认配置', 'success');
      } else {
        toast((await r.json()).error || '重置失败', 'error');
      }
    } catch {
      toast('重置失败', 'error');
    }
  }

  // ===== Custom Provider CRUD =====
  function openNewCustomModal() {
    editingCustom = null;
    customForm = { name: '', endpoint: '', api_key: '', models_json: '' };
    showCustomModal = true;
  }

  function openEditCustomModal(p: any) {
    editingCustom = p;
    customForm = {
      name: p.name,
      endpoint: p.endpoint,
      api_key: p.api_key || '',
      models_json: p.models_json || '',
    };
    showCustomModal = true;
  }

  function closeCustomModal() {
    showCustomModal = false;
    editingCustom = null;
  }

  async function saveCustomProvider() {
    const token = getToken();
    try {
      if (editingCustom) {
        const r = await fetch(`/api/v1/llm/custom-providers/${editingCustom.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify(customForm),
        });
        if (r.ok) {
          toast('已更新', 'success');
        } else {
          toast((await r.json()).error || '更新失败', 'error');
          return;
        }
      } else {
        const r = await fetch('/api/v1/llm/custom-providers', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify(customForm),
        });
        if (r.ok) {
          toast('已添加', 'success');
        } else {
          toast((await r.json()).error || '添加失败', 'error');
          return;
        }
      }
      closeCustomModal();
      await loadAll();
    } catch {
      toast('操作失败', 'error');
    }
  }

  async function deleteCustomProvider(id: string) {
    deletingCustomId = id;
    const token = getToken();
    try {
      const r = await fetch(`/api/v1/llm/custom-providers/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (r.ok) {
        toast('已删除', 'success');
        customProviders = customProviders.filter(p => p.id !== id);
      } else {
        toast((await r.json()).error || '删除失败', 'error');
      }
    } catch {
      toast('删除失败', 'error');
    }
    deletingCustomId = '';
  }

  function isPresetProvider(id: string) {
    return !id.startsWith('custom-') && presetProviders.some(p => p.id === id);
  }

  function getProvider(id: string) {
    return presetProviders.find(p => p.id === id) || customProviders.find(p => p.id === id);
  }

  function providerStatus(p: any) {
    if (p.id === currentProvider) return 'current';
    if (p.id === 'opencode-zen' || p.id === 'opencode-go') return 'builtin';
    const cfg = providerConfigs[p.id];
    if (cfg?.api_key) return 'configured';
    if (p.requires_key) return 'needs_key';
    return 'ready';
  }

  function statusBadgeClass(status: string) {
    switch (status) {
      case 'current': return 'bg-primary/20 text-primary';
      case 'builtin': return 'bg-cyan-500/20 text-cyan-400';
      case 'configured': return 'bg-green-500/20 text-green-400';
      case 'needs_key': return 'bg-amber-500/20 text-amber-400';
      case 'ready': return 'bg-zinc-500/20 text-zinc-400';
      default: return 'bg-zinc-500/20 text-zinc-400';
    }
  }

  function statusLabel(status: string) {
    switch (status) {
      case 'current': return '使用中';
      case 'builtin': return '免费';
      case 'configured': return '已配置';
      case 'needs_key': return '需配置';
      case 'ready': return '就绪';
      default: return '-';
    }
  }
</script>

<div class="p-6 max-w-4xl mx-auto space-y-8">
  <div>
    <h1 class="text-2xl font-bold text-[var(--color-text)]">设置</h1>
    <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">管理你的 ModuForge 配置</p>
  </div>

  <!-- Profile -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-1">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--gradient-brand-subtle)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">person</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">个人信息</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">你的账户基本信息</p>
      </div>
    </div>
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

  <!-- Current Provider Info -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: linear-gradient(135deg, rgba(6,182,212,0.15), rgba(139,92,246,0.15))">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">psychology</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">LLM 提供商管理</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">配置 AI 提供商和模型</p>
      </div>
    </div>

    <!-- Current selection -->
    <div class="flex items-center gap-3 mb-5 p-3 rounded-xl" style="background: var(--gradient-brand-subtle); border: 1px solid var(--color-border);">
      <span class="material-symbols-outlined text-[var(--color-primary)] text-lg">check_circle</span>
      <div class="flex-1 min-w-0">
        <p class="text-sm font-medium text-[var(--color-text)] truncate">
          当前: {getProvider(currentProvider)?.name || currentProvider}
          {#if currentModelId}
            <span class="text-[var(--color-text-muted)]">/ {currentModelId}</span>
          {/if}
        </p>
      </div>
    </div>
  </section>

  <!-- Preset Providers -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">cloud</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">预设提供商</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">内置提供商，可自定义 Endpoint 和 API Key</p>
      </div>
    </div>

    {#if loading}
      <div class="space-y-2">
        {#each Array(5) as _}
          <div class="skeleton h-12 w-full rounded-xl"></div>
        {/each}
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-[var(--color-text-muted)]">
              <th class="pb-3 pr-4 font-medium">名称</th>
              <th class="pb-3 pr-4 font-medium">模型数</th>
              <th class="pb-3 pr-4 font-medium">状态</th>
              <th class="pb-3 pr-4 font-medium">Endpoint</th>
              <th class="pb-3 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each presetProviders as p}
              <tr class="border-t border-[var(--color-border)]">
                <td class="py-3 pr-4">
                  <span class="font-medium text-[var(--color-text)]">{p.name}</span>
                </td>
                <td class="py-3 pr-4 text-[var(--color-text-secondary)]">{p.models?.length || 0}</td>
                <td class="py-3 pr-4">
                  <span class="badge text-xs {statusBadgeClass(providerStatus(p))}">
                    {statusLabel(providerStatus(p))}
                  </span>
                </td>
                <td class="py-3 pr-4 max-w-[200px] truncate text-[var(--color-text-muted)] text-xs" title={p.endpoint}>
                  {providerConfigs[p.id]?.endpoint || p.endpoint || '-'}
                </td>
                <td class="py-3 text-right">
                  <div class="flex items-center justify-end gap-1">
                    <button class="btn-ghost text-xs px-2.5 py-1.5 min-h-0" onclick={() => openConfigModal(p)}>
                      配置
                    </button>
                    {#if providerConfigs[p.id]}
                      <button class="btn-ghost text-xs px-2.5 py-1.5 min-h-0 text-[var(--color-error)]" onclick={() => resetProviderConfig(p.id)}>
                        重置
                      </button>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>

  <!-- Custom Providers -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-success-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-success)">add_box</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">自定义提供商</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">添加 Open AI 兼容的自定义提供商</p>
      </div>
      <button class="btn-primary text-sm" onclick={openNewCustomModal}>
        <span class="material-symbols-outlined text-[16px]">add</span>
        添加
      </button>
    </div>

    {#if customProviders.length === 0}
      <p class="text-sm text-[var(--color-text-muted)] text-center py-6">暂无自定义提供商</p>
    {:else}
      <div class="space-y-2">
        {#each customProviders as cp}
          <div class="flex items-center gap-3 p-3 rounded-xl" style="border: 1px solid var(--color-border);">
            <span class="material-symbols-outlined text-[var(--color-text-muted)]">dns</span>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-[var(--color-text)]">{cp.name}</p>
              <p class="text-xs text-[var(--color-text-muted)] truncate">{cp.endpoint}</p>
            </div>
            <span class="badge text-xs {cp.id === currentProvider ? 'bg-primary/20 text-primary' : 'bg-zinc-500/20 text-zinc-400'}">
              {cp.id === currentProvider ? '使用中' : cp.api_key ? '已配置' : '无 Key'}
            </span>
            <div class="flex items-center gap-1">
              <button class="btn-ghost text-xs px-2.5 py-1.5 min-h-0" onclick={() => openEditCustomModal(cp)}>编辑</button>
              <button class="btn-ghost text-xs px-2.5 py-1.5 min-h-0 text-[var(--color-error)]" onclick={() => deleteCustomProvider(cp.id)} disabled={deletingCustomId === cp.id}>
                {deletingCustomId === cp.id ? '删除中...' : '删除'}
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </section>

  <!-- Theme -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: linear-gradient(135deg, rgba(249,115,22,0.15), rgba(168,85,247,0.15))">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-warning)">palette</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">外观</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">自定义界面主题和显示效果</p>
      </div>
    </div>
    <div class="flex items-center justify-between">
      <div>
        <p class="text-sm font-medium text-[var(--color-text)]">主题模式</p>
        <p class="text-xs" style="color: var(--color-text-muted)">当前: {theme === 'dark' ? '深色模式' : '浅色模式'}</p>
      </div>
      <button
        class="theme-toggle w-14 h-8 rounded-full transition-all duration-300 relative {theme === 'dark' ? 'active' : ''}"
        onclick={toggleTheme}
        aria-label="切换主题"
      >
        <div class="theme-toggle-thumb w-6 h-6 rounded-full bg-white shadow-md absolute top-1 transition-all duration-300 {theme === 'dark' ? 'left-7' : 'left-1'} flex items-center justify-center">
          <span class="material-symbols-outlined text-[14px] transition-colors duration-300 {theme === 'dark' ? 'text-violet-600' : 'text-amber-500'}">
            {theme === 'dark' ? 'dark_mode' : 'light_mode'}
          </span>
        </div>
      </button>
    </div>
  </section>

  <!-- About -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-success-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-success)">info</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">关于</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">ModuForge 版本和系统信息</p>
      </div>
    </div>
    <div class="space-y-2 text-sm text-[var(--color-text-secondary)]">
      <div class="flex justify-between py-1"><span>版本</span><span class="font-medium text-[var(--color-text)]">2.0-lite</span></div>
      <div class="flex justify-between py-1"><span>前端框架</span><span class="font-medium text-[var(--color-text)]">Svelte 5 + UnoCSS</span></div>
      <div class="flex justify-between py-1"><span>后端框架</span><span class="font-medium text-[var(--color-text)]">Go + Fiber + SQLite</span></div>
    </div>
  </section>
</div>

<!-- Provider Config Modal -->
{#if configModalProvider}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" onclick={closeConfigModal}>
    <div class="card p-6 w-full max-w-md" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="flex items-center gap-3 mb-5">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center" style="background: var(--gradient-brand-subtle)">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-primary)">settings</span>
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--color-text)]">配置 {configModalProvider.name}</h3>
          <p class="text-xs text-[var(--color-text-muted)]">自定义 Endpoint 和 API Key</p>
        </div>
      </div>
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Endpoint</label>
          <input type="text" class="input-field" bind:value={configEndpoint} placeholder="https://api.openai.com/v1/chat/completions" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Key</label>
          <input type="password" class="input-field" bind:value={configApiKey} placeholder="sk-..." />
          <p class="text-xs text-[var(--color-text-muted)] mt-1">密钥加密存储在服务器</p>
        </div>
      </div>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button class="btn-ghost text-sm" onclick={closeConfigModal}>取消</button>
        <button class="btn-primary text-sm" onclick={saveProviderConfig}>保存</button>
      </div>
    </div>
  </div>
{/if}

<!-- Custom Provider Modal -->
{#if showCustomModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" onclick={closeCustomModal}>
    <div class="card p-6 w-full max-w-md" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="flex items-center gap-3 mb-5">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center" style="background: var(--color-success-light)">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-success)">dns</span>
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--color-text)]">{editingCustom ? '编辑' : '添加'}自定义提供商</h3>
          <p class="text-xs text-[var(--color-text-muted)]">OpenAI 兼容的提供商</p>
        </div>
      </div>
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">名称</label>
          <input type="text" class="input-field" bind:value={customForm.name} placeholder="My Provider" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Endpoint</label>
          <input type="text" class="input-field" bind:value={customForm.endpoint} placeholder="https://api.example.com/v1/chat/completions" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Key</label>
          <input type="password" class="input-field" bind:value={customForm.api_key} placeholder="sk-..." />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">模型列表 (JSON 数组)</label>
            <textarea class="input-field resize-none" rows="4" bind:value={customForm.models_json} placeholder={`[{"id":"my-model","name":"My Model","max_tokens":32000}]`}></textarea>
          <p class="text-xs text-[var(--color-text-muted)] mt-1">JSON 数组格式，每个模型需包含 id、name、max_tokens 字段</p>
        </div>
      </div>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button class="btn-ghost text-sm" onclick={closeCustomModal}>取消</button>
        <button class="btn-primary text-sm" onclick={saveCustomProvider} disabled={!customForm.name || !customForm.endpoint}>
          {editingCustom ? '更新' : '添加'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .theme-toggle {
    background: var(--color-surface);
    border: 2px solid var(--color-border);
  }
  .theme-toggle.active {
    background: linear-gradient(135deg, #8b5cf6 0%, #06b6d4 100%);
    border-color: transparent;
    box-shadow: 0 0 16px rgba(139,92,246,0.3);
  }
  .theme-toggle:hover {
    transform: scale(1.05);
  }
  .theme-toggle:active {
    transform: scale(0.98);
  }
  .theme-toggle-thumb {
    transition: all 0.3s cubic-bezier(0.68, -0.55, 0.265, 1.55);
  }
  table {
    border-collapse: collapse;
  }
</style>

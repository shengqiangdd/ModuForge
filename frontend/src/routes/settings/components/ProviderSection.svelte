<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';
  import ProviderCard from './ProviderCard.svelte';
  import ModelListModal from './ModelListModal.svelte';
  import ProviderStats from './ProviderStats.svelte';

  let { onConfigChange }: { onConfigChange?: () => void } = $props();

  interface Provider {
    id: string;
    name: string;
    endpoint: string;
    models: { id: string; name: string; max_tokens: number }[];
    requires_key: boolean;
    is_free: boolean;
    tier: string;
    models_json?: string;
    api_key?: string;
  }

  let currentProvider = $state('opencode-zen');
  let currentModelId = $state('');
  let presetProviders: Provider[] = $state([]);
  let providerConfigs = $state<Record<string, { endpoint: string; api_key: string; models_json?: string }>>({});
  let customProviders: Provider[] = $state([]);
  let userModelsMap = $state<Record<string, Array<{ id: string; name: string }>>>({});

  // Config modal
  let configModalProvider: { id: string; name: string; endpoint: string } | null = $state(null);
  let configEndpoint = $state('');
  let configApiKey = $state('');

  // Models modal
  let showModelsModal = $state(false);
  let modelsModalProvider: Provider | null = $state(null);

  // Custom provider modal
  let showCustomModal = $state(false);
  let editingCustom: Provider | null = $state(null);
  let customForm = $state({ name: '', endpoint: '', api_key: '', models: [] as { id: string; name: string; max_tokens: number }[] });
  let deletingCustomId = $state('');

  // All providers modal
  let showAllProvidersModal = $state(false);

  const FEATURED_IDS = ['opencode-zen', 'opencode-go', 'openai', 'anthropic', 'google', 'deepseek'];
  let featuredProviders = $derived(presetProviders.filter((p: Provider) => FEATURED_IDS.includes(p.id)));

  function providerStatus(p: Provider) {
    if (p.id === currentProvider) return 'current';
    if (p.tier === 'free' || p.is_free) return 'free';
    if (p.tier === 'subscription') return 'subscription';
    const cfg = providerConfigs[p.id];
    if (cfg?.api_key) return 'configured';
    if (p.requires_key) return 'needs_key';
    return 'ready';
  }

  function badgeClass(status: string) {
    switch (status) {
      case 'current': return 'bg-primary/20 text-primary';
      case 'free': return 'bg-green-500/20 text-green-400';
      case 'subscription': return 'bg-violet-500/20 text-violet-400';
      case 'configured': return 'bg-green-500/20 text-green-400';
      case 'needs_key': return 'bg-amber-500/20 text-amber-400';
      default: return 'bg-zinc-500/20 text-zinc-400';
    }
  }

  function badgeLabel(status: string) {
    switch (status) {
      case 'current': return '使用中';
      case 'free': return '免费';
      case 'subscription': return '订阅';
      case 'configured': return '已配置';
      case 'needs_key': return '需配置';
      case 'ready': return '就绪';
      default: return '-';
    }
  }

  async function loadProviders() {
    const token = getToken();
    try {
      const r = await fetch('/api/v1/llm/config', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const cfg = await r.json();
        currentProvider = cfg.provider || 'opencode-zen';
        currentModelId = cfg.model_id || '';
      }
    } catch {}
    try {
      const r = await fetch('/api/v1/llm/providers', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        presetProviders = data.providers || [];
      }
    } catch {}
    try {
      const r = await fetch('/api/v1/llm/provider-configs', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        for (const c of data.configs || []) {
          providerConfigs[c.id] = { endpoint: c.endpoint, api_key: c.api_key, models_json: c.models_json };
          if (c.models_json) {
            try { userModelsMap[c.id] = JSON.parse(c.models_json); } catch {}
          }
        }
      }
    } catch {}
    try {
      const r = await fetch('/api/v1/llm/custom-providers', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        customProviders = data.providers || [];
      }
    } catch {}
  }

  function openConfigModal(p: Provider) {
    configModalProvider = { id: p.id, name: p.name, endpoint: p.endpoint };
    configEndpoint = providerConfigs[p.id]?.endpoint || p.endpoint || '';
    configApiKey = providerConfigs[p.id]?.api_key || '';
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
          models_json: providerConfigs[configModalProvider.id]?.models_json || '',
        }),
      });
      if (r.ok) {
        providerConfigs[configModalProvider.id] = { endpoint: configEndpoint, api_key: configApiKey, models_json: providerConfigs[configModalProvider.id]?.models_json || '' };
        providerConfigs = { ...providerConfigs };
        toast('配置已保存', 'success');
        configModalProvider = null;
        onConfigChange?.();
      } else {
        toast((await r.json()).error || '保存失败', 'error');
      }
    } catch { toast('保存失败', 'error'); }
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
        delete userModelsMap[providerId];
        userModelsMap = { ...userModelsMap };
        toast('已恢复默认配置', 'success');
      }
    } catch { toast('重置失败', 'error'); }
  }

  function openModelsModal(p: Provider) {
    modelsModalProvider = p;
    showModelsModal = true;
  }

  // Custom provider CRUD
  function openNewCustomModal() {
    editingCustom = null;
    customForm = { name: '', endpoint: '', api_key: '', models: [] };
    showCustomModal = true;
  }

  function openEditCustomModal(p: Provider) {
    editingCustom = p;
    let parsedModels: { id: string; name: string; max_tokens: number }[] = [];
    try { parsedModels = JSON.parse(p.models_json || '[]'); } catch {}
    customForm = { name: p.name, endpoint: p.endpoint, api_key: p.api_key || '', models: parsedModels };
    showCustomModal = true;
  }

  function addCustomModel() {
    customForm.models = [...customForm.models, { id: '', name: '', max_tokens: 32000 }];
  }

  function removeCustomModel(index: number) {
    customForm.models = customForm.models.filter((_, i) => i !== index);
  }

  async function saveCustomProvider() {
    const token = getToken();
    const payload = { ...customForm, models_json: JSON.stringify(customForm.models) };
    try {
      if (editingCustom) {
        const r = await fetch(`/api/v1/llm/custom-providers/${editingCustom.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify(payload),
        });
        if (r.ok) { toast('已更新', 'success'); } else { toast((await r.json()).error || '更新失败', 'error'); return; }
      } else {
        const r = await fetch('/api/v1/llm/custom-providers', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify(payload),
        });
        if (r.ok) { toast('已添加', 'success'); } else { toast((await r.json()).error || '添加失败', 'error'); return; }
      }
      showCustomModal = false;
      editingCustom = null;
      await loadProviders();
    } catch { toast('操作失败', 'error'); }
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
    } catch { toast('删除失败', 'error'); }
    deletingCustomId = '';
  }

  onMount(() => { loadProviders(); });
</script>

<ProviderStats {currentProvider} {currentModelId} {presetProviders} {customProviders} {userModelsMap} />

<!-- Preset Providers Table -->
<div class="providers-section">
  <div class="section-header">
    <h3>预设提供商</h3>
    <p class="text-xs text-[var(--color-text-muted)]">内置提供商，可自定义 Endpoint 和 API Key</p>
  </div>
  <div class="overflow-x-auto -mx-5 sm:mx-0 px-5 sm:px-0">
    <table class="provider-table w-full text-sm">
      <thead>
        <tr class="text-left text-[var(--color-text-muted)]">
          <th class="pb-3 pr-4 font-medium">名称</th>
          <th class="pb-3 pr-4 font-medium hidden sm:table-cell">模型数</th>
          <th class="pb-3 pr-4 font-medium">状态</th>
          <th class="pb-3 pr-4 font-medium hidden md:table-cell">Endpoint</th>
          <th class="pb-3 text-right font-medium">操作</th>
        </tr>
      </thead>
      <tbody>
        {#each featuredProviders as p}
          <ProviderCard
            provider={p}
            isCurrent={p.id === currentProvider}
            config={providerConfigs[p.id]}
            userModels={userModelsMap[p.id] || []}
            status={providerStatus(p)}
            onOpenModels={() => openModelsModal(p)}
            onOpenConfig={() => openConfigModal(p)}
            onResetConfig={() => resetProviderConfig(p.id)}
          />
        {/each}
        {#if presetProviders.length > FEATURED_IDS.length}
          <tr class="border-t border-[var(--color-border)]">
            <td colspan="5" class="py-3 text-center">
              <button class="text-sm text-[var(--color-primary)] hover:underline inline-flex items-center gap-1.5" onclick={() => showAllProvidersModal = true}>
                <span class="material-symbols-outlined text-[16px]">unfold_more</span>
                更多供应商 ({presetProviders.length - FEATURED_IDS.length})
              </button>
            </td>
          </tr>
        {/if}
      </tbody>
    </table>
  </div>
</div>

<!-- Custom Providers -->
<div class="providers-section">
  <div class="section-header">
    <div class="flex-1">
      <h3>自定义提供商</h3>
      <p class="text-xs text-[var(--color-text-muted)]">添加 Open AI 兼容的自定义提供商</p>
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
        <div class="flex items-center gap-2 sm:gap-3 p-2.5 sm:p-3 rounded-xl" style="border: 1px solid var(--color-border);">
          <span class="material-symbols-outlined text-[var(--color-text-muted)] hidden sm:block">dns</span>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-[var(--color-text)]">{cp.name}</p>
            <p class="text-xs text-[var(--color-text-muted)] truncate">{cp.endpoint}</p>
          </div>
          <span class="badge text-xs whitespace-nowrap {cp.id === currentProvider ? 'bg-primary/20 text-primary' : 'bg-zinc-500/20 text-zinc-400'}">
            {cp.id === currentProvider ? '使用中' : cp.api_key ? '已配置' : '无 Key'}
          </span>
          <div class="flex items-center gap-0.5 sm:gap-1">
            <button class="btn-ghost text-xs px-1.5 sm:px-2.5 py-1.5 min-h-0" onclick={() => openEditCustomModal(cp)}>编辑</button>
            <button class="btn-ghost text-xs px-1.5 sm:px-2.5 py-1.5 min-h-0 text-[var(--color-error)]" onclick={() => deleteCustomProvider(cp.id)} disabled={deletingCustomId === cp.id}>
              {deletingCustomId === cp.id ? '...' : '删除'}
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Config Modal -->
{#if configModalProvider}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) configModalProvider = null; }}>
    <div class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">配置 {configModalProvider.name}</h3>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => configModalProvider = null}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-4">
        <div>
          <label for="config-endpoint" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">Endpoint</label>
          <input id="config-endpoint" type="text" class="input-field w-full" bind:value={configEndpoint} placeholder="API endpoint" />
        </div>
        <div>
          <label for="config-apikey" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Key</label>
          <input id="config-apikey" type="password" class="input-field w-full" bind:value={configApiKey} placeholder="sk-..." />
        </div>
      </div>
      <div class="p-5 border-t flex justify-end gap-3" style="border-color: var(--color-border)">
        <button class="btn-ghost text-sm" onclick={() => configModalProvider = null}>取消</button>
        <button class="btn-primary text-sm" onclick={saveProviderConfig}>保存</button>
      </div>
    </div>
  </div>
{/if}

<!-- Models Modal (delegated) -->
<ModelListModal
  show={showModelsModal}
  provider={modelsModalProvider}
  {userModelsMap}
  {providerConfigs}
  onClose={() => { showModelsModal = false; modelsModalProvider = null; }}
  onModelsChanged={() => { loadProviders(); }}
/>

<!-- Custom Provider Modal -->
{#if showCustomModal}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) { showCustomModal = false; editingCustom = null; } }}>
    <div class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">{editingCustom ? '编辑' : '添加'}自定义提供商</h3>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => { showCustomModal = false; editingCustom = null; }}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-4">
        <div>
          <label for="custom-name" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">名称</label>
          <input id="custom-name" type="text" class="input-field w-full" bind:value={customForm.name} placeholder="My Provider" />
        </div>
        <div>
          <label for="custom-endpoint" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">Endpoint</label>
          <input id="custom-endpoint" type="text" class="input-field w-full" bind:value={customForm.endpoint} placeholder="https://api.example.com/v1" />
        </div>
        <div>
          <label for="custom-apikey" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Key</label>
          <input id="custom-apikey" type="password" class="input-field w-full" bind:value={customForm.api_key} placeholder="sk-..." />
        </div>
        <div>
          <div class="flex items-center justify-between mb-2">
            <span class="text-sm font-medium text-[var(--color-text-secondary)]">模型列表</span>
            <button class="text-xs text-[var(--color-primary)]" onclick={addCustomModel}>+ 添加模型</button>
          </div>
          {#each customForm.models as model, i}
            <div class="flex items-center gap-2 mb-2">
              <input type="text" class="input-field text-xs flex-1" placeholder="模型 ID" bind:value={model.id} />
              <input type="text" class="input-field text-xs flex-1" placeholder="模型名称" bind:value={model.name} />
              <button class="text-xs text-[var(--color-error)]" onclick={() => removeCustomModel(i)}>删除</button>
            </div>
          {/each}
        </div>
      </div>
      <div class="p-5 border-t flex justify-end gap-3" style="border-color: var(--color-border)">
        <button class="btn-ghost text-sm" onclick={() => { showCustomModal = false; editingCustom = null; }}>取消</button>
        <button class="btn-primary text-sm" onclick={saveCustomProvider} disabled={!customForm.name || !customForm.endpoint}>保存</button>
      </div>
    </div>
  </div>
{/if}

<!-- All Providers Modal -->
{#if showAllProvidersModal}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showAllProvidersModal = false; }}>
    <div class="rounded-2xl max-w-lg w-full border animate-[scaleIn_0.2s_ease-out] max-h-[80vh] overflow-hidden flex flex-col" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">全部提供商</h3>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => showAllProvidersModal = false}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      <div class="p-5 overflow-auto flex-1 space-y-2">
        {#each presetProviders as p}
          <div class="flex items-center justify-between p-3 rounded-xl" style="border: 1px solid var(--color-border);">
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-[var(--color-text)]">{p.name}</p>
              <p class="text-xs text-[var(--color-text-muted)] truncate">{p.endpoint}</p>
            </div>
            <span class="badge text-xs {badgeClass(providerStatus(p))}">{badgeLabel(providerStatus(p))}</span>
          </div>
        {/each}
      </div>
      <div class="p-4 border-t flex justify-end" style="border-color: var(--color-border)">
        <button class="btn-ghost text-sm" onclick={() => showAllProvidersModal = false}>关闭</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .providers-section { margin-bottom: 1.5rem; }
  .section-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: 1rem;
  }
  .section-header h3 {
    font-size: 1rem;
    font-weight: 600;
    color: var(--color-text);
    margin: 0;
  }
</style>

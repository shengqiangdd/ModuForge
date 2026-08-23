<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

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

  let {
    show = false,
    provider,
    userModelsMap,
    providerConfigs,
    onClose,
    onModelsChanged,
  }: {
    show: boolean;
    provider: Provider | null;
    userModelsMap: Record<string, Array<{ id: string; name: string }>>;
    providerConfigs: Record<string, { endpoint: string; api_key: string; models_json?: string }>;
    onClose: () => void;
    onModelsChanged: () => void;
  } = $props();

  let addingProviderId = $state('');
  let newModelId = $state('');
  let newModelName = $state('');
  let savingModel = $state(false);
  let removingKey = $state('');

  function handleBackdrop(e: MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }

  function startAdd() {
    if (!provider) return;
    addingProviderId = provider.id;
    newModelId = '';
    newModelName = '';
  }

  function cancelAdd() {
    addingProviderId = '';
    newModelId = '';
    newModelName = '';
  }

  async function saveAdd() {
    if (!provider || !newModelId.trim() || !newModelName.trim()) {
      toast('请填写模型 ID 和名称', 'error');
      return;
    }
    const pid = addingProviderId;
    const existing = userModelsMap[pid] || [];
    if (existing.some(m => m.id === newModelId.trim())) {
      toast('该模型 ID 已存在', 'error');
      return;
    }
    const updated = [...existing, { id: newModelId.trim(), name: newModelName.trim() }];
    const token = getToken();
    savingModel = true;
    try {
      const cfg = providerConfigs[pid] || {};
      const r = await fetch('/api/v1/llm/provider-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          id: pid,
          endpoint: cfg.endpoint || '',
          api_key: cfg.api_key || '',
          models_json: JSON.stringify(updated),
        }),
      });
      if (r.ok) {
        toast('模型已添加', 'success');
        addingProviderId = '';
        newModelId = '';
        newModelName = '';
        onModelsChanged();
      } else {
        toast((await r.json()).error || '添加失败', 'error');
      }
    } catch {
      toast('添加失败', 'error');
    } finally {
      savingModel = false;
    }
  }

  async function removeModel(modelId: string) {
    if (!provider) return;
    const pid = provider.id;
    const existing = userModelsMap[pid] || [];
    const updated = existing.filter(m => m.id !== modelId);
    const token = getToken();
    removingKey = `${pid}:${modelId}`;
    try {
      const cfg = providerConfigs[pid] || {};
      const r = await fetch('/api/v1/llm/provider-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          id: pid,
          endpoint: cfg.endpoint || '',
          api_key: cfg.api_key || '',
          models_json: updated.length > 0 ? JSON.stringify(updated) : '',
        }),
      });
      if (r.ok) {
        toast('模型已移除', 'success');
        onModelsChanged();
      } else {
        toast('移除失败', 'error');
      }
    } catch {
      toast('移除失败', 'error');
    } finally {
      removingKey = '';
    }
  }
</script>

{#if show && provider}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={handleBackdrop}>
    <div class="rounded-2xl max-w-lg w-full border animate-[scaleIn_0.2s_ease-out] max-h-[80vh] overflow-hidden flex flex-col" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">{provider.name} — 模型管理</h3>
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>
      <div class="p-5 overflow-auto flex-1 space-y-3">
        <!-- Built-in models -->
        {#if (provider.models?.length ?? 0) > 0}
          <div>
            <p class="text-xs font-medium text-[var(--color-text-muted)] mb-2">内置模型</p>
            {#each provider.models as model}
              <div class="flex items-center justify-between py-2 border-b border-[var(--color-border)]">
                <div>
                  <span class="text-sm text-[var(--color-text)]">{model.name || model.id}</span>
                  {#if model.name && model.id !== model.name}
                    <span class="text-xs text-[var(--color-text-muted)] ml-1">({model.id})</span>
                  {/if}
                </div>
                <span class="text-xs text-[var(--color-text-muted)]">{model.max_tokens ? `${Math.round(model.max_tokens / 1000)}k` : ''}</span>
              </div>
            {/each}
          </div>
        {/if}

        <!-- User-added models -->
        {#if (userModelsMap[provider.id] || []).length > 0}
          <div>
            <p class="text-xs font-medium text-[var(--color-text-muted)] mb-2">自定义模型</p>
            {#each (userModelsMap[provider.id] || []) as model}
              <div class="flex items-center justify-between py-2 border-b border-[var(--color-border)]">
                <div>
                  <span class="text-sm text-[var(--color-text)]">{model.name}</span>
                  <span class="text-xs text-[var(--color-text-muted)] ml-1">({model.id})</span>
                </div>
                <button class="text-xs text-[var(--color-error)]" onclick={() => removeModel(model.id)} disabled={removingKey === `${provider.id}:${model.id}`}>
                  {removingKey === `${provider.id}:${model.id}` ? '移除中...' : '移除'}
                </button>
              </div>
            {/each}
          </div>
        {/if}

        <!-- Add model form -->
        {#if addingProviderId === provider.id}
          <div class="p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
            <p class="text-xs font-medium text-[var(--color-text-muted)] mb-2">添加自定义模型</p>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 mb-2">
              <input type="text" class="input-field text-xs" placeholder="模型 ID (如 gpt-4-custom)" bind:value={newModelId} />
              <input type="text" class="input-field text-xs" placeholder="模型名称" bind:value={newModelName} />
            </div>
            <div class="flex gap-2">
              <button class="btn-primary text-xs" onclick={saveAdd} disabled={savingModel || !newModelId.trim() || !newModelName.trim()}>
                {savingModel ? '保存中...' : '保存'}
              </button>
              <button class="btn-ghost text-xs" onclick={cancelAdd}>取消</button>
            </div>
          </div>
        {:else}
          <button class="btn-ghost text-xs w-full py-2" onclick={startAdd}>
            <span class="material-symbols-outlined text-[14px]">add</span>
            添加自定义模型
          </button>
        {/if}
      </div>
      <div class="p-4 border-t flex justify-end" style="border-color: var(--color-border)">
        <button class="btn-ghost text-sm" onclick={onClose}>关闭</button>
      </div>
    </div>
  </div>
{/if}

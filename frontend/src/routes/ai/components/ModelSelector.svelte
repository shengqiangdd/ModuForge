<script lang="ts">
import type { Provider, Model } from '../lib/types';

let {
  providers = [],
  selectedProviderID = '',
  selectedModelID = '',
  configLoaded = false,
  showModelDropdown = false,
  editingModelMaxTokens = '',
  editMaxTokensValue = '',
  availableModels = [],
  freeModels = [],
  paidModels = [],
  selectedModel = null,
  onProviderChange,
  onModelSelect,
  onEditMaxTokens,
  onSaveMaxTokens,
  onToggleDropdown,
  onEditMaxTokensStart,
}: {
  providers: Provider[];
  selectedProviderID: string;
  selectedModelID: string;
  configLoaded: boolean;
  showModelDropdown: boolean;
  editingModelMaxTokens: string;
  editMaxTokensValue: string;
  availableModels: Model[];
  freeModels: Model[];
  paidModels: Model[];
  selectedModel: Model | null;
  onProviderChange: (v: string) => void;
  onModelSelect: (modelId: string) => void;
  onEditMaxTokens: (modelId: string, value: string) => void;
  onSaveMaxTokens: (modelId: string, value: string) => void;
  onToggleDropdown: () => void;
  onEditMaxTokensStart: (modelId: string, currentValue: string) => void;
} = $props();
</script>

{#if configLoaded && providers.length > 0}
  <div class="px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] top-bar-select">
    <div class="flex items-center gap-2">
      <select class="px-3 py-2 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] cursor-pointer top-bar-provider" value={selectedProviderID} onchange={(e) => onProviderChange((e.target as HTMLSelectElement).value)}>
        {#each providers as p}
          <option value={p.id}>{p.name} {p.is_free ? '🆓' : p.tier === 'subscription' ? '💳' : ''}</option>
        {/each}
      </select>
      <div class="relative flex-1 min-w-0 top-bar-model-wrap">
        <button class="w-full px-3 py-2 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] cursor-pointer text-left truncate top-bar-model" onclick={onToggleDropdown}>
          {selectedModel?.name || selectedModelID || '选择模型'}
        </button>
        {#if showModelDropdown}
          <div class="absolute left-0 top-full mt-1 z-50 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] shadow-xl overflow-y-auto model-dropdown" style="width: 100%; max-width: 100vw; max-height: 50vh; min-width: 260px;">
            {#if freeModels.length > 0}
              <div class="px-3 pt-2 pb-1 text-[10px] font-medium text-[var(--color-text-muted)]">🆓 免费</div>
              {#each freeModels as m}
                <div class="w-full px-3 py-1.5 text-sm hover:bg-[var(--color-surface)] transition-colors flex items-center gap-1.5 {m.id === selectedModelID ? 'text-[var(--color-primary)] bg-[var(--color-surface)]' : 'text-[var(--color-text)]'}">
                  <button class="flex-1 text-left truncate" onclick={() => onModelSelect(m.id)}>{m.name}</button>
                  {#if editingModelMaxTokens === m.id}
                    <input type="number" class="w-16 px-1 py-0.5 text-[10px] rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] text-right" value={editMaxTokensValue} oninput={(e) => onEditMaxTokens(m.id, (e.target as HTMLInputElement).value)} onkeydown={(e) => { if (e.key === 'Enter') onSaveMaxTokens(m.id, editMaxTokensValue); if (e.key === 'Escape') onEditMaxTokens('', ''); }} onblur={() => onSaveMaxTokens(m.id, editMaxTokensValue)} placeholder="tokens" />
                  {:else}
                    <button class="text-[10px] text-[var(--color-text-muted)] hover:text-[var(--color-text)] whitespace-nowrap cursor-pointer" onclick={(e) => { e.stopPropagation(); onEditMaxTokensStart(m.id, String(m.max_tokens || '')); }} title="设置最大输出tokens">{m.max_tokens ? (m.max_tokens >= 1000 ? Math.round(m.max_tokens/1000) + 'K' : m.max_tokens) : '⚙️'}</button>
                  {/if}
                </div>
              {/each}
            {/if}
            {#if paidModels.length > 0}
              <div class="px-3 pt-2 pb-1 text-[10px] font-medium text-[var(--color-text-muted)]">💰 付费</div>
              {#each paidModels as m}
                <div class="w-full px-3 py-1.5 text-sm hover:bg-[var(--color-surface)] transition-colors flex items-center gap-1.5 {m.id === selectedModelID ? 'text-[var(--color-primary)] bg-[var(--color-surface)]' : 'text-[var(--color-text)]'}">
                  <button class="flex-1 text-left truncate" onclick={() => onModelSelect(m.id)}>{m.name}</button>
                  {#if editingModelMaxTokens === m.id}
                    <input type="number" class="w-16 px-1 py-0.5 text-[10px] rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] text-right" value={editMaxTokensValue} oninput={(e) => onEditMaxTokens(m.id, (e.target as HTMLInputElement).value)} onkeydown={(e) => { if (e.key === 'Enter') onSaveMaxTokens(m.id, editMaxTokensValue); if (e.key === 'Escape') onEditMaxTokens('', ''); }} onblur={() => onSaveMaxTokens(m.id, editMaxTokensValue)} placeholder="tokens" />
                  {:else}
                    <button class="text-[10px] text-[var(--color-text-muted)] hover:text-[var(--color-text)] whitespace-nowrap cursor-pointer" onclick={(e) => { e.stopPropagation(); onEditMaxTokensStart(m.id, String(m.max_tokens || '')); }} title="设置最大输出tokens">{m.max_tokens ? (m.max_tokens >= 1000 ? Math.round(m.max_tokens/1000) + 'K' : m.max_tokens) : '⚙️'}</button>
                  {/if}
                </div>
              {/each}
            {/if}
          </div>
        {/if}
      </div>
    </div>
    {#if selectedModel && selectedModel.price_input_per_m > 0}
      <div class="mt-1 text-[10px] text-[var(--color-text-muted)] text-right">${selectedModel.price_input_per_m}/M tokens</div>
    {/if}
  </div>
{/if}

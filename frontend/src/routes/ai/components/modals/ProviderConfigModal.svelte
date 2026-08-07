<script lang="ts">
import { focusTrap } from '$lib/utils/focusTrap';
import type { Provider } from '../../lib/types';

let {
  show = false,
  providers = [],
  selectedProviderID = '',
  configEndpoint = '',
  configApiKey = '',
  configSaving = false,
  onClose,
  onEndpointChange,
  onApiKeyChange,
  onSave,
}: {
  show: boolean;
  providers: Provider[];
  selectedProviderID: string;
  configEndpoint: string;
  configApiKey: string;
  configSaving: boolean;
  onClose: () => void;
  onEndpointChange: (v: string) => void;
  onApiKeyChange: (v: string) => void;
  onSave: () => void;
} = $props();

let providerName = $derived(providers.find(p => p.id === selectedProviderID)?.name || '提供商');
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }} onkeydown={(e) => { if (e.key === 'Escape') onClose(); }}>
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 w-full max-w-md border border-[var(--color-border)] shadow-2xl" role="dialog" aria-modal="true" tabindex="-1" use:focusTrap>
      <div class="flex items-center gap-3 mb-5">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center" style="background: linear-gradient(135deg, color-mix(in srgb, var(--color-info) 15%, transparent), color-mix(in srgb, var(--color-primary) 15%, transparent))">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-info)">tune</span>
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--color-text)]">配置 {providerName}</h3>
          <p class="text-xs text-[var(--color-text-muted)]">设置 API Key 和 Base URL</p>
        </div>
      </div>
      <div class="space-y-4">
        <div>
          <label for="provider-base-url" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">Base URL</label>
          <input id="provider-base-url" type="text" class="w-full px-3 py-2 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] focus:outline-none focus:ring-2 focus:ring-primary-500/30" value={configEndpoint} oninput={(e) => onEndpointChange((e.target as HTMLInputElement).value)} placeholder="https://api.openai.com/v1/chat/completions" />
        </div>
        <div>
          <label for="provider-api-key" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Key</label>
          <input id="provider-api-key" type="password" class="w-full px-3 py-2 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] focus:outline-none focus:ring-2 focus:ring-primary-500/30" value={configApiKey} oninput={(e) => onApiKeyChange((e.target as HTMLInputElement).value)} placeholder="sk-..." />
          <p class="text-xs text-[var(--color-text-muted)] mt-1">密钥加密存储在服务器端</p>
        </div>
      </div>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>取消</button>
        <button class="flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={onSave} disabled={configSaving}>
          {#if configSaving}
            <span class="material-symbols-outlined text-[14px] animate-spin">progress_activity</span>
          {:else}
            <span class="material-symbols-outlined text-[14px]">save</span>
          {/if}
          保存
        </button>
      </div>
    </div>
  </div>
{/if}

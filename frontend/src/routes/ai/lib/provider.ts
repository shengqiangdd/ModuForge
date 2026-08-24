import { toast } from '$lib/stores/toast.svelte';
import type { Provider, Model } from './types';
import { authFetch } from './history';

interface CustomProviderPayload {
  id?: string;
  name?: string;
  endpoint?: string;
  api_key?: string;
  models_json?: string;
}

function toCustomProvider(cp: CustomProviderPayload): Provider {
  let models: Model[] = [];
  try {
    const raw = JSON.parse(cp.models_json || '[]');
    if (Array.isArray(raw)) {
      models = raw
        .map((m: { id?: string; name?: string }) => ({
          id: m.id || '',
          name: m.name || m.id || '',
          provider: cp.id || '',
          max_tokens: 0,
          supports_stream: true,
          price_input_per_m: 0,
          price_output_per_m: 0,
        }))
        .filter(m => m.id);
    }
  } catch (e) { console.warn('Failed to parse custom provider models:', e); }
  return {
    id: cp.id || '',
    name: cp.name || cp.id || '',
    endpoint: cp.endpoint || '',
    models,
    requires_key: !!cp.api_key,
    is_free: false,
    tier: 'paid',
    models_json: cp.models_json,
    api_key: cp.api_key,
  };
}

// ─── Load providers from backend ───
export async function loadProvidersFromBackend(): Promise<{
  providers: Provider[];
  selectedProviderID: string;
  selectedModelID: string;
}> {
  let providers: Provider[] = [];
  let selectedProviderID = '';
  let selectedModelID = '';

  try {
    const res = await fetch('/api/v1/llm/providers');
    const data = await res.json();
    providers = data.providers || [];

    // /api/v1/llm/providers 请求未带 token，后端不会合并自定义提供商，需单独拉取并按 id 去重合并
    try {
      const customRes = await authFetch('/api/v1/llm/custom-providers');
      if (customRes.ok) {
        const customData = await customRes.json();
        const knownIDs = new Set(providers.map(p => p.id));
        for (const cp of customData.providers || []) {
          if (knownIDs.has(cp.id)) continue;
          knownIDs.add(cp.id);
          providers.push(toCustomProvider(cp));
        }
      }
    } catch (e) { console.warn('Failed to load custom providers:', e); }

    let savedProvider = '';
    let savedModel = '';
    try {
      const cfgRes = await authFetch('/api/v1/llm/config');
      if (cfgRes.ok) {
        const cfg = await cfgRes.json();
        savedProvider = cfg.provider || '';
        savedModel = cfg.model_id || '';
      }
    } catch (e) { console.error('Failed to load LLM config:', e); }
    if (!savedProvider) {
      savedProvider = localStorage.getItem('moduforge_ai_provider') || '';
      savedModel = localStorage.getItem('moduforge_ai_model') || '';
    }

    if (savedProvider && providers.some(p => p.id === savedProvider)) {
      selectedProviderID = savedProvider;
      const provider = providers.find(p => p.id === savedProvider);
      if (savedModel && provider?.models.some(m => m.id === savedModel)) {
        selectedModelID = savedModel;
      } else if (provider && provider.models.length > 0) {
        selectedModelID = provider.models[0].id;
      }
    } else if (providers.length > 0) {
      selectedProviderID = providers[0].id;
      if (providers[0].models.length > 0) selectedModelID = providers[0].models[0].id;
    }
  } catch (e) { console.warn('loadProvidersFromBackend failed:', e); }

  return { providers, selectedProviderID, selectedModelID };
}

// ─── Save model selection to localStorage + backend ───
export function saveModelSelectionToStorage(providerID: string, modelID: string): void {
  if (providerID && modelID) {
    localStorage.setItem('moduforge_ai_provider', providerID);
    localStorage.setItem('moduforge_ai_model', modelID);
  }
}

// Only persist backend config when the selection actually changed, so we don't
// write DB on every message send (provider/model unchanged is the common case).
let lastSavedConfig: { provider: string; model: string } | null = null;

export async function saveConfigToBackend(providerID: string, modelID: string): Promise<void> {
  const token = localStorage.getItem('moduforge_token');
  if (!token || !providerID || !modelID) return;
  if (lastSavedConfig && lastSavedConfig.provider === providerID && lastSavedConfig.model === modelID) return;
  lastSavedConfig = { provider: providerID, model: modelID };
  try {
    const res = await authFetch('/api/v1/llm/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ provider: providerID, model_id: modelID }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      console.error('saveConfig failed:', err);
      toast(err.error || '保存配置失败', 'error');
    }
  } catch (e) { console.error('saveConfig error:', e); }
}

export async function refreshModelsFromBackend(): Promise<void> {
  try { await authFetch('/api/v1/llm/refresh'); } catch (e) { console.warn('refreshModels failed:', e); }
}

// ─── Save model max_tokens ───
export async function saveModelMaxTokens(
  providerID: string,
  providers: Provider[],
  modelId: string,
  maxTokens: number,
): Promise<boolean> {
  const token = localStorage.getItem('moduforge_token');
  if (!token || !providerID) return false;
  const provider = providers.find(p => p.id === providerID);
  if (!provider) return false;
  const models = provider.models.map(m => ({
    id: m.id,
    name: m.name,
    max_tokens: m.id === modelId ? maxTokens : (m.max_tokens || 0),
    price_input_per_m: m.price_input_per_m || 0,
    price_output_per_m: m.price_output_per_m || 0,
  }));
  try {
    await authFetch('/api/v1/llm/provider-config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: providerID, models_json: JSON.stringify(models) }),
    });
    const m = provider.models.find(x => x.id === modelId);
    if (m) m.max_tokens = maxTokens;
    toast(`已更新 ${m?.name || modelId} 最大输出为 ${maxTokens} tokens`, 'success');
    return true;
  } catch { toast('保存失败', 'error'); return false; }
}

// ─── Provider Config (API Key + Base URL) ───
export async function loadProviderConfig(pid: string): Promise<{ endpoint: string; apiKey: string }> {
  let endpoint = '';
  let apiKey = '';
  try {
    const r = await authFetch('/api/v1/llm/provider-configs');
    if (r.ok) {
      const data = await r.json();
      for (const c of data.configs || []) {
        if (c.id === pid) {
          endpoint = c.endpoint || '';
          apiKey = c.api_key || '';
          break;
        }
      }
    }
  } catch (e) { console.warn('loadProviderConfig failed:', e); }
  return { endpoint, apiKey };
}

export async function saveProviderConfigToBackend(
  providerID: string,
  endpoint: string,
  apiKey: string,
): Promise<boolean> {
  if (!providerID) return false;
  try {
    const r = await authFetch('/api/v1/llm/provider-config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: providerID, endpoint, api_key: apiKey }),
    });
    if (r.ok) {
      toast('提供商配置已保存', 'success');
      return true;
    } else {
      const err = await r.json().catch(() => ({}));
      toast(err.error || '保存失败', 'error');
      return false;
    }
  } catch { toast('保存失败', 'error'); return false; }
}

// ─── AI Capability ───
export async function fetchCapability(): Promise<any | null> {
  try {
    const res = await authFetch('/api/v1/ai/capability');
    if (res.ok) {
      const data = await res.json();
      return data.capability;
    }
  } catch (e) { console.warn('fetchCapability failed:', e); }
  return null;
}

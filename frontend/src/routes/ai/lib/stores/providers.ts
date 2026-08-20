/**
 * Provider & model selection state and operations.
 */
import { toast } from '$lib/stores/toast.svelte';
import type { Provider, Model } from '../types';
import {
  loadProvidersFromBackend,
  saveModelSelectionToStorage,
  saveConfigToBackend,
  refreshModelsFromBackend,
  saveModelMaxTokens,
  loadProviderConfig,
  saveProviderConfigToBackend,
  fetchCapability,
} from '../provider';

export interface ProviderState {
  providers: Provider[];
  selectedProviderID: string;
  selectedModelID: string;
  configLoaded: boolean;
  refreshing: boolean;
  showModelDropdown: boolean;
  editingModelMaxTokens: string;
  editMaxTokensValue: string;
  // Provider config modal
  showProviderConfig: boolean;
  configEndpoint: string;
  configApiKey: string;
  configSaving: boolean;
  // AI Capability
  showCapability: boolean;
  capability: any;
  capabilityLoading: boolean;
}

export function createProviderState(): ProviderState {
  return {
    providers: [],
    selectedProviderID: '',
    selectedModelID: '',
    configLoaded: false,
    refreshing: false,
    showModelDropdown: false,
    editingModelMaxTokens: '',
    editMaxTokensValue: '',
    showProviderConfig: false,
    configEndpoint: '',
    configApiKey: '',
    configSaving: false,
    showCapability: false,
    capability: null,
    capabilityLoading: false,
  };
}

// ─── Derived helpers ───

export function getAvailableModels(s: ProviderState): Model[] {
  return (s.providers || []).find(x => x.id === s.selectedProviderID)?.models || [];
}

export function getFreeModels(s: ProviderState): Model[] {
  return getAvailableModels(s).filter(m => m.price_input_per_m === 0 && m.price_output_per_m === 0);
}

export function getPaidModels(s: ProviderState): Model[] {
  return getAvailableModels(s).filter(m => m.price_input_per_m > 0 || m.price_output_per_m > 0);
}

export function getSelectedModel(s: ProviderState): Model | null {
  return getAvailableModels(s).find(m => m.id === s.selectedModelID) || null;
}

// ─── Actions ───

export async function loadProviders(s: ProviderState): Promise<void> {
  const result = await loadProvidersFromBackend();
  s.providers = result.providers;
  s.selectedProviderID = result.selectedProviderID;
  s.selectedModelID = result.selectedModelID;
  s.configLoaded = true;
}

export function onProviderChange(s: ProviderState): void {
  const models = getAvailableModels(s);
  if (models.length > 0) s.selectedModelID = models[0].id;
  saveModelSelectionToStorage(s.selectedProviderID, s.selectedModelID);
  saveConfigToBackend(s.selectedProviderID, s.selectedModelID);
}

export async function onModelSelect(s: ProviderState, modelId: string): Promise<void> {
  s.selectedModelID = modelId;
  saveModelSelectionToStorage(s.selectedProviderID, s.selectedModelID);
  await saveConfigToBackend(s.selectedProviderID, s.selectedModelID);
}

export async function refreshModels(s: ProviderState): Promise<void> {
  s.refreshing = true;
  await refreshModelsFromBackend();
  await loadProviders(s);
  s.refreshing = false;
}

export async function saveProviderConfigAction(s: ProviderState): Promise<void> {
  s.configSaving = true;
  const ok = await saveProviderConfigToBackend(s.selectedProviderID, s.configEndpoint, s.configApiKey);
  s.configSaving = false;
  if (ok) s.showProviderConfig = false;
}

export async function openProviderConfig(s: ProviderState): Promise<void> {
  s.showProviderConfig = true;
  s.configEndpoint = '';
  s.configApiKey = '';
  const cfg = await loadProviderConfig(s.selectedProviderID);
  s.configEndpoint = cfg.endpoint;
  s.configApiKey = cfg.apiKey;
}

export async function onSaveMaxTokens(s: ProviderState, modelId: string, value: string): Promise<void> {
  const maxTokens = parseInt(value);
  if (isNaN(maxTokens) || maxTokens <= 0) {
    toast('请输入有效的 token 数', 'error');
    return;
  }
  await saveModelMaxTokens(s.selectedProviderID, s.providers, modelId, maxTokens);
  s.editingModelMaxTokens = '';
  s.editMaxTokensValue = '';
}

export async function loadCapability(s: ProviderState): Promise<void> {
  s.capabilityLoading = true;
  s.capability = await fetchCapability();
  s.capabilityLoading = false;
  s.showCapability = true;
}

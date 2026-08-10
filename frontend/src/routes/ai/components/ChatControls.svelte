<script lang="ts">
  import type { Component } from 'svelte';
  import ModelSelector from './ModelSelector.svelte';
  import CompactToolbar from './CompactToolbar.svelte';
  import type { Mode, Provider, Model } from '../lib/types';

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
    mode = 'generate',
    streaming = false,
    showComparison = false,
    showProjectContext = false,
    showHistorySidebar = false,
    showCapability = false,
    onProviderChange,
    onModelSelect,
    onEditMaxTokens,
    onSaveMaxTokens,
    onToggleDropdown,
    onEditMaxTokensStart,
    onModeChange,
    onToggleComparison,
    onToggleProjectContext,
    onToggleHistory,
    onLoadCapability,
    onOpenPromptSettings,
    onOpenMDPrompts,
    onNavigate,
  }: {
    providers?: Provider[];
    selectedProviderID?: string;
    selectedModelID?: string;
    configLoaded?: boolean;
    showModelDropdown?: boolean;
    editingModelMaxTokens?: string;
    editMaxTokensValue?: string;
    availableModels?: Model[];
    freeModels?: Model[];
    paidModels?: Model[];
    selectedModel?: Model | null;
    mode?: Mode;
    streaming?: boolean;
    showComparison?: boolean;
    showProjectContext?: boolean;
    showHistorySidebar?: boolean;
    showCapability?: boolean;
    onProviderChange?: (v: string) => void;
    onModelSelect?: (id: string) => void;
    onEditMaxTokens?: (id: string, val: string) => void;
    onSaveMaxTokens?: (id: string, val: string) => void;
    onToggleDropdown?: () => void;
    onEditMaxTokensStart?: (id: string, val: string) => void;
    onModeChange?: (m: Mode) => void;
    onToggleComparison?: () => void;
    onToggleProjectContext?: () => void;
    onToggleHistory?: () => void;
    onLoadCapability?: () => void;
    onOpenPromptSettings?: () => void;
    onOpenMDPrompts?: () => void;
    onNavigate?: (route: string, id?: string) => void;
  } = $props();
</script>

<ModelSelector {providers} {selectedProviderID} {selectedModelID} {configLoaded} {showModelDropdown} {editingModelMaxTokens} {editMaxTokensValue} {availableModels} {freeModels} {paidModels} {selectedModel}
  onProviderChange={(v) => onProviderChange?.(v)}
  onModelSelect={(id) => onModelSelect?.(id)}
  onEditMaxTokens={(id, val) => onEditMaxTokens?.(id, val)}
  onSaveMaxTokens={(id, val) => onSaveMaxTokens?.(id, val)}
  onToggleDropdown={() => onToggleDropdown?.()}
  onEditMaxTokensStart={(id, val) => onEditMaxTokensStart?.(id, val)}
/>

<CompactToolbar {mode} {streaming} {showComparison} {showProjectContext} {showHistorySidebar} {showCapability}
  onModeChange={(m) => onModeChange?.(m)}
  onToggleComparison={() => onToggleComparison?.()}
  onToggleProjectContext={() => onToggleProjectContext?.()}
  onToggleHistory={() => onToggleHistory?.()}
  onLoadCapability={() => onLoadCapability?.()}
  onOpenPromptSettings={() => onOpenPromptSettings?.()}
  onOpenMDPrompts={() => onOpenMDPrompts?.()}
  {onNavigate}
/>

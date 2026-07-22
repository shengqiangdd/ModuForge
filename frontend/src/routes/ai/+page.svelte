<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { client, streamRequest } from '../../lib/api/client';

  type Mode = 'generate' | 'chat' | 'repair';

  // --- Provider/Model Selection ---
  interface Model {
    id: string;
    name: string;
    provider: string;
    max_tokens: number;
    supports_stream: boolean;
    price_input_per_m: number;
    price_output_per_m: number;
  }
  interface Provider {
    name: string;
    id: string;
    endpoint: string;
    models: Model[];
    requires_key: boolean;
    is_free: boolean;
    tier: string;
  }

  let providers = $state<Provider[]>([]);
  let selectedProviderID = $state('');
  let selectedModelID = $state('');
  let configLoaded = $state(false);
  let refreshing = $state(false);
  let refreshResult = $state<{added: string[]; removed: string[]; total_remote: number; total_local: number} | null>(null);

  let availableModels = $derived(
    providers.find(x => x.id === selectedProviderID)?.models || []
  );

  // 分离免费和付费模型
  let freeModels = $derived(availableModels.filter(m => m.price_input_per_m === 0 && m.price_output_per_m === 0));
  let paidModels = $derived(availableModels.filter(m => m.price_input_per_m > 0 || m.price_output_per_m > 0));

  let selectedModel = $derived(
    availableModels.find(m => m.id === selectedModelID) || null
  );

  let needsKey = $derived(
    providers.find(x => x.id === selectedProviderID)?.requires_key || false
  );

  let tierLabel = $derived.by(() => {
    const p = providers.find(x => x.id === selectedProviderID);
    if (!p) return '';
    if (p.is_free) return '🆓 免费';
    if (p.tier === 'subscription') return '💳 订阅制';
    return '💰 按量付费';
  });

  // --- Chat State ---
  let mode = $state<Mode>('generate');
  let input = $state('');
  let messages = $state<{role: string; content: string}[]>([]);
  let streaming = $state(false);
  let moduleType = $state('magisk');
  let buildLog = $state('');

  let streamCtrl: EventSource | null = null;

  const modes = [
    { value: 'generate' as const, label: '生成模块', icon: 'auto_fix_high' },
    { value: 'chat' as const, label: 'AI 对话', icon: 'chat' },
    { value: 'repair' as const, label: '修复构建', icon: 'build_circle' },
  ];

  onMount(async () => {
    window.addEventListener('ai-stream', onStreamData);
    window.addEventListener('ai-stream-done', onStreamDone);
    await loadProviders();
  });

  onDestroy(() => {
    window.removeEventListener('ai-stream', onStreamData);
    window.removeEventListener('ai-stream-done', onStreamDone);
  });

  async function loadProviders() {
    try {
      const res = await fetch('/api/v1/llm/providers');
      const data = await res.json();
      providers = data.providers || [];

      // Load current config
      try {
        const cfgRes = await fetch('/api/v1/llm/config', {
          headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` }
        });
        if (cfgRes.ok) {
          const cfg = await cfgRes.json();
          if (cfg.provider) selectedProviderID = cfg.provider;
          if (cfg.model_id) selectedModelID = cfg.model_id;
        }
      } catch { /* config not available */ }

      // Defaults
      if (!selectedProviderID && providers.length > 0) {
        selectedProviderID = providers[0].id;
        if (providers[0].models.length > 0) {
          selectedModelID = providers[0].models[0].id;
        }
      }
      configLoaded = true;
    } catch {
      configLoaded = true;
    }
  }

  function onProviderChange() {
    const models = availableModels;
    if (models.length > 0) {
      selectedModelID = models[0].id;
    }
  }

  async function saveConfig() {
    const token = localStorage.getItem('moduforge_token');
    if (!token) return;
    try {
      await fetch('/api/v1/llm/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ provider: selectedProviderID, model_id: selectedModelID })
      });
    } catch { /* silent */ }
  }

  async function refreshModels() {
    refreshing = true;
    refreshResult = null;
    try {
      const res = await fetch('/api/v1/llm/refresh');
      const data = await res.json();
      refreshResult = {
        added: data.added || [],
        removed: data.removed || [],
        total_remote: data.total_remote || 0,
        total_local: data.total_local || 0,
      };
      // 自动重新加载提供商列表以获取最新模型
      await loadProviders();
    } catch {
      refreshResult = { added: [], removed: [], total_remote: 0, total_local: 0 };
    } finally {
      refreshing = false;
    }
  }

  function onStreamData(e: Event) {
    const detail = (e as CustomEvent).detail as string;
    const lines = detail.split('\n');
    for (const line of lines) {
      if (line.startsWith('data: ')) {
        const data = line.slice(6);
        if (data === '[DONE]') { streaming = false; return; }
        try {
          const parsed = JSON.parse(data);
          if (parsed.content) {
            if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') {
              messages[messages.length - 1].content += parsed.content;
              messages = messages;
            } else {
              messages = [...messages, { role: 'assistant', content: parsed.content }];
            }
          }
        } catch {
          if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') {
            messages[messages.length - 1].content += data;
            messages = messages;
          } else {
            messages = [...messages, { role: 'assistant', content: data }];
          }
        }
      }
    }
  }

  function onStreamDone() {
    streaming = false;
  }

  async function send() {
    const text = input.trim();
    if (!text || streaming) return;

    input = '';
    messages = [...messages, { role: 'user', content: text }];
    streaming = true;

    await saveConfig();

    let body: unknown;
    let path: string;

    if (mode === 'generate') {
      path = '/ai/generate';
      body = { description: text, module_type: moduleType };
    } else if (mode === 'repair') {
      path = '/ai/repair';
      body = { build_log: buildLog || text };
    } else {
      path = '/ai/chat';
      body = { message: text };
    }

    streamCtrl = streamRequest(path, body);
  }

  function stopStream() {
    streamCtrl?.close();
    streaming = false;
  }

  function modelDisplayName(m: Model): string {
    const price = m.price_input_per_m === 0 && m.price_output_per_m === 0;
    const suffix = price ? ' 🆓' : ` $${m.price_input_per_m}/M`;
    return `${m.name}${suffix}`;
  }
</script>

<div class="flex flex-col h-screen">
  <!-- Provider/Model Selection Bar -->
  {#if configLoaded && providers.length > 0}
    <div class="flex items-center gap-3 px-4 py-2 border-b border-outline-variant bg-surface-container-low text-xs flex-wrap">
      <!-- Provider -->
      <div class="flex items-center gap-1.5">
        <span class="text-on-surface-variant font-medium">提供商:</span>
        <select
          class="px-2 py-1 rounded border border-outline-variant bg-surface text-on-surface cursor-pointer"
          bind:value={selectedProviderID}
          onchange={onProviderChange}
        >
          {#each providers as p}
            <option value={p.id}>{p.name} {p.is_free ? '(免费)' : p.tier === 'subscription' ? '(订阅)' : ''}</option>
          {/each}
        </select>
      </div>

      <!-- Model -->
      <div class="flex items-center gap-1.5">
        <span class="text-on-surface-variant font-medium">模型:</span>
        <select
          class="px-2 py-1 rounded border border-outline-variant bg-surface text-on-surface cursor-pointer"
          bind:value={selectedModelID}
        >
          {#if freeModels.length > 0}
            <optgroup label="🆓 免费模型">
              {#each freeModels as m}
                <option value={m.id}>{m.name}</option>
              {/each}
            </optgroup>
          {/if}
          {#if paidModels.length > 0}
            <optgroup label="💰 付费模型">
              {#each paidModels as m}
                <option value={m.id}>{m.name} (${m.price_input_per_m}/{m.price_output_per_m} per M)</option>
              {/each}
            </optgroup>
          {/if}
        </select>
      </div>

      <!-- Tier badge -->
      <span class="px-1.5 py-0.5 rounded text-[10px] bg-surface-container-high text-on-surface-variant">
        {tierLabel}
      </span>

      <!-- Pricing info -->
      {#if selectedModel}
        {@const model = selectedModel}
        {#if model.price_input_per_m > 0 || model.price_output_per_m > 0}
          <span class="text-on-surface-variant">
            💲 {model.price_input_per_m}/M in · {model.price_output_per_m}/M out
          </span>
        {:else}
          <span class="text-green-600 font-medium">免费</span>
        {/if}
      {/if}

      <!-- 模型总数 -->
      <span class="text-on-surface-variant">
        {availableModels.length} 个模型
      </span>

      <!-- 刷新按钮 -->
      <button
        class="px-2 py-1 rounded text-xs bg-primary-container text-on-primary-container cursor-pointer flex items-center gap-1 hover:opacity-80 transition-opacity"
        onclick={refreshModels}
        disabled={refreshing}
      >
        <md-icon class="text-sm">{refreshing ? 'sync' : 'refresh'}</md-icon>
        {refreshing ? '刷新中...' : '刷新模型'}
      </button>

      {#if refreshResult}
        <span class="text-[10px] text-on-surface-variant">
          远程 {refreshResult.total_remote} · 本地 {refreshResult.total_local}
          {#if refreshResult.added.length > 0}
            · <span class="text-green-600">+{refreshResult.added.length} 新增</span>
          {/if}
          {#if refreshResult.removed.length > 0}
            · <span class="text-orange-600">-{refreshResult.removed.length} 移除</span>
          {/if}
        </span>
      {/if}
    </div>
  {/if}

  <!-- 标签栏 -->
  <div class="flex items-center gap-1 px-4 py-2 border-b border-outline-variant bg-surface-container-high">
    {#each modes as m}
      <button
        class="flex items-center gap-1 px-3 py-1.5 rounded-lg text-sm transition-colors cursor-pointer"
        class:bg-primary-container={mode === m.value}
        class:text-on-primary-container={mode === m.value}
        onclick={() => mode = m.value}
      >
        <md-icon class="text-base">{m.icon}</md-icon>
        {m.label}
      </button>
    {/each}
  </div>

  <!-- 消息区 -->
  <div class="flex-1 overflow-auto p-4 space-y-4">
    {#if messages.length === 0}
      <div class="flex items-center justify-center h-full text-on-surface-variant">
        <div class="text-center">
          <md-icon class="text-6xl mb-4">psychology</md-icon>
          <p class="text-body-large">
            {#if mode === 'generate'}
              描述你想要的模块，AI 帮你生成代码
            {:else if mode === 'repair'}
              粘贴构建日志，AI 分析问题并给出修复建议
            {:else}
              与 AI 对话，获取模块开发帮助
            {/if}
          </p>
        </div>
      </div>
    {:else}
      {#each messages as msg}
        <div class="flex {msg.role === 'user' ? 'justify-end' : 'justify-start'}">
          <div class="max-w-2xl px-4 py-3 rounded-2xl text-sm whitespace-pre-wrap
            {msg.role === 'user' ? 'bg-primary text-on-primary' : 'bg-surface-container-high text-on-surface'}">
            {msg.content}
          </div>
        </div>
      {/each}
    {/if}
  </div>

  <!-- 输入区 -->
  <div class="border-t border-outline-variant p-4 bg-surface">
    {#if mode === 'generate'}
      <div class="flex items-center gap-2 mb-2">
        <span class="text-sm text-on-surface-variant">模块类型:</span>
        {#each ['magisk', 'ksu', 'apatch'] as t}
          <button
            class="px-2 py-0.5 rounded text-xs border cursor-pointer"
            class:border-primary={moduleType === t}
            class:bg-primary-container={moduleType === t}
            onclick={() => moduleType = t}
          >{t}</button>
        {/each}
      </div>
    {/if}

    {#if mode === 'repair'}
      <textarea
        class="w-full p-2 mb-2 border border-outline-variant rounded text-sm font-mono"
        rows="3"
        placeholder="粘贴构建日志（可选，留空则使用输入框内容）"
        bind:value={buildLog}
      ></textarea>
    {/if}

    <div class="flex items-end gap-2">
      <textarea
        class="flex-1 p-3 border border-outline-variant rounded-xl resize-none focus:outline-none focus:border-primary text-sm"
        rows="2"
        placeholder={mode === 'generate' ? '描述你的模块功能...' : mode === 'repair' ? '描述问题...' : '输入消息...'}
        bind:value={input}
        onkeydown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }}
      ></textarea>
      {#if streaming}
        <md-icon-button onclick={stopStream}>
          <md-icon>stop_circle</md-icon>
        </md-icon-button>
      {:else}
        <md-filled-button onclick={send} disabled={!input.trim()}>
          <md-icon slot="start">send</md-icon>
          发送
        </md-filled-button>
      {/if}
    </div>
  </div>
</div>

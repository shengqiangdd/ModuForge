<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  type Mode = 'generate' | 'chat' | 'repair';

  interface Model {
    id: string; name: string; provider: string; max_tokens: number;
    supports_stream: boolean; price_input_per_m: number; price_output_per_m: number;
  }
  interface Provider {
    name: string; id: string; endpoint: string; models: Model[];
    requires_key: boolean; is_free: boolean; tier: string;
  }

  let providers = $state<Provider[]>([]);
  let selectedProviderID = $state('');
  let selectedModelID = $state('');
  let configLoaded = $state(false);
  let refreshing = $state(false);

  let availableModels = $derived(providers.find(x => x.id === selectedProviderID)?.models || []);
  let freeModels = $derived(availableModels.filter(m => m.price_input_per_m === 0 && m.price_output_per_m === 0));
  let paidModels = $derived(availableModels.filter(m => m.price_input_per_m > 0 || m.price_output_per_m > 0));
  let selectedModel = $derived(availableModels.find(m => m.id === selectedModelID) || null);

  let mode = $state<Mode>('generate');
  let input = $state('');
  let messages = $state<{role: string; content: string}[]>([]);
  let streaming = $state(false);
  let moduleType = $state('magisk');
  let buildLog = $state('');
  let streamCtrl: any = null;
  let showSettings = $state(false);
  let chatEnd: HTMLDivElement | undefined = $state();

  const modes = [
    { value: 'generate' as const, label: '生成模块', icon: 'auto_fix_high', desc: '描述需求，AI 生成代码' },
    { value: 'chat' as const, label: 'AI 对话', icon: 'chat', desc: '与 AI 对话获取帮助' },
    { value: 'repair' as const, label: '修复构建', icon: 'build_circle', desc: '粘贴日志分析问题' },
  ];

  $effect(() => {
    if (chatEnd) chatEnd.scrollIntoView({ behavior: 'smooth' });
  });

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
      try {
        const cfgRes = await fetch('/api/v1/llm/config', { headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` } });
        if (cfgRes.ok) { const cfg = await cfgRes.json(); if (cfg.provider) selectedProviderID = cfg.provider; if (cfg.model_id) selectedModelID = cfg.model_id; }
      } catch {}
      if (!selectedProviderID && providers.length > 0) { selectedProviderID = providers[0].id; if (providers[0].models.length > 0) selectedModelID = providers[0].models[0].id; }
      configLoaded = true;
    } catch { configLoaded = true; }
  }

  function onProviderChange() { if (availableModels.length > 0) selectedModelID = availableModels[0].id; }

  async function saveConfig() {
    const token = localStorage.getItem('moduforge_token');
    if (!token) return;
    try { await fetch('/api/v1/llm/config', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, body: JSON.stringify({ provider: selectedProviderID, model_id: selectedModelID }) }); } catch {}
  }

  async function refreshModels() {
    refreshing = true;
    try { await fetch('/api/v1/llm/refresh'); await loadProviders(); } catch {}
    refreshing = false;
  }

  function onStreamData(e: Event) {
    const detail = (e as CustomEvent).detail as string;
    for (const line of detail.split('\n')) {
      if (!line.startsWith('data: ')) continue;
      const data = line.slice(6);
      if (data === '[DONE]') { streaming = false; return; }
      try {
        const parsed = JSON.parse(data);
        if (parsed.content) {
          if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') {
            messages[messages.length - 1].content += parsed.content;
            messages = messages;
          } else { messages = [...messages, { role: 'assistant', content: parsed.content }]; }
        }
      } catch {
        if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') {
          messages[messages.length - 1].content += data; messages = messages;
        } else { messages = [...messages, { role: 'assistant', content: data }]; }
      }
    }
  }

  function onStreamDone() { streaming = false; }

  async function send() {
    const text = input.trim();
    if (!text || streaming) return;
    input = '';
    messages = [...messages, { role: 'user', content: text }];
    streaming = true;
    await saveConfig();
    let body: unknown; let path: string;
    if (mode === 'generate') { path = '/ai/generate'; body = { description: text, module_type: moduleType }; }
    else if (mode === 'repair') { path = '/ai/repair'; body = { build_log: buildLog || text }; }
    else { path = '/ai/chat'; body = { message: text }; }
    streamCtrl = (await import('../../lib/api/client')).streamRequest(path, body);
  }

  function stopStream() { streamCtrl?.close(); streaming = false; }
</script>

<div class="flex flex-col h-full">
  <!-- Top Bar: Model Selection -->
  {#if configLoaded && providers.length > 0}
    <div class="flex items-center gap-2 px-4 py-2.5 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex-wrap">
      <select class="px-3 py-1.5 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] cursor-pointer" bind:value={selectedProviderID} onchange={onProviderChange}>
        {#each providers as p}
          <option value={p.id}>{p.name} {p.is_free ? '🆓' : ''}</option>
        {/each}
      </select>
      <select class="px-3 py-1.5 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] cursor-pointer" bind:value={selectedModelID}>
        {#if freeModels.length > 0}<optgroup label="🆓 免费">{#each freeModels as m}<option value={m.id}>{m.name}</option>{/each}</optgroup>{/if}
        {#if paidModels.length > 0}<optgroup label="💰 付费">{#each paidModels as m}<option value={m.id}>{m.name}</option>{/each}</optgroup>{/if}
      </select>
      {#if selectedModel}
        <span class="text-xs text-[var(--color-text-muted)]">
          {selectedModel.price_input_per_m === 0 ? '✅ 免费' : `$${selectedModel.price_input_per_m}/M in`}
        </span>
      {/if}
      <button class="ml-auto flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs text-[var(--color-text-secondary)] hover:bg-neutral-100 transition-colors" onclick={refreshModels} disabled={refreshing}>
        <span class="material-symbols-outlined text-[14px] {refreshing ? 'animate-spin' : ''}">refresh</span>
        刷新
      </button>
    </div>
  {/if}

  <!-- Mode Tabs -->
  <div class="flex items-center gap-1 px-4 py-2 border-b border-[var(--color-border)] bg-[var(--color-surface)]">
    {#each modes as m}
      <button
        class="flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium transition-all duration-150
          {mode === m.value ? 'bg-primary-600 text-white shadow-sm' : 'text-[var(--color-text-secondary)] hover:bg-neutral-200'}"
        onclick={() => mode = m.value}
      >
        <span class="material-symbols-outlined text-[16px]">{m.icon}</span>
        {m.label}
      </button>
    {/each}
  </div>

  <!-- Messages -->
  <div class="flex-1 overflow-y-auto p-4 space-y-4">
    {#if messages.length === 0}
      <div class="flex items-center justify-center h-full">
        <div class="text-center">
          <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-100 to-primary-200 flex items-center justify-center mx-auto mb-4">
            <span class="material-symbols-outlined text-primary-600 text-3xl">psychology</span>
          </div>
          <p class="text-lg font-semibold text-[var(--color-text)]">{modes.find(m => m.value === mode)?.desc}</p>
          <p class="text-sm text-[var(--color-text-muted)] mt-1">
            {#if mode === 'generate'}支持 Magisk / KernelSU / APatch{:else if mode === 'repair'}粘贴构建日志，AI 分析问题并给出修复建议{:else}随时提问关于模块开发的问题{/if}
          </p>
        </div>
      </div>
    {:else}
      {#each messages as msg}
        <div class="flex {msg.role === 'user' ? 'justify-end' : 'justify-start'}">
          <div class="max-w-2xl px-4 py-3 rounded-2xl text-sm leading-relaxed whitespace-pre-wrap
            {msg.role === 'user'
              ? 'bg-primary-600 text-white rounded-br-md'
              : 'bg-[var(--color-surface)] text-[var(--color-text)] border border-[var(--color-border)] rounded-bl-md'}">
            {#if msg.role === 'assistant'}
              <div class="flex items-center gap-1.5 mb-1.5">
                <span class="material-symbols-outlined text-primary-500 text-[14px]">auto_awesome</span>
                <span class="text-xs font-medium text-primary-600">AI</span>
              </div>
            {/if}
            {msg.content}
          </div>
        </div>
      {/each}
    {/if}
    <div bind:this={chatEnd}></div>
  </div>

  <!-- Input -->
  <div class="border-t border-[var(--color-border)] p-4 bg-[var(--color-bg-elevated)]">
    {#if mode === 'generate'}
      <div class="flex items-center gap-2 mb-2">
        <span class="text-xs text-[var(--color-text-muted)]">类型:</span>
        {#each ['magisk', 'ksu', 'apatch'] as t}
          <button class="px-3 py-1 rounded-lg text-xs font-medium border transition-all {moduleType === t ? 'bg-primary-600 text-white border-primary-600' : 'border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-neutral-100'}" onclick={() => moduleType = t}>{t}</button>
        {/each}
      </div>
    {/if}
    {#if mode === 'repair'}
      <textarea class="input-field text-xs font-mono resize-none mb-2" rows="2" placeholder="粘贴构建日志（可选）" bind:value={buildLog}></textarea>
    {/if}
    <div class="flex items-end gap-2">
      <textarea
        class="flex-1 input-field resize-none"
        rows="2"
        placeholder={mode === 'generate' ? '描述你的模块功能...' : mode === 'repair' ? '描述问题...' : '输入消息...'}
        bind:value={input}
        onkeydown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }}
      ></textarea>
      {#if streaming}
        <button class="p-3 rounded-xl bg-red-50 text-red-500 hover:bg-red-100 transition-colors" onclick={stopStream}>
          <span class="material-symbols-outlined text-[20px]">stop_circle</span>
        </button>
      {:else}
        <button class="p-3 rounded-xl bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={send} disabled={!input.trim()}>
          <span class="material-symbols-outlined text-[20px]">send</span>
        </button>
      {/if}
    </div>
  </div>
</div>

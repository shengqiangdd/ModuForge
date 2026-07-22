<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { streamRequest } from '../../lib/api/client';

  type Mode = 'generate' | 'chat' | 'repair';

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

  onMount(() => {
    window.addEventListener('ai-stream', onStreamData);
    window.addEventListener('ai-stream-done', onStreamDone);
    return () => {
      window.removeEventListener('ai-stream', onStreamData);
      window.removeEventListener('ai-stream-done', onStreamDone);
    };
  });

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
            // Append to last assistant message or create new
            if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') {
              messages[messages.length - 1].content += parsed.content;
              messages = messages; // trigger reactivity
            } else {
              messages = [...messages, { role: 'assistant', content: parsed.content }];
            }
          }
        } catch {
          // Not JSON, append raw
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
</script>

<div class="flex flex-col h-screen">
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

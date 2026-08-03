<script lang="ts">
  import { onMount } from 'svelte';

  let { execCommand = async (_cmd: string): Promise<string> => { return ''; }, onClose }: {
    execCommand?: (cmd: string) => Promise<string>;
    onClose?: () => void;
  } = $props();

  let output = $state<string[]>([]);
  let input = $state('');
  let history = $state<string[]>([]);
  let historyIndex = $state(-1);
  let inputEl: HTMLInputElement | undefined = $state();
  let outputEl: HTMLDivElement | undefined = $state();
  let running = $state(false);

  function focus() {
    setTimeout(() => inputEl?.focus(), 50);
  }

  function scrollToBottom() {
    setTimeout(() => {
      if (outputEl) {
        outputEl.scrollTop = outputEl.scrollHeight;
      }
    }, 50);
  }

  async function run(cmd: string) {
    const trimmed = cmd.trim();
    if (!trimmed) return;

    output = [...output, `$ ${trimmed}`];
    history = [...history, trimmed];
    historyIndex = -1;
    input = '';
    running = true;

    try {
      const result = await execCommand(trimmed);
      output = [...output, result || '(empty output)'];
    } catch (e: any) {
      output = [...output, `Error: ${e.message || e}`];
    } finally {
      running = false;
      scrollToBottom();
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      run(input);
      return;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (history.length === 0) return;
      const idx = historyIndex === -1 ? history.length - 1 : Math.max(0, historyIndex - 1);
      historyIndex = idx;
      input = history[idx];
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (historyIndex === -1) return;
      if (historyIndex >= history.length - 1) {
        historyIndex = -1;
        input = '';
        return;
      }
      const idx = historyIndex + 1;
      historyIndex = idx;
      input = history[idx];
      return;
    }
    if (e.key === 'l' && e.ctrlKey) {
      e.preventDefault();
      output = [];
    }
  }

  function clearOutput() {
    output = [];
  }

  onMount(() => {
    output = ['Welcome to ModuForge Terminal', 'Type a command and press Enter'];
    scrollToBottom();
  });
</script>

<div class="terminal">
  <div class="terminal-header">
    <span class="terminal-title">Terminal</span>
    <div class="terminal-actions">
      <button class="terminal-btn" onclick={clearOutput} title="清屏">
        <span class="material-symbols-outlined text-[14px]">delete_sweep</span>
      </button>
      {#if onClose}
        <button class="terminal-btn" onclick={onClose} title="关闭终端">
          <span class="material-symbols-outlined text-[14px]">close</span>
        </button>
      {/if}
    </div>
  </div>
  <div class="terminal-body" bind:this={outputEl} onclick={focus}>
    {#each output as line}
      <div class="terminal-line">{line}</div>
    {/each}
    <div class="terminal-input-line">
      <span class="terminal-prompt">$</span>
      <input
        bind:this={inputEl}
        type="text"
        bind:value={input}
        onkeydown={handleKeydown}
        disabled={running}
        class="terminal-input"
        placeholder="输入命令..."
      />
    </div>
  </div>
</div>

<style>
  .terminal {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: #0d1117;
    border-top: 1px solid #30363d;
    font-family: monospace;
    font-size: 13px;
  }
  .terminal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 12px;
    background: #161b22;
    border-bottom: 1px solid #30363d;
    flex-shrink: 0;
  }
  .terminal-title {
    color: #8b949e;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .terminal-actions {
    display: flex;
    gap: 4px;
  }
  .terminal-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border-radius: 4px;
    border: none;
    background: transparent;
    color: #8b949e;
    cursor: pointer;
    transition: all 0.15s;
  }
  .terminal-btn:hover {
    background: #21262d;
    color: #e6edf3;
  }
  .terminal-body {
    flex: 1;
    overflow-y: auto;
    padding: 8px 12px;
    cursor: text;
  }
  .terminal-body::-webkit-scrollbar {
    width: 6px;
  }
  .terminal-body::-webkit-scrollbar-track {
    background: transparent;
  }
  .terminal-body::-webkit-scrollbar-thumb {
    background: #30363d;
    border-radius: 3px;
  }
  .terminal-line {
    color: #e6edf3;
    white-space: pre-wrap;
    word-break: break-all;
    line-height: 1.6;
    min-height: 1.6em;
  }
  .terminal-line:first-child {
    color: #8b949e;
  }
  .terminal-input-line {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 2px;
  }
  .terminal-prompt {
    color: #22c55e;
    font-weight: 500;
    flex-shrink: 0;
  }
  .terminal-input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    color: #e6edf3;
    font-family: inherit;
    font-size: inherit;
    caret-color: #22c55e;
  }
  .terminal-input::placeholder {
    color: #484f58;
  }
</style>

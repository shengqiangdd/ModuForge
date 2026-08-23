<script lang="ts">
  import { onMount } from 'svelte';
  import type { ContextProject } from '../lib/types';

  let {
    input = '',
    streaming = false,
    mode = 'generate' as string,
    messages = [] as any[],
    buildLog = '',
    analysisModes = [] as { id: string; label: string; icon: string; prompt: string }[],
    mcpToolCount = 0,
    // Project context props
    showProjectContext = false,
    contextProjectList = [] as { id: string; name: string }[],
    contextProjects = [] as ContextProject[],
    selectedProject = '',
    selectedFile = '',
    projectContext = '',
    onToggleProjectContext,
    onProjectChange,
    onFileAdd,
    onContextChange,
    // End project context props
    onSend,
    onStop,
    onSendAnalysis,
    onBuildLogChange,
    onInputChange,
    onOpenMcpTools,
  }: {
    input?: string;
    streaming?: boolean;
    mode?: string;
    messages?: any[];
    buildLog?: string;
    analysisModes?: { id: string; label: string; icon: string; prompt: string }[];
    mcpToolCount?: number;
    showProjectContext?: boolean;
    contextProjectList?: { id: string; name: string }[];
    contextProjects?: ContextProject[];
    selectedProject?: string;
    selectedFile?: string;
    projectContext?: string;
    onToggleProjectContext?: () => void;
    onProjectChange?: (v: string) => void;
    onFileAdd?: (v: string) => void;
    onContextChange?: (v: string) => void;
    onSend?: () => void;
    onStop?: () => void;
    onSendAnalysis?: (text: string, modeId: string) => void;
    onBuildLogChange?: (value: string) => void;
    onInputChange?: (value: string) => void;
    onOpenMcpTools?: () => void;
  } = $props();

  let textareaEl: HTMLTextAreaElement | undefined = $state();
  let showContextDropdown = $state(false);

  // 输入历史：↑ 恢复上一条发送内容，↓ 下一条（sessionStorage 持久化，跨刷新保留）
  const HISTORY_KEY = 'moduforge_ai_input_history';
  const MAX_HISTORY = 30;
  let inputHistory = $state<string[]>([]);
  let historyIndex = $state(-1);

  onMount(() => {
    try {
      const raw = sessionStorage.getItem(HISTORY_KEY);
      if (raw) {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed)) inputHistory = parsed.filter((h: unknown) => typeof h === 'string');
      }
    } catch { /* ignore */ }
    textareaEl?.focus();
  });

  function saveToHistory(text: string) {
    const t = text.trim();
    if (!t) return;
    const next = [t, ...inputHistory.filter((h) => h !== t)].slice(0, MAX_HISTORY);
    inputHistory = next;
    try { sessionStorage.setItem(HISTORY_KEY, JSON.stringify(next)); } catch { /* ignore */ }
  }

  const MAX_INPUT_HEIGHT = 200;
  function autoResize(el: HTMLTextAreaElement | undefined) {
    if (!el) return;
    el.style.height = 'auto';
    const h = Math.max(48, Math.min(el.scrollHeight, MAX_INPUT_HEIGHT));
    el.style.height = h + 'px';
    el.style.overflowY = el.scrollHeight > MAX_INPUT_HEIGHT ? 'auto' : 'hidden';
  }
  $effect(() => {
    if (input === '') autoResize(textareaEl);
  });

  const actionBtnClass =
    'w-9 h-9 max-sm:w-8 max-sm:h-8 p-0 rounded-xl flex items-center justify-center flex-shrink-0 select-none ' +
    'transition-all active:scale-95 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50';
  const iconClass = 'material-symbols-outlined pointer-events-none';
  const iconStyle = 'font-size: 20px; line-height: 1;';

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey && !e.isComposing) {
      e.preventDefault();
      if (input.trim()) saveToHistory(input);
      onSend?.();
      return;
    }
    if (e.key === 'ArrowUp' && inputHistory.length > 0 && !e.shiftKey && !e.altKey) {
      const el = textareaEl;
      const atStart = !el || el.selectionStart === 0;
      if (input === '' || atStart) {
        e.preventDefault();
        historyIndex = Math.min(historyIndex + 1, inputHistory.length - 1);
        onInputChange?.(inputHistory[historyIndex] ?? '');
      }
    } else if (e.key === 'ArrowDown' && historyIndex >= 0 && !e.shiftKey && !e.altKey) {
      e.preventDefault();
      historyIndex = historyIndex - 1;
      onInputChange?.(historyIndex >= 0 ? inputHistory[historyIndex] : '');
    }
  }

  // Close context dropdown on outside click
  function handleOutsideClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest('.context-bar')) {
      showContextDropdown = false;
    }
  }

  $effect(() => {
    if (showContextDropdown) {
      document.addEventListener('click', handleOutsideClick);
      return () => document.removeEventListener('click', handleOutsideClick);
    }
  });
</script>

<div class="border-t border-[var(--color-border)] bg-[var(--color-bg-elevated)] ai-input-area">
  <!-- Project Context Bar -->
  <div class="context-bar flex items-center gap-2 px-3 pt-2 pb-1">
    <button
      class="flex items-center gap-1.5 px-2 py-1 rounded-lg text-xs font-medium transition-all {showProjectContext ? 'bg-primary-500/10 text-primary-500' : 'text-[var(--color-text-muted)] hover:bg-[var(--color-surface)] hover:text-[var(--color-text-secondary)]'}"
      onclick={() => {
        if (onToggleProjectContext) {
          onToggleProjectContext();
          showContextDropdown = !showProjectContext;
        }
      }}
      title="项目上下文"
    >
      <span class="material-symbols-outlined text-[14px]">folder</span>
      {#if selectedProject}
        <span class="max-w-[120px] truncate">{contextProjectList.find(p => p.id === selectedProject)?.name || selectedProject.slice(0, 8)}</span>
      {:else}
        <span>项目</span>
      {/if}
    </button>
    {#if selectedProject && contextProjects.length > 0}
      <button
        class="flex items-center gap-1 px-2 py-1 rounded-lg text-xs transition-all text-[var(--color-text-muted)] hover:bg-[var(--color-surface)] hover:text-[var(--color-text-secondary)]"
        onclick={() => showContextDropdown = !showContextDropdown}
        title="选择文件"
      >
        <span class="material-symbols-outlined text-[12px]">description</span>
        <span>{contextProjects.reduce((sum, cp) => sum + cp.files.length, 0)} 文件</span>
        <span class="material-symbols-outlined text-[10px]">expand_more</span>
      </button>
    {/if}
    {#if projectContext}
      <span class="px-1.5 py-0.5 rounded text-[10px] font-medium" style="background: var(--color-primary-light); color: var(--color-primary);">
        自定义上下文
      </span>
    {/if}
  </div>

  <!-- Context Dropdown -->
  {#if showContextDropdown && showProjectContext}
    <div class="px-3 pb-2 context-bar">
      <div class="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] p-2 space-y-2">
        <!-- Project selector -->
        <div>
          <label class="text-[10px] font-medium text-[var(--color-text-muted)] mb-1 block">项目</label>
          <select class="w-full input-field text-xs" value={selectedProject} onchange={(e) => onProjectChange?.((e.target as HTMLSelectElement).value)}>
            <option value="">选择项目...</option>
            {#each contextProjectList as p}
              <option value={p.id}>{p.name}</option>
            {/each}
          </select>
        </div>
        <!-- File selector -->
        {#if contextProjects.length > 0}
          <div>
            <label class="text-[10px] font-medium text-[var(--color-text-muted)] mb-1 block">添加文件</label>
            <select class="w-full input-field text-xs" value={selectedFile} onchange={(e) => { const v = (e.target as HTMLSelectElement).value; if (v) { onFileAdd?.(v); } }}>
              <option value="">选择文件添加到上下文...</option>
              {#each contextProjects as cp}
                {#each cp.files as f}
                  <option value={f}>{cp.name ? cp.name + ' / ' : ''}{f}</option>
                {/each}
              {/each}
            </select>
          </div>
        {/if}
        <!-- Custom context -->
        <div>
          <label class="text-[10px] font-medium text-[var(--color-text-muted)] mb-1 block">额外上下文</label>
          <textarea class="input-field text-xs font-mono resize-none w-full" rows="2" placeholder="补充上下文信息..." value={projectContext} oninput={(e) => onContextChange?.((e.target as HTMLTextAreaElement).value)}></textarea>
        </div>
      </div>
    </div>
  {/if}

  <!-- Mode-specific banners -->
  <div class="px-3 pb-1">
    {#if mode === 'generate'}
      <div class="flex items-center gap-2">
        <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium" style="background: var(--color-primary-light); color: var(--color-primary)">
          <span class="material-symbols-outlined text-[12px]">hub</span>
          Universal · Magisk + KSU + APatch
        </span>
      </div>
    {/if}
    {#if mode === 'repair'}
      <textarea
        class="input-field text-xs font-mono resize-none w-full"
        rows="2"
        placeholder="粘贴构建日志（可选）"
        value={buildLog}
        oninput={(e) => onBuildLogChange?.((e.target as HTMLTextAreaElement).value)}
      ></textarea>
    {/if}
  </div>

  <!-- Input row -->
  <div class="flex items-end gap-2 px-3 pb-2 input-row">
    <button
      class="{actionBtnClass} relative mb-0.5"
      style="color: var(--color-text-secondary); background: var(--color-surface); border: 1px solid var(--color-border);"
      onclick={onOpenMcpTools}
      title="MCP 工具面板"
      aria-label="打开 MCP 工具面板"
    >
      <span class="{iconClass}" style="{iconStyle}">hub</span>
      {#if mcpToolCount > 0}
        <span class="absolute -top-1 -right-1 text-[8px] font-bold px-1 py-0.5 rounded-full min-w-[14px] text-center"
              style="background: var(--color-success); color: white;">{mcpToolCount > 99 ? '99+' : mcpToolCount}</span>
      {/if}
    </button>
    <textarea
      class="flex-1 input-field resize-none"
      rows="2"
      style="min-height: 48px;"
      placeholder={mode === 'auto-build' ? '描述你想要创建的模块，AI 将自动完成开发全流程...' : mode === 'generate' ? '描述你的通用模块功能...' : mode === 'repair' ? '描述问题...' : mode === 'gather' ? '描述你的模块想法（如：我想做个电量管理模块）...' : '输入消息...'}
      value={input}
      oninput={(e) => { onInputChange?.((e.target as HTMLTextAreaElement).value); autoResize(e.target as HTMLTextAreaElement); }}
      onkeydown={handleKeydown}
      bind:this={textareaEl}
    ></textarea>
    {#if streaming}
      <button class="{actionBtnClass} mb-0.5" onclick={onStop} style="background: var(--color-error-light); color: var(--color-error);" title="停止生成" aria-label="停止生成">
        <span class="{iconClass}" style="{iconStyle}">stop_circle</span>
      </button>
    {:else}
      <button
        class="{actionBtnClass} bg-primary-600 text-white hover:bg-primary-700 disabled:opacity-40 disabled:cursor-not-allowed mb-0.5"
        onclick={() => onSend?.()}
        disabled={!input.trim()}
        title="发送 (Enter)"
        aria-label="发送"
      >
        <span class="{iconClass}" style="{iconStyle}">send</span>
      </button>
    {/if}
  </div>

  <!-- Bottom hints -->
  <div class="hidden sm:flex items-center justify-between px-3 pb-2">
    <span class="text-[9px] text-[var(--color-text-muted)] opacity-60">Enter 发送 · Shift+Enter 换行 · ↑ 历史</span>
    <span class="text-[9px] text-[var(--color-text-muted)] opacity-60" style="opacity: 0.45;">{input.length} 字符</span>
  </div>
</div>

<script lang="ts">
  import ChatMessage from './ChatMessage.svelte';
  import type { Message, TokenUsage } from '../lib/types';

  let {
    messages = $bindable([] as Message[]),
    mode = 'generate' as string,
    streaming = false,
    expandedReasoning = new Set<number>(),
    messageUsages = new Map<number, TokenUsage>(),
    messageTimes = new Map<number, number>(),
    // Agent inline steps
    allAgentSteps = [] as any[],
    agentExpandedSteps = new Set<number>(),
    onToggleAgentStep = (idx: number) => {},
    onToggleReasoning = (idx: number) => {},
    onEdit = (idx: number) => {},
    onDelete = (idx: number) => {},
    onReply = (idx: number) => {},
    onCopy = (text: string) => {},
    onOpenImportDialog = () => {},
    onOpenPreview = (files: { path: string; content: string }[]) => {},
    onInsertToInput = (text: string) => {},
    hasMoreMessages = false,
    loadingEarlier = false,
    onLoadEarlier = () => {},
    showSearch = false,
    onSearchClose = () => {},
  } = $props();

  // Helper: get steps for a specific message round
  function getStepsForRound(round: number | undefined): any[] {
    if (round === undefined) return [];
    return allAgentSteps.filter((s: any) => s.round === round);
  }

  // Virtual scroll state
  let scrollTop = $state(0);
  let containerHeight = $state(0);
  let containerEl: HTMLDivElement | undefined = $state();
  let chatEnd: HTMLDivElement | undefined = $state();

  const ITEM_HEIGHT = 80;
  const OVERSCAN = 5;
  let virtualStart = $derived(Math.max(0, Math.floor(scrollTop / ITEM_HEIGHT) - OVERSCAN));
  let virtualEnd = $derived(Math.min(messages.length, Math.ceil((scrollTop + containerHeight) / ITEM_HEIGHT) + OVERSCAN));
  let virtualMessages = $derived(messages.slice(virtualStart, virtualEnd));
  let virtualSpacerTop = $derived(virtualStart * ITEM_HEIGHT);
  let virtualSpacerBottom = $derived((messages.length - virtualEnd) * ITEM_HEIGHT);

  // Auto-scroll when new messages are added — but only follow when the user
  // is already near the bottom (don't yank them away while reading history).
  let stickToBottom = $state(true);
  let lastMsgCount = $state(0);

  $effect(() => {
    const count = messages.length;
    if (count > lastMsgCount && containerEl && stickToBottom) {
      lastMsgCount = count;
      requestAnimationFrame(() => {
        if (containerEl) {
          containerEl.scrollTop = containerEl.scrollHeight;
        }
      });
    }
    lastMsgCount = count;
  });

  // Keep following while streaming if the user hasn't scrolled away
  let wasStreaming = $state(false);
  $effect(() => {
    if (streaming && !wasStreaming) stickToBottom = true; // new stream starts → follow
    wasStreaming = streaming;
  });
  $effect(() => {
    const last = messages[messages.length - 1];
    if (streaming && stickToBottom && containerEl && last) {
      void last.content; // track streaming content changes to keep scrolling
      requestAnimationFrame(() => {
        if (containerEl) {
          containerEl.scrollTop = containerEl.scrollHeight;
        }
      });
    }
  });

  function handleScroll(e: Event) {
    const el = e.currentTarget as HTMLElement;
    scrollTop = el.scrollTop;
    const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    stickToBottom = distanceToBottom < 120;
  }

  // Exposed for parent to call after sending a message
  export function scrollToBottom() {
    const el = document.querySelector('.messages-area') as HTMLElement;
    if (el) requestAnimationFrame(() => { el.scrollTop = el.scrollHeight; });
  }

  // 回到最新：用户上滚阅读历史时显示浮动按钮，点击一键回到底部并恢复跟随
  function jumpToLatest() {
    if (!containerEl) return;
    stickToBottom = true;
    containerEl.scrollTop = containerEl.scrollHeight;
  }

  // ─── 会话内全文搜索（本地遍历 + 虚拟滚动定位跳转 + 短暂高亮）───
  let searchOpen = $state(false);
  
  // Sync external search toggle
  $effect(() => {
    if (showSearch && !searchOpen) {
      searchOpen = true;
    } else if (!showSearch && searchOpen) {
      closeSearch();
    }
  });
  let searchQuery = $state('');
  let searchResults = $state<number[]>([]);
  let searchIdx = $state(-1);
  let highlightIdx = $state(-1);
  let highlightTimer: ReturnType<typeof setTimeout> | undefined;

  function runSearch(q: string) {
    searchQuery = q;
    const needle = q.trim().toLowerCase();
    if (!needle) {
      searchResults = [];
      searchIdx = -1;
      return;
    }
    const hits: number[] = [];
    messages.forEach((m, i) => {
      if ((m.content || '').toLowerCase().includes(needle) || (m.reasoning || '').toLowerCase().includes(needle)) hits.push(i);
    });
    searchResults = hits;
    searchIdx = hits.length > 0 ? 0 : -1;
    if (searchIdx >= 0) gotoSearchHit();
  }

  function gotoSearchHit() {
    if (searchIdx < 0 || searchIdx >= searchResults.length || !containerEl) return;
    const idx = searchResults[searchIdx];
    // 虚拟滚动按 80px 行高近似定位（定位到命中消息附近）
    containerEl.scrollTop = Math.max(0, idx * ITEM_HEIGHT - 80);
    stickToBottom = false;
    highlightIdx = idx;
    if (highlightTimer) clearTimeout(highlightTimer);
    highlightTimer = setTimeout(() => { highlightIdx = -1; }, 2200);
  }

  function prevSearchHit() {
    if (searchResults.length === 0) return;
    searchIdx = searchIdx <= 0 ? searchResults.length - 1 : searchIdx - 1;
    gotoSearchHit();
  }
  function nextSearchHit() {
    if (searchResults.length === 0) return;
    searchIdx = searchIdx >= searchResults.length - 1 ? 0 : searchIdx + 1;
    gotoSearchHit();
  }

  export function closeSearch() {
    searchOpen = false;
    searchQuery = '';
    searchResults = [];
    searchIdx = -1;
    highlightIdx = -1;
    if (highlightTimer) { clearTimeout(highlightTimer); highlightTimer = undefined; }
    onSearchClose();
  }
</script>

<div class="relative flex-1 min-h-0 flex flex-col">
  {#if !searchOpen && messages.length > 0}
    <!-- Search button moved to CompactToolbar -->
  {/if}
  {#if searchOpen}
    <div class="px-3 py-2 border-b border-[var(--color-border)] flex items-center gap-2 bg-[var(--color-bg)] z-10 flex-shrink-0">
      <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]" style="font-size: 14px; line-height: 1;">search</span>
      <input
        class="flex-1 min-w-0 text-xs bg-transparent text-[var(--color-text)] outline-none placeholder-[var(--color-text-muted)]"
        placeholder="搜索此会话..."
        value={searchQuery}
        oninput={(e) => runSearch((e.target as HTMLInputElement).value)}
        onkeydown={(e) => {
          if (e.key === 'Enter') { e.preventDefault(); if (e.shiftKey) prevSearchHit(); else nextSearchHit(); }
          if (e.key === 'Escape') closeSearch();
        }}
        autofocus
      />
      {#if searchResults.length > 0}
        <span class="text-[10px] text-[var(--color-text-muted)] flex-shrink-0">{searchIdx + 1}/{searchResults.length}</span>
      {:else if searchQuery.trim()}
        <span class="text-[10px] text-[var(--color-error)] flex-shrink-0">无匹配</span>
      {/if}
      <button class="p-1 rounded hover:bg-[var(--color-surface)] flex-shrink-0" onclick={() => prevSearchHit()} title="上一个 (Shift+Enter)" disabled={searchResults.length === 0}>
        <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]" style="font-size: 14px; line-height: 1;">arrow_upward</span>
      </button>
      <button class="p-1 rounded hover:bg-[var(--color-surface)] flex-shrink-0" onclick={() => nextSearchHit()} title="下一个 (Enter)" disabled={searchResults.length === 0}>
        <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]" style="font-size: 14px; line-height: 1;">arrow_downward</span>
      </button>
      <button class="p-1 rounded hover:bg-[var(--color-surface)] flex-shrink-0" onclick={closeSearch} title="关闭 (Esc)">
        <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]" style="font-size: 14px; line-height: 1;">close</span>
      </button>
    </div>
  {/if}
  <div class="flex-1 overflow-y-auto px-3 py-1.5 space-y-1.5 messages-area"
    onscroll={handleScroll}
    bind:this={containerEl}
    bind:clientHeight={containerHeight}>
    {#if hasMoreMessages}
      <div class="flex justify-center py-2">
        <button
          class="text-xs px-3 py-1.5 rounded-full border border-[var(--color-border)] text-[var(--color-text-muted)] hover:text-[var(--color-text)] hover:bg-[var(--color-surface)] transition-colors disabled:opacity-50"
          onclick={() => onLoadEarlier()}
          disabled={loadingEarlier}
        >
          {loadingEarlier ? '加载中...' : '加载更早消息'}
        </button>
      </div>
    {/if}
    {#if messages.length === 0}
      <div class="flex items-center justify-center h-full">
        <div class="text-center">
          <div class="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4" style="background: var(--gradient-brand-subtle)">
            <span class="material-symbols-outlined text-3xl" style="color: var(--color-primary)">psychology</span>
          </div>
          <p class="text-lg font-semibold text-[var(--color-text)]">AI 助手</p>
          <p class="text-sm text-[var(--color-text-muted)] mt-1">开始对话，或者选择一种模式来生成模块、修复构建错误等</p>
        </div>
      </div>
    {:else}
      {#if virtualSpacerTop > 0}<div style="height:{virtualSpacerTop}px"></div>{/if}
      {#each virtualMessages as msg, i (virtualStart + i + '-' + msg.role)}
        {@const msgSteps = getStepsForRound(msg.round)}
        <ChatMessage {msg} index={virtualStart + i} {mode} {streaming} {expandedReasoning} {messageUsages} {messageTimes}
          showTypingCursor={streaming && virtualStart + i === messages.length - 1 && msg.role === 'assistant'}
          highlighted={highlightIdx === virtualStart + i}
          agentSteps={msgSteps} {agentExpandedSteps} onToggleAgentStep={(idx) => onToggleAgentStep(idx)}
          onToggleReasoning={(idx) => onToggleReasoning(idx)}
          onEdit={(idx) => onEdit(idx)}
          onDelete={(idx) => onDelete(idx)}
          onReply={(idx) => onReply(idx)}
          onCopy={(text) => onCopy(text)}
          onOpenImportDialog={() => onOpenImportDialog()}
          onOpenPreview={(files) => onOpenPreview(files)}
          onInsertToInput={(text) => onInsertToInput(text)}
        />
      {/each}
      {#if virtualSpacerBottom > 0}<div style="height:{virtualSpacerBottom}px"></div>{/if}
      {#if streaming && messages[messages.length - 1].role !== 'assistant'}
        <div class="flex items-start gap-2 py-1.5">
          <div class="w-6 h-6 rounded-full flex items-center justify-center flex-shrink-0" style="background: var(--gradient-brand-subtle)">
            <span class="material-symbols-outlined" style="color: var(--color-primary); font-size: 13px; line-height: 1;">psychology</span>
          </div>
          <div class="px-3 py-2 rounded-2xl text-[13px] flex items-center gap-1.5" style="background: var(--color-surface); border: 1px solid var(--color-border); color: var(--color-text-secondary);">
            <span>正在思考</span>
            <span class="thinking-dots"><span>.</span><span>.</span><span>.</span></span>
          </div>
        </div>
      {/if}
      <div bind:this={chatEnd}></div>
    {/if}
  </div>

  {#if !stickToBottom && messages.length > 0}
    <button
      class="absolute bottom-4 right-4 z-10 flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium shadow-lg backdrop-blur transition-all hover:scale-105 active:scale-95"
      style="background: var(--color-bg-elevated); border: 1px solid var(--color-border); color: var(--color-text-secondary); box-shadow: 0 4px 16px rgba(0,0,0,0.15);"
      onclick={jumpToLatest}
      title="回到最新消息"
    >
      <span class="material-symbols-outlined text-[14px]" style="font-size: 14px; line-height: 1;">keyboard_arrow_down</span>
      <span>回到最新</span>
    </button>
  {/if}
</div>

<style>
  .thinking-dots span {
    display: inline-block;
    animation: thinking-bounce 1.2s ease-in-out infinite;
  }
  .thinking-dots span:nth-child(2) { animation-delay: 0.15s; }
  .thinking-dots span:nth-child(3) { animation-delay: 0.3s; }
  @keyframes thinking-bounce {
    0%, 60%, 100% { opacity: 0.3; transform: translateY(0); }
    30% { opacity: 1; transform: translateY(-2px); }
  }
</style>
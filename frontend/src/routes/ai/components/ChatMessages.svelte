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
  let lastMsgCount = $state(0);

  const ITEM_HEIGHT = 80;
  const OVERSCAN = 5;
  let virtualStart = $derived(Math.max(0, Math.floor(scrollTop / ITEM_HEIGHT) - OVERSCAN));
  let virtualEnd = $derived(Math.min(messages.length, Math.ceil((scrollTop + containerHeight) / ITEM_HEIGHT) + OVERSCAN));
  let virtualMessages = $derived(messages.slice(virtualStart, virtualEnd));
  let virtualSpacerTop = $derived(virtualStart * ITEM_HEIGHT);
  let virtualSpacerBottom = $derived((messages.length - virtualEnd) * ITEM_HEIGHT);

  // Auto-scroll when new messages are added
  $effect(() => {
    const count = messages.length;
    if (count > lastMsgCount && containerEl) {
      lastMsgCount = count;
      requestAnimationFrame(() => {
        if (containerEl) {
          containerEl.scrollTop = containerEl.scrollHeight;
        }
      });
    }
    lastMsgCount = count;
  });

  // Exposed for parent to call after sending a message
  export function scrollToBottom() {
    const el = document.querySelector('.messages-area') as HTMLElement;
    if (el) requestAnimationFrame(() => { el.scrollTop = el.scrollHeight; });
  }
</script>

<div class="flex-1 overflow-y-auto px-3 py-1.5 space-y-1.5 messages-area"
  onscroll={(e) => { scrollTop = e.currentTarget.scrollTop; }}
  bind:this={containerEl}
  bind:clientHeight={containerHeight}>
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
    <div bind:this={chatEnd}></div>
  {/if}
</div>
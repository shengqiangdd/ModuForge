<script lang="ts">
  import CodeBlock from './CodeBlock.svelte';
  import AgentStepsInline from './AgentStepsInline.svelte';

  type TokenUsage = {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
  };

  type Message = {
    role: string;
    content: string;
    round?: number;
    reasoning?: string;
    created_at?: string;
  };

  type Segment = { type: 'text'; content: string } | { type: 'code'; language: string; content: string };
  type RecFile = { path: string; required: boolean; description: string };
  type ErrDetail = { message: string; suggestion: string };

  let {
    msg,
    index,
    lastAssistantIdx = -1,
    streaming = false,
    mode = 'generate',
    selectedModel = null as any,
    expandedReasoning = new Set<number>(),
    editingMessageIdx = -1,
    editingMessageText = '',
    codeBlocksCollapsed = new Map<string, boolean>(),
    memoRenderMarkdown = (text: string) => text,
    memoParseSegments = (text: string): Segment[] => [{ type: 'text', content: text }],
    memoExtractFiles = (content: string): { path: string; content: string }[] | null => null,
    memoExtractRecFiles = (content: string): RecFile[] | null => null,
    memoParseErrorDetail = (content: string): ErrDetail | null => null,
    memoCheckWebUI = (files: { path: string; content: string }[]): boolean => false,
    cleanRecommendedContent = (content: string) => content,
    // Agent inline steps
    agentSteps = [] as any[],
    agentExpandedSteps = new Set<number>(),
    onToggleAgentStep = (idx: number) => {},
    onRetry,
    onInsertToInput,
    onCopy,
    onInsertFile,
    onInsertRecFile,
    onEdit,
    onEditingTextChange,
    onSaveEdit,
    onCancelEdit,
    onDelete,
    onReply,
    onToggleReasoning,
    onToggleCodeCollapse,
    onOpenPreview,
    onOpenImportDialog,
    onHandleMsgClick,
    messageUsages = new Map<number, TokenUsage>(),
    messageTimes = new Map<number, number>(),
    showTypingCursor = false,
  }: {
    msg: Message;
    index: number;
    lastAssistantIdx?: number;
    streaming?: boolean;
    mode?: string;
    selectedModel?: any;
    expandedReasoning?: Set<number>;
    editingMessageIdx?: number;
    editingMessageText?: string;
    codeBlocksCollapsed?: Map<string, boolean>;
    memoRenderMarkdown?: (text: string) => string;
    memoParseSegments?: (text: string) => Segment[];
    memoExtractFiles?: (content: string) => { path: string; content: string }[] | null;
    memoExtractRecFiles?: (content: string) => RecFile[] | null;
    memoParseErrorDetail?: (content: string) => ErrDetail | null;
    memoCheckWebUI?: (files: { path: string; content: string }[]) => boolean;
    cleanRecommendedContent?: (content: string) => string;
    agentSteps?: any[];
    agentExpandedSteps?: Set<number>;
    onToggleAgentStep?: (idx: number) => void;
    onRetry?: () => void;
    onInsertToInput?: (text: string) => void;
    onCopy?: (text: string) => void;
    onInsertFile?: (file: { path: string; content: string }) => void;
    onInsertRecFile?: (file: RecFile) => void;
    onEdit?: (index: number) => void;
    onEditingTextChange?: (text: string) => void;
    onSaveEdit?: () => void;
    onCancelEdit?: () => void;
    onDelete?: (index: number) => void;
    onReply?: (index: number) => void;
    onToggleReasoning?: (index: number) => void;
    onToggleCodeCollapse?: (key: string, lineCount: number) => void;
    onOpenPreview?: (files: { path: string; content: string }[]) => void;
    onOpenImportDialog?: (index: number) => void;
    onHandleMsgClick?: (msg: Message) => void;
    messageUsages?: Map<number, TokenUsage>;
    messageTimes?: Map<number, number>;
    showTypingCursor?: boolean;
  } = $props();

  let isEditing = $derived(editingMessageIdx === index);
  let usage = $derived(messageUsages.get(index));
  let respTime = $derived(messageTimes.get(index));
  let relTime = $derived(formatRelativeTime(msg.created_at));

  function formatRelativeTime(ts?: string): string {
    if (!ts) return '';
    const t = new Date(ts).getTime();
    if (isNaN(t)) return '';
    const diff = Date.now() - t;
    if (diff < 0) return '';
    if (diff < 60_000) return '刚刚';
    if (diff < 3600_000) return Math.floor(diff / 60_000) + ' 分钟前';
    if (diff < 86_400_000) return Math.floor(diff / 3600_000) + ' 小时前';
    return new Date(t).toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
  }

  function handleCopy(text: string) {
    onCopy?.(text);
  }
</script>

<div class="flex {msg.role === 'user' ? 'justify-end' : 'justify-start'}">
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="{msg.role === 'user' ? 'max-w-[85%] sm:max-w-[75%]' : 'max-w-lg sm:max-w-xl md:max-w-2xl'} rounded-2xl text-[13px] leading-snug msg-bubble group
    {msg.role === 'user'
      ? 'bg-primary-600 text-white rounded-br-md px-2 py-0.5'
      : 'bg-[var(--color-surface)] text-[var(--color-text)] border border-[var(--color-border)] rounded-bl-md px-3 py-1.5'}
    {mode === 'agent' && msg.role === 'user' && msg.round !== undefined ? 'cursor-pointer hover:opacity-90' : ''}"
    title={mode === 'agent' && msg.role === 'user' && msg.round !== undefined ? '点击查看本轮步骤' : ''}
    onclick={() => onHandleMsgClick?.(msg)}>
    {#if msg.role === 'assistant'}
      <div class="flex items-center gap-1 mb-0.5">
        <span class="material-symbols-outlined text-primary-500 text-[14px]">auto_awesome</span>
        <span class="text-xs font-medium text-primary-600">AI</span>
        {#if selectedModel}
          <span class="text-[9px] px-1 py-0.5 rounded text-[var(--color-text-muted)]" style="background: var(--color-surface); border: 1px solid var(--color-border);">{selectedModel.name}</span>
        {/if}
        {#if relTime}
          <span class="text-[9px] text-[var(--color-text-muted)]" title={msg.created_at ? new Date(msg.created_at).toLocaleString('zh-CN') : ''}>{relTime}</span>
        {/if}
        <button
          class="ml-auto p-1 rounded opacity-0 group-hover:opacity-100 hover:bg-[var(--color-surface)] transition-all"
          onclick={() => handleCopy(msg.content)}
          title="复制全部内容"
        >
          <span class="material-symbols-outlined text-[13px] text-[var(--color-text-muted)]">content_copy</span>
        </button>
      </div>
      {#if msg.reasoning}
        <div class="mb-2 rounded-xl overflow-hidden border border-[var(--color-border)]">
          <button
            class="flex items-center gap-1.5 w-full px-3 py-1.5 text-xs font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors"
            onclick={() => onToggleReasoning?.(index)}
          >
            <span class="material-symbols-outlined text-[14px]">psychology</span>
            思考过程
            <span class="material-symbols-outlined text-[14px] ml-auto transition-transform {expandedReasoning.has(index) ? 'rotate-180' : ''}">expand_more</span>
          </button>
          {#if expandedReasoning.has(index)}
            <div class="px-3 py-2 text-xs leading-relaxed whitespace-pre-wrap" style="color: var(--color-text-muted); background: var(--color-surface); border-top: 1px solid var(--color-border);">
              {msg.reasoning}
            </div>
          {/if}
        </div>
      {/if}
      {@const _msgSegments = memoParseSegments(cleanRecommendedContent(msg.content))}
      {@const _recFiles = memoExtractRecFiles(msg.content)}
      {#if _recFiles}
        <div class="mt-2 mb-2">
          <p class="text-xs font-medium text-[var(--color-text-secondary)] mb-1.5">推荐文件清单：</p>
          <div class="space-y-1">
            {#each _recFiles as rf}
              <div class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-xs" style="background: var(--color-surface);">
                <span class="material-symbols-outlined text-[14px] {rf.required ? 'text-primary-500' : 'text-[var(--color-text-muted)]'}">
                  {rf.required ? 'required' : 'check_circle'}
                </span>
                <code class="font-mono font-medium text-[var(--color-text)]">{rf.path}</code>
                {#if rf.description}
                  <span class="text-[var(--color-text-muted)] ml-1">— {rf.description}</span>
                {/if}
                {#if rf.required}
                  <span class="ml-auto px-1.5 py-0.5 rounded text-[10px] font-medium" style="background: var(--color-primary-light); color: var(--color-primary)">必需</span>
                {:else}
                  <span class="ml-auto px-1.5 py-0.5 rounded text-[10px] font-medium" style="background: var(--color-surface); color: var(--color-text-muted)">可选</span>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/if}
      {@const _errDetail = memoParseErrorDetail(msg.content)}
      {#if _errDetail}
        <div class="p-3 rounded-lg text-xs" style="background: color-mix(in srgb, var(--color-error, #ef4444) 8%, var(--color-bg)); border: 1px solid color-mix(in srgb, var(--color-error, #ef4444) 20%, transparent);">
          <div class="flex items-center gap-1.5 mb-1">
            <span class="material-symbols-outlined text-[14px]" style="color: var(--color-error, #ef4444)">error_outline</span>
            <span class="font-medium" style="color: var(--color-error, #ef4444)">生成失败</span>
          </div>
          <p class="text-[var(--color-text)]">{_errDetail.message}</p>
          {#if _errDetail.suggestion}
            <p class="mt-1 text-[var(--color-text-secondary)]">{_errDetail.suggestion}</p>
          {/if}
        </div>
      {:else}
        <div class="space-y-2">
          {#each _msgSegments as seg, si}
            {#if seg.type === 'code'}
              {@const _codeKey = 'msg-' + index + '-seg-' + si}
              {@const _lineCount = seg.content.split('\n').length}
              <CodeBlock
                language={seg.language}
                content={seg.content}
                codeKey={_codeKey}
                lineCount={_lineCount}
                collapsed={codeBlocksCollapsed.get(_codeKey) ?? (!codeBlocksCollapsed.has(_codeKey) && _lineCount > 50)}
                onCopy={handleCopy}
              />
            {:else}
              <div class="ai-markdown max-w-none text-[var(--color-text)]">
                {@html memoRenderMarkdown(seg.content)}
              </div>
            {/if}
          {/each}
          {#if showTypingCursor}
            <span class="typing-cursor" aria-hidden="true"></span>
          {/if}
        </div>
        {@const _files = memoExtractFiles(msg.content)}
        {#if _files}
          {@const _hasWebUI = memoCheckWebUI(_files)}
          <div class="mt-2 flex flex-wrap gap-2">
            <button
              class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
              style="background: var(--color-surface); color: var(--color-text); border: 1px solid var(--color-border);"
              onclick={() => onOpenPreview?.(_files)}
            >
              <span class="material-symbols-outlined text-[14px]">folder_open</span>
              一键预览
            </button>
            {#if _hasWebUI}
              <button
                class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
                style="background: var(--color-surface); color: var(--color-primary); border: 1px solid var(--color-primary);"
                onclick={() => onOpenPreview?.(_files)}
              >
                <span class="material-symbols-outlined text-[14px]">web</span>
                预览 WebUI
              </button>
            {/if}
            <button
              class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
              style="background: var(--color-primary-light); color: var(--color-primary)"
              onclick={() => onOpenImportDialog?.(index)}
            >
              <span class="material-symbols-outlined text-[14px]">download</span>
              导入到项目
            </button>
          </div>
        {/if}
      {/if}
    {:else}
      {#if mode === 'agent' && msg.round !== undefined}
        <div class="flex items-center gap-1 mb-1 opacity-60">
          <span class="text-[9px] px-1 py-0.5 rounded" style="background: rgba(255,255,255,0.15);">轮次 {msg.round + 1}</span>
        </div>
      {/if}
      <div class="ai-markdown" role="article">{@html memoRenderMarkdown(msg.content)}</div>
    {/if}
    {#if msg.role === 'assistant' && agentSteps.length > 0}
      <AgentStepsInline steps={agentSteps} expandedSteps={agentExpandedSteps} onToggleStep={(idx) => onToggleAgentStep(idx)} />
    {/if}
    {#if msg.role === 'assistant' && (usage || respTime)}
      <div class="flex items-center gap-3 mt-2 pt-2 border-t border-[var(--color-border)] text-[10px] text-[var(--color-text-muted)]">
        {#if usage}
          <span class="flex items-center gap-1">
            <span class="material-symbols-outlined text-[10px]">token</span>
            输入 {usage?.prompt_tokens?.toLocaleString() || 0}
          </span>
          <span class="flex items-center gap-1">
            <span class="material-symbols-outlined text-[10px]">output</span>
            输出 {usage?.completion_tokens?.toLocaleString() || 0}
          </span>
          <span class="font-medium text-[var(--color-text-secondary)]">
            共 {usage?.total_tokens?.toLocaleString() || 0} tokens
          </span>
        {/if}
        {#if respTime}
          <span class="flex items-center gap-1" title="响应耗时">
            <span class="material-symbols-outlined text-[10px]">schedule</span>
            用时 {respTime >= 1000 ? (respTime / 1000).toFixed(1) + 's' : respTime + 'ms'}
          </span>
        {/if}
      </div>
    {/if}
    <div class="flex items-center gap-1 mt-1.5 opacity-0 hover:opacity-100 transition-opacity">
      {#if msg.role === 'assistant'}
        {#if index === lastAssistantIdx && !streaming}
          <button class="flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] text-[var(--color-text-muted)] hover:bg-[var(--color-surface)] transition-colors" onclick={onRetry} title="重新生成">
            <span class="material-symbols-outlined text-[10px]">refresh</span>
            重新生成
          </button>
        {/if}
      {:else}
        {#if isEditing}
          <div class="flex items-center gap-1">
            <input class="flex-1 px-2 py-1 rounded text-xs border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)]" value={editingMessageText} oninput={(e) => onEditingTextChange?.((e.target as HTMLInputElement).value)} onkeydown={(e) => { if (e.key === 'Enter') onSaveEdit?.(); if (e.key === 'Escape') onCancelEdit?.(); }} />
            <button class="p-0.5 rounded text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]" onclick={onSaveEdit}><span class="material-symbols-outlined text-[12px]">check</span></button>
            <button class="p-0.5 rounded text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]" onclick={onCancelEdit}><span class="material-symbols-outlined text-[12px]">close</span></button>
          </div>
        {:else}
          <button class="flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] text-[var(--color-text-muted)] hover:bg-[var(--color-surface)] transition-colors" onclick={() => onEdit?.(index)} title="编辑">
            <span class="material-symbols-outlined text-[10px]">edit</span>
          </button>
        {/if}
      {/if}
      <button class="flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] text-[var(--color-text-muted)] hover:bg-[var(--color-surface)] transition-colors" onclick={() => onReply?.(index)} title="引用回复">
        <span class="material-symbols-outlined text-[10px]">reply</span>
      </button>
      <button class="flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] text-[var(--color-text-muted)] hover:bg-[var(--color-surface)] transition-colors" onclick={() => onDelete?.(index)} title="删除">
        <span class="material-symbols-outlined text-[10px]">delete</span>
      </button>
    </div>
  </div>
</div>

<style>
  .typing-cursor {
    display: inline-block;
    width: 7px;
    height: 14px;
    margin-left: 2px;
    vertical-align: text-bottom;
    border-radius: 1px;
    background: var(--color-primary);
    animation: typing-blink 1s steps(2, start) infinite;
  }
  @keyframes typing-blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0; }
  }
</style>

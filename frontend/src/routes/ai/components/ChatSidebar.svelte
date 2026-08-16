<script lang="ts">
  type SessionInfo = {
    session_id: string;
    title: string;
    mode: string;
    model: string;
    msg_count: number;
    last_at: string;
  };

  type SavedConv = {
    id: string;
    title: string;
    mode: string;
    model: string;
    message_count: number;
    created_at: string;
    updated_at: string;
    token_usage?: number;
  };

  type GenHistoryItem = {
    id: string;
    title: string;
    timestamp: number;
    model: string;
    mode: string;
    messageCount: number;
    preview: string;
    messages?: { role: string; content: string }[];
  };

  let {
    sessions = [],
    savedConversations = [],
    genHistory = [],
    convSaving = false,
    convLoading = false,
    historyTab = 'conversations' as 'conversations' | 'generations',
    activeSessionId = '',
    searchResults = [] as { session_id: string; role: string; content: string; step_type: string; created_at: string }[],
    messagesLength = 0,
    onNewConversation,
    onRefresh,
    onSave,
    onClose,
    onTabChange,
    onSearch,
    onSelectConversation,
    onSelectSession,
    onDeleteSession,
    onExportSession,
    onDeleteConversation,
    onRestoreHistory,
    onClearHistory,
    onLoadMore,
    sessionsTotal = 0,
    sessionsLoading = false,
  }: {
    sessions?: SessionInfo[];
    savedConversations?: SavedConv[];
    genHistory?: GenHistoryItem[];
    convSaving?: boolean;
    convLoading?: boolean;
    historyTab?: 'conversations' | 'generations';
    activeSessionId?: string;
    searchResults?: { session_id: string; role: string; content: string; step_type: string; created_at: string }[];
    messagesLength?: number;
    onNewConversation?: () => void;
    onRefresh?: () => void;
    onSave?: () => void;
    onClose?: () => void;
    onTabChange?: (tab: 'conversations' | 'generations') => void;
    onSearch?: (query: string) => void;
    onSelectConversation?: (id: string) => void;
    onSelectSession?: (sessionId: string) => void;
    onDeleteSession?: (sessionId: string) => void;
    onExportSession?: (sessionId: string, format: 'markdown' | 'json') => void;
    onDeleteConversation?: (id: string) => void;
    onRestoreHistory?: (item: GenHistoryItem) => void;
    onClearHistory?: () => void;
    onLoadMore?: () => void;
    sessionsTotal?: number;
    sessionsLoading?: boolean;
  } = $props();

  let searchQuery = $state('');

  function formatTokens(t: number): string {
    if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(1)}M`;
    if (t >= 1000) return `${(t / 1000).toFixed(1)}K`;
    return `${t}`;
  }

  function handleSearch(e: Event) {
    const q = (e.target as HTMLInputElement).value.trim();
    searchQuery = q;
    onSearch?.(q);
  }
</script>

<div class="w-72 flex-shrink-0 border-r border-[var(--color-border)] bg-[var(--color-bg)] flex flex-col overflow-hidden history-sidebar">
  <!-- Sidebar Header -->
  <div class="flex items-center justify-between px-3 py-2 border-b border-[var(--color-border)]">
    <div class="flex items-center gap-1.5">
      <span class="material-symbols-outlined text-[14px] text-primary-500">history</span>
      <span class="text-xs font-semibold text-[var(--color-text)]">历史记录</span>
    </div>
    <div class="flex items-center gap-0.5">
      <button class="p-1 rounded hover:bg-primary-500/10 text-primary-500 transition-colors" onclick={onNewConversation} title="新对话">
        <span class="material-symbols-outlined text-[14px]">add_comment</span>
      </button>
      <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onRefresh} title="刷新">
        <span class="material-symbols-outlined text-[14px]">refresh</span>
      </button>
      <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onSave} disabled={convSaving || messagesLength === 0} title="保存当前对话">
        <span class="material-symbols-outlined text-[14px]">{convSaving ? 'progress_activity' : 'save'}</span>
      </button>
      <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
        <span class="material-symbols-outlined text-[14px]">close</span>
      </button>
    </div>
  </div>

  <!-- Tabs -->
  <div class="flex border-b border-[var(--color-border)]">
    <button
      class="flex-1 flex items-center justify-center gap-1 py-2 text-xs font-medium transition-colors {historyTab === 'conversations' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'}"
      onclick={() => onTabChange?.('conversations')}
    >
      <span class="material-symbols-outlined text-[13px]">chat</span>
      对话记录
    </button>
    <button
      class="flex-1 flex items-center justify-center gap-1 py-2 text-xs font-medium transition-colors {historyTab === 'generations' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'}"
      onclick={() => onTabChange?.('generations')}
    >
      <span class="material-symbols-outlined text-[13px]">auto_fix_high</span>
      生成历史
    </button>
  </div>

  <!-- Tab Content -->
  <div class="flex-1 overflow-y-auto p-2">
    {#if historyTab === 'conversations'}
      {#if convLoading}
        <div class="flex items-center justify-center py-6">
          <span class="material-symbols-outlined text-[20px] animate-spin text-primary-500">progress_activity</span>
        </div>
      {:else}
        {#if savedConversations.length > 0}
          <div class="mb-2">
            <p class="text-[10px] font-medium text-[var(--color-text-muted)] uppercase tracking-wider mb-1.5 px-1">保存的对话{#if sessionsTotal} · {sessionsTotal}{/if}</p>
            <div class="space-y-1">
              {#each savedConversations as conv (conv.id)}
                <div class="px-2 py-1.5 rounded-lg transition-colors hover:bg-[var(--color-surface)] group">
                  <div class="flex items-center gap-1.5">
                    <button class="flex-1 text-left min-w-0" onclick={() => onSelectConversation?.(conv.id)}>
                      <span class="text-xs font-medium text-[var(--color-text)] truncate block">{conv.title || '未命名'}</span>
                      <div class="flex items-center gap-1.5 mt-0.5">
                        <span class="text-[10px] px-1 py-0.5 rounded" style="background: var(--color-primary-light); color: var(--color-primary)">{conv.mode}</span>
                        <span class="text-[10px] text-[var(--color-text-muted)]">{conv.message_count}条</span>
                        {#if conv.token_usage}
                          <span class="text-[10px] text-[var(--color-text-muted)]" title="该会话累计 token 用量">{formatTokens(conv.token_usage)} tokens</span>
                        {/if}
                      </div>
                    </button>
                    <button class="p-0.5 rounded opacity-0 group-hover:opacity-100 hover:bg-[var(--color-surface)] transition-all" onclick={() => onDeleteConversation?.(conv.id)} title="删除">
                      <span class="material-symbols-outlined text-[12px] text-red-500">delete</span>
                    </button>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}
        <!-- Search Box -->
        <div class="mb-2">
          <div class="relative">
            <span class="material-symbols-outlined absolute left-2 top-1/2 -translate-y-1/2 text-[12px] text-[var(--color-text-muted)]">search</span>
            <input
              type="text"
              placeholder="搜索会话内容..."
              class="w-full pl-7 pr-2 py-1.5 text-[11px] rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] placeholder-[var(--color-text-muted)] focus:outline-none focus:border-primary-500"
              oninput={handleSearch}
            />
          </div>
          {#if searchResults.length > 0}
            <div class="mt-1.5 space-y-1 max-h-40 overflow-y-auto">
              {#each searchResults.slice(0, 10) as sr}
                <button class="w-full text-left px-2 py-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={() => onSelectSession?.(sr.session_id)}>
                  <span class="text-[10px] text-[var(--color-text-muted)]">{sr.role} · {new Date(sr.created_at).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                  <p class="text-[11px] text-[var(--color-text)] truncate">{sr.content}</p>
                </button>
              {/each}
            </div>
          {/if}
        </div>
        {#if sessions.length > 0}
          <div>
            <p class="text-[10px] font-medium text-[var(--color-text-muted)] uppercase tracking-wider mb-1.5 px-1">会话记录</p>
            <div class="space-y-1">
              {#each sessions as sess (sess.session_id)}
                <div class="px-2 py-1.5 rounded-lg transition-colors hover:bg-[var(--color-surface)] group {activeSessionId === sess.session_id ? 'bg-primary-500/5' : ''}">
                  <div class="flex items-center gap-1.5">
                    <button class="flex-1 text-left min-w-0" onclick={() => onSelectSession?.(sess.session_id)}>
                      <span class="text-xs font-medium text-[var(--color-text)] truncate block">{sess.title || '会话 ' + sess.session_id.slice(0, 8) + '...'}</span>
                      <div class="flex items-center gap-1.5 mt-0.5 flex-wrap">
                        {#if sess.mode}
                          <span class="text-[10px] px-1 py-0.5 rounded" style="background: var(--color-primary-light); color: var(--color-primary)">{sess.mode}</span>
                        {/if}
                        {#if sess.model}
                          <span class="text-[10px] px-1 py-0.5 rounded bg-[var(--color-surface)] text-[var(--color-text-muted)]">{sess.model.split('/').pop()}</span>
                        {/if}
                        <span class="text-[10px] text-[var(--color-text-muted)]">{sess.msg_count}条</span>
                        <span class="text-[10px] text-[var(--color-text-muted)]">{new Date(sess.last_at).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                      </div>
                    </button>
                    <div class="flex flex-col gap-0.5 opacity-0 group-hover:opacity-100 transition-all">
                      <button class="p-0.5 rounded hover:bg-[var(--color-surface)]" onclick={() => onExportSession?.(sess.session_id, 'markdown')} title="导出 Markdown">
                        <span class="material-symbols-outlined text-[12px] text-[var(--color-text-muted)]">download</span>
                      </button>
                      <button class="p-0.5 rounded hover:bg-[var(--color-surface)]" onclick={() => onDeleteSession?.(sess.session_id)} title="删除">
                        <span class="material-symbols-outlined text-[12px] text-red-500">delete</span>
                      </button>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}
        {#if sessionsTotal !== undefined && sessions.length < sessionsTotal}
          <button
            class="w-full py-2 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text)] rounded-lg hover:bg-[var(--color-surface)] transition-colors disabled:opacity-50"
            onclick={() => onLoadMore?.()}
            disabled={sessionsLoading}
          >
            {sessionsLoading ? '加载中...' : `加载更多（${sessions.length}/${sessionsTotal}）`}
          </button>
        {/if}
        {#if savedConversations.length === 0 && sessions.length === 0}
          <div class="flex flex-col items-center justify-center py-8 text-[var(--color-text-muted)] gap-1.5">
            <span class="material-symbols-outlined text-[24px]">chat</span>
            <span class="text-xs">暂无对话记录</span>
          </div>
        {/if}
      {/if}
    {:else}
      <!-- Generations Tab -->
      {#if genHistory.length === 0}
        <div class="flex flex-col items-center justify-center py-8 text-[var(--color-text-muted)] gap-1.5">
          <span class="material-symbols-outlined text-[24px]">auto_fix_high</span>
          <span class="text-xs">暂无生成记录</span>
        </div>
      {:else}
        <div class="space-y-1">
          {#each genHistory as item (item.id)}
            <button
              class="w-full text-left px-2 py-1.5 rounded-lg transition-colors hover:bg-[var(--color-surface)] group"
              onclick={() => onRestoreHistory?.(item)}
            >
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-[var(--color-text)] truncate max-w-[160px]">{item.title || '未命名'}</span>
                <span class="text-[10px] text-[var(--color-text-muted)]">{new Date(item.timestamp).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
              </div>
              <div class="flex items-center gap-1.5 mt-0.5">
                <span class="text-[10px] px-1 py-0.5 rounded" style="background: var(--color-primary-light); color: var(--color-primary)">{item.mode}</span>
                <span class="text-[10px] text-[var(--color-text-muted)]">{item.model}</span>
              </div>
              {#if item.preview}
                <p class="text-[10px] text-[var(--color-text-secondary)] mt-0.5" style="display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical; overflow: hidden;">{item.preview}</p>
              {/if}
            </button>
          {/each}
        </div>
      {/if}
    {/if}
  </div>

  <!-- Sidebar Footer -->
  <div class="px-2 py-1.5 border-t border-[var(--color-border)]">
    {#if historyTab === 'conversations'}
      <button class="w-full px-2 py-1.5 rounded-lg text-[11px] font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={onSave} disabled={convSaving || messagesLength === 0}>
        {convSaving ? '保存中...' : '保存当前对话'}
      </button>
    {:else}
      <button
        class="w-full px-2 py-1.5 rounded-lg text-[11px] font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors"
        onclick={onClearHistory}
      >
        清空历史
      </button>
    {/if}
  </div>
</div>

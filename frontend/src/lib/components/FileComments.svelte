<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { authFetch } from '$lib/api/client';

  let { projectId = '', filePath = '', onLineNumberClick = (line: number) => {} }: {
    projectId?: string;
    filePath?: string;
    onLineNumberClick?: (line: number) => void;
  } = $props();

  interface FileComment {
    id: number;
    project_id: string;
    file_path: string;
    user_id: string;
    username: string;
    line_number: number;
    content: string;
    parent_id: number;
    resolved: boolean;
    replies?: FileComment[];
    created_at: string;
  }

  let comments = $state<FileComment[]>([]);
  let loading = $state(false);
  let newCommentLine = $state(0);
  let newCommentContent = $state('');
  let submitting = $state(false);
  let replyingTo = $state<number | null>(null);
  let replyContent = $state('');
  let replying = $state(false);
  let expandedLines = $state<Set<number>>(new Set());

  // Derive comments grouped by line number
  let commentsByLine = $derived(() => {
    const map = new Map<number, FileComment[]>();
    for (const c of comments) {
      if (c.parent_id === 0) {
        const line = c.line_number;
        if (!map.has(line)) map.set(line, []);
        map.get(line)!.push(c);
      }
    }
    return map;
  });

  // Unique lines that have comments
  let commentLines = $derived(() => {
    const lines = new Set<number>();
    for (const c of comments) {
      lines.add(c.line_number);
    }
    return lines;
  });

  // filePath 出现/变化时加载评论。untrack 包住 loadComments(): 它异步写
  // comments/loading 等 $state, 不 untrack 会因这些 state 变化自我重跑死循环
  // (同 McpToolPanel/MetricsPanel/MDPromptsModal 已修模式)。只追踪 filePath 边沿。
  $effect(() => {
    if (filePath) untrack(() => loadComments());
  });

  async function loadComments() {
    if (!projectId || !filePath) return;
    loading = true;
    try {
      const res = await authFetch(`/api/v1/projects/${projectId}/files/${encodeURIComponent(filePath)}/comments`);
      if (res.ok) {
        const data = await res.json();
        comments = data.comments || [];
      }
    } catch (e) { console.error('load comments failed:', e); }
    loading = false;
  }

  async function addComment(line: number) {
    if (!newCommentContent.trim() || !projectId || !filePath) return;
    submitting = true;
    try {
      const res = await authFetch(`/api/v1/projects/${projectId}/files/${encodeURIComponent(filePath)}/comments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          line_number: line,
          content: newCommentContent,
          parent_id: 0,
        }),
      });
      if (res.ok) {
        newCommentContent = '';
        newCommentLine = 0;
        await loadComments();
      }
    } catch (e) { console.error('add comment failed:', e); }
    submitting = false;
  }

  async function replyToComment(commentId: number) {
    if (!replyContent.trim() || !projectId) return;
    replying = true;
    try {
      const res = await authFetch(`/api/v1/projects/${projectId}/comments/${commentId}/reply`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: replyContent }),
      });
      if (res.ok) {
        replyContent = '';
        replyingTo = null;
        await loadComments();
      }
    } catch (e) { console.error('reply comment failed:', e); }
    replying = false;
  }

  async function deleteComment(commentId: number) {
    if (!projectId) return;
    try {
      const res = await authFetch(`/api/v1/projects/${projectId}/comments/${commentId}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        await loadComments();
      }
    } catch (e) { console.error('delete comment failed:', e); }
  }

  function toggleLineComments(line: number) {
    if (expandedLines.has(line)) {
      expandedLines.delete(line);
      expandedLines = new Set(expandedLines);
    } else {
      expandedLines.add(line);
      expandedLines = new Set(expandedLines);
    }
  }

  function startNewComment(line: number) {
    newCommentLine = line;
    newCommentContent = '';
  }

  function cancelNewComment() {
    newCommentLine = 0;
    newCommentContent = '';
  }

  function startReply(commentId: number) {
    replyingTo = commentId;
    replyContent = '';
  }

  function cancelReply() {
    replyingTo = null;
    replyContent = '';
  }

  function getInitials(name: string): string {
    return name.slice(0, 2).toUpperCase();
  }

  function getAvatarColor(name: string): string {
    const colors = ['#e53935', '#1e88e5', '#43a047', '#fb8c00', '#8e24aa', '#00acc1', '#6d4c41', '#546e7a'];
    let hash = 0;
    for (let i = 0; i < name.length; i++) {
      hash = name.charCodeAt(i) + ((hash << 5) - hash);
    }
    return colors[Math.abs(hash) % colors.length];
  }
</script>

{#if comments.length > 0 || newCommentLine > 0}
  <div class="mt-2 border-t border-[var(--color-border)] pt-2">
    {#if loading}
      <div class="text-xs text-[var(--color-text-muted)] py-2">加载评论中...</div>
    {/if}

    {#each Array.from(commentLines()).sort((a, b) => a - b) as line}
      {@const lineComments = commentsByLine().get(line) || []}
      <div class="mb-2">
        <button
          class="flex items-center gap-1.5 text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text)] transition-colors"
          onclick={() => toggleLineComments(line)}
        >
          <span class="material-symbols-outlined text-[14px]">chat_bubble</span>
          <span>行 {line} ({lineComments.length})</span>
        </button>

        {#if expandedLines.has(line)}
          <div class="ml-4 mt-1 space-y-2 border-l-2 border-[var(--color-border)] pl-3">
            {#each lineComments as comment}
              <div class="p-2 rounded-lg bg-[var(--color-surface)] border border-[var(--color-border)]">
                <div class="flex items-center gap-2 mb-1">
                  <div
                    class="w-5 h-5 rounded-full flex items-center justify-center text-[9px] text-white font-bold"
                    style="background: {getAvatarColor(comment.username)}"
                  >
                    {getInitials(comment.username)}
                  </div>
                  <span class="text-xs font-medium text-[var(--color-text)]">{comment.username}</span>
                  <span class="text-[10px] text-[var(--color-text-muted)]">{new Date(comment.created_at).toLocaleString('zh-CN')}</span>
                  <button
                    class="ml-auto text-[10px] text-[var(--color-text-muted)] hover:text-red-500 transition-colors"
                    onclick={() => deleteComment(comment.id)}
                  >
                    删除
                  </button>
                </div>
                <p class="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap">{comment.content}</p>

                <!-- Replies -->
                {#if comment.replies && comment.replies.length > 0}
                  <div class="mt-2 space-y-1 border-l border-[var(--color-border)] pl-2">
                    {#each comment.replies as reply}
                      <div class="p-1.5 rounded bg-[var(--color-bg)]">
                        <div class="flex items-center gap-1.5 mb-0.5">
                          <span class="text-[10px] font-medium text-[var(--color-text)]">{reply.username}</span>
                          <span class="text-[9px] text-[var(--color-text-muted)]">{new Date(reply.created_at).toLocaleString('zh-CN')}</span>
                        </div>
                        <p class="text-[11px] text-[var(--color-text-secondary)]">{reply.content}</p>
                      </div>
                    {/each}
                  </div>
                {/if}

                <!-- Reply input -->
                {#if replyingTo === comment.id}
                  <div class="mt-2 flex gap-1.5">
                    <input
                      type="text"
                      class="flex-1 px-2 py-1 text-[11px] border border-[var(--color-border)] rounded bg-[var(--color-bg)] text-[var(--color-text)]"
                      placeholder="回复..."
                      bind:value={replyContent}
                      onkeydown={(e) => { if (e.key === 'Enter') replyToComment(comment.id); if (e.key === 'Escape') cancelReply(); }}
                    />
                    <button class="text-[10px] px-2 py-1 rounded bg-primary-500 text-white" onclick={() => replyToComment(comment.id)} disabled={replying}>
                      回复
                    </button>
                    <button class="text-[10px] px-2 py-1 rounded bg-[var(--color-surface)] text-[var(--color-text-secondary)]" onclick={cancelReply}>取消</button>
                  </div>
                {:else}
                  <button class="mt-1 text-[10px] text-[var(--color-text-muted)] hover:text-primary-500 transition-colors" onclick={() => startReply(comment.id)}>
                    回复
                  </button>
                {/if}
              </div>
            {/each}

            <!-- Add new comment on this line -->
            {#if newCommentLine === line}
              <div class="flex gap-1.5">
                <input
                  type="text"
                  class="flex-1 px-2 py-1 text-[11px] border border-[var(--color-border)] rounded bg-[var(--color-bg)] text-[var(--color-text)]"
                  placeholder="添加评论..."
                  bind:value={newCommentContent}
                  onkeydown={(e) => { if (e.key === 'Enter') addComment(line); if (e.key === 'Escape') cancelNewComment(); }}
                />
                <button class="text-[10px] px-2 py-1 rounded bg-primary-500 text-white" onclick={() => addComment(line)} disabled={submitting}>
                  添加
                </button>
                <button class="text-[10px] px-2 py-1 rounded bg-[var(--color-surface)] text-[var(--color-text-secondary)]" onclick={cancelNewComment}>取消</button>
              </div>
            {:else}
              <button class="text-[10px] text-[var(--color-text-muted)] hover:text-primary-500 transition-colors" onclick={() => startNewComment(line)}>
                + 添加评论
              </button>
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  </div>
{/if}

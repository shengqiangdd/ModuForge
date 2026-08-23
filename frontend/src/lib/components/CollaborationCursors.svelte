<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  let { projectId = '', currentUserId = '' }: {
    projectId?: string;
    currentUserId?: string;
  } = $props();

  interface CollabUser {
    user_id: string;
    username: string;
    file_path: string;
    cursor_line: number;
    cursor_col: number;
    color: string;
  }

  let onlineUsers = $state<CollabUser[]>([]);
  let wsConnected = $state(false);
  let ws: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  const USER_COLORS: Record<string, string> = {};
  const COLOR_PALETTE = ['#e53935', '#1e88e5', '#43a047', '#fb8c00', '#8e24aa', '#00acc1', '#6d4c41', '#546e7a'];

  function getUserColor(userId: string): string {
    if (!USER_COLORS[userId]) {
      const idx = Object.keys(USER_COLORS).length % COLOR_PALETTE.length;
      USER_COLORS[userId] = COLOR_PALETTE[idx];
    }
    return USER_COLORS[userId];
  }

  onMount(() => {
    if (projectId) connectWs();
  });

  onDestroy(() => {
    if (reconnectTimer) clearTimeout(reconnectTimer);
    if (ws) ws.close();
  });

  function connectWs() {
    if (!projectId) return;

    const token = localStorage.getItem('moduforge_token') || '';
    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/v1/ws/collaborate/${projectId}?token=${token}`;

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      wsConnected = true;
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleWsMessage(data);
      } catch {}
    };

    ws.onclose = () => {
      wsConnected = false;
      // Reconnect after 3 seconds
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        if (projectId) connectWs();
      }, 3000);
    };

    ws.onerror = () => {
      wsConnected = false;
    };
  }

  function handleWsMessage(data: any) {
    switch (data.type || data.event) {
      case 'online_users':
        // data.content is comma-separated user IDs
        if (data.content) {
          const userIds = data.content.split(',').filter(Boolean);
          onlineUsers = userIds.map((uid: string) => ({
            user_id: uid,
            username: uid,
            file_path: '',
            cursor_line: 0,
            cursor_col: 0,
            color: getUserColor(uid),
          }));
        }
        break;
      case 'join':
        if (data.user_id && data.user_id !== currentUserId) {
          onlineUsers = [...onlineUsers.filter(u => u.user_id !== data.user_id), {
            user_id: data.user_id,
            username: data.username || data.user_id,
            file_path: '',
            cursor_line: 0,
            cursor_col: 0,
            color: getUserColor(data.user_id),
          }];
        }
        break;
      case 'leave':
        onlineUsers = onlineUsers.filter(u => u.user_id !== data.user_id);
        break;
      case 'cursor':
        if (data.user_id && data.user_id !== currentUserId) {
          onlineUsers = onlineUsers.map(u =>
            u.user_id === data.user_id
              ? { ...u, file_path: data.file_path, cursor_line: data.line, cursor_col: data.column }
              : u
          );
        }
        break;
    }
  }

  export function sendCursorPosition(filePath: string, line: number, column: number) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        type: 'cursor',
        file_path: filePath,
        line,
        column,
      }));
    }
  }

  export function sendEditOperation(type: string, filePath: string, line: number, content: string) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        type,
        file_path: filePath,
        line,
        content,
      }));
    }
  }
</script>

{#if onlineUsers.length > 0 || wsConnected}
  <div class="flex items-center gap-1.5 ml-2">
    <!-- Connection indicator -->
    <div class="w-2 h-2 rounded-full {wsConnected ? 'bg-green-500' : 'bg-red-500'}" title={wsConnected ? '协作已连接' : '协作未连接'}></div>

    <!-- Online users avatars -->
    {#each onlineUsers.slice(0, 5) as user}
      <div
        class="w-5 h-5 rounded-full flex items-center justify-center text-[8px] text-white font-bold border-2 border-[var(--color-bg)]"
        style="background: {user.color}; margin-left: -6px;"
        title="{user.username} 在线"
      >
        {user.username.slice(0, 1).toUpperCase()}
      </div>
    {/each}
    {#if onlineUsers.length > 5}
      <div class="w-5 h-5 rounded-full flex items-center justify-center text-[8px] text-white font-bold bg-[var(--color-text-muted)] border-2 border-[var(--color-bg)]" style="margin-left: -6px;">
        +{onlineUsers.length - 5}
      </div>
    {/if}
    {#if onlineUsers.length > 0}
      <span class="text-[10px] text-[var(--color-text-muted)] ml-1">{onlineUsers.length} 在线</span>
    {/if}
  </div>
{/if}

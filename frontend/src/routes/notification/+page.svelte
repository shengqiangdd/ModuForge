<script lang="ts">
  import { onMount } from 'svelte';

  interface Notification {
    id: string;
    type: 'build_success' | 'build_failed' | 'security' | 'system';
    title: string;
    message: string;
    read: boolean;
    created_at: string;
  }

  let notifications = $state<Notification[]>([]);
  let loading = $state(true);
  let errorMsg = $state('');
  let successMsg = $state('');
  let filter = $state<'all' | 'unread'>('all');

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }

  function msg(err: string, ok?: string) {
    errorMsg = err; successMsg = ok || '';
    setTimeout(() => { errorMsg = ''; successMsg = ''; }, 4000);
  }

  const typeIcon = (t: string) => {
    if (t === 'build_success') return 'check_circle';
    if (t === 'build_failed') return 'error';
    if (t === 'security') return 'shield';
    return 'info';
  };

  const typeColor = (t: string) => {
    if (t === 'build_success') return '#22c55e';
    if (t === 'build_failed') return '#ef4444';
    if (t === 'security') return '#f97316';
    return '#3b82f6';
  };

  const typeLabel = (t: string) => {
    if (t === 'build_success') return '构建成功';
    if (t === 'build_failed') return '构建失败';
    if (t === 'security') return '安全警告';
    return '系统通知';
  };

  async function loadNotifications() {
    loading = true;
    try {
      const r = await fetch('/api/v1/notifications', { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { const d = await r.json(); notifications = d.notifications || d || []; }
    } catch {}
    loading = false;
  }

  async function markRead(id: string) {
    try {
      const r = await fetch(`/api/v1/notifications/${id}/read`, { method: 'PUT', headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) notifications = notifications.map(n => n.id === id ? { ...n, read: true } : n);
    } catch {}
  }

  async function markAllRead() {
    try {
      const r = await fetch('/api/v1/notifications/read-all', { method: 'POST', headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { notifications = notifications.map(n => ({ ...n, read: true })); msg('', '已全部标记为已读'); }
    } catch (e: any) { msg(e?.message || '操作失败'); }
  }

  async function deleteNotification(id: string) {
    try {
      const r = await fetch(`/api/v1/notifications/${id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) notifications = notifications.filter(n => n.id !== id);
    } catch {}
  }

  const unreadCount = $derived(notifications.filter(n => !n.read).length);

  const filteredNotifications = $derived(
    (filter === 'unread' ? notifications.filter(n => !n.read) : notifications)
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
  );

  onMount(loadNotifications);
</script>

<div class="w-full p-4 md:p-6 max-w-5xl mx-auto space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold text-[var(--color-text)]">通知中心</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">
        {unreadCount > 0 ? `${unreadCount} 条未读通知` : '所有通知已读'}
      </p>
    </div>
    <div class="flex gap-2">
      <button class="px-3 py-1.5 rounded-lg text-sm flex items-center gap-1.5" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={loadNotifications}>
        <span class="material-symbols-outlined text-[16px]">refresh</span>
        刷新
      </button>
      {#if unreadCount > 0}
        <button class="px-3 py-1.5 rounded-lg text-sm font-medium" style="background: var(--color-primary); color: white" onclick={markAllRead}>
          <span class="material-symbols-outlined text-[16px] align-middle">done_all</span>
          全部已读
        </button>
      {/if}
    </div>
  </div>

  {#if errorMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-error-light); color: var(--color-error)">{errorMsg}</div>
  {/if}
  {#if successMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-success-light); color: var(--color-success)">{successMsg}</div>
  {/if}

  <!-- Filter Tabs -->
  <div class="flex gap-1 p-1 rounded-lg w-fit" style="background: var(--color-surface)">
    <button
      class="px-3 py-1.5 rounded-md text-sm font-medium transition-colors"
      style="background: {filter === 'all' ? 'var(--color-primary)' : 'transparent'}; color: {filter === 'all' ? 'white' : 'var(--color-text-secondary)'}"
      onclick={() => filter = 'all'}
    >全部</button>
    <button
      class="px-3 py-1.5 rounded-md text-sm font-medium transition-colors"
      style="background: {filter === 'unread' ? 'var(--color-primary)' : 'transparent'}; color: {filter === 'unread' ? 'white' : 'var(--color-text-secondary)'}"
      onclick={() => filter = 'unread'}
    >未读 {#if unreadCount > 0}<span class="ml-1 inline-flex items-center justify-center w-5 h-5 rounded-full text-[10px] font-bold" style="background: var(--color-error); color: white">{unreadCount}</span>{/if}</button>
  </div>

  <!-- Notification List -->
  {#if loading}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">加载中...</div>
  {:else if filteredNotifications.length === 0}
    <div class="text-center py-12">
      <span class="material-symbols-outlined text-[48px]" style="color: var(--color-text-muted)">{filter === 'unread' ? 'mark_email_read' : 'notifications_off'}</span>
      <p class="text-sm mt-3" style="color: var(--color-text-muted)">{filter === 'unread' ? '没有未读通知' : '暂无通知'}</p>
    </div>
  {:else}
    <div class="space-y-2">
      {#each filteredNotifications as n (n.id)}
        <div
          class="card p-4 transition-all cursor-pointer hover:shadow-md {n.read ? '' : 'border-l-2'}"
          style={n.read ? '' : `border-left-color: ${typeColor(n.type)}`}
          onclick={() => markRead(n.id)}
          role="button"
          tabindex="0"
          onkeydown={(e) => { if (e.key === 'Enter') markRead(n.id); }}
        >
          <div class="flex items-start gap-3">
            <div class="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0" style="background: {typeColor(n.type)}18">
              <span class="material-symbols-outlined text-[18px]" style="color: {typeColor(n.type)}">{typeIcon(n.type)}</span>
            </div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-[var(--color-text)]">{n.title}</span>
                {#if !n.read}
                  <span class="w-2 h-2 rounded-full flex-shrink-0" style="background: {typeColor(n.type)}"></span>
                {/if}
                <span class="text-xs px-1.5 py-0.5 rounded-full" style="background: {typeColor(n.type)}18; color: {typeColor(n.type)}">{typeLabel(n.type)}</span>
              </div>
              <p class="text-xs mt-1" style="color: var(--color-text-secondary)">{n.message}</p>
              <span class="text-xs text-[var(--color-text-muted)]">{new Date(n.created_at).toLocaleString()}</span>
            </div>
            <button
              class="flex-shrink-0 p-1 rounded hover:bg-[var(--color-error-light)] transition-colors"
              style="color: var(--color-text-muted)"
              onclick={(e) => { e.stopPropagation(); deleteNotification(n.id); }}
            >
              <span class="material-symbols-outlined text-[16px]">close</span>
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

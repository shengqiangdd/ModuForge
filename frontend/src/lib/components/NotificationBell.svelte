<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n';
  import { getToken } from '$lib/api/client';

  let open = $state(false);
  let unreadCount = $state(0);
  let notifications = $state<any[]>([]);
  let loading = $state(false);

  const notifIcons: Record<string, string> = {
    build_started: 'construction',
    build_completed: 'check_circle',
    build_failed: 'error',
    team_invite: 'person_add',
    market_review: 'rate_review',
    review: 'comment',
  };

  async function loadUnread() {
    const token = getToken();
    if (!token) return;
    try {
      const res = await fetch('/api/v1/notifications/unread-count', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) {
        const data = await res.json();
        unreadCount = data.count || 0;
      }
    } catch {}
  }

  async function loadList() {
    const token = getToken();
    if (!token) return;
    loading = true;
    try {
      const res = await fetch('/api/v1/notifications?limit=20', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) {
        const data = await res.json();
        notifications = data.notifications || [];
      }
    } catch {}
    loading = false;
  }

  async function markRead(id: number) {
    const token = getToken();
    if (!token) return;
    try {
      await fetch(`/api/v1/notifications/${id}/read`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      notifications = notifications.map((n: any) => n.id === id ? { ...n, is_read: true } : n);
      unreadCount = Math.max(0, unreadCount - 1);
    } catch {}
  }

  async function markAllRead() {
    const token = getToken();
    if (!token) return;
    try {
      await fetch('/api/v1/notifications/read-all', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      notifications = notifications.map((n: any) => ({ ...n, is_read: true }));
      unreadCount = 0;
    } catch {}
  }

  function toggle() {
    open = !open;
    if (open) loadList();
  }

  function timeAgo(dateStr: string) {
    const diff = Date.now() - new Date(dateStr).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return '刚刚';
    if (mins < 60) return `${mins} 分钟前`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs} 小时前`;
    const days = Math.floor(hrs / 24);
    return `${days} 天前`;
  }

  onMount(() => {
    loadUnread();
    const interval = setInterval(loadUnread, 30000);
    return () => clearInterval(interval);
  });
</script>

<div class="relative">
  <button
    class="relative p-2 rounded-lg min-w-[44px] min-h-[44px] flex items-center justify-center notif-bell"
    style="color: var(--color-text-muted)"
    onclick={toggle}
    aria-label="通知"
  >
    <span class="material-symbols-outlined text-[20px]">notifications</span>
    {#if unreadCount > 0}
      <span class="absolute top-1.5 right-1.5 w-4 h-4 rounded-full flex items-center justify-center text-[9px] font-bold text-white"
            style="background: var(--color-error); min-width: 16px;">
        {unreadCount > 99 ? '99+' : unreadCount}
      </span>
    {/if}
  </button>

  {#if open}
    <div class="notif-dropdown fixed md:absolute right-2 md:right-0 top-14 md:top-full md:mt-2 w-[calc(100vw-16px)] md:w-80 rounded-2xl border shadow-2xl z-50 animate-[scaleIn_0.15s_ease-out]"
         style="background: var(--color-bg-elevated); border-color: var(--color-border);">
      <div class="flex items-center justify-between px-4 py-3 border-b" style="border-color: var(--color-border)">
        <span class="text-sm font-semibold" style="color: var(--color-text)">{$t('notif.title')}</span>
        {#if notifications.some((n: any) => !n.is_read)}
          <button class="text-xs text-[var(--color-primary)] hover:underline" onclick={markAllRead}>
            {$t('notif.mark_all_read')}
          </button>
        {/if}
      </div>
      <div class="max-h-80 overflow-y-auto">
        {#if loading}
          <div class="p-6 text-center text-sm" style="color: var(--color-text-muted)">{$t('common.loading')}</div>
        {:else if notifications.length === 0}
          <div class="p-6 text-center text-sm" style="color: var(--color-text-muted)">{$t('notif.empty')}</div>
        {:else}
          {#each notifications as notif (notif.id)}
            <button
              class="w-full flex items-start gap-3 px-4 py-3 text-left transition-colors hover:bg-[var(--color-surface)]"
              class:opacity-60={notif.is_read}
              onclick={() => { if (!notif.is_read) markRead(notif.id); }}
            >
              <span class="material-symbols-outlined text-[18px] mt-0.5 flex-shrink-0"
                    style="color: {notif.is_read ? 'var(--color-text-muted)' : 'var(--color-primary)'}">
                {notifIcons[notif.type] || 'notifications'}
              </span>
              <div class="flex-1 min-w-0">
                <p class="text-sm font-medium truncate" style="color: var(--color-text)">{notif.title}</p>
                <p class="text-xs truncate" style="color: var(--color-text-secondary)">{notif.message}</p>
                <p class="text-[10px] mt-0.5" style="color: var(--color-text-muted)">{timeAgo(notif.created_at)}</p>
              </div>
            </button>
          {/each}
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  @media (max-width: 768px) {
    .notif-dropdown {
      position: fixed;
      top: 56px;
      right: 8px;
      left: 8px;
      width: auto;
      max-height: 60vh;
      z-index: 100;
    }
  }
</style>

{#if open}
  <div class="fixed inset-0 z-40" role="presentation" onclick={() => open = false}></div>
{/if}

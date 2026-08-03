<script lang="ts">
  let { onAction }: { onAction: (action: string) => void } = $props();

  let open = $state(false);

  const actions = [
    { id: 'new-project', icon: 'add_circle', label: '新建项目', color: 'var(--color-primary)' },
    { id: 'open-project', icon: 'folder_open', label: '打开项目', color: 'var(--color-info)' },
    { id: 'ai-chat', icon: 'psychology', label: 'AI 对话', color: 'var(--color-success)' },
    { id: 'build', icon: 'build', label: '构建模块', color: 'var(--color-warning)' },
    { id: 'settings', icon: 'settings', label: '设置', color: 'var(--color-text-muted)' },
    { id: 'help', icon: 'help', label: '帮助', color: 'var(--color-text-muted)' },
  ];

  function handleAction(actionId: string) {
    open = false;
    onAction(actionId);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && open) {
      open = false;
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Floating Action Button -->
<button
  class="fixed bottom-20 md:bottom-6 right-4 z-40 w-14 h-14 rounded-full flex items-center justify-center shadow-lg transition-all duration-300 hover:scale-110 hover:shadow-xl active:scale-95"
  style="background: var(--gradient-brand); color: white; box-shadow: 0 4px 20px rgba(139,92,246,0.4)"
  onclick={() => open = !open}
  title="快捷操作 (Ctrl+Q)"
>
  <span class="material-symbols-outlined text-2xl transition-transform duration-300" style="transform: {open ? 'rotate(45deg)' : 'rotate(0deg)'}">
    {open ? 'close' : 'bolt'}
  </span>
</button>

<!-- Backdrop -->
{#if open}
  <div
    class="fixed inset-0 z-30 animate-[fadeIn_0.15s_ease-out]"
    style="background: rgba(0,0,0,0.3); backdrop-filter: blur(4px)"
    onclick={() => open = false}
    onkeydown={(e) => { if (e.key === 'Escape') open = false; }}
    role="button"
    tabindex="-1"
  ></div>
{/if}

<!-- Quick Actions Panel -->
{#if open}
  <div
    class="fixed bottom-24 md:bottom-20 right-4 z-40 w-56 rounded-2xl border shadow-2xl p-2 animate-[slideUp_0.2s_ease-out]"
    style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: 0 20px 40px rgba(0,0,0,0.3)"
  >
    <div class="px-3 py-2 mb-1">
      <p class="text-xs font-semibold" style="color: var(--color-text-muted)">快捷操作</p>
    </div>
    {#each actions as action}
      <button
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-150 text-sm"
        style="color: var(--color-text-secondary)"
        onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'var(--color-surface)'}
        onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = 'transparent'}
        onclick={() => handleAction(action.id)}
      >
        <span class="material-symbols-outlined text-[18px]" style="color: {action.color}">{action.icon}</span>
        <span class="font-medium">{action.label}</span>
      </button>
    {/each}
    <div class="mt-1 pt-1 border-t" style="border-color: var(--color-border)">
      <div class="px-3 py-1.5 flex items-center gap-1.5">
        <span class="text-[10px]" style="color: var(--color-text-muted)">快捷键</span>
        <kbd class="px-1.5 py-0.5 rounded text-[10px] font-mono" style="background: var(--color-surface); color: var(--color-text-muted)">Ctrl+Q</kbd>
      </div>
    </div>
  </div>
{/if}

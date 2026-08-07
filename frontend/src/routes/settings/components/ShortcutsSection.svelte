<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';
  import { loadShortcuts, saveShortcuts, resetShortcuts, defaultShortcuts, type Shortcut } from '$lib/stores/shortcuts';

  let userShortcuts = $state<Shortcut[]>([]);
  let editingShortcutId = $state<string | null>(null);
  let recordingShortcut = $state(false);
  let shortcutRecordEl = $state<HTMLInputElement | undefined>();

  function initShortcuts() {
    userShortcuts = loadShortcuts();
  }

  function handleShortcutRecord(e: KeyboardEvent) {
    e.preventDefault();
    const sc = userShortcuts.find(s => s.id === editingShortcutId);
    if (!sc) return;
    sc.key = e.key;
    sc.ctrlKey = e.ctrlKey;
    sc.shiftKey = e.shiftKey;
    sc.metaKey = e.metaKey;
    userShortcuts = [...userShortcuts];
    recordingShortcut = false;
    editingShortcutId = null;
    saveShortcuts(userShortcuts);
    toast('快捷键已更新', 'success');
  }

  function startRecord(id: string) {
    editingShortcutId = id;
    recordingShortcut = true;
    setTimeout(() => shortcutRecordEl?.focus(), 50);
  }

  function doResetShortcuts() {
    if (!confirm('确认重置所有快捷键为默认值？')) return;
    userShortcuts = resetShortcuts();
    toast('快捷键已重置', 'success');
  }

  function formatKey(sc: Shortcut): string {
    const parts: string[] = [];
    if (sc.ctrlKey) parts.push('Ctrl');
    if (sc.metaKey) parts.push('⌘');
    if (sc.shiftKey) parts.push('Shift');
    if (sc.altKey) parts.push('Alt');
    parts.push(sc.key);
    return parts.join(' + ');
  }

  import { onMount } from 'svelte';
  onMount(() => { initShortcuts(); });
</script>

<div class="settings-section">
  <div class="section-header">
    <h2 class="section-title">快捷键</h2>
    <button class="btn-secondary btn-sm" onclick={doResetShortcuts}>重置为默认</button>
  </div>

  <div class="shortcuts-list">
    {#each userShortcuts as sc (sc.id)}
      <div class="shortcut-item">
        <span class="shortcut-label">{sc.label}</span>
        {#if editingShortcutId === sc.id && recordingShortcut}
          <input
            bind:this={shortcutRecordEl}
            type="text"
            class="shortcut-input recording"
            value="按下快捷键..."
            onkeydown={handleShortcutRecord}
            readonly
          />
        {:else}
          <button class="shortcut-key" onclick={() => startRecord(sc.id)}>
            {formatKey(sc)}
          </button>
        {/if}
      </div>
    {/each}
  </div>
</div>

<style>
  .settings-section { margin-bottom: 2rem; }
  .section-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
  .section-title { font-size: 1.25rem; font-weight: 600; margin: 0; }
  .shortcuts-list { display: flex; flex-direction: column; gap: 0.5rem; }
  .shortcut-item { display: flex; align-items: center; justify-content: space-between; padding: 0.75rem 1rem; background: var(--color-bg-secondary); border: 1px solid var(--color-border); border-radius: 0.5rem; }
  .shortcut-label { font-size: 0.875rem; }
  .shortcut-key { padding: 0.25rem 0.75rem; font-family: monospace; font-size: 0.875rem; background: var(--color-bg); border: 1px solid var(--color-border); border-radius: 0.25rem; cursor: pointer; }
  .shortcut-key:hover { border-color: var(--color-primary); }
  .shortcut-input { padding: 0.25rem 0.75rem; font-family: monospace; font-size: 0.875rem; text-align: center; border: 2px solid var(--color-primary); border-radius: 0.25rem; outline: none; width: 150px; }
  .shortcut-input.recording { animation: pulse 1s infinite; }
  .btn-secondary { padding: 0.375rem 0.75rem; border: 1px solid var(--color-border); border-radius: 0.375rem; background: var(--color-bg); cursor: pointer; font-size: 0.875rem; }
  .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.75rem; }
  @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.7; } }
</style>

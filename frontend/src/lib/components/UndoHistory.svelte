<script lang="ts">
  import { historyStore } from '$lib/stores/history';

  let { onClose = () => {}, onRollback = (_index: number) => {} } = $props();

  let history = $state(historyStore.getHistory());

  function refresh() {
    history = historyStore.getHistory();
  }

  function rollback(index: number) {
    onRollback(index);
    onClose();
  }

  function describeAction(a: { type: string; data: any }): string {
    switch (a.type) {
      case 'file_save': return `保存文件: ${a.data.path || 'unknown'}`;
      case 'file_create': return `创建文件: ${a.data.path || 'unknown'}`;
      case 'file_delete': return `删除文件: ${a.data.path || 'unknown'}`;
      case 'file_upload': return `上传文件: ${(a.data.count || 1)} 个文件`;
      default: return a.type;
    }
  }

  function iconForType(type: string): string {
    switch (type) {
      case 'file_save': return 'save';
      case 'file_create': return 'note_add';
      case 'file_delete': return 'delete';
      case 'file_upload': return 'upload_file';
      default: return 'history';
    }
  }
</script>

<div class="overlay" onclick={onClose} role="dialog" aria-modal="true">
  <div class="panel" onclick={(e) => e.stopPropagation()} role="document">
    <div class="panel-header">
      <h2 class="panel-title">操作历史</h2>
      <button class="close-btn" onclick={onClose} aria-label="关闭">
        <span class="material-symbols-outlined text-[18px]">close</span>
      </button>
    </div>
    <div class="panel-body">
      {#if history.length === 0}
        <div class="empty">暂无操作历史</div>
      {:else}
        <div class="timeline">
          {#each [...history].reverse() as action, i}
            {@const idx = history.length - 1 - i}
            <button
              class="timeline-item"
              onclick={() => rollback(idx)}
            >
              <div class="timeline-dot"></div>
              <div class="timeline-content">
                <div class="timeline-icon">
                  <span class="material-symbols-outlined text-[14px]">{iconForType(action.type)}</span>
                </div>
                <div class="timeline-text">
                  <span class="timeline-desc">{describeAction(action)}</span>
                  <span class="timeline-time">{new Date(action.timestamp).toLocaleTimeString()}</span>
                </div>
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </div>
    <div class="panel-footer">
      <span class="hint">点击条目回滚到该操作</span>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0,0,0,0.6);
    backdrop-filter: blur(8px);
    padding: 16px;
  }
  .panel {
    width: 100%;
    max-width: 420px;
    max-height: 80vh;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: 16px;
    box-shadow: var(--shadow-xl);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--color-border);
  }
  .panel-title {
    font-size: 16px;
    font-weight: 700;
    color: var(--color-text);
  }
  .close-btn {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 8px;
    border: none;
    background: var(--color-surface);
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: all 0.15s;
  }
  .close-btn:hover {
    background: var(--color-border);
    color: var(--color-text);
  }
  .panel-body {
    flex: 1;
    overflow-y: auto;
    padding: 12px 20px;
  }
  .empty {
    text-align: center;
    padding: 32px 0;
    color: var(--color-text-muted);
    font-size: 13px;
  }
  .timeline {
    position: relative;
  }
  .timeline::before {
    content: '';
    position: absolute;
    left: 8px;
    top: 8px;
    bottom: 8px;
    width: 2px;
    background: var(--color-border);
    border-radius: 1px;
  }
  .timeline-item {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 8px 8px 8px 0;
    width: 100%;
    text-align: left;
    border: none;
    background: transparent;
    cursor: pointer;
    border-radius: 8px;
    transition: background 0.15s;
    position: relative;
  }
  .timeline-item:hover {
    background: var(--color-surface);
  }
  .timeline-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--color-primary);
    margin-top: 6px;
    flex-shrink: 0;
    position: relative;
    z-index: 1;
  }
  .timeline-content {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    flex: 1;
    min-width: 0;
  }
  .timeline-icon {
    width: 28px;
    height: 28px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    background: var(--color-primary-light);
    color: var(--color-primary);
  }
  .timeline-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .timeline-desc {
    font-size: 13px;
    color: var(--color-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .timeline-time {
    font-size: 11px;
    color: var(--color-text-muted);
  }
  .panel-footer {
    padding: 10px 20px;
    border-top: 1px solid var(--color-border);
    text-align: center;
  }
  .hint {
    font-size: 11px;
    color: var(--color-text-muted);
  }
</style>

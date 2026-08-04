<script lang="ts">
  import { onMount } from 'svelte';

  let { onClose = () => {} }: { onClose?: () => void } = $props();

  const categories = [
    {
      name: '通用',
      shortcuts: [
        { keys: 'Ctrl+K', desc: '搜索项目/文件' },
        { keys: 'Ctrl+S', desc: '保存当前文件' },
        { keys: 'Ctrl+P', desc: '文件快速搜索' },
        { keys: '?', desc: '打开快捷键帮助' },
      ],
    },
    {
      name: '编辑器',
      shortcuts: [
        { keys: 'Ctrl+Z', desc: '撤销' },
        { keys: 'Ctrl+Shift+Z', desc: '重做' },
        { keys: 'Ctrl+/', desc: '切换注释' },
        { keys: 'Ctrl+F', desc: '在文件中查找' },
        { keys: 'Ctrl+H', desc: '查找替换' },
      ],
    },
    {
      name: '导航',
      shortcuts: [
        { keys: 'Ctrl+1-9', desc: '切换标签页' },
        { keys: 'Ctrl+W', desc: '关闭当前标签' },
        { keys: 'Ctrl+Tab', desc: '下一个标签' },
        { keys: 'Ctrl+Shift+Tab', desc: '上一个标签' },
      ],
    },
    {
      name: '终端',
      shortcuts: [
        { keys: 'Ctrl+`', desc: '切换终端面板' },
        { keys: 'Ctrl+L', desc: '清屏' },
        { keys: '↑/↓', desc: '命令历史' },
      ],
    },
  ];

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      onClose();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeydown);
    return () => window.removeEventListener('keydown', handleKeydown);
  });
</script>

<div class="overlay" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
  <div class="panel" role="dialog" aria-modal="true" tabindex="-1">
    <div class="panel-header">
      <h2 class="panel-title">快捷键</h2>
      <button class="close-btn" onclick={onClose} aria-label="关闭">
        <span class="material-symbols-outlined text-[18px]">close</span>
      </button>
    </div>
    <div class="panel-body">
      {#each categories as cat}
        <div class="category">
          <h3 class="category-title">{cat.name}</h3>
          <table class="shortcut-table">
            <tbody>
              {#each cat.shortcuts as sc}
                <tr>
                  <td class="keys-cell">
                    <kbd class="key-badge">{sc.keys}</kbd>
                  </td>
                  <td class="desc-cell">{sc.desc}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/each}
    </div>
    <div class="panel-footer">
      <span class="hint">按 <kbd class="key-badge">ESC</kbd> 关闭</span>
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
    max-width: 480px;
    max-height: 90vh;
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
  .category {
    margin-bottom: 16px;
  }
  .category:last-child {
    margin-bottom: 0;
  }
  .category-title {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--color-primary);
    margin-bottom: 8px;
  }
  .shortcut-table {
    width: 100%;
    border-collapse: collapse;
  }
  .shortcut-table tr + tr {
    border-top: 1px solid var(--color-border);
  }
  .keys-cell {
    padding: 8px 12px 8px 0;
    width: 140px;
  }
  .desc-cell {
    padding: 8px 0;
    font-size: 13px;
    color: var(--color-text-secondary);
  }
  .key-badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    font-family: monospace;
    font-weight: 500;
    background: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
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

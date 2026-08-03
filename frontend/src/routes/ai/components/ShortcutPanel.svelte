<script lang="ts">
  let { show, onClose }: { show: boolean; onClose: () => void } = $props();

  const shortcuts = [
    { keys: ['?'], desc: '打开快捷键面板' },
    { keys: ['Ctrl', 'K'], desc: '新建对话' },
    { keys: ['Ctrl', 'E'], desc: '导出为 Markdown' },
    { keys: ['1-6'], desc: '切换工作模式（对话/生成/自动构建/需求/修复/分析）' },
    { keys: ['Esc'], desc: '关闭弹窗/侧边栏' },
  ];

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') { onClose(); }
  }
</script>

{#if show}
  <div class="shortcut-overlay" role="dialog" aria-label="快捷键" onkeydown={handleKeydown}>
    <div class="shortcut-card">
      <div class="header">
        <h3>⌨️ 快捷键</h3>
        <button class="close-btn" onclick={onClose} aria-label="关闭">
          <span class="material-symbols-outlined" style="font-size:20px">close</span>
        </button>
      </div>

      <div class="shortcut-list">
        {#each shortcuts as s}
          <div class="shortcut-row">
            <div class="keys">
              {#each s.keys as k, i}
                {#if i > 0}<span class="plus">+</span>{/if}
                <kbd>{k}</kbd>
              {/each}
            </div>
            <span class="desc">{s.desc}</span>
          </div>
        {/each}
      </div>

      <div class="footer">
        按 <kbd>Esc</kbd> 关闭
      </div>
    </div>
  </div>
{/if}

<style>
  .shortcut-overlay {
    position: fixed; inset: 0; z-index: 9998;
    background: rgba(0,0,0,0.5); backdrop-filter: blur(4px);
    display: flex; align-items: center; justify-content: center;
    animation: fadeIn 0.15s ease;
  }
  .shortcut-card {
    background: var(--color-surface, #fff);
    border-radius: 12px; padding: 24px;
    max-width: 380px; width: 90%;
    box-shadow: 0 16px 48px rgba(0,0,0,0.25);
    animation: slideUp 0.2s ease;
  }
  .header {
    display: flex; align-items: center; justify-content: space-between;
    margin-bottom: 16px;
  }
  h3 { margin: 0; font-size: 16px; font-weight: 600; color: var(--color-text); }
  .close-btn {
    background: none; border: none; cursor: pointer;
    color: var(--color-text-muted); padding: 4px; border-radius: 6px;
    transition: all 0.15s;
  }
  .close-btn:hover { background: var(--color-bg-secondary, #f0f0f0); }
  .shortcut-list { display: flex; flex-direction: column; gap: 10px; }
  .shortcut-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 0; border-bottom: 1px solid var(--color-border, #eee);
  }
  .shortcut-row:last-child { border-bottom: none; }
  .keys { display: flex; align-items: center; gap: 4px; }
  kbd {
    display: inline-block; padding: 3px 8px; border-radius: 6px;
    background: var(--color-bg-secondary, #f5f5f5);
    border: 1px solid var(--color-border, #ddd);
    font-family: 'SF Mono', 'Consolas', monospace;
    font-size: 12px; font-weight: 500; color: var(--color-text);
    min-width: 24px; text-align: center;
  }
  .plus { color: var(--color-text-muted); font-size: 11px; }
  .desc { font-size: 13px; color: var(--color-text-muted); text-align: right; }
  .footer {
    margin-top: 16px; text-align: center;
    font-size: 12px; color: var(--color-text-muted);
  }
  .footer kbd { font-size: 11px; padding: 2px 6px; }
  @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
  @keyframes slideUp { from { opacity: 0; transform: translateY(12px); } to { opacity: 1; transform: translateY(0); } }
</style>

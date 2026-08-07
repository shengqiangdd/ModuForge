<script lang="ts">
  let {
    screenWidth = 0,
    screenHeight = 0,
    screenRefreshing = false,
    screenAutoRefresh = false,
    inputText = '',
    holdDuration = 1000,
    holdActive = false,
    recording = false,
    onRefresh,
    onToggleAutoRefresh,
    onToggleRecording,
    onSendKey,
    onSendKeyCombo,
    onSendInputText,
    onSwipeScreen,
    onRotateScreen,
    onInputChange,
    onHoldDurationChange,
  }: {
    screenWidth?: number;
    screenHeight?: number;
    screenRefreshing?: boolean;
    screenAutoRefresh?: boolean;
    inputText?: string;
    holdDuration?: number;
    holdActive?: boolean;
    recording?: boolean;
    onRefresh?: () => void;
    onToggleAutoRefresh?: () => void;
    onToggleRecording?: () => void;
    onSendKey?: (key: string) => void;
    onSendKeyCombo?: (keys: string[]) => void;
    onSendInputText?: (text: string) => void;
    onSwipeScreen?: (x1: number, y1: number, x2: number, y2: number, duration: number) => void;
    onRotateScreen?: (dir: string) => void;
    onInputChange?: (v: string) => void;
    onHoldDurationChange?: (v: number) => void;
  } = $props();
</script>

<div class="flex items-center gap-2 flex-wrap">
  <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-primary); color: white" onclick={onRefresh} disabled={screenRefreshing}>
    <span class="material-symbols-outlined text-[14px] align-middle">refresh</span>
    {screenRefreshing ? '刷新中...' : '刷新截图'}
  </button>
  <button
    class="text-xs px-3 py-1.5 rounded-md transition-colors"
    style={screenAutoRefresh ? 'background: var(--color-primary); color: white' : 'background: var(--color-surface); color: var(--color-text-muted); border: 1px solid var(--color-border)'}
    onclick={onToggleAutoRefresh}
  >
    <span class="material-symbols-outlined text-[14px] align-middle">{screenAutoRefresh ? 'pause' : 'play_arrow'}</span>
    {screenAutoRefresh ? '停止自动刷新' : '自动刷新'}
  </button>
  <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={onToggleRecording}>
    <span class="material-symbols-outlined text-[14px] align-middle">{recording ? 'stop' : 'videocam'}</span>
    {recording ? '停止录制' : '录制屏幕'}
  </button>
  <span class="text-xs" style="color: var(--color-text-muted)">
    {screenWidth}x{screenHeight}
  </span>
</div>

<div>
  <p class="text-xs font-medium mb-2" style="color: var(--color-text-secondary)">导航</p>
  <div class="grid grid-cols-3 gap-2 max-w-[200px]">
    <div></div>
    <button class="px-3 py-2 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_DPAD_UP')}>▲</button>
    <div></div>
    <button class="px-3 py-2 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_DPAD_LEFT')}>◄</button>
    <button class="px-3 py-2 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_DPAD_CENTER')}>●</button>
    <button class="px-3 py-2 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_DPAD_RIGHT')}>►</button>
    <div></div>
    <button class="px-3 py-2 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_DPAD_DOWN')}>▼</button>
    <div></div>
  </div>
</div>

<div>
  <p class="text-xs font-medium mb-2" style="color: var(--color-text-secondary)">功能键</p>
  <div class="flex flex-wrap gap-2">
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_HOME')}>Home</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_BACK')}>返回</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_APP_SWITCH')}>最近</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_POWER')}>电源</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_VOLUME_UP')}>音量+</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKey?.('KEYCODE_VOLUME_DOWN')}>音量-</button>
  </div>
</div>

<div>
  <p class="text-xs font-medium mb-2" style="color: var(--color-text-secondary)">文字输入</p>
  <div class="flex gap-2">
    <input type="text" class="input-field flex-1 text-xs" value={inputText} placeholder="输入文字..." oninput={(e) => onInputChange?.((e.target as HTMLInputElement).value)} onkeydown={(e) => { if (e.key === 'Enter') onSendInputText?.(inputText); }} />
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium text-white transition-colors" style="background: var(--color-primary)" onclick={() => onSendInputText?.(inputText)} disabled={!inputText}>发送</button>
  </div>
</div>

<div>
  <p class="text-xs font-medium mb-2" style="color: var(--color-text-secondary)">快捷操作</p>
  <div class="flex flex-wrap gap-2">
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSwipeScreen?.(screenWidth/2, screenHeight*0.8, screenWidth/2, screenHeight*0.2, 300)}>↓ 向下滑</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSwipeScreen?.(screenWidth/2, screenHeight*0.2, screenWidth/2, screenHeight*0.8, 300)}>↑ 向上滑</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSwipeScreen?.(screenWidth*0.8, screenHeight/2, screenWidth*0.2, screenHeight/2, 300)}>→ 向左滑</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSwipeScreen?.(screenWidth*0.2, screenHeight/2, screenWidth*0.8, screenHeight/2, 300)}>← 向右滑</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSwipeScreen?.(screenWidth/2, screenHeight/2, screenWidth/2, screenHeight/2, 0)}>点击(无滑动)</button>
  </div>
</div>

<div>
  <p class="text-xs font-medium mb-2" style="color: var(--color-text-secondary)">按键组合</p>
  <div class="flex flex-wrap gap-2">
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKeyCombo?.(['KEYCODE_CTRL_LEFT', 'KEYCODE_C'])}>Ctrl+C</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKeyCombo?.(['KEYCODE_CTRL_LEFT', 'KEYCODE_V'])}>Ctrl+V</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKeyCombo?.(['KEYCODE_CTRL_LEFT', 'KEYCODE_A'])}>Ctrl+A</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKeyCombo?.(['KEYCODE_CTRL_LEFT', 'KEYCODE_Z'])}>Ctrl+Z</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKeyCombo?.(['KEYCODE_CTRL_LEFT', 'KEYCODE_S'])}>Ctrl+S</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onSendKeyCombo?.(['KEYCODE_ALT_LEFT', 'KEYCODE_TAB'])}>Alt+Tab</button>
  </div>
</div>

<div>
  <p class="text-xs font-medium mb-2" style="color: var(--color-text-secondary)">屏幕旋转</p>
  <div class="flex flex-wrap gap-2">
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onRotateScreen?.('portrait')}>竖屏</button>
    <button class="px-3 py-1.5 rounded-xl text-xs font-medium transition-colors" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={() => onRotateScreen?.('landscape')}>横屏</button>
  </div>
</div>

<div>
  <p class="text-xs font-medium mb-2" style="color: var(--color-text-secondary)">长按设置</p>
  <div class="flex items-center gap-2">
    <span class="text-xs" style="color: var(--color-text-muted)">{holdDuration}ms</span>
    <input type="range" min="200" max="5000" step="100" value={holdDuration} oninput={(e) => onHoldDurationChange?.(parseInt((e.target as HTMLInputElement).value))} class="w-32" />
    {#if holdActive}
      <span class="text-xs" style="color: var(--color-primary)">长按中...</span>
    {/if}
  </div>
</div>

<div class="p-3 rounded-xl text-xs" style="background: var(--color-info-light); color: var(--color-info)">
  点击/触摸截图可直接操作。短按=tap，长按=long press，滑动=swipe。支持自动刷新模式。
</div>

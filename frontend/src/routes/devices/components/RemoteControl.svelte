<script lang="ts">
  import ScreenCanvas from '$lib/components/ScreenCanvas.svelte';
  import ScreenControls from './ScreenControls.svelte';

  let {
    selectedDevice,
    screenFitWidth,
    screenWidth,
    screenHeight,
    screenRefreshing,
    screenAutoRefresh,
    inputText,
    holdDuration,
    holdActive,
    recording,
    onRefresh,
    onToggleAutoRefresh,
    onToggleRecording,
    onSendKey,
    onSendKeyCombo,
    onSendInputText,
    onSwipeScreen,
    onRotateScreen,
    onInputChange,
    onHoldDurationChange
  }: {
    selectedDevice: string;
    screenFitWidth: number;
    screenWidth: number;
    screenHeight: number;
    screenRefreshing: boolean;
    screenAutoRefresh: boolean;
    inputText: string;
    holdDuration: number;
    holdActive: boolean;
    recording: boolean;
    onRefresh: () => void;
    onToggleAutoRefresh: () => void;
    onToggleRecording: () => void;
    onSendKey: (key: string) => void;
    onSendKeyCombo: (keys: string[]) => void;
    onSendInputText: (text: string) => void;
    onSwipeScreen: (x1: number, y1: number, x2: number, y2: number, duration: number) => void;
    onRotateScreen: (dir: string) => void;
    onInputChange: (v: string) => void;
    onHoldDurationChange: (v: number) => void;
  } = $props();

  function handleInputSubmit() {
    if (inputText.trim()) {
      onSendInputText(inputText.trim());
      onInputChange('');
    }
  }
</script>

<div class="space-y-4">
  <div class="flex items-center gap-2 flex-wrap">
    <button class="btn-primary text-xs" onclick={onRefresh} disabled={screenRefreshing}>
      <span class="material-symbols-outlined text-[14px]">refresh</span>
      {screenRefreshing ? '刷新中...' : '刷新截图'}
    </button>
    <button
      class="text-xs px-3 py-1.5 rounded-md"
      style={screenAutoRefresh ? 'background: var(--color-primary); color: white' : 'background: var(--color-surface); color: var(--color-text-muted); border: 1px solid var(--color-border)'}
      onclick={onToggleAutoRefresh}
      aria-label={screenAutoRefresh ? '停止自动刷新' : '开始自动刷新'}
    >
      <span class="material-symbols-outlined text-[14px] align-middle">{screenAutoRefresh ? 'pause' : 'play_arrow'}</span>
      {screenAutoRefresh ? '停止自动刷新' : '自动刷新'}
    </button>
    <button class="btn-ghost text-xs" onclick={onToggleRecording} aria-label={recording ? '停止录制' : '开始录制'}>
      <span class="material-symbols-outlined text-[14px]">{recording ? 'stop' : 'videocam'}</span>
      {recording ? '停止录制' : '录制屏幕'}
    </button>
    <span class="text-xs" style="color: var(--color-text-muted)">
      {screenWidth}x{screenHeight}
    </span>
    <div class="flex items-center gap-1 ml-auto">
      <span class="text-xs" style="color: var(--color-text-muted)">显示宽度:</span>
      <input type="range" min="200" max="600" bind:value={screenFitWidth} class="w-24" />
      <span class="text-xs" style="color: var(--color-text-muted)">{screenFitWidth}px</span>
    </div>
  </div>

  <div class="flex gap-4">
    <div class="flex-shrink-0">
      <ScreenCanvas
        serial={selectedDevice}
        fitWidth={screenFitWidth}
        onKey={onSendKey}
        onInput={onSendInputText}
      />
    </div>

    <div class="flex-1 space-y-4">
      <ScreenControls
        {screenWidth} {screenHeight} {screenRefreshing} {screenAutoRefresh}
        {inputText} {holdDuration} {holdActive} {recording}
        onRefresh={onRefresh}
        onToggleAutoRefresh={onToggleAutoRefresh}
        onToggleRecording={onToggleRecording}
        onSendKey={onSendKey}
        onSendKeyCombo={onSendKeyCombo}
        onSendInputText={onSendInputText}
        onSwipeScreen={onSwipeScreen}
        onRotateScreen={onRotateScreen}
        onInputChange={onInputChange}
        onHoldDurationChange={onHoldDurationChange}
      />
    </div>
  </div>
</div>

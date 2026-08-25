<script lang="ts">
  import { apiGet, apiPost } from '../device-api';
  import RemoteControl from './RemoteControl.svelte';

  let {
    serial,
    onMsg
  }: {
    serial: string;
    onMsg: (err: string, ok?: string) => void;
  } = $props();

  let screenImage = $state('');
  let screenWidth = $state(0);
  let screenHeight = $state(0);
  let screenLoading = $state(false);
  let screenRefreshing = $state(false);
  let inputText = $state('');
  let screenFitWidth = $state(360);
  let tapIndicator = $state<{x: number, y: number} | null>(null);
  let screenAutoRefresh = $state(false);
  let screenAutoRefreshTimer: ReturnType<typeof setInterval> | null = null;
  let holdDuration = $state(1000);
  let holdActive = $state(false);
  let recording = $state(false);
  let screenRotation = $state('portrait');

  // Touch gesture state
  let touchStartTime = $state(0);
  let touchStartX = $state(0);
  let touchStartY = $state(0);
  let touchStartScreenX = $state(0);
  let touchStartScreenY = $state(0);

  async function loadScreenInfo() {
    if (!serial) return;
    screenLoading = true;
    try {
      const sizeRes = await apiGet(`/api/v1/adb/screen/size?serial=${serial}`);
      if (sizeRes) { screenWidth = sizeRes.width; screenHeight = sizeRes.height; }
      await refreshScreen();
    } catch (e) { console.error('load screen info failed:', e); }
    screenLoading = false;
  }

  async function refreshScreen() {
    if (!serial || screenRefreshing) return;
    screenRefreshing = true;
    try {
      const res = await apiGet(`/api/v1/adb/screen/screenshot?serial=${serial}`);
      if (res?.image_base64) { screenImage = `data:image/png;base64,${res.image_base64}`; }
    } catch (e) { console.error('refresh screen failed:', e); }
    screenRefreshing = false;
  }

  async function tapScreen(x: number, y: number) {
    if (!serial) return;
    apiPost('/api/v1/adb/screen/tap', { serial, x, y });
    setTimeout(refreshScreen, 150);
  }

  async function swipeScreen(x1: number, y1: number, x2: number, y2: number, duration: number = 300) {
    if (!serial) return;
    apiPost('/api/v1/adb/screen/swipe', { serial, x1, y1, x2, y2, duration });
    setTimeout(refreshScreen, Math.max(150, duration + 100));
  }

  async function sendKey(key: string) {
    if (!serial) return;
    apiPost('/api/v1/adb/screen/key', { serial, key });
    setTimeout(refreshScreen, 150);
  }

  async function sendInputText() {
    if (!serial || !inputText) return;
    apiPost('/api/v1/adb/screen/input', { serial, text: inputText });
    inputText = '';
    setTimeout(refreshScreen, 150);
  }

  function getDeviceCoords(clientX: number, clientY: number, imgEl: HTMLImageElement): {x: number, y: number} {
    const rect = imgEl.getBoundingClientRect();
    const scaleX = screenWidth / rect.width;
    const scaleY = screenHeight / rect.height;
    const x = Math.round((clientX - rect.left) * scaleX);
    const y = Math.round((clientY - rect.top) * scaleY);
    return { x: Math.max(0, Math.min(x, screenWidth)), y: Math.max(0, Math.min(y, screenHeight)) };
  }

  function handleScreenClick(e: MouseEvent) {
    const img = e.currentTarget as HTMLImageElement;
    const { x, y } = getDeviceCoords(e.clientX, e.clientY, img);
    const rect = img.getBoundingClientRect();
    tapIndicator = { x: e.clientX - rect.left, y: e.clientY - rect.top };
    setTimeout(() => tapIndicator = null, 500);
    tapScreen(x, y);
  }

  function handleTouchStart(e: TouchEvent) {
    const touch = e.touches[0];
    touchStartTime = Date.now();
    touchStartX = touch.clientX;
    touchStartY = touch.clientY;
    const img = e.currentTarget as HTMLImageElement;
    const { x, y } = getDeviceCoords(touch.clientX, touch.clientY, img);
    touchStartScreenX = x;
    touchStartScreenY = y;
  }

  function handleTouchEnd(e: TouchEvent) {
    const touch = e.changedTouches[0];
    const endTime = Date.now();
    const dt = endTime - touchStartTime;
    const dx = touch.clientX - touchStartX;
    const dy = touch.clientY - touchStartY;
    const img = e.currentTarget as HTMLImageElement;
    const rect = img.getBoundingClientRect();

    tapIndicator = { x: touchStartX - rect.left, y: touchStartY - rect.top };
    setTimeout(() => tapIndicator = null, 500);

    const distThreshold = 15;
    if (Math.abs(dx) < distThreshold && Math.abs(dy) < distThreshold) {
      if (dt < 300) {
        tapScreen(touchStartScreenX, touchStartScreenY);
      } else if (dt >= 300) {
        swipeScreen(touchStartScreenX, touchStartScreenY, touchStartScreenX, touchStartScreenY, Math.min(dt, 3000));
      }
    } else {
      const end = getDeviceCoords(touch.clientX, touch.clientY, img);
      const swipeDuration = Math.max(100, Math.min(dt, 1000));
      swipeScreen(touchStartScreenX, touchStartScreenY, end.x, end.y, swipeDuration);
    }
  }

  function toggleScreenAutoRefresh() {
    screenAutoRefresh = !screenAutoRefresh;
    if (screenAutoRefresh) {
      screenAutoRefreshTimer = setInterval(() => {
        if (!screenRefreshing && serial) {
          refreshScreen();
        }
      }, 100);
    } else {
      if (screenAutoRefreshTimer) {
        clearInterval(screenAutoRefreshTimer);
        screenAutoRefreshTimer = null;
      }
    }
  }

  async function toggleScreenRecord() {
    if (!serial) return;
    if (recording) {
      await apiPost('/api/v1/adb/screen/record', { serial, action: 'stop' });
      recording = false;
      onMsg('', '屏幕录制已停止，文件已保存');
    } else {
      await apiPost('/api/v1/adb/screen/record', { serial, action: 'start' });
      recording = true;
      onMsg('', '屏幕录制已开始');
    }
  }

  async function rotateScreen(dir: string) {
    if (!serial) return;
    await apiPost('/api/v1/adb/screen/key', { serial, key: `KEYCODE_ROTATE` });
    screenRotation = dir;
    setTimeout(refreshScreen, 300);
  }

  async function holdScreen(x: number, y: number) {
    if (!serial) return;
    holdActive = true;
    apiPost('/api/v1/adb/screen/swipe', { serial, x1: x, y1: y, x2: x, y2: y, duration: holdDuration });
    setTimeout(() => { holdActive = false; refreshScreen(); }, holdDuration + 150);
  }

  async function sendKeyCombo(keys: string[]) {
    if (!serial) return;
    for (const key of keys) {
      apiPost('/api/v1/adb/screen/key', { serial, key });
      await new Promise(r => setTimeout(r, 50));
    }
    setTimeout(refreshScreen, 150);
  }

  $effect(() => {
    if (serial) loadScreenInfo();
    return () => {
      if (screenAutoRefreshTimer) {
        clearInterval(screenAutoRefreshTimer);
        screenAutoRefreshTimer = null;
      }
    };
  });
</script>

<div class="space-y-4">
  <div class="flex items-center gap-2 flex-wrap">
    <button class="btn-primary text-xs" onclick={refreshScreen} disabled={screenRefreshing}>
      <span class="material-symbols-outlined text-[14px]">refresh</span>
      {screenRefreshing ? '刷新中...' : '刷新截图'}
    </button>
    <button
      class="text-xs px-3 py-1.5 rounded-md"
      style={screenAutoRefresh ? 'background: var(--color-primary); color: white' : 'background: var(--color-surface); color: var(--color-text-muted); border: 1px solid var(--color-border)'}
      onclick={toggleScreenAutoRefresh}
    >
      <span class="material-symbols-outlined text-[14px] align-middle">{screenAutoRefresh ? 'pause' : 'play_arrow'}</span>
      {screenAutoRefresh ? '停止自动刷新' : '自动刷新'}
    </button>
    <button class="btn-ghost text-xs" onclick={toggleScreenRecord}>
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

  <div class="flex flex-col md:flex-row gap-4">
    <div class="w-full md:w-auto md:flex-shrink-0 flex justify-center">
      <RemoteControl
        {serial}
        {screenFitWidth}
        {screenWidth}
        {screenHeight}
        {screenRefreshing}
        {screenAutoRefresh}
        {inputText}
        {holdDuration}
        {holdActive}
        {recording}
        onRefresh={refreshScreen}
        onToggleAutoRefresh={toggleScreenAutoRefresh}
        onToggleRecording={toggleScreenRecord}
        onSendKey={(key) => sendKey(key)}
        onSendKeyCombo={(keys) => sendKeyCombo(keys)}
        onSendInputText={(text) => { inputText = text; sendInputText(); }}
        onSwipeScreen={(x1, y1, x2, y2, dur) => swipeScreen(x1, y1, x2, y2, dur)}
        onRotateScreen={(dir) => rotateScreen(dir)}
        onInputChange={(v) => inputText = v}
        onHoldDurationChange={(v) => holdDuration = v}
      />
    </div>
  </div>
</div>

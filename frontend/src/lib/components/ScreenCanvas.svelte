<script lang="ts">
  let {
    serial = '',
    onKey = (_key: string) => {},
    onInput = (_text: string) => {},
    fitWidth = 360,
  }: {
    serial: string;
    onKey?: (key: string) => void;
    onInput?: (text: string) => void;
    fitWidth?: number;
  } = $props();

  let canvasEl = $state<HTMLCanvasElement | null>(null);
  let ws: WebSocket | null = null;
  let connected = $state(false);
  let deviceWidth = $state(1080);
  let deviceHeight = $state(1920);
  let fps = $state(0);
  let bitrate = $state(0);
  let codecLabel = $state('');
  let frameCount = 0;
  let bytesCount = 0;
  let fpsTimer: ReturnType<typeof setInterval> | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let pingTimer: ReturnType<typeof setInterval> | null = null;
  let intentionalClose = false;
  let lastActivity = Date.now();

  // ─── Resolution settings ───
  let quality = $state(70);
  let scale = $state(4);
  let targetWidth = $state(0); // 0 = auto (use device native)
  let showSettings = $state(false);

  // ─── H.264 WebCodecs decoder ───
  let decoder: VideoDecoder | null = null;
  let canvasCtx: OffscreenCanvasRenderingContext2D | OffscreenCanvasRenderingContext2D | null = null;
  let h264Width = 0;
  let h264Height = 0;

  // ─── PNG fallback ───
  let reuseImg: HTMLImageElement | null = null;
  let pendingFrame: Uint8Array | null = null;
  let renderingFrame = false;

  // Touch state
  let pointerDown = $state(false);
  let startX = $state(0);
  let startY = $state(0);
  let startTime = 0;
  let tapIndicator = $state<{ x: number; y: number } | null>(null);
  let lastTapTime = $state(0);
  let activePointers = new Map<number, { x: number; y: number }>();
  let pinchStartDist = $state(0);
  let isPinching = $state(false);

  function getDeviceCoords(clientX: number, clientY: number): { x: number; y: number } {
    if (!canvasEl) return { x: 0, y: 0 };
    const rect = canvasEl.getBoundingClientRect();
    
    // Account for object-fit: contain letterboxing
    const canvasAspect = deviceWidth / deviceHeight;
    const displayAspect = rect.width / rect.height;
    
    let drawWidth: number, drawHeight: number, offsetX: number, offsetY: number;
    if (displayAspect > canvasAspect) {
      // Display is wider → image is height-constrained
      drawHeight = rect.height;
      drawWidth = drawHeight * canvasAspect;
      offsetX = (rect.width - drawWidth) / 2;
      offsetY = 0;
    } else {
      // Display is taller → image is width-constrained
      drawWidth = rect.width;
      drawHeight = drawWidth / canvasAspect;
      offsetX = 0;
      offsetY = (rect.height - drawHeight) / 2;
    }
    
    const x = Math.round(((clientX - rect.left - offsetX) / drawWidth) * deviceWidth);
    const y = Math.round(((clientY - rect.top - offsetY) / drawHeight) * deviceHeight);
    
    return {
      x: Math.max(0, Math.min(x, deviceWidth)),
      y: Math.max(0, Math.min(y, deviceHeight)),
    };
  }

  // ─── WebCodecs H.264 decoder ───

  function ensureH264Decoder(width: number, height: number) {
    if (decoder && decoder.state === 'configured' && h264Width === width && h264Height === height) {
      return true;
    }
    // Destroy old decoder
    if (decoder) {
      try { decoder.close(); } catch {}
      decoder = null;
    }
    if (!('VideoDecoder' in globalThis)) return false;

    h264Width = width;
    h264Height = height;
    if (canvasEl) {
      canvasEl.width = width;
      canvasEl.height = height;
    }

    decoder = new VideoDecoder({
      output: (frame: VideoFrame) => {
        if (!canvasEl) { frame.close(); return; }
        const ctx = canvasEl.getContext('2d');
        if (!ctx) { frame.close(); return; }
        if (canvasEl.width !== frame.displayWidth || canvasEl.height !== frame.displayHeight) {
          canvasEl.width = frame.displayWidth;
          canvasEl.height = frame.displayHeight;
        }
        ctx.drawImage(frame, 0, 0);
        frame.close();
      },
      error: (e) => {
        console.error('[ScreenWS] VideoDecoder error:', e);
        decoder = null;
      },
    });

    decoder.configure({
      codec: 'avc1.42001E', // H.264 Baseline Level 3.0
      codedWidth: width,
      codedHeight: height,
    });
    return decoder.state === 'configured';
  }

  function handleH264Frame(data: Uint8Array, width: number, height: number, isKeyframe: boolean) {
    if (!ensureH264Decoder(width, height)) return;
    if (!decoder || decoder.state !== 'configured') return;

    try {
      const chunk = new EncodedVideoChunk({
        type: isKeyframe ? 'key' : 'delta',
        timestamp: performance.now() * 1000,
        data: data,
      });
      decoder.decode(chunk);
    } catch (e) {
      console.warn('[ScreenWS] decode error:', e);
    }
  }

  // ─── PNG fallback rendering ───

  function renderPNGFrame(imageBytes: Uint8Array) {
    if (!canvasEl) return;
    if (renderingFrame) {
      pendingFrame = imageBytes;
      return;
    }
    renderingFrame = true;

    if (!reuseImg) reuseImg = new Image();
    const blob = new Blob([imageBytes as BlobPart], { type: 'image/png' });
    const url = URL.createObjectURL(blob);

    reuseImg.onload = () => {
      URL.revokeObjectURL(url);
      if (!canvasEl) { renderingFrame = false; return; }
      const img = reuseImg!;
      const ctx = canvasEl.getContext('2d');
      if (!ctx) { renderingFrame = false; return; }
      if (canvasEl.width !== img.naturalWidth || canvasEl.height !== img.naturalHeight) {
        canvasEl.width = img.naturalWidth;
        canvasEl.height = img.naturalHeight;
      }
      ctx.drawImage(img, 0, 0);
      renderingFrame = false;
      if (pendingFrame) {
        const next = pendingFrame;
        pendingFrame = null;
        requestAnimationFrame(() => renderPNGFrame(next));
      }
    };
    reuseImg.onerror = () => {
      URL.revokeObjectURL(url);
      renderingFrame = false;
    };
    reuseImg.src = url;
  }

  // ─── JPEG rendering (same as PNG but with JPEG mime type) ───

  function renderJPEGFrame(imageBytes: Uint8Array) {
    if (!canvasEl) return;
    if (renderingFrame) {
      pendingFrame = imageBytes;
      return;
    }
    renderingFrame = true;

    if (!reuseImg) reuseImg = new Image();
    const blob = new Blob([imageBytes as BlobPart], { type: 'image/jpeg' });
    const url = URL.createObjectURL(blob);

    reuseImg.onload = () => {
      URL.revokeObjectURL(url);
      if (!canvasEl) { renderingFrame = false; return; }
      const img = reuseImg!;
      const ctx = canvasEl.getContext('2d');
      if (!ctx) { renderingFrame = false; return; }
      if (canvasEl.width !== img.naturalWidth || canvasEl.height !== img.naturalHeight) {
        canvasEl.width = img.naturalWidth;
        canvasEl.height = img.naturalHeight;
      }
      ctx.drawImage(img, 0, 0);
      renderingFrame = false;
      if (pendingFrame) {
        const next = pendingFrame;
        pendingFrame = null;
        requestAnimationFrame(() => renderJPEGFrame(next));
      }
    };
    reuseImg.onerror = () => {
      URL.revokeObjectURL(url);
      renderingFrame = false;
    };
    reuseImg.src = url;
  }

  // ─── WebSocket ───

  function connect() {
    if (!serial) return;
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return;
    intentionalClose = false;
    disconnect();

    const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
    if (!token) return;

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    let url = `${proto}//${location.host}/api/v1/ws/screen?token=${encodeURIComponent(token)}&serial=${encodeURIComponent(serial)}&quality=${quality}&scale=${scale}`;
    if (targetWidth > 0) {
      url += `&width=${targetWidth}`;
    }

    ws = new WebSocket(url);
    ws.binaryType = 'arraybuffer';

    ws.onopen = () => {
      connected = true;
      lastActivity = Date.now();
      fpsTimer = setInterval(() => {
        fps = frameCount;
        bitrate = bytesCount;
        frameCount = 0;
        bytesCount = 0;
      }, 1000);
      // Heartbeat: send ping every 25s to prevent proxy/server timeouts
      pingTimer = setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ action: 'ping' }));
        }
      }, 25000);
    };

    ws.onmessage = (ev) => {
      lastActivity = Date.now();
      if (ev.data instanceof ArrayBuffer) {
        const view = new DataView(ev.data);
        const type = view.getUint16(0);

        if (type === 0x01) {
          const w = view.getUint32(2);
          const h = view.getUint32(6);
          const codec = ev.data.byteLength > 10 ? new Uint8Array(ev.data, 10, 1)[0] : 0;
          const flags = ev.data.byteLength > 11 ? new Uint8Array(ev.data, 11, 1)[0] : 0;
          const payload = new Uint8Array(ev.data, 12);

          if (codec === 0x01) {
            if (!codecLabel) codecLabel = 'H.264';
            handleH264Frame(payload, w, h, (flags & 0x01) !== 0);
          } else if (codec === 0x02) {
            if (!codecLabel) codecLabel = 'JPEG';
            renderJPEGFrame(payload);
          } else {
            if (!codecLabel) codecLabel = 'PNG';
            renderPNGFrame(payload);
          }
          frameCount++;
          bytesCount += payload.length;
        } else if (type === 0x02) {
          deviceWidth = view.getUint32(2);
          deviceHeight = view.getUint32(6);
        }
      } else if (typeof ev.data === 'string') {
        // Text message — could be pong or error
        try {
          const msg = JSON.parse(ev.data);
          if (msg.action === 'error') {
            console.warn('[ScreenWS] server error:', msg.message);
          }
        } catch {}
      }
    };

    ws.onerror = () => { connected = false; };
    ws.onclose = () => {
      connected = false;
      ws = null;
      clearTimers();
      if (!intentionalClose && serial) {
        // Exponential backoff: 1s → 2s → 4s → 8s → max 15s
        const delay = Math.min(15000, 1000 * Math.pow(2, reconnectAttempts));
        reconnectAttempts++;
        reconnectTimer = setTimeout(() => connect(), delay);
      }
    };
  }

  let reconnectAttempts = 0;

  function clearTimers() {
    if (fpsTimer) { clearInterval(fpsTimer); fpsTimer = null; }
    if (pingTimer) { clearInterval(pingTimer); pingTimer = null; }
    if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null; }
  }

  function disconnect() {
    intentionalClose = true;
    reconnectAttempts = 0;
    clearTimers();
    if (ws) { ws.close(); ws = null; }
    connected = false;
    if (decoder) { try { decoder.close(); } catch {} decoder = null; }
  }

  // ─── Page Visibility — reconnect when tab comes back ───

  function handleVisibilityChange() {
    if (document.hidden) {
      // Tab went to background — stop pinging, let connection idle
      if (pingTimer) { clearInterval(pingTimer); pingTimer = null; }
    } else {
      // Tab came back — check if WS is still alive
      if (intentionalClose) return;
      if (ws && ws.readyState === WebSocket.OPEN) {
        // Connection alive, resume heartbeat
        if (!pingTimer && connected) {
          pingTimer = setInterval(() => {
            if (ws && ws.readyState === WebSocket.OPEN) {
              ws.send(JSON.stringify({ action: 'ping' }));
            }
          }, 25000);
        }
      } else {
        // Connection dead, reconnect immediately
        ws = null;
        connected = false;
        reconnectAttempts = 0;
        setTimeout(() => connect(), 200);
      }
    }
  }

  function sendCommand(cmd: object) {
    if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(cmd));
  }

  // ─── Pointer Events ───

  function handlePointerDown(e: PointerEvent) {
    const { x, y } = getDeviceCoords(e.clientX, e.clientY);
    activePointers.set(e.pointerId, { x, y });
    if (activePointers.size === 1) {
      pointerDown = true;
      startX = x; startY = y;
      startTime = Date.now();
    } else if (activePointers.size === 2) {
      isPinching = true;
      pointerDown = false;
      const pts = Array.from(activePointers.values());
      pinchStartDist = Math.hypot(pts[1].x - pts[0].x, pts[1].y - pts[0].y);
    }
    (e.target as HTMLElement)?.setPointerCapture(e.pointerId);
    e.preventDefault();
  }

  function handlePointerUp(e: PointerEvent) {
    const pt = activePointers.get(e.pointerId);
    activePointers.delete(e.pointerId);

    if (isPinching) {
      if (activePointers.size < 2) {
        const remaining = Array.from(activePointers.values());
        if (remaining.length === 1 && pt) {
          const lastPt = remaining[0];
          const newDist = Math.hypot(lastPt.x - pt.x, lastPt.y - pt.y);
          if (Math.abs(newDist - pinchStartDist) > 30) {
            sendCommand({ action: 'pinch', x1: lastPt.x - 100, y1: lastPt.y - 100, x2: lastPt.x + 100, y2: lastPt.y + 100, duration: 300 });
          }
        }
        isPinching = false;
        pinchStartDist = 0;
      }
      return;
    }

    if (!pointerDown || !pt) return;
    pointerDown = false;

    const { x, y } = pt;
    const dt = Date.now() - startTime;
    const dx = Math.abs(x - startX);
    const dy = Math.abs(y - startY);

    const rect = canvasEl?.getBoundingClientRect();
    if (rect) {
      const clientX = (x / deviceWidth) * rect.width + rect.left;
      const clientY = (y / deviceHeight) * rect.height + rect.top;
      tapIndicator = { x: clientX - rect.left, y: clientY - rect.top };
      setTimeout(() => tapIndicator = null, 300);
    }

    if (dx < 15 && dy < 15) {
      const now = Date.now();
      if (now - lastTapTime < 300) {
        sendCommand({ action: 'double_tap', x: startX, y: startY });
        lastTapTime = 0;
      } else if (dt > 500) {
        sendCommand({ action: 'long_press', x: startX, y: startY, duration: Math.min(dt, 2000) });
        lastTapTime = 0;
      } else {
        sendCommand({ action: 'tap', x: startX, y: startY });
        lastTapTime = now;
      }
    } else {
      sendCommand({ action: 'swipe', x1: startX, y1: startY, x2: x, y2: y, duration: Math.max(50, Math.min(dt, 800)) });
    }
  }

  function handlePointerMove(e: PointerEvent) {
    if (!activePointers.has(e.pointerId)) return;
    const { x, y } = getDeviceCoords(e.clientX, e.clientY);
    activePointers.set(e.pointerId, { x, y });
    e.preventDefault();
  }

  function handlePointerCancel(e: PointerEvent) {
    activePointers.delete(e.pointerId);
    if (activePointers.size < 2) isPinching = false;
  }

  // ─── Navigation ───

  function goHome() { sendCommand({ action: 'home' }); }
  function goBack() { sendCommand({ action: 'back' }); }
  function goRecent() { sendCommand({ action: 'recent' }); }

  // ─── Resolution settings ───
  function applySettings() {
    showSettings = false;
    // Reconnect with new quality/scale
    disconnect();
    intentionalClose = false;
    setTimeout(() => connect(), 100);
  }

  // ─── Lifecycle ───

  $effect(() => {
    if (serial) connect();
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
      disconnect();
    };
  });
</script>

<div class="screen-canvas-wrapper">
  <div class="relative inline-block">
    {#if !connected}
      <div
        class="flex items-center justify-center rounded-xl border"
        style="width: {fitWidth}px; height: {fitWidth * 2}px; background: var(--color-surface); border-color: var(--color-border)"
      >
        <div class="text-center">
          <span class="material-symbols-outlined text-4xl block mb-2" style="color: var(--color-text-muted)">wifi_off</span>
          <p class="text-xs" style="color: var(--color-text-muted)">连接中...</p>
        </div>
      </div>
    {:else}
      <canvas
        bind:this={canvasEl}
        class="rounded-xl border cursor-crosshair select-none"
        style="width: {fitWidth}px; max-height: {fitWidth * 2}px; object-fit: contain; touch-action: none; border-color: var(--color-border)"
        onpointerdown={handlePointerDown}
        onpointerup={handlePointerUp}
        onpointermove={handlePointerMove}
        onpointercancel={handlePointerCancel}
      ></canvas>
      {#if tapIndicator}
        <div
          class="absolute w-6 h-6 rounded-full border-2 pointer-events-none"
          style="left: {tapIndicator.x - 12}px; top: {tapIndicator.y - 12}px; border-color: var(--color-primary); animation: ping 0.4s ease-out"
        ></div>
      {/if}
    {/if}
  </div>

  {#if connected}
    <div class="flex items-center gap-3 mt-2">
      <span class="flex items-center gap-1">
        <span class="w-2 h-2 rounded-full" style="background: var(--color-success)"></span>
        <span class="text-xs" style="color: var(--color-text-muted)">{fps} fps</span>
      </span>
      <span class="text-xs" style="color: var(--color-text-muted)">
        {#if bitrate > 1024000}
          {(bitrate / 1024000).toFixed(1)} MB/s
        {:else if bitrate > 1024}
          {(bitrate / 1024).toFixed(0)} KB/s
        {:else}
          {bitrate} B/s
        {/if}
      </span>
      <span class="text-xs" style="color: var(--color-text-muted)">{deviceWidth}×{deviceHeight}</span>
      {#if codecLabel}
        <span class="text-xs" style="color: var(--color-primary)">{codecLabel}</span>
      {/if}
    </div>

    <div class="flex items-center justify-center gap-4 mt-3">
      <button class="nav-btn" onclick={goBack} title="返回">
        <span class="material-symbols-outlined">arrow_back</span>
      </button>
      <button class="nav-btn" onclick={goHome} title="主页">
        <span class="material-symbols-outlined">home</span>
      </button>
      <button class="nav-btn" onclick={goRecent} title="最近任务">
        <span class="material-symbols-outlined">swap_horiz</span>
      </button>
      <div class="relative">
        <button class="nav-btn" onclick={() => showSettings = !showSettings} title="画质设置">
          <span class="material-symbols-outlined">tune</span>
        </button>
        {#if showSettings}
          <div class="settings-panel">
            <div class="settings-row">
              <span class="settings-label">画质</span>
              <div class="settings-btns">
                {#each [{v:40,l:'低'},{v:70,l:'中'},{v:90,l:'高'}] as opt}
                  <button class="settings-opt" class:active={quality===opt.v} onclick={() => quality=opt.v}>{opt.l}</button>
                {/each}
              </div>
            </div>
            <div class="settings-row">
              <span class="settings-label">缩放</span>
              <div class="settings-btns">
                {#each [{v:2,l:'清晰'},{v:4,l:'平衡'},{v:8,l:'流畅'}] as opt}
                  <button class="settings-opt" class:active={scale===opt.v} onclick={() => scale=opt.v}>{opt.l}</button>
                {/each}
              </div>
            </div>
            <div class="settings-row">
              <span class="settings-label">分辨率</span>
              <div class="settings-btns">
                {#each [{v:0,l:'原生'},{v:720,l:'720p'},{v:1080,l:'1080p'},{v:1440,l:'2K'}] as opt}
                  <button class="settings-opt" class:active={targetWidth===opt.v} onclick={() => targetWidth=opt.v}>{opt.l}</button>
                {/each}
              </div>
            </div>
            <button class="settings-apply" onclick={applySettings}>应用</button>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  @keyframes ping {
    0% { transform: scale(1); opacity: 1; }
    100% { transform: scale(2.5); opacity: 0; }
  }
  .nav-btn {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    color: var(--color-text);
    cursor: pointer;
    transition: all 0.15s;
  }
  .nav-btn:hover {
    background: var(--color-bg-elevated);
    border-color: var(--color-primary);
    color: var(--color-primary);
  }
  .nav-btn:active { transform: scale(0.9); }
  .settings-panel {
    position: absolute;
    bottom: 44px;
    right: 0;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 10px;
    padding: 10px;
    min-width: 180px;
    z-index: 10;
    box-shadow: 0 4px 16px rgba(0,0,0,0.15);
  }
  .settings-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
  }
  .settings-label {
    font-size: 12px;
    color: var(--color-text-muted);
  }
  .settings-btns {
    display: flex;
    gap: 4px;
  }
  .settings-opt {
    padding: 2px 8px;
    border-radius: 6px;
    border: 1px solid var(--color-border);
    background: transparent;
    color: var(--color-text-muted);
    font-size: 11px;
    cursor: pointer;
    transition: all 0.15s;
  }
  .settings-opt.active {
    background: var(--color-primary);
    color: white;
    border-color: var(--color-primary);
  }
  .settings-apply {
    width: 100%;
    padding: 6px;
    border-radius: 8px;
    border: none;
    background: var(--color-primary);
    color: white;
    font-size: 12px;
    cursor: pointer;
    margin-top: 4px;
  }
  .settings-apply:hover { opacity: 0.9; }
</style>

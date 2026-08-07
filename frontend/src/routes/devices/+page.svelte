<script lang="ts">
  import { onMount } from 'svelte';
  import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
  import ScreenCanvas from '$lib/components/ScreenCanvas.svelte';
  import DeviceList from './components/DeviceList.svelte';
  import DeviceDetail from './components/DeviceDetail.svelte';
  import ScreenControls from './components/ScreenControls.svelte';
  import RootManager from './components/RootManager.svelte';
  import ModuleDetail from './components/ModuleDetail.svelte';
  import { apiGet, apiPost } from './device-api';
  import type { Device, SavedDevice, DeviceInfo, InstalledModule, AppInfo, FileInfo } from './device-types';

  // Confirmation dialog state
  let confirmOpen = $state(false);
  let confirmTitle = $state('');
  let confirmMessage = $state('');
  let confirmVariant = $state<'primary' | 'danger'>('danger');
  let confirmCallback = $state<(() => void) | null>(null);

  function showConfirm(title: string, message: string, variant: 'primary' | 'danger' = 'danger', callback: () => void) {
    confirmTitle = title;
    confirmMessage = message;
    confirmVariant = variant;
    confirmCallback = callback;
    confirmOpen = true;
  }

  function executeConfirm() {
    if (confirmCallback) confirmCallback();
    confirmOpen = false;
  }

  // State
  let devices = $state<Device[]>([]);
  let savedDevices = $state<SavedDevice[]>([]);
  let selectedDevice = $state('');
  let deviceInfo = $state<DeviceInfo | null>(null);
  let modules = $state<InstalledModule[]>([]);
  let apps = $state<AppInfo[]>([]);
  let files = $state<FileInfo[]>([]);
  let fileSearchQuery = $state('');
  let filteredFiles = $derived(!fileSearchQuery ? files : files.filter(f => f.name.toLowerCase().includes(fileSearchQuery.toLowerCase())));
  let currentPath = $state('/sdcard/');
  let uploading = $state(false);
  let uploadTarget = $state('');
  let fileInput = $state<HTMLInputElement | null>(null);
  let dragOver = $state(false);
  let newFolderName = $state('');
  let renameTarget = $state('');
  let renamePath = $state('');
  let previewContent = $state('');
  let previewPath = $state('');
  let logs = $state('');
  let shellOutput = $state('');
  let shellCmd = $state('');
  let connectAddress = $state('');
  let activeTab = $state<'info' | 'modules' | 'apps' | 'files' | 'logs' | 'shell' | 'screen'>('info');
  let appFilter = $state('all');
  let logFilter = $state('');
  let logLevel = $state('');
  let installing = $state(false);
  let installPath = $state('');
  let errorMsg = $state('');
  let successMsg = $state('');
  let loading = $state(false);

  // Batch operations
  let selectedDevices = $state<Set<string>>(new Set());
  let selectAllDevices = $state(false);

  // Auto refresh
  let autoRefresh = $state(false);
  let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;

  // Screen control enhancements
  let holdDuration = $state(1000);
  let holdActive = $state(false);
  let keyCombo = $state('');
  let screenRotation = $state('portrait');
  let recording = $state(false);

  // Screen control
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

  // Touch gesture state
  let touchStartTime = $state(0);
  let touchStartX = $state(0);
  let touchStartY = $state(0);
  let touchStartScreenX = $state(0);
  let touchStartScreenY = $state(0);

  // Module search/filter (1.5)
  let moduleSearchQuery = $state('');
  let filteredModules = $derived(!moduleSearchQuery ? modules : modules.filter(m =>
    m.name.toLowerCase().includes(moduleSearchQuery.toLowerCase()) ||
    (m.description || '').toLowerCase().includes(moduleSearchQuery.toLowerCase()) ||
    (m.author || '').toLowerCase().includes(moduleSearchQuery.toLowerCase())
  ));

  // Batch module operations (1.3)
  let selectedModules = $state<Set<string>>(new Set());
  let showModuleCheckboxes = $state(false);

  

  // Module detail
  let selectedModuleDetail = $state<InstalledModule | null>(null);

  // Module backup/restore (1.1)
  let backupModuleName = $state('');
  let restoreFileName = $state('');
  let backupRestoreMsg = $state('');
  let showBackupRestore = $state(false);

  // Module update (1.2)
  let moduleUpdateInfo = $state<Record<string, any>>({});
  let updatingCheck = $state<Set<string>>(new Set());

  

  

  function msg(err: string, ok?: string) {
    errorMsg = err;
    successMsg = ok || '';
    setTimeout(() => { errorMsg = ''; successMsg = ''; }, 5000);
  }

  // ─── Device ───
  async function listDevices() {
    loading = true;
    const d = await apiGet('/api/v1/adb/devices');
    devices = d.devices || [];
    if (devices.length > 0 && !selectedDevice) {
      selectedDevice = devices[0].serial;
      loadDeviceInfo();
    }
    loading = false;
  }

  async function loadDeviceInfo() {
    if (!selectedDevice) return;
    const d = await apiGet(`/api/v1/adb/device-info?serial=${selectedDevice}`);
    deviceInfo = d;
  }

  async function loadSavedDevices() {
    const d = await apiGet('/api/v1/adb/saved-devices');
    savedDevices = d.devices || [];
  }

  async function deleteSavedDevice(id: number) {
    const token = localStorage.getItem('moduforge_token') || '';
    await fetch(`/api/v1/adb/saved-devices/${id}`, {
      method: 'DELETE',
      headers: { 'Authorization': `Bearer ${token}` },
    });
    loadSavedDevices();
  }

  function selectSavedDevice(addr: string) {
    connectAddress = addr;
  }

  async function connectDevice() {
    if (!connectAddress.trim()) return;
    const d = await apiPost('/api/v1/adb/connect', { address: connectAddress.trim() });
    if (d.error) { msg(d.error); return; }
    successMsg = d.output || 'Connected';
    listDevices();
    loadSavedDevices();
  }

  async function disconnectDevice(addr: string) {
    const d = await apiPost('/api/v1/adb/disconnect', { address: addr });
    if (d.error) { msg(d.error); return; }
    listDevices();
  }

  async function rebootDevice(mode: string) {
    if (!selectedDevice) return;
    await apiPost('/api/v1/adb/reboot', { serial: selectedDevice, mode });
    msg('', 'Rebooting...');
  }

  // ─── Modules ───
  async function loadModules() {
    if (!selectedDevice) return;
    const d = await apiGet(`/api/v1/adb/modules?serial=${selectedDevice}`);
    modules = d.modules || [];
    moduleSearchQuery = '';
  }

  async function toggleModule(name: string) {
    const mod = modules.find(m => m.name === name);
    await apiPost(`/api/v1/adb/modules/${name}/toggle`, { serial: selectedDevice, enable: !mod?.enabled });
    loadModules();
  }

  async function uninstallModule(name: string) {
    await apiPost(`/api/v1/adb/modules/${name}/uninstall`, { serial: selectedDevice });
    loadModules();
  }

  async function installModule() {
    if (!installPath.trim()) return;
    installing = true;
    const d = await apiPost('/api/v1/adb/install', { serial: selectedDevice, zip_path: installPath.trim() });
    if (d.error) { msg(d.error); } else { successMsg = 'Installed'; installPath = ''; loadModules(); }
    installing = false;
  }

  // ─── Module Batch Operations (1.3) ───
  function toggleModuleSelect(name: string) {
    const next = new Set(selectedModules);
    if (next.has(name)) next.delete(name); else next.add(name);
    selectedModules = next;
  }

  function selectAllModules() {
    if (selectedModules.size === filteredModules.length) {
      selectedModules = new Set();
    } else {
      selectedModules = new Set(filteredModules.map(m => m.name));
    }
  }

  async function batchToggleModules(enable: boolean) {
    if (selectedModules.size === 0) { msg('请先选择模块'); return; }
    for (const name of selectedModules) {
      await apiPost(`/api/v1/adb/modules/${name}/toggle`, { serial: selectedDevice, enable });
    }
    selectedModules = new Set();
    loadModules();
  }

  async function batchUninstallModules() {
    if (selectedModules.size === 0) { msg('请先选择模块'); return; }
    showConfirm('批量卸载', `确定要卸载 ${selectedModules.size} 个模块吗？`, 'danger', async () => {
      for (const name of selectedModules) {
        await apiPost(`/api/v1/adb/modules/${name}/uninstall`, { serial: selectedDevice });
      }
      selectedModules = new Set();
      loadModules();
    });
  }

  // ─── Module Backup/Restore (1.1) ───

  

  // ─── Module Backup/Restore (1.1) ───
  async function backupModule(name: string) {
    const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
    const res = await fetch('/api/v1/adb/module/backup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ serial: selectedDevice, module_name: name }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: '下载失败' }));
      msg(err.error || '下载失败');
      return;
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = name + '_backup.tar.gz';
    document.body.appendChild(a); a.click(); a.remove();
    URL.revokeObjectURL(url);
    msg('', '备份已下载');
  }

  async function restoreModule() {
    if (!restoreFileName.trim()) { msg('请输入备份文件路径'); return; }
    const d = await apiPost('/api/v1/adb/module/restore', { serial: selectedDevice, local_path: restoreFileName.trim() });
    if (d.error) { msg(d.error); } else { msg('', d.output || '模块已恢复'); restoreFileName = ''; loadModules(); }
  }

  async function exportModule(name: string) {
    const d = await apiPost('/api/v1/adb/module/export', { serial: selectedDevice, module_name: name });
    if (d.error) { msg(d.error); } else { msg('', `模块已导出至设备: ${d.export_path}`); }
  }

  // ─── Module Update Check (1.2) ───
  async function checkModuleUpdate(name: string) {
    updatingCheck = new Set(updatingCheck).add(name);
    const d = await apiGet(`/api/v1/adb/module/check-update?serial=${selectedDevice}&module_name=${name}`);
    moduleUpdateInfo = { ...moduleUpdateInfo, [name]: d };
    updatingCheck = new Set([...updatingCheck].filter(x => x !== name));
    if (d.has_update) {
      msg('', `模块 ${name} 有新版本: ${d.latest_version}`);
    } else {
      msg('', `模块 ${name} 已是最新版本`);
    }
  }

  

  // ─── Apps ───
  async function loadApps() {
    if (!selectedDevice) return;
    const filter = appFilter !== 'all' ? `&filter=${appFilter}` : '';
    const d = await apiGet(`/api/v1/adb/apps?serial=${selectedDevice}${filter}`);
    apps = d.apps || [];
  }

  async function uninstallApp(pkg: string) {
    await apiPost('/api/v1/adb/app/uninstall', { serial: selectedDevice, package: pkg });
    loadApps();
  }

  async function forceStopApp(pkg: string) {
    await apiPost('/api/v1/adb/app/force-stop', { serial: selectedDevice, package: pkg });
  }

  async function launchApp(pkg: string) {
    const d = await apiPost('/api/v1/adb/app/launch', { serial: selectedDevice, package: pkg });
    if (d.error) msg(d.error);
  }

  async function clearAppData(pkg: string) {
    await apiPost('/api/v1/adb/app/clear-data', { serial: selectedDevice, package: pkg });
  }

  async function toggleApp(pkg: string, enable: boolean) {
    await apiPost('/api/v1/adb/app/toggle', { serial: selectedDevice, package: pkg, enable });
    loadApps();
  }

  // ─── Files ───
  async function loadFiles(path?: string) {
    if (!selectedDevice) return;
    if (path !== undefined) currentPath = path;
    const d = await apiGet(`/api/v1/adb/files?serial=${selectedDevice}&path=${encodeURIComponent(currentPath)}`);
    files = d.files || [];
    currentPath = d.path || currentPath;
  }

  function navigateTo(path: string) {
    loadFiles(path);
  }

  function goUp() {
    if (currentPath === '/') return;
    const parts = currentPath.split('/').filter(Boolean);
    parts.pop();
    const parent = parts.length === 0 ? '/' : '/' + parts.join('/') + '/';
    loadFiles(parent);
  }

  async function deleteFile(path: string) {
    await apiPost('/api/v1/adb/delete', { serial: selectedDevice, remote_path: path });
    loadFiles();
  }

  async function downloadFile(path: string) {
    const token = localStorage.getItem('moduforge_token') || '';
    const res = await fetch(`/api/v1/adb/file/download?serial=${encodeURIComponent(selectedDevice)}&path=${encodeURIComponent(path)}`, {
      headers: { 'Authorization': `Bearer ${token}` },
    });
    if (!res.ok) { msg('下载失败'); return; }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = path.split('/').filter(Boolean).pop() || 'file';
    a.click();
    URL.revokeObjectURL(url);
    msg('', '下载完成');
  }

  async function mkdir() {
    if (!newFolderName.trim()) { msg('请输入文件夹名'); return; }
    const d = await apiPost('/api/v1/adb/mkdir', { serial: selectedDevice, remote_path: currentPath + newFolderName.trim() });
    if (d.error) { msg(d.error); return; }
    newFolderName = '';
    msg('', '文件夹已创建');
    loadFiles();
  }

  async function renameFile(oldPath: string) {
    const newName = renameTarget.trim();
    if (!newName) { msg('请输入新文件名'); return; }
    const parts = oldPath.split('/');
    parts[parts.length - 1] = newName;
    const newPath = parts.join('/');
    const d = await apiPost('/api/v1/adb/rename', { serial: selectedDevice, old_path: oldPath, new_path: newPath });
    if (d.error) { msg(d.error); return; }
    renameTarget = '';
    renamePath = '';
    msg('', '重命名成功');
    loadFiles();
  }

  async function previewFile(path: string) {
    const token = localStorage.getItem('moduforge_token') || '';
    const res = await fetch('/api/v1/adb/file/read', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ serial: selectedDevice, remote_path: path }),
    });
    const d = await res.json();
    if (d.error) { msg(d.error); return; }
    previewContent = d.content || '(空文件)';
    previewPath = path;
  }

  function closePreview() {
    previewPath = '';
    previewContent = '';
  }

  async function uploadFile() {
    if (!fileInput?.files?.length) { msg('请选择文件'); return; }
    const targetPath = uploadTarget.trim() || currentPath;
    uploading = true;
    const token = localStorage.getItem('moduforge_token') || '';
    const form = new FormData();
    form.append('serial', selectedDevice);
    form.append('remote_path', targetPath.endsWith('/') ? targetPath + fileInput.files[0].name : targetPath);
    form.append('file', fileInput.files[0]);
    const res = await fetch('/api/v1/adb/file/upload', {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}` },
      body: form,
    });
    const d = await res.json();
    uploading = false;
    if (d.error) { msg(d.error); return; }
    fileInput.value = '';
    uploadTarget = '';
    msg('', '上传成功');
    loadFiles();
  }

  // ─── Logs ───
  async function loadLogs() {
    if (!selectedDevice) return;
    const params = new URLSearchParams({ serial: selectedDevice });
    if (logFilter) params.set('filter', logFilter);
    if (logLevel) params.set('level', logLevel);
    const d = await apiGet(`/api/v1/adb/logcat?${params}`);
    logs = d.logs || '';
  }

  async function clearLogs() {
    await apiPost('/api/v1/adb/logcat/clear', { serial: selectedDevice });
    logs = '';
  }

  // ─── Shell ───
  async function runShell() {
    if (!shellCmd.trim()) return;
    const d = await apiPost('/api/v1/adb/shell', { serial: selectedDevice, command: shellCmd.trim() });
    shellOutput = d.error || d.output || '';
  }

  // ─── Screen Control ───
  async function loadScreenInfo() {
    if (!selectedDevice) return;
    screenLoading = true;
    try {
      const sizeRes = await apiGet(`/api/v1/adb/screen/size?serial=${selectedDevice}`);
      if (sizeRes) { screenWidth = sizeRes.width; screenHeight = sizeRes.height; }
      await refreshScreen();
    } catch {}
    screenLoading = false;
  }

  async function refreshScreen() {
    if (!selectedDevice || screenRefreshing) return; // guard against overlapping requests
    screenRefreshing = true;
    try {
      const res = await apiGet(`/api/v1/adb/screen/screenshot?serial=${selectedDevice}`);
      if (res?.image_base64) { screenImage = `data:image/png;base64,${res.image_base64}`; }
    } catch {}
    screenRefreshing = false;
  }

  async function tapScreen(x: number, y: number) {
    if (!selectedDevice) return;
    apiPost('/api/v1/adb/screen/tap', { serial: selectedDevice, x, y }); // fire-and-forget
    setTimeout(refreshScreen, 150);
  }

  async function swipeScreen(x1: number, y1: number, x2: number, y2: number, duration: number = 300) {
    if (!selectedDevice) return;
    apiPost('/api/v1/adb/screen/swipe', { serial: selectedDevice, x1, y1, x2, y2, duration }); // fire-and-forget
    setTimeout(refreshScreen, Math.max(150, duration + 100));
  }

  async function sendKey(key: string) {
    if (!selectedDevice) return;
    apiPost('/api/v1/adb/screen/key', { serial: selectedDevice, key }); // fire-and-forget
    setTimeout(refreshScreen, 150);
  }

  async function sendInputText() {
    if (!selectedDevice || !inputText) return;
    apiPost('/api/v1/adb/screen/input', { serial: selectedDevice, text: inputText }); // fire-and-forget
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

    const distThreshold = 15; // pixels — minimum movement to count as swipe
    if (Math.abs(dx) < distThreshold && Math.abs(dy) < distThreshold) {
      if (dt < 300) {
        // Tap
        tapScreen(touchStartScreenX, touchStartScreenY);
      } else if (dt >= 300) {
        // Long press
        swipeScreen(touchStartScreenX, touchStartScreenY, touchStartScreenX, touchStartScreenY, Math.min(dt, 3000));
      }
    } else {
      // Swipe — use the endpoint
      const end = getDeviceCoords(touch.clientX, touch.clientY, img);
      const swipeDuration = Math.max(100, Math.min(dt, 1000));
      swipeScreen(touchStartScreenX, touchStartScreenY, end.x, end.y, swipeDuration);
    }
  }

  function toggleScreenAutoRefresh() {
    screenAutoRefresh = !screenAutoRefresh;
    if (screenAutoRefresh) {
      screenAutoRefreshTimer = setInterval(() => {
        if (!screenRefreshing && selectedDevice) {
          refreshScreen();
        }
      }, 100); // ~10fps
    } else {
      if (screenAutoRefreshTimer) {
        clearInterval(screenAutoRefreshTimer);
        screenAutoRefreshTimer = null;
      }
    }
  }

  // ─── Init ───
  function switchTab(tab: typeof activeTab) {
    // Stop screen auto-refresh when leaving screen tab
    if (activeTab === 'screen' && tab !== 'screen' && screenAutoRefresh) {
      screenAutoRefresh = false;
      if (screenAutoRefreshTimer) {
        clearInterval(screenAutoRefreshTimer);
        screenAutoRefreshTimer = null;
      }
    }
    activeTab = tab;
    if (tab === 'modules') loadModules();
    if (tab === 'apps') loadApps();
    if (tab === 'files') loadFiles();
    if (tab === 'logs') loadLogs();
    if (tab === 'info') loadDeviceInfo();
    if (tab === 'screen') loadScreenInfo();
  }

  function formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatUptime(secs: string): string {
    const s = parseFloat(secs);
    if (isNaN(s)) return secs;
    const d = Math.floor(s / 86400);
    const h = Math.floor((s % 86400) / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (d > 0) return `${d}天${h}小时${m}分钟`;
    if (h > 0) return `${h}小时${m}分钟`;
    return `${m}分钟`;
  }

  // ─── Auto Refresh ───
  $effect(() => {
    if (autoRefresh && selectedDevice) {
      autoRefreshTimer = setInterval(() => {
        loadDeviceInfo();
      }, 5000);
    } else {
      if (autoRefreshTimer) {
        clearInterval(autoRefreshTimer);
        autoRefreshTimer = null;
      }
    }
    return () => {
      if (autoRefreshTimer) {
        clearInterval(autoRefreshTimer);
        autoRefreshTimer = null;
      }
    };
  });

  // ─── Drag & Drop Upload ───
  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    dragOver = true;
  }

  function handleDragLeave() {
    dragOver = false;
  }

  async function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
    const files = e.dataTransfer?.files;
    if (!files || files.length === 0) return;
    const token = localStorage.getItem('moduforge_token') || '';
    for (let i = 0; i < files.length; i++) {
      const form = new FormData();
      form.append('serial', selectedDevice);
      form.append('remote_path', currentPath + files[i].name);
      form.append('file', files[i]);
      await fetch('/api/v1/adb/file/upload', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
        body: form,
      });
    }
    msg('', `已上传 ${files.length} 个文件`);
    loadFiles();
  }

  // ─── Screen Control Enhancements ───
  async function holdScreen(x: number, y: number) {
    if (!selectedDevice) return;
    holdActive = true;
    apiPost('/api/v1/adb/screen/swipe', { serial: selectedDevice, x1: x, y1: y, x2: x, y2: y, duration: holdDuration }); // fire-and-forget
    setTimeout(() => { holdActive = false; refreshScreen(); }, holdDuration + 150);
  }

  async function sendKeyCombo(keys: string[]) {
    if (!selectedDevice) return;
    for (const key of keys) {
      apiPost('/api/v1/adb/screen/key', { serial: selectedDevice, key }); // fire-and-forget
      await new Promise(r => setTimeout(r, 50)); // minimal delay between keys
    }
    setTimeout(refreshScreen, 150);
  }

  async function rotateScreen(dir: string) {
    if (!selectedDevice) return;
    await apiPost('/api/v1/adb/screen/key', { serial: selectedDevice, key: `KEYCODE_ROTATE` });
    screenRotation = dir;
    setTimeout(refreshScreen, 300);
  }

  async function toggleScreenRecord() {
    if (!selectedDevice) return;
    if (recording) {
      await apiPost('/api/v1/adb/screen/record', { serial: selectedDevice, action: 'stop' });
      recording = false;
      msg('', '屏幕录制已停止，文件已保存');
    } else {
      await apiPost('/api/v1/adb/screen/record', { serial: selectedDevice, action: 'start' });
      recording = true;
      msg('', '屏幕录制已开始');
    }
  }

  async function batchAction(action: string) {
    if (selectedDevices.size === 0) { msg('请先选择设备'); return; }
    for (const serial of selectedDevices) {
      if (action === 'reboot') {
        await apiPost('/api/v1/adb/reboot', { serial, mode: '' });
      }
    }
    msg('', `批量操作完成: ${action}`);
  }

  function toggleDeviceSelect(serial: string) {
    const next = new Set(selectedDevices);
    if (next.has(serial)) next.delete(serial);
    else next.add(serial);
    selectedDevices = next;
  }

  function toggleSelectAll() {
    if (selectAllDevices) {
      selectedDevices = new Set();
      selectAllDevices = false;
    } else {
      selectedDevices = new Set(devices.map(d => d.serial));
      selectAllDevices = true;
    }
  }

  // ─── On mount cleanup ───
  onMount(() => {
    listDevices();
    loadSavedDevices();
    return () => {
      if (autoRefreshTimer) clearInterval(autoRefreshTimer);
      if (screenAutoRefreshTimer) clearInterval(screenAutoRefreshTimer);
    };
  });
</script>

<ConfirmDialog
  open={confirmOpen}
  title={confirmTitle}
  message={confirmMessage}
  variant={confirmVariant}
  onConfirm={executeConfirm}
  onCancel={() => confirmOpen = false}
/>

<div class="p-4 md:p-6 max-w-7xl mx-auto">
  <!-- Header -->
  <div class="flex items-center justify-between mb-6">
    <div>
      <h1 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">设备管理</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">ADB 设备连接、模块管理、应用管理、文件浏览</p>
    </div>
    <div class="flex items-center gap-2">
      <button class="btn-ghost flex items-center gap-1.5 text-sm" onclick={() => { listDevices(); if (selectedDevice) { loadDeviceInfo(); } }}>
        <span class="material-symbols-outlined text-[16px]">refresh</span>
        刷新
      </button>
    </div>
  </div>

  {#if errorMsg}
    <div class="mb-4 px-4 py-3 rounded-xl text-sm" style="background: var(--color-error-light); color: var(--color-error)">
      {errorMsg}
    </div>
  {/if}
  {#if successMsg}
    <div class="mb-4 px-4 py-3 rounded-xl text-sm" style="background: var(--color-success-light); color: var(--color-success)">
      {successMsg}
    </div>
  {/if}

  <!-- Device Selector + Connect -->
  <DeviceList
    devices={devices}
    selectedDevice={selectedDevice}
    selectedDevices={selectedDevices}
    loading={loading}
    onSelect={(serial) => { selectedDevice = serial; loadDeviceInfo(); switchTab(activeTab); }}
    onSelectBatch={(s) => { selectedDevices = s; }}
    onConnect={(addr) => { connectAddress = addr; connectDevice(); }}
    onRefresh={() => { listDevices(); if (selectedDevice) loadDeviceInfo(); }}
  />

  {#if selectedDevice}
    <!-- Tabs -->
    <div class="flex gap-1 mb-6 overflow-x-auto border-b" style="border-color: var(--color-border)">
      {#each [
        { key: 'info', icon: 'phone_android', label: '设备信息' },
        { key: 'modules', icon: 'extension', label: '模块' },
        { key: 'apps', icon: 'apps', label: '应用' },
        { key: 'files', icon: 'folder', label: '文件' },
        { key: 'screen', icon: 'screen_lock_portrait', label: '屏幕' },
        { key: 'logs', icon: 'terminal', label: '日志' },
        { key: 'shell', icon: 'code', label: '终端' },
      ] as tab}
        <button
          class="flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium transition-colors whitespace-nowrap"
          style={activeTab === tab.key
            ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary)'
            : 'color: var(--color-text-secondary)'}
          onclick={() => switchTab(tab.key as any)}
        >
          <span class="material-symbols-outlined text-[18px]">{tab.icon}</span>
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Tab Content -->
    {#if activeTab === 'info'}
      <div class="flex items-center gap-2 mb-3">
        <button class="btn-ghost text-xs flex items-center gap-1" onclick={() => loadDeviceInfo()}>
          <span class="material-symbols-outlined text-[14px]">refresh</span>
          刷新
        </button>
        <button
          class="text-xs px-3 py-1.5 rounded-md"
          style={autoRefresh ? 'background: var(--color-primary); color: white' : 'background: var(--color-surface); color: var(--color-text-muted)'}
          onclick={() => autoRefresh = !autoRefresh}
        >
          {autoRefresh ? '自动刷新中 (5s)' : '自动刷新'}
        </button>
      </div>

      <DeviceDetail deviceInfo={deviceInfo} loading={loading} />

      <RootManager serial={selectedDevice} onMsg={msg} />

      <!-- Install Module -->
      <div class="info-card p-5 min-w-0">
        <h3 class="text-sm font-semibold mb-3" style="color: var(--color-text)">安装模块</h3>
        <div class="flex gap-2">
          <input type="text" class="input-field flex-1" placeholder="设备上的 ZIP 文件路径，如 /sdcard/module.zip" bind:value={installPath} />
          <button class="btn-primary text-sm" disabled={installing} onclick={installModule}>
            {installing ? '安装中...' : '安装'}
          </button>
        </div>
      </div>

    {:else if activeTab === 'modules'}
      <div class="info-card overflow-x-auto">
        <div class="p-4 border-b flex items-center justify-between" style="border-color: var(--color-border)">
          <span class="text-sm font-semibold" style="color: var(--color-text)">已安装模块 ({modules.length})</span>
          <div class="flex items-center gap-2">
            <button class="btn-ghost text-xs" onclick={() => showModuleCheckboxes = !showModuleCheckboxes}>
              {showModuleCheckboxes ? '取消选择' : '批量'}
            </button>
            <button class="btn-ghost text-xs" onclick={() => showBackupRestore = !showBackupRestore}>
              {showBackupRestore ? '关闭' : '备份/恢复'}
            </button>
            <button class="btn-ghost text-xs" onclick={loadModules}>刷新</button>
          </div>
        </div>

        <!-- Search Bar (1.5) -->
        <div class="p-3 border-b" style="border-color: var(--color-border); background: var(--color-surface)">
          <div class="flex items-center gap-2">
            <input type="text" class="input-field text-xs flex-1" placeholder="搜索模块名称、描述、作者..." bind:value={moduleSearchQuery}>
            {#if moduleSearchQuery}
              <button class="btn-ghost text-xs" onclick={() => { moduleSearchQuery = ''; }}>清除</button>
            {/if}
          </div>
        </div>

        <!-- Backup/Restore Panel (1.1) -->
        {#if showBackupRestore}
          <div class="p-3 border-b space-y-2" style="border-color: var(--color-border); background: var(--color-surface)">
            <div class="text-xs font-medium" style="color: var(--color-text-secondary)">备份 / 恢复 / 导出</div>
            <div class="flex items-center gap-2">
              <input type="text" class="input-field text-xs flex-1" placeholder="模块名（备份当前模块）" bind:value={backupModuleName} />
              <button class="btn-primary text-xs" onclick={() => { if (backupModuleName.trim()) backupModule(backupModuleName.trim()); }}>备份</button>
            </div>
            <div class="flex items-center gap-2">
              <input type="text" class="input-field text-xs flex-1" placeholder="本地 ZIP 路径（恢复）" bind:value={restoreFileName} />
              <button class="btn-primary text-xs" onclick={restoreModule}>恢复</button>
            </div>
            {#if backupRestoreMsg}
              <div class="text-xs" style="color: var(--color-success)">{backupRestoreMsg}</div>
            {/if}
          </div>
        {/if}

        <!-- Batch Operations Bar (1.3) -->
        {#if showModuleCheckboxes && filteredModules.length > 0}
          <div class="p-3 border-b" style="border-color: var(--color-border); background: var(--color-surface)">
            <div class="flex items-center gap-2 text-xs">
              <button class="btn-ghost text-xs" onclick={selectAllModules}>全选/取消</button>
              <span style="color: var(--color-text-muted)">已选 {selectedModules.size} 个</span>
              <button class="btn-ghost text-xs" style="color: var(--color-success)" onclick={() => batchToggleModules(true)}>批量启用</button>
              <button class="btn-ghost text-xs" style="color: var(--color-warning)" onclick={() => batchToggleModules(false)}>批量禁用</button>
              <button class="btn-ghost text-xs" style="color: var(--color-error)" onclick={batchUninstallModules}>批量卸载</button>
            </div>
          </div>
        {/if}

        {#if filteredModules.length === 0}
          <div class="text-center py-12" style="color: var(--color-text-muted)">无已安装模块</div>
        {:else}
          <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr style="color: var(--color-text-secondary); border-bottom: 1px solid var(--color-border)">
                {#if showModuleCheckboxes}<th class="px-2 py-3 w-8"><input type="checkbox" onchange={selectAllModules} checked={selectedModules.size === filteredModules.length && filteredModules.length > 0} /></th>{/if}
                <th class="text-left px-4 py-3">名称</th>
                <th class="text-left px-4 py-3">版本</th>
                <th class="text-left px-4 py-3">作者</th>
                <th class="text-left px-4 py-3">大小</th>
                <th class="text-center px-4 py-3">状态</th>
                <th class="text-right px-4 py-3">操作</th>
              </tr>
            </thead>
            <tbody>
              {#each filteredModules as mod (mod.name)}
                <tr style="border-bottom: 1px solid var(--color-border)">
                  {#if showModuleCheckboxes}
                    <td class="px-2 py-3 text-center">
                      <input type="checkbox" checked={selectedModules.has(mod.name)} onchange={() => toggleModuleSelect(mod.name)} />
                    </td>
                  {/if}
                  <td class="px-4 py-3">
                    <button class="font-medium text-left" style="color: var(--color-text); text-decoration: underline; text-underline-offset: 2px;" onclick={() => selectedModuleDetail = mod}>
                      {mod.name}
                    </button>
                    {#if mod.description}<div class="text-xs" style="color: var(--color-text-muted)">{mod.description}</div>{/if}
                  </td>
                  <td class="px-4 py-3" style="color: var(--color-text-secondary)">{mod.version}</td>
                  <td class="px-4 py-3" style="color: var(--color-text-secondary)">{mod.author}</td>
                  <td class="px-4 py-3" style="color: var(--color-text-secondary)">{mod.size}</td>
                  <td class="px-4 py-3 text-center">
                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs"
                          style={mod.enabled ? 'background: var(--color-success-light); color: var(--color-success)' : 'background: var(--color-surface); color: var(--color-text-muted)'}>
                      {mod.enabled ? '启用' : '禁用'}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-right">
                    <div class="flex items-center justify-end gap-1.5 flex-wrap">
                      <button class="text-xs whitespace-nowrap" style="color: var(--color-primary)" onclick={() => toggleModule(mod.name)}>
                        {mod.enabled ? '禁用' : '启用'}
                      </button>
                      <button class="text-xs whitespace-nowrap" style="color: var(--color-primary)" onclick={() => checkModuleUpdate(mod.name)}>
                        {updatingCheck.has(mod.name) ? '检查中...' : '检查更新'}
                      </button>
                      <button class="text-xs whitespace-nowrap" style="color: var(--color-primary)" onclick={() => exportModule(mod.name)}>导出</button>
                      <button class="text-xs whitespace-nowrap" style="color: var(--color-primary)" onclick={() => backupModule(mod.name)}>备份</button>
                      <button class="text-xs whitespace-nowrap" style="color: var(--color-error)" onclick={() => showConfirm('卸载模块', `确定要卸载模块 ${mod.name} 吗？`, 'danger', () => uninstallModule(mod.name))}>卸载</button>
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
          </div>
        {/if}
      </div>

      <ModuleDetail
        mod={selectedModuleDetail}
        onClose={() => selectedModuleDetail = null}
        onExport={(name: string) => { exportModule(name); selectedModuleDetail = null; }}
        onBackup={(name: string) => { backupModule(name); selectedModuleDetail = null; }}
      />

    {:else if activeTab === 'apps'}
      <div class="info-card overflow-hidden">
        <div class="p-4 border-b flex items-center justify-between" style="border-color: var(--color-border)">
          <div class="flex items-center gap-3">
            <span class="text-sm font-semibold" style="color: var(--color-text)">应用列表 ({apps.length})</span>
            <select class="input-field text-xs py-1" bind:value={appFilter} onchange={loadApps}>
              <option value="all">全部</option>
              <option value="thirdparty">第三方</option>
              <option value="system">系统</option>
            </select>
          </div>
          <button class="btn-ghost text-xs" onclick={loadApps}>刷新</button>
        </div>
        <div class="max-h-[500px] overflow-y-auto">
          {#each apps as app (app.package_name)}
            <div class="flex items-center justify-between px-4 py-2.5 text-sm" style="border-bottom: 1px solid var(--color-border)">
              <div class="flex-1 min-w-0">
                <div class="font-medium truncate" style="color: var(--color-text)">{app.package_name}</div>
                <div class="text-xs" style="color: var(--color-text-muted)">v{app.version_name} (SDK {app.target_sdk})</div>
              </div>
              <div class="flex items-center gap-1.5 ml-2">
                <button class="text-xs px-2 py-1 rounded" style="color: var(--color-primary)" onclick={() => launchApp(app.package_name)}>启动</button>
                <button class="text-xs px-2 py-1 rounded" style="color: var(--color-warning)" onclick={() => forceStopApp(app.package_name)}>停止</button>
                <button class="text-xs px-2 py-1 rounded" style="color: var(--color-error)" onclick={() => showConfirm('卸载应用', `确定要卸载 ${app.app_name || app.package_name} 吗？`, 'danger', () => uninstallApp(app.package_name))}>卸载</button>
              </div>
            </div>
          {/each}
        </div>
      </div>

    {:else if activeTab === 'files'}
      <div class="info-card overflow-hidden">
        <div class="p-4 border-b flex items-center justify-between" style="border-color: var(--color-border)">
          <div class="flex items-center gap-2 text-sm">
            <button class="btn-ghost text-xs px-2" onclick={goUp} disabled={currentPath === '/'}>⬆</button>
            <span style="color: var(--color-text-secondary)">路径:</span>
            <code class="text-xs px-2 py-1 rounded" style="background: var(--color-surface)">{currentPath}</code>
          </div>
          <button class="btn-ghost text-xs" onclick={() => loadFiles()}>刷新</button>
        </div>
        <!-- New Folder + Upload + Search Area -->
        <div class="p-3 border-b space-y-2" style="border-color: var(--color-border); background: var(--color-surface)">
          <div class="flex items-center gap-2">
            <input type="text" class="input-field text-xs flex-1" placeholder="搜索文件..." bind:value={fileSearchQuery}
              onkeydown={(e: KeyboardEvent) => { if (e.key === 'Escape') fileSearchQuery = ''; }} />
            {#if fileSearchQuery}
              <span class="text-xs" style="color: var(--color-text-muted)">{filteredFiles.length}/{files.length}</span>
            {/if}
          </div>
          <div class="flex items-center gap-2">
            <input type="text" class="input-field text-xs flex-1" placeholder="新建文件夹名" bind:value={newFolderName}
              onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') mkdir(); }} />
            <button class="btn-ghost text-xs" onclick={mkdir}>新建文件夹</button>
          </div>
          <div class="flex items-center gap-2">
            <input type="file" bind:this={fileInput} class="text-xs" style="color: var(--color-text)" />
            <input type="text" class="input-field text-xs flex-1" placeholder="目标路径（留空使用当前目录）" bind:value={uploadTarget} />
            <button class="btn-primary text-xs" disabled={uploading} onclick={uploadFile}>
              {uploading ? '上传中...' : '上传'}
            </button>
          </div>
        </div>
        <!-- Drag & Drop Zone -->
        <div
          class="relative"
          role="presentation"
          ondragover={handleDragOver}
          ondragleave={handleDragLeave}
          ondrop={handleDrop}
        >
          {#if dragOver}
            <div class="absolute inset-0 z-10 flex items-center justify-center rounded-lg" style="background: color-mix(in srgb, var(--color-primary) 10%, transparent); border: 2px dashed var(--color-primary)">
              <span class="text-sm font-medium" style="color: var(--color-primary)">释放文件以上传</span>
            </div>
          {/if}
        <div class="max-h-[500px] overflow-y-auto">
          {#each filteredFiles as file (file.path)}
            <div class="flex items-center px-4 py-2 text-sm" style="border-bottom: 1px solid var(--color-border)">
              <span class="material-symbols-outlined text-[18px] mr-2" style="color: {file.is_dir ? 'var(--color-primary)' : 'var(--color-text-muted)'}">
                {file.is_dir ? 'folder' : 'description'}
              </span>
              {#if renamePath === file.path}
                <input type="text" class="input-field text-xs flex-1" bind:value={renameTarget}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') renameFile(file.path); if (e.key === 'Escape') { renamePath = ''; renameTarget = ''; } }}
                  onblur={() => { if (renameTarget !== file.name) renameFile(file.path); else { renamePath = ''; renameTarget = ''; } }} />
              {:else}
                <button
                  class="flex-1 text-left truncate"
                  style="color: {file.is_dir ? 'var(--color-primary)' : 'var(--color-text)'}"
                  onclick={() => file.is_dir ? navigateTo(file.path + '/') : null}
                >
                  {file.name}
                </button>
              {/if}
              <span class="text-xs ml-4" style="color: var(--color-text-muted)">{file.is_dir ? '' : formatSize(file.size)}</span>
              <span class="text-xs ml-4 hidden sm:inline" style="color: var(--color-text-muted)">{file.mode}</span>
              <button class="text-xs ml-2" style="color: var(--color-text-secondary)" onclick={() => { renamePath = file.path; renameTarget = file.name; }}>重命名</button>
              {#if !file.is_dir}
                <button class="text-xs ml-2" style="color: var(--color-primary)" onclick={() => downloadFile(file.path)}>下载</button>
                <button class="text-xs ml-2" style="color: var(--color-text-secondary)" onclick={() => previewFile(file.path)}>预览</button>
              {/if}
              <button class="text-xs ml-2" style="color: var(--color-error)" onclick={() => showConfirm('删除文件', `确定要删除 ${file.name} 吗？此操作不可撤销。`, 'danger', () => deleteFile(file.path))}>删除</button>
            </div>
          {/each}
          {#if files.length === 0}
            <div class="text-center py-12" style="color: var(--color-text-muted)">空目录</div>
          {/if}
        </div>
        </div>
        <!-- Preview Modal -->
        {#if previewPath}
          <div class="border-t" style="border-color: var(--color-border)">
            <div class="flex items-center justify-between px-4 py-2 bg-surface" style="border-bottom: 1px solid var(--color-border)">
              <span class="text-xs font-semibold" style="color: var(--color-text)">预览: {previewPath}</span>
              <button class="text-xs" style="color: var(--color-text-secondary)" onclick={closePreview}>关闭</button>
            </div>
            <pre class="p-4 text-xs overflow-auto max-h-[300px] font-mono" style="background: var(--color-surface); color: var(--color-text); white-space: pre-wrap; word-break: break-all">{previewContent}</pre>
          </div>
        {/if}
      </div>

    {:else if activeTab === 'logs'}
      <div class="info-card overflow-hidden">
        <div class="p-4 border-b flex items-center gap-3" style="border-color: var(--color-border)">
          <input type="text" class="input-field text-xs flex-1" placeholder="Tag 过滤" bind:value={logFilter} />
          <select class="input-field text-xs py-1" bind:value={logLevel}>
            <option value="">全部</option>
            <option value="v">Verbose</option>
            <option value="d">Debug</option>
            <option value="i">Info</option>
            <option value="w">Warn</option>
            <option value="e">Error</option>
          </select>
          <button class="btn-primary text-xs" onclick={loadLogs}>查询</button>
          <button class="btn-ghost text-xs" onclick={clearLogs}>清空</button>
        </div>
        <pre class="p-4 text-xs overflow-auto max-h-[500px] font-mono" style="background: var(--color-surface); color: var(--color-text); white-space: pre-wrap; word-break: break-all">{logs || '无日志'}</pre>
      </div>

    {:else if activeTab === 'screen'}
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

        <div class="flex gap-4">
          <div class="flex-shrink-0">
            <ScreenCanvas
              serial={selectedDevice}
              fitWidth={screenFitWidth}
              onKey={(key) => sendKey(key)}
              onInput={(text) => apiPost(`/api/v1/adb/input/text`, { serial: selectedDevice, text })}
            />
          </div>

          <div class="flex-1 space-y-4">
            <ScreenControls
              {screenWidth} {screenHeight} {screenRefreshing} {screenAutoRefresh}
              {inputText} {holdDuration} {holdActive} {recording}
              onRefresh={refreshScreen}
              onToggleAutoRefresh={toggleScreenAutoRefresh}
              onToggleRecording={toggleScreenRecord}
              onSendKey={(key) => sendKey(key)}
              onSendKeyCombo={(keys) => sendKeyCombo(keys)}
              onSendInputText={sendInputText}
              onSwipeScreen={(x1, y1, x2, y2, dur) => swipeScreen(x1, y1, x2, y2, dur)}
              onRotateScreen={(dir) => rotateScreen(dir)}
              onInputChange={(v) => inputText = v}
              onHoldDurationChange={(v) => holdDuration = v}
            />
          </div>
        </div>
      </div>

    {:else if activeTab === 'shell'}
      <div class="info-card overflow-hidden">
        <div class="p-4 border-b" style="border-color: var(--color-border)">
          <div class="flex gap-2">
            <input type="text" class="input-field flex-1 font-mono text-sm" placeholder="输入 Shell 命令..." bind:value={shellCmd}
              onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') runShell(); }} />
            <button class="btn-primary text-sm" onclick={runShell}>执行</button>
          </div>
        </div>
        <pre class="p-4 text-xs overflow-auto max-h-[500px] font-mono" style="background: var(--color-surface); color: var(--color-text); white-space: pre-wrap; word-break: break-all">{shellOutput || '$ '}</pre>
      </div>
    {/if}
  {:else}
    <div class="text-center py-16" style="color: var(--color-text-muted)">
      <span class="material-symbols-outlined text-5xl mb-3 block">phone_android</span>
      <p>请连接 ADB 设备或输入无线调试地址</p>
    </div>
  {/if}
</div>

<style>
  .info-card {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
  }
  .input-field {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 6px 12px;
    color: var(--color-text);
    font-size: 14px;
    outline: none;
  }
  .input-field:focus { border-color: var(--color-primary); }
  .btn-primary {
    background: var(--color-primary);
    color: white;
    border: none;
    border-radius: var(--radius-md);
    padding: 6px 16px;
    cursor: pointer;
  }
  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-ghost {
    background: transparent;
    color: var(--color-text-secondary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 6px 12px;
    cursor: pointer;
  }
  .btn-ghost:hover { background: var(--color-surface); }
  .btn-ghost:disabled { opacity: 0.5; cursor: not-allowed; }
</style>

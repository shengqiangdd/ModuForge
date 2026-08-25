<script lang="ts">
  import { onMount } from 'svelte';
  import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
  import DeviceList from './components/DeviceList.svelte';
  import DeviceDetail from './components/DeviceDetail.svelte';
  import DeviceTabs from './components/DeviceTabs.svelte';
  import RootManager from './components/RootManager.svelte';
  import ModuleManager from './components/ModuleManager.svelte';
  import AppManager from './components/AppManager.svelte';
  import FileManager from './components/FileManager.svelte';
  import LogViewer from './components/LogViewer.svelte';
  import ShellTerminal from './components/ShellTerminal.svelte';
  import ScreenTab from './components/ScreenTab.svelte';
  import { apiGet, apiPost } from './device-api';
  import type { Device, SavedDevice, DeviceInfo } from './device-types';

  // ─── Confirm Dialog ───
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

  // ─── State ───
  let devices = $state<Device[]>([]);
  let savedDevices = $state<SavedDevice[]>([]);
  let selectedDevice = $state('');
  let deviceInfo = $state<DeviceInfo | null>(null);
  let loading = $state(false);
  let errorMsg = $state('');
  let successMsg = $state('');
  let connectAddress = $state('');
  let activeTab = $state<'info' | 'modules' | 'apps' | 'files' | 'logs' | 'shell' | 'screen'>('info');
  let autoRefresh = $state(false);
  let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;
  let installing = $state(false);
  let installPath = $state('');

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
    try {
      const res = await fetch(`/api/v1/adb/saved-devices/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || ''}` },
      });
      if (!res.ok) {
        let data: any = {};
        try { data = await res.json(); } catch { /* ignore */ }
        throw new Error(data.error || data.message || `删除失败 (${res.status})`);
      }
      msg('', '设备已删除');
      savedDevices = savedDevices.filter(sd => sd.id !== id);
      devices = devices.filter(d => d.id !== id);
      loadSavedDevices();
      listDevices();
    } catch (e: any) {
      msg(e?.message || '删除失败');
    }
  }

  function onDeleteDevice(device: Device) {
    if (device.id === undefined || device.id === null) return;
    onDeleteSavedId(device.id, device.serial);
  }

  function onDeleteSavedId(id: number, serial: string) {
    showConfirm(
      '删除设备',
      `确定要从已保存设备中删除 ${serial} 吗？`,
      'danger',
      () => deleteSavedDevice(id)
    );
  }

  async function connectDevice() {
    if (!connectAddress.trim()) return;
    const d = await apiPost('/api/v1/adb/connect', { address: connectAddress.trim() });
    if (d.error) { msg(d.error); return; }
    if (d.status === 'connected') {
      successMsg = d.message || '连接成功 ✅';
      selectedDevice = d.serial || connectAddress.trim();
      loadDeviceInfo();
    } else {
      const suggestions = (d.suggestions || []).join('\n');
      msg((d.message || '连接失败') + (suggestions ? '\n' + suggestions : ''));
    }
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

  async function installModule() {
    if (!installPath.trim()) return;
    installing = true;
    const d = await apiPost('/api/v1/adb/install', { serial: selectedDevice, zip_path: installPath.trim() });
    if (d.error) { msg(d.error); } else { successMsg = 'Installed'; installPath = ''; }
    installing = false;
  }

  function switchTab(tab: typeof activeTab) {
    activeTab = tab;
    if (tab === 'info') loadDeviceInfo();
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

  // ─── Init ───
  onMount(() => {
    listDevices();
    loadSavedDevices();
    return () => {
      if (autoRefreshTimer) clearInterval(autoRefreshTimer);
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

<div class="w-full p-4 md:p-6 max-w-7xl mx-auto min-w-0">
  <!-- Header -->
  <div class="flex flex-wrap items-center justify-between gap-3 mb-6">
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

  <DeviceList
    {devices}
    {selectedDevice}
    {loading}
    onSelect={(serial) => { selectedDevice = serial; loadDeviceInfo(); switchTab(activeTab); }}
    onConnect={(addr) => { connectAddress = addr; connectDevice(); }}
    onDelete={(device) => onDeleteDevice(device)}
    onRefresh={() => { listDevices(); if (selectedDevice) loadDeviceInfo(); }}
  />

  {#if savedDevices.length > 0}
    <div class="mt-4 mb-2">
      <div class="text-xs font-medium mb-2" style="color: var(--color-text-secondary)">已保存设备</div>
      <div class="flex flex-wrap gap-2">
        {#each savedDevices as sd}
          <button
            class="saved-chip"
            onclick={() => { connectAddress = sd.address; }}
            title={`最近连接: ${sd.last_connected_at || '未知'}`}
          >
            <span class="saved-chip-addr">{sd.address}</span>
            {#if sd.name}
              <span class="saved-chip-name">{sd.name}</span>
            {/if}
            <span
              class="saved-chip-del"
              role="button"
              tabindex="0"
              onclick={(e) => { e.stopPropagation(); onDeleteSavedId(sd.id, sd.address); }}
              onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); e.stopPropagation(); onDeleteSavedId(sd.id, sd.address); } }}
            >×</span>
          </button>
        {/each}
      </div>
    </div>
  {/if}

  {#if selectedDevice}
    <DeviceTabs {activeTab} onTabChange={(tab) => switchTab(tab as typeof activeTab)} />

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

      <DeviceDetail {deviceInfo} {loading} />

      <RootManager serial={selectedDevice} onMsg={msg} />

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
      <ModuleManager serial={selectedDevice} onMsg={msg} onConfirm={showConfirm} />

    {:else if activeTab === 'apps'}
      <AppManager serial={selectedDevice} onMsg={msg} onConfirm={showConfirm} />

    {:else if activeTab === 'files'}
      <FileManager serial={selectedDevice} onMsg={msg} onConfirm={showConfirm} />

    {:else if activeTab === 'logs'}
      <LogViewer serial={selectedDevice} />

    {:else if activeTab === 'screen'}
      <ScreenTab serial={selectedDevice} onMsg={msg} />

    {:else if activeTab === 'shell'}
      <ShellTerminal serial={selectedDevice} onMsg={msg} />
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
  .saved-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: 9999px;
    font-size: 12px;
    color: var(--color-text-secondary);
    cursor: pointer;
    max-width: 100%;
  }
  .saved-chip:hover { border-color: var(--color-primary); }
  .saved-chip-addr {
    font-family: monospace;
    color: var(--color-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .saved-chip-name {
    color: var(--color-text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .saved-chip-del {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    line-height: 1;
    border-radius: 50%;
    color: var(--color-text-secondary);
    flex-shrink: 0;
    font-size: 14px;
  }
  .saved-chip-del:hover {
    background: var(--color-error-light);
    color: var(--color-error);
  }
</style>

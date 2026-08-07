<script lang="ts">
  import ListTransition from '$lib/components/ui/ListTransition.svelte';
  import Skeleton from '$lib/components/ui/Skeleton.svelte';

  interface Device {
    serial: string;
    model: string;
    brand: string;
    state: string;
    android_version: string;
  }

  let {
    devices = [],
    selectedDevice = '',
    selectedDevices = new Set(),
    loading = false,
    onSelect,
    onSelectBatch,
    onConnect,
    onRefresh
  }: {
    devices?: Device[];
    selectedDevice?: string;
    selectedDevices?: Set<string>;
    loading?: boolean;
    onSelect?: (serial: string) => void;
    onSelectBatch?: (devices: Set<string>) => void;
    onConnect?: (address: string) => void;
    onRefresh?: () => void;
  } = $props();

  let connectAddress = $state('');
  let selectAll = $state(false);

  function toggleSelectAll() {
    if (selectAll) {
      onSelectBatch?.(new Set());
    } else {
      onSelectBatch?.(new Set(devices.map(d => d.serial)));
    }
    selectAll = !selectAll;
  }

  function toggleDevice(serial: string) {
    const newSet = new Set(selectedDevices);
    if (newSet.has(serial)) {
      newSet.delete(serial);
    } else {
      newSet.add(serial);
    }
    onSelectBatch?.(newSet);
  }
</script>

<div class="device-list">
  <div class="list-header">
    <h3>设备列表</h3>
    <div class="header-actions">
      {#if devices.length > 0}
        <label class="select-all">
          <input type="checkbox" checked={selectAll} onchange={toggleSelectAll} />
          全选
        </label>
      {/if}
      <button class="btn-icon" onclick={onRefresh} title="刷新">
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M23 4v6h-6M1 20v-6h6"/>
          <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
        </svg>
      </button>
    </div>
  </div>

  <!-- Connect Form -->
  <div class="connect-form">
    <input
      type="text"
      bind:value={connectAddress}
      placeholder="IP:端口 或 serial"
      class="connect-input"
    />
    <button class="btn-primary btn-sm" onclick={() => onConnect?.(connectAddress)} disabled={!connectAddress}>
      连接
    </button>
  </div>

  {#if loading}
    <Skeleton count={4} lines={[100, 80, 90, 70]} />
  {:else if devices.length === 0}
    <div class="empty-state">
      <p>未检测到设备</p>
      <p class="hint">请连接设备或检查 ADB 服务</p>
    </div>
  {:else}
    <ListTransition items={devices} key="serial">
      {#snippet children(device: Device)}
        <div
          class="device-item"
          class:selected={selectedDevice === device.serial}
          class:batch-selected={selectedDevices.has(device.serial)}
        >
          <input
            type="checkbox"
            checked={selectedDevices.has(device.serial)}
            onchange={() => toggleDevice(device.serial)}
            class="device-checkbox"
          />
          <button class="device-info" onclick={() => onSelect?.(device.serial)}>
            <span class="device-model">{device.brand} {device.model}</span>
            <span class="device-serial">{device.serial}</span>
            <span class="device-android">Android {device.android_version}</span>
          </button>
          <span class="device-state" class:online={device.state === 'device'}>
            {device.state === 'device' ? '在线' : '离线'}
          </span>
        </div>
      {/snippet}
    </ListTransition>
  {/if}
</div>

<style>
  .device-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .list-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .list-header h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .select-all {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    cursor: pointer;
  }

  .btn-icon {
    padding: 0.375rem;
    border: none;
    background: transparent;
    border-radius: 0.375rem;
    cursor: pointer;
    color: var(--color-text-secondary);
  }

  .btn-icon:hover {
    background: var(--color-bg-hover);
  }

  .connect-form {
    display: flex;
    gap: 0.5rem;
  }

  .connect-input {
    flex: 1;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--color-border);
    border-radius: 0.375rem;
    font-size: 0.875rem;
  }

  .btn-primary {
    padding: 0.5rem 1rem;
    background: var(--color-primary);
    color: white;
    border: none;
    border-radius: 0.375rem;
    cursor: pointer;
  }

  .btn-primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-sm {
    padding: 0.375rem 0.75rem;
    font-size: 0.875rem;
  }

  .empty-state {
    text-align: center;
    padding: 2rem;
    color: var(--color-text-secondary);
  }

  .empty-state p {
    margin: 0;
  }

  .hint {
    font-size: 0.875rem;
    margin-top: 0.5rem !important;
  }

  .device-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem;
    background: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: 0.5rem;
    transition: border-color 0.2s;
  }

  .device-item:hover {
    border-color: var(--color-primary);
  }

  .device-item.selected {
    border-color: var(--color-primary);
    background: var(--color-primary-light);
  }

  .device-item.batch-selected {
    border-color: var(--color-primary);
  }

  .device-checkbox {
    cursor: pointer;
  }

  .device-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
    background: none;
    border: none;
    text-align: left;
    cursor: pointer;
    padding: 0;
  }

  .device-model {
    font-weight: 500;
    font-size: 0.875rem;
  }

  .device-serial {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    font-family: monospace;
  }

  .device-android {
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .device-state {
    font-size: 0.75rem;
    padding: 0.125rem 0.5rem;
    border-radius: 9999px;
    background: var(--color-bg);
    color: var(--color-text-secondary);
  }

  .device-state.online {
    background: var(--color-success-light);
    color: var(--color-success);
  }
</style>

<script lang="ts">
  import Skeleton from '$lib/components/ui/Skeleton.svelte';

  interface DeviceInfo {
    serial: string;
    model: string;
    brand: string;
    manufacturer: string;
    android_version: string;
    sdk_version: string;
    build_id: string;
    security_patch: string;
    magisk_version: string;
    ksu_version: string;
    apatch_version: string;
    battery_level: number;
    battery_status: string;
    storage_total: string;
    storage_used: string;
    storage_free: string;
    ram_total: string;
    ram_free: string;
    ram_used: string;
    uptime: string;
    kernel: string;
    abi: string;
  }

  let { deviceInfo = null, loading = false }: { deviceInfo?: DeviceInfo | null; loading?: boolean } = $props();
</script>

<div class="device-detail">
  {#if loading}
    <Skeleton count={8} lines={[60, 80, 50, 70, 60, 90, 40, 70]} />
  {:else if !deviceInfo}
    <div class="empty-state">
      <p>选择设备查看详细信息</p>
    </div>
  {:else}
    <div class="info-grid">
      <div class="info-item">
        <span class="info-label">品牌</span>
        <span class="info-value">{deviceInfo.brand}</span>
      </div>
      <div class="info-item">
        <span class="info-label">型号</span>
        <span class="info-value">{deviceInfo.model}</span>
      </div>
      <div class="info-item">
        <span class="info-label">制造商</span>
        <span class="info-value">{deviceInfo.manufacturer}</span>
      </div>
      <div class="info-item">
        <span class="info-label">Android</span>
        <span class="info-value">{deviceInfo.android_version} (SDK {deviceInfo.sdk_version})</span>
      </div>
      <div class="info-item">
        <span class="info-label">Build ID</span>
        <span class="info-value mono">{deviceInfo.build_id}</span>
      </div>
      <div class="info-item">
        <span class="info-label">安全补丁</span>
        <span class="info-value">{deviceInfo.security_patch}</span>
      </div>
      <div class="info-item">
        <span class="info-label">内核</span>
        <span class="info-value mono">{deviceInfo.kernel}</span>
      </div>
      <div class="info-item">
        <span class="info-label">ABI</span>
        <span class="info-value">{deviceInfo.abi}</span>
      </div>
    </div>

    <!-- Root Manager -->
    <div class="section">
      <h4>Root 管理器</h4>
      <div class="root-info">
        {#if deviceInfo.ksu_version}
          <span class="root-badge ksu">KernelSU {deviceInfo.ksu_version}</span>
        {/if}
        {#if deviceInfo.apatch_version}
          <span class="root-badge apatch">APatch {deviceInfo.apatch_version}</span>
        {/if}
        {#if deviceInfo.magisk_version}
          <span class="root-badge magisk">Magisk {deviceInfo.magisk_version}</span>
        {/if}
        {#if !deviceInfo.ksu_version && !deviceInfo.apatch_version && !deviceInfo.magisk_version}
          <span class="root-badge none">未检测到 Root</span>
        {/if}
      </div>
    </div>

    <!-- Battery -->
    <div class="section">
      <h4>电池</h4>
      <div class="battery-bar">
        <div class="battery-fill" style="width: {deviceInfo.battery_level}%"></div>
      </div>
      <span class="battery-text">{deviceInfo.battery_level}% ({deviceInfo.battery_status})</span>
    </div>

    <!-- Storage -->
    <div class="section">
      <h4>存储</h4>
      <div class="storage-bar">
        <div class="storage-fill" style="width: {deviceInfo.storage_total ? '60' : '0'}%"></div>
      </div>
      <span class="storage-text">{deviceInfo.storage_used} / {deviceInfo.storage_total}</span>
    </div>

    <!-- RAM -->
    <div class="section">
      <h4>内存</h4>
      <div class="ram-bar">
        <div class="ram-fill" style="width: {deviceInfo.ram_total ? '40' : '0'}%"></div>
      </div>
      <span class="ram-text">{deviceInfo.ram_used} / {deviceInfo.ram_total}</span>
    </div>

    <!-- Uptime -->
    <div class="section">
      <h4>运行时间</h4>
      <span class="uptime-text">{deviceInfo.uptime}</span>
    </div>
  {/if}
</div>

<style>
  .device-detail {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .empty-state {
    text-align: center;
    padding: 2rem;
    color: var(--color-text-secondary);
  }

  .info-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.75rem;
  }

  @media (max-width: 640px) {
    .info-grid {
      grid-template-columns: 1fr;
    }
  }

  .info-item {
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
  }

  .info-label {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
  }

  .info-value {
    font-size: 0.875rem;
    font-weight: 500;
  }

  .info-value.mono {
    font-family: monospace;
    font-size: 0.75rem;
  }

  .section {
    padding: 0.75rem;
    background: var(--color-bg-secondary);
    border-radius: 0.5rem;
    border: 1px solid var(--color-border);
  }

  .section h4 {
    margin: 0 0 0.5rem;
    font-size: 0.875rem;
    font-weight: 600;
  }

  .root-info {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .root-badge {
    padding: 0.25rem 0.5rem;
    font-size: 0.75rem;
    border-radius: 0.25rem;
    font-weight: 500;
  }

  .root-badge.ksu { background: #3b82f620; color: #3b82f6; }
  .root-badge.apatch { background: #8b5cf620; color: #8b5cf6; }
  .root-badge.magisk { background: #f59e0b20; color: #f59e0b; }
  .root-badge.none { background: var(--color-bg); color: var(--color-text-secondary); }

  .battery-bar, .storage-bar, .ram-bar {
    height: 8px;
    background: var(--color-bg);
    border-radius: 4px;
    overflow: hidden;
    margin-bottom: 0.25rem;
  }

  .battery-fill { height: 100%; background: var(--color-success); border-radius: 4px; }
  .storage-fill { height: 100%; background: var(--color-primary); border-radius: 4px; }
  .ram-fill { height: 100%; background: var(--color-warning); border-radius: 4px; }

  .battery-text, .storage-text, .ram-text, .uptime-text {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
  }
</style>

<script lang="ts">
  import { apiGet, apiPost } from '../device-api';
  import type { AppInfo } from '../device-types';

  let {
    serial,
    onMsg,
    onConfirm
  }: {
    serial: string;
    onMsg: (err: string, ok?: string) => void;
    onConfirm: (title: string, message: string, variant: 'primary' | 'danger', cb: () => void) => void;
  } = $props();

  let apps = $state<AppInfo[]>([]);
  let appFilter = $state('all');

  async function loadApps() {
    if (!serial) return;
    const filter = appFilter !== 'all' ? `&filter=${appFilter}` : '';
    const d = await apiGet(`/api/v1/adb/apps?serial=${serial}${filter}`);
    apps = d.apps || [];
  }

  async function uninstallApp(pkg: string) {
    await apiPost('/api/v1/adb/app/uninstall', { serial, package: pkg });
    loadApps();
  }

  async function forceStopApp(pkg: string) {
    await apiPost('/api/v1/adb/app/force-stop', { serial, package: pkg });
  }

  async function launchApp(pkg: string) {
    const d = await apiPost('/api/v1/adb/app/launch', { serial, package: pkg });
    if (d.error) onMsg(d.error);
  }

  $effect(() => {
    if (serial) loadApps();
  });
</script>

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
          <button class="text-xs px-2 py-1 rounded" style="color: var(--color-error)" onclick={() => onConfirm('卸载应用', `确定要卸载 ${app.app_name || app.package_name} 吗？`, 'danger', () => uninstallApp(app.package_name))}>卸载</button>
        </div>
      </div>
    {/each}
  </div>
</div>

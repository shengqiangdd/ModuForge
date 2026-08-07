<script lang="ts">
  let { serial, onMsg }: { serial: string; onMsg: (err: string, ok?: string) => void } = $props();

  let rootManagers = $state<any[]>([]);
  let rootPermissions = $state<any[]>([]);
  let rootModuleList = $state<any[]>([]);
  let showRootPermissions = $state(false);
  let showRootModules = $state(false);
  let rootPermPackage = $state('');
  let rootPermGrant = $state(true);

  async function loadRootManagers() {
    if (!serial) return;
    const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
    const res = await fetch(`/api/v1/adb/root/managers?serial=${serial}`, { headers: { 'Authorization': `Bearer ${token}` } });
    const d = await res.json();
    rootManagers = d.managers || [];
  }

  async function loadRootPermissions() {
    if (!serial) return;
    const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
    const res = await fetch(`/api/v1/adb/root/permissions?serial=${serial}`, { headers: { 'Authorization': `Bearer ${token}` } });
    const d = await res.json();
    rootPermissions = d.permissions || [];
    showRootPermissions = true;
  }

  async function manageRootPermission() {
    if (!rootPermPackage.trim()) { onMsg('请输入包名'); return; }
    const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
    const res = await fetch('/api/v1/adb/root/permission', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ serial, package_name: rootPermPackage.trim(), grant: rootPermGrant }),
    });
    const d = await res.json();
    if (d.error) { onMsg(d.error); } else { onMsg('', d.output || '操作成功'); rootPermPackage = ''; loadRootPermissions(); }
  }

  async function loadRootModules() {
    if (!serial) return;
    const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
    const res = await fetch(`/api/v1/adb/root/modules?serial=${serial}`, { headers: { 'Authorization': `Bearer ${token}` } });
    const d = await res.json();
    rootModuleList = d.modules || [];
    showRootModules = true;
  }
</script>

<div class="info-card p-5 min-w-0">
  <h3 class="text-sm font-semibold mb-3" style="color: var(--color-text)">Root 管理器</h3>
  <div class="space-y-2 text-sm">
    {#if rootManagers.length === 0}
      <div class="text-xs" style="color: var(--color-text-muted)">未检测到 Root 管理器，点击扫描</div>
    {:else}
      {#each rootManagers as rm}
        <div class="info-row">
          <span style="color: var(--color-text-secondary)">{rm.name}</span>
          <span>{rm.version || '未知'} <span class="text-xs" style="color: var(--color-text-muted)">({rm.path})</span></span>
        </div>
      {/each}
    {/if}
  </div>
  <div class="mt-4 flex flex-wrap gap-2">
    <button class="btn-ghost text-xs" onclick={loadRootManagers}>扫描</button>
    <button class="btn-ghost text-xs" onclick={loadRootPermissions}>权限列表</button>
    <button class="btn-ghost text-xs" onclick={loadRootModules}>模块列表</button>
  </div>
  {#if showRootPermissions}
    <div class="mt-4 border-t pt-3" style="border-color: var(--color-border)">
      <h4 class="text-xs font-semibold mb-2" style="color: var(--color-text)">Root 权限管理</h4>
      <div class="flex gap-2 mb-2">
        <input type="text" class="input-field text-xs flex-1" placeholder="包名" bind:value={rootPermPackage} />
        <label class="flex items-center gap-1 text-xs" style="color: var(--color-text-secondary)">
          <input type="checkbox" bind:checked={rootPermGrant} /> 授予
        </label>
        <button class="btn-primary text-xs" onclick={manageRootPermission}>执行</button>
      </div>
      <div class="max-h-32 overflow-y-auto space-y-1">
        {#each rootPermissions as perm}
          <div class="flex justify-between text-xs px-2 py-1 rounded" style="background: var(--color-surface)">
            <span style="color: var(--color-text)">{perm.package}</span>
            <span style="color: var(--color-success)">{perm.status}</span>
          </div>
        {/each}
        {#if rootPermissions.length === 0}
          <div class="text-xs" style="color: var(--color-text-muted)">无已授权的 Root 权限</div>
        {/if}
      </div>
    </div>
  {/if}
  {#if showRootModules}
    <div class="mt-4 border-t pt-3" style="border-color: var(--color-border)">
      <h4 class="text-xs font-semibold mb-2" style="color: var(--color-text)">Root 模块</h4>
      <div class="max-h-32 overflow-y-auto space-y-1">
        {#each rootModuleList as rm}
          <div class="flex justify-between text-xs px-2 py-1 rounded" style="background: var(--color-surface)">
            <span style="color: var(--color-text)">{rm.name}</span>
            <span class="flex items-center gap-1">
              <span style="color: var(--color-text-muted)">{rm.version}</span>
              <span class="px-1 py-0.5 rounded text-[10px]" style={rm.enabled ? 'background: var(--color-success-light); color: var(--color-success)' : 'background: var(--color-surface); color: var(--color-text-muted)'}>
                {rm.enabled ? '启用' : '禁用'}
              </span>
            </span>
          </div>
        {/each}
        {#if rootModuleList.length === 0}
          <div class="text-xs" style="color: var(--color-text-muted)">无模块</div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .info-row {
    display: flex;
    justify-content: space-between;
    padding: 4px 0;
    border-bottom: 1px solid var(--color-border);
  }
  .info-row:last-child { border-bottom: none; }
</style>
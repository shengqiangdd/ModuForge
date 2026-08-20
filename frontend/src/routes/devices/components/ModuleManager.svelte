<script lang="ts">
  import { apiGet, apiPost } from '../device-api';
  import type { InstalledModule } from '../device-types';
  import ModuleDetail from './ModuleDetail.svelte';

  let {
    serial,
    onMsg,
    onConfirm
  }: {
    serial: string;
    onMsg: (err: string, ok?: string) => void;
    onConfirm: (title: string, message: string, variant: 'primary' | 'danger', cb: () => void) => void;
  } = $props();

  let modules = $state<InstalledModule[]>([]);
  let moduleSearchQuery = $state('');
  let filteredModules = $derived(!moduleSearchQuery ? modules : modules.filter(m =>
    m.name.toLowerCase().includes(moduleSearchQuery.toLowerCase()) ||
    (m.description || '').toLowerCase().includes(moduleSearchQuery.toLowerCase()) ||
    (m.author || '').toLowerCase().includes(moduleSearchQuery.toLowerCase())
  ));
  let selectedModules = $state<Set<string>>(new Set());
  let showModuleCheckboxes = $state(false);
  let showBackupRestore = $state(false);
  let backupModuleName = $state('');
  let restoreFileName = $state('');
  let backupRestoreMsg = $state('');
  let moduleUpdateInfo = $state<Record<string, any>>({});
  let updatingCheck = $state<Set<string>>(new Set());
  let selectedModuleDetail = $state<InstalledModule | null>(null);
  let installing = $state(false);
  let installPath = $state('');

  async function loadModules() {
    if (!serial) return;
    const d = await apiGet(`/api/v1/adb/modules?serial=${serial}`);
    modules = d.modules || [];
    moduleSearchQuery = '';
  }

  async function toggleModule(name: string) {
    const mod = modules.find(m => m.name === name);
    await apiPost(`/api/v1/adb/modules/${name}/toggle`, { serial, enable: !mod?.enabled });
    loadModules();
  }

  async function uninstallModule(name: string) {
    await apiPost(`/api/v1/adb/modules/${name}/uninstall`, { serial });
    loadModules();
  }

  async function installModule() {
    if (!installPath.trim()) return;
    installing = true;
    const d = await apiPost('/api/v1/adb/install', { serial, zip_path: installPath.trim() });
    if (d.error) { onMsg(d.error); } else { onMsg('', 'Installed'); installPath = ''; loadModules(); }
    installing = false;
  }

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
    if (selectedModules.size === 0) { onMsg('请先选择模块'); return; }
    for (const name of selectedModules) {
      await apiPost(`/api/v1/adb/modules/${name}/toggle`, { serial, enable });
    }
    selectedModules = new Set();
    loadModules();
  }

  async function batchUninstallModules() {
    if (selectedModules.size === 0) { onMsg('请先选择模块'); return; }
    onConfirm('批量卸载', `确定要卸载 ${selectedModules.size} 个模块吗？`, 'danger', async () => {
      for (const name of selectedModules) {
        await apiPost(`/api/v1/adb/modules/${name}/uninstall`, { serial });
      }
      selectedModules = new Set();
      loadModules();
    });
  }

  async function backupModule(name: string) {
    const token = localStorage.getItem('moduforge_token') || sessionStorage.getItem('moduforge_token') || '';
    const res = await fetch('/api/v1/adb/module/backup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ serial, module_name: name }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: '下载失败' }));
      onMsg(err.error || '下载失败');
      return;
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = name + '_backup.tar.gz';
    document.body.appendChild(a); a.click(); a.remove();
    URL.revokeObjectURL(url);
    onMsg('', '备份已下载');
  }

  async function restoreModule() {
    if (!restoreFileName.trim()) { onMsg('请输入备份文件路径'); return; }
    const d = await apiPost('/api/v1/adb/module/restore', { serial, local_path: restoreFileName.trim() });
    if (d.error) { onMsg(d.error); } else { onMsg('', d.output || '模块已恢复'); restoreFileName = ''; loadModules(); }
  }

  async function exportModule(name: string) {
    const d = await apiPost('/api/v1/adb/module/export', { serial, module_name: name });
    if (d.error) { onMsg(d.error); } else { onMsg('', `模块已导出至设备: ${d.export_path}`); }
  }

  async function checkModuleUpdate(name: string) {
    updatingCheck = new Set(updatingCheck).add(name);
    const d = await apiGet(`/api/v1/adb/module/check-update?serial=${serial}&module_name=${name}`);
    moduleUpdateInfo = { ...moduleUpdateInfo, [name]: d };
    updatingCheck = new Set([...updatingCheck].filter(x => x !== name));
    if (d.has_update) {
      onMsg('', `模块 ${name} 有新版本: ${d.latest_version}`);
    } else {
      onMsg('', `模块 ${name} 已是最新版本`);
    }
  }

  $effect(() => {
    if (serial) loadModules();
  });
</script>

<div class="info-card overflow-x-auto">
  <div class="p-4 border-b flex flex-wrap items-center justify-between gap-2" style="border-color: var(--color-border)">
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
      <div class="flex flex-wrap items-center gap-2 text-xs">
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
                <button class="text-xs whitespace-nowrap" style="color: var(--color-error)" onclick={() => onConfirm('卸载模块', `确定要卸载模块 ${mod.name} 吗？`, 'danger', () => uninstallModule(mod.name))}>卸载</button>
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

<script lang="ts">
  import type { InstalledModule } from '../device-types';

  let { mod, onClose, onExport, onBackup }: {
    mod: InstalledModule | null;
    onClose: () => void;
    onExport: (name: string) => void;
    onBackup: (name: string) => void;
  } = $props();

  function handleOverlayClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }
</script>

{#if mod}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" role="presentation" onclick={handleOverlayClick}>
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 w-full max-w-lg border border-[var(--color-border)] shadow-2xl" role="dialog" aria-modal="true" tabindex="-1">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-bold text-[var(--color-text)]">{mod.name}</h3>
        <button class="p-1 rounded-lg hover:bg-[var(--color-surface)]" onclick={onClose}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="space-y-3">
        <div class="flex justify-between"><span class="text-sm" style="color: var(--color-text-muted)">版本</span><span class="text-sm text-[var(--color-text)]">{mod.version}</span></div>
        <div class="flex justify-between"><span class="text-sm" style="color: var(--color-text-muted)">作者</span><span class="text-sm text-[var(--color-text)]">{mod.author || '未知'}</span></div>
        <div class="flex justify-between"><span class="text-sm" style="color: var(--color-text-muted)">来源</span><span class="text-sm text-[var(--color-text)]">{mod.source || '未知'}</span></div>
        <div class="flex justify-between"><span class="text-sm" style="color: var(--color-text-muted)">大小</span><span class="text-sm text-[var(--color-text)]">{mod.size}</span></div>
        <div class="flex justify-between"><span class="text-sm" style="color: var(--color-text-muted)">描述</span><span class="text-sm text-[var(--color-text)]">{mod.description || '无描述'}</span></div>
        <div class="flex justify-between"><span class="text-sm" style="color: var(--color-text-muted)">更新日期</span><span class="text-sm text-[var(--color-text)]">{mod.update_date || '未知'}</span></div>
        <div class="flex justify-between"><span class="text-sm" style="color: var(--color-text-muted)">有更新</span><span class="text-sm" style="color: {mod.has_update ? 'var(--color-success)' : 'var(--color-text-muted)'}">{mod.has_update ? '是' : '否'}</span></div>
        <div class="flex justify-between"><span class="text-sm" style="color: var(--color-text-muted)">状态</span><span class="text-sm" style="color: {mod.enabled ? 'var(--color-success)' : 'var(--color-text-muted)'}">{mod.enabled ? '已启用' : '已禁用'}</span></div>
      </div>
      <div class="flex justify-end gap-2 mt-6">
        <button class="btn-ghost text-sm" onclick={onClose}>关闭</button>
        <button class="btn-primary text-sm" onclick={() => { onExport(mod.name); onClose(); }}>导出</button>
        <button class="btn-primary text-sm" onclick={() => { onBackup(mod.name); onClose(); }}>备份</button>
      </div>
    </div>
  </div>
{/if}
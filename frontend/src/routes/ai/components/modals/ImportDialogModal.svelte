<script lang="ts">
import { focusTrap } from '$lib/utils/focusTrap';
import type { SecurityScanResult } from '../../lib/types';

let {
  show = false,
  importFiles = [],
  importProjects = [],
  selectedImportProject = '',
  scanning = false,
  scanResult = null,
  importing = false,
  onClose,
  onProjectChange,
  onScanAndImport,
}: {
  show: boolean;
  importFiles: { path: string; content: string }[];
  importProjects: { id: string; name: string }[];
  selectedImportProject: string;
  scanning: boolean;
  scanResult: SecurityScanResult | null;
  importing: boolean;
  onClose: () => void;
  onProjectChange: (v: string) => void;
  onScanAndImport: () => void;
} = $props();
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }} onkeydown={(e) => { if (e.key === 'Escape') onClose(); }}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-md border border-[var(--color-border)] max-h-[85vh] flex flex-col" role="dialog" aria-modal="true" tabindex="-1" use:focusTrap>
      <div class="px-6 py-4 border-b border-[var(--color-border)]">
        <h3 class="text-lg font-semibold text-[var(--color-text)]">导入到项目</h3>
      </div>
      <div class="px-6 py-4 space-y-4 overflow-y-auto flex-1">
        <p class="text-sm text-[var(--color-text-secondary)]">选择目标项目，导入 {importFiles.length} 个文件</p>
        <div>
          <label for="import-target-project" class="block text-sm font-medium mb-1.5 text-[var(--color-text-secondary)]">目标项目</label>
          <select id="import-target-project" class="w-full px-3 py-2 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)]" value={selectedImportProject} onchange={(e) => onProjectChange((e.target as HTMLSelectElement).value)}>
            {#each importProjects as p}
              <option value={p.id}>{p.name}</option>
            {/each}
          </select>
        </div>

        {#if scanning}
          <div class="flex items-center gap-2 py-3">
            <span class="material-symbols-outlined text-[18px] animate-spin text-primary-500">progress_activity</span>
            <span class="text-sm text-[var(--color-text-secondary)]">安全扫描中...</span>
          </div>
        {:else if scanResult}
          <div class="rounded-xl p-3 border" style="border-color: {scanResult.safe ? 'var(--color-success, #22c55e)' : 'var(--color-error, #ef4444)'}; background: color-mix(in srgb, {scanResult.safe ? '#22c55e' : '#ef4444'} 8%, var(--color-bg))">
            <div class="flex items-center gap-2 mb-1">
              <span class="material-symbols-outlined text-[18px]" style="color: {scanResult.safe ? '#22c55e' : '#ef4444'}">{scanResult.safe ? 'verified' : 'warning'}</span>
              <span class="text-sm font-medium" style="color: {scanResult.safe ? '#22c55e' : '#ef4444'}">安全评分：{scanResult.score}/100</span>
            </div>
            <p class="text-xs text-[var(--color-text-secondary)]">{scanResult.summary}</p>
            {#if scanResult.issues.length > 0}
              <div class="mt-2 space-y-1 max-h-32 overflow-y-auto">
                {#each scanResult.issues as issue}
                  <div class="flex items-start gap-1.5 text-xs px-2 py-1 rounded" style="background: color-mix(in srgb, var(--color-surface) 50%, transparent)">
                    <span class="material-symbols-outlined text-[12px] mt-0.5 flex-shrink-0" style="color: {issue.severity === 'critical' ? '#ef4444' : issue.severity === 'warning' ? '#f59e0b' : '#6b7280'}">
                      {issue.severity === 'critical' ? 'error' : issue.severity === 'warning' ? 'warning' : 'info'}
                    </span>
                    <span style="color: var(--color-text-secondary)"><strong>{issue.rule}</strong>: {issue.message}</span>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {/if}

        <div class="text-xs text-[var(--color-text-muted)]">
          <span class="flex items-center gap-1">
            <span class="material-symbols-outlined text-[12px]">security</span>
            导入前将自动进行安全扫描
          </span>
        </div>
      </div>
      <div class="flex justify-end gap-2 px-6 py-4 border-t border-[var(--color-border)]">
        <button class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>取消</button>
        <button class="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={onScanAndImport} disabled={importing || scanning || !selectedImportProject}>
          {#if importing || scanning}
            <span class="material-symbols-outlined text-[14px] animate-spin">progress_activity</span>
          {:else}
            <span class="material-symbols-outlined text-[14px]">security</span>
          {/if}
          {importing ? '导入中...' : scanning ? '扫描中...' : '安全导入'}
        </button>
      </div>
    </div>
  </div>
{/if}

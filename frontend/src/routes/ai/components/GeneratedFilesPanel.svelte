<script lang="ts">
import type { Mode } from '../lib/types';

let {
  show = false,
  files = [],
  mode = 'generate',
  viewMode = 'files',
  onClose,
  onViewModeChange,
  onDeploy,
}: {
  show: boolean;
  files: { path: string; content: string; oldContent?: string }[];
  mode: Mode;
  viewMode: 'diff' | 'files';
  onClose: () => void;
  onViewModeChange: (m: 'diff' | 'files') => void;
  onDeploy: () => void;
} = $props();
</script>

{#if show && files.length > 0}
  <div class="border-t border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
    <div class="flex items-center gap-2 px-4 py-2 border-b border-[var(--color-border)]">
      <span class="text-xs font-semibold text-[var(--color-text)]">生成的文件</span>
      <div class="flex gap-1 ml-auto">
        <button class="px-2 py-1 rounded text-[10px] font-medium transition-colors" style="background: {viewMode === 'diff' ? 'var(--color-primary-light)' : 'var(--color-surface)'}; color: {viewMode === 'diff' ? 'var(--color-primary)' : 'var(--color-text-secondary)'}" onclick={() => onViewModeChange('diff')}>Diff</button>
        <button class="px-2 py-1 rounded text-[10px] font-medium transition-colors" style="background: {viewMode === 'files' ? 'var(--color-primary-light)' : 'var(--color-surface)'}; color: {viewMode === 'files' ? 'var(--color-primary)' : 'var(--color-text-secondary)'}" onclick={() => onViewModeChange('files')}>Files</button>
        {#if mode === 'auto-build'}
          <button class="flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-green-600 text-white hover:bg-green-700 transition-colors" onclick={onDeploy} title="一键部署到设备">
            <span class="material-symbols-outlined text-[12px]">sell</span>
            部署
          </button>
        {/if}
        <button class="p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[14px]" style="color: var(--color-text-muted)">close</span>
        </button>
      </div>
    </div>
    <div class="max-h-48 overflow-y-auto p-3 space-y-2">
      {#each files as gf}
        <div class="rounded-lg border border-[var(--color-border)] overflow-hidden">
          <div class="flex items-center gap-2 px-3 py-1.5 text-xs font-mono" style="background: var(--color-surface); border-bottom: 1px solid var(--color-border);">
            <span class="material-symbols-outlined text-[12px] text-primary-500">description</span>
            {gf.path}
          </div>
          {#if viewMode === 'diff' && gf.oldContent != null}
            <div class="p-2 text-[10px] font-mono leading-relaxed" style="color: var(--color-text);">
              <div class="text-green-500">+ {gf.content}</div>
              <div class="text-red-500">- {gf.oldContent}</div>
            </div>
          {:else}
            <pre class="p-2 text-[10px] font-mono leading-relaxed overflow-x-auto" style="color: var(--color-text);">{gf.content}</pre>
          {/if}
        </div>
      {/each}
    </div>
  </div>
{/if}

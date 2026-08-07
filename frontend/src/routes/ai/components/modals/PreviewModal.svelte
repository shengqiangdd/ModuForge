<script lang="ts">
import { focusTrap } from '$lib/utils/focusTrap';
import { getFileLanguage, getFileIcon, getWebUIPreviewHTML, checkWebUIFiles } from '../../lib/utils';

let {
  show = false,
  files = [],
  onClose,
}: {
  show: boolean;
  files: { path: string; content: string }[];
  onClose: () => void;
} = $props();

let selectedFile = $state<string | null>(null);
let webUIMode = $state(false);
let hasWebUI = $derived(checkWebUIFiles(files));
let webUIHTML = $derived(hasWebUI ? getWebUIPreviewHTML(files) : '');

function getPreviewContent(): string {
  if (!selectedFile) return '';
  const f = files.find(x => x.path === selectedFile);
  return f?.content || '';
}

$effect(() => {
  if (show && files.length > 0 && !selectedFile) {
    selectedFile = files[0].path;
  }
});
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) { onClose(); webUIMode = false; } }} onkeydown={(e) => { if (e.key === 'Escape') { onClose(); webUIMode = false; } }}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-4xl max-h-[85vh] flex flex-col border border-[var(--color-border)]" role="dialog" aria-modal="true" tabindex="-1" use:focusTrap>
      <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)]">
        <div class="flex items-center gap-2">
          {#if hasWebUI && webUIMode}
            <span class="material-symbols-outlined text-primary-600">web</span>
            <h2 class="text-lg font-semibold text-[var(--color-text)]">WebUI 预览</h2>
          {:else}
            <span class="material-symbols-outlined text-primary-600">folder_open</span>
            <h2 class="text-lg font-semibold text-[var(--color-text)]">模块文件预览</h2>
          {/if}
        </div>
        <div class="flex items-center gap-2">
          {#if hasWebUI}
            <button
              class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors {webUIMode ? 'bg-primary-600 text-white' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
              onclick={() => webUIMode = !webUIMode}
            >
              <span class="material-symbols-outlined text-[14px]">{webUIMode ? 'folder_open' : 'web'}</span>
              {webUIMode ? '文件视图' : 'WebUI 预览'}
            </button>
          {/if}
          <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={() => { onClose(); webUIMode = false; }} aria-label="关闭">
            <span class="material-symbols-outlined text-[20px]">close</span>
          </button>
        </div>
      </div>
      {#if hasWebUI && webUIMode && webUIHTML}
        <div class="flex flex-1 overflow-hidden">
          <div class="flex-1 flex flex-col">
            <iframe sandbox="allow-scripts" srcdoc={webUIHTML} class="w-full h-full border-0" title="WebUI Preview"></iframe>
          </div>
          <div class="w-64 border-l border-[var(--color-border)] overflow-y-auto p-3 flex-shrink-0">
            <p class="text-xs font-medium text-[var(--color-text-secondary)] mb-2">WebUI 文件</p>
            <div class="space-y-0.5">
              {#each files.filter(f => f.path.startsWith('webroot/')) as pf}
                <button
                  class="flex items-center gap-2 w-full px-2.5 py-1.5 rounded-lg text-xs text-left transition-colors {selectedFile === pf.path ? 'bg-primary-600 text-white' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
                  onclick={() => { selectedFile = pf.path; webUIMode = false; }}
                >
                  <span class="material-symbols-outlined text-[14px]">{getFileIcon(pf.path)}</span>
                  <span class="font-mono truncate">{pf.path.split('/').pop()}</span>
                </button>
              {/each}
            </div>
          </div>
        </div>
      {:else}
        <div class="flex flex-1 overflow-hidden">
          <div class="w-64 border-r border-[var(--color-border)] overflow-y-auto p-3 flex-shrink-0">
            <div class="space-y-0.5">
              {#each files as pf}
                <button
                  class="flex items-center gap-2 w-full px-2.5 py-1.5 rounded-lg text-xs text-left transition-colors {selectedFile === pf.path ? 'bg-primary-600 text-white' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
                  onclick={() => selectedFile = pf.path}
                >
                  <span class="material-symbols-outlined text-[14px]">{getFileIcon(pf.path)}</span>
                  <span class="font-mono truncate">{pf.path.split('/').pop()}</span>
                </button>
              {/each}
            </div>
          </div>
          <div class="flex-1 flex flex-col overflow-hidden">
            {#if selectedFile}
              <div class="px-4 py-2 border-b border-[var(--color-border)] bg-[var(--color-surface)] flex items-center gap-2">
                <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]">{getFileIcon(selectedFile)}</span>
                <code class="text-xs font-mono text-[var(--color-text-secondary)]">{selectedFile}</code>
                <span class="ml-auto text-[10px] px-1.5 py-0.5 rounded font-mono" style="background: var(--color-primary-light); color: var(--color-primary)">
                  {getFileLanguage(selectedFile)}
                </span>
              </div>
              <div class="flex-1 overflow-auto p-4">
                <pre class="text-xs font-mono leading-relaxed whitespace-pre-wrap" style="color: var(--color-text); tab-size: 2;"><code>{getPreviewContent()}</code></pre>
              </div>
            {:else}
              <div class="flex items-center justify-center h-full text-sm text-[var(--color-text-muted)]">选择一个文件查看内容</div>
            {/if}
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

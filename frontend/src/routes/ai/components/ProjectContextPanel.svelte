<script lang="ts">
import type { ContextProject } from '../lib/types';

let {
  show = false,
  contextProjectList = [],
  contextProjects = [],
  selectedProject = '',
  selectedFile = '',
  projectContext = '',
  onClose,
  onProjectChange,
  onFileAdd,
  onContextChange,
}: {
  show: boolean;
  contextProjectList: { id: string; name: string }[];
  contextProjects: ContextProject[];
  selectedProject: string;
  selectedFile: string;
  projectContext: string;
  onClose: () => void;
  onProjectChange: (v: string) => void;
  onFileAdd: (v: string) => void;
  onContextChange: (v: string) => void;
} = $props();
</script>

{#if show}
  <div class="border-t border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-3">
    <div class="flex items-center gap-2 mb-2">
      <span class="text-xs font-semibold text-[var(--color-text)]">项目上下文</span>
      <button class="ml-auto p-1 rounded hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
        <span class="material-symbols-outlined text-[14px]" style="color: var(--color-text-muted)">close</span>
      </button>
    </div>
    <div class="flex gap-2 mb-2">
      <select class="flex-1 input-field text-xs" value={selectedProject} onchange={(e) => onProjectChange((e.target as HTMLSelectElement).value)}>
        <option value="">选择项目...</option>
        {#each contextProjectList as p}
          <option value={p.id}>{p.name} ({p.id.slice(0,8)}…)</option>
        {/each}
      </select>
    </div>
    {#if contextProjects.length > 0}
      <select class="input-field text-xs w-full mb-2" value={selectedFile} onchange={(e) => { const v = (e.target as HTMLSelectElement).value; if (v) { onFileAdd(v); } }}>
        <option value="">选择文件添加到上下文...</option>
        {#each contextProjects as cp}
          {#each cp.files as f}
            <option value={f}>{cp.name ? cp.name + ' / ' : ''}{f}</option>
          {/each}
        {/each}
      </select>
    {/if}
    <textarea class="input-field text-xs font-mono resize-none w-full" rows="3" placeholder="额外的上下文信息..." value={projectContext} oninput={(e) => onContextChange((e.target as HTMLTextAreaElement).value)}></textarea>
  </div>
{/if}

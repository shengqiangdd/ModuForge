<script lang="ts">
import CodeDiff from '$lib/components/CodeDiff.svelte';
import type { DiffEntry } from '$lib/api/client';
import { toast } from '$lib/stores/toast.svelte';

let {
  show = false,
  diffs = [],
  filePath = '',
  onClose,
}: {
  show: boolean;
  diffs: DiffEntry[];
  filePath: string;
  onClose: () => void;
} = $props();

let viewMode = $state<'unified' | 'split'>('unified');
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" onclick={onClose}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-4xl max-h-[85vh] flex flex-col border border-[var(--color-border)]" onclick={(e) => e.stopPropagation()}>
      <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)]">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-primary-600">difference</span>
          <h2 class="text-lg font-semibold text-[var(--color-text)]">代码差异对比</h2>
        </div>
        <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="flex-1 overflow-y-auto p-4">
        <CodeDiff {diffs} {filePath} bind:viewMode />
      </div>
      <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-[var(--color-border)]">
        <button class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>关闭</button>
        <button class="flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium bg-green-600 text-white hover:bg-green-700 transition-colors" onclick={() => { toast('修改已接受', 'success'); onClose(); }}>
          <span class="material-symbols-outlined text-[14px]">check</span>
          接受修改
        </button>
      </div>
    </div>
  </div>
{/if}

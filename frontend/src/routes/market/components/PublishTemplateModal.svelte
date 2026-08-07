<script lang="ts">
  let { show, onClose, onPublish, publishing, publishName, publishDesc, publishCategory }: {
    show: boolean;
    onClose: () => void;
    onPublish: () => void;
    publishing: boolean;
    publishName: string;
    publishDesc: string;
    publishCategory: string;
  } = $props();
</script>

{#if show}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
    <div class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">发布模板</h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="p-5 space-y-3">
        <div>
          <label for="publish-name" class="block text-sm font-medium mb-1">模板名称</label>
          <input id="publish-name" type="text" placeholder="e.g. System Prop Tweaks" class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={publishName} />
        </div>
        <div>
          <label for="publish-desc" class="block text-sm font-medium mb-1">描述</label>
          <textarea id="publish-desc" placeholder="描述此模板的功能..." class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] h-20 resize-none" bind:value={publishDesc}></textarea>
        </div>
        <div>
          <label for="publish-category" class="block text-sm font-medium mb-1">分类</label>
          <select id="publish-category" class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={publishCategory}>
            <option value="">选择分类</option>
            <option value="system">系统</option>
            <option value="ui">界面</option>
            <option value="audio">音频</option>
            <option value="display">显示</option>
            <option value="utility">工具</option>
          </select>
        </div>
      </div>
      <div class="p-5 border-t flex justify-end gap-2" style="border-color: var(--color-border)">
        <button class="px-4 py-2 rounded-xl text-sm font-medium transition-colors" style="border: 1px solid var(--color-border); color: var(--color-text-secondary)" onclick={onClose}>取消</button>
        <button class="px-4 py-2 rounded-xl text-sm font-semibold text-white transition-all" style="background: var(--gradient-brand)" onclick={onPublish} disabled={publishing || !publishName.trim()}>
          {publishing ? '发布中...' : '发布'}
        </button>
      </div>
    </div>
  </div>
{/if}
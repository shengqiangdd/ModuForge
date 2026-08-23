<script lang="ts">
  interface Props {
    search: string;
    loading: boolean;
    onSearch: (value: string) => void;
    onRefresh: () => void;
    onAdd: () => void;
  }

  let { search, loading, onSearch, onRefresh, onAdd }: Props = $props();
</script>

<div class="flex flex-col md:flex-row md:items-center gap-3 mb-6">
  <div class="flex items-center gap-3">
    <div class="w-11 h-11 rounded-2xl flex items-center justify-center flex-shrink-0" style="background: var(--gradient-brand-subtle)">
      <span class="material-symbols-outlined" style="color: var(--color-primary)">hub</span>
    </div>
    <div class="flex-1 min-w-0">
      <h1 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">MCP 服务器</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">
        Model Context Protocol — 外部工具接入（Claude Code / OpenCode 同款协议）
      </p>
    </div>
  </div>
  <div class="flex items-center gap-2 md:ml-auto">
    <div class="relative flex-1 md:flex-none">
      <span class="material-symbols-outlined absolute left-2.5 top-1/2 -translate-y-1/2 text-[16px]" style="color: var(--color-text-muted)">search</span>
      <input
        value={search}
        oninput={(e) => onSearch((e.target as HTMLInputElement).value)}
        placeholder="搜索工具..."
        class="input-field pl-8 py-2 text-sm w-full md:w-56"
      />
    </div>
    <button class="btn-primary flex items-center gap-1.5" onclick={onRefresh} disabled={loading}>
      <span class="material-symbols-outlined text-[18px]">refresh</span>
      <span class="hidden md:inline">刷新</span>
    </button>
    <button class="btn-primary flex items-center gap-1.5" onclick={onAdd}>
      <span class="material-symbols-outlined text-[18px]">add</span>
      <span class="hidden md:inline">添加服务器</span>
    </button>
  </div>
</div>

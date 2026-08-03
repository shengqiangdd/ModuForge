<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';

  let {
    language = '',
    content,
    codeKey,
    lineCount = 0,
    collapsed = false,
    onCopy,
  }: {
    language?: string;
    content: string;
    codeKey: string;
    lineCount?: number;
    collapsed?: boolean;
    onCopy?: (text: string) => void;
  } = $props();

  let collapsedState = $state(collapsed);
  let defaultCollapsed = $derived(lineCount > 50 && !collapsedState);
  let isCollapsed = $derived(collapsedState || defaultCollapsed);

  function handleCopy() {
    if (onCopy) {
      onCopy(content);
    } else {
      navigator.clipboard?.writeText(content).then(() => {
        toast('已复制', 'success');
      }).catch(() => {
        toast('复制失败', 'error');
      });
    }
  }

  function toggleCollapse() {
    collapsedState = !collapsedState;
  }
</script>

<div class="rounded-xl overflow-hidden border border-[var(--color-border)] relative group" style="background: #1e1e2e;">
  {#if language}
    <div class="flex items-center gap-1.5 px-3 py-1.5 text-[10px] font-medium" style="background: #181825; color: #a6adc8; border-bottom: 1px solid #313244;">
      <span class="material-symbols-outlined text-[12px]">code</span>
      {language}
      <div class="ml-auto flex items-center gap-1">
        <button
          class="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] transition-colors hover:bg-white/10"
          style="color: #a6adc8;"
          onclick={handleCopy}
        >
          <span class="material-symbols-outlined text-[12px]">content_copy</span>
        </button>
        {#if lineCount > 20}
          <button
            class="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] transition-colors hover:bg-white/10"
            style="color: #a6adc8;"
            onclick={toggleCollapse}
          >
            <span class="material-symbols-outlined text-[12px]">{isCollapsed ? 'expand_more' : 'expand_less'}</span>
            {isCollapsed ? '展开' : '折叠'}
          </button>
        {/if}
      </div>
    </div>
  {:else}
    <button
      class="absolute top-2 right-2 z-10 p-1.5 rounded-lg opacity-0 group-hover:opacity-100 transition-opacity hover:bg-white/10"
      style="color: #a6adc8;"
      onclick={handleCopy}
    >
      <span class="material-symbols-outlined text-[12px]">content_copy</span>
    </button>
  {/if}
  <pre
    class="p-3 text-xs font-mono leading-relaxed overflow-x-auto"
    style="color: #cdd6f4; tab-size: 2; {isCollapsed && lineCount > 20 ? 'max-height: 200px; overflow-y: hidden;' : ''}"
  ><code>{content}</code></pre>
  {#if isCollapsed && lineCount > 20}
    <button
      class="w-full py-2 text-[10px] font-medium text-center transition-colors cursor-pointer relative z-10 hover:bg-white/5"
      style="background: #181825; color: #a6adc8; border-top: 1px solid #313244;"
      onclick={toggleCollapse}
    >
      展开全部 ({lineCount} 行)
    </button>
  {/if}
</div>

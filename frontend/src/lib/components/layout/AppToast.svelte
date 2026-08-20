<script lang="ts">
  import Toast from '$lib/components/ui/Toast.svelte';

  interface Props {
    offline: boolean;
    errorCaught: string | null;
    errorDetail: string | null;
    onDismissError: () => void;
    onReloadPage: () => void;
  }

  let { offline, errorCaught, errorDetail, onDismissError, onReloadPage }: Props = $props();
</script>

<Toast />

{#if offline}
  <div class="fixed top-0 left-0 right-0 z-[100] flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-white" style="background: var(--color-error);">
    <span class="material-symbols-outlined text-[16px]">wifi_off</span>
    <span>网络已断开 — 部分功能不可用</span>
  </div>
{/if}

{#if errorCaught}
  <div class="fixed inset-0 z-[200] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);">
    <div class="rounded-2xl shadow-2xl p-8 max-w-md text-center border" style="background: var(--color-bg-elevated); border-color: var(--color-border);">
      <span class="material-symbols-outlined text-5xl mb-3 block" style="color: var(--color-error)">error_outline</span>
      <h2 class="text-lg font-bold mb-2" style="color: var(--color-text)">出现异常</h2>
      <p class="text-sm mb-2" style="color: var(--color-text-secondary)">{errorCaught}</p>
      {#if errorDetail}
        <p class="text-xs mb-4 font-mono" style="color: var(--color-text-muted); word-break: break-all">{errorDetail}</p>
      {/if}
      <div class="flex gap-3 justify-center">
        <button class="btn-primary" onclick={onDismissError}>重试</button>
        <button class="btn-ghost" onclick={onReloadPage}>刷新页面</button>
      </div>
    </div>
  </div>
{/if}

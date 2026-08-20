<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';

  let showInstallPrompt = $state(false);
  let deferredPrompt: Event & { prompt: () => void; userChoice: Promise<{ outcome: string }> } | null = null;

  async function installPWA() {
    if (!deferredPrompt) return;
    deferredPrompt.prompt();
    const result = await deferredPrompt.userChoice;
    if (result.outcome === 'accepted') {
      toast('已安装到桌面', 'success');
    }
    deferredPrompt = null;
    showInstallPrompt = false;
  }

  function handleBeforeInstallPrompt(e: Event) {
    e.preventDefault();
    deferredPrompt = e as any;
    showInstallPrompt = true;
  }

  onMount(() => {
    window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt);
  });

  onDestroy(() => {
    window.removeEventListener('beforeinstallprompt', handleBeforeInstallPrompt);
  });
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">download</span>
    </div>
    <div class="flex-1">
      <h2 class="text-base font-semibold text-[var(--color-text)]">安装到桌面</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">将 ModuForge 安装为 PWA 应用</p>
    </div>
  </div>
  {#if showInstallPrompt}
    <button class="btn-primary text-sm" onclick={installPWA}>
      <span class="material-symbols-outlined text-[16px]">install_mobile</span>
      安装到桌面
    </button>
  {:else}
    <p class="text-xs" style="color: var(--color-text-muted)">PWA 安装按钮会在支持的应用商店中自动显示</p>
  {/if}
</section>

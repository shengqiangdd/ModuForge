<script lang="ts">
  let { htmlContent = '', url = '' }: {
    htmlContent?: string;
    url?: string;
  } = $props();

  let iframe: HTMLIFrameElement;
  let loading = $state(true);
  let error = $state(false);

  function refresh() {
    loading = true;
    error = false;
    if (iframe) {
      if (url) {
        iframe.src = url;
      } else if (htmlContent) {
        const blob = new Blob([htmlContent], { type: 'text/html' });
        iframe.src = URL.createObjectURL(blob);
      }
    }
  }

  function onLoad() {
    loading = false;
  }

  function onError() {
    loading = false;
    error = true;
  }

  $effect(() => {
    if (htmlContent || url) {
      refresh();
    }
  });
</script>

<div class="preview-panel flex flex-col h-full">
  <div class="flex items-center justify-between px-4 py-2 border-b border-[var(--color-border)] bg-surface">
    <span class="text-body-medium text-[var(--color-text)]">预览</span>
    <md-filled-tonal-button size="small" onclick={refresh}>
      <md-icon slot="start">refresh</md-icon>
      刷新
    </md-filled-tonal-button>
  </div>

  <div class="flex-1 relative bg-white">
    {#if loading}
      <div class="absolute inset-0 flex items-center justify-center bg-[var(--color-surface)]">
        <md-circular-progress indeterminate />
      </div>
    {/if}

    {#if error}
      <div class="absolute inset-0 flex items-center justify-center bg-[var(--color-surface)]">
        <div class="text-center text-error">
          <md-icon class="text-4xl mb-2">error</md-icon>
          <p>加载失败</p>
        </div>
      </div>
    {/if}

    <iframe
      bind:this={iframe}
      sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
      class="w-full h-full border-0"
      title="预览"
      onload={onLoad}
      onerror={onError}
    ></iframe>
  </div>
</div>

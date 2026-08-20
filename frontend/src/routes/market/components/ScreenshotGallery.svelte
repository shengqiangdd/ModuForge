<script lang="ts">
  let {
    screenshots = [],
    coverImage = '',
    onFullscreen,
  }: {
    screenshots?: { url: string }[];
    coverImage?: string;
    onFullscreen?: (url: string) => void;
  } = $props();

  let galleryIndex = $state(0);
</script>

{#if screenshots && screenshots.length > 0}
  <div class="mt-4">
    <div class="relative rounded-xl overflow-hidden">
      <button class="block w-full" onclick={() => onFullscreen?.(screenshots[galleryIndex]?.url)}>
        <img src={screenshots[galleryIndex]?.url} alt="截图" class="w-full h-48 object-cover cursor-pointer" />
      </button>
      {#if screenshots.length > 1}
        <button
          class="absolute left-2 top-1/2 -translate-y-1/2 w-8 h-8 rounded-full flex items-center justify-center bg-black/40 text-white"
          onclick={(e) => { e.stopPropagation(); galleryIndex = galleryIndex > 0 ? galleryIndex - 1 : screenshots.length - 1; }}
        >
          <span class="material-symbols-outlined text-[18px]">chevron_left</span>
        </button>
        <button
          class="absolute right-2 top-1/2 -translate-y-1/2 w-8 h-8 rounded-full flex items-center justify-center bg-black/40 text-white"
          onclick={(e) => { e.stopPropagation(); galleryIndex = galleryIndex < screenshots.length - 1 ? galleryIndex + 1 : 0; }}
        >
          <span class="material-symbols-outlined text-[18px]">chevron_right</span>
        </button>
      {/if}
    </div>
    {#if screenshots.length > 1}
      <div class="flex gap-1.5 mt-2 overflow-x-auto pb-1">
        {#each screenshots as ss, i}
          <div
            role="button"
            tabindex="0"
            class="w-12 h-8 rounded overflow-hidden flex-shrink-0 cursor-pointer border-2 transition-colors"
            style="border-color: {i === galleryIndex ? 'var(--color-primary)' : 'transparent'}"
            onclick={() => galleryIndex = i}
            onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); galleryIndex = i; } }}
          >
            <img src={ss.url} alt="" class="w-full h-full object-cover" />
          </div>
        {/each}
      </div>
    {/if}
  </div>
{:else if coverImage}
  <div class="mt-4 rounded-xl overflow-hidden">
    <img src={coverImage} alt="封面" class="w-full h-48 object-cover" />
  </div>
{/if}

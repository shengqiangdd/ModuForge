<script lang="ts">
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-success-light)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-success)">info</span>
    </div>
    <div>
      <h2 class="text-base font-semibold text-[var(--color-text)]">关于</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">ModuForge 版本和系统信息</p>
    </div>
  </div>
  <div class="space-y-2 text-sm text-[var(--color-text-secondary)]">
    <div class="flex justify-between py-1"><span>版本</span><span class="font-medium text-[var(--color-text)]">2.0-lite</span></div>
    <div class="flex justify-between py-1"><span>前端框架</span><span class="font-medium text-[var(--color-text)]">Svelte 5 + UnoCSS</span></div>
    <div class="flex justify-between py-1"><span>后端框架</span><span class="font-medium text-[var(--color-text)]">Go + Fiber + SQLite</span></div>
  </div>
  <a href="/docs" target="_blank" class="mt-4 inline-flex items-center gap-2 text-sm font-medium" style="color: var(--color-primary)">
    <span class="material-symbols-outlined text-[16px]">description</span>
    API 文档（Swagger UI）
  </a>
</section>

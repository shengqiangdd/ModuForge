<script lang="ts">
  import { renderMarkdown } from '$lib/utils/markdown';

  let { show, versions, versionsLoading, selectedModuleSlug, rollingBack, showVersionUpdate, newVersionCode, newChangelog, updatingVersion, onClose, onRollback, onPublishVersion, onShowUpdate, onHideUpdate }: {
    show: boolean;
    versions: { id: string; version: string; version_code: string; changelog?: string; created_at?: string }[];
    versionsLoading: boolean;
    selectedModuleSlug: string;
    rollingBack: string | null;
    showVersionUpdate: boolean;
    newVersionCode: string;
    newChangelog: string;
    updatingVersion: boolean;
    onClose: () => void;
    onRollback: (versionId: string) => void;
    onPublishVersion: () => void;
    onShowUpdate: () => void;
    onHideUpdate: () => void;
  } = $props();
</script>

{#if show}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
    <div class="rounded-2xl max-w-lg w-full max-h-[70vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">版本历史</h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="p-4">
        <button class="w-full flex items-center gap-2 px-3 py-2 rounded-xl text-sm font-medium mb-3 transition-colors" style="background: var(--color-primary-light); color: var(--color-primary)" onclick={onShowUpdate}>
          <span class="material-symbols-outlined text-[16px]">add</span>
          发布新版本
        </button>
        {#if versionsLoading}
          <div class="flex justify-center py-8"><div class="animate-spin h-6 w-6 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div></div>
        {:else if versions.length === 0}
          <p class="text-center py-8 text-sm text-[var(--color-text-muted)]">暂无版本记录</p>
        {:else}
          <div class="space-y-3">
            {#each versions as ver}
              <div class="p-4 rounded-xl" style="background: var(--color-surface)">
                <div class="flex items-start justify-between">
                  <div>
                    <div class="flex items-center gap-2 mb-1">
                      <span class="text-sm font-semibold text-[var(--color-text)]">{ver.version}</span>
                      <span class="text-xs text-[var(--color-text-muted)]">{ver.version_code}</span>
                    </div>
                    {#if ver.changelog}
                      <p class="text-xs text-[var(--color-text-secondary)] mt-1">{ver.changelog}</p>
                    {/if}
                    <p class="text-[11px] text-[var(--color-text-muted)] mt-1">{ver.created_at?.slice(0, 10)}</p>
                  </div>
                  <button
                    class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50"
                    style="background: var(--color-warning-light); color: var(--color-warning)"
                    disabled={rollingBack === ver.id}
                    onclick={() => onRollback(ver.id)}
                  >
                    {rollingBack === ver.id ? '回滚中...' : '回滚到此版本'}
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </div>

  <!-- Version Update Dialog -->
  {#if showVersionUpdate}
    <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onHideUpdate(); }}>
      <div class="rounded-2xl max-w-md w-full border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
        <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
          <h3 class="text-lg font-bold text-[var(--color-text)]">发布新版本</h3>
          <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={onHideUpdate}>
            <span class="material-symbols-outlined text-[20px]">close</span>
          </button>
        </div>
        <div class="p-5 space-y-4">
          <div>
            <label for="version-code" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">版本代码</label>
            <input id="version-code" type="text" class="input-field" placeholder="1.1.0" bind:value={newVersionCode} />
          </div>
          <div>
            <label for="version-changelog" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">更新日志</label>
            <textarea id="version-changelog" class="input-field resize-none" rows="3" placeholder="描述本次更新内容..." bind:value={newChangelog}></textarea>
          </div>
          <div class="flex justify-end gap-3 pt-2">
            <button class="btn-ghost" onclick={onHideUpdate}>取消</button>
            <button
              class="btn-primary disabled:opacity-50"
              disabled={updatingVersion || !newVersionCode.trim()}
              onclick={onPublishVersion}
            >
              {updatingVersion ? '发布中...' : '发布'}
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}
{/if}
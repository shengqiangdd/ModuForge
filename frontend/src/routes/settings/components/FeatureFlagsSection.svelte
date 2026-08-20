<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  interface FeatureFlag {
    key: string;
    description: string;
    enabled: boolean;
  }

  let flags = $state<FeatureFlag[]>([]);
  let loading = $state(true);
  let saving = $state(false);
  let savingBatch = $state(false);
  let searchQuery = $state('');
  let pendingChanges = $state<Record<string, boolean>>({});

  const authHeaders = () => ({
    'Content-Type': 'application/json',
    Authorization: `Bearer ${getToken()}`,
  });

  async function loadFlags() {
    loading = true;
    try {
      const r = await fetch('/api/v1/admin/feature-flags', { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) {
        const data = await r.json();
        flags = data.flags ?? [];
      } else {
        toast('加载功能开关失败', 'error');
      }
    } catch {
      toast('网络错误', 'error');
    }
    loading = false;
  }

  function toggleFlag(key: string, currentEnabled: boolean) {
    pendingChanges = { ...pendingChanges, [key]: !currentEnabled };
  }

  function getEffectiveEnabled(flag: FeatureFlag): boolean {
    return flag.key in pendingChanges ? pendingChanges[flag.key] : flag.enabled;
  }

  async function saveSingle(key: string) {
    saving = true;
    const enabled = pendingChanges[key];
    try {
      const r = await fetch(`/api/v1/admin/feature-flags/${key}`, {
        method: 'PUT',
        headers: authHeaders(),
        body: JSON.stringify({ enabled }),
      });
      if (r.ok) {
        toast(`已${enabled ? '启用' : '禁用'} ${flagDescription(key)}`, 'success');
        const next = { ...pendingChanges };
        delete next[key];
        pendingChanges = next;
        await loadFlags();
      } else {
        toast('更新失败', 'error');
      }
    } catch {
      toast('网络错误', 'error');
    }
    saving = false;
  }

  async function saveAll() {
    savingBatch = true;
    const items = Object.entries(pendingChanges).map(([key, enabled]) => ({ key, enabled }));
    try {
      const r = await fetch('/api/v1/admin/feature-flags/batch', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ flags: items }),
      });
      if (r.ok) {
        toast(`已批量更新 ${items.length} 个功能开关`, 'success');
        pendingChanges = {};
        await loadFlags();
      } else {
        toast('批量更新失败', 'error');
      }
    } catch {
      toast('网络错误', 'error');
    }
    savingBatch = false;
  }

  function flagDescription(key: string): string {
    const f = flags.find((f) => f.key === key);
    return f?.description ?? key;
  }

  function resetPending() {
    pendingChanges = {};
  }

  const filteredFlags = $derived(
    searchQuery
      ? flags.filter(
          (f) =>
            f.key.includes(searchQuery.toLowerCase()) ||
            f.description.includes(searchQuery)
        )
      : flags
  );

  const hasPendingChanges = $derived(Object.keys(pendingChanges).length > 0);

  onMount(loadFlags);
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light, rgba(99,102,241,0.1))">
      <span class="material-symbols-outlined text-[18px] text-indigo-500">toggle_on</span>
    </div>
    <div>
      <h2 class="text-base font-semibold text-[var(--color-text)]">功能开关</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">启用或禁用平台功能模块，需管理员权限</p>
    </div>
  </div>

  {#if loading}
    <div class="flex items-center justify-center py-8">
      <span class="material-symbols-outlined animate-spin text-[var(--color-text-muted)]">progress_activity</span>
    </div>
  {:else}
    <div class="space-y-4">
      <!-- Search -->
      <div class="relative">
        <span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-[16px]" style="color: var(--color-text-muted)">search</span>
        <input
          type="text"
          class="input-field pl-9"
          placeholder="搜索功能名称..."
          bind:value={searchQuery}
        />
      </div>

      <!-- Pending changes indicator -->
      {#if hasPendingChanges}
        <div class="flex items-center gap-2 p-3 rounded-lg" style="background: rgba(251,191,36,0.08); border: 1px solid rgba(251,191,36,0.25)">
          <span class="material-symbols-outlined text-[16px] text-amber-500">info</span>
          <span class="text-sm" style="color: var(--color-text-secondary)">
            有 {Object.keys(pendingChanges).length} 个未保存的更改
          </span>
          <div class="flex gap-2 ml-auto">
            <button
              type="button"
              class="px-3 py-1 text-xs font-medium rounded-lg"
              style="background: var(--color-surface-secondary); color: var(--color-text); border: 1px solid var(--color-border)"
              onclick={resetPending}
            >
              取消
            </button>
            <button
              type="button"
              class="px-3 py-1 text-xs font-medium rounded-lg text-white"
              style="background: var(--color-primary, #6366f1)"
              onclick={saveAll}
              disabled={savingBatch}
            >
              {savingBatch ? '保存中...' : '全部保存'}
            </button>
          </div>
        </div>
      {/if}

      <!-- Flags list -->
      <div class="space-y-2">
        {#each filteredFlags as flag (flag.key)}
          {@const effectiveEnabled = getEffectiveEnabled(flag)}
          {@const isDirty = flag.key in pendingChanges}
          <div
            class="flex items-center justify-between p-3 rounded-lg transition-colors"
            style="background: {isDirty ? 'rgba(99,102,241,0.04)' : 'var(--color-surface-secondary, rgba(0,0,0,0.02))'}; border: 1px solid {isDirty ? 'rgba(99,102,241,0.2)' : 'var(--color-border, transparent)'}"
          >
            <div class="flex-1 min-w-0 mr-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-[var(--color-text)] truncate">{flag.key}</span>
                {#if isDirty}
                  <span class="px-1.5 py-0.5 text-[10px] font-semibold rounded" style="background: rgba(251,191,36,0.15); color: #b45309">未保存</span>
                {/if}
              </div>
              <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">{flag.description}</p>
            </div>
            <div class="flex items-center gap-2">
              <button
                type="button"
                role="switch"
                aria-checked={effectiveEnabled}
                aria-label={effectiveEnabled ? `禁用 ${flag.key}` : `启用 ${flag.key}`}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full transition-colors"
                style="background: {effectiveEnabled ? 'var(--color-primary, #6366f1)' : 'var(--color-border, #d1d5db)'}"
                onclick={() => toggleFlag(flag.key, effectiveEnabled)}
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transition-transform"
                  style="transform: translateX({effectiveEnabled ? '20px' : '2px'})"
                ></span>
              </button>
              {#if isDirty}
                <button
                  type="button"
                  class="p-1 rounded hover:bg-[var(--color-surface-hover)]"
                  title="保存此项"
                  onclick={() => saveSingle(flag.key)}
                  disabled={saving}
                >
                  <span class="material-symbols-outlined text-[16px] text-[var(--color-primary, #6366f1)]">check</span>
                </button>
              {/if}
            </div>
          </div>
        {/each}

        {#if filteredFlags.length === 0}
          <p class="text-center py-6 text-sm" style="color: var(--color-text-muted)">
            {searchQuery ? '未找到匹配的功能开关' : '暂无功能开关'}
          </p>
        {/if}
      </div>
    </div>
  {/if}
</section>

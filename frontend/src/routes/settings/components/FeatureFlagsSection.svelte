<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let { isAdmin }: { isAdmin: boolean } = $props();

  interface FeatureFlag {
    key: string;
    description: string;
    enabled: boolean;
  }

  let flags: FeatureFlag[] = $state([]);
  let loading = $state(false);
  let saving = $state(false);
  let searchText = $state('');

  const filteredFlags = $derived(
    searchText
      ? flags.filter(
          (f) =>
            f.key.toLowerCase().includes(searchText.toLowerCase()) ||
            f.description.toLowerCase().includes(searchText.toLowerCase())
        )
      : flags
  );

  const enabledCount = $derived(flags.filter((f) => f.enabled).length);

  async function loadFlags() {
    loading = true;
    try {
      const r = await fetch('/api/v1/admin/feature-flags', {
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      if (r.ok) {
        const d = await r.json();
        flags = d.flags || [];
      } else {
        toast('加载功能开关失败', 'error');
      }
    } catch {
      toast('加载功能开关失败', 'error');
    }
    loading = false;
  }

  async function toggleFlag(key: string, enabled: boolean) {
    const prev = flags.map((f) => ({ ...f }));
    flags = flags.map((f) => (f.key === key ? { ...f, enabled } : f));

    try {
      const r = await fetch(`/api/v1/admin/feature-flags/${key}`, {
        method: 'PUT',
        headers: {
          Authorization: `Bearer ${getToken()}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ enabled }),
      });
      if (r.ok) {
        toast(`${key} 已${enabled ? '启用' : '禁用'}`, 'success');
      } else {
        flags = prev;
        toast('更新失败', 'error');
      }
    } catch {
      flags = prev;
      toast('更新失败', 'error');
    }
  }

  async function enableAll() {
    saving = true;
    const batchFlags = flags.map((f) => ({ key: f.key, enabled: true }));
    try {
      const r = await fetch('/api/v1/admin/feature-flags/batch', {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${getToken()}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ flags: batchFlags }),
      });
      if (r.ok) {
        flags = flags.map((f) => ({ ...f, enabled: true }));
        toast('已启用全部功能', 'success');
      } else {
        toast('批量操作失败', 'error');
      }
    } catch {
      toast('批量操作失败', 'error');
    }
    saving = false;
  }

  async function disableAll() {
    saving = true;
    const batchFlags = flags.map((f) => ({ key: f.key, enabled: false }));
    try {
      const r = await fetch('/api/v1/admin/feature-flags/batch', {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${getToken()}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ flags: batchFlags }),
      });
      if (r.ok) {
        flags = flags.map((f) => ({ ...f, enabled: false }));
        toast('已禁用全部功能', 'success');
      } else {
        toast('批量操作失败', 'error');
      }
    } catch {
      toast('批量操作失败', 'error');
    }
    saving = false;
  }

  onMount(() => {
    if (isAdmin) loadFlags();
  });
</script>

{#if isAdmin}
<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-warning-light)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-warning)">toggle_on</span>
    </div>
    <div class="flex-1">
      <h2 class="text-base font-semibold text-[var(--color-text)]">功能开关</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">启用或禁用系统功能模块（{enabledCount}/{flags.length} 已启用）</p>
    </div>
    <button class="btn-ghost text-sm" onclick={loadFlags} disabled={loading}>
      <span class="material-symbols-outlined text-[16px] {loading ? 'animate-spin' : ''}">refresh</span>
      刷新
    </button>
  </div>

  {#if loading && flags.length === 0}
    <div class="space-y-3">
      {#each Array(6) as _}
        <div class="skeleton h-14 rounded-xl"></div>
      {/each}
    </div>
  {:else}
    <div class="flex items-center gap-3 mb-4 flex-wrap">
      <input
        type="text"
        class="text-sm px-3 py-1.5 rounded-lg border flex-1 min-w-[160px]"
        style="border-color: var(--color-border); background: var(--color-bg); color: var(--color-text)"
        placeholder="搜索功能..."
        bind:value={searchText}
      />
      <div class="flex gap-2 ml-auto">
        <button
          class="btn-ghost text-xs px-3 py-1.5"
          style="color: var(--color-success)"
          onclick={enableAll}
          disabled={saving}
        >
          <span class="material-symbols-outlined text-[14px]">done_all</span>
          全部启用
        </button>
        <button
          class="btn-ghost text-xs px-3 py-1.5"
          style="color: var(--color-error)"
          onclick={disableAll}
          disabled={saving}
        >
          <span class="material-symbols-outlined text-[14px]">block</span>
          全部禁用
        </button>
      </div>
    </div>

    <div class="space-y-2">
      {#each filteredFlags as flag (flag.key)}
        <div
          class="flex items-center gap-3 p-3 rounded-xl transition-colors"
          style="background: var(--color-surface); border: 1px solid var(--color-border)"
        >
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-[var(--color-text)] truncate">{flag.key}</span>
              <span
                class="text-[10px] px-1.5 py-0.5 rounded font-medium"
                style="background: {flag.enabled ? 'var(--color-success-light)' : 'var(--color-error-light)'}; color: {flag.enabled ? 'var(--color-success)' : 'var(--color-error)'}"
              >
                {flag.enabled ? '已启用' : '已禁用'}
              </span>
            </div>
            <p class="text-xs mt-0.5 truncate" style="color: var(--color-text-muted)">{flag.description}</p>
          </div>

          <button
            class="toggle-switch"
            class:active={flag.enabled}
            onclick={() => toggleFlag(flag.key, !flag.enabled)}
            aria-label={flag.enabled ? '禁用' : '启用'}
          >
            <span class="toggle-knob"></span>
          </button>
        </div>
      {/each}

      {#if filteredFlags.length === 0 && !loading}
        <p class="text-sm text-center py-6" style="color: var(--color-text-muted)">
          {searchText ? '没有匹配的功能开关' : '暂无功能开关'}
        </p>
      {/if}
    </div>
  {/if}
</section>
{/if}

<style>
  .toggle-switch {
    position: relative;
    width: 44px;
    height: 24px;
    border-radius: 12px;
    background: var(--color-border);
    border: none;
    cursor: pointer;
    transition: background 0.2s;
    flex-shrink: 0;
  }

  .toggle-switch.active {
    background: var(--color-success);
  }

  .toggle-knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: white;
    transition: transform 0.2s;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  }

  .toggle-switch.active .toggle-knob {
    transform: translateX(20px);
  }

  .toggle-switch:hover {
    opacity: 0.9;
  }

  .toggle-switch:active {
    transform: scale(0.95);
  }
</style>

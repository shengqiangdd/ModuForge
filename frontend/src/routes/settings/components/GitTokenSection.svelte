<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let hasToken = $state(false);
  let maskedToken = $state('');
  let rawToken = $state('');
  let inputToken = $state('');
  let saving = $state(false);
  let loading = $state(false);
  let showInput = $state(false);

  async function loadToken() {
    loading = true;
    try {
      const r = await fetch('/api/v1/auth/github-token', {
        headers: { Authorization: `Bearer ${getToken()}` }
      });
      if (r.ok) {
        const data = await r.json();
        hasToken = data.has_token;
        maskedToken = data.token || '';
        rawToken = data.raw_token || '';
      }
    } catch {}
    loading = false;
  }

  async function saveToken() {
    if (!inputToken.trim()) {
      toast('请输入 Token', 'error');
      return;
    }
    saving = true;
    try {
      const r = await fetch('/api/v1/auth/github-token', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${getToken()}`
        },
        body: JSON.stringify({ token: inputToken.trim() })
      });
      if (r.ok) {
        toast('GitHub Token 已保存', 'success');
        inputToken = '';
        showInput = false;
        await loadToken();
      } else {
        const d = await r.json();
        toast(d.error || '保存失败', 'error');
      }
    } catch { toast('保存失败', 'error'); }
    saving = false;
  }

  async function clearToken() {
    if (!confirm('确定删除已保存的 GitHub Token？')) return;
    saving = true;
    try {
      const r = await fetch('/api/v1/auth/github-token', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${getToken()}`
        },
        body: JSON.stringify({ token: '' })
      });
      if (r.ok) {
        toast('GitHub Token 已删除', 'success');
        hasToken = false;
        maskedToken = '';
        rawToken = '';
      }
    } catch { toast('删除失败', 'error'); }
    saving = false;
  }

  import { onMount } from 'svelte';
  onMount(() => { loadToken(); });
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-surface); border: 1px solid var(--color-border)">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" style="color: var(--color-text)">
        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
      </svg>
    </div>
    <div class="flex-1">
      <h2 class="text-base font-semibold text-[var(--color-text)]">GitHub Token</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">用于 Git 推送到 GitHub，所有项目共享此 Token</p>
    </div>
  </div>

  {#if loading}
    <div class="skeleton h-16 rounded-xl"></div>
  {:else if hasToken && !showInput}
    <div class="flex items-center gap-3 p-3 rounded-xl" style="background: var(--color-success-light); border: 1px solid var(--color-success)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-success)">check_circle</span>
      <div class="flex-1">
        <p class="text-sm font-medium text-[var(--color-text)]">已配置 Token</p>
        <p class="text-xs font-mono" style="color: var(--color-text-muted)">{maskedToken}</p>
      </div>
      <button class="text-xs px-3 py-1.5 rounded-lg font-medium" style="color: var(--color-primary); background: var(--color-bg); border: 1px solid var(--color-border)" onclick={() => showInput = true}>
        更换
      </button>
      <button class="text-xs px-3 py-1.5 rounded-lg font-medium" style="color: var(--color-error); background: var(--color-bg); border: 1px solid var(--color-border)" onclick={clearToken} disabled={saving}>
        删除
      </button>
    </div>
  {:else}
    <div class="space-y-3">
      <p class="text-xs" style="color: var(--color-text-muted)">
        配置后，所有项目的 Git Push 操作将自动使用此 Token 认证，无需每次手动输入。
      </p>
      <div class="flex gap-2">
        <input
          type="password"
          class="input-field flex-1"
          placeholder="ghp_xxxxxxxxxxxx 或 github_pat_xxxxxxxxxxxx"
          bind:value={inputToken}
          onkeydown={(e) => { if (e.key === 'Enter') saveToken(); }}
        />
        <button class="auth-submit px-5 py-2 rounded-xl font-semibold text-sm text-white disabled:opacity-50" onclick={saveToken} disabled={saving || !inputToken.trim()}>
          {saving ? '保存中...' : '保存'}
        </button>
      </div>
      <p class="text-[11px]" style="color: var(--color-text-muted)">
        前往
        <a href="https://github.com/settings/tokens" target="_blank" style="color: var(--color-primary)">GitHub Settings → Developer settings → Personal access tokens</a>
        生成 Token，需要 <code class="px-1 py-0.5 rounded text-[10px]" style="background: var(--color-surface); border: 1px solid var(--color-border)">repo</code> 权限。
      </p>
      {#if showInput && hasToken}
        <button class="text-xs" style="color: var(--color-text-muted)" onclick={() => { showInput = false; inputToken = ''; }}>取消</button>
      {/if}
    </div>
  {/if}
</section>

<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { client } from '../../../lib/api/client';
  import { toast } from '$lib/stores/toast.svelte';
  import './auth-styles.css';

  let {
    email = '',
    username = '',
    password = '',
    onVerified,
    onBack,
  }: {
    email?: string;
    username?: string;
    password?: string;
    onVerified: (token: string, user: { username: string; email: string }) => void;
    onBack: () => void;
  } = $props();

  let verifyCode = $state('');
  let loading = $state(false);
  let error = $state('');
  let cooldown = $state(0);
  let interval = $state<ReturnType<typeof setInterval> | null>(null);

  function startCountdown() {
    cooldown = 60;
    if (interval) clearInterval(interval);
    interval = setInterval(() => {
      cooldown--;
      if (cooldown <= 0 && interval) {
        clearInterval(interval);
        interval = null;
      }
    }, 1000);
  }

  onDestroy(() => {
    if (interval) clearInterval(interval);
  });

  onMount(() => {
    startCountdown();
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') handleSubmit();
  }

  async function handleSubmit() {
    if (verifyCode.length !== 6) return;
    loading = true;
    error = '';
    try {
      await client.post('/auth/verify-email', { token: verifyCode });
      toast('邮箱已验证成功', 'success');
      const res = await client.post<{ token: string; user: { username: string; email: string } }>('/auth/login', { username, password });
      onVerified(res.token, res.user);
      // Award beta_tester badge
      fetch('/api/v1/badges/my', { headers: { Authorization: `Bearer ${res.token}` } });
    } catch (e: any) {
      error = e.message || '验证失败';
    } finally {
      loading = false;
    }
  }

  async function resendVerify() {
    if (cooldown > 0) return;
    try {
      await client.post('/auth/resend-verification', { email });
      startCountdown();
      toast('验证码已重新发送', 'info');
    } catch {
      error = '发送失败';
    }
  }
</script>

<div class="space-y-5">
  <div class="text-center mb-4">
    <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-success-light)">
      <span class="material-symbols-outlined text-2xl text-green-500">mail</span>
    </div>
    <h3 class="text-base font-semibold" style="color: var(--color-text)">验证邮箱</h3>
    <p class="text-sm mt-1" style="color: var(--color-text-secondary)">验证码已发送至 {email || 'your email'}</p>
  </div>

  {#if error}
    <div class="mb-2 px-4 py-3 rounded-xl text-sm flex items-center gap-2.5" style="background: var(--color-error-light); border: 1px solid rgba(239,68,68,0.3); color: var(--color-error); animation: shake 0.4s ease-out">
      <span class="material-symbols-outlined text-[18px] flex-shrink-0">error</span>
      <span>{error}</span>
    </div>
  {/if}

  <div>
    <input
      type="text"
      inputmode="numeric"
      pattern="[0-9]*"
      maxlength="6"
      autocomplete="one-time-code"
      bind:value={verifyCode}
      placeholder="000000"
      class="input-field text-center text-2xl tracking-[0.5em] font-mono"
      onkeydown={handleKeydown}
    />
  </div>

  <button
    type="button"
    class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white transition-all duration-300 disabled:opacity-50 min-h-[52px]"
    onclick={handleSubmit}
    disabled={loading || verifyCode.length !== 6}
  >
    {#if loading}
      <span class="inline-flex items-center gap-2.5">
        <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
        验证中...
      </span>
    {:else}
      验证邮箱
    {/if}
  </button>

  <button
    type="button"
    class="w-full text-center text-sm transition-colors"
    style="color: var(--color-text-muted)"
    onclick={resendVerify}
    disabled={cooldown > 0}
  >
    {#if cooldown > 0}
      重新发送 ({cooldown}s)
    {:else}
      重新发送验证码
    {/if}
  </button>

  <button
    type="button"
    class="w-full text-center text-sm text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors"
    onclick={onBack}
  >
    返回登录
  </button>
</div>

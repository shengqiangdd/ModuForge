<script lang="ts">
  import { onDestroy } from 'svelte';
  import { client } from '../../../lib/api/client';
  import { toast } from '$lib/stores/toast.svelte';
  import './auth-styles.css';

  let {
    onBack,
  }: {
    onBack: () => void;
  } = $props();

  let step = $state<'email' | 'code' | 'done'>('email');
  let loading = $state(false);
  let error = $state('');
  let forgotEmail = $state('');
  let forgotCode = $state('');
  let forgotNewPassword = $state('');
  let forgotConfirmPassword = $state('');
  let cooldown = $state(0);
  let cooldownInterval = $state<ReturnType<typeof setInterval> | null>(null);

  function startCooldown() {
    cooldown = 60;
    if (cooldownInterval) clearInterval(cooldownInterval);
    cooldownInterval = setInterval(() => {
      cooldown--;
      if (cooldown <= 0 && cooldownInterval) {
        clearInterval(cooldownInterval);
        cooldownInterval = null;
      }
    }, 1000);
  }

  onDestroy(() => {
    if (cooldownInterval) clearInterval(cooldownInterval);
  });

  async function requestForgotCode() {
    if (!forgotEmail.trim()) { error = '请输入邮箱'; return; }
    loading = true;
    error = '';
    try {
      await client.post('/auth/forgot-password', { email: forgotEmail });
      step = 'code';
      startCooldown();
      toast('重置码已发送到邮箱', 'info');
    } catch {
      error = '发送失败';
    }
    loading = false;
  }

  async function submitForgotCode() {
    if (forgotCode.length !== 6) { error = '请输入验证码'; return; }
    if (!forgotNewPassword) { error = '请输入新密码'; return; }
    if (forgotNewPassword !== forgotConfirmPassword) { error = '两次密码不一致'; return; }
    if (forgotNewPassword.length < 6) { error = '密码至少6位'; return; }
    loading = true;
    error = '';
    try {
      await client.post('/auth/reset-password', { token: forgotCode, password: forgotNewPassword });
      step = 'done';
      toast('密码重置成功', 'success');
    } catch (e: any) {
      error = e.message || '重置失败';
    }
    loading = false;
  }
</script>

<div class="space-y-5">
  {#if step === 'email'}
    <div class="text-center mb-4">
      <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-warning-light)">
        <span class="material-symbols-outlined text-2xl text-amber-500">lock_reset</span>
      </div>
      <h3 class="text-base font-semibold" style="color: var(--color-text)">忘记密码</h3>
      <p class="text-sm mt-1" style="color: var(--color-text-secondary)">输入注册邮箱，我们将发送重置码</p>
    </div>

    {#if error}
      <div class="mb-2 px-4 py-3 rounded-xl text-sm flex items-center gap-2.5" style="background: var(--color-error-light); border: 1px solid rgba(239,68,68,0.3); color: var(--color-error); animation: shake 0.4s ease-out">
        <span class="material-symbols-outlined text-[18px] flex-shrink-0">error</span>
        <span>{error}</span>
      </div>
    {/if}

    <div>
      <label for="forgot-email" class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">邮箱</label>
      <input id="forgot-email" type="email" class="input-field" placeholder="you@example.com" bind:value={forgotEmail} />
    </div>
    <button
      type="button"
      class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white disabled:opacity-50 min-h-[52px]"
      onclick={requestForgotCode}
      disabled={loading || !forgotEmail}
    >
      {loading ? '发送中...' : '发送重置码'}
    </button>
    <button type="button" class="w-full text-center text-sm text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors" onclick={onBack}>返回登录</button>

  {:else if step === 'code'}
    <div class="text-center mb-4">
      <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-primary-light)">
        <span class="material-symbols-outlined text-2xl" style="color: var(--color-primary)">password</span>
      </div>
      <h3 class="text-base font-semibold" style="color: var(--color-text)">重置密码</h3>
      <p class="text-sm mt-1" style="color: var(--color-text-secondary)">输入验证码和新密码</p>
    </div>

    {#if error}
      <div class="mb-2 px-4 py-3 rounded-xl text-sm flex items-center gap-2.5" style="background: var(--color-error-light); border: 1px solid rgba(239,68,68,0.3); color: var(--color-error); animation: shake 0.4s ease-out">
        <span class="material-symbols-outlined text-[18px] flex-shrink-0">error</span>
        <span>{error}</span>
      </div>
    {/if}

    <div>
      <label for="forgot-code" class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">验证码</label>
      <input id="forgot-code" type="text" inputmode="numeric" pattern="[0-9]*" maxlength="6" class="input-field text-center text-xl tracking-[0.5em] font-mono" placeholder="000000" bind:value={forgotCode} />
    </div>
    <div>
      <label for="forgot-new-password" class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">新密码</label>
      <input id="forgot-new-password" type="password" class="input-field" placeholder="至少 6 位" bind:value={forgotNewPassword} />
    </div>
    <div>
      <label for="forgot-confirm-password" class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">确认密码</label>
      <input id="forgot-confirm-password" type="password" class="input-field" placeholder="再次输入新密码" bind:value={forgotConfirmPassword} />
    </div>
    <button
      type="button"
      class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white disabled:opacity-50 min-h-[52px]"
      onclick={submitForgotCode}
      disabled={loading || forgotCode.length !== 6}
    >
      {loading ? '重置中...' : '重置密码'}
    </button>
    <button
      type="button"
      class="w-full text-center text-sm text-[var(--color-text-muted)] transition-colors"
      disabled={loading}
      onclick={() => { if (cooldown <= 0) requestForgotCode(); }}
    >
      {#if cooldown > 0}
        重新发送 ({cooldown}s)
      {:else}
        重新发送验证码
      {/if}
    </button>
    <button type="button" class="w-full text-center text-sm text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors" onclick={onBack}>返回登录</button>

  {:else if step === 'done'}
    <div class="text-center py-4">
      <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-success-light)">
        <span class="material-symbols-outlined text-2xl text-green-500">check_circle</span>
      </div>
      <h3 class="text-base font-semibold" style="color: var(--color-text)">密码已重置</h3>
      <p class="text-sm mt-1 mb-4" style="color: var(--color-text-secondary)">请使用新密码登录</p>
      <button type="button" class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white min-h-[52px]" onclick={onBack}>返回登录</button>
    </div>
  {/if}
</div>

<script lang="ts">
import { onMount } from 'svelte';
import { t } from '$lib/i18n';
import { toast } from '$lib/stores/toast.svelte';
import { client } from '../../lib/api/client';

let { onAuth }: { onAuth: (token: string, action: 'login' | 'register', user?: {username: string; email: string}, rememberMe?: boolean) => void } = $props();

let isLogin = $state(true);
let username = $state('');
let email = $state('');
let password = $state('');
let showPassword = $state(false);
let loading = $state(false);
let error = $state('');
let mounted = $state(false);
let usernameTouched = $state(false);
let passwordTouched = $state(false);
let rememberMe = $state(false);

// 记住我：localStorage keys
const SAVED_USER_KEY = 'moduforge_saved_user';
const SAVED_PASS_KEY = 'moduforge_saved_pass';

onMount(() => {
  // 读取保存的用户名密码
  const savedUser = localStorage.getItem(SAVED_USER_KEY) || '';
  const savedPass = localStorage.getItem(SAVED_PASS_KEY) || '';
  if (savedUser) {
    username = savedUser;
    rememberMe = true;
  }
  if (savedPass) {
    try { password = atob(savedPass); } catch {}
  }
  mounted = true;
});

function saveCredentials() {
  if (rememberMe) {
    localStorage.setItem(SAVED_USER_KEY, username);
    localStorage.setItem(SAVED_PASS_KEY, btoa(password));
  } else {
    localStorage.removeItem(SAVED_USER_KEY);
    localStorage.removeItem(SAVED_PASS_KEY);
  }
}

function getPasswordStrength(pw: string): { score: number; label: string; color: string } {
  let score = 0;
  if (pw.length >= 8) score++;
  if (pw.length >= 12) score++;
  if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) score++;
  if (/\d/.test(pw)) score++;
  if (/[^a-zA-Z0-9]/.test(pw)) score++;
  if (score <= 1) return { score, label: '弱', color: 'var(--color-error)' };
  if (score <= 2) return { score, label: '中等', color: 'var(--color-warning)' };
  if (score <= 3) return { score, label: '强', color: 'var(--color-info)' };
  return { score, label: '很强', color: 'var(--color-success)' };
}
let passwordStrength = $derived(getPasswordStrength(password));

  // 2FA flow
  let show2FA = $state(false);
  let tempToken = $state('');
  let totpCode = $state('');

  // Email verification flow
  let showVerify = $state(false);
  let verifyCode = $state('');
  let verifyCooldown = $state(0);
  let verifyInterval = $state<ReturnType<typeof setInterval> | null>(null);
  let registeredUser = $state<{ username: string; email: string } | null>(null);

  // Forgot password flow
  let showForgot = $state(false);
  let forgotEmail = $state('');
  let forgotCode = $state('');
  let forgotNewPassword = $state('');
  let forgotConfirmPassword = $state('');
  let forgotStep = $state<'email' | 'code' | 'done'>('email');
  let forgotCooldown = $state(0);

  $effect(() => {
    setTimeout(() => mounted = true, 50);
  });

  function startVerifyCountdown() {
    verifyCooldown = 60;
    if (verifyInterval) clearInterval(verifyInterval);
    verifyInterval = setInterval(() => {
      verifyCooldown--;
      if (verifyCooldown <= 0) { if (verifyInterval) clearInterval(verifyInterval); verifyInterval = null; }
    }, 1000);
  }

  async function resendVerify() {
    if (verifyCooldown > 0) return;
    try {
      await client.post('/auth/resend-verification', { email: registeredUser?.email });
      startVerifyCountdown();
      toast('验证码已重新发送', 'info');
    } catch { error = '发送失败'; }
  }

  async function submitVerify() {
    if (verifyCode.length !== 6) return;
    loading = true;
    error = '';
    try {
      await client.post('/auth/verify-email', { token: verifyCode });
      toast('邮箱已验证成功', 'success');
      if (registeredUser) {
        const res = await client.post<{ token: string; user: {username: string; email: string} }>('/auth/login', { username: registeredUser.username, password });
        onAuth(res.token, 'register', res.user);
        // Award beta_tester badge
        const token = res.token;
        fetch('/api/v1/badges/my', { headers: { Authorization: `Bearer ${token}` } });
      }
    } catch (e: any) {
      error = e.message || '验证失败';
    } finally {
      loading = false;
    }
  }

  async function handleSubmit() {
    usernameTouched = true;
    passwordTouched = true;

    if (!username.trim()) {
      error = '请输入用户名';
      return;
    }
    if (!password) {
      error = '请输入密码';
      return;
    }

    loading = true;
    error = '';
    try {
      if (isLogin) {
        const res = await client.post<{ token: string; user: {username: string; email: string}; requires_2fa?: boolean; temp_token?: string }>('/auth/login', { username, password });
        if (res.requires_2fa) {
          show2FA = true;
          tempToken = res.temp_token || '';
          loading = false;
          return;
        }
        onAuth(res.token, 'login', res.user, rememberMe);
        saveCredentials();
      } else {
        await client.post('/auth/register', { username, email, password });
        registeredUser = { username, email };
        showVerify = true;
        startVerifyCountdown();
        loading = false;
        error = '';
      }
    } catch (e: any) {
      error = e.message || 'Authentication failed';
    } finally {
      if (!showVerify) loading = false;
    }
  }

  async function submit2FA() {
    if (totpCode.length !== 6) return;
    loading = true;
    error = '';
    try {
      const res = await client.post<{ token: string; user: {username: string; email: string} }>('/auth/login', { username, password, totp_code: totpCode });
      onAuth(res.token, 'login', res.user, rememberMe);
      saveCredentials();
    } catch (e: any) {
      error = e.message || '验证失败';
    } finally {
      loading = false;
    }
  }

  async function requestForgotCode() {
    if (!forgotEmail.trim()) { error = '请输入邮箱'; return; }
    loading = true; error = '';
    try {
      await client.post('/auth/forgot-password', { email: forgotEmail });
      forgotStep = 'code';
      forgotCooldown = 60;
      const interval = setInterval(() => {
        forgotCooldown--;
        if (forgotCooldown <= 0) clearInterval(interval);
      }, 1000);
      toast('重置码已发送到邮箱', 'info');
    } catch { error = '发送失败'; }
    loading = false;
  }

  async function submitForgotCode() {
    if (forgotCode.length !== 6) { error = '请输入验证码'; return; }
    if (!forgotNewPassword) { error = '请输入新密码'; return; }
    if (forgotNewPassword !== forgotConfirmPassword) { error = '两次密码不一致'; return; }
    if (forgotNewPassword.length < 6) { error = '密码至少6位'; return; }
    loading = true; error = '';
    try {
      await client.post('/auth/reset-password', { token: forgotCode, password: forgotNewPassword });
      forgotStep = 'done';
      toast('密码重置成功', 'success');
    } catch (e: any) { error = e.message || '重置失败'; }
    loading = false;
  }

  function switchTab(login: boolean) {
    if (isLogin === login) return;
    isLogin = login;
    error = '';
  }

  function switchToForgot() { showForgot = true; show2FA = false; error = ''; }
  function backToLogin() { showForgot = false; show2FA = false; showVerify = false; error = ''; }
</script>

<div class="auth-page min-h-screen flex items-center justify-center relative overflow-hidden" style="background: var(--color-bg)">
  <!-- Grid dot pattern background -->
  <div class="auth-grid absolute inset-0 pointer-events-none" style="opacity: 0.4"></div>

  <!-- Decorative gradient orbs -->
  <div class="absolute inset-0 overflow-hidden pointer-events-none">
    <div class="auth-orb auth-orb-1 absolute w-[500px] h-[500px] rounded-full blur-[140px]" style="background: color-mix(in srgb, var(--color-primary) 20%, transparent)"></div>
    <div class="auth-orb auth-orb-2 absolute w-[400px] h-[400px] rounded-full blur-[120px]" style="background: color-mix(in srgb, var(--color-info) 12%, transparent)"></div>
    <div class="auth-orb auth-orb-3 absolute w-[300px] h-[300px] rounded-full blur-[100px]" style="background: color-mix(in srgb, var(--color-primary) 6%, transparent)"></div>
  </div>

  <!-- Login Card -->
  <div class="relative w-full max-w-[420px] mx-4 transition-all duration-700 ease-out {mounted ? 'opacity-100 translate-y-0 scale-100' : 'opacity-0 translate-y-6 scale-95'}">
    <!-- Logo -->
    <div class="text-center mb-8">
      <div class="auth-logo w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-5 relative">
        <div class="absolute inset-0 rounded-2xl" style="background: var(--gradient-brand); animation: breatheGlow 3s ease-in-out infinite"></div>
        <span class="material-symbols-outlined text-white text-3xl relative z-10">extension</span>
      </div>
      <h1 class="text-2xl font-bold tracking-tight" style="color: var(--color-text)">ModuForge</h1>
      <p class="text-sm mt-1.5" style="color: var(--color-text-secondary)">{isLogin ? 'Welcome back' : 'Create your account'}</p>
    </div>

    <!-- Card with glassmorphism -->
    <div class="auth-card rounded-2xl p-8 border relative overflow-hidden" style="border-color: rgba(255,255,255,0.08); backdrop-filter: blur(24px); -webkit-backdrop-filter: blur(24px);">
      <!-- Tab switcher -->
      <div class="flex rounded-xl p-1 mb-6 relative" style="background: var(--color-surface)">
        <!-- Sliding indicator -->
        <div class="absolute top-1 bottom-1 rounded-lg transition-all duration-300 ease-out z-0" 
             style="background: var(--color-bg-elevated); box-shadow: var(--shadow-sm); width: calc(50% - 4px); transform: translateX({isLogin ? '4px' : 'calc(100% + 4px)'})">
        </div>
        <button
          class="flex-1 py-2.5 text-sm font-medium rounded-lg transition-all duration-300 min-h-[44px] relative z-10"
          style={isLogin ? 'color: var(--color-text)' : 'color: var(--color-text-muted)'}
          onclick={() => switchTab(true)}
          type="button"
        >
          {$t('nav.login')}
        </button>
        <button
          class="flex-1 py-2.5 text-sm font-medium rounded-lg transition-all duration-300 min-h-[44px] relative z-10"
          style={!isLogin ? 'color: var(--color-text)' : 'color: var(--color-text-muted)'}
          onclick={() => switchTab(false)}
          type="button"
        >
          {$t('nav.register')}
        </button>
      </div>

      <!-- Error -->
      {#if error}
        <div class="mb-5 px-4 py-3 rounded-xl text-sm flex items-center gap-2.5" style="background: var(--color-error-light); border: 1px solid rgba(239,68,68,0.3); color: var(--color-error); animation: shake 0.4s ease-out">
          <span class="material-symbols-outlined text-[18px] flex-shrink-0">error</span>
          <span>{error}</span>
        </div>
      {/if}

      {#if showVerify}
        <div class="space-y-5">
          <div class="text-center mb-4">
            <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-success-light)">
              <span class="material-symbols-outlined text-2xl text-green-500">mail</span>
            </div>
            <h3 class="text-base font-semibold" style="color: var(--color-text)">验证邮箱</h3>
            <p class="text-sm mt-1" style="color: var(--color-text-secondary)">验证码已发送至 {registeredUser?.email || 'your email'}</p>
          </div>
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
              onkeydown={(e) => { if (e.key === 'Enter') submitVerify(); }}
            />
          </div>
          <button
            type="button"
            class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white transition-all duration-300 disabled:opacity-50 min-h-[52px]"
            onclick={submitVerify}
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
          <button type="button" class="w-full text-center text-sm transition-colors" style="color: var(--color-text-muted)" onclick={resendVerify} disabled={verifyCooldown > 0}>
            {#if verifyCooldown > 0}
              重新发送 ({verifyCooldown}s)
            {:else}
              重新发送验证码
            {/if}
          </button>
          <button type="button" class="w-full text-center text-sm text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors" onclick={backToLogin}>
            返回登录
          </button>
        </div>
      {:else if show2FA}
        <div class="space-y-5">
          <div class="text-center mb-4">
            <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-primary-light)">
              <span class="material-symbols-outlined text-2xl" style="color: var(--color-primary)">security</span>
            </div>
            <h3 class="text-base font-semibold" style="color: var(--color-text)">两步验证</h3>
            <p class="text-sm mt-1" style="color: var(--color-text-secondary)">请输入 Authenticator App 中的 6 位验证码</p>
          </div>
          <div>
            <input
              type="text"
              inputmode="numeric"
              pattern="[0-9]*"
              maxlength="6"
              autocomplete="one-time-code"
              bind:value={totpCode}
              placeholder="000000"
              class="input-field text-center text-2xl tracking-[0.5em] font-mono"
              onkeydown={(e) => { if (e.key === 'Enter') submit2FA(); }}
            />
          </div>
          <button
            type="button"
            class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white transition-all duration-300 disabled:opacity-50 min-h-[52px]"
            onclick={submit2FA}
            disabled={loading || totpCode.length !== 6}
          >
            {#if loading}
              <span class="inline-flex items-center gap-2.5">
                <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                验证中...
              </span>
            {:else}
              验证
            {/if}
          </button>
          <button type="button" class="w-full text-center text-sm text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors" onclick={() => { show2FA = false; totpCode = ''; }}>
            返回登录
          </button>
        </div>
      {:else if showForgot}
        <div class="space-y-5">
          {#if forgotStep === 'email'}
            <div class="text-center mb-4">
              <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-warning-light)">
                <span class="material-symbols-outlined text-2xl text-amber-500">lock_reset</span>
              </div>
              <h3 class="text-base font-semibold" style="color: var(--color-text)">忘记密码</h3>
              <p class="text-sm mt-1" style="color: var(--color-text-secondary)">输入注册邮箱，我们将发送重置码</p>
            </div>
            <div>
              <label for="forgot-email" class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">邮箱</label>
              <input id="forgot-email" type="email" class="input-field" placeholder="you@example.com" bind:value={forgotEmail} />
            </div>
            <button type="button" class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white disabled:opacity-50 min-h-[52px]" onclick={requestForgotCode} disabled={loading || !forgotEmail}>
              {loading ? '发送中...' : '发送重置码'}
            </button>
            <button type="button" class="w-full text-center text-sm text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors" onclick={backToLogin}>返回登录</button>
          {:else if forgotStep === 'code'}
            <div class="text-center mb-4">
              <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-primary-light)">
                <span class="material-symbols-outlined text-2xl" style="color: var(--color-primary)">password</span>
              </div>
              <h3 class="text-base font-semibold" style="color: var(--color-text)">重置密码</h3>
              <p class="text-sm mt-1" style="color: var(--color-text-secondary)">输入验证码和新密码</p>
            </div>
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
            <button type="button" class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white disabled:opacity-50 min-h-[52px]" onclick={submitForgotCode} disabled={loading || forgotCode.length !== 6}>
              {loading ? '重置中...' : '重置密码'}
            </button>
            <button type="button" class="w-full text-center text-sm text-[var(--color-text-muted)] transition-colors" disabled={loading} onclick={() => { if (forgotCooldown <= 0) requestForgotCode(); }}>
              {#if forgotCooldown > 0}
                重新发送 ({forgotCooldown}s)
              {:else}
                重新发送验证码
              {/if}
            </button>
            <button type="button" class="w-full text-center text-sm text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors" onclick={backToLogin}>返回登录</button>
          {:else if forgotStep === 'done'}
            <div class="text-center py-4">
              <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-success-light)">
                <span class="material-symbols-outlined text-2xl text-green-500">check_circle</span>
              </div>
              <h3 class="text-base font-semibold" style="color: var(--color-text)">密码已重置</h3>
              <p class="text-sm mt-1 mb-4" style="color: var(--color-text-secondary)">请使用新密码登录</p>
              <button type="button" class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white min-h-[52px]" onclick={backToLogin}>返回登录</button>
            </div>
          {/if}
        </div>
      {:else}
      <!-- Form -->
      <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-5">
        <!-- Username -->
        <div>
          <label class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)" for="auth-username">{$t('auth.username')}</label>
          <div class="relative">
            <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-[18px] pointer-events-none transition-colors duration-200" style="color: var(--color-text-muted)">person</span>
            <div class="absolute left-[38px] top-2.5 bottom-2.5 w-px pointer-events-none" style="background: var(--color-border)"></div>
            <input
              id="auth-username"
              type="text"
              bind:value={username}
              class="auth-input"
              placeholder="your_username"
              required
              onfocus={() => {}}
              onblur={() => usernameTouched = true}
            />
          </div>
        </div>

        <!-- Email (register only) -->
        {#if !isLogin}
          <div style="animation: slideDown 0.2s ease-out">
            <label class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)" for="auth-email">{$t('auth.email')}</label>
            <div class="relative">
              <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-[18px] pointer-events-none" style="color: var(--color-text-muted)">mail</span>
              <div class="absolute left-[38px] top-2.5 bottom-2.5 w-px pointer-events-none" style="background: var(--color-border)"></div>
              <input
                id="auth-email"
                type="email"
                bind:value={email}
                class="auth-input"
                placeholder="you@example.com"
                required={!isLogin}
              />
          </div>
          {#if !isLogin && password.length > 0}
            <div class="flex items-center gap-2 mt-1">
              <div class="flex-1 h-1.5 rounded-full overflow-hidden" style="background: var(--color-surface)">
                <div class="h-full rounded-full transition-all duration-300" style="width: {passwordStrength.score * 20}%; background: {passwordStrength.color}"></div>
              </div>
              <span class="text-xs" style="color: {passwordStrength.color}">{passwordStrength.label}</span>
            </div>
          {/if}
        </div>
        {/if}

        <!-- Password with show/hide toggle -->
        <div>
          <div class="flex items-center justify-between mb-1.5">
            <label class="text-sm font-medium" style="color: var(--color-text-secondary)" for="auth-password">{$t('auth.password')}</label>
            {#if isLogin}
              <button type="button" class="text-xs font-medium transition-colors hover:text-[var(--color-primary)]" style="color: var(--color-text-muted)">
                忘记密码？
              </button>
            {/if}
          </div>
          <div class="relative">
            <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-[18px] pointer-events-none" style="color: var(--color-text-muted)">lock</span>
            <div class="absolute left-[38px] top-2.5 bottom-2.5 w-px pointer-events-none" style="background: var(--color-border)"></div>
            <input
              id="auth-password"
              type={showPassword ? 'text' : 'password'}
              bind:value={password}
              class="auth-input pr-12"
              placeholder="••••••••"
              required
              onblur={() => passwordTouched = true}
            />
            <button
              type="button"
              class="absolute right-3 top-1/2 -translate-y-1/2 p-1.5 rounded-lg transition-all duration-200 hover:bg-[var(--color-surface)]"
              style="color: var(--color-text-muted)"
              onclick={() => showPassword = !showPassword}
              aria-label={showPassword ? '隐藏密码' : '显示密码'}
            >
              <span class="material-symbols-outlined text-[20px]">{showPassword ? 'visibility_off' : 'visibility'}</span>
            </button>
          </div>
        </div>

        {#if isLogin}
          <div class="flex items-center gap-2">
            <input type="checkbox" id="remember-me" bind:checked={rememberMe} class="rounded" style="accent-color: var(--color-primary)" />
            <label for="remember-me" class="text-sm" style="color: var(--color-text-secondary)">记住我</label>
          </div>
        {/if}

        <!-- Submit Button -->
        <button
          type="submit"
          disabled={loading}
          class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed active:scale-[0.98] min-h-[52px] mt-6"
          style="box-shadow: var(--shadow-glow)"
        >
          {#if loading}
            <span class="inline-flex items-center gap-2.5">
              <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
              {isLogin ? 'Signing in...' : 'Creating account...'}
            </span>
          {:else}
            {isLogin ? $t('auth.login_btn') : $t('auth.register_btn')}
          {/if}
        </button>
        {#if isLogin}
          <button type="button" class="w-full text-center text-sm transition-colors" style="color: var(--color-text-muted)" onclick={switchToForgot}>忘记密码？</button>
        {/if}
      </form>

      <!-- Divider -->
      <div class="flex items-center gap-4 my-6">
        <div class="flex-1 h-px" style="background: var(--color-border)"></div>
        <span class="text-xs" style="color: var(--color-text-muted)">or</span>
        <div class="flex-1 h-px" style="background: var(--color-border)"></div>
      </div>

      <!-- Switch -->
      <p class="text-center text-sm" style="color: var(--color-text-muted)">
        {isLogin ? $t('auth.switch_to_register') : $t('auth.switch_to_login')}
      </p>
      {/if}
    </div>

    <!-- Footer -->
    <p class="text-center text-xs mt-6" style="color: var(--color-text-muted)">
      ModuForge v2.0 — Built for the Android modding community
    </p>
  </div>
</div>

<style>
  /* Auth card glassmorphism */
  .auth-card {
    background: color-mix(in srgb, var(--color-bg-elevated) 82%, color-mix(in srgb, var(--color-primary) 8%, transparent));
    box-shadow: 
      var(--shadow-xl),
      inset 0 1px 0 color-mix(in srgb, var(--color-primary) 10%, transparent);
  }

  /* Grid dot pattern */
  .auth-grid {
    background-image: radial-gradient(circle, color-mix(in srgb, var(--color-primary) 15%, transparent) 1px, transparent 1px);
    background-size: 24px 24px;
  }

  /* Floating orbs animation */
  .auth-orb-1 { top: -15%; right: -10%; animation: float1 20s ease-in-out infinite; }
  .auth-orb-2 { bottom: -15%; left: -10%; animation: float2 25s ease-in-out infinite; }
  .auth-orb-3 { top: 40%; left: 35%; animation: float3 18s ease-in-out infinite; }

  /* Auth input styling */
  .auth-input {
    width: 100%;
    padding: 12px 16px 12px 48px;
    border-radius: 12px;
    border: 1.5px solid var(--color-border);
    background: var(--color-surface);
    color: var(--color-text);
    font-size: 14px;
    transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    min-height: 48px;
  }
  .auth-input::placeholder { color: var(--color-text-muted); letter-spacing: 0.01em; }
  .auth-input:focus {
    outline: none;
    border-color: var(--color-primary);
    box-shadow: 0 0 0 3px var(--color-primary-light), 0 0 20px color-mix(in srgb, var(--color-primary) 10%, transparent);
    background: var(--color-bg-elevated);
  }
  .auth-input:hover:not(:focus) {
    border-color: var(--color-text-muted);
  }

  /* Submit button */
  .auth-submit {
    background: var(--gradient-brand);
    position: relative;
    overflow: hidden;
  }
  .auth-submit::before {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(135deg, rgba(255,255,255,0.15) 0%, transparent 50%);
    opacity: 0;
    transition: opacity 0.3s;
  }
  .auth-submit:hover::before { opacity: 1; }
  .auth-submit:hover {
    box-shadow: var(--shadow-glow), 0 4px 20px color-mix(in srgb, var(--color-primary) 30%, transparent);
    transform: translateY(-1px);
  }
  .auth-submit:active { transform: translateY(0) scale(0.98); }

  /* Breathing glow animation */
  @keyframes breatheGlow {
    0%, 100% { box-shadow: 0 0 20px color-mix(in srgb, var(--color-primary) 30%, transparent), 0 0 40px color-mix(in srgb, var(--color-primary) 10%, transparent); }
    50% { box-shadow: 0 0 30px color-mix(in srgb, var(--color-primary) 50%, transparent), 0 0 60px color-mix(in srgb, var(--color-primary) 20%, transparent), 0 0 80px color-mix(in srgb, var(--color-info) 10%, transparent); }
  }

  /* Floating animations */
  @keyframes float1 {
    0%, 100% { transform: translate(0, 0) scale(1); }
    33% { transform: translate(-30px, 30px) scale(1.05); }
    66% { transform: translate(20px, -20px) scale(0.95); }
  }
  @keyframes float2 {
    0%, 100% { transform: translate(0, 0) scale(1); }
    33% { transform: translate(40px, -20px) scale(1.1); }
    66% { transform: translate(-20px, 40px) scale(0.9); }
  }
  @keyframes float3 {
    0%, 100% { transform: translate(0, 0) scale(1); }
    50% { transform: translate(-25px, 25px) scale(1.05); }
  }

  /* Shake animation for errors */
  @keyframes shake {
    0%, 100% { transform: translateX(0); }
    20% { transform: translateX(-6px); }
    40% { transform: translateX(6px); }
    60% { transform: translateX(-4px); }
    80% { transform: translateX(4px); }
  }

  /* Slide down for email field */
  @keyframes slideDown {
    from { opacity: 0; transform: translateY(-8px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>

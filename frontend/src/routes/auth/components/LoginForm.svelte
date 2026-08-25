<script lang="ts">
  import { t } from '$lib/i18n';
  import { client } from '../../../lib/api/client';
  import './auth-styles.css';

  let {
    onAuth,
    onShow2FA,
    onShowVerify,
    onShowForgot,
  }: {
    onAuth: (token: string, action: 'login' | 'register', user?: { username: string; email: string }, rememberMe?: boolean) => void;
    onShow2FA: (username: string, password: string) => void;
    onShowVerify: (username: string, email: string, password: string) => void;
    onShowForgot: () => void;
  } = $props();

  let isLogin = $state(true);
  let username = $state('');
  let email = $state('');
  let password = $state('');
  let showPassword = $state(false);
  let loading = $state(false);
  let error = $state('');
  let usernameTouched = $state(false);
  let passwordTouched = $state(false);
  let rememberMe = $state(false);

  // 记住我：localStorage keys
  const SAVED_USER_KEY = 'moduforge_saved_user';
  const SAVED_PASS_KEY = 'moduforge_saved_pass';

  import { onMount } from 'svelte';

  onMount(() => {
    const savedUser = localStorage.getItem(SAVED_USER_KEY) || '';
    const savedPass = localStorage.getItem(SAVED_PASS_KEY) || '';
    if (savedUser) {
      username = savedUser;
      rememberMe = true;
    }
    if (savedPass) {
      try { password = atob(savedPass); } catch (e) { console.warn('Failed to decode saved password:', e); }
    }
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

  function switchTab(login: boolean) {
    if (isLogin === login) return;
    isLogin = login;
    error = '';
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
        const res = await client.post<{ token: string; user: { username: string; email: string }; requires_2fa?: boolean; temp_token?: string }>('/auth/login', { username, password });
        if (res.requires_2fa) {
          onShow2FA(username, password);
          loading = false;
          return;
        }
        onAuth(res.token, 'login', res.user, rememberMe);
        saveCredentials();
      } else {
        await client.post('/auth/register', { username, email, password });
        onShowVerify(username, email, password);
        loading = false;
        error = '';
      }
    } catch (e: any) {
      error = e.message || 'Authentication failed';
    } finally {
      loading = false;
    }
  }
</script>

<!-- Tab switcher -->
<div class="flex rounded-xl p-1 mb-6 relative" style="background: var(--color-surface)">
  <div
    class="absolute top-1 bottom-1 rounded-lg transition-all duration-300 ease-out z-0"
    style="background: var(--color-bg-elevated); box-shadow: var(--shadow-sm); width: calc(50% - 4px); transform: translateX({isLogin ? '4px' : 'calc(100% + 4px)'})"
  ></div>
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
      {#if password.length > 0}
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
        <button type="button" class="text-xs font-medium transition-colors hover:text-[var(--color-primary)]" style="color: var(--color-text-muted)" onclick={onShowForgot}>
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
    <button type="button" class="w-full text-center text-sm transition-colors" style="color: var(--color-text-muted)" onclick={onShowForgot}>忘记密码？</button>
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

<style>
  @keyframes shake {
    0%, 100% { transform: translateX(0); }
    20% { transform: translateX(-6px); }
    40% { transform: translateX(6px); }
    60% { transform: translateX(-4px); }
    80% { transform: translateX(4px); }
  }

  @keyframes slideDown {
    from { opacity: 0; transform: translateY(-8px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>

<script lang="ts">
  import { t } from '$lib/i18n';
  import { client } from '../../lib/api/client';

  let { onAuth }: { onAuth: (token: string, action: 'login' | 'register') => void } = $props();

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

  $effect(() => {
    setTimeout(() => mounted = true, 50);
  });

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
        const res = await client.post<{ token: string }>('/auth/login', { username, password });
        onAuth(res.token, 'login');
      } else {
        await client.post('/auth/register', { username, email, password });
        const res = await client.post<{ token: string }>('/auth/login', { username, password });
        onAuth(res.token, 'register');
      }
    } catch (e: any) {
      error = e.message || 'Authentication failed';
    } finally {
      loading = false;
    }
  }

  function switchTab(login: boolean) {
    if (isLogin === login) return;
    isLogin = login;
    error = '';
  }
</script>

<div class="auth-page min-h-screen flex items-center justify-center relative overflow-hidden" style="background: var(--color-bg)">
  <!-- Grid dot pattern background -->
  <div class="auth-grid absolute inset-0 pointer-events-none" style="opacity: 0.4"></div>

  <!-- Decorative gradient orbs -->
  <div class="absolute inset-0 overflow-hidden pointer-events-none">
    <div class="auth-orb auth-orb-1 absolute w-[500px] h-[500px] rounded-full blur-[140px]" style="background: rgba(139,92,246,0.2)"></div>
    <div class="auth-orb auth-orb-2 absolute w-[400px] h-[400px] rounded-full blur-[120px]" style="background: rgba(6,182,212,0.12)"></div>
    <div class="auth-orb auth-orb-3 absolute w-[300px] h-[300px] rounded-full blur-[100px]" style="background: rgba(139,92,246,0.06)"></div>
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
    background: color-mix(in srgb, var(--color-bg-elevated) 82%, rgba(139,92,246,0.08));
    box-shadow: 
      var(--shadow-xl),
      inset 0 1px 0 rgba(139,92,246,0.1);
  }
  .light .auth-card {
    background: color-mix(in srgb, var(--color-bg-elevated) 90%, transparent);
    box-shadow: 
      var(--shadow-xl),
      inset 0 1px 0 rgba(255,255,255,0.8);
  }

  /* Grid dot pattern */
  .auth-grid {
    background-image: radial-gradient(circle, rgba(139,92,246,0.15) 1px, transparent 1px);
    background-size: 24px 24px;
  }
  .light .auth-grid { opacity: 0.3 !important; }

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
    box-shadow: 0 0 0 3px var(--color-primary-light), 0 0 20px rgba(139,92,246,0.1);
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
    box-shadow: var(--shadow-glow), 0 4px 20px rgba(139,92,246,0.3);
    transform: translateY(-1px);
  }
  .auth-submit:active { transform: translateY(0) scale(0.98); }

  /* Breathing glow animation */
  @keyframes breatheGlow {
    0%, 100% { box-shadow: 0 0 20px rgba(139,92,246,0.3), 0 0 40px rgba(139,92,246,0.1); }
    50% { box-shadow: 0 0 30px rgba(139,92,246,0.5), 0 0 60px rgba(139,92,246,0.2), 0 0 80px rgba(6,182,212,0.1); }
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

<script lang="ts">
  import { onMount } from 'svelte';
  import AuthHeader from './components/AuthHeader.svelte';
  import LoginForm from './components/LoginForm.svelte';
  import TwoFactorForm from './components/TwoFactorForm.svelte';
  import EmailVerifyForm from './components/EmailVerifyForm.svelte';
  import ForgotPasswordForm from './components/ForgotPasswordForm.svelte';
  import { client } from '../../lib/api/client';

  let { onAuth }: { onAuth: (token: string, action: 'login' | 'register', user?: { username: string; email: string }, rememberMe?: boolean) => void } = $props();

  let mounted = $state(false);

  // Flow state
  type AuthFlow = 'login' | '2fa' | 'verify' | 'forgot';
  let flow = $state<AuthFlow>('login');
  let loading = $state(false);

  // 2FA context
  let twoFAUsername = $state('');
  let twoFAPassword = $state('');

  // Verify context
  let verifyUsername = $state('');
  let verifyEmail = $state('');
  let verifyPassword = $state('');

  $effect(() => {
    setTimeout(() => { mounted = true; }, 50);
  });

  // 2FA handlers
  function handleShow2FA(username: string, password: string) {
    twoFAUsername = username;
    twoFAPassword = password;
    flow = '2fa';
  }

  async function handle2FAVerify(code: string) {
    loading = true;
    try {
      const res = await client.post<{ token: string; user: { username: string; email: string } }>('/auth/login', {
        username: twoFAUsername,
        password: twoFAPassword,
        totp_code: code,
      });
      onAuth(res.token, 'login', res.user);
      flow = 'login';
    } finally {
      loading = false;
    }
  }

  // Verify handlers
  function handleShowVerify(username: string, email: string, password: string) {
    verifyUsername = username;
    verifyEmail = email;
    verifyPassword = password;
    flow = 'verify';
  }

  function handleVerified(token: string, user: { username: string; email: string }) {
    onAuth(token, 'register', user);
    flow = 'login';
  }

  // Forgot password handler
  function handleShowForgot() {
    flow = 'forgot';
  }

  // Back to login from any sub-flow
  function handleBackToLogin() {
    flow = 'login';
    loading = false;
  }
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
    <!-- Header -->
    <AuthHeader isLogin={flow === 'login' || flow === '2fa'} />

    <!-- Card with glassmorphism -->
    <div class="auth-card rounded-2xl p-8 border relative overflow-hidden" style="border-color: rgba(255,255,255,0.08); backdrop-filter: blur(24px); -webkit-backdrop-filter: blur(24px);">
      {#if flow === 'login'}
        <LoginForm
          {onAuth}
          onShow2FA={handleShow2FA}
          onShowVerify={handleShowVerify}
          onShowForgot={handleShowForgot}
        />
      {:else if flow === '2fa'}
        <TwoFactorForm
          {loading}
          onVerify={handle2FAVerify}
          onBack={handleBackToLogin}
        />
      {:else if flow === 'verify'}
        <EmailVerifyForm
          email={verifyEmail}
          username={verifyUsername}
          password={verifyPassword}
          onVerified={handleVerified}
          onBack={handleBackToLogin}
        />
      {:else if flow === 'forgot'}
        <ForgotPasswordForm onBack={handleBackToLogin} />
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
</style>

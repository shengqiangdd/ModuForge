<script lang="ts">
  import { t } from '$lib/i18n';
  import { client } from '../../lib/api/client';

  let { onAuth }: { onAuth: (token: string) => void } = $props();

  let isLogin = $state(true);
  let username = $state('');
  let email = $state('');
  let password = $state('');
  let loading = $state(false);
  let error = $state('');
  let mounted = $state(false);

  $effect(() => {
    setTimeout(() => mounted = true, 50);
  });

  async function handleSubmit() {
    loading = true;
    error = '';
    try {
      if (isLogin) {
        const res = await client.post<{ token: string }>('/auth/login', { username, password });
        onAuth(res.token);
      } else {
        await client.post('/auth/register', { username, email, password });
        const res = await client.post<{ token: string }>('/auth/login', { username, password });
        onAuth(res.token);
      }
    } catch (e: any) {
      error = e.message || 'Authentication failed';
    } finally {
      loading = false;
    }
  }
</script>

<div class="min-h-screen flex items-center justify-center bg-gradient-mesh relative overflow-hidden">
  <!-- Decorative elements -->
  <div class="absolute inset-0 overflow-hidden pointer-events-none">
    <div class="absolute -top-40 -right-40 w-80 h-80 bg-primary-500/10 rounded-full blur-3xl"></div>
    <div class="absolute -bottom-40 -left-40 w-80 h-80 bg-primary-300/10 rounded-full blur-3xl"></div>
    <div class="absolute top-1/3 left-1/4 w-64 h-64 bg-primary-200/5 rounded-full blur-3xl"></div>
  </div>

  <!-- Login Card -->
  <div class="relative w-full max-w-md mx-4 transition-all duration-500 ease-out {mounted ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-4'}">
    <!-- Logo -->
    <div class="text-center mb-8">
      <div class="w-14 h-14 rounded-2xl bg-gradient-to-br from-primary-500 to-primary-700 flex items-center justify-center mx-auto mb-4 shadow-glow">
        <span class="material-symbols-outlined text-white text-2xl">extension</span>
      </div>
      <h1 class="text-2xl font-bold text-[var(--color-text)] tracking-tight">ModuForge</h1>
      <p class="text-sm text-[var(--color-text-secondary)] mt-1">Magisk Module Builder & Marketplace</p>
    </div>

    <!-- Card -->
    <div class="bg-[var(--color-bg-elevated)] rounded-2xl shadow-elevated-lg p-8 border border-[var(--color-border)]">
      <!-- Tab switcher -->
      <div class="flex bg-[var(--color-surface)] rounded-xl p-1 mb-6">
        <button
          class="flex-1 py-2.5 text-sm font-medium rounded-lg transition-all duration-200 {isLogin ? 'bg-[var(--color-bg-elevated)] shadow-sm text-[var(--color-text)]' : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text)]'}"
          onclick={() => { isLogin = true; error = ''; }}
        >
          {$t('nav.login')}
        </button>
        <button
          class="flex-1 py-2.5 text-sm font-medium rounded-lg transition-all duration-200 {!isLogin ? 'bg-[var(--color-bg-elevated)] shadow-sm text-[var(--color-text)]' : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text)]'}"
          onclick={() => { isLogin = false; error = ''; }}
        >
          {$t('nav.register')}
        </button>
      </div>

      <!-- Error -->
      {#if error}
        <div class="mb-4 px-4 py-3 rounded-xl bg-red-50 border border-red-200 text-red-700 text-sm flex items-center gap-2">
          <span class="material-symbols-outlined text-[18px]">error</span>
          {error}
        </div>
      {/if}

      <!-- Form -->
      <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">{$t('auth.username')}</label>
          <div class="relative">
            <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-neutral-400 text-[18px]">person</span>
            <input
              type="text"
              bind:value={username}
              class="input-field pl-10"
              placeholder="your_username"
              required
            />
          </div>
        </div>

        {#if !isLogin}
          <div>
            <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">{$t('auth.email')}</label>
            <div class="relative">
              <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-neutral-400 text-[18px]">mail</span>
              <input
                type="email"
                bind:value={email}
                class="input-field pl-10"
                placeholder="you@example.com"
                required={!isLogin}
              />
            </div>
          </div>
        {/if}

        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1.5">{$t('auth.password')}</label>
          <div class="relative">
            <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-neutral-400 text-[18px]">lock</span>
            <input
              type="password"
              bind:value={password}
              class="input-field pl-10"
              placeholder="••••••••"
              required
            />
          </div>
        </div>

        <button
          type="submit"
          disabled={loading}
          class="w-full py-3 rounded-xl font-semibold text-sm text-white transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed
            bg-gradient-to-r from-primary-600 to-primary-700 hover:from-primary-700 hover:to-primary-800 active:scale-[0.98] shadow-sm hover:shadow-glow"
        >
          {#if loading}
            <span class="inline-flex items-center gap-2">
              <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
              {isLogin ? $t('nav.login') : $t('nav.register')}...
            </span>
          {:else}
            {isLogin ? $t('auth.login_btn') : $t('auth.register_btn')}
          {/if}
        </button>
      </form>

      <p class="text-center text-sm text-[var(--color-text-muted)] mt-6">
        {isLogin ? $t('auth.switch_to_register') : $t('auth.switch_to_login')}
      </p>
    </div>

    <!-- Footer -->
    <p class="text-center text-xs text-[var(--color-text-muted)] mt-6">
      ModuForge v1.0 — Built for the Android modding community
    </p>
  </div>
</div>

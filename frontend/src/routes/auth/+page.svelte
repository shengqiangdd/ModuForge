<script lang="ts">
  let mode = $state<'login' | 'register'>('login');
  let username = $state('');
  let email = $state('');
  let password = $state('');
  let loading = $state(false);
  let error = $state('');

  const onAuth = $props<{ onAuth: (token: string) => void }>();

  async function submit() {
    error = '';
    loading = true;
    try {
      const body = mode === 'login'
        ? { username, password }
        : { username, email, password };

      const res = await fetch(`/api/v1/auth/${mode}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });

      const data = await res.json();
      if (!res.ok) {
        error = data.error || 'Request failed';
        return;
      }

      localStorage.setItem('moduforge_token', data.token);
      onAuth(data.token);
    } catch {
      error = 'Network error, please try again';
    } finally {
      loading = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') submit();
  }
</script>

<div class="min-h-screen flex items-center justify-center bg-surface-container">
  <div class="w-full max-w-md bg-surface rounded-2xl shadow-xl p-8">
    <div class="text-center mb-8">
      <h1 class="text-3xl font-bold text-on-surface">ModuForge</h1>
      <p class="text-on-surface-variant mt-1">Magisk/KSU 模块开发平台</p>
    </div>

    <!-- Mode toggle -->
    <div class="flex bg-surface-container rounded-lg p-1 mb-6">
      <button
        class="flex-1 py-2 rounded-md text-body-medium transition-colors"
        class:bg-primary={mode === 'login'}
        class:text-on-primary={mode === 'login'}
        class:text-on-surface-variant={mode !== 'login'}
        onclick={() => { mode = 'login'; error = ''; }}
      >
        登录
      </button>
      <button
        class="flex-1 py-2 rounded-md text-body-medium transition-colors"
        class:bg-primary={mode === 'register'}
        class:text-on-primary={mode === 'register'}
        class:text-on-surface-variant={mode !== 'register'}
        onclick={() => { mode = 'register'; error = ''; }}
      >
        注册
      </button>
    </div>

    {#if error}
      <div class="mb-4 p-3 bg-error-container text-on-error-container rounded-lg text-body-small flex items-center gap-2">
        <span class="material-symbols-outlined text-sm">error</span>
        {error}
      </div>
    {/if}

    <div class="space-y-4">
      <div>
        <label class="block text-label-medium text-on-surface-variant mb-1" for="username">用户名</label>
        <input
          id="username"
          type="text"
          class="w-full px-4 py-3 border border-outline rounded-lg bg-surface text-on-surface focus:border-primary focus:outline-none transition-colors"
          placeholder="输入用户名"
          bind:value={username}
          onkeydown={handleKeydown}
          disabled={loading}
        />
      </div>

      {#if mode === 'register'}
        <div>
          <label class="block text-label-medium text-on-surface-variant mb-1" for="email">邮箱</label>
          <input
            id="email"
            type="email"
            class="w-full px-4 py-3 border border-outline rounded-lg bg-surface text-on-surface focus:border-primary focus:outline-none transition-colors"
            placeholder="输入邮箱"
            bind:value={email}
            onkeydown={handleKeydown}
            disabled={loading}
          />
        </div>
      {/if}

      <div>
        <label class="block text-label-medium text-on-surface-variant mb-1" for="password">密码</label>
        <input
          id="password"
          type="password"
          class="w-full px-4 py-3 border border-outline rounded-lg bg-surface text-on-surface focus:border-primary focus:outline-none transition-colors"
          placeholder="输入密码"
          bind:value={password}
          onkeydown={handleKeydown}
          disabled={loading}
        />
      </div>

      <md-filled-button
        class="w-full"
        onclick={submit}
        disabled={loading || !username || !password || (mode === 'register' && !email)}
      >
        {#if loading}
          <md-circular-progress indeterminate class="w-5 h-5 mr-2"></md-circular-progress>
        {/if}
        {loading ? '处理中...' : mode === 'login' ? '登录' : '注册'}
      </md-filled-button>
    </div>
  </div>
</div>

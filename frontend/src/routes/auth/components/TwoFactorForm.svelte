<script lang="ts">
  import { t } from '$lib/i18n';
  import './auth-styles.css';

  let {
    loading = false,
    onVerify,
    onBack,
  }: {
    loading?: boolean;
    onVerify: (code: string) => Promise<void>;
    onBack: () => void;
  } = $props();

  let totpCode = $state('');
  let error = $state('');

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') handleSubmit();
  }

  async function handleSubmit() {
    if (totpCode.length === 6) {
      error = '';
      try {
        await onVerify(totpCode);
      } catch (e: any) {
        error = e.message || '验证失败';
      }
    }
  }
</script>

<div class="space-y-5">
  <div class="text-center mb-4">
    <div class="w-14 h-14 rounded-2xl flex items-center justify-center mx-auto mb-3" style="background: var(--color-primary-light)">
      <span class="material-symbols-outlined text-2xl" style="color: var(--color-primary)">security</span>
    </div>
    <h3 class="text-base font-semibold" style="color: var(--color-text)">两步验证</h3>
    <p class="text-sm mt-1" style="color: var(--color-text-secondary)">请输入 Authenticator App 中的 6 位验证码</p>
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
      bind:value={totpCode}
      placeholder="000000"
      class="input-field text-center text-2xl tracking-[0.5em] font-mono"
      onkeydown={handleKeydown}
    />
  </div>
  <button
    type="button"
    class="auth-submit w-full py-3.5 rounded-xl font-semibold text-sm text-white transition-all duration-300 disabled:opacity-50 min-h-[52px]"
    onclick={handleSubmit}
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
  <button
    type="button"
    class="w-full text-center text-sm text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors"
    onclick={() => { totpCode = ''; error = ''; onBack(); }}
  >
    返回登录
  </button>
</div>

<style>
  @keyframes shake {
    0%, 100% { transform: translateX(0); }
    20% { transform: translateX(-6px); }
    40% { transform: translateX(6px); }
    60% { transform: translateX(-4px); }
    80% { transform: translateX(4px); }
  }
</style>

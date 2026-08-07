<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  // 2FA state
  let totpQrUrl = $state('');
  let totpSecret = $state('');
  let totpCode = $state('');
  let totpEnabled = $state(false);
  let totpLoading = $state(false);
  let showTotpQr = $state(false);

  async function setup2FA() {
    totpLoading = true;
    try {
      const res = await fetch('/api/v1/auth/2fa/setup', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${getToken()}` },
      });
      if (res.ok) {
        const data = await res.json();
        totpSecret = data.secret;
        totpQrUrl = data.qr_url;
        showTotpQr = true;
        totpCode = '';
      } else {
        toast((await res.json()).error || '设置失败', 'error');
      }
    } catch { toast('设置失败', 'error'); }
    totpLoading = false;
  }

  async function enable2FA() {
    if (!totpCode) return;
    totpLoading = true;
    try {
      const res = await fetch('/api/v1/auth/2fa/enable', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${getToken()}` },
        body: JSON.stringify({ code: totpCode }),
      });
      if (res.ok) {
        totpEnabled = true;
        showTotpQr = false;
        toast('两步验证已启用', 'success');
      } else {
        toast((await res.json()).error || '验证失败', 'error');
      }
    } catch { toast('启用失败', 'error'); }
    totpLoading = false;
  }

  async function disable2FA() {
    if (!totpCode) return;
    totpLoading = true;
    try {
      const res = await fetch('/api/v1/auth/2fa/disable', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${getToken()}` },
        body: JSON.stringify({ code: totpCode }),
      });
      if (res.ok) {
        totpEnabled = false;
        totpCode = '';
        toast('两步验证已禁用', 'success');
      } else {
        toast((await res.json()).error || '验证失败', 'error');
      }
    } catch { toast('禁用失败', 'error'); }
    totpLoading = false;
  }
</script>

<div class="settings-section">
  <h2 class="section-title">安全设置</h2>

  <div class="security-card">
    <div class="security-header">
      <div class="security-icon">
        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
        </svg>
      </div>
      <div class="security-info">
        <h3>两步验证 (2FA)</h3>
        <p class="security-status" class:enabled={totpEnabled}>
          {totpEnabled ? '已启用' : '未启用'}
        </p>
      </div>
    </div>

    {#if !totpEnabled}
      {#if !showTotpQr}
        <button class="btn-primary" onclick={setup2FA} disabled={totpLoading}>
          {totpLoading ? '设置中...' : '启用两步验证'}
        </button>
      {:else}
        <div class="totp-setup">
          <p class="totp-instruction">使用认证器应用扫描此二维码：</p>
          {#if totpQrUrl}
            <img src={totpQrUrl} alt="TOTP QR Code" class="totp-qr" />
          {/if}
          <p class="totp-secret">或手动输入密钥：<code>{totpSecret}</code></p>
          <div class="totp-input-group">
            <input
              type="text"
              bind:value={totpCode}
              placeholder="输入6位验证码"
              maxlength="6"
              class="totp-input"
            />
            <button class="btn-primary" onclick={enable2FA} disabled={totpLoading || totpCode.length !== 6}>
              {totpLoading ? '验证中...' : '确认启用'}
            </button>
          </div>
        </div>
      {/if}
    {:else}
      <div class="totp-disable">
        <p>输入验证码以禁用两步验证：</p>
        <div class="totp-input-group">
          <input
            type="text"
            bind:value={totpCode}
            placeholder="输入6位验证码"
            maxlength="6"
            class="totp-input"
          />
          <button class="btn-danger" onclick={disable2FA} disabled={totpLoading || totpCode.length !== 6}>
            {totpLoading ? '处理中...' : '禁用'}
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .settings-section {
    margin-bottom: 2rem;
  }

  .section-title {
    font-size: 1.25rem;
    font-weight: 600;
    margin-bottom: 1rem;
    color: var(--color-text);
  }

  .security-card {
    padding: 1.5rem;
    background: var(--color-bg-secondary);
    border-radius: 0.75rem;
    border: 1px solid var(--color-border);
  }

  .security-header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1rem;
  }

  .security-icon {
    width: 48px;
    height: 48px;
    border-radius: 0.5rem;
    background: var(--color-primary-light);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-primary);
  }

  .security-info h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
  }

  .security-status {
    margin: 0.25rem 0 0;
    font-size: 0.875rem;
    color: var(--color-text-secondary);
  }

  .security-status.enabled {
    color: var(--color-success);
  }

  .totp-setup {
    margin-top: 1rem;
    padding: 1rem;
    background: var(--color-bg);
    border-radius: 0.5rem;
    border: 1px solid var(--color-border);
  }

  .totp-instruction {
    margin: 0 0 1rem;
    font-size: 0.875rem;
    color: var(--color-text-secondary);
  }

  .totp-qr {
    display: block;
    margin: 0 auto 1rem;
    max-width: 200px;
    border-radius: 0.5rem;
  }

  .totp-secret {
    margin: 0 0 1rem;
    font-size: 0.875rem;
    color: var(--color-text-secondary);
    text-align: center;
  }

  .totp-secret code {
    background: var(--color-bg-secondary);
    padding: 0.25rem 0.5rem;
    border-radius: 0.25rem;
    font-family: monospace;
  }

  .totp-input-group {
    display: flex;
    gap: 0.5rem;
  }

  .totp-input {
    flex: 1;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--color-border);
    border-radius: 0.375rem;
    background: var(--color-bg);
    color: var(--color-text);
    font-size: 1rem;
    text-align: center;
    letter-spacing: 0.5em;
  }

  .totp-input:focus {
    outline: none;
    border-color: var(--color-primary);
  }

  .totp-disable {
    margin-top: 1rem;
  }

  .totp-disable p {
    margin: 0 0 0.5rem;
    font-size: 0.875rem;
    color: var(--color-text-secondary);
  }

  .btn-primary {
    padding: 0.5rem 1rem;
    background: var(--color-primary);
    color: white;
    border: none;
    border-radius: 0.375rem;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.2s;
  }

  .btn-primary:hover:not(:disabled) {
    background: var(--color-primary-hover);
  }

  .btn-primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-danger {
    padding: 0.5rem 1rem;
    background: var(--color-danger);
    color: white;
    border: none;
    border-radius: 0.375rem;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.2s;
  }

  .btn-danger:hover:not(:disabled) {
    background: var(--color-danger-hover);
  }

  .btn-danger:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>

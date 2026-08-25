<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let smtpHost = $state('');
  let smtpPort = $state('587');
  let smtpUser = $state('');
  let smtpPass = $state('');
  let smtpFromName = $state('ModuForge');
  let smtpFrom = $state('');
  let smtpUseTLS = $state(1);
  let smtpIsActive = $state(0);
  let savingEmail = $state(false);
  let testingEmail = $state(false);
  let testingConnection = $state(false);

  async function loadEmailConfig() {
    const token = getToken();
    try {
      const r = await fetch('/api/v1/admin/email-config', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        smtpHost = data.smtp_host || '';
        smtpPort = String(data.smtp_port || '587');
        smtpUser = data.smtp_user || '';
        smtpPass = '';
        smtpFromName = data.from_name || 'ModuForge';
        smtpFrom = data.from_email || '';
        smtpUseTLS = data.use_tls ?? 1;
        smtpIsActive = data.is_active ?? 0;
      }
    } catch (e) { console.error('Failed to load email config:', e); }
  }

  async function saveEmailConfig() {
    savingEmail = true;
    try {
      const r = await fetch('/api/v1/admin/email-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({
          smtp_host: smtpHost,
          smtp_port: parseInt(smtpPort) || 587,
          smtp_user: smtpUser,
          smtp_password: smtpPass,
          from_name: smtpFromName,
          from_email: smtpFrom,
          use_tls: smtpUseTLS,
          is_active: smtpIsActive
        }),
      });
      if (r.ok) toast('邮箱配置已保存', 'success');
      else toast((await r.json()).error || '保存失败', 'error');
    } catch { toast('保存失败', 'error'); }
    savingEmail = false;
  }

  async function sendTestEmail() {
    testingEmail = true;
    try {
      const r = await fetch('/api/v1/admin/email-config/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({ to: smtpFrom })
      });
      if (r.ok) toast('测试邮件已发送', 'success');
      else toast((await r.json()).error || '发送失败', 'error');
    } catch { toast('发送失败', 'error'); }
    testingEmail = false;
  }

  async function testConnection() {
    testingConnection = true;
    try {
      const r = await fetch('/api/v1/admin/email-config/test-connection', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({})
      });
      const data = await r.json();
      if (data.ok) toast('连接测试成功', 'success');
      else toast(data.message || '连接测试失败', 'error');
    } catch { toast('连接测试失败', 'error'); }
    testingConnection = false;
  }

  onMount(() => {
    loadEmailConfig();
  });
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-warning-light)">
      <span class="material-symbols-outlined text-[18px] text-amber-500">mail</span>
    </div>
    <div>
      <h2 class="text-base font-semibold text-[var(--color-text)]">邮件服务配置</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">配置 SMTP 发送验证码和通知邮件</p>
    </div>
  </div>
  <div class="space-y-4">
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <label for="smtp-host" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">SMTP 主机</label>
        <input id="smtp-host" type="text" class="input-field" placeholder="smtp.example.com" bind:value={smtpHost} />
      </div>
      <div>
        <label for="smtp-port" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">端口</label>
        <input id="smtp-port" type="number" class="input-field" placeholder="587" bind:value={smtpPort} />
      </div>
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <label for="smtp-user" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">用户名</label>
        <input id="smtp-user" type="text" class="input-field" placeholder="user@example.com" bind:value={smtpUser} />
      </div>
      <div>
        <label for="smtp-pass" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">密码</label>
        <input id="smtp-pass" type="password" class="input-field" placeholder="SMTP 密码" bind:value={smtpPass} />
      </div>
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <label for="smtp-from-name" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">发件人名称</label>
        <input id="smtp-from-name" type="text" class="input-field" placeholder="ModuForge" bind:value={smtpFromName} />
      </div>
      <div>
        <label for="smtp-from" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">发件人地址</label>
        <input id="smtp-from" type="email" class="input-field" placeholder="noreply@example.com" bind:value={smtpFrom} />
      </div>
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <label for="smtp-use-tls" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">加密方式</label>
        <select id="smtp-use-tls" class="input-field" bind:value={smtpUseTLS}>
          <option value={0}>无加密（不推荐）</option>
          <option value={1}>STARTTLS（端口 587）</option>
          <option value={2}>SSL/TLS（端口 465）</option>
        </select>
      </div>
      <div>
        <label for="smtp-is-active" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">启用状态</label>
        <select id="smtp-is-active" class="input-field" bind:value={smtpIsActive}>
          <option value={0}>禁用</option>
          <option value={1}>启用</option>
        </select>
      </div>
    </div>
    <div class="flex gap-3 flex-wrap">
      <button type="button" class="auth-submit px-5 py-2 rounded-xl font-semibold text-sm text-white disabled:opacity-50" onclick={saveEmailConfig} disabled={savingEmail}>
        {savingEmail ? '保存中...' : '保存配置'}
      </button>
      <button type="button" class="px-5 py-2 rounded-xl font-semibold text-sm" style="background: var(--color-surface-secondary); color: var(--color-text); border: 1px solid var(--color-border)" onclick={testConnection} disabled={testingConnection}>
        {testingConnection ? '测试中...' : '🔌 测试连接'}
      </button>
      <button type="button" class="px-5 py-2 rounded-xl font-semibold text-sm" style="background: var(--color-surface-secondary); color: var(--color-text); border: 1px solid var(--color-border)" onclick={sendTestEmail} disabled={testingEmail}>
        {testingEmail ? '发送中...' : '📧 发送测试邮件'}
      </button>
    </div>
  </div>
</section>

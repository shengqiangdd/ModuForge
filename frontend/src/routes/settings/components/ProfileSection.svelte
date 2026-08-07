<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let { onProfileUpdate, username = '', onProfileLoaded }: { onProfileUpdate?: () => void; username?: string; onProfileLoaded?: (data: { isAdmin: boolean }) => void } = $props();

  // Profile state
  let displayName = $state('');
  let bio = $state('');
  let location = $state('');
  let website = $state('');
  let avatarUrl = $state('');
  let savingProfile = $state(false);
  let isAdmin = $state(false);
  let avatarInput: HTMLInputElement | undefined = $state();

  // Change password state
  let showChangePassword = $state(false);
  let oldPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let changingPassword = $state(false);

  async function loadProfile() {
    const token = getToken();
    try {
      const r = await fetch('/api/v1/auth/profile', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        displayName = data.display_name || '';
        bio = data.bio || '';
        location = data.location || '';
        website = data.website || '';
        avatarUrl = data.avatar_url || '';
        isAdmin = data.is_admin || false;
        onProfileLoaded?.({ isAdmin: data.is_admin || false });
      }
    } catch {}
  }

  async function saveProfile() {
    savingProfile = true;
    try {
      const r = await fetch('/api/v1/auth/profile', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({ display_name: displayName, bio, location, website }),
      });
      if (r.ok) {
        toast('资料已保存', 'success');
        onProfileUpdate?.();
      } else {
        toast((await r.json()).error || '保存失败', 'error');
      }
    } catch { toast('保存失败', 'error'); }
    savingProfile = false;
  }

  async function changePassword() {
    if (newPassword !== confirmPassword) { toast('两次输入的密码不一致', 'error'); return; }
    if (newPassword.length < 8) { toast('新密码至少 8 位', 'error'); return; }
    changingPassword = true;
    try {
      const r = await fetch('/api/v1/auth/change-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
      });
      if (r.ok) {
        toast('密码已修改', 'success');
        showChangePassword = false;
        oldPassword = '';
        newPassword = '';
        confirmPassword = '';
      } else {
        const d = await r.json().catch(() => ({}));
        toast(d.error || '修改失败', 'error');
      }
    } catch { toast('修改失败', 'error'); }
    changingPassword = false;
  }

  async function uploadAvatar() {
    const file = avatarInput?.files?.[0];
    if (!file) return;
    if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(file.type)) {
      toast('不支持的图片格式', 'error'); return;
    }
    if (file.size > 2 * 1024 * 1024) { toast('图片不能超过 2MB', 'error'); return; }
    const fd = new FormData();
    fd.append('avatar', file);
    try {
      const r = await fetch('/api/v1/auth/avatar', {
        method: 'POST',
        headers: { Authorization: `Bearer ${getToken()}` },
        body: fd
      });
      if (r.ok) {
        const data = await r.json();
        avatarUrl = data.avatar_url || '';
        toast('头像已更新', 'success');
      } else toast((await r.json()).error || '上传失败', 'error');
    } catch { toast('上传失败', 'error'); }
    avatarInput!.value = '';
  }

  // Load profile on mount
  import { onMount } from 'svelte';
  onMount(() => {
    loadProfile();
  });
</script>

<div class="settings-section">
  <h2 class="section-title">个人资料</h2>

  <div class="profile-card">
    <!-- Avatar -->
    <div class="avatar-section">
      <div class="avatar-wrapper">
        {#if avatarUrl}
          <img src={avatarUrl} alt="Avatar" class="avatar-img" />
        {:else}
          <div class="avatar-placeholder">
            <span class="avatar-initial">{displayName?.[0] || username?.[0] || '?'}</span>
          </div>
        {/if}
        <label class="avatar-upload-btn" for="avatar-input">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
            <circle cx="12" cy="13" r="4"/>
          </svg>
        </label>
        <input bind:this={avatarInput} type="file" id="avatar-input" accept="image/*" class="hidden" onchange={uploadAvatar} />
      </div>
    </div>

    <!-- Profile Form -->
    <div class="profile-form">
      <div class="form-group">
        <label for="display-name">显示名称</label>
        <input id="display-name" type="text" bind:value={displayName} placeholder="显示名称" />
      </div>

      <div class="form-group">
        <label for="bio">简介</label>
        <textarea id="bio" bind:value={bio} placeholder="个人简介" rows="3"></textarea>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label for="location">位置</label>
          <input id="location" type="text" bind:value={location} placeholder="位置" />
        </div>
        <div class="form-group">
          <label for="website">网站</label>
          <input id="website" type="url" bind:value={website} placeholder="https://" />
        </div>
      </div>

      <div class="form-actions">
        <button class="btn-primary" onclick={saveProfile} disabled={savingProfile}>
          {savingProfile ? '保存中...' : '保存资料'}
        </button>
      </div>
    </div>
  </div>

  <!-- Change Password -->
  <div class="password-section">
    <button class="btn-secondary" onclick={() => showChangePassword = !showChangePassword}>
      {showChangePassword ? '取消' : '修改密码'}
    </button>

    {#if showChangePassword}
      <div class="password-form">
        <div class="form-group">
          <label for="old-password">当前密码</label>
          <input id="old-password" type="password" bind:value={oldPassword} placeholder="当前密码" />
        </div>
        <div class="form-group">
          <label for="new-password">新密码</label>
          <input id="new-password" type="password" bind:value={newPassword} placeholder="新密码（至少8位）" />
        </div>
        <div class="form-group">
          <label for="confirm-password">确认密码</label>
          <input id="confirm-password" type="password" bind:value={confirmPassword} placeholder="确认新密码" />
        </div>
        <div class="form-actions">
          <button class="btn-primary" onclick={changePassword} disabled={changingPassword}>
            {changingPassword ? '修改中...' : '确认修改'}
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

  .profile-card {
    display: flex;
    gap: 2rem;
    padding: 1.5rem;
    background: var(--color-bg-secondary);
    border-radius: 0.75rem;
    border: 1px solid var(--color-border);
  }

  .avatar-section {
    flex-shrink: 0;
  }

  .avatar-wrapper {
    position: relative;
    width: 96px;
    height: 96px;
  }

  .avatar-img {
    width: 96px;
    height: 96px;
    border-radius: 50%;
    object-fit: cover;
  }

  .avatar-placeholder {
    width: 96px;
    height: 96px;
    border-radius: 50%;
    background: var(--color-primary);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .avatar-initial {
    font-size: 2rem;
    font-weight: 600;
    color: white;
  }

  .avatar-upload-btn {
    position: absolute;
    bottom: 0;
    right: 0;
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: var(--color-bg);
    border: 2px solid var(--color-border);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: background 0.2s;
  }

  .avatar-upload-btn:hover {
    background: var(--color-bg-hover);
  }

  .hidden {
    display: none;
  }

  .profile-form {
    flex: 1;
  }

  .form-group {
    margin-bottom: 1rem;
  }

  .form-group label {
    display: block;
    font-size: 0.875rem;
    font-weight: 500;
    margin-bottom: 0.25rem;
    color: var(--color-text-secondary);
  }

  .form-group input,
  .form-group textarea {
    width: 100%;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--color-border);
    border-radius: 0.375rem;
    background: var(--color-bg);
    color: var(--color-text);
    font-size: 0.875rem;
  }

  .form-group input:focus,
  .form-group textarea:focus {
    outline: none;
    border-color: var(--color-primary);
    box-shadow: 0 0 0 2px var(--color-primary-light);
  }

  .form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }

  .form-actions {
    margin-top: 1rem;
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

  .btn-secondary {
    padding: 0.5rem 1rem;
    background: var(--color-bg);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: 0.375rem;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.2s;
  }

  .btn-secondary:hover {
    background: var(--color-bg-hover);
  }

  .password-section {
    margin-top: 1.5rem;
    padding: 1.5rem;
    background: var(--color-bg-secondary);
    border-radius: 0.75rem;
    border: 1px solid var(--color-border);
  }

  .password-form {
    margin-top: 1rem;
  }

  @media (max-width: 640px) {
    .profile-card {
      flex-direction: column;
      align-items: center;
    }

    .form-row {
      grid-template-columns: 1fr;
    }
  }
</style>

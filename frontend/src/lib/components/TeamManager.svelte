<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';

  let { projectId = '' }: { projectId?: string } = $props();

  interface TeamMember {
    id: number; project_id: string; user_id: string;
    role: string; invited_by: string; created_at: string;
  }

  interface AuditLog {
    id: number; project_id: string; user_id: string;
    action: string; details: string; created_at: string;
  }

  let members = $state<TeamMember[]>([]);
  let auditLogs = $state<AuditLog[]>([]);
  let loadingMembers = $state(false);
  let loadingAudit = $state(false);
  let inviteUser = $state('');
  let inviteRole = $state('member');
  let showAudit = $state(false);
  let removing = $state<string | null>(null);

  const roleOptions = [
    { value: 'owner', label: 'Owner', desc: '完全控制' },
    { value: 'admin', label: 'Admin', desc: '管理成员和设置' },
    { value: 'member', label: 'Member', desc: '编辑文件' },
    { value: 'viewer', label: 'Viewer', desc: '只读访问' },
  ];

  const roleColors: Record<string, string> = {
    owner: 'background: rgba(239,68,68,0.15); color: #ef4444',
    admin: 'background: rgba(245,158,11,0.15); color: #f59e0b',
    member: 'background: rgba(34,197,94,0.15); color: #22c55e',
    viewer: 'background: rgba(161,161,170,0.15); color: #a1a1aa',
  };

  async function loadMembers() {
    if (!projectId) return;
    loadingMembers = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}/members`, {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) { const d = await res.json(); members = d.members || []; }
    } catch { members = []; }
    loadingMembers = false;
  }

  async function loadAuditLogs() {
    if (!projectId) return;
    loadingAudit = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}/audit-logs`, {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) { const d = await res.json(); auditLogs = d.logs || []; }
    } catch { auditLogs = []; }
    loadingAudit = false;
  }

  async function inviteMember() {
    if (!inviteUser.trim()) return;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${projectId}/members`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ user_id: inviteUser.trim(), role: inviteRole }),
      });
      if (res.ok) {
        inviteUser = '';
        toast('成员已添加', 'success');
        loadMembers();
      } else {
        const d = await res.json();
        toast(d.error || '添加失败', 'error');
      }
    } catch {
      toast('请求失败', 'error');
    }
  }

  async function removeMember(userId: string) {
    removing = userId;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      await fetch(`/api/v1/projects/${projectId}/members/${userId}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      toast('成员已移除', 'success');
      loadMembers();
    } catch {
      toast('移除失败', 'error');
    }
    removing = null;
  }

  async function updateRole(userId: string, role: string) {
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      await fetch(`/api/v1/projects/${projectId}/members/${userId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ role }),
      });
      loadMembers();
    } catch {
      toast('修改角色失败', 'error');
    }
  }

  function toggleAudit() {
    showAudit = !showAudit;
    if (showAudit) loadAuditLogs();
  }

  const roleLabel: Record<string, string> = {
    owner: 'Owner', admin: 'Admin', member: 'Member', viewer: 'Viewer',
  };

  onMount(() => { loadMembers(); });
</script>

<div class="space-y-6">
  <!-- Invite Form -->
  <div class="flex items-end gap-3">
    <div class="flex-1">
      <label class="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">添加成员（用户ID）</label>
      <input type="text" class="input-field" placeholder="输入用户 ID..." bind:value={inviteUser} />
    </div>
    <div>
      <label class="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">角色</label>
      <select class="input-field" bind:value={inviteRole}>
        {#each roleOptions as opt}
          <option value={opt.value}>{opt.label}</option>
        {/each}
      </select>
    </div>
    <button class="btn-primary text-sm" onclick={inviteMember} disabled={!inviteUser.trim()}>邀请</button>
  </div>

  <!-- Members List -->
  <div>
    <h4 class="text-sm font-semibold text-[var(--color-text)] mb-3">团队成员</h4>
    {#if loadingMembers}
      <div class="flex justify-center py-8">
        <div class="animate-spin h-5 w-5 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
      </div>
    {:else if members.length === 0}
      <p class="text-sm text-[var(--color-text-muted)] text-center py-8">暂无成员</p>
    {:else}
      <div class="space-y-2">
        {#each members as m}
          <div class="flex items-center justify-between p-3 rounded-xl" style="background: var(--color-surface)">
            <div class="flex items-center gap-3">
              <div class="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold text-white" style="background: var(--gradient-brand)">{m.user_id.slice(0, 2).toUpperCase()}</div>
              <div>
                <p class="text-sm font-medium text-[var(--color-text)]">{m.user_id}</p>
                <p class="text-[11px] text-[var(--color-text-muted)]">加入于 {m.created_at?.slice(0, 10)}</p>
<style>
  @media (max-width: 640px) {
    .team-table { display: none !important; }
    .team-cards { display: flex !important; }
  }
  @media (min-width: 641px) {
    .team-cards { display: none !important; }
  }
</style>
</div>
            </div>
            <div class="flex items-center gap-2">
              <select
                class="text-xs px-2 py-1 rounded-lg border"
                style="background: var(--color-bg); border-color: var(--color-border); color: var(--color-text)"
                value={m.role}
                onchange={(e) => updateRole(m.user_id, (e.target as HTMLSelectElement).value)}
              >
                {#each roleOptions as opt}
                  <option value={opt.value} disabled={opt.value === 'owner'}>{opt.label}</option>
                {/each}
              </select>
              <button
                class="p-1.5 rounded-lg transition-colors hover:bg-[var(--color-error-light)]"
                style="color: var(--color-text-muted)"
                disabled={removing === m.user_id}
                onclick={() => removeMember(m.user_id)}
              >
                <span class="material-symbols-outlined text-[16px]" style="color: var(--color-error)">remove_circle</span>
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Audit Log Toggle -->
  <div class="pt-2">
    <button
      class="flex items-center gap-2 text-sm font-medium transition-colors"
      style="color: var(--color-text-secondary)"
      onclick={toggleAudit}
    >
      <span class="material-symbols-outlined text-[16px]">{showAudit ? 'expand_less' : 'expand_more'}</span>
      审计日志
    </button>
  </div>

  {#if showAudit}
    <div>
      {#if loadingAudit}
        <div class="flex justify-center py-4">
          <div class="animate-spin h-4 w-4 rounded-full" style="border: 2px solid var(--color-primary); border-top-color: transparent"></div>
        </div>
      {:else if auditLogs.length === 0}
        <p class="text-sm text-[var(--color-text-muted)] text-center py-4">暂无操作记录</p>
      {:else}
        <div class="overflow-x-auto rounded-xl border" style="border-color: var(--color-border)">
          <table class="w-full text-xs">
            <thead>
              <tr style="background: var(--color-surface); border-bottom: 1px solid var(--color-border)">
                <th class="px-3 py-2 text-left font-medium" style="color: var(--color-text-secondary)">时间</th>
                <th class="px-3 py-2 text-left font-medium" style="color: var(--color-text-secondary)">用户</th>
                <th class="px-3 py-2 text-left font-medium" style="color: var(--color-text-secondary)">操作</th>
                <th class="px-3 py-2 text-left font-medium" style="color: var(--color-text-secondary)">详情</th>
              </tr>
            </thead>
            <tbody>
              {#each auditLogs as log}
                <tr style="border-bottom: 1px solid var(--color-border)">
                  <td class="px-3 py-2" style="color: var(--color-text-muted)">{log.created_at?.slice(0, 19).replace('T', ' ')}</td>
                  <td class="px-3 py-2" style="color: var(--color-text)">{log.user_id?.slice(0, 8)}</td>
                  <td class="px-3 py-2">
                    <span class="px-1.5 py-0.5 rounded" style="background: var(--color-primary-light); color: var(--color-primary)">{log.action}</span>
                  </td>
                  <td class="px-3 py-2" style="color: var(--color-text-secondary)">{log.details}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}
</div>

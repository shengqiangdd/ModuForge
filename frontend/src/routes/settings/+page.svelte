<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getTheme, setTheme } from '$lib/stores/theme';
import { loadShortcuts, saveShortcuts, resetShortcuts, defaultShortcuts, type Shortcut } from '$lib/stores/shortcuts';
import { getToken } from '$lib/api/client';

  // ===== Profile =====
  let username = $state('');
  let email = $state('');

  // ===== Agent Settings =====
  let agentMaxIterations = $state(50);
  let agentMaxResultLen = $state(32768);
  let savingAgentConfig = $state(false);

  // ===== Current Provider =====
  let currentProvider = $state('opencode-zen');
  let currentModelId = $state('');

  // ===== Preset Providers =====
  let presetProviders = $state<any[]>([]);
  let providerConfigs = $state<Record<string, {endpoint: string, api_key: string, models_json?: string}>>({});
  let configModalProvider = $state<any>(null);
  let configEndpoint = $state('');
  let configApiKey = $state('');
  let showAllProvidersModal = $state(false);

  // ===== Models Modal =====
  let showModelsModal = $state(false);
  let modelsModalProvider = $state<any>(null);

  // Featured provider IDs (shown by default, rest go behind "more")
  const FEATURED_IDS = ['opencode-zen', 'opencode-go', 'openai', 'anthropic', 'google', 'deepseek'];

  // Compute featured providers outside the template to avoid Svelte 5 closure bugs with .filter() in {#each}
  let featuredProviders: any[] = $derived(presetProviders.filter((p: any) => FEATURED_IDS.includes(p.id)));

  // ===== Custom Providers =====
  let customProviders = $state<any[]>([]);
  let showCustomModal = $state(false);
  let editingCustom = $state<any>(null);
  let customForm = $state({ name: '', endpoint: '', api_key: '', models: [] as {id: string; name: string; max_tokens: number}[] });
  let deletingCustomId = $state('');

  // ===== Add Model to Preset Provider =====
  let addingModelProviderId = $state('');
  let newModelId = $state('');
  let newModelName = $state('');
  let savingModel = $state(false);
  let userModelsMap = $state<Record<string, Array<{id: string; name: string}>>>({});
  let removingModelKey = $state('');

  // ===== 2FA =====
  let totpQrUrl = $state('');
  let totpSecret = $state('');
  let totpCode = $state('');
  let totpEnabled = $state(false);
  let totpLoading = $state(false);
  let showTotpQr = $state(false);

  // ===== Favorites =====
  let favoriteItems = $state<{ id: number; item_type: string; item_id: number; created_at: string }[]>([]);
  let favFilter = $state('');

  // ===== Search History =====
  let searchHistory = $state<{ id: number; query: string; result_count: number; searched_at: string }[]>([]);

  async function loadFavorites() {
    const token = getToken();
    try {
      const params = favFilter ? `?type=${favFilter}` : '';
      const r = await fetch(`/api/v1/favorites${params}`, { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) { const d = await r.json(); favoriteItems = d.favorites || []; }
    } catch { favoriteItems = []; }
  }

  async function removeFavorite(f: { id: number; item_type: string; item_id: number }) {
    const token = getToken();
    try {
      await fetch(`/api/v1/favorites/${f.item_type}/${f.item_id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      favoriteItems = favoriteItems.filter(i => i.id !== f.id);
    } catch {}
  }

  async function loadSearchHistory() {
    const token = getToken();
    try {
      const r = await fetch('/api/v1/search/history', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) { const d = await r.json(); searchHistory = d.history || []; }
    } catch { searchHistory = []; }
  }

  async function deleteSearchHistoryItem(id: number) {
    const token = getToken();
    try {
      await fetch(`/api/v1/search/history/${id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      searchHistory = searchHistory.filter(h => h.id !== id);
    } catch {}
  }

  async function clearSearchHistory() {
    const token = getToken();
    try {
      await fetch('/api/v1/search/history', { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      searchHistory = [];
    } catch {}
  }

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

  let avatarInput: HTMLInputElement | undefined = $state();
  let displayName = $state('');
  let bio = $state('');
  let location = $state('');
  let website = $state('');
  let avatarUrl = $state('');
  let savingProfile = $state(false);
  let isAdmin = $state(false);

  // Change password
  let showChangePassword = $state(false);
  let oldPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let changingPassword = $state(false);

  let smtpHost = $state('');
  let smtpPort = $state('587');
  let smtpUser = $state('');
  let smtpPass = $state('');
  let smtpFromName = $state('ModuForge');
  let smtpFrom = $state('');
  let smtpUseTLS = $state(1); // 0=无, 1=STARTTLS, 2=SSL/TLS
  let smtpIsActive = $state(0);
  let savingEmail = $state(false);
  let testingEmail = $state(false);
  let testingConnection = $state(false);

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
      if (r.ok) toast('资料已保存', 'success');
      else toast((await r.json()).error || '保存失败', 'error');
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
    if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(file.type)) { toast('不支持的图片格式', 'error'); return; }
    if (file.size > 2 * 1024 * 1024) { toast('图片不能超过 2MB', 'error'); return; }
    const fd = new FormData();
    fd.append('avatar', file);
    try {
      const r = await fetch('/api/v1/auth/avatar', { method: 'POST', headers: { Authorization: `Bearer ${getToken()}` }, body: fd });
      if (r.ok) {
        const data = await r.json();
        avatarUrl = data.avatar_url || '';
        toast('头像已更新', 'success');
      } else toast((await r.json()).error || '上传失败', 'error');
    } catch { toast('上传失败', 'error'); }
    avatarInput!.value = '';
  }

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
    } catch {}
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

  async function disable2FA() {
    if (!totpCode) { toast('请输入验证码', 'error'); return; }
    totpLoading = true;
    try {
      const res = await fetch('/api/v1/auth/2fa/disable', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${getToken()}` },
        body: JSON.stringify({ code: totpCode }),
      });
      if (res.ok) {
        totpEnabled = false;
        totpSecret = '';
        totpQrUrl = '';
        totpCode = '';
        toast('两步验证已禁用', 'success');
      } else {
        toast((await res.json()).error || '验证失败', 'error');
      }
    } catch { toast('禁用失败', 'error'); }
    totpLoading = false;
  }

  // ===== Custom Skills =====
  let customSkills = $state<any[]>([]);
  let builtinSkills = $state<any[]>([]);
  let allSkills = $derived([...builtinSkills, ...customSkills]);
  let showAllSkills = $state(false);
  let expandedSkillId = $state<number | null>(null);
  let showSkillModal = $state(false);
  let editingSkill = $state<any>(null);
  let skillForm = $state({ name: '', description: '', prompt: '', input_schema: '{}', is_public: false });
  let loadingSkills = $state(false);
  let testingSkillId = $state<number | null>(null);
  let testSkillId = $state<number | null>(null);
  let testInput = $state('');
  let testResult = $state('');
  let showTestInput = $state(false);

  // Skill evolution (6)
  let skillEvolutionData = $state<Record<number, any>>({});
  let loadingEvolution = $state<Set<number>>(new Set());
  let showEvolution = $state<number | null>(null);

  async function loadCustomSkills() {
    loadingSkills = true;
    try {
      // Load built-in skills
      const resBuiltin = await fetch('/api/v1/agent/skills', {
        headers: { 'Authorization': `Bearer ${getToken()}` },
      });
      if (resBuiltin.ok) {
        const data = await resBuiltin.json();
        builtinSkills = (data.skills || []).map((s: any) => ({ ...s, id: `builtin_${s.name}`, is_builtin: true }));
      }
      // Load custom skills
      const res = await fetch('/api/v1/agent/custom-skills', {
        headers: { 'Authorization': `Bearer ${getToken()}` },
      });
      if (res.ok) {
        const data = await res.json();
        customSkills = data.skills || [];
      }
    } catch {}
    loadingSkills = false;
  }

  function openNewSkill() {
    editingSkill = null;
    skillForm = { name: '', description: '', prompt: '', input_schema: '{}', is_public: false };
    testInput = '';
    testResult = '';
    showSkillModal = true;
  }

  function openEditSkill(s: any) {
    editingSkill = s;
    skillForm = { name: s.name, description: s.description, prompt: s.prompt, input_schema: s.input_schema || '{}', is_public: s.is_public };
    testInput = '';
    testResult = '';
    showSkillModal = true;
  }

  async function saveSkill() {
    const token = getToken();
    const isEdit = !!editingSkill;
    const url = isEdit ? `/api/v1/agent/custom-skills/${editingSkill.id}` : '/api/v1/agent/custom-skills';
    const method = isEdit ? 'PUT' : 'POST';
    try {
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify(skillForm),
      });
      if (res.ok) {
        toast(isEdit ? '技能已更新' : '技能已创建', 'success');
        showSkillModal = false;
        await loadCustomSkills();
      } else {
        toast((await res.json()).error || '保存失败', 'error');
      }
    } catch { toast('保存失败', 'error'); }
  }

  async function deleteSkill(id: number) {
    if (!confirm('确定删除此技能？')) return;
    try {
      const res = await fetch(`/api/v1/agent/custom-skills/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      if (res.ok) {
        customSkills = customSkills.filter((s: any) => s.id !== id);
        toast('已删除', 'success');
      } else {
        toast((await res.json()).error || '删除失败', 'error');
      }
    } catch { toast('删除失败', 'error'); }
  }

  async function testSkill(id: number) {
    if (!testInput) return;
    testingSkillId = id;
    testResult = '';
    try {
      const res = await fetch(`/api/v1/agent/custom-skills/${id}/execute`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({ input: testInput }),
      });
      if (res.ok) {
        testResult = await res.text();
      } else {
        testResult = (await res.json()).error || '测试失败';
      }
    } catch (e: any) { testResult = e.message || '测试失败'; }
    testingSkillId = null;
  }

  // ─── Skill Evolution (6) ───
  async function loadSkillEvolution(skillId: number) {
    if (loadingEvolution.has(skillId)) return;
    loadingEvolution = new Set(loadingEvolution).add(skillId);
    try {
      const [evoRes, optRes] = await Promise.all([
        fetch(`/api/v1/agent/custom-skills/${skillId}/evolution`, {
          headers: { Authorization: `Bearer ${getToken()}` },
        }),
        fetch(`/api/v1/agent/custom-skills/${skillId}/optimize`, {
          headers: { Authorization: `Bearer ${getToken()}` },
        }),
      ]);
      const evo = evoRes.ok ? await evoRes.json() : {};
      const opt = optRes.ok ? await optRes.json() : {};
      skillEvolutionData = { ...skillEvolutionData, [skillId]: { ...evo, ...opt } };
    } catch {}
    loadingEvolution = new Set([...loadingEvolution].filter(x => x !== skillId));
  }

  async function recordSkillExecution(skillId: number, data: { input: string; output: string; success: boolean; duration_ms: number; feedback?: string }) {
    try {
      await fetch(`/api/v1/agent/custom-skills/${skillId}/evolution`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify(data),
      });
    } catch {}
  }

  // ===== AI Memory =====
  let memoryEntries = $state<any[]>([]);
  let loadingMemory = $state(false);

  async function loadMemory() {
    loadingMemory = true;
    try {
      const res = await fetch('/api/v1/ai/memory', {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        const data = await res.json();
        memoryEntries = data.entries || [];
      }
    } catch {}
    loadingMemory = false;
  }

  async function deleteMemory(memType: string, key: string) {
    try {
      const res = await fetch(`/api/v1/ai/memory/${encodeURIComponent(memType)}/${encodeURIComponent(key)}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        memoryEntries = memoryEntries.filter((e: any) => !(e.memory_type === memType && e.key === key));
        toast('记忆已删除', 'info');
      }
    } catch {}
  }

  async function clearAllMemory() {
    try {
      const res = await fetch('/api/v1/ai/memory', {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        memoryEntries = [];
        toast('所有记忆已清除', 'info');
      }
    } catch {}
  }

  // ===== Backup & Restore =====
  let exportingDB = $state(false);
  let importingDB = $state(false);

  async function exportDatabase() {
    exportingDB = true;
    try {
      const res = await fetch('/api/v1/backup/export', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (!res.ok) throw new Error('导出失败');
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `moduforge_backup_${Date.now()}.sql`;
      a.click();
      URL.revokeObjectURL(url);
      toast('数据库导出成功', 'success');
    } catch (e: any) {
      toast(e.message || '导出失败', 'error');
    } finally {
      exportingDB = false;
    }
  }

  async function importDatabase() {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.sql';
    input.onchange = async () => {
      const file = input.files?.[0];
      if (!file) return;
      importingDB = true;
      try {
        const form = new FormData();
        form.append('file', file);
        const res = await fetch('/api/v1/backup/import', {
          method: 'POST',
          headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
          body: form,
        });
        if (!res.ok) throw new Error('导入失败');
        toast('数据库导入成功', 'success');
      } catch (e: any) {
        toast(e.message || '导入失败', 'error');
      } finally {
        importingDB = false;
      }
    };
    input.click();
  }

  // ===== Backup Schedules =====
  let schedules = $state<any[]>([]);
  let schedulesLoading = $state(false);
  let showScheduleModal = $state(false);
  let scheduleForm = $state({ name: '', frequency: 'daily', keep_count: 7 });
  let runningScheduleId = $state<number | null>(null);

  async function loadSchedules() {
    const token = getToken();
    if (!token) return;
    schedulesLoading = true;
    try {
      const r = await fetch('/api/v1/backup/schedules', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) { const d = await r.json(); schedules = d.schedules || []; }
    } catch {}
    schedulesLoading = false;
  }

  async function createSchedule() {
    const token = getToken();
    if (!token) return;
    try {
      const r = await fetch('/api/v1/backup/schedules', {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify(scheduleForm),
      });
      if (r.ok) { toast('备份计划已创建', 'success'); showScheduleModal = false; await loadSchedules(); }
      else { toast('创建失败', 'error'); }
    } catch { toast('创建失败', 'error'); }
  }

  async function toggleSchedule(id: number, active: boolean) {
    const token = getToken();
    if (!token) return;
    try {
      await fetch(`/api/v1/backup/schedules/${id}`, {
        method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ is_visible: active }),
      });
      schedules = schedules.map(s => s.id === id ? { ...s, is_active: active } : s);
    } catch {}
  }

  async function deleteSchedule(id: number) {
    const token = getToken();
    if (!token) return;
    if (!confirm('确定删除此备份计划？')) return;
    try {
      const r = await fetch(`/api/v1/backup/schedules/${id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) { schedules = schedules.filter(s => s.id !== id); toast('已删除', 'success'); }
    } catch {}
  }

  async function runSchedule(id: number) {
    const token = getToken();
    if (!token) return;
    runningScheduleId = id;
    try {
      const r = await fetch(`/api/v1/backup/schedules/${id}/run`, { method: 'POST', headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) { toast('备份已执行', 'success'); await loadSchedules(); }
    } catch {}
    runningScheduleId = null;
  }

  // ===== Recycle Bin =====
  let recycleItems = $state<any[]>([]);
  let recycleLoading = $state(false);

  async function loadRecycleBin() {
    recycleLoading = true;
    try {
      const r = await fetch('/api/v1/recycle-bin', { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { const d = await r.json(); recycleItems = d.items || []; }
    } catch {}
    recycleLoading = false;
  }

  async function restoreItem(item: any) {
    try {
      const r = await fetch(`/api/v1/recycle-bin/${item.item_type}/${item.item_id}/restore`, { method: 'POST', headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { recycleItems = recycleItems.filter(i => i.id !== item.id); toast(`已恢复 ${item.item_name}`, 'success'); }
      else { toast((await r.json()).error || '恢复失败', 'error'); }
    } catch { toast('恢复失败', 'error'); }
  }

  async function permanentlyDelete(item: any) {
    if (!confirm(`确认永久删除 ${item.item_name}？此操作不可撤销。`)) return;
    try {
      await fetch(`/api/v1/recycle-bin/${item.item_type}/${item.item_id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${getToken()}` } });
      recycleItems = recycleItems.filter(i => i.id !== item.id);
      toast('已永久删除', 'info');
    } catch { toast('删除失败', 'error'); }
  }

  async function clearRecycleBin() {
    if (!confirm('确认清空回收站？所有项目将被永久删除。')) return;
    try {
      await fetch('/api/v1/recycle-bin', { method: 'DELETE', headers: { Authorization: `Bearer ${getToken()}` } });
      recycleItems = [];
      toast('回收站已清空', 'info');
    } catch { toast('操作失败', 'error'); }
  }

  // ===== Shortcuts =====
  let userShortcuts = $state<Shortcut[]>([]);
  let editingShortcutId = $state<string | null>(null);
  let recordingShortcut = $state(false);
  let shortcutRecordEl = $state<HTMLInputElement | undefined>();

  function initShortcuts() {
    userShortcuts = loadShortcuts();
  }

  function handleShortcutRecord(e: KeyboardEvent) {
    e.preventDefault();
    const sc = userShortcuts.find(s => s.id === editingShortcutId);
    if (!sc) return;
    sc.key = e.key;
    sc.ctrlKey = e.ctrlKey;
    sc.shiftKey = e.shiftKey;
    sc.metaKey = e.metaKey;
    userShortcuts = [...userShortcuts];
    recordingShortcut = false;
    editingShortcutId = null;
    saveShortcuts(userShortcuts);
    toast('快捷键已更新', 'success');
  }

  function startRecord(id: string) {
    editingShortcutId = id;
    recordingShortcut = true;
    setTimeout(() => shortcutRecordEl?.focus(), 50);
  }

  function doResetShortcuts() {
    if (!confirm('确认重置所有快捷键为默认值？')) return;
    userShortcuts = resetShortcuts();
    toast('快捷键已重置', 'success');
  }

  // ===== System Health =====
  let healthData: any = $state(null);
  let healthLoading = $state(false);

  // ===== Cache =====
  let cacheData: any = $state(null);
  let cacheLoading = $state(false);
  let cacheClearing = $state(false);

  // ===== Logs =====
  let logs: any[] = $state([]);
  let logsLoading = $state(false);
  let logsTotal = $state(0);
  let logsPage = $state(1);
  let logsLevel = $state('');
  let logsModule = $state('');
  let logsStats: any = $state(null);
  let logsStatsLoading = $state(false);
  let logsCleanupLoading = $state(false);
  let cleanupDays = $state(30);

  async function loadHealth() {
    healthLoading = true;
    try {
      const r = await fetch('/api/v1/health/system', {
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) healthData = await r.json();
    } catch {}
    healthLoading = false;
  }

  async function loadCacheStatus() {
    cacheLoading = true;
    try {
      const r = await fetch('/api/v1/admin/cache/status', {
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) cacheData = await r.json();
    } catch {}
    cacheLoading = false;
  }

  async function clearCache() {
    cacheClearing = true;
    try {
      const r = await fetch('/api/v1/admin/cache/clear', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) {
        const d = await r.json();
        toast(`缓存已清除，共 ${d.entries} 条`, 'success');
        cacheData = { entries: 0, ttl: cacheData?.ttl || '5m0s' };
      } else {
        toast('清除缓存失败', 'error');
      }
    } catch { toast('清除缓存失败', 'error'); }
    cacheClearing = false;
  }

  // ===== Logs Functions =====
  async function loadLogs() {
    logsLoading = true;
    try {
      const params = new URLSearchParams({ page: String(logsPage), limit: '50' });
      if (logsLevel) params.set('level', logsLevel);
      if (logsModule) params.set('module', logsModule);
      const r = await fetch(`/api/v1/admin/logs?${params}`, {
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) {
        const d = await r.json();
        logs = d.logs || [];
        logsTotal = d.total || 0;
      }
    } catch {}
    logsLoading = false;
  }

  async function loadLogsStats() {
    logsStatsLoading = true;
    try {
      const r = await fetch('/api/v1/admin/logs/stats', {
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) logsStats = await r.json();
    } catch {}
    logsStatsLoading = false;
  }

  async function cleanupLogs() {
    logsCleanupLoading = true;
    try {
      const r = await fetch(`/api/v1/admin/logs/cleanup?days=${cleanupDays}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${getToken()}` }
      });
      if (r.ok) {
        const d = await r.json();
        toast(d.message || '日志已清理', 'success');
        loadLogs();
        loadLogsStats();
      } else {
        toast('清理失败', 'error');
      }
    } catch { toast('清理失败', 'error'); }
    logsCleanupLoading = false;
  }

  function getLevelColor(level: string): string {
    switch (level) {
      case 'error': return 'var(--color-error)';
      case 'warn': case 'warning': return 'var(--color-warning)';
      case 'info': return 'var(--color-success)';
      case 'debug': return 'var(--color-text-muted)';
      default: return 'var(--color-text-secondary)';
    }
  }

  function getLevelBadgeBg(level: string): string {
    switch (level) {
      case 'error': return 'var(--color-error-light)';
      case 'warn': case 'warning': return 'var(--color-warning-light)';
      case 'info': return 'var(--color-success-light)';
      default: return 'var(--color-surface)';
    }
  }

  async function exportProject() {
    toast('请在项目编辑器中使用导出功能', 'info');
  }

  async function importProject() {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.zip';
    input.onchange = async () => {
      const file = input.files?.[0];
      if (!file) return;
      try {
        const form = new FormData();
        form.append('file', file);
        const res = await fetch('/api/v1/projects/import', {
          method: 'POST',
          headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
          body: form,
        });
        if (!res.ok) throw new Error('导入失败');
        toast('项目导入成功', 'success');
      } catch (e: any) {
        toast(e.message || '导入失败', 'error');
      }
    };
    input.click();
  }

  // ===== Theme =====
  let themeMode = $state(getTheme());
  function changeTheme(mode: 'light' | 'dark' | 'system') {
    themeMode = mode;
    setTheme(mode);
    toast(`已切换到${mode === 'dark' ? '深色' : mode === 'light' ? '浅色' : '跟随系统'}模式`, 'info');
  }

  // ===== Loading =====
  let loading = $state(true);

  async function loadAll() {
    loading = true;
    const token = getToken();

    // Load current config
    try {
      const r = await fetch('/api/v1/llm/config', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const cfg = await r.json();
        currentProvider = cfg.provider || 'opencode-zen';
        currentModelId = cfg.model_id || '';
      }
    } catch {}

    // Load providers
    try {
      const r = await fetch('/api/v1/llm/providers', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        presetProviders = data.providers || [];
      }
    } catch {}

    // Load provider configs (DB overrides)
    try {
      const r = await fetch('/api/v1/llm/provider-configs', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        for (const c of data.configs || []) {
          providerConfigs[c.id] = { endpoint: c.endpoint, api_key: c.api_key, models_json: c.models_json };
          // Parse user-added models
          if (c.models_json) {
            try {
              userModelsMap[c.id] = JSON.parse(c.models_json);
            } catch {}
          }
        }
      }
    } catch {}

    // Load custom providers
    try {
      const r = await fetch('/api/v1/llm/custom-providers', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        customProviders = data.providers || [];
      }
    } catch {}

    // Load agent config
    try {
      const r = await fetch('/api/v1/settings/agent', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        agentMaxIterations = parseInt(data.max_iterations) || 50;
        agentMaxResultLen = parseInt(data.max_result_len) || 32768;
      }
    } catch {}

    loading = false;
  }

  // ===== PWA Install =====
  let showInstallPrompt = $state(false);
  let deferredPrompt: any = null;

  async function installPWA() {
    if (!deferredPrompt) return;
    deferredPrompt.prompt();
    const result = await deferredPrompt.userChoice;
    if (result.outcome === 'accepted') {
      toast('已安装到桌面', 'success');
    }
    deferredPrompt = null;
    showInstallPrompt = false;
  }

  onMount(() => {
    void (async () => {
      username = localStorage.getItem('moduforge_username') || '';
      email = localStorage.getItem('moduforge_email') || '';

      await loadAll();
      await loadProfile(); // Sets isAdmin — must complete before admin-only calls

      // Parallelize independent data loads
      await Promise.all([
        loadRecycleBin(),
        loadFavorites(),
        loadSearchHistory(),
        loadCustomSkills(),
        loadMemory(),
        loadSchedules(),
      ]);
      initShortcuts();

      // Admin-only endpoints (loadProfile must have run first)
      if (isAdmin) {
        await Promise.all([loadEmailConfig(), loadHealth()]);
      }
    })();

    const handler = (e: Event) => {
      e.preventDefault();
      deferredPrompt = e;
      showInstallPrompt = true;
    };
    window.addEventListener('beforeinstallprompt', handler);

    return () => window.removeEventListener('beforeinstallprompt', handler);
  });

  // ===== Provider Config =====
  function openConfigModal(providerOrId: any) {
    let provider: any;
    if (typeof providerOrId === 'string') {
      provider = presetProviders.find((p: any) => p.id === providerOrId);
    } else {
      provider = providerOrId;
    }
    if (!provider) return;
    configModalProvider = { id: provider.id, name: provider.name, endpoint: provider.endpoint };
    const existing = providerConfigs[provider.id];
    configEndpoint = existing?.endpoint || provider.endpoint || '';
    configApiKey = existing?.api_key || '';
  }

  // ===== Models Modal =====
  function openModelsModal(providerOrId: any) {
    let provider: any;
    if (typeof providerOrId === 'string') {
      provider = presetProviders.find((p: any) => p.id === providerOrId);
    } else {
      provider = providerOrId;
    }
    if (!provider) return;
    modelsModalProvider = provider;
    showModelsModal = true;
  }

  function closeModelsModal() {
    showModelsModal = false;
    modelsModalProvider = null;
  }

  function closeConfigModal() {
    configModalProvider = null;
    configEndpoint = '';
    configApiKey = '';
  }

  async function saveProviderConfig() {
    if (!configModalProvider) return;
    const token = getToken();
    try {
      const r = await fetch('/api/v1/llm/provider-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          id: configModalProvider.id,
          endpoint: configEndpoint,
          api_key: configApiKey,
          models_json: providerConfigs[configModalProvider.id]?.models_json || '',
        }),
      });
      if (r.ok) {
        providerConfigs[configModalProvider.id] = { endpoint: configEndpoint, api_key: configApiKey, models_json: providerConfigs[configModalProvider.id]?.models_json || '' };
        toast('配置已保存', 'success');
        closeConfigModal();
      } else {
        toast((await r.json()).error || '保存失败', 'error');
      }
    } catch {
      toast('保存失败', 'error');
    }
  }

  async function resetProviderConfig(providerId: string) {
    const token = getToken();
    try {
      const r = await fetch(`/api/v1/llm/provider-config/${providerId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (r.ok) {
        delete providerConfigs[providerId];
        providerConfigs = { ...providerConfigs };
        delete userModelsMap[providerId];
        userModelsMap = { ...userModelsMap };
        toast('已恢复默认配置', 'success');
      } else {
        toast((await r.json()).error || '重置失败', 'error');
      }
    } catch {
      toast('重置失败', 'error');
    }
  }

  // ===== Add/Remove Model for Preset Provider =====
  function startAddModel(providerId: string) {
    addingModelProviderId = providerId;
    newModelId = '';
    newModelName = '';
  }

  function cancelAddModel() {
    addingModelProviderId = '';
    newModelId = '';
    newModelName = '';
  }

  async function saveAddModel() {
    if (!newModelId.trim() || !newModelName.trim()) {
      toast('请填写模型 ID 和名称', 'error');
      return;
    }
    const providerId = addingModelProviderId;
    const existing = userModelsMap[providerId] || [];
    // Check duplicate
    if (existing.some(m => m.id === newModelId.trim())) {
      toast('该模型 ID 已存在', 'error');
      return;
    }
    const updated = [...existing, { id: newModelId.trim(), name: newModelName.trim() }];
    userModelsMap[providerId] = updated;
    userModelsMap = { ...userModelsMap };
    const token = getToken();
    savingModel = true;
    try {
      const cfg = providerConfigs[providerId] || {};
      const r = await fetch('/api/v1/llm/provider-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          id: providerId,
          endpoint: cfg.endpoint || '',
          api_key: cfg.api_key || '',
          models_json: JSON.stringify(updated),
        }),
      });
      if (r.ok) {
        providerConfigs[providerId] = { ...cfg, models_json: JSON.stringify(updated) };
        providerConfigs = { ...providerConfigs };
        toast('模型已添加', 'success');
        addingModelProviderId = '';
        newModelId = '';
        newModelName = '';
      } else {
        // Rollback
        userModelsMap[providerId] = existing;
        userModelsMap = { ...userModelsMap };
        toast((await r.json()).error || '添加失败', 'error');
      }
    } catch {
      userModelsMap[providerId] = existing;
      userModelsMap = { ...userModelsMap };
      toast('添加失败', 'error');
    } finally {
      savingModel = false;
    }
  }

  async function removeUserModel(providerId: string, modelId: string) {
    const existing = userModelsMap[providerId] || [];
    const updated = existing.filter(m => m.id !== modelId);
    userModelsMap[providerId] = updated;
    userModelsMap = { ...userModelsMap };
    const token = getToken();
    removingModelKey = `${providerId}:${modelId}`;
    try {
      const cfg = providerConfigs[providerId] || {};
      const r = await fetch('/api/v1/llm/provider-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          id: providerId,
          endpoint: cfg.endpoint || '',
          api_key: cfg.api_key || '',
          models_json: updated.length > 0 ? JSON.stringify(updated) : '',
        }),
      });
      if (r.ok) {
        providerConfigs[providerId] = { ...cfg, models_json: updated.length > 0 ? JSON.stringify(updated) : '' };
        providerConfigs = { ...providerConfigs };
        toast('模型已移除', 'success');
      } else {
        userModelsMap[providerId] = existing;
        userModelsMap = { ...userModelsMap };
        toast('移除失败', 'error');
      }
    } catch {
      userModelsMap[providerId] = existing;
      userModelsMap = { ...userModelsMap };
      toast('移除失败', 'error');
    } finally {
      removingModelKey = '';
    }
  }

  // ===== Custom Provider CRUD =====
  function openNewCustomModal() {
    editingCustom = null;
    customForm = { name: '', endpoint: '', api_key: '', models: [] };
    showCustomModal = true;
  }

  function openEditCustomModal(p: any) {
    editingCustom = p;
    let parsedModels: {id: string; name: string; max_tokens: number}[] = [];
    try { parsedModels = JSON.parse(p.models_json || '[]'); } catch {}
    customForm = {
      name: p.name,
      endpoint: p.endpoint,
      api_key: p.api_key || '',
      models: parsedModels,
    };
    showCustomModal = true;
  }

  function closeCustomModal() {
    showCustomModal = false;
    editingCustom = null;
  }

  function addCustomModel() {
    customForm.models = [...customForm.models, { id: '', name: '', max_tokens: 32000 }];
  }

  function removeCustomModel(index: number) {
    customForm.models = customForm.models.filter((_, i) => i !== index);
  }

  async function saveCustomProvider() {
    const token = getToken();
    const payload = { ...customForm, models_json: JSON.stringify(customForm.models) };
    try {
      if (editingCustom) {
        const r = await fetch(`/api/v1/llm/custom-providers/${editingCustom.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify(payload),
        });
        if (r.ok) {
          toast('已更新', 'success');
        } else {
          toast((await r.json()).error || '更新失败', 'error');
          return;
        }
      } else {
        const r = await fetch('/api/v1/llm/custom-providers', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify(payload),
        });
        if (r.ok) {
          toast('已添加', 'success');
        } else {
          toast((await r.json()).error || '添加失败', 'error');
          return;
        }
      }
      closeCustomModal();
      await loadAll();
    } catch {
      toast('操作失败', 'error');
    }
  }

  async function deleteCustomProvider(id: string) {
    deletingCustomId = id;
    const token = getToken();
    try {
      const r = await fetch(`/api/v1/llm/custom-providers/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (r.ok) {
        toast('已删除', 'success');
        customProviders = customProviders.filter(p => p.id !== id);
      } else {
        toast((await r.json()).error || '删除失败', 'error');
      }
    } catch {
      toast('删除失败', 'error');
    }
    deletingCustomId = '';
  }

  // ===== Agent Config =====
  async function saveAgentConfig() {
    savingAgentConfig = true;
    const token = getToken();
    try {
      const r = await fetch('/api/v1/settings/agent', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          max_iterations: agentMaxIterations,
          max_result_len: agentMaxResultLen,
        }),
      });
      if (r.ok) {
        toast('Agent 配置已保存', 'success');
      } else {
        const data = await r.json();
        toast(data.error || '保存失败', 'error');
      }
    } catch {
      toast('保存失败', 'error');
    }
    savingAgentConfig = false;
  }

  function isPresetProvider(id: string) {
    return !id.startsWith('custom-') && presetProviders.some(p => p.id === id);
  }

  function getProvider(id: string) {
    return presetProviders.find(p => p.id === id) || customProviders.find(p => p.id === id);
  }

  function providerStatus(p: any) {
    if (p.id === currentProvider) return 'current';
    if (p.tier === 'free' || p.is_free) return 'free';
    if (p.tier === 'subscription') return 'subscription';
    const cfg = providerConfigs[p.id];
    if (cfg?.api_key) return 'configured';
    if (p.requires_key) return 'needs_key';
    return 'ready';
  }

  function statusBadgeClass(status: string) {
    switch (status) {
      case 'current': return 'bg-primary/20 text-primary';
      case 'free': return 'bg-green-500/20 text-green-400';
      case 'subscription': return 'bg-violet-500/20 text-violet-400';
      case 'configured': return 'bg-green-500/20 text-green-400';
      case 'needs_key': return 'bg-amber-500/20 text-amber-400';
      case 'ready': return 'bg-zinc-500/20 text-zinc-400';
      default: return 'bg-zinc-500/20 text-zinc-400';
    }
  }

  function statusLabel(status: string) {
    switch (status) {
      case 'current': return '使用中';
      case 'free': return '免费';
      case 'subscription': return '订阅';
      case 'configured': return '已配置';
      case 'needs_key': return '需配置';
      case 'ready': return '就绪';
      default: return '-';
    }
  }
</script>

<div class="settings-grid p-6 w-full max-w-4xl mx-auto space-y-8 overflow-x-hidden">
  <div>
    <h1 class="text-2xl font-bold text-[var(--color-text)]">设置</h1>
    <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">管理你的 ModuForge 配置</p>
  </div>

  <!-- Profile -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--gradient-brand-subtle)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">person</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">个人信息</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">你的账户信息与个人资料</p>
      </div>
    </div>
    <div class="space-y-4">
      <div class="flex items-center gap-4">
        <div class="relative group">
          <div class="w-16 h-16 rounded-full overflow-hidden" style="background: var(--color-surface-secondary)">
            {#if avatarUrl}
              <img src={avatarUrl} alt="avatar" class="w-full h-full object-cover" />
            {:else}
              <div class="w-full h-full flex items-center justify-center text-2xl font-bold text-[var(--color-text-muted)]">{username?.charAt(0)?.toUpperCase() || '?'}</div>
            {/if}
          </div>
          <button type="button" class="absolute inset-0 rounded-full bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-white text-xs" onclick={() => avatarInput?.click()}>
            上传
          </button>
          <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" class="hidden" bind:this={avatarInput} onchange={uploadAvatar} />
        </div>
        <div class="text-sm text-[var(--color-text-muted)]">
          <p>支持 PNG, JPG, GIF, WebP</p>
          <p>最大 2MB</p>
        </div>
      </div>
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">用户名</label>
        <input type="text" class="input-field" bind:value={username} disabled />
      </div>
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">邮箱</label>
        <input type="email" class="input-field" bind:value={email} disabled />
      </div>
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">显示名称</label>
        <input type="text" class="input-field" placeholder="输入显示名称" bind:value={displayName} />
      </div>
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">个人简介</label>
        <textarea class="input-field" rows="3" placeholder="介绍一下自己" bind:value={bio}></textarea>
      </div>
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">所在地</label>
        <input type="text" class="input-field" placeholder="城市, 国家" bind:value={location} />
      </div>
      <div>
        <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">网站</label>
        <input type="url" class="input-field" placeholder="https://example.com" bind:value={website} />
      </div>
      <button type="button" class="auth-submit px-6 py-2.5 rounded-xl font-semibold text-sm text-white disabled:opacity-50" onclick={saveProfile} disabled={savingProfile}>
        {savingProfile ? '保存中...' : '保存'}
      </button>
    </div>
  </section>

  <!-- Email Config (Admin) -->
  {#if isAdmin}
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
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">SMTP 主机</label>
          <input type="text" class="input-field" placeholder="smtp.example.com" bind:value={smtpHost} />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">端口</label>
          <input type="number" class="input-field" placeholder="587" bind:value={smtpPort} />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">用户名</label>
          <input type="text" class="input-field" placeholder="user@example.com" bind:value={smtpUser} />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">密码</label>
          <input type="password" class="input-field" placeholder="SMTP 密码" bind:value={smtpPass} />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">发件人名称</label>
          <input type="text" class="input-field" placeholder="ModuForge" bind:value={smtpFromName} />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">发件人地址</label>
          <input type="email" class="input-field" placeholder="noreply@example.com" bind:value={smtpFrom} />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">加密方式</label>
          <select class="input-field" bind:value={smtpUseTLS}>
            <option value={0}>无加密（不推荐）</option>
            <option value={1}>STARTTLS（端口 587）</option>
            <option value={2}>SSL/TLS（端口 465）</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">启用状态</label>
          <select class="input-field" bind:value={smtpIsActive}>
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
  {/if}

  <!-- Current Provider Info -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: linear-gradient(135deg, rgba(6,182,212,0.15), rgba(139,92,246,0.15))">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">psychology</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">LLM 提供商管理</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">配置 AI 提供商和模型</p>
      </div>
    </div>

    <!-- Current selection -->
    <div class="flex items-center gap-3 mb-5 p-3 rounded-xl" style="background: var(--gradient-brand-subtle); border: 1px solid var(--color-border);">
      <span class="material-symbols-outlined text-[var(--color-primary)] text-lg">check_circle</span>
      <div class="flex-1 min-w-0">
        <p class="text-sm font-medium text-[var(--color-text)] truncate">
          当前: {getProvider(currentProvider)?.name || currentProvider}
          {#if currentModelId}
            <span class="text-[var(--color-text-muted)]">/ {currentModelId}</span>
          {/if}
        </p>
      </div>
    </div>
  </section>

  <!-- Agent Settings -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: linear-gradient(135deg, rgba(249,115,22,0.15), rgba(239,68,68,0.15))">
        <span class="material-symbols-outlined text-[18px]" style="color: #f97316">smart_toy</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">Agent 配置</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">调整 Agent 行为参数</p>
      </div>
    </div>
    <div class="space-y-4">
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">最大迭代次数</label>
          <input type="number" class="input-field" min="1" max="100" bind:value={agentMaxIterations} />
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">Agent 单次任务最多执行步骤数（1-100）</p>
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">技能结果最大长度</label>
          <input type="number" class="input-field" min="500" max="100000" step="1000" bind:value={agentMaxResultLen} />
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">单次技能返回内容最大字符数（500-100000）</p>
        </div>
      </div>
      <button type="button" class="auth-submit px-6 py-2.5 rounded-xl font-semibold text-sm text-white disabled:opacity-50" onclick={saveAgentConfig} disabled={savingAgentConfig}>
        {savingAgentConfig ? '保存中...' : '保存 Agent 配置'}
      </button>
    </div>
  </section>

  <!-- Preset Providers -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">cloud</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">预设提供商</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">内置提供商，可自定义 Endpoint 和 API Key</p>
      </div>
    </div>

    {#if loading}
      <div class="space-y-2">
        {#each Array(5) as _}
          <div class="skeleton h-12 w-full rounded-xl"></div>
        {/each}
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="provider-table w-full text-sm">
          <thead>
            <tr class="text-left text-[var(--color-text-muted)]">
              <th class="pb-3 pr-4 font-medium">名称</th>
              <th class="pb-3 pr-4 font-medium">模型数</th>
              <th class="pb-3 pr-4 font-medium">状态</th>
              <th class="pb-3 pr-4 font-medium">Endpoint</th>
              <th class="pb-3 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each featuredProviders as p}
              {@const userModels = userModelsMap[p.id] || []}
              {@const totalModels = (p.models?.length || 0) + userModels.length}
              <tr class="border-t border-[var(--color-border)]">
                <td class="py-3 pr-4">
                  <span class="font-medium text-[var(--color-text)]">{p.name}</span>
                </td>
                <td class="py-3 pr-4 text-[var(--color-text-secondary)]">
                  <button class="hover:text-[var(--color-primary)] transition-colors cursor-pointer" onclick={() => { modelsModalProvider = p; showModelsModal = true; }}>
                    {totalModels}
                    {#if userModels.length > 0}
                      <span class="text-xs text-[var(--color-primary)]">(+{userModels.length})</span>
                    {/if}
                  </button>
                </td>
                <td class="py-3 pr-4">
                  <span class="badge text-xs {statusBadgeClass(providerStatus(p))}">
                    {statusLabel(providerStatus(p))}
                  </span>
                </td>
                <td class="py-3 pr-4 max-w-[200px] truncate text-[var(--color-text-muted)] text-xs" title={p.endpoint}>
                  {providerConfigs[p.id]?.endpoint || p.endpoint || '-'}
                </td>
                <td class="py-3 text-right">
                  <div class="flex items-center justify-end gap-1">
                    <button class="btn-ghost text-xs px-2.5 py-1.5 min-h-0" onclick={() => { modelsModalProvider = p; showModelsModal = true; }}>
                      模型
                    </button>
                    <button class="btn-ghost text-xs px-2.5 py-1.5 min-h-0" onclick={() => { configModalProvider = { id: p.id, name: p.name, endpoint: p.endpoint }; configEndpoint = providerConfigs[p.id]?.endpoint || p.endpoint || ''; configApiKey = providerConfigs[p.id]?.api_key || ''; }}>配置</button>
                    {#if providerConfigs[p.id]}
                      <button class="btn-ghost text-xs px-2.5 py-1.5 min-h-0 text-[var(--color-error)]" onclick={() => resetProviderConfig(p.id)}>
                        重置
                      </button>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
            <!-- "More providers" row -->
            {#if presetProviders.length > FEATURED_IDS.length}
              <tr class="border-t border-[var(--color-border)]">
                <td colspan="5" class="py-3 text-center">
                  <button
                    class="text-sm text-[var(--color-primary)] hover:underline inline-flex items-center gap-1.5"
                    onclick={() => showAllProvidersModal = true}
                  >
                    <span class="material-symbols-outlined text-[16px]">unfold_more</span>
                    更多供应商 ({presetProviders.length - FEATURED_IDS.length})
                  </button>
                </td>
              </tr>
            {/if}
          </tbody>
        </table>
      </div>
      <!-- Mobile card layout -->
      <div class="provider-cards-mobile flex-col gap-3" style="display: none;">
        {#each presetProviders.filter(p => FEATURED_IDS.includes(p.id)) as p}
          {@const userModels = userModelsMap[p.id] || []}
          {@const totalModels = (p.models?.length || 0) + userModels.length}
          <div class="p-4 rounded-xl" style="border: 1px solid var(--color-border);">
            <div class="flex items-center justify-between mb-2">
              <span class="font-medium text-sm text-[var(--color-text)]">{p.name}</span>
              <span class="badge text-xs {statusBadgeClass(providerStatus(p))}">
                {statusLabel(providerStatus(p))}
              </span>
            </div>
            <div class="flex items-center justify-between mb-2">
              <button class="text-xs text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors cursor-pointer" onclick={() => { modelsModalProvider = p; showModelsModal = true; }}>
                {totalModels} 个模型
                {#if userModels.length > 0}
                  <span class="text-[var(--color-primary)]">(+{userModels.length})</span>
                {/if}
              </button>
              <span class="text-xs text-[var(--color-text-muted)] truncate max-w-[160px]" title={p.endpoint}>
                {providerConfigs[p.id]?.endpoint || p.endpoint || '-'}
              </span>
            </div>
            <div class="flex gap-2 mt-2">
              <button class="btn-ghost text-xs px-3 py-1.5 min-h-0 flex-1" onclick={() => { modelsModalProvider = p; showModelsModal = true; }}>模型</button>
              <button class="btn-ghost text-xs px-3 py-1.5 min-h-0 flex-1" onclick={() => { configModalProvider = { id: p.id, name: p.name, endpoint: p.endpoint }; configEndpoint = providerConfigs[p.id]?.endpoint || p.endpoint || ""; configApiKey = providerConfigs[p.id]?.api_key || ""; }}>配置</button>
              {#if providerConfigs[p.id]}
                <button class="btn-ghost text-xs px-3 py-1.5 min-h-0 text-[var(--color-error)]" onclick={() => resetProviderConfig(p.id)}>重置</button>
              {/if}
            </div>
          </div>
        {/each}
        {#if presetProviders.length > FEATURED_IDS.length}
          <button
            class="w-full py-3 rounded-xl text-sm text-[var(--color-primary)] hover:bg-[var(--color-bg-secondary)] transition-colors inline-flex items-center justify-center gap-1.5"
            style="border: 1px dashed var(--color-border);"
            onclick={() => showAllProvidersModal = true}
          >
            <span class="material-symbols-outlined text-[16px]">unfold_more</span>
            更多供应商 ({presetProviders.length - FEATURED_IDS.length})
          </button>
        {/if}
      </div>
    {/if}
  </section>

  <!-- Custom Providers -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-success-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-success)">add_box</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">自定义提供商</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">添加 Open AI 兼容的自定义提供商</p>
      </div>
      <button class="btn-primary text-sm" onclick={openNewCustomModal}>
        <span class="material-symbols-outlined text-[16px]">add</span>
        添加
      </button>
    </div>

    {#if customProviders.length === 0}
      <p class="text-sm text-[var(--color-text-muted)] text-center py-6">暂无自定义提供商</p>
    {:else}
      <div class="space-y-2">
        {#each customProviders as cp}
          <div class="custom-provider-item flex items-center gap-3 p-3 rounded-xl" style="border: 1px solid var(--color-border);">
            <span class="material-symbols-outlined text-[var(--color-text-muted)]">dns</span>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-[var(--color-text)]">{cp.name}</p>
              <p class="text-xs text-[var(--color-text-muted)] truncate">{cp.endpoint}</p>
            </div>
            <span class="badge text-xs {cp.id === currentProvider ? 'bg-primary/20 text-primary' : 'bg-zinc-500/20 text-zinc-400'}">
              {cp.id === currentProvider ? '使用中' : cp.api_key ? '已配置' : '无 Key'}
            </span>
            <div class="custom-provider-actions flex items-center gap-1">
              <button class="btn-ghost text-xs px-2.5 py-1.5 min-h-0" onclick={() => openEditCustomModal(cp)}>编辑</button>
              <button class="btn-ghost text-xs px-2.5 py-1.5 min-h-0 text-[var(--color-error)]" onclick={() => deleteCustomProvider(cp.id)} disabled={deletingCustomId === cp.id}>
                {deletingCustomId === cp.id ? '删除中...' : '删除'}
              </button>
            </div>
          </div>
        {/each}
      </div>
      <!-- Mobile card layout -->
      <div class="custom-cards-mobile flex-col gap-3" style="display: none;">
        {#each customProviders as cp}
          <div class="p-4 rounded-xl" style="border: 1px solid var(--color-border);">
            <div class="flex items-center justify-between mb-2">
              <span class="font-medium text-sm text-[var(--color-text)]">{cp.name}</span>
              <span class="badge text-xs {cp.id === currentProvider ? 'bg-primary/20 text-primary' : 'bg-zinc-500/20 text-zinc-400'}">
                {cp.id === currentProvider ? '使用中' : cp.api_key ? '已配置' : '无 Key'}
              </span>
            </div>
            <p class="text-xs text-[var(--color-text-muted)] truncate mb-2">{cp.endpoint}</p>
            <div class="flex gap-2">
              <button class="btn-ghost text-xs px-3 py-1.5 min-h-0 flex-1" onclick={() => openEditCustomModal(cp)}>编辑</button>
              <button class="btn-ghost text-xs px-3 py-1.5 min-h-0 flex-1 text-[var(--color-error)]" onclick={() => deleteCustomProvider(cp.id)} disabled={deletingCustomId === cp.id}>
                {deletingCustomId === cp.id ? '删除中...' : '删除'}
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </section>

  <!-- Shortcuts -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">keyboard</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">键盘快捷键</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">自定义编辑器快捷键绑定</p>
      </div>
      <button class="btn-ghost text-xs px-3 py-1.5" onclick={doResetShortcuts}>重置默认</button>
    </div>
    <div class="space-y-2">
      {#each userShortcuts as sc}
        <div class="flex items-center gap-3 p-3 rounded-xl" style="border: 1px solid var(--color-border);">
          <div class="flex-1 min-w-0">
            <p class="text-sm text-[var(--color-text)]">{sc.label}</p>
          </div>
          {#if editingShortcutId === sc.id}
            <input
              type="text"
              class="shortcut-record-input text-xs px-3 py-1.5 rounded-lg"
              style="background: var(--color-surface); border: 2px solid var(--color-primary); color: var(--color-text); width: 140px; text-align: center;"
              bind:this={shortcutRecordEl}
              onkeydown={handleShortcutRecord}
              placeholder="按下快捷键..."
              readonly
            />
          {:else}
            <span class="text-xs px-3 py-1.5 rounded-lg font-mono" style="background: var(--color-surface); color: var(--color-text-secondary)">
              {sc.ctrlKey ? 'Ctrl+' : ''}{sc.shiftKey ? 'Shift+' : ''}{sc.key.toUpperCase()}
            </span>
          {/if}
          <button class="btn-ghost text-xs px-2 py-1" onclick={() => startRecord(sc.id)} disabled={recordingShortcut}>
            {editingShortcutId === sc.id ? '录制中...' : '编辑'}
          </button>
        </div>
      {/each}
    </div>
  </section>

  <!-- Theme -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: linear-gradient(135deg, rgba(249,115,22,0.15), rgba(168,85,247,0.15))">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-warning)">palette</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">外观</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">自定义界面主题和显示效果</p>
      </div>
    </div>
    <div class="flex items-center justify-between">
      <div>
        <p class="text-sm font-medium text-[var(--color-text)]">主题模式</p>
        <p class="text-xs" style="color: var(--color-text-muted)">当前: {themeMode === 'dark' ? '深色模式' : themeMode === 'light' ? '浅色模式' : '跟随系统'}</p>
      </div>
      <div class="flex gap-1 p-1 rounded-xl" style="background: var(--color-surface)">
        {#each [
          { mode: 'light' as const, icon: 'light_mode', label: '浅色' },
          { mode: 'dark' as const, icon: 'dark_mode', label: '深色' },
          { mode: 'system' as const, icon: 'brightness_auto', label: '系统' },
        ] as opt}
          <button
            class="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
            style={themeMode === opt.mode ? 'background: var(--color-primary); color: white' : 'color: var(--color-text-secondary)'}
            onclick={() => changeTheme(opt.mode)}
          >
            <span class="material-symbols-outlined text-[14px]">{opt.icon}</span>
            <span class="hidden sm:inline">{opt.label}</span>
          </button>
        {/each}
      </div>
    </div>
  </section>

  <!-- AI Memory -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">memory</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">AI 记忆</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">Agent 保存的用户偏好和上下文记忆</p>
      </div>
      <div class="flex gap-2">
        <button class="btn-ghost border text-xs px-3 py-1 rounded-lg" style="border-color: var(--color-border)" onclick={loadMemory}>
          <span class="material-symbols-outlined text-[14px]">refresh</span>
          刷新
        </button>
        {#if memoryEntries.length > 0}
          <button class="text-xs px-3 py-1 rounded-lg" style="border: 1px solid var(--color-error); color: var(--color-error)" onclick={clearAllMemory}>
            清空
          </button>
        {/if}
      </div>
    </div>
    {#if loadingMemory}
      <p class="text-sm text-[var(--color-text-secondary)]">加载中...</p>
    {:else if memoryEntries.length === 0}
      <p class="text-sm text-[var(--color-text-muted)]">暂无 AI 记忆。使用 Agent 后，偏好信息会自动保存。</p>
    {:else}
      <div class="space-y-2 max-h-64 overflow-auto">
        {#each memoryEntries as entry}
          <div class="flex items-center justify-between p-3 rounded-xl" style="background: var(--color-surface)">
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-0.5">
                <span class="text-xs px-2 py-0.5 rounded-full" style="background: var(--color-primary-light); color: var(--color-primary)">{entry.memory_type}</span>
                <span class="text-xs font-medium text-[var(--color-text)]">{entry.key}</span>
              </div>
              <p class="text-xs truncate" style="color: var(--color-text-secondary)">{entry.value}</p>
            </div>
            <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] flex-shrink-0" onclick={() => deleteMemory(entry.memory_type, entry.key)}>
              <span class="material-symbols-outlined text-[16px]" style="color: var(--color-text-muted)">delete</span>
            </button>
          </div>
        {/each}
      </div>
    {/if}
  </section>

  <!-- Backup & Restore -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-warning-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-warning)">backup</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">备份与恢复</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">导出或导入数据库与项目</p>
      </div>
    </div>
    <div class="grid grid-cols-2 gap-3">
      <button class="p-4 rounded-xl border text-left transition-all hover:border-[var(--color-primary)]" style="border-color: var(--color-border)" onclick={exportDatabase} disabled={exportingDB}>
        <span class="material-symbols-outlined text-[20px] block mb-1" style="color: var(--color-primary)">file_download</span>
        <span class="text-sm font-medium text-[var(--color-text)]">导出数据库</span>
        <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">下载 .sql 备份文件</p>
      </button>
      <button class="p-4 rounded-xl border text-left transition-all hover:border-[var(--color-primary)]" style="border-color: var(--color-border)" onclick={importDatabase} disabled={importingDB}>
        <span class="material-symbols-outlined text-[20px] block mb-1" style="color: var(--color-warning)">file_upload</span>
        <span class="text-sm font-medium text-[var(--color-text)]">导入数据库</span>
        <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">从 .sql 文件恢复</p>
      </button>
      <button class="p-4 rounded-xl border text-left transition-all hover:border-[var(--color-primary)]" style="border-color: var(--color-border)" onclick={exportProject}>
        <span class="material-symbols-outlined text-[20px] block mb-1" style="color: var(--color-primary)">folder_zip</span>
        <span class="text-sm font-medium text-[var(--color-text)]">导出项目</span>
        <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">备份整个项目为 ZIP</p>
      </button>
      <button class="p-4 rounded-xl border text-left transition-all hover:border-[var(--color-primary)]" style="border-color: var(--color-border)" onclick={importProject}>
        <span class="material-symbols-outlined text-[20px] block mb-1" style="color: var(--color-warning)">folder_open</span>
        <span class="text-sm font-medium text-[var(--color-text)]">导入项目</span>
        <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">从 ZIP 文件恢复</p>
      </button>
    </div>
  </section>

  <!-- Backup Schedules -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">schedule</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">定时备份</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">自动备份数据库和项目</p>
      </div>
      <button class="btn-primary text-sm" onclick={() => { scheduleForm = { name: '', frequency: 'daily', keep_count: 7 }; showScheduleModal = true; }}>
        <span class="material-symbols-outlined text-[14px]">add</span> 创建计划
      </button>
    </div>
    {#if schedulesLoading}
      <div class="skeleton h-16 w-full rounded-xl"></div>
    {:else if schedules.length === 0}
      <p class="text-sm text-center py-6" style="color: var(--color-text-muted)">暂无备份计划</p>
    {:else}
      <div class="space-y-2">
        {#each schedules as s}
          <div class="flex items-center gap-3 p-3 rounded-xl" style="border: 1px solid var(--color-border);">
            <span class="material-symbols-outlined text-[var(--color-text-muted)]">backup</span>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-[var(--color-text)]">{s.name}</p>
              <p class="text-xs" style="color: var(--color-text-muted)">
                {s.frequency === 'daily' ? '每日' : s.frequency === 'weekly' ? '每周' : '每月'} · 保留 {s.keep_count} 份
                {#if s.last_backup_at} · 上次: {new Date(s.last_backup_at).toLocaleDateString()}{/if}
              </p>
            </div>
            <button class="btn-ghost text-xs px-2 py-1" disabled={runningScheduleId === s.id} onclick={() => runSchedule(s.id)}>
              {runningScheduleId === s.id ? '执行中...' : '执行'}
            </button>
            <button class="btn-ghost text-xs px-2 py-1" onclick={() => toggleSchedule(s.id, !s.is_active)} style="color: {s.is_active ? 'var(--color-success)' : 'var(--color-text-muted)'}">
              {s.is_active ? '已启用' : '已禁用'}
            </button>
            <button class="btn-ghost text-xs px-2 py-1 text-[var(--color-error)]" onclick={() => deleteSchedule(s.id)}>删除</button>
          </div>
        {/each}
      </div>
    {/if}
  </section>

  <!-- System Health (admin only) -->
  {#if isAdmin}
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-success-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-success)">monitor_heart</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">系统健康</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">各服务运行状态和资源使用</p>
      </div>
      <button class="btn-ghost text-sm" onclick={loadHealth} disabled={healthLoading}>
        <span class="material-symbols-outlined text-[16px] {healthLoading ? 'animate-spin' : ''}">refresh</span>
        刷新
      </button>
    </div>
    {#if healthLoading}
      <div class="grid grid-cols-2 gap-3">
        {#each Array(4) as _}
          <div class="skeleton h-20 rounded-xl"></div>
        {/each}
      </div>
    {:else if healthData}
      <div class="flex items-center gap-2 mb-4">
        <span class="w-2.5 h-2.5 rounded-full" style="background: {healthData.status === 'healthy' ? 'var(--color-success)' : 'var(--color-error)'}"></span>
        <span class="text-sm font-medium" style="color: {healthData.status === 'healthy' ? 'var(--color-success)' : 'var(--color-error)'}">{healthData.status === 'healthy' ? '健康' : '异常'}</span>
        <span class="text-xs ml-auto" style="color: var(--color-text-muted)">运行时间: {healthData.uptime} · v{healthData.version}</span>
      </div>
      <div class="grid grid-cols-2 gap-3">
        {#each Object.entries(healthData.checks || {}) as [key, check]}
          <div class="p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
            <div class="flex items-center justify-between mb-1">
              <span class="text-xs font-medium" style="color: var(--color-text-secondary)">{key}</span>
              <span class="w-2 h-2 rounded-full" style="background: {(check as any).status === 'ok' || (check as any).status === 'healthy' ? 'var(--color-success)' : 'var(--color-error)'}"></span>
            </div>
            {#if (check as any).response_ms != null}
              <p class="text-lg font-bold text-[var(--color-text)]">{(check as any).response_ms}ms</p>
            {:else if (check as any).free_gb != null}
              <p class="text-lg font-bold text-[var(--color-text)]">{(check as any).free_gb}GB / {(check as any).total_gb}GB</p>
            {:else if (check as any).used_mb != null}
              <p class="text-lg font-bold text-[var(--color-text)]">{(check as any).used_mb}MB / {(check as any).total_mb}MB</p>
            {:else}
              <p class="text-lg font-bold text-[var(--color-text)]">{(check as any).status}</p>
            {/if}
          </div>
        {/each}
      </div>
    {:else}
      <button class="btn-primary text-sm" onclick={loadHealth}>加载健康检查</button>
    {/if}
  </section>
  {/if}

  <!-- Application Logs (admin only) -->
  {#if isAdmin}
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">receipt_long</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">应用日志</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">查看和管理系统日志</p>
      </div>
      <button class="btn-ghost text-sm" onclick={loadLogs} disabled={logsLoading}>
        <span class="material-symbols-outlined text-[16px] {logsLoading ? 'animate-spin' : ''}">refresh</span>
        刷新
      </button>
    </div>

    <!-- Log Stats Summary -->
    {#if logsStats}
      <div class="flex gap-2 mb-4 flex-wrap">
        {#each Object.entries(logsStats.levels || {}) as [level, count]}
          <span class="text-xs px-2 py-1 rounded-lg" style="background: {getLevelBadgeBg(level)}; color: {getLevelColor(level)}">
            {level}: {count}
          </span>
        {/each}
        <span class="text-xs px-2 py-1 rounded-lg" style="background: var(--color-surface); color: var(--color-text-muted)">
          共 {logsStats.total_logs || 0} 条
        </span>
      </div>
    {/if}

    <!-- Filters -->
    <div class="flex gap-2 mb-4 flex-wrap">
      <select class="text-sm px-3 py-1.5 rounded-lg border" style="border-color: var(--color-border); background: var(--color-bg); color: var(--color-text)" bind:value={logsLevel} onchange={() => { logsPage = 1; loadLogs(); }}>
        <option value="">所有级别</option>
        <option value="debug">Debug</option>
        <option value="info">Info</option>
        <option value="warn">Warning</option>
        <option value="error">Error</option>
      </select>
      <input type="text" class="text-sm px-3 py-1.5 rounded-lg border flex-1 min-w-[120px]" style="border-color: var(--color-border); background: var(--color-bg); color: var(--color-text)" placeholder="模块筛选..." bind:value={logsModule} oninput={() => { logsPage = 1; loadLogs(); }} />
      <button class="btn-ghost text-xs px-3 py-1.5" onclick={loadLogsStats} disabled={logsStatsLoading}>
        <span class="material-symbols-outlined text-[14px] {logsStatsLoading ? 'animate-spin' : ''}">bar_chart</span>
        统计
      </button>
    </div>

    <!-- Log List -->
    {#if logsLoading && logs.length === 0}
      <div class="space-y-2">
        {#each Array(5) as _}
          <div class="skeleton h-16 rounded-xl"></div>
        {/each}
      </div>
    {:else if logs.length === 0}
      <p class="text-sm text-center py-6" style="color: var(--color-text-muted)">暂无日志</p>
    {:else}
      <div class="space-y-1.5 max-h-96 overflow-y-auto">
        {#each logs as log}
          <div class="p-3 rounded-lg border text-sm" style="border-color: var(--color-border); background: var(--color-surface)">
            <div class="flex items-center gap-2 mb-1">
              <span class="text-xs px-1.5 py-0.5 rounded font-medium" style="background: {getLevelBadgeBg(log.level)}; color: {getLevelColor(log.level)}">{log.level}</span>
              {#if log.module}
                <span class="text-xs" style="color: var(--color-text-secondary)">{log.module}</span>
              {/if}
              <span class="text-xs ml-auto" style="color: var(--color-text-muted)">{new Date(log.created_at).toLocaleString()}</span>
            </div>
            <p class="text-sm" style="color: var(--color-text)">{log.message}</p>
            {#if log.details}
              <pre class="mt-1 text-xs p-2 rounded" style="background: var(--color-bg); color: var(--color-text-secondary); white-space: pre-wrap; max-height: 80px; overflow: auto">{log.details}</pre>
            {/if}
          </div>
        {/each}
      </div>

      <!-- Pagination -->
      {#if logsTotal > 50}
        <div class="flex items-center justify-between mt-4 pt-3 border-t" style="border-color: var(--color-border)">
          <span class="text-xs" style="color: var(--color-text-muted)">共 {logsTotal} 条，第 {logsPage} 页</span>
          <div class="flex gap-2">
            <button class="btn-ghost text-xs px-3 py-1" disabled={logsPage <= 1} onclick={() => { logsPage--; loadLogs(); }}>上一页</button>
            <button class="btn-ghost text-xs px-3 py-1" disabled={logsPage * 50 >= logsTotal} onclick={() => { logsPage++; loadLogs(); }}>下一页</button>
          </div>
        </div>
      {/if}
    {/if}

    <!-- Cleanup -->
    <div class="flex items-center gap-3 mt-4 pt-3 border-t" style="border-color: var(--color-border)">
      <span class="text-xs" style="color: var(--color-text-secondary)">清理旧日志：</span>
      <select class="text-xs px-2 py-1 rounded border" style="border-color: var(--color-border); background: var(--color-bg); color: var(--color-text)" bind:value={cleanupDays}>
        <option value={7}>7 天前</option>
        <option value={14}>14 天前</option>
        <option value={30}>30 天前</option>
        <option value={60}>60 天前</option>
        <option value={90}>90 天前</option>
      </select>
      <button class="btn-ghost text-xs px-3 py-1 text-[var(--color-error)]" onclick={cleanupLogs} disabled={logsCleanupLoading}>
        <span class="material-symbols-outlined text-[14px] {logsCleanupLoading ? 'animate-spin' : ''}">delete_sweep</span>
        {logsCleanupLoading ? '清理中...' : '清理'}
      </button>
    </div>
  </section>
  {/if}

  <!-- Recycle Bin -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-warning-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-warning)">recycling</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">回收站</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">已删除的项目和模块，30 天后自动清理</p>
      </div>
    </div>
    {#if recycleItems.length === 0}
      <p class="text-sm text-center py-6" style="color: var(--color-text-muted)">回收站为空</p>
    {:else}
      <div class="space-y-2">
        {#each recycleItems as item}
          <div class="flex items-center gap-3 p-3 rounded-xl" style="border: 1px solid var(--color-border);">
            <span class="material-symbols-outlined text-[var(--color-text-muted)]">{item.item_type === 'project' ? 'folder_delete' : 'inventory'}</span>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-[var(--color-text)] truncate">{item.item_name}</p>
              <p class="text-xs" style="color: var(--color-text-muted)">{new Date(item.deleted_at).toLocaleString()} · 过期: {new Date(item.expires_at).toLocaleDateString()}</p>
            </div>
            <button class="btn-ghost text-xs px-2 py-1 text-[var(--color-primary)]" onclick={() => restoreItem(item)}>恢复</button>
            <button class="btn-ghost text-xs px-2 py-1 text-[var(--color-error)]" onclick={() => permanentlyDelete(item)}>永久删除</button>
          </div>
        {/each}
      </div>
      <button class="mt-3 text-xs px-3 py-1.5 rounded-lg" style="background: var(--color-danger-light); color: var(--color-error)" onclick={clearRecycleBin}>清空回收站</button>
    {/if}
  </section>

  <!-- Favorites -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-error)">favorite</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">收藏夹</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">收藏的模块、项目和设备</p>
      </div>
    </div>
    <div class="space-y-2">
      <div class="flex gap-2 mb-3">
        <button class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors" style={favFilter === '' ? 'background: var(--color-primary-light); color: var(--color-primary)' : 'background: var(--color-surface); color: var(--color-text-muted)'} onclick={() => { favFilter = ''; loadFavorites(); }}>全部</button>
        <button class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors" style={favFilter === 'module' ? 'background: var(--color-primary-light); color: var(--color-primary)' : 'background: var(--color-surface); color: var(--color-text-muted)'} onclick={() => { favFilter = 'module'; loadFavorites(); }}>模块</button>
        <button class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors" style={favFilter === 'project' ? 'background: var(--color-primary-light); color: var(--color-primary)' : 'background: var(--color-surface); color: var(--color-text-muted)'} onclick={() => { favFilter = 'project'; loadFavorites(); }}>项目</button>
        <button class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors" style={favFilter === 'device' ? 'background: var(--color-primary-light); color: var(--color-primary)' : 'background: var(--color-surface); color: var(--color-text-muted)'} onclick={() => { favFilter = 'device'; loadFavorites(); }}>设备</button>
      </div>
      {#each favoriteItems as f}
        <div class="flex items-center justify-between p-3 rounded-xl" style="background: var(--color-surface-secondary); border: 1px solid var(--color-border)">
          <div class="flex-1 min-w-0">
            <p class="text-sm text-[var(--color-text)]">{f.item_type}: {f.item_id}</p>
            <p class="text-xs text-[var(--color-text-muted)]">{new Date(f.created_at).toLocaleDateString()}</p>
          </div>
          <button class="text-xs px-2 py-1 rounded-lg" style="background: var(--color-danger-light); color: var(--color-danger)" onclick={() => removeFavorite(f)}>取消收藏</button>
        </div>
      {:else}
        <p class="text-sm text-[var(--color-text-muted)]">暂无收藏</p>
      {/each}
    </div>
  </section>

  <!-- Search History -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">history</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">搜索历史</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">最近搜索记录</p>
      </div>
      <button class="ml-auto text-xs px-3 py-1.5 rounded-lg" style="background: var(--color-danger-light); color: var(--color-danger)" onclick={clearSearchHistory}>清空</button>
    </div>
    <div class="space-y-2">
      {#each searchHistory as h}
        <div class="flex items-center justify-between p-3 rounded-xl" style="background: var(--color-surface-secondary); border: 1px solid var(--color-border)">
          <div class="flex-1 min-w-0">
            <p class="text-sm text-[var(--color-text)]">{h.query}</p>
            <p class="text-xs text-[var(--color-text-muted)]">{h.result_count} 条结果 · {new Date(h.searched_at).toLocaleDateString()}</p>
          </div>
          <button class="text-xs px-2 py-1 rounded-lg" style="background: var(--color-danger-light); color: var(--color-danger)" onclick={() => deleteSearchHistoryItem(h.id)}>删除</button>
        </div>
      {:else}
        <p class="text-sm text-[var(--color-text-muted)]">暂无搜索记录</p>
      {/each}
    </div>
  </section>

  <!-- Security / 2FA -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-error-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-error)">security</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">安全</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">两步验证增强账户安全</p>
      </div>
    </div>
    <div class="flex items-center justify-between p-4 rounded-xl" style="border: 1px solid var(--color-border);">
      <div>
        <p class="text-sm font-medium text-[var(--color-text)]">两步验证 (TOTP)</p>
        <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">{totpEnabled ? '已启用' : '未启用'}</p>
      </div>
      {#if !showTotpQr}
        <button class="btn-ghost text-sm px-4 py-2" onclick={totpEnabled ? () => { totpCode = ''; showTotpQr = true; } : setup2FA} disabled={totpLoading}>
          {totpLoading ? '处理中...' : totpEnabled ? '禁用' : '启用'}
        </button>
      {/if}
    </div>
    {#if showTotpQr}
      <div class="mt-4 p-4 rounded-xl" style="background: var(--color-surface);">
        {#if totpQrUrl}
          <div class="flex justify-center mb-4">
            <img src={totpQrUrl} alt="TOTP QR Code" class="w-48 h-48 rounded-xl" />
          </div>
          <p class="text-xs text-center mb-3" style="color: var(--color-text-muted)">
            使用 Authenticator App 扫描二维码，或手动输入密钥：
          </p>
          <div class="flex items-center justify-center gap-2 mb-4">
            <code class="px-3 py-1.5 rounded-lg text-xs font-mono" style="background: var(--color-bg); color: var(--color-text); border: 1px solid var(--color-border);">{totpSecret}</code>
            <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)]" onclick={() => { navigator.clipboard.writeText(totpSecret); toast('已复制', 'success'); }}>
              <span class="material-symbols-outlined text-[16px]" style="color: var(--color-text-muted)">content_copy</span>
            </button>
          </div>
        {/if}
        <div class="flex items-center gap-3">
          <input type="text" bind:value={totpCode} placeholder="输入 6 位验证码" maxlength="6" class="input-field flex-1 text-center text-lg tracking-widest" onkeydown={(e) => { if (e.key === 'Enter') totpEnabled ? disable2FA() : enable2FA(); }} />
          <button class="btn-primary text-sm px-4 py-2" onclick={totpEnabled ? disable2FA : enable2FA} disabled={totpLoading || totpCode.length !== 6}>
            {totpLoading ? '验证中...' : totpEnabled ? '确认禁用' : '确认启用'}
          </button>
          <button class="btn-ghost text-sm px-3 py-2" onclick={() => { showTotpQr = false; totpCode = ''; }}>取消</button>
        </div>
      </div>
    {/if}

    <!-- Change Password -->
    <div class="flex items-center justify-between p-4 rounded-xl mt-3" style="border: 1px solid var(--color-border);">
      <div>
        <p class="text-sm font-medium text-[var(--color-text)]">修改密码</p>
        <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">更新你的登录密码</p>
      </div>
      {#if !showChangePassword}
        <button class="btn-ghost text-sm px-4 py-2" onclick={() => { showChangePassword = true; oldPassword = ''; newPassword = ''; confirmPassword = ''; }}>
          修改
        </button>
      {/if}
    </div>
    {#if showChangePassword}
      <div class="mt-3 p-4 rounded-xl space-y-3" style="background: var(--color-surface);">
        <div>
          <label class="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">当前密码</label>
          <input type="password" class="input-field w-full" placeholder="输入当前密码" bind:value={oldPassword} />
        </div>
        <div>
          <label class="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">新密码</label>
          <input type="password" class="input-field w-full" placeholder="至少 8 位" bind:value={newPassword} />
        </div>
        <div>
          <label class="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">确认新密码</label>
          <input type="password" class="input-field w-full" placeholder="再次输入新密码" bind:value={confirmPassword}
            onkeydown={(e) => { if (e.key === 'Enter') changePassword(); }} />
        </div>
        <div class="flex items-center gap-3 pt-1">
          <button class="btn-primary text-sm px-5 py-2" onclick={changePassword} disabled={changingPassword || !oldPassword || !newPassword || !confirmPassword}>
            {changingPassword ? '修改中...' : '确认修改'}
          </button>
          <button class="btn-ghost text-sm px-3 py-2" onclick={() => { showChangePassword = false; }}>取消</button>
        </div>
      </div>
    {/if}
  </section>

  <!-- PWA / Install -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">download</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">安装到桌面</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">将 ModuForge 安装为 PWA 应用</p>
      </div>
    </div>
    {#if showInstallPrompt}
      <button class="btn-primary text-sm" onclick={installPWA}>
        <span class="material-symbols-outlined text-[16px]">install_mobile</span>
        安装到桌面
      </button>
    {:else}
      <p class="text-xs" style="color: var(--color-text-muted)">PWA 安装按钮会在支持的应用商店中自动显示</p>
    {/if}
  </section>

  <!-- Custom Skills -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">smart_toy</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">AI 技能</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">共 {allSkills.length} 个技能</p>
      </div>
      <button class="btn-primary text-sm" onclick={openNewSkill}>
        <span class="material-symbols-outlined text-[16px]">add</span>
        创建
      </button>
    </div>
    {#if loadingSkills}
      <div class="space-y-2">
        {#each Array(3) as _}
          <div class="skeleton h-12 w-full rounded-xl"></div>
        {/each}
      </div>
    {:else if allSkills.length === 0}
      <p class="text-sm text-center py-6" style="color: var(--color-text-muted)">暂无技能</p>
    {:else}
      <div class="space-y-1.5">
        {#each (showAllSkills ? allSkills : allSkills.slice(0, 6)) as skill}
          <div class="rounded-xl overflow-hidden" style="border: 1px solid var(--color-border);">
            <!-- Skill header - click to expand -->
            <button
              type="button"
              class="w-full flex items-center gap-2.5 p-2.5 text-left hover:bg-[var(--color-surface-secondary)] transition-colors"
              onclick={() => expandedSkillId = expandedSkillId === skill.id ? null : skill.id}
            >
              <span class="material-symbols-outlined text-[16px] text-[var(--color-text-muted)]">
                {expandedSkillId === skill.id ? 'expand_more' : 'chevron_right'}
              </span>
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-[var(--color-text)]">{skill.name}</span>
                  {#if skill.is_builtin}
                    <span class="text-[10px] px-1.5 py-0.5 rounded" style="background: var(--color-primary-light); color: var(--color-primary)">内置</span>
                  {:else if skill.is_public}
                    <span class="text-[10px] px-1.5 py-0.5 rounded" style="background: var(--color-success-light); color: var(--color-success)">公开</span>
                  {/if}
                </div>
                <p class="text-xs truncate max-w-md" style="color: var(--color-text-muted)">{skill.description}</p>
              </div>
              {#if !skill.is_builtin}
                <div class="flex items-center gap-1" onclick={(e) => e.stopPropagation()}>
                  <button class="text-[10px] px-2 py-1 rounded hover:bg-[var(--color-surface)]" style="color: var(--color-text-muted)" onclick={() => openEditSkill(skill)}>编辑</button>
                  <button class="text-[10px] px-2 py-1 rounded hover:bg-[var(--color-error-light)]" style="color: var(--color-error)" onclick={() => deleteSkill(skill.id)}>删除</button>
                </div>
              {/if}
            </button>
            <!-- Expanded content - show prompt or description -->
            {#if expandedSkillId === skill.id}
              <div class="px-3 pb-3 pt-1" style="border-top: 1px solid var(--color-border)">
                {#if skill.is_builtin}
                  <div class="text-[10px] font-medium mb-1" style="color: var(--color-text-muted)">功能说明</div>
                  <div class="text-xs p-2.5 rounded-lg whitespace-pre-wrap" style="background: var(--color-bg); color: var(--color-text-secondary); line-height: 1.6">{skill.description}</div>
                {:else}
                  <div class="text-[10px] font-medium mb-1" style="color: var(--color-text-muted)">PROMPT</div>
                  <pre class="text-xs p-2.5 rounded-lg overflow-x-auto max-h-48 overflow-y-auto whitespace-pre-wrap" style="background: var(--color-bg); color: var(--color-text-secondary); font-family: 'JetBrains Mono', monospace; font-size: 11px; line-height: 1.5">{skill.prompt || '无提示词'}</pre>
                  {#if skill.input_schema && skill.input_schema !== '{}'}
                    <div class="text-[10px] font-medium mt-2 mb-1" style="color: var(--color-text-muted)">INPUT SCHEMA</div>
                    <pre class="text-xs p-2 rounded-lg overflow-x-auto" style="background: var(--color-bg); color: var(--color-text-secondary); font-family: 'JetBrains Mono', monospace; font-size: 10px">{typeof skill.input_schema === 'string' ? skill.input_schema : JSON.stringify(skill.input_schema, null, 2)}</pre>
                  {/if}
                  <div class="flex items-center gap-2 mt-2">
                    <button class="btn-ghost text-[11px] px-2.5 py-1" onclick={() => { loadSkillEvolution(skill.id); showEvolution = showEvolution === skill.id ? false : skill.id; }}>
                      📊 进化数据
                    </button>
                    <button class="btn-ghost text-[11px] px-2.5 py-1" onclick={() => { testSkillId = skill.id; testResult = ''; showTestInput = true; }}>
                      🧪 测试
                    </button>
                  </div>
                  {#if showEvolution === skill.id && skillEvolutionData[skill.id]}
                    {@const evo = skillEvolutionData[skill.id]}
                    {@const stats = evo.stats || {}}
                    <div class="grid grid-cols-3 gap-2 mt-2">
                      <div class="text-center p-1.5 rounded-lg" style="background: var(--color-bg)">
                        <div class="text-sm font-bold" style="color: var(--color-text)">{stats.total_runs || 0}</div>
                        <div class="text-[9px]" style="color: var(--color-text-muted)">运行次数</div>
                      </div>
                      <div class="text-center p-1.5 rounded-lg" style="background: var(--color-bg)">
                        <div class="text-sm font-bold" style="color: {evo.success_rate && parseFloat(evo.success_rate) >= 80 ? 'var(--color-success)' : 'var(--color-warning)'}">
                          {evo.success_rate || '0%'}
                        </div>
                        <div class="text-[9px]" style="color: var(--color-text-muted)">成功率</div>
                      </div>
                      <div class="text-center p-1.5 rounded-lg" style="background: var(--color-bg)">
                        <div class="text-sm font-bold" style="color: var(--color-text)">{evo.avg_duration || '0ms'}</div>
                        <div class="text-[9px]" style="color: var(--color-text-muted)">平均耗时</div>
                      </div>
                    </div>
                  {/if}
                {/if}
              </div>
            {/if}
          </div>
        {/each}
      </div>
      {#if allSkills.length > 6}
        <button
          type="button"
          class="w-full mt-3 py-2 text-xs font-medium rounded-lg transition-colors"
          style="color: var(--color-primary); background: var(--color-primary-light)"
          onclick={() => showAllSkills = !showAllSkills}
        >
          {showAllSkills ? '收起' : `显示全部 (${allSkills.length})`}
        </button>
      {/if}
    {/if}
  </section>

  <!-- API Cache -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">cached</span>
      </div>
      <div class="flex-1">
        <h2 class="text-base font-semibold text-[var(--color-text)]">API 缓存</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">只读 API 响应缓存，减少重复请求</p>
      </div>
      <button class="btn-ghost text-sm" onclick={loadCacheStatus} disabled={cacheLoading}>
        <span class="material-symbols-outlined text-[16px] {cacheLoading ? 'animate-spin' : ''}">refresh</span>
        刷新
      </button>
    </div>
    {#if cacheLoading}
      <div class="skeleton h-20 rounded-xl"></div>
    {:else if cacheData}
      <div class="flex items-center gap-4 mb-4">
        <div class="flex-1 p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
          <p class="text-xs" style="color: var(--color-text-muted)">缓存条目</p>
          <p class="text-lg font-bold text-[var(--color-text)]">{cacheData.entries || 0}</p>
        </div>
        <div class="flex-1 p-3 rounded-xl" style="background: var(--color-surface); border: 1px solid var(--color-border)">
          <p class="text-xs" style="color: var(--color-text-muted)">TTL</p>
          <p class="text-lg font-bold text-[var(--color-text)]">{cacheData.ttl || '5m'}</p>
        </div>
      </div>
      <p class="text-xs mb-3" style="color: var(--color-text-muted)">缓存的 API：模板列表、市场模块、热门模块、项目列表、LLM 供应商</p>
      <button class="btn-ghost text-sm border" style="border-color: var(--color-border); color: var(--color-error)" onclick={clearCache} disabled={cacheClearing}>
        <span class="material-symbols-outlined text-[16px] {cacheClearing ? 'animate-spin' : ''}">delete_sweep</span>
        {cacheClearing ? '清除中...' : '清除缓存'}
      </button>
    {:else}
      <button class="btn-primary text-sm" onclick={loadCacheStatus}>加载缓存状态</button>
    {/if}
  </section>

  <!-- About -->
  <section class="card p-6">
    <div class="flex items-center gap-3 mb-5">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-success-light)">
        <span class="material-symbols-outlined text-[18px]" style="color: var(--color-success)">info</span>
      </div>
      <div>
        <h2 class="text-base font-semibold text-[var(--color-text)]">关于</h2>
        <p class="text-xs" style="color: var(--color-text-muted)">ModuForge 版本和系统信息</p>
      </div>
    </div>
    <div class="space-y-2 text-sm text-[var(--color-text-secondary)]">
      <div class="flex justify-between py-1"><span>版本</span><span class="font-medium text-[var(--color-text)]">2.0-lite</span></div>
      <div class="flex justify-between py-1"><span>前端框架</span><span class="font-medium text-[var(--color-text)]">Svelte 5 + UnoCSS</span></div>
      <div class="flex justify-between py-1"><span>后端框架</span><span class="font-medium text-[var(--color-text)]">Go + Fiber + SQLite</span></div>
    </div>
    <a href="/docs" target="_blank" class="mt-4 inline-flex items-center gap-2 text-sm font-medium" style="color: var(--color-primary)">
      <span class="material-symbols-outlined text-[16px]">description</span>
      API 文档（Swagger UI）
    </a>
  </section>
</div>

<!-- Provider Config Modal -->
{#if configModalProvider}
  <div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" onclick={closeConfigModal}>
    <div class="card p-6 w-full max-w-md" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="flex items-center gap-3 mb-5">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center" style="background: var(--gradient-brand-subtle)">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-primary)">settings</span>
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--color-text)]">配置 {configModalProvider.name}</h3>
          <p class="text-xs text-[var(--color-text-muted)]">自定义 Endpoint 和 API Key</p>
        </div>
      </div>
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Endpoint</label>
          <input type="text" class="input-field" bind:value={configEndpoint} placeholder="https://api.openai.com/v1/chat/completions" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Key</label>
          <input type="password" class="input-field" bind:value={configApiKey} placeholder="sk-..." />
          <p class="text-xs text-[var(--color-text-muted)] mt-1">密钥加密存储在服务器</p>
        </div>
      </div>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button class="btn-ghost text-sm" onclick={closeConfigModal}>取消</button>
        <button class="btn-primary text-sm" onclick={saveProviderConfig}>保存</button>
      </div>
    </div>
  </div>
{/if}

<!-- Models Modal -->
{#if showModelsModal && modelsModalProvider}
  <div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" onclick={closeModelsModal}>
    <div class="card w-full max-w-2xl max-h-[80vh] flex flex-col" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="flex items-center justify-between p-5 pb-0">
        <div class="flex items-center gap-3">
          <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
            <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">smart_toy</span>
          </div>
          <div>
            <h3 class="text-base font-semibold text-[var(--color-text)]">{modelsModalProvider.name} · 模型管理</h3>
            <p class="text-xs text-[var(--color-text-muted)]">
              内置 {(modelsModalProvider.models?.length || 0)} 个
              {#if (userModelsMap[modelsModalProvider.id] || []).length > 0}
                + 用户 {(userModelsMap[modelsModalProvider.id] || []).length} 个
              {/if}
            </p>
          </div>
        </div>
        <button class="btn-ghost p-2 min-h-0" onclick={closeModelsModal}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="overflow-y-auto p-5 pt-4 flex-1 space-y-4">
        <!-- Built-in models -->
        {#if modelsModalProvider.models?.length > 0}
          <div>
            <h4 class="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wider mb-2">内置模型</h4>
            <div class="space-y-1">
              {#each modelsModalProvider.models as m}
                <div class="flex items-center justify-between py-2 px-3 rounded-lg" style="background: var(--color-bg-secondary);">
                  <div class="flex items-center gap-3 min-w-0">
                    <span class="font-medium text-sm text-[var(--color-text)] truncate">{m.name}</span>
                    <span class="text-xs text-[var(--color-text-muted)] truncate">{m.id}</span>
                  </div>
                  <div class="flex items-center gap-2 shrink-0">
                    {#if m.max_tokens}
                      <span class="text-xs text-[var(--color-text-muted)]">{(m.max_tokens / 1000).toFixed(0)}k</span>
                    {/if}
                    {#if m.price_input !== undefined && m.price_input > 0}
                      <span class="text-xs text-[var(--color-text-muted)]">${m.price_input}/${m.price_output}</span>
                    {/if}
                    {#if m.price_input === 0}
                      <span class="badge text-xs bg-[var(--color-success-light)] text-[var(--color-success)]">免费</span>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}
        <!-- User-added models -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <h4 class="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wider">用户自定义模型</h4>
          </div>
          {#if (userModelsMap[modelsModalProvider.id] || []).length > 0}
            <div class="space-y-1 mb-3">
              {#each userModelsMap[modelsModalProvider.id] || [] as um}
                <div class="flex items-center justify-between py-2 px-3 rounded-lg" style="background: var(--color-bg-secondary);">
                  <div class="flex items-center gap-3 min-w-0">
                    <span class="material-symbols-outlined text-[14px] text-[var(--color-primary)]">add_circle</span>
                    <span class="font-medium text-sm text-[var(--color-text)] truncate">{um.name}</span>
                    <span class="text-xs text-[var(--color-text-muted)] truncate">{um.id}</span>
                  </div>
                  <button
                    class="btn-ghost text-xs px-2 py-1 min-h-0 text-[var(--color-error)] shrink-0"
                    onclick={() => removeUserModel(modelsModalProvider.id, um.id)}
                    disabled={removingModelKey === `${modelsModalProvider.id}:${um.id}`}
                  >
                    {removingModelKey === `${modelsModalProvider.id}:${um.id}` ? '...' : '移除'}
                  </button>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-xs text-[var(--color-text-muted)] py-2">暂无自定义模型</p>
          {/if}
          <!-- Add model form -->
          {#if addingModelProviderId === modelsModalProvider.id}
            <div class="p-3 rounded-lg" style="border: 1px dashed var(--color-border);">
              <div class="flex items-center gap-3 flex-wrap">
                <input
                  type="text"
                  placeholder="模型 ID (如 my-model-v1)"
                  class="input-field text-xs py-1.5 flex-1 min-w-[140px]"
                  bind:value={newModelId}
                  onkeydown={(e) => { if (e.key === 'Enter') saveAddModel(); }}
                />
                <input
                  type="text"
                  placeholder="显示名称"
                  class="input-field text-xs py-1.5 flex-1 min-w-[120px]"
                  bind:value={newModelName}
                  onkeydown={(e) => { if (e.key === 'Enter') saveAddModel(); }}
                />
                <button
                  class="btn-primary text-xs px-3 py-1.5 min-h-0"
                  onclick={saveAddModel}
                  disabled={savingModel || !newModelId.trim() || !newModelName.trim()}
                >
                  {savingModel ? '保存中...' : '保存'}
                </button>
                <button class="btn-ghost text-xs px-3 py-1.5 min-h-0" onclick={cancelAddModel}>取消</button>
              </div>
            </div>
          {:else}
            <button
              class="w-full py-2.5 rounded-lg text-sm text-[var(--color-primary)] hover:bg-[var(--color-bg-secondary)] transition-colors inline-flex items-center justify-center gap-1.5"
              style="border: 1px dashed var(--color-border);"
              onclick={() => startAddModel(modelsModalProvider.id)}
            >
              <span class="material-symbols-outlined text-[16px]">add</span>
              添加自定义模型
            </button>
          {/if}
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- All Providers Modal -->
{#if showAllProvidersModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" onclick={() => showAllProvidersModal = false}>
    <div class="card w-full max-w-3xl max-h-[80vh] flex flex-col" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="flex items-center justify-between p-5 pb-0">
        <div class="flex items-center gap-3">
          <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
            <span class="material-symbols-outlined text-[18px]" style="color: var(--color-info)">cloud</span>
          </div>
          <div>
            <h3 class="text-base font-semibold text-[var(--color-text)]">全部预设供应商</h3>
            <p class="text-xs text-[var(--color-text-muted)]">共 {presetProviders.length} 个，可配置 Endpoint 和 API Key</p>
          </div>
        </div>
        <button class="btn-ghost p-2 min-h-0" onclick={() => showAllProvidersModal = false}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      <div class="overflow-y-auto p-5 pt-4 flex-1">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {#each presetProviders as p}
              {@const userModels = userModelsMap[p.id] || []}
            {@const totalModels = (p.models?.length || 0) + userModels.length}
            <div class="p-3 rounded-xl transition-colors" style="border: 1px solid var(--color-border); {p.id === currentProvider ? 'background: var(--color-primary-light); border-color: var(--color-primary);' : ''}">
              <div class="flex items-center justify-between mb-1.5">
                <span class="font-medium text-sm text-[var(--color-text)]">{p.name}</span>
                <span class="badge text-xs {statusBadgeClass(providerStatus(p))}">
                  {statusLabel(providerStatus(p))}
                </span>
              </div>
              <div class="mb-2">
                <button class="text-xs text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors cursor-pointer" onclick={() => { modelsModalProvider = p; showModelsModal = true; }}>
                  {totalModels} 个模型
                  {#if userModels.length > 0}
                    <span class="text-[var(--color-primary)]">(+{userModels.length})</span>
                  {/if}
                  <span class="material-symbols-outlined text-[12px] align-middle ml-0.5">open_in_new</span>
                </button>
              </div>
              <div class="flex gap-1.5">
                <button class="btn-ghost text-xs px-2 py-1 min-h-0 flex-1" onclick={() => { modelsModalProvider = p; showModelsModal = true; }}>模型</button>
                <button class="btn-ghost text-xs px-2 py-1 min-h-0 flex-1" onclick={() => { configModalProvider = { id: p.id, name: p.name, endpoint: p.endpoint }; configEndpoint = providerConfigs[p.id]?.endpoint || p.endpoint || ""; configApiKey = providerConfigs[p.id]?.api_key || ""; }}>配置</button>
                {#if providerConfigs[p.id]}
                  <button class="btn-ghost text-xs px-2 py-1 min-h-0 text-[var(--color-error)]" onclick={() => resetProviderConfig(p.id)}>重置</button>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- Custom Provider Modal -->
{#if showCustomModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" onclick={closeCustomModal}>
    <div class="card p-6 w-full max-w-md" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="flex items-center gap-3 mb-5">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center" style="background: var(--color-success-light)">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-success)">dns</span>
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--color-text)]">{editingCustom ? '编辑' : '添加'}自定义提供商</h3>
          <p class="text-xs text-[var(--color-text-muted)]">OpenAI 兼容的提供商</p>
        </div>
      </div>
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">名称</label>
          <input type="text" class="input-field" bind:value={customForm.name} placeholder="My Provider" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Endpoint</label>
          <input type="text" class="input-field" bind:value={customForm.endpoint} placeholder="https://api.example.com/v1/chat/completions" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">API Key</label>
          <input type="password" class="input-field" bind:value={customForm.api_key} placeholder="sk-..." />
        </div>
        <div>
          <div class="flex items-center justify-between mb-1">
            <label class="text-sm font-medium text-[var(--color-text-secondary)]">模型列表</label>
            <button class="text-xs text-[var(--color-primary)] hover:underline flex items-center gap-0.5" onclick={addCustomModel}>
              <span class="material-symbols-outlined text-[14px]">add</span> 添加模型
            </button>
          </div>
          {#if customForm.models.length === 0}
            <p class="text-xs text-[var(--color-text-muted)] py-2">暂无模型，点击上方添加</p>
          {:else}
            <div class="space-y-2 max-h-48 overflow-y-auto">
              {#each customForm.models as model, i}
                <div class="flex items-center gap-2 p-2 rounded-lg border border-[var(--color-border)]">
                  <input type="text" class="flex-1 px-2 py-1 rounded-lg text-xs border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={model.id} placeholder="model-id" />
                  <input type="text" class="flex-1 px-2 py-1 rounded-lg text-xs border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={model.name} placeholder="显示名称" />
                  <input type="number" class="w-20 px-2 py-1 rounded-lg text-xs border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={model.max_tokens} placeholder="tokens" />
                  <button class="p-1 rounded-lg hover:bg-[var(--color-surface)] text-[var(--color-error)]" onclick={() => removeCustomModel(i)}>
                    <span class="material-symbols-outlined text-[16px]">close</span>
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button class="btn-ghost text-sm" onclick={closeCustomModal}>取消</button>
        <button class="btn-primary text-sm" onclick={saveCustomProvider} disabled={!customForm.name || !customForm.endpoint}>
          {editingCustom ? '更新' : '添加'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Custom Skill Modal -->
{#if showSkillModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" onclick={() => showSkillModal = false}>
    <div class="card p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="flex items-center gap-3 mb-5">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-primary)">smart_toy</span>
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--color-text)]">{editingSkill ? '编辑' : '创建'}自定义技能</h3>
          <p class="text-xs text-[var(--color-text-muted)]">定义 AI 技能模板</p>
        </div>
      </div>
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">名称</label>
          <input type="text" class="input-field" bind:value={skillForm.name} placeholder="技能名称" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">描述</label>
          <input type="text" class="input-field" bind:value={skillForm.description} placeholder="简要描述技能功能" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">提示词模板</label>
          <textarea class="input-field resize-none" rows="5" bind:value={skillForm.prompt} placeholder="使用 {'{input}'} 表示用户输入位置"></textarea>
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">输入 Schema (JSON)</label>
            <textarea class="input-field resize-none font-mono text-xs" rows="3" bind:value={skillForm.input_schema} placeholder={`{"type": "object", "properties": {"input": {"type": "string"}}}`}></textarea>
        </div>
        <div class="flex items-center gap-2">
          <input type="checkbox" id="skill-public" bind:checked={skillForm.is_public} />
          <label for="skill-public" class="text-sm text-[var(--color-text)]">公开（其他用户可查看和使用）</label>
        </div>
        {#if editingSkill}
          <div class="border-t pt-4" style="border-color: var(--color-border);">
            <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">测试运行</label>
            <div class="flex gap-2 mb-2">
              <input type="text" class="input-field flex-1" bind:value={testInput} placeholder="输入测试内容" />
              <button class="btn-primary text-sm px-3 py-1.5" onclick={() => editingSkill && testSkill(editingSkill.id)} disabled={!editingSkill || testingSkillId === editingSkill?.id || !testInput}>
                {testingSkillId === editingSkill?.id ? '运行中...' : '运行'}
              </button>
            </div>
            {#if testResult}
              <pre class="p-3 rounded-xl text-xs font-mono max-h-40 overflow-y-auto" style="background: var(--color-bg); color: var(--color-text); border: 1px solid var(--color-border);">{testResult}</pre>
            {/if}
          </div>
        {/if}
      </div>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button class="btn-ghost text-sm" onclick={() => showSkillModal = false}>取消</button>
        <button class="btn-primary text-sm" onclick={saveSkill} disabled={!skillForm.name || !skillForm.prompt}>
          保存
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Schedule Modal -->
{#if showScheduleModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" onclick={() => showScheduleModal = false}>
    <div class="card p-6 w-full max-w-md" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="flex items-center gap-3 mb-5">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center" style="background: var(--color-info-light)">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-info)">schedule</span>
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--color-text)]">创建备份计划</h3>
          <p class="text-xs text-[var(--color-text-muted)]">配置自动备份</p>
        </div>
      </div>
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">名称</label>
          <input type="text" class="input-field" bind:value={scheduleForm.name} placeholder="每日备份" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">频率</label>
          <select class="input-field" bind:value={scheduleForm.frequency}>
            <option value="daily">每日</option>
            <option value="weekly">每周</option>
            <option value="monthly">每月</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">保留份数</label>
          <input type="number" class="input-field" bind:value={scheduleForm.keep_count} min="1" max="365" />
        </div>
      </div>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button class="btn-ghost text-sm" onclick={() => showScheduleModal = false}>取消</button>
        <button class="btn-primary text-sm" onclick={createSchedule} disabled={!scheduleForm.name}>创建</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .settings-grid {
    width: 100%;
    max-width: 100%;
  }
  .theme-toggle {
    background: var(--color-surface);
    border: 2px solid var(--color-border);
  }
  .theme-toggle.active {
    background: linear-gradient(135deg, #8b5cf6 0%, #06b6d4 100%);
    border-color: transparent;
    box-shadow: 0 0 16px rgba(139,92,246,0.3);
  }
  .theme-toggle:hover {
    transform: scale(1.05);
  }
  .theme-toggle:active {
    transform: scale(0.98);
  }
  .theme-toggle-thumb {
    transition: all 0.3s cubic-bezier(0.68, -0.55, 0.265, 1.55);
  }
  table {
    border-collapse: collapse;
  }

  /* Mobile responsive */
  @media (max-width: 640px) {
    .settings-grid {
      padding: 1rem !important;
      gap: 1.25rem !important;
    }
    .provider-table {
      display: none !important;
    }
    .provider-cards-mobile {
      display: flex !important;
    }
    .custom-provider-item {
      display: none !important;
    }
    .custom-cards-mobile {
      display: flex !important;
    }
    .add-model-form {
      flex-direction: column !important;
    }
    .add-model-form .input-field {
      min-width: 100% !important;
    }
    .custom-provider-item {
      flex-wrap: wrap !important;
    }
    .custom-provider-actions {
      width: 100% !important;
      justify-content: flex-end !important;
      margin-top: 0.5rem;
    }
    .config-modal-body {
      padding: 1rem !important;
    }
  }
  @media (min-width: 641px) {
    .provider-cards-mobile {
      display: none !important;
    }
    .custom-cards-mobile {
      display: none !important;
    }
  }
</style>

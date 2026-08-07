<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getTheme, setTheme } from '$lib/stores/theme';
  import { getToken } from '$lib/api/client';
  import ProfileSection from './components/ProfileSection.svelte';
  import SecuritySection from './components/SecuritySection.svelte';
  import ProviderSection from './components/ProviderSection.svelte';
  import ShortcutsSection from './components/ShortcutsSection.svelte';
  import AppearanceSection from './components/AppearanceSection.svelte';
  import AdvancedSection from './components/AdvancedSection.svelte';
  import DangerZone from './components/DangerZone.svelte';

  // ===== Agent Settings =====
  let agentMaxIterations = $state(50);
  let agentMaxResultLen = $state(32768);
  let savingAgentConfig = $state(false);

  // ===== Favorites =====
  let favoriteItems = $state<{ id: number; item_type: string; item_id: number; created_at: string }[]>([]);
  let favFilter = $state('');

  // ===== Search History =====
  let searchHistory = $state<{ id: number; query: string; result_count: number; searched_at: string }[]>([]);

  // ===== Loading =====
  let loading = $state(true);

  // ===== Email Config =====
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

  let isAdmin = $state(false);

  // ===== Custom Skills =====
  interface SkillInfo {
    id: number;
    name: string;
    description: string;
    prompt: string;
    input_schema: string;
    is_public: boolean;
    is_builtin?: boolean;
    icon?: string;
    category?: string;
  }

  interface EvolutionData {
    versions: { version: number; timestamp: string; changes: string }[];
    score: number;
    metrics: Record<string, number>;
    stats?: Record<string, unknown>;
    success_rate?: string;
    avg_duration?: string;
  }

  interface MemoryEntry {
    id: number;
    key: string;
    value: string;
    tags: string[];
    created_at: string;
    updated_at: string;
  }

  interface BackupSchedule {
    id: number;
    name: string;
    frequency: string;
    keep_count: number;
    last_run: string | null;
    next_run: string | null;
    active: boolean;
  }

  interface RecycleItem {
    id: number;
    type: string;
    name: string;
    deleted_at: string;
    deleted_by: string;
  }

  let customSkills: SkillInfo[] = $state([]);
  let builtinSkills: SkillInfo[] = $state([]);
  let allSkills = $derived([...builtinSkills, ...customSkills]);
  let showAllSkills = $state(false);
  let expandedSkillId = $state<number | null>(null);
  let showSkillModal = $state(false);
  let editingSkill: SkillInfo | null = $state(null);
  let skillForm = $state({ name: '', description: '', prompt: '', input_schema: '{}', is_public: false });
  let loadingSkills = $state(false);
  let testingSkillId = $state<number | null>(null);
  let testSkillId = $state<number | null>(null);
  let testInput = $state('');
  let testResult = $state('');
  let showTestInput = $state(false);

  let skillEvolutionData = $state<Record<number, EvolutionData>>({});
  let loadingEvolution = $state<Set<number>>(new Set());
  let showEvolution = $state<number | null>(null);

  // ===== AI Memory =====
  let memoryEntries: MemoryEntry[] = $state([]);
  let loadingMemory = $state(false);

  // ===== Backup & Restore =====
  let exportingDB = $state(false);
  let importingDB = $state(false);

  // ===== Backup Schedules =====
  let schedules: BackupSchedule[] = $state([]);
  let schedulesLoading = $state(false);
  let showScheduleModal = $state(false);
  let scheduleForm = $state({ name: '', frequency: 'daily', keep_count: 7 });
  let runningScheduleId = $state<number | null>(null);

  // ===== Recycle Bin =====
  let recycleItems: RecycleItem[] = $state([]);
  let recycleLoading = $state(false);

  // ===== System Health =====
  interface HealthInfo {
    uptime: string;
    version: string;
    goroutines: number;
    memory: string;
  }
  let healthData: HealthInfo | null = $state(null);
  let healthLoading = $state(false);

  // ===== Cache =====
  interface CacheInfo {
    entries: number;
    ttl: string;
    size?: string;
    keys?: number;
  }
  let cacheData: CacheInfo | null = $state(null);
  let cacheLoading = $state(false);
  let cacheClearing = $state(false);

  // ===== Logs =====
  interface LogEntry {
    id: number;
    level: string;
    module: string;
    message: string;
    timestamp: string;
  }
  interface LogStats {
    total: number;
    levels: Record<string, number>;
  }
  let logs: LogEntry[] = $state([]);
  let logsLoading = $state(false);
  let logsTotal = $state(0);
  let logsPage = $state(1);
  let logsLevel = $state('');
  let logsModule = $state('');
  let logsStats: LogStats | null = $state(null);
  let logsStatsLoading = $state(false);
  let logsCleanupLoading = $state(false);
  let cleanupDays = $state(30);

  // ===== Theme =====
  let themeMode = $state(getTheme());

  // ===== PWA Install =====
  let showInstallPrompt = $state(false);
  let deferredPrompt: Event & { prompt: () => void; userChoice: Promise<{ outcome: string }> } | null = null;

  // ===== Functions =====

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

  async function loadCustomSkills() {
    loadingSkills = true;
    try {
      const resBuiltin = await fetch('/api/v1/agent/skills', {
        headers: { 'Authorization': `Bearer ${getToken()}` },
      });
      if (resBuiltin.ok) {
        const data = await resBuiltin.json();
        builtinSkills = (data.skills || []).map((s: any) => ({ ...s, id: `builtin_${s.name}`, is_builtin: true }));
      }
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
    const url = isEdit ? `/api/v1/agent/custom-skills/${editingSkill!.id}` : '/api/v1/agent/custom-skills';
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

  function changeTheme(mode: 'light' | 'dark' | 'system') {
    themeMode = mode;
    setTheme(mode);
    toast(`已切换到${mode === 'dark' ? '深色' : mode === 'light' ? '浅色' : '跟随系统'}模式`, 'info');
  }

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

  async function loadAll() {
    loading = true;
    const token = getToken();
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

  onMount(() => {
    void (async () => {
      await loadAll();

      await Promise.all([
        loadRecycleBin(),
        loadFavorites(),
        loadSearchHistory(),
        loadCustomSkills(),
        loadMemory(),
        loadSchedules(),
      ]);
    })();

    const handler = (e: Event) => {
      e.preventDefault();
      deferredPrompt = e as any;
      showInstallPrompt = true;
    };
    window.addEventListener('beforeinstallprompt', handler);

    return () => window.removeEventListener('beforeinstallprompt', handler);
  });
</script>

<div class="settings-grid p-6 w-full max-w-4xl mx-auto space-y-8 overflow-x-hidden">
  <div>
    <h1 class="text-2xl font-bold text-[var(--color-text)]">设置</h1>
    <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">管理你的 ModuForge 配置</p>
  </div>

  <!-- Profile -->
  <ProfileSection onProfileLoaded={(data) => {
    isAdmin = (data as any).isAdmin ?? data as any;
    if (isAdmin) {
      Promise.all([loadEmailConfig(), loadHealth()]);
    }
  }} />

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
          <label for="smtp-host" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">SMTP 主机</label>
          <input id="smtp-host" type="text" class="input-field" placeholder="smtp.example.com" bind:value={smtpHost} />
        </div>
        <div>
          <label for="smtp-port" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">端口</label>
          <input id="smtp-port" type="number" class="input-field" placeholder="587" bind:value={smtpPort} />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label for="smtp-user" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">用户名</label>
          <input id="smtp-user" type="text" class="input-field" placeholder="user@example.com" bind:value={smtpUser} />
        </div>
        <div>
          <label for="smtp-pass" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">密码</label>
          <input id="smtp-pass" type="password" class="input-field" placeholder="SMTP 密码" bind:value={smtpPass} />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label for="smtp-from-name" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">发件人名称</label>
          <input id="smtp-from-name" type="text" class="input-field" placeholder="ModuForge" bind:value={smtpFromName} />
        </div>
        <div>
          <label for="smtp-from" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">发件人地址</label>
          <input id="smtp-from" type="email" class="input-field" placeholder="noreply@example.com" bind:value={smtpFrom} />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
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
  {/if}

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
          <label for="agent-max-iterations" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">最大迭代次数</label>
          <input id="agent-max-iterations" type="number" class="input-field" min="1" max="100" bind:value={agentMaxIterations} />
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">Agent 单次任务最多执行步骤数（1-100）</p>
        </div>
        <div>
          <label for="agent-max-result-len" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">技能结果最大长度</label>
          <input id="agent-max-result-len" type="number" class="input-field" min="500" max="100000" step="1000" bind:value={agentMaxResultLen} />
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">单次技能返回内容最大字符数（500-100000）</p>
        </div>
      </div>
      <button type="button" class="auth-submit px-6 py-2.5 rounded-xl font-semibold text-sm text-white disabled:opacity-50" onclick={saveAgentConfig} disabled={savingAgentConfig}>
        {savingAgentConfig ? '保存中...' : '保存 Agent 配置'}
      </button>
    </div>
  </section>

  <!-- LLM Providers -->
  <ProviderSection />

  <!-- Keyboard Shortcuts -->
  <ShortcutsSection />

  <!-- Appearance -->
  <AppearanceSection themeMode={themeMode} onThemeChange={(mode) => { themeMode = mode; }} />

  <!-- Advanced Settings -->
  <AdvancedSection isAdmin={isAdmin} />

  <!-- Danger Zone -->
  <DangerZone onClear={() => {
    loadRecycleBin();
    loadFavorites();
    loadSearchHistory();
    loadMemory();
  }} />

  <!-- Security / 2FA -->
  <SecuritySection />

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
            <div
              role="button"
              tabindex="0"
              class="w-full flex items-center gap-2.5 p-2.5 text-left hover:bg-[var(--color-surface-secondary)] transition-colors cursor-pointer"
              onclick={() => expandedSkillId = expandedSkillId === skill.id ? null : skill.id}
              onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); expandedSkillId = expandedSkillId === skill.id ? null : skill.id; } }}
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
                <div class="flex items-center gap-1">
                  <button class="text-[10px] px-2 py-1 rounded hover:bg-[var(--color-surface)]" style="color: var(--color-text-muted)" onclick={(e) => { e.stopPropagation(); openEditSkill(skill); }}>编辑</button>
                  <button class="text-[10px] px-2 py-1 rounded hover:bg-[var(--color-error-light)]" style="color: var(--color-error)" onclick={(e) => { e.stopPropagation(); deleteSkill(skill.id); }}>删除</button>
                </div>
              {/if}
            </div>
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
                    <button class="btn-ghost text-[11px] px-2.5 py-1" onclick={() => { loadSkillEvolution(skill.id); showEvolution = showEvolution === skill.id ? null : skill.id; }}>
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

<!-- Custom Skill Modal -->
{#if showSkillModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showSkillModal = false; }} onkeydown={(e) => { if (e.key === 'Escape') showSkillModal = false; }}>
    <div class="card p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto" role="dialog" aria-modal="true" tabindex="-1">
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
          <label for="skill-name" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">名称</label>
          <input id="skill-name" type="text" class="input-field" bind:value={skillForm.name} placeholder="技能名称" />
        </div>
        <div>
          <label for="skill-description" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">描述</label>
          <input id="skill-description" type="text" class="input-field" bind:value={skillForm.description} placeholder="简要描述技能功能" />
        </div>
        <div>
          <label for="skill-prompt" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">提示词模板</label>
          <textarea id="skill-prompt" class="input-field resize-none" rows="5" bind:value={skillForm.prompt} placeholder="使用 {'{input}'} 表示用户输入位置"></textarea>
        </div>
        <div>
          <label for="skill-schema" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">输入 Schema (JSON)</label>
            <textarea id="skill-schema" class="input-field resize-none font-mono text-xs" rows="3" bind:value={skillForm.input_schema} placeholder={`{"type": "object", "properties": {"input": {"type": "string"}}}`}></textarea>
        </div>
        <div class="flex items-center gap-2">
          <input type="checkbox" id="skill-public" bind:checked={skillForm.is_public} />
          <label for="skill-public" class="text-sm text-[var(--color-text)]">公开（其他用户可查看和使用）</label>
        </div>
        {#if editingSkill}
          <div class="border-t pt-4" style="border-color: var(--color-border);">
            <span class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">测试运行</span>
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
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showScheduleModal = false; }} onkeydown={(e) => { if (e.key === 'Escape') showScheduleModal = false; }}>
    <div class="card p-6 w-full max-w-md" role="dialog" aria-modal="true" tabindex="-1">
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
          <label for="schedule-name" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">名称</label>
          <input id="schedule-name" type="text" class="input-field" bind:value={scheduleForm.name} placeholder="每日备份" />
        </div>
        <div>
          <label for="schedule-frequency" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">频率</label>
          <select id="schedule-frequency" class="input-field" bind:value={scheduleForm.frequency}>
            <option value="daily">每日</option>
            <option value="weekly">每周</option>
            <option value="monthly">每月</option>
          </select>
        </div>
        <div>
          <label for="schedule-keep-count" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">保留份数</label>
          <input id="schedule-keep-count" type="number" class="input-field" bind:value={scheduleForm.keep_count} min="1" max="365" />
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

  /* Mobile responsive */
  @media (max-width: 640px) {
    .settings-grid {
      padding: 1rem !important;
      gap: 1.25rem !important;
    }
  }
</style>

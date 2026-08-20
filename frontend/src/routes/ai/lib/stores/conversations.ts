/**
 * Conversation & session management state and operations.
 */
import { toast } from '$lib/stores/toast.svelte';
import { client } from '$lib/api/client';
import type { Mode, AgentStep, TokenUsage } from '../types';
import { MODES } from '../types';
import { generateUUID } from '../utils';
import {
  loadGenHistory as loadGenHistoryFromStorage,
  fetchConversations,
  fetchConversation,
  deleteConversationById,
  saveConversationToBackend,
  loadSessionsList,
  deleteSessionById,
  exportSessionById,
  renameSessionById,
  fetchSessionMessages,
  exportConversationToFile,
  searchSessions,
  fetchProjectFiles,
  deployToAdb,
} from '../history';
import { loadProjectFilesState } from '../context';

export interface ConversationState {
  // History sidebar
  showHistorySidebar: boolean;
  historyTab: 'conversations' | 'generations';
  convSaving: boolean;
  convLoading: boolean;
  savedConversations: any[];
  // Sessions
  sessions: any[];
  sessionsTotal: number;
  sessionsLoading: boolean;
  hasMoreMessages: boolean;
  loadingEarlier: boolean;
  activeSessionId: string;
  searchResults: any[];
  // Generation history
  genHistory: any[];
}

const SESSIONS_PAGE_SIZE = 50;
const modes = MODES;

export function createConversationState(): ConversationState {
  return {
    showHistorySidebar: false,
    historyTab: 'conversations',
    convSaving: false,
    convLoading: false,
    savedConversations: [],
    sessions: [],
    sessionsTotal: 0,
    sessionsLoading: false,
    hasMoreMessages: false,
    loadingEarlier: false,
    activeSessionId: '',
    searchResults: [],
    genHistory: [],
  };
}

// ─── Session management ───

export async function loadSessions(s: ConversationState): Promise<void> {
  s.sessionsLoading = true;
  const { sessions: list, total } = await loadSessionsList(0, SESSIONS_PAGE_SIZE);
  s.sessions = list;
  s.sessionsTotal = total;
  s.sessionsLoading = false;
}

export async function loadMoreSessions(s: ConversationState): Promise<void> {
  if (s.sessionsLoading) return;
  s.sessionsLoading = true;
  const { sessions: more, total } = await loadSessionsList(s.sessions.length, SESSIONS_PAGE_SIZE);
  s.sessions = [...s.sessions, ...more];
  s.sessionsTotal = total;
  s.sessionsLoading = false;
}

export async function deleteSession(s: ConversationState, targetSessionId: string, activeId: string): Promise<string> {
  const ok = await deleteSessionById(targetSessionId);
  if (ok) {
    const newActiveId = activeId === targetSessionId ? '' : activeId;
    toast('对话已删除', 'success');
    await loadSessions(s);
    return newActiveId;
  }
  toast('删除失败', 'error');
  return activeId;
}

export async function exportSession(targetSessionId: string, format: 'markdown' | 'json' = 'markdown'): Promise<void> {
  const ok = await exportSessionById(targetSessionId, format);
  if (ok) toast(format === 'json' ? '已导出 JSON' : '导出成功', 'success');
  else toast('导出失败', 'error');
}

export async function renameSession(s: ConversationState, targetSessionId: string, title: string): Promise<void> {
  const ok = await renameSessionById(targetSessionId, title);
  if (ok) {
    toast('已重命名', 'success');
    await loadSessions(s);
  } else {
    toast('重命名失败', 'error');
  }
}

// ─── Conversation management ───

export async function loadConversations(s: ConversationState): Promise<void> {
  s.convLoading = true;
  s.savedConversations = await fetchConversations();
  s.convLoading = false;
}

export async function deleteConversation(
  s: ConversationState,
  id: string,
  activeSessionId: string,
): Promise<string> {
  const ok = await deleteConversationById(id);
  if (ok) {
    const newActiveId = activeSessionId === id ? '' : activeSessionId;
    toast('对话已删除', 'success');
    await loadConversations(s);
    return newActiveId;
  }
  toast('删除失败', 'error');
  return activeSessionId;
}

export function loadGenHistory(s: ConversationState): void {
  s.genHistory = loadGenHistoryFromStorage();
}

export async function loadSessionMessages(
  s: ConversationState,
  sessId: string,
  callbacks: {
    setMode: (m: Mode) => void;
    setAgentMode: (m: 'plan' | 'act') => void;
    setAutoBuildProjectId: (id: string) => void;
    setSelectedContextProject: (id: string) => void;
    loadProjectFiles: (id: string) => Promise<void>;
    setAllAgentSteps: (steps: AgentStep[]) => void;
    setAgentSteps: (steps: AgentStep[]) => void;
    setMaxRoundIndex: (n: number) => void;
    setSelectedRound: (n: number) => void;
    setMessages: (msgs: any[]) => void;
    setMessageUsages: (usages: Map<number, TokenUsage>) => void;
    setSessionId: (id: string) => void;
    setAgentStepsCollapsed: (v: boolean) => void;
    setAgentHadFinalAnswer: (v: boolean) => void;
    setStreaming: (v: boolean) => void;
    filterStepsByRound: (steps: AgentStep[], round: number) => AgentStep[];
  },
): Promise<void> {
  s.activeSessionId = sessId;
  callbacks.setStreaming(false);
  const result = await fetchSessionMessages(sessId, 50);
  if (!result) {
    toast('无法加载对话消息', 'error');
    return;
  }
  if (result.mode && modes.some(m => m.value === result.mode)) callbacks.setMode(result.mode as Mode);
  if (result.agent_mode === 'plan' || result.agent_mode === 'act') callbacks.setAgentMode(result.agent_mode);
  if (result.project_id) {
    callbacks.setAutoBuildProjectId(result.project_id);
    callbacks.setSelectedContextProject(result.project_id);
    await callbacks.loadProjectFiles(result.project_id);
  }
  const allSteps = result.allSteps as AgentStep[];
  callbacks.setAllAgentSteps(allSteps);
  callbacks.setMaxRoundIndex(result.maxRound);
  callbacks.setSelectedRound(result.maxRound);
  const latestSteps = callbacks.filterStepsByRound(allSteps, result.maxRound);
  callbacks.setAgentSteps(latestSteps);
  callbacks.setMessages(result.messages);
  s.hasMoreMessages = result.has_more;

  // Restore per-message token usage
  const restored = new Map<number, TokenUsage>();
  result.messages.forEach((m: any, i: number) => {
    if (m.token_usage) restored.set(i, m.token_usage);
  });
  if (restored.size > 0) callbacks.setMessageUsages(restored);
  callbacks.setSessionId(sessId);
  s.showHistorySidebar = false;
  callbacks.setAgentStepsCollapsed(true);
  callbacks.setAgentHadFinalAnswer(false);
  toast(`已加载对话 (${result.messages.length} 条消息${result.has_more ? '，可加载更早' : ''})`, 'success');
}

export async function loadEarlierMessages(
  s: ConversationState,
  sessionId: string,
  messages: any[],
  callbacks: {
    setMessages: (msgs: any[]) => void;
    setAllAgentSteps: (steps: AgentStep[]) => void;
  },
): Promise<void> {
  if (!sessionId || messages.length === 0 || s.loadingEarlier) return;
  const earliest = messages[0];
  if (!earliest.created_at) {
    toast('无法加载更早消息', 'error');
    return;
  }
  s.loadingEarlier = true;
  try {
    const result = await fetchSessionMessages(sessionId, 50, earliest.created_at, earliest.id);
    if (!result || result.messages.length === 0) {
      s.hasMoreMessages = false;
      return;
    }
    callbacks.setMessages([...result.messages, ...messages]);
    callbacks.setAllAgentSteps([...(result.allSteps as AgentStep[]), ...((callbacks as any)._allAgentSteps || [])]);
    s.hasMoreMessages = result.has_more;
  } catch {
    // silent
  } finally {
    s.loadingEarlier = false;
  }
}

export async function saveConversation(
  s: ConversationState,
  params: {
    messages: any[];
    mode: Mode;
    sessionId: string;
    selectedModelName: string;
    selectedModelID: string;
    autoBuildProjectId: string;
    modeIsAgent: boolean;
  },
): Promise<void> {
  if (params.messages.length === 0) return;
  s.convSaving = true;
  const convId = params.modeIsAgent
    ? (params.sessionId || s.activeSessionId || '')
    : (s.activeSessionId || '');
  const id = await saveConversationToBackend({
    id: convId,
    title: '',
    mode: params.mode,
    messages: params.messages,
    model: params.selectedModelName || params.selectedModelID || '',
    project_id: params.autoBuildProjectId || '',
  });
  if (id) {
    s.activeSessionId = id;
    toast('对话已保存', 'success');
    await loadConversations(s);
  } else {
    toast('保存失败', 'error');
  }
  s.convSaving = false;
}

export function exportConversation(messages: any[], format: 'json' | 'markdown'): void {
  exportConversationToFile(messages, format);
  toast(`已导出为 ${format === 'json' ? 'JSON' : 'Markdown'} 格式`, 'success');
}

export async function deployAutoBuild(autoBuildFiles: { path: string; content: string }[]): Promise<void> {
  if (autoBuildFiles.length === 0) {
    toast('没有可部署的文件', 'error');
    return;
  }
  const ok = await deployToAdb(autoBuildFiles.map(f => ({ path: f.path, content: f.content })));
  if (ok) toast('部署请求已发送', 'success');
  else toast('部署失败，请检查 ADB 连接', 'error');
}

export async function runComparison(
  providers: any[],
  comparisonInput: string,
): Promise<{ model: string; response: string; time: number }[]> {
  if (!comparisonInput.trim()) return [];
  const results: { model: string; response: string; time: number }[] = [];
  for (const p of providers) {
    for (const m of p.models.slice(0, 1)) {
      const start = Date.now();
      try {
        const token = localStorage.getItem('moduforge_token') || '';
        const res = await fetch('/api/v1/ai/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify({
            message: comparisonInput,
            provider_id: p.id,
            model: m.id,
            messages: [{ role: 'user', content: comparisonInput }],
          }),
        });
        const data = await res.json();
        results.push({
          model: `${p.name} / ${m.name}`,
          response: data.content || data.error || 'No response',
          time: Date.now() - start,
        });
      } catch {
        results.push({
          model: `${p.name} / ${m.name}`,
          response: 'Error',
          time: Date.now() - start,
        });
      }
    }
  }
  return results;
}

// ─── MCP Permission ───

export interface McpPermissionState {
  pendingPermission: {
    request_id: string;
    server: string;
    tool: string;
    args: Record<string, unknown>;
    timeout_s: number;
  } | null;
  permissionBusy: boolean;
}

function redactArgValues(v: unknown, depth = 0): unknown {
  if (depth > 4) return v;
  if (Array.isArray(v)) return v.map(x => redactArgValues(x, depth + 1));
  if (v && typeof v === 'object') {
    const out: Record<string, unknown> = {};
    for (const [k, val] of Object.entries(v as Record<string, unknown>)) {
      if (/token|secret|password|passwd|api[_-]?key|authorization|auth|credential|bearer|cookie/i.test(k)) {
        out[k] = typeof val === 'string' && val.length > 8 ? `${val.slice(0, 4)}***${val.slice(-2)}` : '***';
      } else {
        out[k] = redactArgValues(val, depth + 1);
      }
    }
    return out;
  }
  return v;
}

export function permissionArgsPreview(p: McpPermissionState): string {
  const req = p.pendingPermission;
  if (!req || !req.args || Object.keys(req.args).length === 0) return '（无参数）';
  try {
    return JSON.stringify(redactArgValues(req.args), null, 2);
  } catch {
    return String(req.args);
  }
}

export async function resolvePermission(
  p: McpPermissionState,
  allow: boolean,
): Promise<void> {
  const req = p.pendingPermission;
  if (!req) return;
  p.permissionBusy = true;
  try {
    const res = await fetch('/api/v1/agent/mcp/confirm', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${localStorage.getItem('moduforge_token') || ''}`,
      },
      body: JSON.stringify({ request_id: req.request_id, allow }),
    });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      toast(data.error || '确认请求失败（可能已超时）', 'error');
    } else {
      toast(
        allow ? `已允许调用 ${req.tool}` : `已拒绝调用 ${req.tool}`,
        allow ? 'success' : 'info',
      );
    }
  } catch (e: any) {
    toast(e.message || '确认请求失败', 'error');
  } finally {
    p.permissionBusy = false;
    p.pendingPermission = null;
  }
}

// ─── Project files ───

export async function loadProjectFilesStateFn(
  projectId: string,
  callbacks: {
    setAutoBuildFiles: (files: { path: string; content: string; size: number }[]) => void;
    setGeneratedFiles: (files: { path: string; content: string; oldContent?: string }[]) => void;
    setShowGeneratedFiles: (v: boolean) => void;
  },
): Promise<void> {
  try {
    const result = await loadProjectFilesState(projectId);
    callbacks.setAutoBuildFiles(result.autoBuildFiles);
    callbacks.setGeneratedFiles(result.generatedFiles);
    callbacks.setShowGeneratedFiles(true);
  } catch {
    // silent
  }
}

export async function loadContextProjectListStateFn(): Promise<{ id: string; name: string }[]> {
  try {
    const { loadContextProjectListState } = await import('../context');
    return await loadContextProjectListState();
  } catch {
    return [];
  }
}

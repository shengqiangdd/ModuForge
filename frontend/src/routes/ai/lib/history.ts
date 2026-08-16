import type { GenHistoryItem, SavedConv, AgentStep, Message } from './types';
import { MODES } from './types';

// ─── Generation History (localStorage) ───
export function loadGenHistory(): GenHistoryItem[] {
  try {
    const stored = localStorage.getItem('moduforge_ai_history');
    if (stored) return JSON.parse(stored);
  } catch {}
  return [];
}

export function saveGenHistory(genHistory: GenHistoryItem[]): void {
  try {
    localStorage.setItem('moduforge_ai_history', JSON.stringify(genHistory.slice(0, 30)));
  } catch {}
}

export function addGenHistory(
  title: string,
  mode: string,
  msgs: { role: string; content: string }[],
  selectedModelName: string,
  selectedModelID: string,
): GenHistoryItem | null {
  const lastUser = msgs.filter(m => m.role === 'user').slice(-1)[0];
  const lastAssistant = msgs.filter(m => m.role === 'assistant').slice(-1)[0];
  if (!lastAssistant) return null;
  const pair: { role: string; content: string }[] = [];
  if (lastUser) pair.push({ role: lastUser.role, content: lastUser.content });
  pair.push({ role: lastAssistant.role, content: lastAssistant.content });

  const item: GenHistoryItem = {
    id: Date.now().toString(36),
    title: title.slice(0, 60),
    timestamp: Date.now(),
    model: selectedModelName || selectedModelID || 'unknown',
    mode,
    messageCount: pair.length,
    preview: lastAssistant.content.slice(0, 100) || '',
    messages: pair,
  };
  return item;
}

// ─── Backend Conversations ───
async function authFetch(url: string, opts: RequestInit = {}): Promise<Response> {
  const token = localStorage.getItem('moduforge_token') || '';
  return fetch(url, { ...opts, headers: { 'Authorization': `Bearer ${token}`, ...opts.headers } });
}

export async function fetchConversations(): Promise<SavedConv[]> {
  try {
    const res = await authFetch('/api/v1/ai/conversations');
    if (res.ok) { const data = await res.json(); return data.conversations || []; }
  } catch {}
  return [];
}

export async function fetchConversation(id: string): Promise<any | null> {
  try {
    const res = await authFetch(`/api/v1/ai/conversations/${id}`);
    if (res.ok) return await res.json();
  } catch {}
  return null;
}

export async function deleteConversationById(id: string): Promise<boolean> {
  try {
    const res = await authFetch(`/api/v1/ai/conversations/${id}`, { method: 'DELETE' });
    return res.ok || res.status === 204;
  } catch { return false; }
}

export async function saveConversationToBackend(params: {
  id: string; title: string; mode: string; messages: Message[];
  model: string; project_id: string;
}): Promise<string | null> {
  try {
    const res = await authFetch('/api/v1/ai/conversations', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    });
    if (res.ok) { const data = await res.json(); return data.id || null; }
  } catch {}
  return null;
}

// ─── Sessions ───
export async function loadSessionsList(offset = 0, limit = 100): Promise<{ sessions: any[]; total: number }> {
  try {
    const token = localStorage.getItem('moduforge_token') || '';
    const res = await fetch(`/api/v1/ai/sessions?limit=${limit}&offset=${offset}`, {
      headers: { 'Authorization': `Bearer ${token}` },
    });
    if (res.ok) { const data = await res.json(); return { sessions: data.sessions || [], total: data.total ?? (data.sessions || []).length }; }
  } catch {}
  return { sessions: [], total: 0 };
}

export async function deleteSessionById(targetSessionId: string): Promise<boolean> {
  try {
    const res = await authFetch(`/api/v1/ai/sessions/${targetSessionId}`, { method: 'DELETE' });
    return res.ok;
  } catch { return false; }
}

export async function exportSessionById(targetSessionId: string, format: 'markdown' | 'json' = 'markdown'): Promise<boolean> {
  try {
    const res = await authFetch(`/api/v1/ai/sessions/${targetSessionId}/export?format=${format}`);
    if (!res.ok) throw new Error('export failed');
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `session-${targetSessionId.slice(0, 8)}.${format === 'json' ? 'json' : 'md'}`;
    a.click();
    URL.revokeObjectURL(url);
    return true;
  } catch { return false; }
}

export async function fetchSessionMessages(sessId: string, limit = 0, before = ''): Promise<{
  messages: Message[];
  allSteps: AgentStep[];
  maxRound: number;
  mode: string;
  agent_mode: string;
  project_id: string;
  has_more: boolean;
} | null> {
  try {
    const q = limit > 0 ? `?limit=${limit}${before ? `&before=${encodeURIComponent(before)}` : ''}` : '';
    const res = await authFetch(`/api/v1/ai/sessions/${sessId}/messages${q}`);
    if (!res.ok) return null;
    const data = await res.json();
    if (!data.messages || data.messages.length === 0) return null;

    const chatMsgs: Message[] = [];
    const allSteps: AgentStep[] = [];
    let maxRound = 0;
    for (const m of data.messages) {
      const ri = m.round_index || 0;
      if (m.step_type) {
        if (m.step_type === 'skill_call') {
          const colonIdx = m.content.indexOf(': ');
          const skill = colonIdx > 0 ? m.content.substring(0, colonIdx) : m.step_type;
          const jsonStr = colonIdx > 0 ? m.content.substring(colonIdx + 2) : '{}';
          let inputObj = {};
          try { inputObj = JSON.parse(jsonStr); } catch {}
          allSteps.push({ type: 'skill_call', skill, content: m.content, input: inputObj, round: ri });
        } else if (m.step_type === 'skill_result') {
          const colonIdx = m.content.indexOf(': ');
          const skill = colonIdx > 0 ? m.content.substring(0, colonIdx) : '';
          const resultContent = colonIdx > 0 ? m.content.substring(colonIdx + 2) : m.content;
          allSteps.push({ type: 'skill_result', skill, content: resultContent, round: ri });
        } else {
          allSteps.push({ type: m.step_type as any, content: m.content, round: ri });
        }
        if (ri > maxRound) maxRound = ri;
      } else {
        let tu: Message['token_usage'];
        if (m.role === 'assistant' && m.token_usage) {
          try { tu = typeof m.token_usage === 'string' ? JSON.parse(m.token_usage) : m.token_usage; } catch { tu = undefined; }
        }
        chatMsgs.push({ role: m.role, content: m.content, round: ri, created_at: m.created_at || '', token_usage: tu });
        if (m.role === 'user' && ri > maxRound) maxRound = ri;
      }
    }

    const latestReasoningSteps = allSteps.filter(s => s.type === 'think' && s.round === maxRound);
    if (latestReasoningSteps.length > 0 && chatMsgs.length > 0) {
      const lastAsstIdx = chatMsgs.map((m, i) => m.role === 'assistant' ? i : -1).filter(i => i >= 0).pop();
      if (lastAsstIdx !== undefined && lastAsstIdx >= 0) {
        chatMsgs[lastAsstIdx] = { ...chatMsgs[lastAsstIdx], reasoning: latestReasoningSteps.map(s => s.content).join('\n') };
      }
    }

    return {
      messages: chatMsgs, allSteps: allSteps as AgentStep[], maxRound,
      mode: data.mode || '', agent_mode: data.agent_mode || '', project_id: data.project_id || '',
      has_more: !!data.has_more,
    };
  } catch { return null; }
}

// ─── Export conversation to file ───
export function exportConversationToFile(messages: Message[], format: 'json' | 'markdown'): void {
  const data = format === 'json'
    ? JSON.stringify(messages, null, 2)
    : messages.map(m => `**${m.role === 'user' ? '用户' : 'AI'}**\n\n${m.content}`).join('\n\n---\n\n');
  const blob = new Blob([data], { type: format === 'json' ? 'application/json' : 'text/markdown' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `conversation_${Date.now()}.${format === 'json' ? 'json' : 'md'}`;
  a.click();
  URL.revokeObjectURL(url);
}

// ─── Search sessions ───
export async function searchSessions(query: string): Promise<{ session_id: string; role: string; content: string; step_type: string; created_at: string }[]> {
  try {
    const res = await authFetch(`/api/v1/ai/sessions/search?q=${encodeURIComponent(query)}`);
    if (res.ok) { const data = await res.json(); return data.results || []; }
  } catch {}
  return [];
}

// ─── Project files ───
export async function fetchProjectFiles(projectId: string): Promise<{ path: string; content: string; size: number }[]> {
  try {
    const res = await authFetch(`/api/v1/projects/${projectId}/files`);
    if (res.ok) { const data = await res.json(); return data.files || []; }
  } catch {}
  return [];
}

export async function fetchProjectList(): Promise<{ id: string; name: string }[]> {
  try {
    const res = await authFetch('/api/v1/projects');
    if (res.ok) { const data = await res.json(); return (data.projects || []).map((p: { id: string; name: string }) => ({ id: p.id, name: p.name })); }
  } catch {}
  return [];
}

// ─── Deploy ───
export async function deployToAdb(files: { path: string; content: string }[]): Promise<boolean> {
  try {
    const token = localStorage.getItem('moduforge_token') || '';
    const res = await fetch('/api/v1/adb/install', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ files }),
    });
    return res.ok;
  } catch { return false; }
}

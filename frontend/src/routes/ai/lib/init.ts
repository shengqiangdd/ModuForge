// ─── Initialization logic — extracted from +page.svelte ───
import { client } from '$lib/api/client';
import { initMarkdownWorker, terminateMarkdownWorker, setupCopyCode } from './markdown';
import { generateUUID } from './utils';
import type { StreamHandler } from './stream-handler';
import * as cb from './callbacks';

export function setupOnInit(handler: StreamHandler) {
  setupCopyCode();
  initMarkdownWorker();
  handler.setupEventListeners();
  handler.startElapsedTimer();
}

export async function loadInitialData(opts: {
  loadProviders: () => Promise<void>;
  loadSessions: () => Promise<void>;
  loadConversations: () => Promise<void>;
  loadGenHistory: () => void;
  loadContextProjectList: () => Promise<void>;
  loadSessionMessages: (id: string) => Promise<void>;
  activeSessionId: string;
  sessions: any[];
  messages: any[];
  updateState: (partial: Record<string, any>) => void;
}) {
  const result = await cb.loadProviders();
  opts.updateState({
    providers: result.providers,
    selectedProviderID: result.selectedProviderID,
    selectedModelID: result.selectedModelID,
    configLoaded: true,
  });
  await opts.loadSessions();
  await opts.loadConversations();
  opts.loadGenHistory();
  await opts.loadContextProjectList();

  const activeId = opts.activeSessionId || (opts.updateState as any)._activeSessionId;
  if (activeId) {
    await opts.loadSessionMessages(activeId);
  } else if (opts.sessions.length > 0 && opts.messages.length === 0) {
    const latest = opts.sessions[0];
    if (latest && latest.msg_count > 0) await opts.loadSessionMessages(latest.session_id);
  }
}

export function loadMcpToolCount(updateState: (partial: Record<string, any>) => void) {
  (async () => {
    try {
      const data = await client.get<{ servers: { tools?: unknown[] }[] }>('/agent/mcp/status');
      const count = (data.servers || []).reduce((acc, srv) => acc + (srv.tools?.length || 0), 0);
      updateState({ mcpToolCount: count });
    } catch { /* 静默失败 */ }
  })();
}

export function setupCleanup(handler: StreamHandler) {
  terminateMarkdownWorker();
  handler.cleanup();
  if (typeof window !== 'undefined') delete (window as any).copyCode;
}

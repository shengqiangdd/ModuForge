// ─── Stream event handler — extracted from +page.svelte ───
// Encapsulates SSE stream processing, agent step parsing, and
// auto-build event handling. Accessed via state getters/setters.

import type { Message, AgentStep, AutoBuildPhase, ProgressStepDetail, Mode, GenHistoryItem, Subtask } from './types';
import { AUTO_BUILD_PHASE_DEFS, PROGRESS_LABELS } from './types';
import { parseSSEBuffer, analyzeProgressFromContent, updateProgressDetails, buildToolLabel, buildResultPreview, resolveStepIndex, mapAutoBuildPhaseToStep, StreamBatchManager, ProgressUpdateManager, SafetyTimerManager, AgentStepBatcher } from './streaming';
import { addGenHistory as createGenHistoryItemFn, saveGenHistory as saveGenHistoryToStorage, saveConversationToBackend } from './history';
import { extractGatherSpec } from './utils';
import { streamRequest } from '$lib/api/client';

export interface StreamHandlerState {
  messages: Message[];
  streaming: boolean;
  configLoaded: boolean;
  currentStepIndex: number;
  progressStepDetails: ProgressStepDetail[];
  agentSteps: AgentStep[];
  allAgentSteps: AgentStep[];
  agentStepsCollapsed: boolean;
  selectedRound: number;
  maxRoundIndex: number;
  expandedReasoning: Set<number>;
  messageUsages: Map<number, { prompt_tokens: number; completion_tokens: number; total_tokens: number }>;
  messageTimes: Map<number, number>;
  requestStartTime: number;
  lastStreamAssistantIdx: number;
  seenReadPaths: Set<string>;
  currentToolInput: Record<string, unknown> | null;
  agentHadFinalAnswer: boolean;
  subtasks: Subtask[];
  autoBuildPhases: AutoBuildPhase[];
  autoBuildFiles: { path: string; content: string; size: number }[];
  autoBuildProjectId: string;
  autoBuildProjectName: string;
  stepStartTime: number;
  stepElapsed: string;
  genHistory: GenHistoryItem[];
  gatheredSpec: Record<string, unknown> | null;
  showSpecCard: boolean;
  showGeneratedFiles: boolean;
  generatedFiles: { path: string; content: string; oldContent?: string }[];
  buildLog: string;
  input: string;
  mode: Mode;
  selectedProviderID: string;
  selectedModelID: string;
  selectedModel: { name: string; id: string } | null;
  selectedContextProject: string;
  projectContext: string;
  agentMode: 'plan' | 'act';
  sessionId: string;
  activeSessionId: string;
  showHistorySidebar: boolean;
  providers: { id: string; name: string; endpoint: string; models: { id: string; name: string }[] }[];
}

export interface StreamHandlerCallbacks {
  loadProjectFiles: (projectId: string) => Promise<void>;
  loadConversations: () => Promise<void>;
  loadGenHistory: () => Promise<void>;
  saveConfigToBackend: (providerId: string, modelId: string) => Promise<void>;
  scrollToBottom: () => void | Promise<void>;
  toast: (msg: string, type: 'success' | 'error' | 'warning' | 'info') => void;
}

export class StreamHandler {
  private state: StreamHandlerState;
  private cb: StreamHandlerCallbacks;
  private streamBatchMgr: StreamBatchManager | null = null;
  private progressUpdateMgr: ProgressUpdateManager;
  private safetyTimerMgr: SafetyTimerManager;
  private agentStepBatcher: AgentStepBatcher | null = null;
  private streamCtrl: AbortController | null = null;
  private sseLineBuffer = '';
  private stepElapsedTimer: ReturnType<typeof setInterval> | null = null;
  private autoSaveTimer: ReturnType<typeof setTimeout> | null = null;
  private progressSteps: string[] = Object.values(PROGRESS_LABELS);

  constructor(state: StreamHandlerState, callbacks: StreamHandlerCallbacks) {
    this.state = state;
    this.cb = callbacks;
    this.progressUpdateMgr = new ProgressUpdateManager((content: string) => {
      const stepIdx = analyzeProgressFromContent(content);
      if (stepIdx >= 0) state.currentStepIndex = Math.max(state.currentStepIndex, stepIdx);
    });
    this.safetyTimerMgr = new SafetyTimerManager();
  }

  // ─── Agent step batcher ───
  private ensureAgentStepBatcher(): AgentStepBatcher {
    if (!this.agentStepBatcher) {
      this.agentStepBatcher = new AgentStepBatcher(this.state.allAgentSteps, (steps) => { this.state.agentSteps = steps; });
    }
    return this.agentStepBatcher;
  }

  private pushAgentStep(step: AgentStep) {
    this.ensureAgentStepBatcher().push(step);
  }

  private ensureStreamBatchMgr(): StreamBatchManager {
    if (!this.streamBatchMgr) {
      this.streamBatchMgr = new StreamBatchManager(
        () => this.state.messages,
        (msgs) => { this.state.messages = msgs; },
        () => this.state.lastStreamAssistantIdx,
      );
    }
    return this.streamBatchMgr;
  }

  private appendStreamContent(text: string) { this.ensureStreamBatchMgr().appendContent(text); }
  private appendStreamReasoning(text: string) { this.ensureStreamBatchMgr().appendReasoning(text); }

  private ensureAgentAssistantMsg(): number {
    const msgs = this.state.messages;
    if (msgs.length > 0 && msgs[msgs.length - 1].role === 'assistant') return msgs.length - 1;
    this.state.messages = [...msgs, { role: 'assistant', content: '' }];
    return this.state.messages.length - 1;
  }

  private createNewAssistantMsg(): number {
    this.state.messages = [...this.state.messages, { role: 'assistant', content: '' }];
    return this.state.messages.length - 1;
  }

  // ─── Agent step parsing ───
  private parseAgentStep(parsed: Record<string, unknown>): boolean {
    const s = this.state;
    if (parsed.type === 'round_sync') {
      const backendRound = (parsed.round as number) ?? 0;
      const backendMaxRound = (parsed.max_round as number) ?? backendRound;
      if (backendRound >= 0) {
        if (s.messages.length > 0) {
          const lastUserIdx = s.messages.map((m, i) => m.role === 'user' ? i : -1).filter(i => i >= 0).pop();
          if (lastUserIdx !== undefined && lastUserIdx >= 0) {
            s.messages[lastUserIdx] = { ...s.messages[lastUserIdx], round: backendRound };
            s.messages = [...s.messages];
          }
        }
        s.maxRoundIndex = Math.max(s.maxRoundIndex, backendMaxRound);
        s.selectedRound = backendRound;
      }
      return true;
    }
    if (parsed.type === 'step') {
      const step = parsed.step as string;
      const currentRound = s.selectedRound;
      if (step === 'think') {
        this.pushAgentStep({ type: 'think', content: (parsed.content || parsed.message || '') as string, round: currentRound });
      } else if (step === 'skill_call') {
        this.pushAgentStep({ type: 'skill_call', skill: parsed.skill as string, input: parsed.input as Record<string, unknown> | undefined, round: currentRound });
        s.currentToolInput = (parsed.input as Record<string, unknown>) || null;
        const idx = this.ensureAgentAssistantMsg();
        s.messages[idx] = { ...s.messages[idx], content: buildToolLabel(parsed.skill as string, (parsed.input as Record<string, unknown> | undefined)?.path as string | undefined) };
        s.messages = [...s.messages];
      } else if (step === 'skill_result') {
        this.pushAgentStep({ type: 'skill_result', skill: parsed.skill as string, content: parsed.content as string, round: currentRound });
        if (parsed.skill === 'read_file' && s.currentToolInput?.path) {
          if (s.seenReadPaths.has(s.currentToolInput.path as string)) { s.currentToolInput = null; return true; }
          s.seenReadPaths.add(s.currentToolInput.path as string);
        }
        s.currentToolInput = null;
        const idx = this.ensureAgentAssistantMsg();
        const preview = buildResultPreview(parsed.content as string);
        s.messages[idx] = { ...s.messages[idx], content: `⚙️ ${parsed.skill} 完成${preview ? ': ' + preview : ''}` };
        s.messages = [...s.messages];
      } else if (step === 'answer') {
        if (s.agentSteps.length > 0 && s.agentSteps[s.agentSteps.length - 1].type === 'think') s.agentSteps = s.agentSteps.slice(0, -1);
        this.pushAgentStep({ type: 'answer', content: parsed.content as string, round: currentRound });
        s.seenReadPaths = new Set();
        const isMaxIterError = parsed.content && (parsed.content as string).includes('超出最大迭代次数');
        if (!isMaxIterError) {
          const idx = this.createNewAssistantMsg();
          s.messages[idx] = { ...s.messages[idx], content: parsed.content as string };
          s.messages = [...s.messages];
        } else { s.agentHadFinalAnswer = true; }
      } else if (s.mode === 'generate' || s.mode === 'auto-build') {
        const stepIndex = resolveStepIndex(step);
        if (stepIndex >= 0) {
          s.currentStepIndex = stepIndex;
          s.progressStepDetails = updateProgressDetails(s.progressStepDetails, step, parsed.message as string);
        }
      }
      return true;
    }
    if (parsed.type === 'checkpoint') {
      this.pushAgentStep({ type: 'skill_result', skill: 'checkpoint', content: `📝 文件已修改: ${parsed.path || 'unknown'} (可回滚 #${parsed.checkpoint || 0})`, round: s.selectedRound });
      return true;
    }
    if (parsed.type === 'project_created') {
      if (parsed.project_id) { s.selectedContextProject = parsed.project_id as string; this.cb.loadProjectFiles(parsed.project_id as string); this.cb.toast(`📁 项目已创建: ${(parsed.project_id as string).slice(0, 8)}…`, 'success'); }
      return true;
    }
    if (parsed.type === 'step' && parsed.step === 'compact') {
      this.pushAgentStep({ type: 'think', content: `🗜️ ${parsed.content || '上下文已压缩'}`, round: s.selectedRound });
      return true;
    }
    if (parsed.type === 'step' && parsed.step === 'task_plan') {
      if (parsed.subtasks && Array.isArray(parsed.subtasks)) {
        s.subtasks = parsed.subtasks.map((st: Record<string, unknown>) => ({
          id: st.id as string,
          description: st.description as string,
          status: (st.status as Subtask['status']) || 'pending',
          dependencies: (st.dependencies as string[]) || [],
          files: (st.files as string[]) || [],
          progress: (st.progress as number) || 0,
          started_at: st.started_at as string | undefined,
          completed_at: st.completed_at as string | undefined,
          retry_count: (st.retry_count as number) || 0,
        }));
      }
      this.pushAgentStep({ type: 'think', content: `📋 ${parsed.content || '任务分解完成'}`, round: s.selectedRound });
      return true;
    }
    if (parsed.type === 'step' && parsed.step === 'task_progress') {
      const subtaskId = parsed.subtask_id as string | undefined;
      const status = parsed.status as string | undefined;
      if (subtaskId && status) {
        s.subtasks = s.subtasks.map(st => {
          if (st.id === subtaskId) {
            return { ...st, status: status as Subtask['status'], progress: (parsed.progress as number) ?? st.progress, description: (parsed.description as string) || st.description };
          }
          return st;
        });
      }
      return true;
    }
    return false;
  }

  // ─── Auto-build event handling ───
  private handleAutoBuildEvent(parsed: Record<string, unknown>) {
    const s = this.state;
    if (parsed.type === 'phase') {
      s.autoBuildPhases = s.autoBuildPhases.map(p => {
        if (p.phase === parsed.phase) return { ...p, message: parsed.message as string, status: 'running' as const };
        if (p.status === 'running') return { ...p, status: 'done' as const };
        return p;
      });
      s.stepStartTime = Date.now();
      const mappedStep = mapAutoBuildPhaseToStep(parsed.phase as string);
      if (mappedStep) {
        const stepIndex = resolveStepIndex(mappedStep);
        if (stepIndex >= 0) s.currentStepIndex = stepIndex;
        s.progressStepDetails = updateProgressDetails(s.progressStepDetails, mappedStep, parsed.message as string);
      }
    } else if (parsed.type === 'complete') {
      s.streaming = false;
      s.autoBuildPhases = s.autoBuildPhases.map(p => ({ ...p, status: 'done' as const }));
      if (s.requestStartTime > 0) { const elapsed = Date.now() - s.requestStartTime; const msgIdx = s.messages.length - 1; if (msgIdx >= 0) { s.messageTimes.set(msgIdx, elapsed); s.messageTimes = s.messageTimes; } }
      if (parsed.project_id) s.autoBuildProjectId = parsed.project_id as string;
      if (parsed.project_name) s.autoBuildProjectName = parsed.project_name as string;
      if (parsed.files && Array.isArray(parsed.files)) {
        s.showGeneratedFiles = true;
        const fileListStr = (parsed.files as Array<Record<string, unknown>>).map(f => '- ' + f.path + ' (' + (f.size || 0) + ' bytes)').join('\n');
        s.messages = [...s.messages, { role: 'assistant', content: '✅ **模块开发完成！** 共生成 ' + (parsed.files as Array<unknown>).length + ' 个文件。\n\n文件列表：\n' + fileListStr }];
        if (parsed.project_id) {
          const token = localStorage.getItem('moduforge_token') || '';
          fetch(`/api/v1/projects/${parsed.project_id}/files`, { headers: { 'Authorization': `Bearer ${token}` } })
            .then(res => res.ok ? res.json() : null)
            .then(data => { if (data?.files) { s.autoBuildFiles = data.files; s.generatedFiles = data.files.map((f: Record<string, unknown>) => ({ path: f.path as string, content: f.content as string })); } })
            .catch(e => console.error('Failed to fetch project files:', e));
        }
      } else { s.messages = [...s.messages, { role: 'assistant', content: '✅ **模块开发完成！**' }]; }
    } else if (parsed.type === 'error') {
      s.streaming = false;
      s.autoBuildPhases = s.autoBuildPhases.map(p => p.status === 'running' ? { ...p, status: 'error' as const } : p);
      s.messages = [...s.messages, { role: 'assistant', content: `❌ **构建失败**\n\n${parsed.error}` }];
      this.cb.toast((parsed.error as string) || '构建失败', 'error');
    } else if (parsed.type === 'usage' && parsed.usage) {
      const msgIdx = s.messages.length - 1;
      if (msgIdx >= 0) { s.messageUsages.set(msgIdx, parsed.usage as { prompt_tokens: number; completion_tokens: number; total_tokens: number }); s.messageUsages = s.messageUsages; }
    } else if (parsed.type === 'reasoning') {
      if (s.messages.length === 0 || s.messages[s.messages.length - 1].role !== 'assistant') s.messages = [...s.messages, { role: 'assistant', content: '', reasoning: '' }];
      const lastIdx = s.messages.length - 1;
      s.messages[lastIdx] = { ...s.messages[lastIdx], reasoning: (s.messages[lastIdx].reasoning || '') + parsed.content };
      s.messages = [...s.messages];
    }
  }

  // ─── Stream event handlers ───
  onStreamData = (e: Event) => {
    const s = this.state;
    const safetyMs = (s.mode === 'agent' || s.mode === 'auto-build') ? 1800000 : 60000;
    this.safetyTimerMgr.start(safetyMs, () => {
      if (s.streaming) { s.streaming = false; s.messages = [...s.messages, { role: 'assistant', content: '⏱️ **连接超时**\n\n后端 30 分钟内无数据。请检查 Agent 是否仍在运行。' }]; this.cb.toast('AI 连接超时', 'error'); }
    });
    const detail = (e as CustomEvent).detail as string;
    const { dataChunks, leftover, done } = parseSSEBuffer(this.sseLineBuffer, detail);
    this.sseLineBuffer = leftover;
    for (const data of dataChunks) {
      try {
        const parsed = JSON.parse(data);
        if (s.mode === 'agent' && this.parseAgentStep(parsed)) continue;
        if (s.mode === 'auto-build') { this.handleAutoBuildEvent(parsed); continue; }
        if (parsed.type === 'step') {
          if (s.mode === 'generate' || s.mode === 'auto-build') {
            const stepIndex = resolveStepIndex(parsed.step as string);
            if (stepIndex >= 0) {
              s.currentStepIndex = stepIndex;
              s.progressStepDetails = updateProgressDetails(s.progressStepDetails, parsed.step as string, parsed.message as string);
            }
          }
          if (parsed.step === 'task_plan' && parsed.subtasks && Array.isArray(parsed.subtasks)) {
            s.subtasks = parsed.subtasks.map((st: Record<string, unknown>) => ({
              id: st.id as string, description: st.description as string, status: (st.status as Subtask['status']) || 'pending',
              dependencies: (st.dependencies as string[]) || [], files: (st.files as string[]) || [],
              progress: (st.progress as number) || 0, started_at: st.started_at as string | undefined,
              completed_at: st.completed_at as string | undefined, retry_count: (st.retry_count as number) || 0,
            }));
          }
          if (parsed.step === 'task_progress' && parsed.subtask_id) {
            s.subtasks = s.subtasks.map(st => {
              if (st.id === parsed.subtask_id) return { ...st, status: (parsed.status as string) || st.status, progress: (parsed.progress as number) ?? st.progress };
              return st;
            });
          }
          return;
        }
        if (parsed.type === 'reasoning') {
          if (s.lastStreamAssistantIdx < 0 || s.lastStreamAssistantIdx >= s.messages.length || s.messages[s.lastStreamAssistantIdx]?.role !== 'assistant') {
            s.messages = [...s.messages, { role: 'assistant', content: '', reasoning: '' }];
            s.lastStreamAssistantIdx = s.messages.length - 1;
          }
          this.appendStreamReasoning(parsed.content as string);
          return;
        }
        if (parsed.type === 'error' || parsed.error) {
          s.streaming = false; s.currentStepIndex = -1;
          s.messages = [...s.messages, { role: 'assistant', content: `❌ **AI 错误**\n\n${parsed.error || '未知错误'}` }];
          this.cb.toast(((parsed.error as string) || '未知错误').slice(0, 60), 'error');
          return;
        }
        if (parsed.type === 'usage' && parsed.usage) {
          const msgIdx = s.messages.length - 1;
          if (msgIdx >= 0) { s.messageUsages.set(msgIdx, parsed.usage as { prompt_tokens: number; completion_tokens: number; total_tokens: number }); s.messageUsages = s.messageUsages; }
          return;
        }
        if (s.mode === 'agent' && (parsed.type === 'stream_delta' || parsed.type === 'reasoning')) {
          let content = (parsed.content as string) || (parsed.choices?.[0]?.delta?.content as string) || '';
          if (content && content.trim()) {
            const lastIdx = s.agentSteps.length - 1;
            if (lastIdx >= 0 && s.agentSteps[lastIdx].type === 'think') {
              s.agentSteps[lastIdx] = { ...s.agentSteps[lastIdx], content: s.agentSteps[lastIdx].content + content };
              s.agentSteps = s.agentSteps;
            } else { this.pushAgentStep({ type: 'think', content }); }
          }
          return;
        }
        let content = (parsed.content as string) || (parsed.choices?.[0]?.delta?.content as string) || '';
        let reasoning = (parsed.choices?.[0]?.delta?.reasoning_content as string) || '';
        if (reasoning) { this.appendStreamReasoning(reasoning); }
        if (content) {
          if (s.mode === 'auto-build') return;
          if (s.messages.length > 0 && s.messages[s.messages.length - 1].role === 'assistant') s.lastStreamAssistantIdx = s.messages.length - 1;
          else { s.messages = [...s.messages, { role: 'assistant', content: '' }]; s.lastStreamAssistantIdx = s.messages.length - 1; }
          this.appendStreamContent(content);
          this.progressUpdateMgr.append(content);
        }
      } catch {
        if (s.mode === 'auto-build') return;
        if (s.messages.length > 0 && s.messages[s.messages.length - 1].role === 'assistant') { s.messages[s.messages.length - 1].content += data; s.messages = [...s.messages]; }
        else s.messages = [...s.messages, { role: 'assistant', content: data }];
        this.progressUpdateMgr.append(data);
      }
    }
    if (done) { s.streaming = false; this.sseLineBuffer = ''; return; }
  };

  onTimeout = () => {
    const s = this.state;
    this.safetyTimerMgr.stop();
    s.streaming = false; s.currentStepIndex = -1;
    const hint = s.mode === 'auto-build'
      ? '⏱️ **智能构建超时**（超过 10 分钟无响应）\n\n可能原因：\n1. LLM 生成复杂模块耗时过长\n2. 模型响应太慢（试试换一个更快的模型）\n3. 网络连接不稳定\n\n建议：切换到更快的模型重试'
      : '⏱️ **请求超时**（长时间无响应）\n\n建议：切换到免费模型重试，或在设置中检查 LLM 配置。';
    s.messages = [...s.messages, { role: 'assistant', content: hint }];
    this.cb.toast('AI 请求超时', 'error');
  };

  onStreamError = (e: Event) => {
    const s = this.state;
    const detail = ((e as CustomEvent).detail as string) || '未知错误';
    this.safetyTimerMgr.stop();
    s.streaming = false; s.currentStepIndex = -1;
    s.messages = [...s.messages, { role: 'assistant', content: `❌ **AI 错误**\n\n${detail}` }];
    this.cb.toast(detail, 'error');
  };

  onStreamDone = () => {
    const s = this.state;
    this.safetyTimerMgr.stop();
    if (this.sseLineBuffer.trim()) {
      const leftover = this.sseLineBuffer; this.sseLineBuffer = '';
      if (leftover.startsWith('data: ')) {
        const data = leftover.slice(6);
        if (data !== '[DONE]') { try { const parsed = JSON.parse(data); if (s.mode === 'agent') this.parseAgentStep(parsed); } catch { /* ignore */ } }
      }
    } else { this.sseLineBuffer = ''; }
    if (this.streamBatchMgr) this.streamBatchMgr.cancel();
    this.progressUpdateMgr.flush();

    if (s.streaming) {
      s.streaming = false;
      if (s.mode !== 'agent') s.currentStepIndex = Math.max(s.currentStepIndex, 4);
      if (s.requestStartTime > 0) {
        const elapsed = Date.now() - s.requestStartTime;
        const msgIdx = s.messages.length - 1;
        if (msgIdx >= 0) { s.messageTimes.set(msgIdx, elapsed); s.messageTimes = s.messageTimes; }
        s.requestStartTime = 0;
      }
      const lastAssistant = s.messages.filter(m => m.role === 'assistant').slice(-1)[0];
      if (lastAssistant && lastAssistant.content) {
        const item = createGenHistoryItemFn(lastAssistant.content.slice(0, 60), s.mode, s.messages, s.selectedModel?.name || '', s.selectedModelID);
        if (item) { s.genHistory = [item, ...s.genHistory].slice(0, 50); saveGenHistoryToStorage(s.genHistory); }
      }
      if (s.mode === 'gather' && lastAssistant) {
        const spec = extractGatherSpec(lastAssistant.content);
        if (spec) { s.gatheredSpec = spec; s.showSpecCard = true; }
      }
      this.autoSaveConversation();
      if (this.stepElapsedTimer) { clearInterval(this.stepElapsedTimer); this.stepElapsedTimer = null; }
    }
  };

  // ─── Send ───
  send = async (skipAddUserMsg = false) => {
    const s = this.state;
    const text = s.input.trim();
    if (!text && !skipAddUserMsg) return;
    if (s.streaming) return;
    if (!s.configLoaded) { this.cb.toast('AI 配置加载中...', 'warning'); return; }
    if (!s.selectedProviderID || !s.selectedModelID) { this.cb.toast('请先选择 AI 模型', 'error'); return; }
    if (s.mode === 'agent' && !s.selectedContextProject) {
      this.cb.toast('💡 建议先选择一个项目上下文，Agent 将更精准地操作文件。', 'info');
    }
    s.input = '';
    s.agentSteps = []; s.seenReadPaths = new Set(); s.currentToolInput = null;
    s.agentStepsCollapsed = true; s.agentHadFinalAnswer = false;
    s.expandedReasoning = new Set();
    s.autoBuildPhases = AUTO_BUILD_PHASE_DEFS.map(p => ({ phase: p.phase, message: p.label, status: 'pending' as const }));
    s.autoBuildFiles = []; s.autoBuildProjectId = ''; s.autoBuildProjectName = '';
    if (!skipAddUserMsg) {
      s.messages = [...s.messages, { role: 'user', content: text, round: s.maxRoundIndex + 1 }];
      s.maxRoundIndex++; s.selectedRound = s.maxRoundIndex;
    }
    s.streaming = true;
    s.requestStartTime = Date.now(); this.sseLineBuffer = '';
    this.progressSteps = Object.values(PROGRESS_LABELS); s.currentStepIndex = 0; s.progressStepDetails = [];
    await this.cb.saveConfigToBackend(s.selectedProviderID, s.selectedModelID);

    let body: Record<string, unknown> = { messages: s.messages, session_id: s.sessionId, provider: s.selectedProviderID || '', model: s.selectedModelID || '' };
    let path: string;
    if (s.projectContext.trim()) body.project_context = s.projectContext.trim();
    if (s.selectedContextProject) body.project_id = s.selectedContextProject;
    if (s.mode === 'agent') {
      path = '/agent/run';
      body = { task: text, session_id: s.sessionId, messages: s.messages, provider_id: s.selectedProviderID || '', model: s.selectedModelID || '', project_id: s.selectedContextProject || '', project_context: s.projectContext.trim() || '', agent_mode: s.agentMode };
    } else if (s.mode === 'auto-build') { path = '/ai/auto-build'; body = { description: text, session_id: s.sessionId, project_id: s.autoBuildProjectId || '', provider: s.selectedProviderID || '', model: s.selectedModelID || '' }; }
    else if (s.mode === 'gather') { body.message = text; body.provider = s.selectedProviderID || ''; body.model = s.selectedModelID || ''; path = '/ai/gather'; }
    else if (s.mode === 'generate') { body.description = text; body.provider = s.selectedProviderID || ''; body.model = s.selectedModelID || ''; path = '/ai/generate'; }
    else if (s.mode === 'repair') { body.build_log = s.buildLog || text; body.provider = s.selectedProviderID || ''; body.model = s.selectedModelID || ''; path = '/ai/repair'; }
    else { body.message = text; body.provider = s.selectedProviderID || ''; body.model = s.selectedModelID || ''; path = '/ai/chat'; }

    await this.cb.scrollToBottom();
    this.streamCtrl = streamRequest(path, body, (s.mode === 'auto-build' || s.mode === 'agent') ? 1800000 : undefined);
    const safetyMs = (s.mode === 'agent' || s.mode === 'auto-build') ? 1800000 : 60000;
    this.safetyTimerMgr.start(safetyMs, () => {
      if (s.streaming) {
        s.streaming = false;
        s.messages = [...s.messages, { role: 'assistant', content: '⏱️ **连接超时**\n\n后端 60 秒内无数据。' }];
        this.cb.toast('AI 连接超时', 'error');
        this.autoSaveConversation();
      }
    });
  };

  // ─── Stop stream ───
  stopStream = () => {
    const s = this.state;
    this.streamCtrl?.abort();
    this.safetyTimerMgr.stop();
    s.streaming = false; s.currentStepIndex = -1; this.progressSteps = []; s.progressStepDetails = [];
    s.expandedReasoning = new Set(); s.agentSteps = []; s.agentHadFinalAnswer = false;
    s.lastStreamAssistantIdx = -1; this.sseLineBuffer = '';
    if (this.streamBatchMgr) this.streamBatchMgr.cancel();
    this.progressUpdateMgr.reset();
    if (s.mode === 'auto-build') s.autoBuildPhases = [];
  };

  // ─── Auto-save ───
  autoSaveConversation() {
    const s = this.state;
    if (s.messages.length === 0 || !s.activeSessionId) return;
    if (this.autoSaveTimer) clearTimeout(this.autoSaveTimer);
    this.autoSaveTimer = setTimeout(() => {
      if (s.mode === 'agent' && s.messages.length > 0) {
        saveConversationToBackend({
          id: s.activeSessionId || s.sessionId, title: '', mode: s.mode, messages: s.messages,
          model: s.selectedModel?.name || s.selectedModelID || '',
          project_id: s.autoBuildProjectId || '',
        });
      }
    }, 2000);
  }

  // ─── Lifecycle ───
  setupEventListeners() {
    window.addEventListener('ai-stream', this.onStreamData);
    window.addEventListener('ai-stream-done', this.onStreamDone);
    window.addEventListener('ai-stream-timeout', this.onTimeout);
    window.addEventListener('ai-stream-error', this.onStreamError);
  }

  removeEventListeners() {
    window.removeEventListener('ai-stream', this.onStreamData);
    window.removeEventListener('ai-stream-done', this.onStreamDone);
    window.removeEventListener('ai-stream-timeout', this.onTimeout);
    window.removeEventListener('ai-stream-error', this.onStreamError);
  }

  startElapsedTimer() {
    const s = this.state;
    this.stepElapsedTimer = setInterval(() => {
      if (s.streaming && s.autoBuildPhases.some(p => p.status === 'running')) {
        const secs = Math.floor((Date.now() - s.stepStartTime) / 1000);
        s.stepElapsed = secs >= 60 ? `${Math.floor(secs / 60)}m${secs % 60}s` : `${secs}s`;
      }
    }, 1000);
  }

  stopElapsedTimer() {
    if (this.stepElapsedTimer) { clearInterval(this.stepElapsedTimer); this.stepElapsedTimer = null; }
  }

  cleanup() {
    this.removeEventListeners();
    this.stopElapsedTimer();
    if (this.streamBatchMgr) this.streamBatchMgr.cancel();
    if (this.agentStepBatcher) this.agentStepBatcher.cancel();
    this.progressUpdateMgr.reset();
    if (this.autoSaveTimer) { clearTimeout(this.autoSaveTimer); this.autoSaveTimer = null; }
  }
}
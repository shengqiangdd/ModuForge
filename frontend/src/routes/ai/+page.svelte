<script lang="ts">
import { onMount, onDestroy, tick } from 'svelte';
import { toast } from '$lib/stores/toast.svelte';
import { client, computeDiff } from '$lib/api/client';
import ChatSidebar from './components/ChatSidebar.svelte';
import ChatMessage from './components/ChatMessage.svelte';
import ChatInput from './components/ChatInput.svelte';
import AgentSteps from './components/AgentSteps.svelte';
import ModelSelector from './components/ModelSelector.svelte';
import CompactToolbar from './components/CompactToolbar.svelte';
import ProgressIndicator from './components/ProgressIndicator.svelte';
import AutoBuildProjectCard from './components/AutoBuildProjectCard.svelte';
import GatherSpecCard from './components/GatherSpecCard.svelte';
import GeneratedFilesPanel from './components/GeneratedFilesPanel.svelte';
import ProjectContextPanel from './components/ProjectContextPanel.svelte';
import BuildProgressBar from './components/BuildProgressBar.svelte';
import ProviderConfigModal from './components/modals/ProviderConfigModal.svelte';
import ImportDialogModal from './components/modals/ImportDialogModal.svelte';
import SecurityWarningModal from './components/modals/SecurityWarningModal.svelte';
import DeleteConfirmModal from './components/modals/DeleteConfirmModal.svelte';
import PreviewModal from './components/modals/PreviewModal.svelte';
import ComparisonModal from './components/modals/ComparisonModal.svelte';
import PromptSettingsModal from './components/modals/PromptSettingsModal.svelte';
import PromptTemplatesModal from './components/modals/PromptTemplatesModal.svelte';
import AICapabilityModal from './components/modals/AICapabilityModal.svelte';
import DiffPanelModal from './components/modals/DiffPanelModal.svelte';
import OnboardingGuide from './components/OnboardingGuide.svelte';
import ShortcutPanel from './components/ShortcutPanel.svelte';
import TodoList from './components/TodoList.svelte';
import type { Subtask } from './components/TodoList.svelte';
import {
  generateUUID, cleanRecommendedContent, extractFiles,
  extractGatherSpec, safeCopyText
} from './lib/utils';
import {
  type Mode, type TokenUsage, type AgentStep, type Provider, type Model,
  type AIPrompt, type GenHistoryItem, type Message, type ProgressStepDetail,
  type AutoBuildPhase, type ContextProject, type ComparisonResult,
  type SecurityScanResult, type PreviewFile, type SecurityIssue,
  MODES, PROGRESS_LABELS, AUTO_BUILD_PHASE_DEFS, ANALYSIS_MODES, PROMPT_TEMPLATES
} from './lib/types';
import {
  initMarkdownWorker, terminateMarkdownWorker, memoRenderMarkdown,
  memoParseSegments, memoExtractFiles, memoExtractRecFiles,
  memoParseErrorDetail, memoCheckWebUI, preRenderVisibleMessages, setupCopyCode
} from './lib/markdown';
import {
  parseSSEBuffer, analyzeProgressFromContent, updateProgressDetails,
  buildToolLabel, buildResultPreview, resolveStepIndex, mapAutoBuildPhaseToStep,
  StreamBatchManager, ProgressUpdateManager, SafetyTimerManager, AgentStepBatcher
} from './lib/streaming';
import {
  loadGenHistory as loadGenHistoryFromStorage, saveGenHistory as saveGenHistoryToStorage,
  addGenHistory as createGenHistoryItem, fetchConversations,
  fetchConversation, deleteConversationById, saveConversationToBackend,
  loadSessionsList, deleteSessionById, exportSessionById,
  fetchSessionMessages, exportConversationToFile, searchSessions,
  fetchProjectFiles, fetchProjectList, deployToAdb
} from './lib/history';
import {
  loadProvidersFromBackend, saveModelSelectionToStorage, saveConfigToBackend,
  refreshModelsFromBackend, saveModelMaxTokens, loadProviderConfig,
  saveProviderConfigToBackend, fetchCapability
} from './lib/provider';
import { loadPrompts as loadPromptsFromBackend, savePromptToBackend, resetPromptToDefault, loadPromptForMode } from './lib/prompts';
import { loadImportProjects as loadImportProjectsFromBackend, scanFiles, importFilesToProject } from './lib/import-scan';
import {
  findLastAssistantIdx, truncateForRegeneration, editMessageContent,
  deleteMessageAt, getMessageContent
} from './lib/messages';
import { loadProjectFilesState, loadContextProjectListState, addToContextString } from './lib/context';
import { filterStepsByRound } from './lib/rounds';

  let { onNavigate }: { onNavigate?: (route: string, id?: string) => void } = $props();

  // ─── Core state ───
  let providers = $state<Provider[]>([]);
  let selectedProviderID = $state('');
  let selectedModelID = $state('');
  let configLoaded = $state(false);
  let refreshing = $state(false);
  let showModelDropdown = $state(false);
  let editingModelMaxTokens = $state('');
  let editMaxTokensValue = $state('');

  let availableModels = $derived((providers || []).find(x => x.id === selectedProviderID)?.models || []);
  let freeModels = $derived((availableModels || []).filter(m => m.price_input_per_m === 0 && m.price_output_per_m === 0));
  let paidModels = $derived((availableModels || []).filter(m => m.price_input_per_m > 0 || m.price_output_per_m > 0));
  let selectedModel = $derived(availableModels.find(m => m.id === selectedModelID) || null);

  let mode = $state<Mode>('generate');
  let input = $state('');
  let messages = $state<Message[]>([]);
  let lastAssistantIdx = $derived((() => { for (let j = messages.length - 1; j >= 0; j--) { if (messages[j].role === 'assistant') return j; } return -1; })());
  let streaming = $state(false);
  let buildLog = $state('');
  let streamCtrl: any = null;
  let chatEnd: HTMLDivElement | undefined = $state();
  let expandedReasoning = $state(new Set<number>());
  let messageUsages = $state<Map<number, TokenUsage>>(new Map());
  let messageTimes = $state<Map<number, number>>(new Map());
  let requestStartTime = $state(0);
  let buildProgressAbort: AbortController | null = null;

  // Prompt settings
  let editingMessageIdx = $state(-1);
  let editingMessageText = $state('');
  let deletingMessageIdx = $state(-1);
  let showDeleteConfirm = $state(false);
  let showPromptSettings = $state(false);
  let promptTab = $state<Mode>('generate');
  let prompts = $state<AIPrompt[]>([]);
  let promptDraft = $state('');
  let promptSaving = $state(false);
  let promptLoading = $state(false);

  // Provider config modal
  let showProviderConfig = $state(false);
  let configEndpoint = $state('');
  let configApiKey = $state('');
  let configSaving = $state(false);

  // AI Capability dashboard
  let showCapability = $state(false);
  let capability = $state<any>(null);
  let capabilityLoading = $state(false);

  // Gathered requirements card
  let gatheredSpec = $state<any>(null);
  let showSpecCard = $state(false);

  // Progress indicator
  let currentStepIndex = $state(-1);
  let progressStepDetails = $state<ProgressStepDetail[]>([]);

  // Agent steps batching (using extracted AgentStepBatcher)
  let agentStepBatcherRef: AgentStepBatcher | null = null;
  function pushAgentStep(step: any) {
    if (!agentStepBatcherRef) {
      agentStepBatcherRef = new AgentStepBatcher(allAgentSteps, (steps) => { agentSteps = steps; });
    }
    agentStepBatcherRef.push(step);
  }

  // Generation history
  let genHistory = $state<GenHistoryItem[]>([]);

  // Agent state
  let agentSteps = $state<AgentStep[]>([]);
  let allAgentSteps = $state<AgentStep[]>([]);
  let selectedRound = $state(-1);
  let maxRoundIndex = $state(0);
  let expandedSteps = $state<Set<number>>(new Set());
  let sessionId = $state('');
  let agentMode = $state<'plan' | 'act'>('act');

  // Todo list state
  let subtasks = $state<Subtask[]>([]);
  let todoCollapsed = $state(false);

  // Auto-build state
  let autoBuildPhases = $state<AutoBuildPhase[]>([]);
  let autoBuildFiles = $state<{path: string; content: string; size: number}[]>([]);
  let autoBuildProjectId = $state('');
  let autoBuildProjectName = $state('');
  let stepStartTime = $state(Date.now());
  let stepElapsed = $state('0s');

  // Session state
  let sessions = $state<any[]>([]);
  let sessionsLoading = $state(false);
  let activeSessionId = $state('');
  let searchResults = $state<any[]>([]);

  // Persist active session
  $effect(() => {
    const _ = activeSessionId;
    const __ = showHistorySidebar;
    if (activeSessionId) localStorage.setItem('ai_active_session_id', activeSessionId);
    else localStorage.removeItem('ai_active_session_id');
    localStorage.setItem('ai_history_sidebar_open', String(showHistorySidebar));
  });

  // Code Diff
  let diffDiffs = $state<any[]>([]);
  let diffFilePath = $state('');
  let showDiffPanel = $state(false);

  // Build Progress
  let buildProgress = $state<{stage: string; progress: number; message: string} | null>(null);
  let buildProgressActive = $state(false);

  // Preview modal
  let showPreviewModal = $state(false);
  let previewFiles = $state<{path: string; content: string}[]>([]);

  // Security scan
  let scanResult = $state<SecurityScanResult | null>(null);
  let scanning = $state(false);
  let showSecurityWarning = $state(false);
  let pendingImportFiles = $state<{path: string; content: string}[]>([]);

  // Import dialog
  let showImportDialog = $state(false);
  let importFiles = $state<{path: string; content: string}[]>([]);
  let importProjects = $state<{id: string; name: string}[]>([]);
  let selectedImportProject = $state('');
  let importing = $state(false);

  // Onboarding & shortcuts
  let showOnboarding = $state(false);
  let showShortcutPanel = $state(false);
  let onboardingDone = $state(false);

  // Virtual scroll state
  let scrollTop = $state(0);
  let containerHeight = $state(0);
  const ITEM_HEIGHT = 80; // estimated avg message height
  const OVERSCAN = 5; // extra items above/below viewport
  let virtualStart = $derived(Math.max(0, Math.floor(scrollTop / ITEM_HEIGHT) - OVERSCAN));
  let virtualEnd = $derived(Math.min(messages.length, Math.ceil((scrollTop + containerHeight) / ITEM_HEIGHT) + OVERSCAN));
  let virtualMessages = $derived(messages.slice(virtualStart, virtualEnd));
  let virtualSpacerTop = $derived(virtualStart * ITEM_HEIGHT);
  let virtualSpacerBottom = $derived((messages.length - virtualEnd) * ITEM_HEIGHT);

  // Generated files
  let viewMode = $state<'diff' | 'files'>('files');
  let generatedFiles = $state<{path: string; content: string; oldContent?: string}[]>([]);
  let showGeneratedFiles = $state(false);

  // Collapsible panels
  let progressCollapsed = $state(false);
  let projectCardCollapsed = $state(false);
  let agentStepsCollapsed = $state(true);
  let agentHadFinalAnswer = $state(false);

  // Project context
  let projectContext = $state('');
  let showProjectContext = $state(false);
  let contextProjects = $state<ContextProject[]>([]);
  let selectedContextProject = $state('');
  let selectedContextFile = $state('');
  let contextProjectList = $state<{id: string; name: string}[]>([]);

  // Comparison
  let showComparison = $state(false);
  let comparisonResults = $state<ComparisonResult[]>([]);
  let comparisonInput = $state('');
  let comparisonRunning = $state(false);

  // History sidebar
  let showHistorySidebar = $state(false);
  let historyTab = $state<'conversations' | 'generations'>('conversations');
  let convSaving = $state(false);
  let convLoading = $state(false);
  let savedConversations = $state<any[]>([]);

  // Prompt templates
  let showPromptTemplates = $state(false);

  // Misc
  let showExportMenu = $state(false);
  const modes = MODES;

  // ─── Streaming batch state (using extracted StreamBatchManager) ───
  let streamBatchMgr: StreamBatchManager;
  let lastStreamAssistantIdx = -1;
  let sseLineBuffer = '';
  let seenReadPaths = $state(new Set<string>());
  let currentToolInput = $state<Record<string, any> | null>(null);
  let stepElapsedTimer: ReturnType<typeof setInterval> | null = null;
  let autoSaveTimer: ReturnType<typeof setTimeout> | null = null;

  // ─── Progress update debouncing (using extracted ProgressUpdateManager) ───
  let progressUpdateMgr = new ProgressUpdateManager((content: string) => {
    const stepIdx = analyzeProgressFromContent(content);
    if (stepIdx >= 0) currentStepIndex = Math.max(currentStepIndex, stepIdx);
  });

  // ─── Safety timer (using extracted SafetyTimerManager) ───
  let safetyTimerMgr = new SafetyTimerManager();

  // ─── Agent step batcher (using extracted AgentStepBatcher) ───
  let agentStepBatcher: AgentStepBatcher;
  function ensureAgentStepBatcher(): AgentStepBatcher {
    if (!agentStepBatcher) {
      agentStepBatcher = new AgentStepBatcher(allAgentSteps, (steps) => { agentSteps = steps; });
    }
    return agentStepBatcher;
  }

  function ensureStreamBatchMgr(): StreamBatchManager {
    if (!streamBatchMgr) {
      streamBatchMgr = new StreamBatchManager(
        () => messages,
        (msgs) => { messages = msgs; },
        () => lastStreamAssistantIdx,
      );
    }
    return streamBatchMgr;
  }
  function appendStreamContent(text: string) { ensureStreamBatchMgr().appendContent(text); }
  function appendStreamReasoning(text: string) { ensureStreamBatchMgr().appendReasoning(text); }

  // ─── Agent step helpers ───
  function ensureAgentAssistantMsg(): number {
    if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') return messages.length - 1;
    messages = [...messages, { role: 'assistant', content: '' }];
    return messages.length - 1;
  }

  function createNewAssistantMsg(): number {
    messages = [...messages, { role: 'assistant', content: '' }];
    return messages.length - 1;
  }

  function parseAgentStep(parsed: any): boolean {
    if (parsed.type === 'round_sync') {
      const backendRound = parsed.round ?? 0;
      const backendMaxRound = parsed.max_round ?? backendRound;
      if (backendRound >= 0) {
        if (messages.length > 0) {
          const lastUserIdx = messages.map((m, i) => m.role === 'user' ? i : -1).filter(i => i >= 0).pop();
          if (lastUserIdx !== undefined && lastUserIdx >= 0) {
            messages[lastUserIdx] = { ...messages[lastUserIdx], round: backendRound };
            messages = [...messages];
          }
        }
        maxRoundIndex = Math.max(maxRoundIndex, backendMaxRound);
        selectedRound = backendRound;
      }
      return true;
    }
    if (parsed.type === 'step') {
      const step = parsed.step;
      const currentRound = selectedRound;
      if (step === 'think') {
        pushAgentStep({ type: 'think', content: parsed.content || parsed.message || '', round: currentRound });
      } else if (step === 'skill_call') {
        pushAgentStep({ type: 'skill_call', skill: parsed.skill, input: parsed.input, round: currentRound });
        currentToolInput = parsed.input || null;
        const idx = ensureAgentAssistantMsg();
        messages[idx] = { ...messages[idx], content: buildToolLabel(parsed.skill, parsed.input?.path) };
        messages = [...messages];
      } else if (step === 'skill_result') {
        pushAgentStep({ type: 'skill_result', skill: parsed.skill, content: parsed.content, round: currentRound });
        if (parsed.skill === 'read_file' && currentToolInput?.path) {
          if (seenReadPaths.has(currentToolInput.path)) { currentToolInput = null; return true; }
          seenReadPaths.add(currentToolInput.path);
        }
        currentToolInput = null;
        const idx = ensureAgentAssistantMsg();
        const preview = buildResultPreview(parsed.content);
        messages[idx] = { ...messages[idx], content: `⚙️ ${parsed.skill} 完成${preview ? ': ' + preview : ''}` };
        messages = [...messages];
      } else if (step === 'answer') {
        if (agentSteps.length > 0 && agentSteps[agentSteps.length - 1].type === 'think') agentSteps = agentSteps.slice(0, -1);
        pushAgentStep({ type: 'answer', content: parsed.content, round: currentRound });
        seenReadPaths = new Set();
        const isMaxIterError = parsed.content && parsed.content.includes('超出最大迭代次数');
        if (!isMaxIterError) {
          const idx = createNewAssistantMsg();
          messages[idx] = { ...messages[idx], content: parsed.content };
          messages = [...messages];
        } else { agentHadFinalAnswer = true; }
      } else if (mode === 'generate' || mode === 'auto-build') {
        const stepIndex = resolveStepIndex(step);
        if (stepIndex >= 0) {
          currentStepIndex = stepIndex;
          progressStepDetails = updateProgressDetails(progressStepDetails, step, parsed.message);
        }
      }
      return true;
    }
    if (parsed.type === 'checkpoint') {
      pushAgentStep({ type: 'skill_result', skill: 'checkpoint', content: `📝 文件已修改: ${parsed.path || 'unknown'} (可回滚 #${parsed.checkpoint || 0})`, round: selectedRound } as any);
      return true;
    }
    if (parsed.type === 'project_created') {
      if (parsed.project_id) { selectedContextProject = parsed.project_id; loadProjectFiles(parsed.project_id); toast(`📁 项目已创建: ${parsed.project_id.slice(0, 8)}…`, 'success'); }
      return true;
    }
    if (parsed.type === 'step' && parsed.step === 'compact') {
      pushAgentStep({ type: 'think', content: `🗜️ ${parsed.content || '上下文已压缩'}`, round: selectedRound });
      return true;
    }
    // Task decomposition events
    if (parsed.type === 'step' && parsed.step === 'task_plan') {
      if (parsed.subtasks && Array.isArray(parsed.subtasks)) {
        subtasks = parsed.subtasks.map((s: any) => ({
          id: s.id,
          description: s.description,
          status: s.status || 'pending',
          dependencies: s.dependencies || [],
          files: s.files || [],
          progress: s.progress || 0,
          started_at: s.started_at,
          completed_at: s.completed_at,
          retry_count: s.retry_count || 0,
        }));
      }
      pushAgentStep({ type: 'think', content: `📋 ${parsed.content || '任务分解完成'}`, round: selectedRound });
      return true;
    }
    if (parsed.type === 'step' && parsed.step === 'task_progress') {
      const subtaskId = parsed.subtask_id;
      const status = parsed.status;
      if (subtaskId && status) {
        subtasks = subtasks.map(s => {
          if (s.id === subtaskId) {
            return {
              ...s,
              status: status as Subtask['status'],
              progress: parsed.progress ?? s.progress,
              description: parsed.description || s.description,
            };
          }
          return s;
        });
      }
      return true;
    }
    return false;
  }

  // ─── Stream event handlers ───
  function onStreamData(e: Event) {
    const safetyMs = (mode === 'agent' || mode === 'auto-build') ? 1800000 : 60000; // 30 minutes for complex tasks
    safetyTimerMgr.start(safetyMs, () => {
      if (streaming) { streaming = false; messages = [...messages, { role: 'assistant', content: '⏱️ **连接超时**\n\n后端 30 分钟内无数据。请检查 Agent 是否仍在运行。' }]; toast('AI 连接超时', 'error'); }
    });
    const detail = (e as CustomEvent).detail as string;
    const { dataChunks, leftover, done } = parseSSEBuffer(sseLineBuffer, detail);
    sseLineBuffer = leftover;
    // Process data chunks BEFORE checking done — error events may arrive
    // in the same chunk as [DONE], and we must not discard them.
    for (const data of dataChunks) {
      try {
        const parsed = JSON.parse(data);
        if (mode === 'agent' && parseAgentStep(parsed)) continue;
        if (mode === 'auto-build') { handleAutoBuildEvent(parsed); continue; }
        if (parsed.type === 'step') {
          if (mode === 'generate' || (mode as string) === 'auto-build') {
            const stepIndex = resolveStepIndex(parsed.step);
            if (stepIndex >= 0) {
              currentStepIndex = stepIndex;
              progressStepDetails = updateProgressDetails(progressStepDetails, parsed.step, parsed.message);
            }
          }
          // Handle task decomposition events in all modes
          if (parsed.step === 'task_plan' && parsed.subtasks && Array.isArray(parsed.subtasks)) {
            subtasks = parsed.subtasks.map((s: any) => ({
              id: s.id,
              description: s.description,
              status: s.status || 'pending',
              dependencies: s.dependencies || [],
              files: s.files || [],
              progress: s.progress || 0,
              started_at: s.started_at,
              completed_at: s.completed_at,
              retry_count: s.retry_count || 0,
            }));
          }
          if (parsed.step === 'task_progress' && parsed.subtask_id) {
            subtasks = subtasks.map(s => {
              if (s.id === parsed.subtask_id) {
                return { ...s, status: parsed.status || s.status, progress: parsed.progress ?? s.progress };
              }
              return s;
            });
          }
          return;
        }
        if (parsed.type === 'reasoning') {
          if (lastStreamAssistantIdx < 0 || lastStreamAssistantIdx >= messages.length || messages[lastStreamAssistantIdx]?.role !== 'assistant') {
            messages = [...messages, { role: 'assistant', content: '', reasoning: '' }];
            lastStreamAssistantIdx = messages.length - 1;
          }
          appendStreamReasoning(parsed.content);
          return;
        }
        if (parsed.type === 'error' || parsed.error) {
          streaming = false; currentStepIndex = -1;
          messages = [...messages, { role: 'assistant', content: `❌ **AI 错误**\n\n${parsed.error || '未知错误'}` }];
          toast((parsed.error || '未知错误').slice(0, 60), 'error');
          return;
        }
        if (parsed.type === 'usage' && parsed.usage) {
          const msgIdx = messages.length - 1;
          if (msgIdx >= 0) { messageUsages.set(msgIdx, parsed.usage); messageUsages = messageUsages; }
          return;
        }
        if (mode === 'agent' && (parsed.type === 'stream_delta' || parsed.type === 'reasoning')) {
          let content = parsed.content || (parsed.choices?.[0]?.delta?.content) || '';
          if (content && content.trim()) {
            const lastIdx = agentSteps.length - 1;
            if (lastIdx >= 0 && agentSteps[lastIdx].type === 'think') {
              agentSteps[lastIdx] = { ...agentSteps[lastIdx], content: agentSteps[lastIdx].content + content };
              agentSteps = agentSteps;
            } else { pushAgentStep({ type: 'think', content }); }
          }
          return;
        }
        let content = parsed.content || parsed.choices?.[0]?.delta?.content || '';
        let reasoning = parsed.choices?.[0]?.delta?.reasoning_content || '';
        if (reasoning) { appendStreamReasoning(reasoning); }
        if (content) {
          if ((mode as string) === 'auto-build') return;
          if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') lastStreamAssistantIdx = messages.length - 1;
          else { messages = [...messages, { role: 'assistant', content: '' }]; lastStreamAssistantIdx = messages.length - 1; }
          appendStreamContent(content);
          progressUpdateMgr.append(content);
        }
      } catch {
        if (mode === 'auto-build') return;
        if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') { messages[messages.length - 1].content += data; messages = [...messages]; }
        else messages = [...messages, { role: 'assistant', content: data }];
        progressUpdateMgr.append(data);
      }
    }
    // Check done AFTER processing data chunks — [DONE] and error events
    // may arrive in the same TCP chunk; errors must be shown first.
    if (done) { streaming = false; sseLineBuffer = ''; return; }
  }

  function handleAutoBuildEvent(parsed: any) {
    if (parsed.type === 'phase') {
      autoBuildPhases = autoBuildPhases.map(p => {
        if (p.phase === parsed.phase) return { ...p, message: parsed.message, status: 'running' as const };
        if (p.status === 'running') return { ...p, status: 'done' as const };
        return p;
      });
      stepStartTime = Date.now();
      const mappedStep = mapAutoBuildPhaseToStep(parsed.phase);
      if (mappedStep) {
        const stepIndex = resolveStepIndex(mappedStep);
        if (stepIndex >= 0) currentStepIndex = stepIndex;
        progressStepDetails = updateProgressDetails(progressStepDetails, mappedStep, parsed.message);
      }
    } else if (parsed.type === 'complete') {
      streaming = false;
      autoBuildPhases = autoBuildPhases.map(p => ({ ...p, status: 'done' as const }));
      if (requestStartTime > 0) { const elapsed = Date.now() - requestStartTime; const msgIdx = messages.length - 1; if (msgIdx >= 0) { messageTimes.set(msgIdx, elapsed); messageTimes = messageTimes; } }
      if (parsed.project_id) autoBuildProjectId = parsed.project_id;
      if (parsed.project_name) autoBuildProjectName = parsed.project_name;
      if (parsed.files && Array.isArray(parsed.files)) {
        showGeneratedFiles = true;
        const fileListStr = parsed.files.map((f: any) => '- ' + f.path + ' (' + (f.size || 0) + ' bytes)').join('\n');
        messages = [...messages, { role: 'assistant', content: '✅ **模块开发完成！** 共生成 ' + parsed.files.length + ' 个文件。\n\n文件列表：\n' + fileListStr }];
        if (parsed.project_id) {
          const token = localStorage.getItem('moduforge_token') || '';
          fetch(`/api/v1/projects/${parsed.project_id}/files`, { headers: { 'Authorization': `Bearer ${token}` } })
            .then(res => res.ok ? res.json() : null)
            .then(data => { if (data?.files) { autoBuildFiles = data.files; generatedFiles = data.files.map((f: any) => ({ path: f.path, content: f.content })); } })
            .catch(e => console.error('Failed to fetch project files:', e));
        }
      } else { messages = [...messages, { role: 'assistant', content: '✅ **模块开发完成！**' }]; }
    } else if (parsed.type === 'error') {
      streaming = false;
      autoBuildPhases = autoBuildPhases.map(p => p.status === 'running' ? { ...p, status: 'error' as const } : p);
      messages = [...messages, { role: 'assistant', content: `❌ **构建失败**\n\n${parsed.error}` }];
      toast(parsed.error || '构建失败', 'error');
    } else if (parsed.type === 'usage' && parsed.usage) {
      const msgIdx = messages.length - 1;
      if (msgIdx >= 0) { messageUsages.set(msgIdx, parsed.usage); messageUsages = messageUsages; }
    } else if (parsed.type === 'reasoning') {
      if (messages.length === 0 || messages[messages.length - 1].role !== 'assistant') messages = [...messages, { role: 'assistant', content: '', reasoning: '' }];
      const lastIdx = messages.length - 1;
      messages[lastIdx] = { ...messages[lastIdx], reasoning: (messages[lastIdx].reasoning || '') + parsed.content };
      messages = [...messages];
    }
  }

  function onTimeout() {
    safetyTimerMgr.stop();
    streaming = false; currentStepIndex = -1;
    const hint = mode === 'auto-build'
      ? '⏱️ **智能构建超时**（超过 10 分钟无响应）\n\n可能原因：\n1. LLM 生成复杂模块耗时过长\n2. 模型响应太慢（试试换一个更快的模型）\n3. 网络连接不稳定\n\n建议：切换到更快的模型重试'
      : '⏱️ **请求超时**（长时间无响应）\n\n建议：切换到免费模型重试，或在设置中检查 LLM 配置。';
    messages = [...messages, { role: 'assistant', content: hint }];
    toast('AI 请求超时', 'error');
  }

  function onStreamError(e: Event) {
    const detail = (e as CustomEvent).detail || '未知错误';
    safetyTimerMgr.stop();
    streaming = false; currentStepIndex = -1;
    messages = [...messages, { role: 'assistant', content: `❌ **AI 错误**\n\n${detail}` }];
    toast(detail, 'error');
  }

  function onStreamDone() {
    safetyTimerMgr.stop();
    if (sseLineBuffer.trim()) {
      const leftover = sseLineBuffer; sseLineBuffer = '';
      if (leftover.startsWith('data: ')) {
        const data = leftover.slice(6);
        if (data !== '[DONE]') { try { const parsed = JSON.parse(data); if (mode === 'agent') parseAgentStep(parsed); } catch {} }
      }
    } else { sseLineBuffer = ''; }
    if (streamBatchMgr) streamBatchMgr.cancel();
    progressUpdateMgr.flush();

    if (streaming) {
      streaming = false;
      if (mode !== 'agent') currentStepIndex = Math.max(currentStepIndex, 4);

      // Record timing
      if (requestStartTime > 0) {
        const elapsed = Date.now() - requestStartTime;
        const msgIdx = messages.length - 1;
        if (msgIdx >= 0) { messageTimes.set(msgIdx, elapsed); messageTimes = messageTimes; }
        requestStartTime = 0;
      }

      // Save to history
      const lastAssistant = messages.filter(m => m.role === 'assistant').slice(-1)[0];
      if (lastAssistant && lastAssistant.content) {
        const item = createGenHistoryItem(lastAssistant.content.slice(0, 60), mode, messages, selectedModel?.name || '', selectedModelID);
        if (item) { genHistory = [item, ...genHistory].slice(0, 50); saveGenHistoryToStorage(genHistory); }
      }

      // Check for gathered spec
      if (mode === 'gather' && lastAssistant) {
        const spec = extractGatherSpec(lastAssistant.content);
        if (spec) { gatheredSpec = spec; showSpecCard = true; }
      }

      // Auto-save conversation
      autoSaveConversation();

      // Reset step elapsed timer
      if (stepElapsedTimer) { clearInterval(stepElapsedTimer); stepElapsedTimer = null; }
    }
  }

  // ─── Thin wrappers for extracted modules ───

  async function loadProviders() {
    const result = await loadProvidersFromBackend();
    providers = result.providers;
    selectedProviderID = result.selectedProviderID;
    selectedModelID = result.selectedModelID;
    configLoaded = true;
  }

  function onProviderChange() {
    if (availableModels.length > 0) selectedModelID = availableModels[0].id;
    saveModelSelectionToStorage(selectedProviderID, selectedModelID);
    saveConfigToBackend(selectedProviderID, selectedModelID);
  }

  async function onModelSelect(modelId: string) {
    selectedModelID = modelId;
    saveModelSelectionToStorage(selectedProviderID, selectedModelID);
    await saveConfigToBackend(selectedProviderID, selectedModelID);
  }

  async function refreshModels() {
    refreshing = true;
    await refreshModelsFromBackend();
    await loadProviders();
    refreshing = false;
  }

  async function saveProviderConfig() {
    configSaving = true;
    const ok = await saveProviderConfigToBackend(selectedProviderID, configEndpoint, configApiKey);
    configSaving = false;
    if (ok) showProviderConfig = false;
  }

  async function openProviderConfig() {
    showProviderConfig = true;
    configEndpoint = '';
    configApiKey = '';
    const cfg = await loadProviderConfig(selectedProviderID);
    configEndpoint = cfg.endpoint;
    configApiKey = cfg.apiKey;
  }

  async function onSaveMaxTokens(modelId: string, value: string) {
    const maxTokens = parseInt(value);
    if (isNaN(maxTokens) || maxTokens <= 0) { toast('请输入有效的 token 数', 'error'); return; }
    await saveModelMaxTokens(selectedProviderID, providers, modelId, maxTokens);
    editingModelMaxTokens = '';
    editMaxTokensValue = '';
  }

  // Prompt management wrappers
  async function loadPrompts() {
    const updated = await loadPromptsFromBackend();
    prompts = updated;
    return updated;
  }

  async function switchPromptTab(newMode: Mode) {
    promptTab = newMode;
    promptLoading = true;
    const updated = await loadPrompts();
    const p = updated.find(x => x.mode === newMode);
    promptDraft = p?.content || '';
    promptLoading = false;
  }

  async function openPromptSettings() {
    promptLoading = true;
    const updated = await loadPrompts();
    const p = updated.find(x => x.mode === promptTab);
    promptDraft = p?.content || '';
    showPromptSettings = true;
    promptLoading = false;
  }

  async function savePrompt() {
    promptSaving = true;
    promptLoading = true;
    const ok = await savePromptToBackend(promptTab, promptDraft);
    if (ok) await loadPrompts();
    promptLoading = false;
    promptSaving = false;
  }

  async function resetPrompt() {
    promptLoading = true;
    const content = await resetPromptToDefault(promptTab);
    promptDraft = content;
    promptLoading = false;
  }

  // Import wrappers
  async function loadImportProjects() {
    importProjects = await loadImportProjectsFromBackend();
    if (importProjects.length > 0) selectedImportProject = importProjects[0].id;
  }

  function openImportDialog(messageIndex: number) {
    const msg = messages[messageIndex];
    if (!msg) return;
    const files = memoExtractFiles(msg.content);
    if (!files) return;
    importFiles = files;
    scanResult = null;
    loadImportProjects();
    showImportDialog = true;
  }

  async function scanAndImport() {
    if (!selectedImportProject || importFiles.length === 0) return;
    scanning = true;
    scanResult = await scanFiles(importFiles);
    scanning = false;

    if (scanResult && !scanResult.safe) {
      const criticalIssues = scanResult.issues.filter(i => i.severity === 'critical');
      if (criticalIssues.length > 0) {
        showSecurityWarning = true;
        pendingImportFiles = importFiles;
        return;
      }
    }
    proceedImport();
  }

  function proceedImport() {
    showSecurityWarning = false;
    doImport();
  }

  function continueImportAfterWarning() {
    showSecurityWarning = false;
    doImport();
  }

  async function doImport() {
    if (!selectedImportProject || importFiles.length === 0) return;
    importing = true;
    const result = await importFilesToProject(selectedImportProject, importFiles);
    importing = false;
    if (result.fail === 0) {
      toast(`成功导入 ${result.success} 个文件到项目`, 'success');
      showImportDialog = false;
    } else {
      toast(`导入完成：${result.success} 成功，${result.fail} 失败`, result.success > 0 ? 'warning' : 'error');
    }
  }

  // ─── Capability ───
  async function loadCapability() {
    capabilityLoading = true;
    capability = await fetchCapability();
    capabilityLoading = false;
    showCapability = true;
  }

  // ─── Conversation wrappers ───
  async function loadConversations() {
    convLoading = true;
    savedConversations = await fetchConversations();
    convLoading = false;
  }

  async function loadConversation(id: string) {
    convLoading = true;
    const data = await fetchConversation(id);
    if (!data) { convLoading = false; return; }
    if (data.mode === 'agent') {
      if (data.mode && modes.some(m => m.value === data.mode)) mode = data.mode as Mode;
      if (data.agent_mode === 'plan' || data.agent_mode === 'act') agentMode = data.agent_mode;
      if (data.project_id) { autoBuildProjectId = data.project_id; selectedContextProject = data.project_id; await loadProjectFiles(data.project_id); }
      convLoading = false;
      await loadSessionMessages(id);
      return;
    }
    if (data.messages && data.messages.length > 0) {
      messages = data.messages.map((m: any) => ({ role: m.role, content: m.content }));
      activeSessionId = id;
      sessionId = id;
      if (data.mode && modes.some(m => m.value === data.mode)) mode = data.mode as Mode;
      if (data.agent_mode === 'plan' || data.agent_mode === 'act') agentMode = data.agent_mode;
      if (data.project_id) {
        autoBuildProjectId = data.project_id;
        selectedContextProject = data.project_id;
        await loadProjectFiles(data.project_id);
        showGeneratedFiles = true;
        const files = await fetchProjectFiles(data.project_id);
        autoBuildFiles = files;
        generatedFiles = files.map((f: any) => ({ path: f.path, content: f.content }));
      }
      showHistorySidebar = false;
    }
    convLoading = false;
  }

  async function deleteConversation(id: string) {
    const ok = await deleteConversationById(id);
    if (ok) {
      if (activeSessionId === id) { messages = []; activeSessionId = ''; }
      toast('对话已删除', 'success');
      loadConversations();
    } else { toast('删除失败', 'error'); }
  }

  async function saveConversation() {
    if (messages.length === 0) return;
    convSaving = true;
    const convId = mode === 'agent' ? (sessionId || activeSessionId || '') : (activeSessionId || '');
    const id = await saveConversationToBackend({
      id: convId, title: '', mode, messages,
      model: selectedModel?.name || selectedModelID || '',
      project_id: autoBuildProjectId || '',
    });
    if (id) {
      activeSessionId = id;
      if (mode === 'agent') sessionId = id;
      toast('对话已保存', 'success');
      loadConversations();
    } else { toast('保存失败', 'error'); }
    convSaving = false;
  }

  // ─── Session management ───
  async function loadSessions() {
    sessionsLoading = true;
    sessions = await loadSessionsList();
    sessionsLoading = false;
  }

  async function deleteSession(targetSessionId: string) {
    const ok = await deleteSessionById(targetSessionId);
    if (ok) {
      if (activeSessionId === targetSessionId) { messages = []; activeSessionId = ''; }
      toast('对话已删除', 'success');
      loadSessions();
    } else { toast('删除失败', 'error'); }
  }

  async function exportSession(targetSessionId: string) {
    const ok = await exportSessionById(targetSessionId);
    if (ok) toast('导出成功', 'success');
    else toast('导出失败', 'error');
  }

  async function loadSessionMessages(sessId: string) {
    streaming = false;
    activeSessionId = sessId;
    const result = await fetchSessionMessages(sessId);
    if (!result) { toast('无法加载对话消息', 'error'); return; }
    if (result.mode && modes.some(m => m.value === result.mode)) mode = result.mode as Mode;
    if (result.agent_mode === 'plan' || result.agent_mode === 'act') agentMode = result.agent_mode;
    if (result.project_id) {
      autoBuildProjectId = result.project_id;
      selectedContextProject = result.project_id;
      await loadProjectFiles(result.project_id);
    }
    allAgentSteps = result.allSteps as AgentStep[];
    maxRoundIndex = result.maxRound;
    selectedRound = result.maxRound;
    const latestSteps = allAgentSteps.filter(s => s.round === result.maxRound);
    agentSteps = latestSteps;
    messages = result.messages;
    sessionId = sessId;
    showHistorySidebar = false;
    agentStepsCollapsed = true;
    agentHadFinalAnswer = false;
    seenReadPaths = new Set();
    currentToolInput = null;
    toast(`已加载对话 (${messages.length} 条消息)`, 'success');
  }

  // ─── Auto-save ───
  function autoSaveConversation() {
    if (messages.length === 0 || !activeSessionId) return;
    if (autoSaveTimer) clearTimeout(autoSaveTimer);
    autoSaveTimer = setTimeout(() => {
      if (mode === 'agent' && messages.length > 0) {
        saveConversationToBackend({
          id: activeSessionId || sessionId, title: '', mode, messages,
          model: selectedModel?.name || selectedModelID || '',
          project_id: autoBuildProjectId || '',
        });
      }
    }, 2000);
  }

  // ─── Export ───
  function exportConversation(format: 'json' | 'markdown') {
    exportConversationToFile(messages, format);
    toast(`已导出为 ${format === 'json' ? 'JSON' : 'Markdown'} 格式`, 'success');
  }

  // ─── Deploy ───
  async function deployAutoBuild() {
    if (autoBuildFiles.length === 0) { toast('没有可部署的文件', 'error'); return; }
    const ok = await deployToAdb(autoBuildFiles.map(f => ({ path: f.path, content: f.content })));
    if (ok) toast('部署请求已发送', 'success');
    else toast('部署失败，请检查 ADB 连接', 'error');
  }

  // ─── Message editing ───
  function regenerateMessage() {
    if (messages.length < 2 || streaming) return;
    const result = truncateForRegeneration(messages);
    if (!result) return;
    messages = result.truncated;
    input = result.userInput;
    setTimeout(() => send(true), 100);
  }

  function editMessage(idx: number) {
    const msg = messages[idx];
    if (!msg) return;
    editingMessageIdx = idx;
    editingMessageText = msg.content;
  }

  function saveEditMessage() {
    if (editingMessageIdx < 0) return;
    messages = editMessageContent(messages, editingMessageIdx, editingMessageText);
    editingMessageIdx = -1;
    editingMessageText = '';
  }

  function cancelEditMessage() { editingMessageIdx = -1; editingMessageText = ''; }
  function confirmDeleteMessage(idx: number) { deletingMessageIdx = idx; showDeleteConfirm = true; }

  function deleteMessage() {
    if (deletingMessageIdx < 0) return;
    messages = deleteMessageAt(messages, deletingMessageIdx);
    showDeleteConfirm = false;
    deletingMessageIdx = -1;
    if (activeSessionId) autoSaveConversation();
  }

  function replyToMessage(idx: number) {
    const msg = messages[idx];
    if (!msg) return;
    input = msg.content;
  }

  // ─── Preview ───
  function openPreview(files: {path: string; content: string}[]) {
    previewFiles = files;
    showPreviewModal = true;
  }

  // ─── Spec ───
  function switchToGenerateWithSpec() {
    if (!gatheredSpec) return;
    showSpecCard = false;
    mode = 'generate';
    input = `请根据以下需求规格生成模块：\n\n模块名称: ${gatheredSpec.module_name || ''}\n描述: ${gatheredSpec.description || ''}\n功能: ${(gatheredSpec.features || []).join(', ')}\n目标框架: ${(gatheredSpec.target_frameworks || []).join(', ')}`;
    gatheredSpec = null;
    setTimeout(() => send(), 100);
  }

  // ─── Project files ───
  async function loadProjectFiles(projectId: string) {
    try {
      const result = await loadProjectFilesState(projectId);
      autoBuildFiles = result.autoBuildFiles;
      generatedFiles = result.generatedFiles;
      showGeneratedFiles = true;
    } catch {}
  }

  async function loadContextProjectList() {
    try {
      contextProjectList = await loadContextProjectListState();
    } catch {}
  }

  function addToContext(filePath: string) {
    if (!filePath) return;
    projectContext = addToContextString(projectContext, filePath);
    toast(`已添加 ${filePath.split('/').pop()} 到上下文`, 'success');
  }

  // ─── Agent step round filtering ───
  // filterStepsByRound is imported from ./lib/rounds

  function prevRound() {
    if (selectedRound > 0) {
      selectedRound--;
      agentSteps = filterStepsByRound(allAgentSteps, selectedRound);
    }
  }

  function nextRound() {
    if (selectedRound < maxRoundIndex) {
      selectedRound++;
      agentSteps = filterStepsByRound(allAgentSteps, selectedRound);
    }
  }

  function handleMsgClick(idx: number) {
    // Load session from message click
  }

  // ─── Comparison ───
  async function runComparison() {
    if (!comparisonInput.trim() || comparisonRunning) return;
    comparisonRunning = true;
    comparisonResults = [];
    for (const p of providers) {
      for (const m of p.models.slice(0, 1)) {
        const start = Date.now();
        try {
          const token = localStorage.getItem('moduforge_token') || '';
          const res = await fetch('/api/v1/ai/chat', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ message: comparisonInput, provider_id: p.id, model: m.id, messages: [{ role: 'user', content: comparisonInput }] }),
          });
          const data = await res.json();
          comparisonResults = [...comparisonResults, { model: `${p.name} / ${m.name}`, response: data.content || data.error || 'No response', time: Date.now() - start }];
        } catch {
          comparisonResults = [...comparisonResults, { model: `${p.name} / ${m.name}`, response: 'Error', time: Date.now() - start }];
        }
      }
    }
    comparisonRunning = false;
  }

  // ─── Component callback wrappers ───
  function handleToggleReasoning(idx: number) {
    const next = new Set(expandedReasoning);
    if (next.has(idx)) next.delete(idx); else next.add(idx);
    expandedReasoning = next;
  }
  function handleCopyText(text: string) { safeCopyText(text).then(ok => { if (ok) toast('已复制', 'success'); }); }
  function handleInsertToInput(text: string) { input = text; }
  function handleNewConversation() {
    messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; allAgentSteps = []; selectedRound = -1; maxRoundIndex = 0; expandedReasoning = new Set(); activeSessionId = ''; sessionId = generateUUID(); mode = 'generate'; showHistorySidebar = false; autoBuildProjectId = ''; autoBuildProjectName = ''; autoBuildFiles = [];
  }
  function handleRefreshSidebar() { loadConversations(); loadSessions(); loadGenHistory(); }
  function handleTabChange(tab: 'conversations' | 'generations') { historyTab = tab; }
  function handleSearchSessions(query: string) {
    if (!query) { searchResults = []; return; }
    searchSessions(query).then(r => searchResults = r);
  }
  function handleClearGenHistory() { genHistory = []; localStorage.removeItem('moduforge_ai_history'); }
  function handleToggleAgentStep(idx: number) { const next = new Set(expandedSteps); if (next.has(idx)) next.delete(idx); else next.add(idx); expandedSteps = next; }
  function handleSetAgentMode(m: 'plan' | 'act') { agentMode = m; }
  function handleInputChange(value: string) { input = value; }
  function handleBuildLogChange(value: string) { buildLog = value; }

  // ─── Load gen history ───
  function loadGenHistory() { genHistory = loadGenHistoryFromStorage(); }

  // ─── Stop stream ───
  function stopStream() {
    streamCtrl?.close();
    safetyTimerMgr.stop();
    streaming = false; currentStepIndex = -1; progressSteps = []; progressStepDetails = [];
    expandedReasoning = new Set(); agentSteps = []; agentHadFinalAnswer = false;
    lastStreamAssistantIdx = -1; sseLineBuffer = '';
    if (streamBatchMgr) streamBatchMgr.cancel();
    progressUpdateMgr.reset();
    if (mode === 'auto-build') autoBuildPhases = [];
  }

  // ─── Progress steps ───
  let progressSteps = $state<string[]>([]);
  const progressLabels = PROGRESS_LABELS;

  // ─── Send ───
  async function send(skipAddUserMsg = false) {
    const text = input.trim();
    if (!text && !skipAddUserMsg) return;
    if (streaming) return;
    if (!configLoaded) { toast('AI 配置加载中...', 'warning'); return; }
    if (!selectedProviderID || !selectedModelID) { toast('请先选择 AI 模型', 'error'); return; }
    if (mode === 'agent' && !selectedContextProject) {
      toast('💡 建议先选择一个项目上下文，Agent 将更精准地操作文件。', 'info');
    }
    input = '';
    agentSteps = []; seenReadPaths = new Set(); currentToolInput = null;
    agentStepsCollapsed = true; agentHadFinalAnswer = false;
    expandedReasoning = new Set();
    autoBuildPhases = AUTO_BUILD_PHASE_DEFS.map(p => ({ phase: p.phase, message: p.label, status: 'pending' as const }));
    autoBuildFiles = []; autoBuildProjectId = ''; autoBuildProjectName = '';
    if (!skipAddUserMsg) {
      messages = [...messages, { role: 'user', content: text, round: maxRoundIndex + 1 }];
      maxRoundIndex++; selectedRound = maxRoundIndex;
      await tick();
    }
    streaming = true;
    requestStartTime = Date.now(); sseLineBuffer = '';
    progressSteps = Object.values(progressLabels); currentStepIndex = 0; progressStepDetails = [];
    await saveConfigToBackend(selectedProviderID, selectedModelID);
    let body: Record<string, unknown> = { messages, session_id: sessionId, provider: selectedProviderID || '', model: selectedModelID || '' };
    let path: string;
    if (projectContext.trim()) body.project_context = projectContext.trim();
    if (selectedContextProject) body.project_id = selectedContextProject;
    if (mode === 'agent') {
      path = '/agent/run';
      body = { task: text, session_id: sessionId, messages, provider_id: selectedProviderID || '', model: selectedModelID || '', project_id: selectedContextProject || '', project_context: projectContext.trim() || '', agent_mode: agentMode };
    } else if (mode === 'auto-build') { path = '/ai/auto-build'; body = { description: text, session_id: sessionId, project_id: autoBuildProjectId || '', provider: selectedProviderID || '', model: selectedModelID || '' }; }
    else if (mode === 'gather') { body.message = text; body.provider = selectedProviderID || ''; body.model = selectedModelID || ''; path = '/ai/gather'; }
    else if (mode === 'generate') { body.description = text; body.provider = selectedProviderID || ''; body.model = selectedModelID || ''; path = '/ai/generate'; }
    else if (mode === 'repair') { body.build_log = buildLog || text; body.provider = selectedProviderID || ''; body.model = selectedModelID || ''; path = '/ai/repair'; }
    else { body.message = text; body.provider = selectedProviderID || ''; body.model = selectedModelID || ''; path = '/ai/chat'; }
    requestAnimationFrame(() => { const el = document.querySelector('.messages-area') as HTMLElement; if (el) el.scrollTop = el.scrollHeight; });
    const idleMs = (mode === 'auto-build' || mode === 'agent') ? 1800000 : undefined; // 30 minutes for complex tasks
    streamCtrl = (await import('../../lib/api/client')).streamRequest(path, body, idleMs);
    const safetyMs = (mode === 'agent' || mode === 'auto-build') ? 1800000 : 60000; // 30 minutes for complex tasks
    safetyTimerMgr.start(safetyMs, () => {
      if (streaming) {
        streaming = false;
        messages = [...messages, { role: 'assistant', content: '⏱️ **连接超时**\n\n后端 60 秒内无数据。' }];
        toast('AI 连接超时', 'error');
        autoSaveConversation();
      }
    });
  }

  // ─── Mode switching ───
  function switchMode(m: Mode) {
    if (mode === m) return;
    messages = []; currentStepIndex = -1; progressStepDetails = [];
    autoBuildPhases = []; agentSteps = []; expandedReasoning = new Set();
    activeSessionId = ''; sessionId = generateUUID();
    mode = m;
  }

  // ─── onMount / onDestroy ───
  onMount(async () => {
    setupCopyCode();
    initMarkdownWorker();
    stepElapsedTimer = setInterval(() => {
      if (streaming && autoBuildPhases.some(p => p.status === 'running')) {
        const secs = Math.floor((Date.now() - stepStartTime) / 1000);
        stepElapsed = secs >= 60 ? `${Math.floor(secs / 60)}m${secs % 60}s` : `${secs}s`;
      }
    }, 1000);
    window.addEventListener('ai-stream', onStreamData);
    window.addEventListener('ai-stream-done', onStreamDone);
    window.addEventListener('ai-stream-timeout', onTimeout);
    window.addEventListener('ai-stream-error', onStreamError);
    document.addEventListener('click', handleModelDropdownClickOutside);
    const savedActiveId = localStorage.getItem('ai_active_session_id');
    if (savedActiveId) activeSessionId = savedActiveId;
    const savedSidebar = localStorage.getItem('ai_history_sidebar_open');
    if (savedSidebar === 'true') showHistorySidebar = true;
    sessionId = generateUUID();
    await loadProviders();
    await loadSessions();
    loadConversations();
    loadGenHistory();
    loadContextProjectList();
    // Show onboarding for first-time users
    if (!localStorage.getItem('ai_onboarding_done')) {
      showOnboarding = true;
    }
    if (activeSessionId) {
      await loadSessionMessages(activeSessionId);
    } else if (sessions.length > 0 && messages.length === 0) {
      const latest = sessions[0];
      if (latest && latest.msg_count > 0) await loadSessionMessages(latest.session_id);
    }
  });

  onDestroy(() => {
    terminateMarkdownWorker();
    if (stepElapsedTimer) { clearInterval(stepElapsedTimer); stepElapsedTimer = null; }
    window.removeEventListener('ai-stream', onStreamData);
    window.removeEventListener('ai-stream-done', onStreamDone);
    window.removeEventListener('ai-stream-timeout', onTimeout);
    window.removeEventListener('ai-stream-error', onStreamError);
    document.removeEventListener('click', handleModelDropdownClickOutside);
    if (streamBatchMgr) streamBatchMgr.cancel();
    if (agentStepBatcherRef) agentStepBatcherRef.cancel();
    progressUpdateMgr.reset();
    if (autoSaveTimer) { clearTimeout(autoSaveTimer); autoSaveTimer = null; }
    buildProgressAbort?.abort(); buildProgressAbort = null;
    if (typeof window !== 'undefined') delete (window as any).copyCode;
  });

  function handleModelDropdownClickOutside(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest('.top-bar-model-wrap')) showModelDropdown = false;
  }

  // ─── Auto-scroll ───
  let lastScrollTime = 0;
  $effect(() => {
    if (chatEnd) {
      const container = chatEnd.parentElement;
      if (container) {
        const nearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 150;
        if (nearBottom || messages.length <= 1) {
          const now = Date.now();
          if (now - lastScrollTime > 100 || messages.length <= 1) {
            lastScrollTime = now;
            chatEnd.scrollIntoView({ behavior: messages.length <= 1 ? 'instant' : 'smooth' });
          }
        }
      } else { chatEnd.scrollIntoView({ behavior: 'smooth' }); }
    }
  });
</script>

<div class="flex h-full ai-page" role="presentation" onkeydown={(e) => {
  // Don't trigger shortcuts when typing in inputs
  const tag = (e.target as HTMLElement)?.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA' || (e.target as HTMLElement)?.contentEditable === 'true') return;
  if (e.ctrlKey && e.key === 'k') { e.preventDefault(); messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; allAgentSteps = []; selectedRound = -1; maxRoundIndex = 0; expandedReasoning = new Set(); activeSessionId = ''; sessionId = generateUUID(); mode = 'generate'; autoBuildProjectId = ''; autoBuildProjectName = ''; autoBuildFiles = []; subtasks = []; }
  if (e.ctrlKey && e.key === 'e') { e.preventDefault(); if (messages.length > 0) exportConversation('markdown'); }
  if (e.key === '?') { e.preventDefault(); showShortcutPanel = !showShortcutPanel; }
  if (e.key === 'Escape') { showHistorySidebar = false; showPromptSettings = false; showProviderConfig = false; showPreviewModal = false; showImportDialog = false; showComparison = false; showPromptTemplates = false; showDiffPanel = false; showCapability = false; showShortcutPanel = false; }
  if (!e.ctrlKey && !e.metaKey && !e.altKey && ['1','2','3','4','5','6'].includes(e.key)) {
    const idx = parseInt(e.key) - 1;
    if (idx >= 0 && idx < modes.length && !streaming && mode !== modes[idx].value) { messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; expandedReasoning = new Set(); activeSessionId = ''; sessionId = generateUUID(); mode = modes[idx].value; subtasks = []; }
  }
}}>
  {#if showHistorySidebar}
    <ChatSidebar {sessions} {savedConversations} {genHistory} {convSaving} {convLoading} {historyTab} {activeSessionId} {searchResults} messagesLength={messages.length}
      onNewConversation={() => { messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; allAgentSteps = []; selectedRound = -1; maxRoundIndex = 0; expandedReasoning = new Set(); activeSessionId = ''; sessionId = generateUUID(); mode = 'generate'; showHistorySidebar = false; autoBuildProjectId = ''; autoBuildProjectName = ''; autoBuildFiles = []; subtasks = []; }}
      onRefresh={() => { loadConversations(); loadSessions(); loadGenHistory(); }} onSave={saveConversation}
      onClose={() => showHistorySidebar = false} onTabChange={(t) => historyTab = t}
      onSearch={(q) => { if (!q) { searchResults = []; return; } searchSessions(q).then(r => searchResults = r); }}
      onSelectConversation={loadConversation} onSelectSession={loadSessionMessages}
      onDeleteSession={deleteSession} onExportSession={exportSession}
      onDeleteConversation={deleteConversation}
      onRestoreHistory={(item) => { if (item.messages && item.messages.length > 0) { messages = item.messages.map((m: any) => ({ role: m.role, content: m.content })); activeSessionId = ''; sessionId = generateUUID(); streaming = false; currentStepIndex = -1; progressStepDetails = []; expandedReasoning = new Set(); agentSteps = []; allAgentSteps = []; selectedRound = -1; maxRoundIndex = 0; subtasks = []; if (item.mode && modes.some((m: any) => m.value === item.mode)) mode = item.mode as Mode; showHistorySidebar = false; toast('已加载生成记录', 'success'); } }}
      onClearHistory={() => { genHistory = []; localStorage.removeItem('moduforge_ai_history'); }}
    />
  {/if}

  <div class="flex-1 flex flex-col min-w-0">
    <ModelSelector {providers} {selectedProviderID} {selectedModelID} {configLoaded} {showModelDropdown} {editingModelMaxTokens} {editMaxTokensValue} {availableModels} {freeModels} {paidModels} {selectedModel}
      onProviderChange={(v) => { selectedProviderID = v; onProviderChange(); }}
      onModelSelect={(id) => { selectedModelID = id; showModelDropdown = false; onModelSelect(id); }}
      onEditMaxTokens={(id, val) => { editingModelMaxTokens = id; editMaxTokensValue = val; }}
      onSaveMaxTokens={onSaveMaxTokens}
      onToggleDropdown={() => showModelDropdown = !showModelDropdown}
      onEditMaxTokensStart={(id, val) => { editingModelMaxTokens = id; editMaxTokensValue = val; }}
    />

    <CompactToolbar {mode} {streaming} {showComparison} {showProjectContext} {showHistorySidebar} {showCapability}
      onModeChange={(m) => { if (mode !== m) { messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; expandedReasoning = new Set(); subtasks = []; } mode = m; }}
      onToggleComparison={() => showComparison = !showComparison}
      onToggleProjectContext={() => showProjectContext = !showProjectContext}
      onToggleHistory={() => { if (!showHistorySidebar) { loadConversations(); loadSessions(); loadGenHistory(); } showHistorySidebar = !showHistorySidebar; }}
      onLoadCapability={() => { loadCapability(); showCapability = !showCapability; }}
      onOpenPromptSettings={openPromptSettings}
      {onNavigate}
    />

    <ProgressIndicator show={streaming && currentStepIndex >= 0 && (mode === 'generate' || mode === 'auto-build')} {streaming} {currentStepIndex} {progressStepDetails} {stepElapsed} {progressCollapsed} onToggleCollapse={() => progressCollapsed = !progressCollapsed} />
    <AutoBuildProjectCard projectId={autoBuildProjectId} projectName={autoBuildProjectName} fileCount={autoBuildFiles.length} collapsed={projectCardCollapsed} onToggleCollapse={() => projectCardCollapsed = !projectCardCollapsed} />
    <AgentSteps steps={agentSteps} collapsed={agentStepsCollapsed} {expandedSteps} {maxRoundIndex} {selectedRound} {agentMode}
      onToggleCollapse={() => agentStepsCollapsed = !agentStepsCollapsed}
      onPrevRound={prevRound} onNextRound={nextRound}
      onSetAgentMode={(m) => agentMode = m}
      onToggleStep={(idx) => { const next = new Set(expandedSteps); if (next.has(idx)) next.delete(idx); else next.add(idx); expandedSteps = next; }}
    />

    <TodoList {subtasks} collapsed={todoCollapsed} onToggleCollapse={() => todoCollapsed = !todoCollapsed} />

    <!-- Messages with virtual scrolling -->
    <div class="flex-1 overflow-y-auto px-3 py-1.5 space-y-1.5 messages-area"
      onscroll={(e) => { scrollTop = e.currentTarget.scrollTop; }}
      bind:clientHeight={containerHeight}>
      {#if messages.length === 0}
        <div class="flex items-center justify-center h-full">
          <div class="text-center">
            <div class="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4" style="background: var(--gradient-brand-subtle)">
              <span class="material-symbols-outlined text-3xl" style="color: var(--color-primary)">psychology</span>
            </div>
            <p class="text-lg font-semibold text-[var(--color-text)]">{modes.find(m => m.value === mode)?.desc}</p>
            <p class="text-sm text-[var(--color-text-muted)] mt-1">
              {#if mode === 'auto-build'}AI 自动完成模块开发全流程{:else if mode === 'generate'}生成兼容 Magisk / KernelSU / APatch 的通用模块{:else if mode === 'repair'}粘贴构建日志，AI 分析问题并给出修复建议{:else if mode === 'gather'}描述你的模块想法，AI 引导你完善需求{:else}随时提问关于模块开发的问题{/if}
            </p>
          </div>
        </div>
      {:else}
        <!-- Virtual scroll spacer top -->
        {#if virtualSpacerTop > 0}<div style="height:{virtualSpacerTop}px"></div>{/if}
        {#each virtualMessages as msg, i (virtualStart + i + '-' + msg.role)}
          <ChatMessage {msg} index={virtualStart + i} {mode} {streaming} {expandedReasoning} {messageUsages} {messageTimes}
            onToggleReasoning={(idx) => { const next = new Set(expandedReasoning); if (next.has(idx)) next.delete(idx); else next.add(idx); expandedReasoning = next; }}
            onEdit={editMessage} onDelete={confirmDeleteMessage} onReply={replyToMessage}
            onCopy={(text) => safeCopyText(text).then(ok => { if (ok) toast('已复制', 'success'); })}
            onOpenImportDialog={openImportDialog} onOpenPreview={(files: {path: string; content: string}[]) => openPreview(files)}
            onInsertToInput={(text) => input = text}
          />
        {/each}
        <!-- Virtual scroll spacer bottom -->
        {#if virtualSpacerBottom > 0}<div style="height:{virtualSpacerBottom}px"></div>{/if}
        <div bind:this={chatEnd}></div>
      {/if}
    </div>

    <GatherSpecCard show={showSpecCard} spec={gatheredSpec} onClose={() => showSpecCard = false} onGenerate={switchToGenerateWithSpec} />
    <GeneratedFilesPanel show={showGeneratedFiles} files={generatedFiles} {mode} {viewMode} onClose={() => showGeneratedFiles = false} onViewModeChange={(m) => viewMode = m} onDeploy={deployAutoBuild} />
    <ProjectContextPanel show={showProjectContext} {contextProjectList} {contextProjects} selectedProject={selectedContextProject} selectedFile={selectedContextFile} {projectContext}
      onClose={() => showProjectContext = false}
      onProjectChange={(v) => { selectedContextProject = v; if (v) loadProjectFiles(v); }}
      onFileAdd={(v) => { if (v) { projectContext += (projectContext ? '\n' : '') + '文件: ' + v; selectedContextFile = ''; } }}
      onContextChange={(v) => projectContext = v}
    />

    <ChatInput {input} {mode} {streaming} {buildLog}
      onSend={() => send()} onStop={stopStream}
      onInputChange={(v) => input = v}
      onBuildLogChange={(v) => buildLog = v}
    />
  </div>

  <!-- Modals -->
  <ProviderConfigModal show={showProviderConfig} {providers} {selectedProviderID} {configEndpoint} {configApiKey} {configSaving} onClose={() => showProviderConfig = false} onEndpointChange={(v) => configEndpoint = v} onApiKeyChange={(v) => configApiKey = v} onSave={saveProviderConfig} />
  <ImportDialogModal show={showImportDialog} {importFiles} {importProjects} {selectedImportProject} {scanning} {scanResult} {importing} onClose={() => showImportDialog = false} onProjectChange={(v) => selectedImportProject = v} onScanAndImport={scanAndImport} />
  <SecurityWarningModal show={showSecurityWarning} {scanResult} onClose={() => showSecurityWarning = false} onContinue={continueImportAfterWarning} />
  <DeleteConfirmModal show={showDeleteConfirm} onClose={() => { showDeleteConfirm = false; deletingMessageIdx = -1; }} onConfirm={deleteMessage} />
  <PreviewModal show={showPreviewModal} files={previewFiles} onClose={() => showPreviewModal = false} />
  <ComparisonModal show={showComparison} results={comparisonResults} running={comparisonRunning} input={comparisonInput} onClose={() => showComparison = false} onInputChange={(v) => comparisonInput = v} onRun={runComparison} />
  <PromptSettingsModal show={showPromptSettings} {promptTab} {promptDraft} {promptLoading} {promptSaving} onClose={() => showPromptSettings = false} onTabChange={switchPromptTab} onDraftChange={(v) => promptDraft = v} onSave={savePrompt} onReset={resetPrompt} />
  <PromptTemplatesModal show={showPromptTemplates} onClose={() => showPromptTemplates = false} onSelect={(prompt) => { input = prompt; showPromptTemplates = false; }} />
  <AICapabilityModal show={showCapability} {capability} loading={capabilityLoading} onClose={() => showCapability = false} />
  <DiffPanelModal show={showDiffPanel} diffs={diffDiffs} filePath={diffFilePath} onClose={() => showDiffPanel = false} />
  <BuildProgressBar show={buildProgressActive} progress={buildProgress} />
  <OnboardingGuide show={showOnboarding} onClose={() => showOnboarding = false} onComplete={() => { onboardingDone = true; localStorage.setItem('ai_onboarding_done', '1'); }} />
  <ShortcutPanel show={showShortcutPanel} onClose={() => showShortcutPanel = false} />
</div>

<style>
  @media (max-width: 768px) {
    .ai-page :global(.msg-bubble) { max-width: 92% !important; }
    .ai-page :global(.input-row) { gap: 8px; }
    .ai-page :global(.input-row textarea) { width: 100%; min-height: 48px; padding: 8px 12px; font-size: 13px; }
    .ai-page :global(.top-bar-select) { width: 100%; padding: 6px 8px !important; overflow: hidden; }
    .ai-page :global(.top-bar-provider) { flex: 0 0 auto; min-width: 0; max-width: 35%; min-height: 34px; font-size: 12px !important; padding: 4px 6px !important; }
    .ai-page :global(.top-bar-model-wrap) { position: relative; flex: 1 1 0; min-width: 0; }
    .ai-page :global(.top-bar-model-wrap .top-bar-model) { width: 100%; min-height: 34px; font-size: 12px !important; padding: 4px 6px !important; }
    .ai-page :global(.model-dropdown) { left: 0 !important; right: 0 !important; width: 100% !important; max-width: 100% !important; box-sizing: border-box; }
    .ai-page :global(.ai-input-area) { padding: 8px; padding-bottom: 60px; }
    .ai-page :global(.ai-input-area textarea) { min-height: 60px !important; }
    .ai-page :global(.prompt-modal-overlay) { align-items: stretch !important; padding: 0 !important; }
    .ai-page :global(.prompt-modal-overlay > div) { max-width: 100% !important; max-height: 100% !important; border-radius: 0 !important; width: 100%; height: 100%; }
    .ai-page :global(.prompt-modal-overlay textarea) { height: 60vh !important; }
    .ai-page :global(.messages-area) { padding: 6px; padding-bottom: 60px; min-height: 0; flex: 1 1 0%; }
  }
</style>
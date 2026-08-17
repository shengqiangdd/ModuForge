<script lang="ts">
import { onMount, onDestroy, tick } from 'svelte';
import { toast } from '$lib/stores/toast.svelte';
import { client } from '$lib/api/client';

import ChatSidebar from './components/ChatSidebar.svelte';
import ChatInput from './components/ChatInput.svelte';
import ModelSelector from './components/ModelSelector.svelte';
import MetricsPanel from './components/MetricsPanel.svelte';
import CompactToolbar from './components/CompactToolbar.svelte';
import ChatMessages from './components/ChatMessages.svelte';
import ChatControls from './components/ChatControls.svelte';
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
import MDPromptsModal from './components/modals/MDPromptsModal.svelte';
import AICapabilityModal from './components/modals/AICapabilityModal.svelte';
import DiffPanelModal from './components/modals/DiffPanelModal.svelte';
import OnboardingGuide from './components/OnboardingGuide.svelte';
import ShortcutPanel from './components/ShortcutPanel.svelte';
import McpToolPanel from './components/McpToolPanel.svelte';
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
import { StreamHandler } from './lib/stream-handler';
import {
  loadGenHistory as loadGenHistoryFromStorage, saveGenHistory as saveGenHistoryToStorage,
  addGenHistory as createGenHistoryItem, fetchConversations,
  fetchConversation, deleteConversationById, saveConversationToBackend,
  loadSessionsList, deleteSessionById, exportSessionById, renameSessionById,
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
  let chatMessages: ChatMessages;

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
  let expandedReasoning = $state(new Set<number>());
  let messageUsages = $state<Map<number, TokenUsage>>(new Map());
  let messageTimes = $state<Map<number, number>>(new Map());

  // Prompt settings
  let editingMessageIdx = $state(-1);
  let editingMessageText = $state('');
  let deletingMessageIdx = $state(-1);
  let showDeleteConfirm = $state(false);
  let showPromptSettings = $state(false);
  let showMDPrompts = $state(false);
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
  let showMcpTools = $state(false);
  let mcpToolCount = $state(0);
  let capability = $state<any>(null);
  let capabilityLoading = $state(false);

  // Gathered requirements card
  let gatheredSpec = $state<any>(null);
  let showSpecCard = $state(false);

  // Progress indicator
  let currentStepIndex = $state(-1);
  let progressStepDetails = $state<ProgressStepDetail[]>([]);

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
  let sessionsTotal = $state(0);
  let sessionsLoading = $state(false);
  const SESSIONS_PAGE_SIZE = 50;
  let hasMoreMessages = $state(false);
  let loadingEarlier = $state(false);
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

  // ─── Stream handler ───
  let handler = $state<StreamHandler>(null!);

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
  // ─── MCP ask-mode permission confirmation ───
  let pendingPermission = $state<{
    request_id: string;
    server: string;
    tool: string;
    args: Record<string, unknown>;
    timeout_s: number;
  } | null>(null);
  let permissionBusy = $state(false);

  async function resolvePermission(allow: boolean) {
    const req = pendingPermission;
    if (!req) return;
    permissionBusy = true;
    try {
      const res = await fetch('/api/v1/agent/mcp/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
        body: JSON.stringify({ request_id: req.request_id, allow }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        toast(data.error || '确认请求失败（可能已超时）', 'error');
      } else {
        toast(allow ? `已允许调用 ${req.tool}` : `已拒绝调用 ${req.tool}`, allow ? 'success' : 'info');
      }
    } catch (e: any) {
      toast(e.message || '确认请求失败', 'error');
    } finally {
      permissionBusy = false;
      pendingPermission = null;
    }
  }

  function permissionArgsPreview(): string {
    const req = pendingPermission;
    if (!req || !req.args || Object.keys(req.args).length === 0) return '（无参数）';
    try { return JSON.stringify(redactArgValues(req.args), null, 2); } catch { return String(req.args); }
  }

  // Extra defense-in-depth: mask any sensitive-looking values in arg preview
  // (backend already redacts; this catches nested/other shapes).
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

  async function loadSessions() {
    sessionsLoading = true;
    const { sessions: list, total } = await loadSessionsList(0, SESSIONS_PAGE_SIZE);
    sessions = list;
    sessionsTotal = total;
    sessionsLoading = false;
  }

  async function loadMoreSessions() {
    if (sessionsLoading) return;
    sessionsLoading = true;
    const { sessions: more, total } = await loadSessionsList(sessions.length, SESSIONS_PAGE_SIZE);
    sessions = [...sessions, ...more];
    sessionsTotal = total;
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

  async function exportSession(targetSessionId: string, format: 'markdown' | 'json' = 'markdown') {
    const ok = await exportSessionById(targetSessionId, format);
    if (ok) toast(format === 'json' ? '已导出 JSON' : '导出成功', 'success');
    else toast('导出失败', 'error');
  }

  async function renameSession(targetSessionId: string, title: string) {
    const ok = await renameSessionById(targetSessionId, title);
    if (ok) {
      toast('已重命名', 'success');
      loadSessions();
    } else { toast('重命名失败', 'error'); }
  }

  async function loadSessionMessages(sessId: string) {
    streaming = false;
    activeSessionId = sessId;
    const result = await fetchSessionMessages(sessId, 50);
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
    hasMoreMessages = result.has_more;
    // Restore per-message token usage from persisted conversation history
    const restored = new Map<number, TokenUsage>();
    result.messages.forEach((m, i) => { if (m.token_usage) restored.set(i, m.token_usage); });
    if (restored.size > 0) messageUsages = restored;
    sessionId = sessId;
    showHistorySidebar = false;
    agentStepsCollapsed = true;
    agentHadFinalAnswer = false;
    toast(`已加载对话 (${messages.length} 条消息${result.has_more ? '，可加载更早' : ''})`, 'success');
  }

  // 向上加载更早的历史消息（复合游标 created_at+id 分页）
  async function loadEarlierMessages() {
    if (!sessionId || messages.length === 0 || loadingEarlier) return;
    const earliest = messages[0];
    if (!earliest.created_at) { toast('无法加载更早消息', 'error'); return; }
    loadingEarlier = true;
    try {
      const result = await fetchSessionMessages(sessionId, 50, earliest.created_at, earliest.id);
      if (!result || result.messages.length === 0) { hasMoreMessages = false; return; }
      messages = [...result.messages, ...messages];
      allAgentSteps = [...(result.allSteps as AgentStep[]), ...allAgentSteps];
      hasMoreMessages = result.has_more;
    } catch {} finally { loadingEarlier = false; }
  }

  // ─── Auto-save ───
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
    setTimeout(() => handler.send(true), 100);
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
    if (activeSessionId) handler.autoSaveConversation();
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
    setTimeout(() => handler.send(), 100);
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
    messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; allAgentSteps = []; selectedRound = -1; maxRoundIndex = 0; expandedReasoning = new Set(); messageUsages = new Map(); activeSessionId = ''; sessionId = generateUUID(); mode = 'generate'; showHistorySidebar = false; autoBuildProjectId = ''; autoBuildProjectName = ''; autoBuildFiles = [];
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
    handler = new StreamHandler({
      get messages() { return messages; },
      set messages(v) { messages = v; },
      get streaming() { return streaming; },
      set streaming(v) { streaming = v; },
      get configLoaded() { return configLoaded; },
      set configLoaded(v) { configLoaded = v; },
      get currentStepIndex() { return currentStepIndex; },
      set currentStepIndex(v) { currentStepIndex = v; },
      get progressStepDetails() { return progressStepDetails; },
      set progressStepDetails(v) { progressStepDetails = v; },
      get agentSteps() { return agentSteps; },
      set agentSteps(v) { agentSteps = v; },
      get allAgentSteps() { return allAgentSteps; },
      set allAgentSteps(v) { allAgentSteps = v; },
      get agentStepsCollapsed() { return agentStepsCollapsed; },
      set agentStepsCollapsed(v) { agentStepsCollapsed = v; },
      get selectedRound() { return selectedRound; },
      set selectedRound(v) { selectedRound = v; },
      get maxRoundIndex() { return maxRoundIndex; },
      set maxRoundIndex(v) { maxRoundIndex = v; },
      get expandedReasoning() { return expandedReasoning; },
      set expandedReasoning(v) { expandedReasoning = v; },
      get messageUsages() { return messageUsages; },
      set messageUsages(v) { messageUsages = v; },
      get messageTimes() { return messageTimes; },
      set messageTimes(v) { messageTimes = v; },
      get requestStartTime() { return 0; },
      set requestStartTime(_v) {},
      get lastStreamAssistantIdx() { return -1; },
      set lastStreamAssistantIdx(_v) {},
      get seenReadPaths() { return new Set<string>(); },
      set seenReadPaths(_v) {},
      get currentToolInput() { return null; },
      set currentToolInput(_v) {},
      get agentHadFinalAnswer() { return agentHadFinalAnswer; },
      set agentHadFinalAnswer(v) { agentHadFinalAnswer = v; },
      get subtasks() { return subtasks; },
      set subtasks(v) { subtasks = v; },
      get autoBuildPhases() { return autoBuildPhases; },
      set autoBuildPhases(v) { autoBuildPhases = v; },
      get autoBuildFiles() { return autoBuildFiles; },
      set autoBuildFiles(v) { autoBuildFiles = v; },
      get autoBuildProjectId() { return autoBuildProjectId; },
      set autoBuildProjectId(v) { autoBuildProjectId = v; },
      get autoBuildProjectName() { return autoBuildProjectName; },
      set autoBuildProjectName(v) { autoBuildProjectName = v; },
      get stepStartTime() { return stepStartTime; },
      set stepStartTime(v) { stepStartTime = v; },
      get stepElapsed() { return stepElapsed; },
      set stepElapsed(v) { stepElapsed = v; },
      get genHistory() { return genHistory; },
      set genHistory(v) { genHistory = v; },
      get gatheredSpec() { return gatheredSpec; },
      set gatheredSpec(v) { gatheredSpec = v; },
      get showSpecCard() { return showSpecCard; },
      set showSpecCard(v) { showSpecCard = v; },
      get showGeneratedFiles() { return showGeneratedFiles; },
      set showGeneratedFiles(v) { showGeneratedFiles = v; },
      get generatedFiles() { return generatedFiles; },
      set generatedFiles(v) { generatedFiles = v; },
      get buildLog() { return buildLog; },
      set buildLog(v) { buildLog = v; },
      get input() { return input; },
      set input(v) { input = v; },
      get mode() { return mode; },
      set mode(v) { mode = v; },
      get selectedProviderID() { return selectedProviderID; },
      set selectedProviderID(v) { selectedProviderID = v; },
      get selectedModelID() { return selectedModelID; },
      set selectedModelID(v) { selectedModelID = v; },
      get selectedModel() { return selectedModel; },
      set selectedModel(v) { selectedModel = v; },
      get selectedContextProject() { return selectedContextProject; },
      set selectedContextProject(v) { selectedContextProject = v; },
      get projectContext() { return projectContext; },
      set projectContext(v) { projectContext = v; },
      get agentMode() { return agentMode; },
      set agentMode(v) { agentMode = v; },
      get sessionId() { return sessionId; },
      set sessionId(v) { sessionId = v; },
      get activeSessionId() { return activeSessionId; },
      set activeSessionId(v) { activeSessionId = v; },
      get showHistorySidebar() { return showHistorySidebar; },
      set showHistorySidebar(v) { showHistorySidebar = v; },
      get providers() { return providers; },
      set providers(v) { providers = v; },
    }, {
      loadProjectFiles,
      loadConversations,
      loadGenHistory: async () => { loadGenHistory(); },
      saveConfigToBackend: (pid, mid) => saveConfigToBackend(pid, mid),
      scrollToBottom: async () => { await tick(); chatMessages?.scrollToBottom(); },
      toast,
      onPermissionRequest: (req) => { pendingPermission = req; },
    });
    handler.setupEventListeners();
    handler.startElapsedTimer();
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
    // Preload MCP tool count for the input badge (best-effort)
    (async () => {
      try {
        const data = await client.get<{ servers: { tools?: unknown[] }[] }>('/agent/mcp/status');
        mcpToolCount = (data.servers || []).reduce((acc, s) => acc + (s.tools?.length || 0), 0);
      } catch { /* 静默失败，仅影响徽章 */ }
    })();
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
    handler.cleanup();
    document.removeEventListener('click', handleModelDropdownClickOutside);
    if (typeof window !== 'undefined') delete (window as any).copyCode;
  });

  function handleModelDropdownClickOutside(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest('.top-bar-model-wrap')) showModelDropdown = false;
  }

  </script>

<div class="flex h-full ai-page" role="presentation" onkeydown={(e) => {
  // Don't trigger shortcuts when typing in inputs
  const tag = (e.target as HTMLElement)?.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA' || (e.target as HTMLElement)?.contentEditable === 'true') return;
  if (e.ctrlKey && e.key === 'k') { e.preventDefault(); messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; allAgentSteps = []; selectedRound = -1; maxRoundIndex = 0; expandedReasoning = new Set(); messageUsages = new Map(); activeSessionId = ''; sessionId = generateUUID(); mode = 'generate'; autoBuildProjectId = ''; autoBuildProjectName = ''; autoBuildFiles = []; subtasks = []; }
  if (e.ctrlKey && e.key === 'e') { e.preventDefault(); if (messages.length > 0) exportConversation('markdown'); }
  if (e.key === '?') { e.preventDefault(); showShortcutPanel = !showShortcutPanel; }
  if (e.key === 'Escape') { showHistorySidebar = false; showPromptSettings = false; showMDPrompts = false; showProviderConfig = false; showPreviewModal = false; showImportDialog = false; showComparison = false; showPromptTemplates = false; showDiffPanel = false; showCapability = false; showMcpTools = false; showShortcutPanel = false; }
  if (!e.ctrlKey && !e.metaKey && !e.altKey && ['1','2','3','4','5','6'].includes(e.key)) {
    const idx = parseInt(e.key) - 1;
    if (idx >= 0 && idx < modes.length && !streaming && mode !== modes[idx].value) { messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; expandedReasoning = new Set(); activeSessionId = ''; sessionId = generateUUID(); mode = modes[idx].value; subtasks = []; }
  }
}}>
  {#if showHistorySidebar}
    <ChatSidebar {sessions} {sessionsTotal} {sessionsLoading} onLoadMore={loadMoreSessions} {savedConversations} {genHistory} {convSaving} {convLoading} {historyTab} {activeSessionId} {searchResults} messagesLength={messages.length}
      onNewConversation={() => { messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; allAgentSteps = []; selectedRound = -1; maxRoundIndex = 0; expandedReasoning = new Set(); activeSessionId = ''; sessionId = generateUUID(); mode = 'generate'; showHistorySidebar = false; autoBuildProjectId = ''; autoBuildProjectName = ''; autoBuildFiles = []; subtasks = []; }}
      onRefresh={() => { loadConversations(); loadSessions(); loadGenHistory(); }} onSave={saveConversation}
      onClose={() => showHistorySidebar = false} onTabChange={(t) => historyTab = t}
      onSearch={(q) => { if (!q) { searchResults = []; return; } searchSessions(q).then(r => searchResults = r); }}
      onSelectConversation={loadConversation} onSelectSession={loadSessionMessages}
      onDeleteSession={deleteSession} onExportSession={exportSession} onRenameSession={renameSession}
      onDeleteConversation={deleteConversation}
      onRestoreHistory={(item) => { if (item.messages && item.messages.length > 0) { messages = item.messages.map((m: any) => ({ role: m.role, content: m.content })); activeSessionId = ''; sessionId = generateUUID(); streaming = false; currentStepIndex = -1; progressStepDetails = []; expandedReasoning = new Set(); agentSteps = []; allAgentSteps = []; selectedRound = -1; maxRoundIndex = 0; subtasks = []; if (item.mode && modes.some((m: any) => m.value === item.mode)) mode = item.mode as Mode; showHistorySidebar = false; toast('已加载生成记录', 'success'); } }}
      onClearHistory={() => { genHistory = []; localStorage.removeItem('moduforge_ai_history'); }}
    />
  {/if}

  <div class="flex-1 flex flex-col min-w-0">
    <ChatControls
      {providers} {selectedProviderID} {selectedModelID} {configLoaded}
      {showModelDropdown} {editingModelMaxTokens} {editMaxTokensValue}
      {availableModels} {freeModels} {paidModels} {selectedModel}
      {mode} {streaming} {showComparison} {showProjectContext}
      {showHistorySidebar} {showCapability} {showMcpTools}
      onProviderChange={(v) => { selectedProviderID = v; onProviderChange(); }}
      onModelSelect={(id) => { selectedModelID = id; showModelDropdown = false; onModelSelect(id); }}
      onEditMaxTokens={(id, val) => { editingModelMaxTokens = id; editMaxTokensValue = val; }}
      onSaveMaxTokens={onSaveMaxTokens}
      onToggleDropdown={() => showModelDropdown = !showModelDropdown}
      onEditMaxTokensStart={(id, val) => { editingModelMaxTokens = id; editMaxTokensValue = val; }}
      onModeChange={(m) => { if (mode !== m) { messages = []; currentStepIndex = -1; progressStepDetails = []; autoBuildPhases = []; agentSteps = []; expandedReasoning = new Set(); subtasks = []; } mode = m; }}
      onToggleComparison={() => showComparison = !showComparison}
      onToggleProjectContext={() => showProjectContext = !showProjectContext}
      onToggleHistory={() => { if (!showHistorySidebar) { loadConversations(); loadSessions(); loadGenHistory(); } showHistorySidebar = !showHistorySidebar; }}
      onLoadCapability={() => { loadCapability(); showCapability = !showCapability; }}
      onToggleMcpTools={() => showMcpTools = !showMcpTools}
      onOpenPromptSettings={openPromptSettings}
      onOpenMDPrompts={() => showMDPrompts = true}
      {onNavigate}
    />

    <MetricsPanel inputPricePerM={selectedModel?.price_input_per_m || 0} outputPricePerM={selectedModel?.price_output_per_m || 0} />

    <ProgressIndicator show={streaming && currentStepIndex >= 0 && (mode === 'generate' || mode === 'auto-build')} {streaming} {currentStepIndex} {progressStepDetails} {stepElapsed} {progressCollapsed} onToggleCollapse={() => progressCollapsed = !progressCollapsed} />
    <AutoBuildProjectCard projectId={autoBuildProjectId} projectName={autoBuildProjectName} fileCount={autoBuildFiles.length} collapsed={projectCardCollapsed} onToggleCollapse={() => projectCardCollapsed = !projectCardCollapsed} />

    <TodoList {subtasks} collapsed={todoCollapsed} onToggleCollapse={() => todoCollapsed = !todoCollapsed} />

    <ChatMessages bind:this={chatMessages}
      bind:messages {mode} {streaming} {expandedReasoning} {messageUsages} {messageTimes}
      {hasMoreMessages} {loadingEarlier} onLoadEarlier={loadEarlierMessages}
      allAgentSteps={allAgentSteps} agentExpandedSteps={expandedSteps}
      onToggleAgentStep={(idx: number) => { const next = new Set(expandedSteps); if (next.has(idx)) next.delete(idx); else next.add(idx); expandedSteps = next; }}
      onToggleReasoning={(idx: number) => { const next = new Set(expandedReasoning); if (next.has(idx)) next.delete(idx); else next.add(idx); expandedReasoning = next; }}
      onEdit={editMessage} onDelete={confirmDeleteMessage} onReply={replyToMessage}
      onCopy={(text: string) => safeCopyText(text).then(ok => { if (ok) toast('已复制', 'success'); })}
      onOpenImportDialog={openImportDialog} onOpenPreview={(files: {path: string; content: string}[]) => openPreview(files)}
      onInsertToInput={(text: string) => input = text}
    />

    <GatherSpecCard show={showSpecCard} spec={gatheredSpec} onClose={() => showSpecCard = false} onGenerate={switchToGenerateWithSpec} />
    <GeneratedFilesPanel show={showGeneratedFiles} files={generatedFiles} {mode} {viewMode} onClose={() => showGeneratedFiles = false} onViewModeChange={(m) => viewMode = m} onDeploy={deployAutoBuild} />
    <ProjectContextPanel show={showProjectContext} {contextProjectList} {contextProjects} selectedProject={selectedContextProject} selectedFile={selectedContextFile} {projectContext}
      onClose={() => showProjectContext = false}
      onProjectChange={(v) => { selectedContextProject = v; if (v) loadProjectFiles(v); }}
      onFileAdd={(v) => { if (v) { projectContext += (projectContext ? '\n' : '') + '文件: ' + v; selectedContextFile = ''; } }}
      onContextChange={(v) => projectContext = v}
    />

    <ChatInput {input} {mode} {streaming} {buildLog} {mcpToolCount}
      onSend={() => handler.send()} onStop={handler.stopStream}
      onInputChange={(v) => input = v}
      onBuildLogChange={(v) => buildLog = v}
      onOpenMcpTools={() => showMcpTools = true}
    />
  </div>

  <!-- MCP write-tool permission confirmation (ask mode) -->
  {#if pendingPermission}
    <div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.55); backdrop-filter: blur(6px)">
      <div class="w-full max-w-md rounded-2xl border p-5 shadow-2xl" style="background: var(--color-surface); border-color: var(--color-border)">
        <div class="flex items-start gap-3">
          <span class="material-symbols-outlined text-[28px] flex-shrink-0" style="color: var(--color-warning)">shield_person</span>
          <div class="min-w-0 flex-1">
            <h3 class="text-base font-semibold" style="color: var(--color-text)">MCP 写操作确认</h3>
            <p class="text-xs mt-1" style="color: var(--color-text-secondary)">
              AI 请求调用 <span class="font-mono font-semibold" style="color: var(--color-warning)">{pendingPermission.tool}</span>
              {#if pendingPermission.server}（{pendingPermission.server}）{/if}，
              该工具会<strong>变更远端状态</strong>。
            </p>
          </div>
          <button class="btn-ghost flex-shrink-0" onclick={() => pendingPermission = null} aria-label="关闭">✕</button>
        </div>
        <div class="mt-3 rounded-lg p-3 overflow-auto max-h-48 font-mono text-[11px]" style="background: var(--color-bg-elevated, rgba(127,127,127,0.07)); color: var(--color-text-secondary)">
          {permissionArgsPreview()}
        </div>
        <div class="mt-4 flex gap-2 justify-end">
          <button class="px-4 py-2 rounded-lg text-sm font-medium border" disabled={permissionBusy}
                  style="border-color: var(--color-border); color: var(--color-text-secondary)"
                  onclick={() => resolvePermission(false)}>拒绝</button>
          <button class="px-4 py-2 rounded-lg text-sm font-semibold" disabled={permissionBusy}
                  style="background: var(--color-warning); color: #fff"
                  onclick={() => resolvePermission(true)}>允许本次调用</button>
        </div>
        <p class="text-[10px] mt-2 text-center" style="color: var(--color-text-muted)">{pendingPermission.timeout_s} 秒内未确认将自动拒绝；可在 MCP 页面设置「自动允许」</p>
      </div>
    </div>
  {/if}

  <!-- Modals -->
  <ProviderConfigModal show={showProviderConfig} {providers} {selectedProviderID} {configEndpoint} {configApiKey} {configSaving} onClose={() => showProviderConfig = false} onEndpointChange={(v) => configEndpoint = v} onApiKeyChange={(v) => configApiKey = v} onSave={saveProviderConfig} />
  <ImportDialogModal show={showImportDialog} {importFiles} {importProjects} {selectedImportProject} {scanning} {scanResult} {importing} onClose={() => showImportDialog = false} onProjectChange={(v) => selectedImportProject = v} onScanAndImport={scanAndImport} />
  <SecurityWarningModal show={showSecurityWarning} {scanResult} onClose={() => showSecurityWarning = false} onContinue={continueImportAfterWarning} />
  <DeleteConfirmModal show={showDeleteConfirm} onClose={() => { showDeleteConfirm = false; deletingMessageIdx = -1; }} onConfirm={deleteMessage} />
  <PreviewModal show={showPreviewModal} files={previewFiles} onClose={() => showPreviewModal = false} />
  <ComparisonModal show={showComparison} results={comparisonResults} running={comparisonRunning} input={comparisonInput} onClose={() => showComparison = false} onInputChange={(v) => comparisonInput = v} onRun={runComparison} />
  <PromptSettingsModal show={showPromptSettings} {promptTab} {promptDraft} {promptLoading} {promptSaving} onClose={() => showPromptSettings = false} onTabChange={switchPromptTab} onDraftChange={(v) => promptDraft = v} onSave={savePrompt} onReset={resetPrompt} />
  <PromptTemplatesModal show={showPromptTemplates} onClose={() => showPromptTemplates = false} onSelect={(prompt) => { input = prompt; showPromptTemplates = false; }} />
  <MDPromptsModal open={showMDPrompts} onClose={() => showMDPrompts = false} />
  <AICapabilityModal show={showCapability} {capability} loading={capabilityLoading} onClose={() => showCapability = false} />
  <DiffPanelModal show={showDiffPanel} diffs={diffDiffs} filePath={diffFilePath} onClose={() => showDiffPanel = false} />
  <BuildProgressBar show={buildProgressActive} progress={buildProgress} />
  <OnboardingGuide show={showOnboarding} onClose={() => showOnboarding = false} onComplete={() => { onboardingDone = true; localStorage.setItem('ai_onboarding_done', '1'); }} />
  <ShortcutPanel show={showShortcutPanel} onClose={() => showShortcutPanel = false} />
  <McpToolPanel show={showMcpTools} onClose={() => showMcpTools = false}
    onInsertTool={(text: string) => { input = text; showMcpTools = false; }}
    onToolCountChange={(n: number) => mcpToolCount = n}
    {onNavigate}
  />
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
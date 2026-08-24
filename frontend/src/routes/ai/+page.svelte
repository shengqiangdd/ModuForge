<script lang="ts">
import { onMount, onDestroy, tick } from 'svelte';
import { toast } from '$lib/stores/toast.svelte';

// ─── Components ───
import ChatSidebar from './components/ChatSidebar.svelte';
import ChatInput from './components/ChatInput.svelte';
import ChatMessages from './components/ChatMessages.svelte';
import ChatControls from './components/ChatControls.svelte';
import MetricsPanel from './components/MetricsPanel.svelte';
import ProgressIndicator from './components/ProgressIndicator.svelte';
import AutoBuildProjectCard from './components/AutoBuildProjectCard.svelte';
import GatherSpecCard from './components/GatherSpecCard.svelte';
import GeneratedFilesPanel from './components/GeneratedFilesPanel.svelte';
import RepoReferencePanel from './components/RepoReferencePanel.svelte';
import BuildProgressBar from './components/BuildProgressBar.svelte';
import McpPermissionModal from './components/McpPermissionModal.svelte';
import OnboardingGuide from './components/OnboardingGuide.svelte';
import ShortcutPanel from './components/ShortcutPanel.svelte';
import McpToolPanel from './components/McpToolPanel.svelte';
import TodoList from './components/TodoList.svelte';
import type { Subtask } from './components/TodoList.svelte';
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

// ─── Lib ───
import { generateUUID, safeCopyText } from './lib/utils';
import type { Mode, TokenUsage, AgentStep, Provider, Model, AIPrompt, GenHistoryItem, Message, AutoBuildPhase, ContextProject, ComparisonResult, SecurityScanResult } from './lib/types';
import { MODES } from './lib/types';
import { memoExtractFiles, preRenderVisibleMessages } from './lib/markdown';
import { StreamHandler } from './lib/stream-handler';
import { createStreamHandler } from './lib/handler-setup';
import { setupOnInit, loadInitialData, setupCleanup } from './lib/init';
import * as cb from './lib/callbacks';

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
  let showSearch = $state(false);
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
  let showRepoReference = $state(false);
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

  // MCP permission
  let pendingPermission = $state<{
    request_id: string; server: string; tool: string;
    args: Record<string, unknown>; timeout_s: number;
  } | null>(null);
  let permissionBusy = $state(false);

  // ─── Stream handler ───
  let handler = $state<StreamHandler>(null!);

  // ─── State accessor for callbacks ───
  // (callbacks read/write state via direct variable references)
  const state = $derived({
    providers, selectedProviderID, selectedModelID, configLoaded, refreshing,
    showModelDropdown, editingModelMaxTokens, editMaxTokensValue,
    mode, input, messages, streaming, buildLog, expandedReasoning,
    messageUsages, messageTimes, editingMessageIdx, editingMessageText,
    deletingMessageIdx, showDeleteConfirm, showPromptSettings, showMDPrompts,
    promptTab, prompts, promptDraft, promptSaving, promptLoading,
    showProviderConfig, configEndpoint, configApiKey, configSaving,
    showCapability, showMcpTools, mcpToolCount, capability, capabilityLoading,
    gatheredSpec, showSpecCard, currentStepIndex, progressStepDetails,
    genHistory, agentSteps, allAgentSteps, selectedRound, maxRoundIndex,
    expandedSteps, sessionId, agentMode, subtasks, todoCollapsed,
    autoBuildPhases, autoBuildFiles, autoBuildProjectId, autoBuildProjectName,
    stepStartTime, stepElapsed, sessions, sessionsTotal, sessionsLoading,
    hasMoreMessages, loadingEarlier, activeSessionId, searchResults,
    diffDiffs, diffFilePath, showDiffPanel, buildProgress, buildProgressActive,
    showPreviewModal, previewFiles, scanResult, scanning, showSecurityWarning,
    pendingImportFiles, showImportDialog, importFiles, importProjects,
    selectedImportProject, importing, showOnboarding, showShortcutPanel,
    showSearch, onboardingDone, viewMode, generatedFiles, showGeneratedFiles,
    progressCollapsed, projectCardCollapsed, agentStepsCollapsed,
    agentHadFinalAnswer, projectContext, showProjectContext, showRepoReference,
    contextProjects, selectedContextProject, selectedContextFile,
    contextProjectList, showComparison, comparisonResults, comparisonInput,
    comparisonRunning, showHistorySidebar, historyTab, convSaving, convLoading,
    savedConversations, showPromptTemplates, showExportMenu,
    pendingPermission, permissionBusy,
    availableModels, freeModels, paidModels, selectedModel,
  } as any);

  // ─── Lifecycle ───
  onMount(async () => {
    handler = createStreamHandler(state as any, null, loadProjectFiles, loadConversations, () => cb.loadGenHistory(state as any));
    // Re-wire chatMessages ref for scrollToBottom
    (handler as any)._chatMessages = () => chatMessages;
    setupOnInit(handler);
    await loadInitialData({
      loadProviders: () => cb.loadProviders().then(r => {
        state.providers = r.providers;
        state.selectedProviderID = r.selectedProviderID;
        state.selectedModelID = r.selectedModelID;
        state.configLoaded = true;
      }),
      loadSessions: () => cb.loadSessions(state as any),
      loadConversations: () => cb.loadConversations(state as any),
      loadGenHistory: () => cb.loadGenHistory(state as any),
      loadContextProjectList: () => cb.loadContextProjectList(state as any),
      loadSessionMessages: (id: string) => cb.loadSessionMessages(state as any, id),
      activeSessionId: state.activeSessionId,
      sessions: state.sessions,
      messages: state.messages,
      updateState: (partial: Record<string, any>) => Object.assign(state, partial),
    });
  });

  onDestroy(() => {
    setupCleanup(handler);
  });

  // ─── Delegate to extracted modules ───
  // (thin wrappers that bind the state accessor)

  async function loadProjectFiles(projectId: string) { await cb.loadProjectFiles(state as any, projectId); }
  async function loadSessionMessages(sessId: string) { await cb.loadSessionMessages(state as any, sessId); }
  async function loadConversations() { await cb.loadConversations(state as any); }
  async function loadSessions() { await cb.loadSessions(state as any); }
  function loadGenHistory() { cb.loadGenHistory(state as any); }
  async function loadContextProjectList() { await cb.loadContextProjectList(state as any); }

  // ─── Callbacks used in template ───
  function handleNewConversation() { cb.handleNewConversation(state as any); }
  function handleRefreshSidebar() { cb.handleRefreshSidebar(state as any, loadConversations, loadSessions, loadGenHistory); }
  function handleTabChange(tab: 'conversations' | 'generations') { cb.handleTabChange(state as any, tab); }
  function handleSearchSessions(q: string) { cb.handleSearchSessions(state as any, q); }
  function handleClearGenHistory() { cb.handleClearGenHistory(state as any); }
  function handleToggleAgentStep(idx: number) { cb.handleToggleAgentStep(state as any, idx); }
  function handleSetAgentMode(m: 'plan' | 'act') { cb.handleSetAgentMode(state as any, m); }
  function switchMode(m: Mode) { cb.switchMode(state as any, m); }
  function switchToGenerateWithSpec() { cb.switchToGenerateWithSpec(state as any, () => handler.send()); }
  function handleCopyText(text: string) { cb.handleCopyText(text); }
  function handleInsertToInput(text: string) { input = text; }
  function addRepoReference(text: string) { cb.addRepoReference(state as any, text); }
  function openImportDialog(messageIndex: number) { cb.openImportDialog(state as any, messageIndex); }
  function openPreview(files: {path: string; content: string}[]) { cb.openPreview(state as any, files); }
  function editMessage(idx: number) { cb.editMessage(state as any, idx); }
  function confirmDeleteMessage(idx: number) { cb.confirmDeleteMessage(state as any, idx); }
  function replyToMessage(idx: number) { cb.replyToMessage(state as any, idx); }
  function deleteMessage() { cb.deleteMessage(state as any, handler); }
  function deployAutoBuild() { cb.deployAutoBuild(state as any); }
  function runComparison() { cb.runComparison(state as any); }
  async function loadCapability() { await cb.loadCapability(state as any); }
  async function openPromptSettings() { await cb.openPromptSettings(state as any); }
  async function savePrompt() { await cb.savePrompt(state as any); }
  async function resetPrompt() { await cb.resetPrompt(state as any); }
  async function switchPromptTab(m: Mode) { await cb.switchPromptTab(state as any, m); }
  async function refreshModels() { await cb.refreshModels(state as any); }
  async function saveProviderConfig() { await cb.saveProviderConfig(state as any); }
  async function openProviderConfig() { await cb.openProviderConfig(state as any); }
  async function onSaveMaxTokens(modelId: string, value: string) { await cb.onSaveMaxTokens(state as any, modelId, value); }
  async function saveConversation() { await cb.saveConversation(state as any, loadConversations); }
  async function loadConversation(id: string) { await cb.loadConversation(state as any, loadProjectFiles, loadSessionMessages, id); }
  async function deleteConversation(id: string) { await cb.deleteConversation(state as any, loadConversations, id); }
  async function loadMoreSessions() { await cb.loadMoreSessions(state as any); }
  async function deleteSession(targetId: string) { await cb.deleteSession(state as any, loadSessions, targetId); }
  async function exportSession(targetId: string, format: 'markdown' | 'json' = 'markdown') { await cb.exportSession(state as any, targetId, format); }
  async function renameSession(targetId: string, title: string) { await cb.renameSession(state as any, loadSessions, targetId, title); }
  async function loadEarlierMessages() { await cb.loadEarlierMessages(state as any); }
  function exportConversation(format: 'json' | 'markdown') { cb.exportConversation(state as any, format); }
  function scanAndImport() { cb.scanAndImport(state as any); }
  function proceedImport() { cb.proceedImport(state as any); }
  function continueImportAfterWarning() { cb.continueImportAfterWarning(state as any); }
  function resolvePermission(allow: boolean) { cb.resolvePermission(state as any, allow); }
  function permissionArgsPreview() { return cb.permissionArgsPreview(state as any); }
  function handleToggleReasoning(idx: number) { cb.handleToggleReasoning(state as any, idx); }
  function addToContext(filePath: string) {
    if (!filePath) return;
    projectContext = cb.addToContextString ? (projectContext ? projectContext + '\n' : '') + '文件: ' + filePath : projectContext;
    toast(`已添加 ${filePath.split('/').pop()} 到上下文`, 'success');
  }

  // ─── Keyboard handler ───
  function handleKeydown(e: KeyboardEvent) {
    const tag = (e.target as HTMLElement)?.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || (e.target as HTMLElement)?.contentEditable === 'true') return;
    if (e.ctrlKey && e.key === 'k') { e.preventDefault(); handleNewConversation(); }
    if (e.ctrlKey && e.key === 'e') { e.preventDefault(); if (messages.length > 0) exportConversation('markdown'); }
    if (e.key === '?') { e.preventDefault(); showShortcutPanel = !showShortcutPanel; }
    if (e.key === 'Escape') {
      showHistorySidebar = false; showPromptSettings = false; showMDPrompts = false;
      showProviderConfig = false; showPreviewModal = false; showImportDialog = false;
      showComparison = false; showPromptTemplates = false; showDiffPanel = false;
      showCapability = false; showMcpTools = false; showShortcutPanel = false;
      showRepoReference = false;
    }
    if (!e.ctrlKey && !e.metaKey && !e.altKey && ['1','2','3','4','5','6'].includes(e.key)) {
      const idx = parseInt(e.key) - 1;
      if (idx >= 0 && idx < modes.length && !streaming && mode !== modes[idx].value) {
        switchMode(modes[idx].value);
      }
    }
  }

  function handleModelDropdownClickOutside(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest('.top-bar-model-wrap')) showModelDropdown = false;
  }
</script>

<div class="flex h-full ai-page" role="presentation" onkeydown={handleKeydown}>
  {#if showHistorySidebar}
    <ChatSidebar {sessions} {sessionsTotal} {sessionsLoading} onLoadMore={loadMoreSessions} {savedConversations} {genHistory} {convSaving} {convLoading} {historyTab} {activeSessionId} {searchResults} messagesLength={messages.length}
      onNewConversation={handleNewConversation}
      onRefresh={handleRefreshSidebar} onSave={saveConversation}
      onClose={() => showHistorySidebar = false} onTabChange={handleTabChange}
      onSearch={handleSearchSessions}
      onSelectConversation={loadConversation} onSelectSession={loadSessionMessages}
      onDeleteSession={deleteSession} onExportSession={exportSession} onRenameSession={renameSession}
      onDeleteConversation={deleteConversation}
      onRestoreHistory={(item) => { if (item.messages && item.messages.length > 0) { messages = item.messages.map((m: any) => ({ role: m.role, content: m.content })); activeSessionId = ''; sessionId = generateUUID(); streaming = false; currentStepIndex = -1; progressStepDetails = []; expandedReasoning = new Set(); agentSteps = []; allAgentSteps = []; selectedRound = -1; maxRoundIndex = 0; subtasks = []; if (item.mode && modes.some((m: any) => m.value === item.mode)) mode = item.mode as Mode; showHistorySidebar = false; toast('已加载生成记录', 'success'); } }}
      onClearHistory={handleClearGenHistory}
    />
  {/if}

  <div class="flex-1 flex flex-col min-w-0">
    <ChatControls
      {providers} {selectedProviderID} {selectedModelID} {configLoaded}
      {showModelDropdown} {editingModelMaxTokens} {editMaxTokensValue}
      {availableModels} {freeModels} {paidModels} {selectedModel}
      {mode} {streaming} {showComparison} {showProjectContext} {showRepoReference}
      {showHistorySidebar} {showCapability} {showMcpTools}
      onProviderChange={(v) => { selectedProviderID = v; cb.applyProviderChange(v, availableModels); }}
      onModelSelect={(id) => { selectedModelID = id; showModelDropdown = false; cb.onModelSelect(state as any, id); }}
      onEditMaxTokens={(id, val) => { editingModelMaxTokens = id; editMaxTokensValue = val; }}
      onSaveMaxTokens={onSaveMaxTokens}
      onToggleDropdown={() => showModelDropdown = !showModelDropdown}
      onEditMaxTokensStart={(id, val) => { editingModelMaxTokens = id; editMaxTokensValue = val; }}
      onModeChange={(m) => { if (mode !== m) switchMode(m); }}
      onToggleComparison={() => showComparison = !showComparison}
      onToggleProjectContext={() => showProjectContext = !showProjectContext}
      onToggleRepoReference={() => showRepoReference = !showRepoReference}
      onToggleHistory={() => { if (!showHistorySidebar) { loadConversations(); loadSessions(); loadGenHistory(); } showHistorySidebar = !showHistorySidebar; }}
      onLoadCapability={() => { loadCapability(); showCapability = !showCapability; }}
      onToggleMcpTools={() => showMcpTools = !showMcpTools}
      onOpenPromptSettings={openPromptSettings}
      onOpenMDPrompts={() => showMDPrompts = true}
      {onNavigate}
      {showSearch}
      onToggleSearch={() => { showSearch = !showSearch; if (!showSearch) chatMessages?.closeSearch?.(); }}
    />

    <MetricsPanel inputPricePerM={selectedModel?.price_input_per_m || 0} outputPricePerM={selectedModel?.price_output_per_m || 0} />

    <ProgressIndicator show={streaming && currentStepIndex >= 0 && (mode === 'generate' || mode === 'auto-build')} {streaming} {currentStepIndex} {progressStepDetails} {stepElapsed} {progressCollapsed} onToggleCollapse={() => progressCollapsed = !progressCollapsed} />
    <AutoBuildProjectCard projectId={autoBuildProjectId} projectName={autoBuildProjectName} fileCount={autoBuildFiles.length} collapsed={projectCardCollapsed} onToggleCollapse={() => projectCardCollapsed = !projectCardCollapsed} />

    <TodoList {subtasks} collapsed={todoCollapsed} onToggleCollapse={() => todoCollapsed = !todoCollapsed} />

    <ChatMessages bind:this={chatMessages}
      bind:messages {mode} {streaming} {expandedReasoning} {messageUsages} {messageTimes}
      {hasMoreMessages} {loadingEarlier} onLoadEarlier={loadEarlierMessages}
      allAgentSteps={allAgentSteps} agentExpandedSteps={expandedSteps}
      onToggleAgentStep={handleToggleAgentStep}
      onToggleReasoning={handleToggleReasoning}
      onEdit={editMessage} onDelete={confirmDeleteMessage} onReply={replyToMessage}
      onCopy={handleCopyText}
      onOpenImportDialog={openImportDialog} onOpenPreview={openPreview}
      onInsertToInput={handleInsertToInput}
      {showSearch}
      onSearchClose={() => showSearch = false}
    />

    <GatherSpecCard show={showSpecCard} spec={gatheredSpec} onClose={() => showSpecCard = false} onGenerate={switchToGenerateWithSpec} />
    <GeneratedFilesPanel show={showGeneratedFiles} files={generatedFiles} {mode} {viewMode} onClose={() => showGeneratedFiles = false} onViewModeChange={(m) => viewMode = m} onDeploy={deployAutoBuild} />
    <RepoReferencePanel show={showRepoReference} onClose={() => showRepoReference = false} onAddReference={addRepoReference} />

    <ChatInput {input} {mode} {streaming} {buildLog} {mcpToolCount}
      {showProjectContext} {contextProjectList} {contextProjects} selectedProject={selectedContextProject} selectedFile={selectedContextFile} {projectContext}
      onToggleProjectContext={() => showProjectContext = !showProjectContext}
      onProjectChange={(v) => { selectedContextProject = v; if (v) loadProjectFiles(v); }}
      onFileAdd={(v) => { if (v) { projectContext += (projectContext ? '\n' : '') + '文件: ' + v; selectedContextFile = ''; } }}
      onContextChange={(v) => projectContext = v}
      onSend={() => handler.send()} onStop={handler.stopStream}
      onInputChange={(v) => input = v}
      onBuildLogChange={(v) => buildLog = v}
      onOpenMcpTools={() => showMcpTools = true}
    />
  </div>

  <McpPermissionModal permission={pendingPermission} busy={permissionBusy} argsPreview={permissionArgsPreview()} onResolve={resolvePermission} onClose={() => pendingPermission = null} />

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
    .ai-page :global(.ai-input-area) { padding: 8px; padding-bottom: calc(8px + env(safe-area-inset-bottom, 0px)); }
    .ai-page :global(.ai-input-area textarea) { min-height: 60px !important; }
    .ai-page :global(.prompt-modal-overlay) { align-items: stretch !important; padding: 0 !important; }
    .ai-page :global(.prompt-modal-overlay > div) { max-width: 100% !important; max-height: 100% !important; border-radius: 0 !important; width: 100%; height: 100%; }
    .ai-page :global(.prompt-modal-overlay textarea) { height: 60vh !important; }
    .ai-page :global(.messages-area) { padding: 6px; min-height: 0; flex: 1 1 0%; }
  }
</style>

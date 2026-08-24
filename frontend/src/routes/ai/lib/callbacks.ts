// ─── Callback functions for AI page — extracted from +page.svelte ───
// These functions accept individual state parameters and return state updates.
// The main page applies returned updates to trigger Svelte reactivity.
import { toast } from "$lib/stores/toast.svelte";
import type { Mode, AgentStep, Provider, Model, AIPrompt, GenHistoryItem, Message, ProgressStepDetail, AutoBuildPhase, ContextProject, ComparisonResult, SecurityScanResult } from './types';
import { MODES } from "./types";
import { generateUUID, safeCopyText, redactArgValues } from "./utils";
import { memoExtractFiles } from "./markdown";
import { authFetch } from './history';
import { loadProvidersFromBackend, saveModelSelectionToStorage, saveConfigToBackend,
  refreshModelsFromBackend, saveModelMaxTokens, loadProviderConfig,
  saveProviderConfigToBackend, fetchCapability
} from './provider';
import { loadPrompts as loadPromptsFromBackend, savePromptToBackend, resetPromptToDefault } from './prompts';
import { loadImportProjects as loadImportProjectsFromBackend, scanFiles, importFilesToProject } from './import-scan';
import { loadGenHistory as loadGenHistoryFromStorage, loadSessionsList, deleteSessionById, exportSessionById, renameSessionById,
  fetchSessionMessages, exportConversationToFile, searchSessions,
  fetchProjectFiles, deployToAdb, fetchConversations, fetchConversation,
  deleteConversationById, saveConversationToBackend
} from './history';
import { truncateForRegeneration, editMessageContent, deleteMessageAt
} from './messages';
import { loadProjectFilesState, loadContextProjectListState } from './context';
import { filterStepsByRound } from "./rounds";
import type { Subtask } from '../components/TodoList.svelte';

const modes = MODES;
const SESSIONS_PAGE_SIZE = 50;

// ─── Pure data helpers ───

export function newConversationState() {
  return {
    messages: [] as Message[], currentStepIndex: -1, progressStepDetails: [] as ProgressStepDetail[],
    autoBuildPhases: [] as AutoBuildPhase[], agentSteps: [] as AgentStep[], allAgentSteps: [] as AgentStep[],
    selectedRound: -1, maxRoundIndex: 0, expandedReasoning: new Set<number>(), messageUsages: new Map<number, any>(),
    activeSessionId: '', sessionId: generateUUID(), mode: 'generate' as Mode,
    showHistorySidebar: false, autoBuildProjectId: '', autoBuildProjectName: '',
    autoBuildFiles: [] as {path:string;content:string;size:number}[], subtasks: [] as Subtask[],
  };
}

export function resetModeState(mode: Mode) {
  return {
    messages: [] as Message[], currentStepIndex: -1, progressStepDetails: [] as ProgressStepDetail[],
    autoBuildPhases: [] as AutoBuildPhase[], agentSteps: [] as AgentStep[],
    expandedReasoning: new Set<number>(), activeSessionId: '',
    sessionId: generateUUID(), mode,
  };
}

// ─── Provider / Model ───

export async function loadProviders() {
  return await loadProvidersFromBackend();
}

export function applyProviderChange(selectedProviderID: string, availableModels: Model[]) {
  const newModelId = availableModels.length > 0 ? availableModels[0].id : '';
  saveModelSelectionToStorage(selectedProviderID, newModelId);
  saveConfigToBackend(selectedProviderID, newModelId);
  return { selectedProviderID, selectedModelID: newModelId };
}

export async function onModelSelect(selectedProviderID: string, modelId: string) {
  saveModelSelectionToStorage(selectedProviderID, modelId);
  await saveConfigToBackend(selectedProviderID, modelId);
}

export async function refreshModels() {
  await refreshModelsFromBackend();
  return await loadProvidersFromBackend();
}

export async function onSaveMaxTokens(selectedProviderID: string, providers: Provider[], modelId: string, value: string) {
  const maxTokens = parseInt(value);
  if (isNaN(maxTokens) || maxTokens <= 0) { toast('请输入有效的 token 数', 'error'); return false; }
  await saveModelMaxTokens(selectedProviderID, providers, modelId, maxTokens);
  return true;
}

// ─── Provider Config ───

export async function saveProviderConfig(selectedProviderID: string, configEndpoint: string, configApiKey: string) {
  const ok = await saveProviderConfigToBackend(selectedProviderID, configEndpoint, configApiKey);
  return ok;
}

export async function openProviderConfig(selectedProviderID: string) {
  const cfg = await loadProviderConfig(selectedProviderID);
  return { endpoint: cfg.endpoint, apiKey: cfg.apiKey };
}

// ─── Prompt management ───

export async function loadPrompts() {
  return await loadPromptsFromBackend();
}

export async function loadPromptForMode(prompts: AIPrompt[], promptTab: Mode) {
  const updated = await loadPromptsFromBackend();
  const p = updated.find(x => x.mode === promptTab);
  return { prompts: updated, promptDraft: p?.content || '' };
}

export async function savePrompt(promptTab: Mode, promptDraft: string) {
  const ok = await savePromptToBackend(promptTab, promptDraft);
  return ok;
}

export async function resetPromptToDefaultAction(promptTab: Mode) {
  return await resetPromptToDefault(promptTab);
}

// ─── Import ───

export async function loadImportProjectsList() {
  return await loadImportProjectsFromBackend();
}

export function extractImportFiles(messages: Message[], messageIndex: number) {
  const msg = messages[messageIndex];
  if (!msg) return null;
  const files = memoExtractFiles(msg.content);
  if (!files) return null;
  return files;
}

export async function scanAndImportFiles(selectedImportProject: string, importFiles: {path:string;content:string}[]) {
  if (!selectedImportProject || importFiles.length === 0) return null;
  const scanResult = await scanFiles(importFiles);
  if (scanResult && !scanResult.safe) {
    const criticalIssues = scanResult.issues.filter(i => i.severity === 'critical');
    if (criticalIssues.length > 0) return { needWarning: true, scanResult };
  }
  const result = await importFilesToProject(selectedImportProject, importFiles);
  return { needWarning: false, scanResult, result };
}

export async function doImportFiles(selectedImportProject: string, importFiles: {path:string;content:string}[]) {
  if (!selectedImportProject || importFiles.length === 0) return null;
  return await importFilesToProject(selectedImportProject, importFiles);
}

// ─── Capability ───

export async function loadCapabilityData() {
  return await fetchCapability();
}

// ─── Conversation ───

export async function loadConversationsList() {
  return await fetchConversations();
}

export async function loadConversationData(id: string) {
  return await fetchConversation(id);
}

export async function deleteConversationByIdAction(id: string) {
  return await deleteConversationById(id);
}

export async function saveConversationData(data: {
  id: string; title: string; mode: Mode; messages: Message[];
  model: string; project_id: string;
}) {
  return await saveConversationToBackend(data);
}

// ─── Session management ───

export async function loadSessionsListData(offset: number, limit: number) {
  return await loadSessionsList(offset, limit);
}

export async function deleteSessionByIdAction(targetSessionId: string) {
  return await deleteSessionById(targetSessionId);
}

export async function exportSessionByIdAction(targetSessionId: string, format: 'markdown' | 'json') {
  return await exportSessionById(targetSessionId, format);
}

export async function renameSessionByIdAction(targetSessionId: string, title: string) {
  return await renameSessionById(targetSessionId, title);
}

export async function fetchSessionMessagesData(sessId: string, limit: number, before?: string, beforeId?: string) {
  return await fetchSessionMessages(sessId, limit, before, beforeId);
}

export function exportConversationToFileAction(messages: Message[], format: 'json' | 'markdown') {
  exportConversationToFile(messages, format);
}

export async function searchSessionsData(query: string) {
  return await searchSessions(query);
}

export async function deployToAdbAction(files: {path: string; content: string}[]) {
  return await deployToAdb(files);
}

// ─── Message editing ───

export function truncateForRegenerationAction(messages: Message[]) {
  if (messages.length < 2) return null;
  return truncateForRegeneration(messages);
}

export function editMessageContentAction(messages: Message[], idx: number, text: string) {
  return editMessageContent(messages, idx, text);
}

export function deleteMessageAtAction(messages: Message[], idx: number) {
  return deleteMessageAt(messages, idx);
}

// ─── Project files ───

export async function loadProjectFilesData(projectId: string) {
  try {
    return await loadProjectFilesState(projectId);
  } catch { return null; }
}

export async function loadContextProjectListData() {
  try {
    return await loadContextProjectListState();
  } catch { return []; }
}

// ─── Agent step round filtering ───

export function filterStepsByRoundAction(allAgentSteps: AgentStep[], round: number) {
  return filterStepsByRound(allAgentSteps, round);
}

// ─── Comparison ───

export async function runComparisonAction(
  providers: Provider[],
  comparisonInput: string,
  onProgress: (results: ComparisonResult[]) => void,
) {
  if (!comparisonInput.trim()) return [];
  const results: ComparisonResult[] = [];
  for (const p of providers) {
    for (const m of p.models.slice(0, 1)) {
      const start = Date.now();
      try {
        const res = await authFetch('/api/v1/ai/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ message: comparisonInput, provider_id: p.id, model: m.id, messages: [{ role: 'user', content: comparisonInput }] }),
        });
        const data = await res.json();
        results.push({ model: `${p.name} / ${m.name}`, response: data.content || data.error || 'No response', time: Date.now() - start });
      } catch {
        results.push({ model: `${p.name} / ${m.name}`, response: 'Error', time: Date.now() - start });
      }
      onProgress([...results]);
    }
  }
  return results;
}

// ─── Copy helper ───

export function handleCopyText(text: string) {
  safeCopyText(text).then(ok => { if (ok) toast('已复制', 'success'); });
}

// ─── MCP permission ───
// redactArgValues imported from utils (shared)

export function permissionArgsPreview(args: Record<string, unknown> | undefined): string {
  if (!args || Object.keys(args).length === 0) return '（无参数）';
  try { return JSON.stringify(redactArgValues(args), null, 2); } catch { return String(args); }
}

export async function resolvePermissionAction(requestId: string, allow: boolean) {
  const res = await authFetch('/api/v1/agent/mcp/confirm', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ request_id: requestId, allow }),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || '确认请求失败（可能已超时）');
  }
}

// ─── Re-exports from stores (for backward compatibility) ───
import {
  loadSessions as loadSessionsFromStore,
  loadMoreSessions as loadMoreSessionsFromStore,
  deleteSession as deleteSessionFromStore,
  exportSession as exportSessionFromStore,
  renameSession as renameSessionFromStore,
  loadConversations as loadConversationsFromStore,
  deleteConversation as deleteConversationFromStore,
  loadGenHistory as loadGenHistoryFromStore,
  loadSessionMessages as loadSessionMessagesFromStore,
  loadEarlierMessages as loadEarlierMessagesFromStore,
  saveConversation as saveConversationFromStore,
  exportConversation as exportConversationFromStore,
  deployAutoBuild as deployAutoBuildFromStore,
  runComparison as runComparisonFromStore,
  resolvePermission as resolvePermissionFromStore,
  loadProjectFilesStateFn as loadProjectFilesStateFnFromStore,
  loadContextProjectListStateFn as loadContextProjectListStateFnFromStore,
} from './stores/conversations';


// ─── Aliases for page compatibility ───
export const loadGenHistory = loadGenHistoryFromStore;
export const loadConversations = loadConversationsFromStore;
export const loadSessions = loadSessionsFromStore;
export const loadMoreSessions = loadMoreSessionsFromStore;
export const loadSessionMessages = loadSessionMessagesFromStore;
export const loadEarlierMessages = loadEarlierMessagesFromStore;
export const loadProjectFiles = loadProjectFilesStateFnFromStore;
export const loadContextProjectList = loadContextProjectListStateFnFromStore;
export const deleteSession = deleteSessionFromStore;
export const deleteConversation = deleteConversationFromStore;
export const saveConversation = saveConversationFromStore;
export const exportConversation = exportConversationFromStore;
export const exportSession = exportSessionFromStore;
export const renameSession = renameSessionFromStore;
export const deployAutoBuild = deployAutoBuildFromStore;
export const runComparison = runComparisonFromStore;
export const resolvePermission = resolvePermissionFromStore;
export const handleToggleAgentStep = filterStepsByRound;

// ─── Simple wrapper functions ───
export function switchMode(s: any, mode: Mode) { return resetModeState(mode); }
export function switchToGenerateWithSpec(s: any) { return resetModeState('generate'); }
export function addRepoReference(s: any, ref: string) { s.projectContext = (s.projectContext || '') + '\n' + ref; }
export function openImportDialog(s: any) { s.showImportDialog = true; }
export function openPreview(s: any, files: any[]) { s.previewFiles = files; s.showPreviewModal = true; }
export function openPromptSettings(s: any) { s.showPromptSettings = true; }
export function switchPromptTab(s: any, tab: Mode) { s.promptTab = tab; }
export function handleClearGenHistory(s: any) { s.genHistory = []; }
export function handleNewConversation(s: any) { Object.assign(s, newConversationState()); }
export function handleRefreshSidebar(s: any, loadConvs: any, loadSess: any, loadGen: any) { loadConvs(); loadSess(); loadGen(); }
export function handleTabChange(s: any, tab: string) { s.historyTab = tab; }
export function handleSearchSessions(s: any, q: string) { searchSessionsData(q); }
export function handleToggleReasoning(s: any, idx: number) {
  const set = new Set(s.expandedReasoning);
  if (set.has(idx)) set.delete(idx); else set.add(idx);
  s.expandedReasoning = set;
}
export function editMessage(s: any, idx: number, text: string) { return editMessageContentAction(s.messages, idx, text); }
export function confirmDeleteMessage(s: any, idx: number) { return deleteMessageAtAction(s.messages, idx); }
export function replyToMessage(s: any, idx: number) { s.editingMessageIdx = idx; }
export function deleteMessage(s: any, idx: number) { return deleteMessageAtAction(s.messages, idx); }
export function loadCapability(s: any) { return loadCapabilityData(); }
export function scanAndImport(s: any, project: string, files: any[]) { return scanAndImportFiles(project, files); }
export function continueImportAfterWarning(s: any, project: string, files: any[]) { return doImportFiles(project, files); }
export function proceedImport(s: any, project: string, files: any[]) { return doImportFiles(project, files); }

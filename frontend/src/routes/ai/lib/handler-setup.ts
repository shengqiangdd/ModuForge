// ─── StreamHandler setup factory — extracted from +page.svelte ───
import { tick } from 'svelte';
import { toast } from '$lib/stores/toast.svelte';
import { StreamHandler } from './stream-handler';
import type { Message, AgentStep, AutoBuildPhase, ProgressStepDetail, Mode, GenHistoryItem, Subtask, Provider, Model } from './types';

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
  messageUsages: Map<number, any>;
  messageTimes: Map<number, number>;
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
  providers: Provider[];
}

export interface StreamHandlerCallbacks {
  loadProjectFiles: (projectId: string) => Promise<void>;
  loadConversations: () => Promise<void>;
  onPermissionRequest?: (req: {
    request_id: string;
    server: string;
    tool: string;
    args: Record<string, unknown>;
    timeout_s: number;
  }) => void;
  loadGenHistory: () => Promise<void>;
  saveConfigToBackend: (providerId: string, modelId: string) => Promise<void>;
  scrollToBottom: () => void | Promise<void>;
  toast: (msg: string, type: 'success' | 'error' | 'warning' | 'info') => void;
}

export function createStreamHandler(
  s: StreamHandlerState,
  cb: StreamHandlerCallbacks,
) {
  return new StreamHandler({
    get messages() { return s.messages; },
    set messages(v) { s.messages = v; },
    get streaming() { return s.streaming; },
    set streaming(v) { s.streaming = v; },
    get configLoaded() { return s.configLoaded; },
    set configLoaded(v) { s.configLoaded = v; },
    get currentStepIndex() { return s.currentStepIndex; },
    set currentStepIndex(v) { s.currentStepIndex = v; },
    get progressStepDetails() { return s.progressStepDetails; },
    set progressStepDetails(v) { s.progressStepDetails = v; },
    get agentSteps() { return s.agentSteps; },
    set agentSteps(v) { s.agentSteps = v; },
    get allAgentSteps() { return s.allAgentSteps; },
    set allAgentSteps(v) { s.allAgentSteps = v; },
    get agentStepsCollapsed() { return s.agentStepsCollapsed; },
    set agentStepsCollapsed(v) { s.agentStepsCollapsed = v; },
    get selectedRound() { return s.selectedRound; },
    set selectedRound(v) { s.selectedRound = v; },
    get maxRoundIndex() { return s.maxRoundIndex; },
    set maxRoundIndex(v) { s.maxRoundIndex = v; },
    get expandedReasoning() { return s.expandedReasoning; },
    set expandedReasoning(v) { s.expandedReasoning = v; },
    get messageUsages() { return s.messageUsages; },
    set messageUsages(v) { s.messageUsages = v; },
    get messageTimes() { return s.messageTimes; },
    set messageTimes(v) { s.messageTimes = v; },
    get requestStartTime() { return 0; },
    set requestStartTime(_v) {},
    get lastStreamAssistantIdx() { return -1; },
    set lastStreamAssistantIdx(_v) {},
    get seenReadPaths() { return new Set<string>(); },
    set seenReadPaths(_v) {},
    get currentToolInput() { return null; },
    set currentToolInput(_v) {},
    get agentHadFinalAnswer() { return s.agentHadFinalAnswer; },
    set agentHadFinalAnswer(v) { s.agentHadFinalAnswer = v; },
    get subtasks() { return s.subtasks; },
    set subtasks(v) { s.subtasks = v; },
    get autoBuildPhases() { return s.autoBuildPhases; },
    set autoBuildPhases(v) { s.autoBuildPhases = v; },
    get autoBuildFiles() { return s.autoBuildFiles; },
    set autoBuildFiles(v) { s.autoBuildFiles = v; },
    get autoBuildProjectId() { return s.autoBuildProjectId; },
    set autoBuildProjectId(v) { s.autoBuildProjectId = v; },
    get autoBuildProjectName() { return s.autoBuildProjectName; },
    set autoBuildProjectName(v) { s.autoBuildProjectName = v; },
    get stepStartTime() { return s.stepStartTime; },
    set stepStartTime(v) { s.stepStartTime = v; },
    get stepElapsed() { return s.stepElapsed; },
    set stepElapsed(v) { s.stepElapsed = v; },
    get genHistory() { return s.genHistory; },
    set genHistory(v) { s.genHistory = v; },
    get gatheredSpec() { return s.gatheredSpec; },
    set gatheredSpec(v) { s.gatheredSpec = v; },
    get showSpecCard() { return s.showSpecCard; },
    set showSpecCard(v) { s.showSpecCard = v; },
    get showGeneratedFiles() { return s.showGeneratedFiles; },
    set showGeneratedFiles(v) { s.showGeneratedFiles = v; },
    get generatedFiles() { return s.generatedFiles; },
    set generatedFiles(v) { s.generatedFiles = v; },
    get buildLog() { return s.buildLog; },
    set buildLog(v) { s.buildLog = v; },
    get input() { return s.input; },
    set input(v) { s.input = v; },
    get mode() { return s.mode; },
    set mode(v) { s.mode = v; },
    get selectedProviderID() { return s.selectedProviderID; },
    set selectedProviderID(v) { s.selectedProviderID = v; },
    get selectedModelID() { return s.selectedModelID; },
    set selectedModelID(v) { s.selectedModelID = v; },
    get selectedModel() { return s.selectedModel; },
    set selectedModel(v) { s.selectedModel = v; },
    get selectedContextProject() { return s.selectedContextProject; },
    set selectedContextProject(v) { s.selectedContextProject = v; },
    get projectContext() { return s.projectContext; },
    set projectContext(v) { s.projectContext = v; },
    get agentMode() { return s.agentMode; },
    set agentMode(v) { s.agentMode = v; },
    get sessionId() { return s.sessionId; },
    set sessionId(v) { s.sessionId = v; },
    get activeSessionId() { return s.activeSessionId; },
    set activeSessionId(v) { s.activeSessionId = v; },
    get showHistorySidebar() { return s.showHistorySidebar; },
    set showHistorySidebar(v) { s.showHistorySidebar = v; },
    get providers() { return s.providers; },
    set providers(v) { s.providers = v; },
  }, cb);
}

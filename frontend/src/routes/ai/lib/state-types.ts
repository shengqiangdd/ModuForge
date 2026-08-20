// ─── Shared state interface for AI page extracted modules ───
import type { Mode, TokenUsage, AgentStep, Provider, Model, AIPrompt, GenHistoryItem, Message, ProgressStepDetail, AutoBuildPhase, ContextProject, ComparisonResult, SecurityScanResult } from './types';
import type { Subtask } from '../components/TodoList.svelte';

export interface AIPageState {
  // Core
  providers: Provider[];
  selectedProviderID: string;
  selectedModelID: string;
  configLoaded: boolean;
  refreshing: boolean;
  showModelDropdown: boolean;
  editingModelMaxTokens: string;
  editMaxTokensValue: string;
  mode: Mode;
  input: string;
  messages: Message[];
  streaming: boolean;
  buildLog: string;
  expandedReasoning: Set<number>;
  messageUsages: Map<number, TokenUsage>;
  messageTimes: Map<number, number>;

  // Prompt settings
  editingMessageIdx: number;
  editingMessageText: string;
  deletingMessageIdx: number;
  showDeleteConfirm: boolean;
  showPromptSettings: boolean;
  showMDPrompts: boolean;
  promptTab: Mode;
  prompts: AIPrompt[];
  promptDraft: string;
  promptSaving: boolean;
  promptLoading: boolean;

  // Provider config
  showProviderConfig: boolean;
  configEndpoint: string;
  configApiKey: string;
  configSaving: boolean;

  // AI Capability
  showCapability: boolean;
  showMcpTools: boolean;
  mcpToolCount: number;
  capability: any;
  capabilityLoading: boolean;

  // Gathered requirements
  gatheredSpec: any;
  showSpecCard: boolean;

  // Progress
  currentStepIndex: number;
  progressStepDetails: ProgressStepDetail[];

  // Generation history
  genHistory: GenHistoryItem[];

  // Agent state
  agentSteps: AgentStep[];
  allAgentSteps: AgentStep[];
  selectedRound: number;
  maxRoundIndex: number;
  expandedSteps: Set<number>;
  sessionId: string;
  agentMode: 'plan' | 'act';

  // Todo
  subtasks: Subtask[];
  todoCollapsed: boolean;

  // Auto-build
  autoBuildPhases: AutoBuildPhase[];
  autoBuildFiles: { path: string; content: string; size: number }[];
  autoBuildProjectId: string;
  autoBuildProjectName: string;
  stepStartTime: number;
  stepElapsed: string;

  // Sessions
  sessions: any[];
  sessionsTotal: number;
  sessionsLoading: boolean;
  hasMoreMessages: boolean;
  loadingEarlier: boolean;
  activeSessionId: string;
  searchResults: any[];

  // Diff
  diffDiffs: any[];
  diffFilePath: string;
  showDiffPanel: boolean;

  // Build progress
  buildProgress: { stage: string; progress: number; message: string } | null;
  buildProgressActive: boolean;

  // Preview
  showPreviewModal: boolean;
  previewFiles: { path: string; content: string }[];

  // Security scan
  scanResult: SecurityScanResult | null;
  scanning: boolean;
  showSecurityWarning: boolean;
  pendingImportFiles: { path: string; content: string }[];

  // Import
  showImportDialog: boolean;
  importFiles: { path: string; content: string }[];
  importProjects: { id: string; name: string }[];
  selectedImportProject: string;
  importing: boolean;

  // Onboarding & shortcuts
  showOnboarding: boolean;
  showShortcutPanel: boolean;
  showSearch: boolean;
  onboardingDone: boolean;

  // Generated files
  viewMode: 'diff' | 'files';
  generatedFiles: { path: string; content: string; oldContent?: string }[];
  showGeneratedFiles: boolean;

  // Collapsible panels
  progressCollapsed: boolean;
  projectCardCollapsed: boolean;
  agentStepsCollapsed: boolean;
  agentHadFinalAnswer: boolean;

  // Project context
  projectContext: string;
  showProjectContext: boolean;
  showRepoReference: boolean;
  contextProjects: ContextProject[];
  selectedContextProject: string;
  selectedContextFile: string;
  contextProjectList: { id: string; name: string }[];

  // Comparison
  showComparison: boolean;
  comparisonResults: ComparisonResult[];
  comparisonInput: string;
  comparisonRunning: boolean;

  // History sidebar
  showHistorySidebar: boolean;
  historyTab: 'conversations' | 'generations';
  convSaving: boolean;
  convLoading: boolean;
  savedConversations: any[];

  // Prompt templates
  showPromptTemplates: boolean;

  // Misc
  showExportMenu: boolean;

  // MCP permission
  pendingPermission: {
    request_id: string;
    server: string;
    tool: string;
    args: Record<string, unknown>;
    timeout_s: number;
  } | null;
  permissionBusy: boolean;
}

/** Derived values computed from state */
export interface AIDerived {
  availableModels: Model[];
  freeModels: Model[];
  paidModels: Model[];
  selectedModel: Model | null;
  lastAssistantIdx: number;
}

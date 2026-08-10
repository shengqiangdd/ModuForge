// ─── Shared types for AI page ───
export type Mode = 'generate' | 'chat' | 'repair' | 'agent' | 'gather' | 'auto-build';

export interface TokenUsage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

export interface AgentStepInput {
  path?: string;
  pattern?: string;
  command?: string;
  old_text?: string;
  content?: string;
  files?: string[];
  project_id?: string;
  [key: string]: unknown;
}

export interface AgentStep {
  type: 'think' | 'skill_call' | 'skill_result' | 'answer';
  skill?: string;
  input?: AgentStepInput;
  content?: string;
  round?: number;
}

export interface Subtask {
  id: string;
  description: string;
  status: 'pending' | 'in_progress' | 'completed' | 'failed' | 'running' | 'done' | 'error' | 'skipped';
  dependencies?: string[];
  files?: string[];
  tools?: string[];
  progress?: number;
  started_at?: string | number;
  completed_at?: string | number;
  retry_count?: number;
}

export interface Model {
  id: string; name: string; provider: string; max_tokens: number;
  supports_stream: boolean; price_input_per_m: number; price_output_per_m: number;
}

export interface Provider {
  name: string; id: string; endpoint: string; models: Model[];
  requires_key: boolean; is_free: boolean; tier: string;
  models_json?: string; api_key?: string;
}

export interface AIPrompt { id: string; mode: string; content: string; updated_at: string; }

export interface GenHistoryItem {
  id: string;
  title: string;
  timestamp: number;
  model: string;
  mode: string;
  messageCount: number;
  preview: string;
  messages?: { role: string; content: string }[];
}

export interface SavedConv {
  id: string;
  title: string;
  mode: string;
  model: string;
  message_count: number;
  created_at: string;
  updated_at: string;
}

export interface PreviewFile {
  path: string;
  content: string;
}

export interface SecurityIssue {
  severity: string;
  file: string;
  line: number;
  rule: string;
  message: string;
}

export interface SecurityScanResult {
  safe: boolean;
  issues: SecurityIssue[];
  score: number;
  summary: string;
}

export interface Message {
  role: string;
  content: string;
  round?: number;
  reasoning?: string;
}

export interface ProgressStepDetail {
  step: string;
  message: string;
  time: number;
}

export interface AutoBuildPhase {
  phase: string;
  message: string;
  status: 'pending' | 'running' | 'done' | 'error';
}

export interface ContextProject {
  id: string;
  name: string;
  files: string[];
}

export interface ComparisonResult {
  model: string;
  response: string;
  time: number;
}

export interface AnalysisMode {
  id: string;
  label: string;
  icon: string;
  prompt: string;
}

// ─── Constants ───
export const MODES = [
  { value: 'generate' as const, label: '生成模块', icon: 'auto_fix_high', desc: '描述需求，AI 生成通用模块代码' },
  { value: 'gather' as const, label: '需求收集', icon: 'checklist', desc: 'AI 引导你完善模块需求' },
  { value: 'chat' as const, label: 'AI 对话', icon: 'chat', desc: '与 AI 对话获取帮助' },
  { value: 'repair' as const, label: '修复构建', icon: 'build_circle', desc: '粘贴日志分析问题' },
  { value: 'agent' as const, label: 'Agent', icon: 'smart_toy', desc: 'AI Agent 自动完成任务' },
  { value: 'auto-build' as const, label: '智能构建', icon: 'rocket_launch', desc: 'AI 自动完成模块开发全流程' },
];

export const PROMPT_MODES = MODES.filter(m => m.value !== 'auto-build');

export const PROGRESS_LABELS: Record<string, string> = {
  start: '正在分析需求...',
  structure: '正在设计架构...',
  script: '正在生成代码...',
  system: '正在验证结构...',
  optimize: '正在构建部署...',
  done: '生成完成！',
};

export const AUTO_BUILD_PHASE_DEFS = [
  { phase: 'start', label: '连接AI', icon: 'smart_toy' },
  { phase: 'structure', label: '分析需求', icon: 'search' },
  { phase: 'script', label: '生成代码', icon: 'code' },
  { phase: 'system', label: '验证文件', icon: 'verified' },
  { phase: 'compile', label: '编译源码', icon: 'build' },
  { phase: 'optimize', label: '构建完成', icon: 'check_circle' },
];

export const PROMPT_TEMPLATES = [
  { id: 'module_gen', name: '模块生成', prompt: '请生成一个兼容 Magisk/KernelSU/APatch 的模块，功能如下：\n\n' },
  { id: 'code_review', name: '代码审查', prompt: '请审查以下代码，指出潜在问题和改进建议：\n\n' },
  { id: 'explain_code', name: '代码解释', prompt: '请解释以下代码的功能和逻辑：\n\n' },
  { id: 'bug_fix', name: 'Bug 修复', prompt: '请分析以下代码中的 bug 并给出修复方案：\n\n' },
  { id: 'optimize', name: '代码优化', prompt: '请优化以下代码，提高性能和可读性：\n\n' },
  { id: 'security_audit', name: '安全审计', prompt: '请对以下代码进行安全审计，指出潜在漏洞：\n\n' },
  { id: 'test_generation', name: '测试生成', prompt: '请为以下代码生成单元测试：\n\n' },
  { id: 'documentation', name: '文档生成', prompt: '请为以下代码生成详细文档：\n\n' },
  { id: 'performance', name: '性能优化', prompt: '请分析以下代码的性能瓶颈并提供优化建议：\n\n' },
  { id: 'migration', name: '迁移指南', prompt: '请提供一个从以下旧实现迁移到新实现的方案：\n\n' },
  { id: 'shell_check', name: 'Shell 检查', prompt: '请检查以下 Shell 脚本中的错误、安全隐患和最佳实践问题：\n\n```sh\n' },
  { id: 'module_prop', name: 'module.prop 生成', prompt: '请为以下 Android 模块生成完整的 module.prop 文件：\n\n' },
];

export const ANALYSIS_MODES: AnalysisMode[] = [
  { id: 'explain', label: '代码解释', icon: 'lightbulb', prompt: '你是一个代码解释专家，能够详细解释代码的功能、逻辑和设计模式。' },
  { id: 'translate', label: '代码翻译', icon: 'translate', prompt: '你是一个代码翻译专家，擅长在不同编程语言间转换，保持功能一致。' },
  { id: 'optimize', label: '代码优化', icon: 'trending_up', prompt: '你是一个代码优化专家，能够分析代码并提供性能、可读性和可维护性的改进建议。' },
  { id: 'security', label: '安全审计', icon: 'security', prompt: '你是一个安全审计专家，能够识别代码中的安全漏洞并提供修复建议。' },
  { id: 'performance', label: '性能分析', icon: 'speed', prompt: '你是一个性能分析专家，能够识别代码中的性能瓶颈并提供优化建议。' },
];

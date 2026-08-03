// ─── Streaming, SSE parsing, agent step handling, progress tracking ───
// Extracted from +page.svelte to reduce main page size.

import type { Message, AgentStep, TokenUsage, AutoBuildPhase, ProgressStepDetail, Mode } from './types';
import { AUTO_BUILD_PHASE_DEFS } from './types';

// ─── SSE buffer parsing ───

export interface SSEParseResult {
  dataChunks: string[];
  leftover: string;
  done: boolean;
}

/** Parse SSE raw detail into individual data chunks. Returns leftover (incomplete line). */
export function parseSSEBuffer(buffer: string, detail: string): SSEParseResult {
  const raw = buffer + detail;
  const lines = raw.split('\n');
  const leftover = lines.pop() || '';
  const dataChunks: string[] = [];
  let done = false;
  for (const line of lines) {
    if (!line.startsWith('data: ')) continue;
    const data = line.slice(6);
    if (data === '[DONE]') { done = true; break; }
    dataChunks.push(data);
  }
  return { dataChunks, leftover, done };
}

// ─── Progress content analysis ───

const PROGRESS_KEYWORDS: [number, string[]][] = [
  [0, ['module.prop', 'analyze', '需求']],
  [1, ['customize.sh', 'install', '结构', 'script']],
  [2, ['#!/', 'set -e', '编写', '代码']],
  [3, ['optimize', 'performance', '优化']],
];

/** Analyze content text for progress step signals. Returns step index or -1 if none. */
export function analyzeProgressFromContent(content: string): number {
  const lower = content.toLowerCase();
  for (const [idx, keywords] of PROGRESS_KEYWORDS) {
    if (keywords.some(kw => lower.includes(kw))) return idx;
  }
  return -1;
}

/** Update progressStepDetails array with a new step entry. Returns new array. */
export function updateProgressDetails(
  details: ProgressStepDetail[],
  step: string,
  message: string
): ProgressStepDetail[] {
  const existingIdx = details.findIndex(d => d.step === step);
  if (existingIdx >= 0) {
    const next = [...details];
    next[existingIdx] = { ...next[existingIdx], message, time: Date.now() };
    return next;
  }
  return [...details, { step, message, time: Date.now() }];
}

// ─── Agent step helpers ───

const TOOL_LABELS: Record<string, string> = {
  read_file: '📖 读取文件',
  write_file: '✏️ 写入文件',
  lint_code: '🔍 检查代码',
  validate: '✅ 验证模块',
};

/** Build a display label for an agent skill call. */
export function buildToolLabel(skill: string, inputPath?: string): string {
  const label = TOOL_LABELS[skill] || `🔧 ${skill}`;
  const shortPath = inputPath ? inputPath.split('/').pop() : '';
  return `${label}${shortPath ? ' \`${shortPath}\`' : ''}...`;
}

/** Build a result preview string. */
export function buildResultPreview(content?: string): string {
  if (!content) return '';
  return content.length > 80 ? content.slice(0, 80) + '...' : content;
}

// ─── Progress step index mapping ───

const STEP_NAMES = ['start', 'structure', 'script', 'system', 'optimize', 'done'];
const AUTO_BUILD_STEP_MAP: Record<string, string> = {
  start: 'start', structure: 'structure', script: 'script',
  system: 'system', compile: 'compile', optimize: 'optimize',
};

/** Map a parsed step name to its numeric index in the progress bar. */
export function resolveStepIndex(step: string): number {
  return STEP_NAMES.indexOf(step);
}

/** Map an auto-build phase to a progress step name. */
export function mapAutoBuildPhaseToStep(phase: string): string | null {
  return AUTO_BUILD_STEP_MAP[phase] || null;
}

// ─── Stream batch manager ───

/**
 * Manages requestAnimationFrame-based batching for stream content updates.
 * Avoids per-token re-renders by accumulating deltas and flushing once per frame.
 */
export class StreamBatchManager {
  private pendingContent = '';
  private pendingReasoning = '';
  private rafId: number | null = null;
  private getMessages: () => Message[];
  private setMessages: (msgs: Message[]) => void;
  private getAssistantIdx: () => number;

  constructor(
    getMessages: () => Message[],
    setMessages: (msgs: Message[]) => void,
    getAssistantIdx: () => number,
  ) {
    this.getMessages = getMessages;
    this.setMessages = setMessages;
    this.getAssistantIdx = getAssistantIdx;
  }

  appendContent(text: string) { this.pendingContent += text; this.schedule(); }
  appendReasoning(text: string) { this.pendingReasoning += text; this.schedule(); }

  private schedule() {
    if (this.rafId === null) this.rafId = requestAnimationFrame(() => this.flush());
  }

  flush() {
    this.rafId = null;
    const msgs = this.getMessages();
    const idx = this.getAssistantIdx();
    if (idx < 0 || idx >= msgs.length) { this.pendingContent = ''; this.pendingReasoning = ''; return; }
    let changed = false;
    if (this.pendingContent) {
      const msg = msgs[idx];
      if (msg?.role === 'assistant') {
        msgs[idx] = { ...msg, content: msg.content + this.pendingContent };
        changed = true;
      }
      this.pendingContent = '';
    }
    if (this.pendingReasoning) {
      const msg = msgs[idx];
      if (msg?.role === 'assistant') {
        msgs[idx] = { ...msg, reasoning: (msg.reasoning || '') + this.pendingReasoning };
        changed = true;
      }
      this.pendingReasoning = '';
    }
    if (changed) this.setMessages([...msgs]);
    requestAnimationFrame(() => {
      const el = document.querySelector('.messages-area') as HTMLElement;
      if (el) el.scrollTop = el.scrollHeight;
    });
  }

  cancel() {
    if (this.rafId !== null) { cancelAnimationFrame(this.rafId); this.rafId = null; }
    this.flush();
  }

  get hasPending() { return this.rafId !== null; }
}

// ─── Progress update debounce manager ───

/**
 * Debounces progress content updates to avoid excessive state churn.
 * Accumulates content and flushes every `delayMs` milliseconds.
 */
export class ProgressUpdateManager {
  private timer: ReturnType<typeof setTimeout> | null = null;
  private pending = '';
  private callback: (content: string) => void;
  private delayMs: number;

  constructor(callback: (content: string) => void, delayMs = 50) {
    this.callback = callback;
    this.delayMs = delayMs;
  }

  append(content: string) {
    this.pending += content;
    if (this.timer) return;
    this.timer = setTimeout(() => {
      this.timer = null;
      this.callback(this.pending);
      this.pending = '';
    }, this.delayMs);
  }

  flush() {
    if (this.timer) { clearTimeout(this.timer); this.timer = null; }
    if (this.pending) { this.callback(this.pending); this.pending = ''; }
  }

  reset() {
    if (this.timer) { clearTimeout(this.timer); this.timer = null; }
    this.pending = '';
  }
}

// ─── Safety timer manager ───

/**
 * Manages stream safety timeouts (auto-disconnect on silence).
 */
export class SafetyTimerManager {
  private timer: ReturnType<typeof setTimeout> | null = null;

  /** Start or reset the safety timer. Calls `onTimeout` if no data arrives within `ms`. */
  start(ms: number, onTimeout: () => void) {
    this.stop();
    this.timer = setTimeout(() => { this.timer = null; onTimeout(); }, ms);
  }

  stop() {
    if (this.timer) { clearTimeout(this.timer); this.timer = null; }
  }
}

// ─── Agent step batch pusher ───

/**
 * Batches agent step pushes via requestAnimationFrame to avoid per-step re-renders.
 */
export class AgentStepBatcher {
  private pending: AgentStep[] = [];
  private rafId: number | null = null;
  private onFlush: (steps: AgentStep[]) => void;
  private target: AgentStep[];

  constructor(target: AgentStep[], onFlush: (steps: AgentStep[]) => void) {
    this.target = target;
    this.onFlush = onFlush;
  }

  push(step: AgentStep) {
    this.pending.push(step);
    this.target.push(step);
    if (this.rafId === null) {
      this.rafId = requestAnimationFrame(() => {
        this.rafId = null;
        if (this.pending.length > 0) {
          this.onFlush([...this.target]);
          this.pending = [];
        }
      });
    }
  }

  cancel() {
    if (this.rafId !== null) { cancelAnimationFrame(this.rafId); this.rafId = null; }
    if (this.pending.length > 0) { this.onFlush([...this.target]); this.pending = []; }
  }
}

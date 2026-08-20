/**
 * Prompt management state and operations.
 */
import type { Mode, AIPrompt } from '../types';
import {
  loadPrompts as loadPromptsFromBackend,
  savePromptToBackend,
  resetPromptToDefault,
} from '../prompts';

export interface PromptState {
  showPromptSettings: boolean;
  showMDPrompts: boolean;
  promptTab: Mode;
  prompts: AIPrompt[];
  promptDraft: string;
  promptSaving: boolean;
  promptLoading: boolean;
  showPromptTemplates: boolean;
}

export function createPromptState(): PromptState {
  return {
    showPromptSettings: false,
    showMDPrompts: false,
    promptTab: 'generate',
    prompts: [],
    promptDraft: '',
    promptSaving: false,
    promptLoading: false,
    showPromptTemplates: false,
  };
}

export async function loadPrompts(s: PromptState): Promise<AIPrompt[]> {
  const updated = await loadPromptsFromBackend();
  s.prompts = updated;
  return updated;
}

export async function switchPromptTab(s: PromptState, newMode: Mode): Promise<void> {
  s.promptTab = newMode;
  s.promptLoading = true;
  const updated = await loadPrompts(s);
  const p = updated.find(x => x.mode === newMode);
  s.promptDraft = p?.content || '';
  s.promptLoading = false;
}

export async function openPromptSettings(s: PromptState): Promise<void> {
  s.promptLoading = true;
  const updated = await loadPrompts(s);
  const p = updated.find(x => x.mode === s.promptTab);
  s.promptDraft = p?.content || '';
  s.showPromptSettings = true;
  s.promptLoading = false;
}

export async function savePrompt(s: PromptState): Promise<void> {
  s.promptSaving = true;
  s.promptLoading = true;
  const ok = await savePromptToBackend(s.promptTab, s.promptDraft);
  if (ok) await loadPrompts(s);
  s.promptLoading = false;
  s.promptSaving = false;
}

export async function resetPrompt(s: PromptState): Promise<void> {
  s.promptLoading = true;
  const content = await resetPromptToDefault(s.promptTab);
  s.promptDraft = content;
  s.promptLoading = false;
}

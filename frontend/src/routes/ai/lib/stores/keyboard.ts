/**
 * Keyboard shortcuts for the AI page.
 */
import type { Mode } from '../types';
import { MODES } from '../types';
import { generateUUID } from '../utils';
import { exportConversation } from './conversations';

const modes = MODES;

export interface KeyboardState {
  showShortcutPanel: boolean;
  showHistorySidebar: boolean;
  showPromptSettings: boolean;
  showMDPrompts: boolean;
  showProviderConfig: boolean;
  showPreviewModal: boolean;
  showImportDialog: boolean;
  showComparison: boolean;
  showPromptTemplates: boolean;
  showDiffPanel: boolean;
  showCapability: boolean;
  showMcpTools: boolean;
  showRepoReference: boolean;
}

export function handleKeydown(
  e: KeyboardEvent,
  ks: KeyboardState,
  callbacks: {
    messages: any[];
    streaming: boolean;
    mode: Mode;
    exportConversation: (fmt: 'json' | 'markdown') => void;
    newConversation: () => void;
    switchMode: (m: Mode) => void;
    setShowShortcutPanel: (v: boolean) => void;
    setShowHistorySidebar: (v: boolean) => void;
    setShowPromptSettings: (v: boolean) => void;
    setShowMDPrompts: (v: boolean) => void;
    setShowProviderConfig: (v: boolean) => void;
    setShowPreviewModal: (v: boolean) => void;
    setShowImportDialog: (v: boolean) => void;
    setShowComparison: (v: boolean) => void;
    setShowPromptTemplates: (v: boolean) => void;
    setShowDiffPanel: (v: boolean) => void;
    setShowCapability: (v: boolean) => void;
    setShowMcpTools: (v: boolean) => void;
    setShowRepoReference: (v: boolean) => void;
  },
): void {
  const tag = (e.target as HTMLElement)?.tagName;
  if (
    tag === 'INPUT' ||
    tag === 'TEXTAREA' ||
    (e.target as HTMLElement)?.contentEditable === 'true'
  )
    return;

  if (e.ctrlKey && e.key === 'k') {
    e.preventDefault();
    callbacks.newConversation();
  }
  if (e.ctrlKey && e.key === 'e') {
    e.preventDefault();
    if (callbacks.messages.length > 0) callbacks.exportConversation('markdown');
  }
  if (e.key === '?') {
    e.preventDefault();
    callbacks.setShowShortcutPanel(!ks.showShortcutPanel);
  }
  if (e.key === 'Escape') {
    callbacks.setShowHistorySidebar(false);
    callbacks.setShowPromptSettings(false);
    callbacks.setShowMDPrompts(false);
    callbacks.setShowProviderConfig(false);
    callbacks.setShowPreviewModal(false);
    callbacks.setShowImportDialog(false);
    callbacks.setShowComparison(false);
    callbacks.setShowPromptTemplates(false);
    callbacks.setShowDiffPanel(false);
    callbacks.setShowCapability(false);
    callbacks.setShowMcpTools(false);
    callbacks.setShowRepoReference(false);
  }
  if (
    !e.ctrlKey &&
    !e.metaKey &&
    !e.altKey &&
    ['1', '2', '3', '4', '5', '6'].includes(e.key)
  ) {
    const idx = parseInt(e.key) - 1;
    if (idx >= 0 && idx < modes.length && !callbacks.streaming && callbacks.mode !== modes[idx].value) {
      callbacks.switchMode(modes[idx].value);
    }
  }
}

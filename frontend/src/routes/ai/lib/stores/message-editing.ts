/**
 * Message editing, deletion, and reply state and operations.
 */
import type { Message } from '../types';
import {
  truncateForRegeneration,
  editMessageContent,
  deleteMessageAt,
} from '../messages';

export interface MessageEditState {
  editingMessageIdx: number;
  editingMessageText: string;
  deletingMessageIdx: number;
  showDeleteConfirm: boolean;
}

export function createMessageEditState(): MessageEditState {
  return {
    editingMessageIdx: -1,
    editingMessageText: '',
    deletingMessageIdx: -1,
    showDeleteConfirm: false,
  };
}

export function editMessage(s: MessageEditState, messages: Message[], idx: number): void {
  const msg = messages[idx];
  if (!msg) return;
  s.editingMessageIdx = idx;
  s.editingMessageText = msg.content;
}

export function saveEditMessage(s: MessageEditState, messages: Message[]): Message[] {
  if (s.editingMessageIdx < 0) return messages;
  const updated = editMessageContent(messages, s.editingMessageIdx, s.editingMessageText);
  s.editingMessageIdx = -1;
  s.editingMessageText = '';
  return updated;
}

export function cancelEditMessage(s: MessageEditState): void {
  s.editingMessageIdx = -1;
  s.editingMessageText = '';
}

export function confirmDeleteMessage(s: MessageEditState, idx: number): void {
  s.deletingMessageIdx = idx;
  s.showDeleteConfirm = true;
}

export function deleteMessage(s: MessageEditState, messages: Message[]): Message[] {
  if (s.deletingMessageIdx < 0) return messages;
  const updated = deleteMessageAt(messages, s.deletingMessageIdx);
  s.showDeleteConfirm = false;
  s.deletingMessageIdx = -1;
  return updated;
}

export function regenerateMessage(
  messages: Message[],
  streaming: boolean,
): { truncated: Message[]; userInput: string } | null {
  if (messages.length < 2 || streaming) return null;
  return truncateForRegeneration(messages);
}

export function replyToMessage(messages: Message[], idx: number): string {
  const msg = messages[idx];
  return msg ? msg.content : '';
}

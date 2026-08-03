// ─── Message editing operations ───
import type { Message } from './types';

/** Find the last assistant message index. */
export function findLastAssistantIdx(messages: Message[]): number {
  for (let j = messages.length - 1; j >= 0; j--) {
    if (messages[j].role === 'assistant') return j;
  }
  return -1;
}

/** Find the last user message index before the last assistant. */
export function findLastUserIdxBefore(messages: Message[], afterIdx: number): number {
  for (let j = afterIdx - 1; j >= 0; j--) {
    if (messages[j].role === 'user') return j;
  }
  return -1;
}

/** Truncate messages to just before the last user message (for regeneration). */
export function truncateForRegeneration(messages: Message[]): { truncated: Message[]; userInput: string } | null {
  const lastAsstIdx = findLastAssistantIdx(messages);
  if (lastAsstIdx < 0) return null;
  const lastUserIdx = findLastUserIdxBefore(messages, lastAsstIdx);
  if (lastUserIdx < 0) return null;
  return {
    truncated: messages.slice(0, lastUserIdx + 1),
    userInput: messages[lastUserIdx].content,
  };
}

/** Edit a message's content at a given index. */
export function editMessageContent(messages: Message[], idx: number, newContent: string): Message[] {
  if (idx < 0 || idx >= messages.length) return messages;
  const next = [...messages];
  next[idx] = { ...next[idx], content: newContent };
  return next;
}

/** Delete a message at a given index. */
export function deleteMessageAt(messages: Message[], idx: number): Message[] {
  return messages.filter((_, i) => i !== idx);
}

/** Get a message's content by index. */
export function getMessageContent(messages: Message[], idx: number): string {
  return idx >= 0 && idx < messages.length ? messages[idx].content : '';
}

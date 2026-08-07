export interface Shortcut {
  id: string;
  label: string;
  key: string;
  ctrlKey: boolean;
  shiftKey: boolean;
  metaKey: boolean;
  altKey?: boolean;
  category?: string;
}

const STORAGE_KEY = 'moduforge_shortcuts';

export const defaultShortcuts: Shortcut[] = [
  { id: 'save', label: '保存文件', key: 's', ctrlKey: true, shiftKey: false, metaKey: false },
  { id: 'search-file', label: '搜索文件', key: 'p', ctrlKey: true, shiftKey: false, metaKey: false },
  { id: 'toggle-terminal', label: '切换终端', key: '`', ctrlKey: true, shiftKey: false, metaKey: false },
  { id: 'undo', label: '撤销', key: 'z', ctrlKey: true, shiftKey: false, metaKey: false },
  { id: 'redo', label: '重做', key: 'z', ctrlKey: true, shiftKey: true, metaKey: false },
  { id: 'format-code', label: '格式化代码', key: 'F', ctrlKey: true, shiftKey: true, metaKey: false },
];

export function loadShortcuts(): Shortcut[] {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) return JSON.parse(stored);
  } catch {}
  return structuredClone(defaultShortcuts);
}

export function saveShortcuts(shortcuts: Shortcut[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(shortcuts));
}

export function resetShortcuts(): Shortcut[] {
  localStorage.removeItem(STORAGE_KEY);
  return structuredClone(defaultShortcuts);
}

export function matchShortcut(e: KeyboardEvent, sc: Shortcut): boolean {
  const isMac = navigator.platform.includes('Mac');
  const mod = isMac ? e.metaKey : e.ctrlKey;
  if (sc.ctrlKey && !sc.metaKey && !mod) return false;
  if (!sc.ctrlKey && !sc.metaKey && mod) return false;
  if (sc.metaKey && !e.metaKey) return false;
  if (sc.shiftKey !== e.shiftKey) return false;
  if (e.key.toLowerCase() !== sc.key.toLowerCase()) return false;
  return true;
}

export function shortcutLabel(sc: Shortcut): string {
  const parts: string[] = [];
  const isMac = navigator.platform.includes('Mac');
  if (sc.ctrlKey) parts.push(isMac ? '⌃' : 'Ctrl');
  if (sc.metaKey) parts.push(isMac ? '⌘' : 'Win');
  if (sc.shiftKey) parts.push('⇧');
  const keyMap: Record<string, string> = { '`': '~', 'F': 'Shift+F' };
  parts.push(keyMap[sc.key] || sc.key.toUpperCase());
  return parts.join('+');
}

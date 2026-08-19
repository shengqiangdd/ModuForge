import { describe, it, expect, beforeEach } from 'vitest';
import { loadShortcuts, saveShortcuts, resetShortcuts, matchShortcut, shortcutLabel, defaultShortcuts } from './shortcuts';

function makeEvent(partial: Partial<KeyboardEvent>): KeyboardEvent {
  return { key: '', ctrlKey: false, shiftKey: false, metaKey: false, altKey: false, ...partial } as KeyboardEvent;
}

describe('shortcuts', () => {
  const STORAGE_KEY = 'moduforge_shortcuts';

  beforeEach(() => {
    localStorage.clear();
    // 模拟非 Mac 平台，保证断言确定性
    Object.defineProperty(navigator, 'platform', { value: 'Linux', configurable: true });
  });

  it('默认快捷键包含 6 个内置项', () => {
    expect(defaultShortcuts.length).toBe(6);
    expect(defaultShortcuts.map(s => s.id)).toEqual([
      'save', 'search-file', 'toggle-terminal', 'undo', 'redo', 'format-code',
    ]);
  });

  it('无存储时 loadShortcuts 返回默认副本', () => {
    const loaded = loadShortcuts();
    expect(loaded).toEqual(defaultShortcuts);
    // 深拷贝，修改不影响默认值
    loaded[0].key = 'x';
    expect(defaultShortcuts[0].key).toBe('s');
  });

  it('损坏的 JSON 存储回退默认值', () => {
    localStorage.setItem(STORAGE_KEY, '{invalid json');
    expect(loadShortcuts()).toEqual(defaultShortcuts);
  });

  it('saveShortcuts 持久化后可加载', () => {
    const custom = [{ ...defaultShortcuts[0], key: 'k' }];
    saveShortcuts(custom);
    expect(loadShortcuts()).toEqual(custom);
  });

  it('resetShortcuts 清除存储并返回默认', () => {
    saveShortcuts([{ ...defaultShortcuts[0], key: 'k' }]);
    const reset = resetShortcuts();
    expect(reset).toEqual(defaultShortcuts);
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });

  it('matchShortcut: 非 Mac 用 ctrl 判定', () => {
    const sc = defaultShortcuts[0]; // save: Ctrl+S
    expect(matchShortcut(makeEvent({ key: 's', ctrlKey: true }), sc)).toBe(true);
    expect(matchShortcut(makeEvent({ key: 's', metaKey: true }), sc)).toBe(false);
    expect(matchShortcut(makeEvent({ key: 's' }), sc)).toBe(false);
  });

  it('matchShortcut: 大小写不敏感', () => {
    const sc = defaultShortcuts[0];
    expect(matchShortcut(makeEvent({ key: 'S', ctrlKey: true }), sc)).toBe(true);
  });

  it('matchShortcut: shift 组合必须完全匹配', () => {
    const redo = defaultShortcuts[4]; // redo: Ctrl+Shift+Z
    expect(matchShortcut(makeEvent({ key: 'z', ctrlKey: true, shiftKey: true }), redo)).toBe(true);
    expect(matchShortcut(makeEvent({ key: 'z', ctrlKey: true }), redo)).toBe(false);
  });

  it('matchShortcut: meta 快捷键需要 meta 键', () => {
    const sc: typeof defaultShortcuts[0] = { ...defaultShortcuts[0], metaKey: true, ctrlKey: false };
    expect(matchShortcut(makeEvent({ key: 's', metaKey: true }), sc)).toBe(true);
    expect(matchShortcut(makeEvent({ key: 's', ctrlKey: true }), sc)).toBe(false);
  });

  it('shortcutLabel: 非 Mac 组合标签', () => {
    const sc = defaultShortcuts[0]; // Ctrl+S
    expect(shortcutLabel(sc)).toBe('Ctrl+S');
    const redo = defaultShortcuts[4]; // Ctrl+Shift+Z
    expect(shortcutLabel(redo)).toBe('Ctrl+⇧+Z');
  });
});

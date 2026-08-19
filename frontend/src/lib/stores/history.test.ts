import { describe, it, expect, beforeEach } from 'vitest';
import { historyStore, type HistoryAction } from './history';

describe('historyStore', () => {
  beforeEach(() => {
    historyStore.clear();
  });

  it('初始状态不可撤销/重做', () => {
    expect(historyStore.canUndo()).toBe(false);
    expect(historyStore.canRedo()).toBe(false);
    expect(historyStore.getUndoCount()).toBe(0);
    expect(historyStore.getRedoCount()).toBe(0);
    expect(historyStore.getHistory()).toEqual([]);
  });

  it('push 后可撤销、不可重做', () => {
    historyStore.push({ type: 'edit', data: { a: 1 } });
    expect(historyStore.canUndo()).toBe(true);
    expect(historyStore.canRedo()).toBe(false);
    expect(historyStore.getUndoCount()).toBe(1);
    expect(historyStore.getRedoCount()).toBe(0);
  });

  it('undo 返回最近动作并推进索引', () => {
    historyStore.push({ type: 'a', data: 1 });
    historyStore.push({ type: 'b', data: 2 });
    const action = historyStore.undo();
    expect(action?.type).toBe('b');
    expect(historyStore.getUndoCount()).toBe(1);
    expect(historyStore.getRedoCount()).toBe(1);
  });

  it('redo 返回被撤销的动作', () => {
    historyStore.push({ type: 'a', data: 1 });
    historyStore.push({ type: 'b', data: 2 });
    historyStore.undo();
    const action = historyStore.redo();
    expect(action?.type).toBe('b');
    expect(historyStore.getUndoCount()).toBe(2);
    expect(historyStore.getRedoCount()).toBe(0);
  });

  it('空栈 undo/redo 返回 null', () => {
    expect(historyStore.undo()).toBeNull();
    expect(historyStore.redo()).toBeNull();
  });

  it('push 会截断 redo 分支', () => {
    historyStore.push({ type: 'a', data: 1 });
    historyStore.push({ type: 'b', data: 2 });
    historyStore.undo();
    historyStore.push({ type: 'c', data: 3 });
    expect(historyStore.canRedo()).toBe(false);
    expect(historyStore.getHistory().map(a => a.type)).toEqual(['a', 'c']);
  });

  it('超过 MAX_STEPS(50) 时丢弃最旧动作', () => {
    for (let i = 0; i < 60; i++) {
      historyStore.push({ type: `t${i}`, data: i });
    }
    expect(historyStore.getHistory().length).toBe(50);
    expect(historyStore.getHistory()[0].type).toBe('t10');
    expect(historyStore.getHistory()[49].type).toBe('t59');
    expect(historyStore.getUndoCount()).toBe(50);
  });

  it('push 自动带时间戳', () => {
    const before = Date.now();
    historyStore.push({ type: 'x', data: null });
    const action = historyStore.getHistory()[0];
    expect(action.timestamp).toBeGreaterThanOrEqual(before);
    expect(action.timestamp).toBeLessThanOrEqual(Date.now());
  });

  it('clear 重置一切', () => {
    historyStore.push({ type: 'a', data: 1 });
    historyStore.clear();
    expect(historyStore.getHistory()).toEqual([]);
    expect(historyStore.canUndo()).toBe(false);
    expect(historyStore.canRedo()).toBe(false);
  });

  it('HistoryAction 类型: undo 返回完整动作对象', () => {
    historyStore.push({ type: 'edit', data: { file: 'a.ts' } });
    const action = historyStore.undo() as HistoryAction<{ file: string }>;
    expect(action?.data.file).toBe('a.ts');
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { getToasts, toast, dismiss, subscribe } from './toast.svelte';

describe('toast store', () => {
  beforeEach(() => {
    // Dismiss all toasts between tests
    const toasts = getToasts();
    for (const t of toasts) {
      dismiss(t.id);
    }
  });

  it('starts with empty toasts', () => {
    expect(getToasts()).toHaveLength(0);
  });

  it('adds a toast', () => {
    toast('hello', 'info', 0);
    expect(getToasts()).toHaveLength(1);
    expect(getToasts()[0].message).toBe('hello');
    expect(getToasts()[0].type).toBe('info');
  });

  it('dismisses a toast by id', () => {
    toast('msg', 'info', 0);
    const id = getToasts()[0].id;
    dismiss(id);
    expect(getToasts()).toHaveLength(0);
  });

  it('enforces max 5 toasts', () => {
    for (let i = 0; i < 10; i++) {
      toast(`msg ${i}`, 'info', 0);
    }
    expect(getToasts().length).toBeLessThanOrEqual(5);
  });

  it('notifies subscribers on add', () => {
    const fn = vi.fn();
    const unsub = subscribe(fn);
    toast('test', 'info', 0);
    expect(fn).toHaveBeenCalledTimes(1);
    unsub();
  });

  it('notifies subscribers on dismiss', () => {
    const fn = vi.fn();
    toast('test', 'info', 0);
    const id = getToasts()[0].id;
    subscribe(fn);
    dismiss(id);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it('auto-dismisses after duration', async () => {
    vi.useFakeTimers();
    toast('auto', 'info', 100);
    expect(getToasts()).toHaveLength(1);
    vi.advanceTimersByTime(100);
    expect(getToasts()).toHaveLength(0);
    vi.useRealTimers();
  });
});
import { describe, it, expect, vi } from 'vitest';
import { debounce, throttle, runOnce, sleep, batchUpdates, createDebouncedValue } from './performance';

describe('debounce', () => {
  it('delays execution', async () => {
    vi.useFakeTimers();
    const fn = vi.fn();
    const debounced = debounce(fn, 100);

    debounced();
    expect(fn).not.toHaveBeenCalled();

    vi.advanceTimersByTime(100);
    expect(fn).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });

  it('only calls once for rapid invocations', async () => {
    vi.useFakeTimers();
    const fn = vi.fn();
    const debounced = debounce(fn, 100);

    debounced();
    debounced();
    debounced();

    vi.advanceTimersByTime(100);
    expect(fn).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });
});

describe('throttle', () => {
  it('calls immediately on first invocation', () => {
    vi.useFakeTimers();
    const fn = vi.fn();
    const throttled = throttle(fn, 100);

    throttled();
    expect(fn).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });

  it('limits calls within the window', () => {
    vi.useFakeTimers();
    const fn = vi.fn();
    const throttled = throttle(fn, 100);

    throttled(); // immediate
    throttled(); // throttled
    expect(fn).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(100);
    expect(fn).toHaveBeenCalledTimes(2);

    vi.useRealTimers();
  });
});

describe('runOnce', () => {
  it('only calls the function once', () => {
    const fn = vi.fn(() => 42);
    const once = runOnce(fn);

    expect(once()).toBe(42);
    expect(once()).toBe(42);
    expect(fn).toHaveBeenCalledTimes(1);
  });
});

describe('sleep', () => {
  it('resolves after the specified time', async () => {
    vi.useFakeTimers();
    const promise = sleep(100);

    vi.advanceTimersByTime(100);
    await expect(promise).resolves.toBeUndefined();

    vi.useRealTimers();
  });
});

describe('batchUpdates', () => {
  it('batches and returns the result', async () => {
    vi.useFakeTimers();
    const fn = vi.fn(() => 'result');
    const promise = batchUpdates(fn, 50);

    vi.advanceTimersByTime(50);
    await expect(promise).resolves.toBe('result');
    expect(fn).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });
});

describe('createDebouncedValue', () => {
  it('updates value after debounce delay', async () => {
    vi.useFakeTimers();
    let value = 1;
    const debounced = createDebouncedValue(() => value, 100);

    expect(debounced.get()).toBe(1);

    value = 2;
    debounced.update();
    // Value should not update immediately
    expect(debounced.get()).toBe(1);

    vi.advanceTimersByTime(100);
    expect(debounced.get()).toBe(2);

    vi.useRealTimers();
  });
});
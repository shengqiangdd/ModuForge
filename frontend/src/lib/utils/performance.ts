/**
 * Debounce function - delays execution until after wait milliseconds
 * have elapsed since the last time the function was called.
 */
export function debounce<T extends (...args: any[]) => any>(
  fn: T,
  wait: number = 150
): (...args: Parameters<T>) => void {
  let timeoutId: ReturnType<typeof setTimeout> | null = null;

  return function (this: any, ...args: Parameters<T>) {
    if (timeoutId !== null) {
      clearTimeout(timeoutId);
    }
    timeoutId = setTimeout(() => {
      fn.apply(this, args);
      timeoutId = null;
    }, wait);
  };
}

/**
 * Throttle function - ensures fn is called at most once every wait milliseconds.
 */
export function throttle<T extends (...args: any[]) => any>(
  fn: T,
  wait: number = 16
): (...args: Parameters<T>) => void {
  let lastTime = 0;
  let timeoutId: ReturnType<typeof setTimeout> | null = null;

  return function (this: any, ...args: Parameters<T>) {
    const now = Date.now();
    const remaining = wait - (now - lastTime);

    if (remaining <= 0) {
      if (timeoutId !== null) {
        clearTimeout(timeoutId);
        timeoutId = null;
      }
      lastTime = now;
      fn.apply(this, args);
    } else if (timeoutId === null) {
      timeoutId = setTimeout(() => {
        lastTime = Date.now();
        timeoutId = null;
        fn.apply(this, args);
      }, remaining);
    }
  };
}

/**
 * Create a debounced store value for Svelte 5.
 * Returns a derived value that updates after the debounce delay.
 */
export function createDebouncedValue<T>(
  getValue: () => T,
  wait: number = 150
): { get: () => T; update: () => void } {
  let currentValue = getValue();
  let timeoutId: ReturnType<typeof setTimeout> | null = null;

  return {
    get: () => currentValue,
    update: () => {
      if (timeoutId !== null) {
        clearTimeout(timeoutId);
      }
      timeoutId = setTimeout(() => {
        currentValue = getValue();
        timeoutId = null;
      }, wait);
    }
  };
}

/**
 * Run once - ensures a function is only called once.
 */
export function runOnce<T extends (...args: any[]) => any>(fn: T): T {
  let called = false;
  let result: any;
  return ((...args: any[]) => {
    if (!called) {
      called = true;
      result = fn(...args);
    }
    return result;
  }) as T;
}

/**
 * Sleep utility for async operations.
 */
export function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * Batch multiple rapid updates into a single update.
 */
export function batchUpdates<T>(
  fn: () => T,
  wait: number = 0
): Promise<T> {
  return new Promise(resolve => {
    setTimeout(() => {
      resolve(fn());
    }, wait);
  });
}

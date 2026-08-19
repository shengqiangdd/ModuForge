// 测试环境全局 setup
//
// vitest 4.x + jsdom 30 下，全局 localStorage 被定义成"空壳对象"（只有
// Object.prototype 方法，缺 setItem/getItem/clear 等 Storage API），导致
// theme/toast 等依赖 localStorage 的测试抛 "localStorage.clear is not a
// function"。这里防御式补齐一个完整的内存版 Storage mock。
const storageStore = new Map<string, string>();

const storageMock: Storage = {
  get length() {
    return storageStore.size;
  },
  clear() {
    storageStore.clear();
  },
  getItem(key: string) {
    return storageStore.has(key) ? storageStore.get(key)! : null;
  },
  key(index: number) {
    return Array.from(storageStore.keys())[index] ?? null;
  },
  removeItem(key: string) {
    storageStore.delete(key);
  },
  setItem(key: string, value: string) {
    storageStore.set(key, String(value));
  },
};

const existingLSAccessor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');

const needsStoragePatch =
  typeof globalThis.localStorage === 'undefined' ||
  (typeof globalThis.localStorage === 'object' &&
    typeof (globalThis.localStorage as Partial<Storage>).setItem !== 'function');

if (needsStoragePatch) {
  Object.defineProperty(globalThis, 'localStorage', {
    value: storageMock,
    configurable: true,
    writable: true,
  });
  // jsdom 有时只在 window 上而非 globalThis 暴露，统一铺到两者
  try {
    Object.defineProperty(window, 'localStorage', {
      value: storageMock,
      configurable: true,
      writable: true,
    });
  } catch {
    /* 非 jsdom/(window 只读) 时忽略 */
  }
} else if (existingLSAccessor) {
  // jsdom 已提供完整 localStorage，保持原样
}

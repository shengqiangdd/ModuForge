import { describe, it, expect, beforeEach, vi } from 'vitest';
import { locales, currentLocale, t, type Locale } from './index';

// i18n 模块在 import 时就会读取 localStorage / navigator.language，
// 所以测试环境必须先 mock 再动态 import。
const storage = new Map<string, string>();
vi.stubGlobal('localStorage', {
  getItem: (k: string) => storage.get(k) ?? null,
  setItem: (k: string, v: string) => { storage.set(k, v); },
  removeItem: (k: string) => { storage.delete(k); },
  clear: () => storage.clear(),
  key: () => null,
  length: 0,
});

describe('i18n', () => {
  beforeEach(() => {
    storage.clear();
    // 默认英文浏览器
    Object.defineProperty(navigator, 'language', { value: 'en-US', configurable: true });
    vi.resetModules();
  });

  it('支持 4 种语言', () => {
    expect(locales.map(l => l.code)).toEqual(['en', 'zh', 'ja', 'ko']);
  });

  it('浏览器 zh 语言默认中文', async () => {
    Object.defineProperty(navigator, 'language', { value: 'zh-CN', configurable: true });
    const mod = await import('./index');
    let v: Locale = 'en';
    mod.currentLocale.subscribe(x => { v = x; })();
    expect(v).toBe('zh');
  });

  it('浏览器 ja 语言默认日语', async () => {
    Object.defineProperty(navigator, 'language', { value: 'ja-JP', configurable: true });
    const mod = await import('./index');
    let v: Locale = 'en';
    mod.currentLocale.subscribe(x => { v = x; })();
    expect(v).toBe('ja');
  });

  it('本地存储优先于浏览器语言', async () => {
    storage.set('moduforge_locale', 'ko');
    const mod = await import('./index');
    let v: Locale = 'en';
    mod.currentLocale.subscribe(x => { v = x; })();
    expect(v).toBe('ko');
  });

  it('切换语言会持久化到 localStorage', async () => {
    const mod = await import('./index');
    mod.currentLocale.set('ja');
    expect(storage.get('moduforge_locale')).toBe('ja');
  });

  it('t 函数在 zh 下返回中文翻译', async () => {
    const mod = await import('./index');
    mod.currentLocale.set('zh');
    let tr: (k: string) => string = (k) => k;
    mod.t.subscribe(fn => { tr = fn as never; })();
    // 用真实存在的 key 验证
    const sampleKey = Object.keys((await import('./zh')).zh)[0];
    const enVal = (await import('./en')).en[sampleKey];
    expect(tr(sampleKey)).not.toBe(enVal);
  });

  it('t 函数翻译缺失 key 回退英文再回退原文', async () => {
    const mod = await import('./index');
    mod.currentLocale.set('zh');
    let tr: (k: string) => string = (k) => k;
    mod.t.subscribe(fn => { tr = fn as never; })();
    expect(tr('__missing_key__')).toBe('__missing_key__');
  });

  it('t 函数支持参数插值', async () => {
    const mod = await import('./index');
    mod.currentLocale.set('en');
    let tr: (k: string, p?: Record<string, string | number>) => string = (k) => k;
    mod.t.subscribe(fn => { tr = fn as never; })();
    // 找一个含 { 的 en key
    const enDict = (await import('./en')).en;
    const paramKey = Object.entries(enDict).find(([, v]) => v.includes('{'))?.[0];
    if (paramKey) {
      const raw = enDict[paramKey];
      const filled = tr(paramKey, { count: 3 });
      expect(filled).toContain('3');
      expect(filled).not.toBe(raw);
    }
  });
});

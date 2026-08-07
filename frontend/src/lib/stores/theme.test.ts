import { describe, it, expect, beforeEach, vi } from 'vitest';
import { getTheme, setTheme, initTheme } from './theme';

describe('theme store', () => {
  beforeEach(() => {
    // 清除 localStorage 和 DOM 操作 mock
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('starts with dark theme', () => {
    expect(getTheme()).toBe('dark');
  });

  it('sets theme explicitly', () => {
    setTheme('light');
    expect(getTheme()).toBe('light');
    setTheme('dark');
    expect(getTheme()).toBe('dark');
  });

  it('persists theme to localStorage', () => {
    setTheme('light');
    expect(localStorage.getItem('moduforge_theme_mode')).toBe('light');
    setTheme('dark');
    expect(localStorage.getItem('moduforge_theme_mode')).toBe('dark');
  });

  it('applies theme to document data attribute', () => {
    setTheme('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
    setTheme('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('initTheme loads saved theme from localStorage', () => {
    // mock window.matchMedia for jsdom
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
    localStorage.setItem('moduforge_theme_mode', 'light');
    initTheme();
    expect(getTheme()).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });
});
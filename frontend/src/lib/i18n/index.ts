import { writable, derived } from 'svelte/store';
import { en } from './en';
import { zh } from './zh';
import { ja } from './ja';
import { ko } from './ko';

export type Locale = 'en' | 'zh' | 'ja' | 'ko';

export const locales: { code: Locale; name: string; flag: string }[] = [
  { code: 'en', name: 'English', flag: '\u{1F1FA}\u{1F1F8}' },
  { code: 'zh', name: '中文', flag: '\u{1F1E8}\u{1F1F3}' },
  { code: 'ja', name: '日本語', flag: '\u{1F1EF}\u{1F1F5}' },
  { code: 'ko', name: '한국어', flag: '\u{1F1F0}\u{1F1F7}' },
];

const translations: Record<Locale, Record<string, string>> = { en, zh, ja, ko };

function getBrowserLocale(): Locale {
  const nav = navigator.language;
  if (nav.startsWith('zh')) return 'zh';
  if (nav.startsWith('ja')) return 'ja';
  if (nav.startsWith('ko')) return 'ko';
  return 'en';
}

const stored = typeof localStorage !== 'undefined' ? (localStorage.getItem('moduforge_locale') as Locale) : null;
export const currentLocale = writable<Locale>(stored || getBrowserLocale());

currentLocale.subscribe((val) => {
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem('moduforge_locale', val);
  }
});

export const t = derived(currentLocale, ($locale) => {
  return (key: string, params?: Record<string, string | number>): string => {
    let str = translations[$locale]?.[key] || translations.en[key] || key;
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        str = str.replace(`{${k}}`, String(v));
      });
    }
    return str;
  };
});

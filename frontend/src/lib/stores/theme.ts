type ThemeMode = 'light' | 'dark' | 'system';

let _theme: ThemeMode = 'dark';

function applyTheme(mode: ThemeMode) {
  let resolved: 'light' | 'dark';
  if (mode === 'system') {
    resolved = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  } else {
    resolved = mode;
  }
  document.documentElement.setAttribute('data-theme', resolved);
}

export function getTheme(): ThemeMode {
  return _theme;
}

export function setTheme(mode: ThemeMode) {
  _theme = mode;
  localStorage.setItem('moduforge_theme_mode', mode);
  applyTheme(mode);
}

export function initTheme() {
  const saved = localStorage.getItem('moduforge_theme_mode') as ThemeMode | null;
  if (saved) {
    _theme = saved;
  }
  applyTheme(_theme);

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (_theme === 'system') {
      applyTheme('system');
    }
  });
}

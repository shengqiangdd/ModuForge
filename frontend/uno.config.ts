import { defineConfig, presetUno, presetWebFonts } from 'unocss';

export default defineConfig({
  presets: [
    presetUno(),
    presetWebFonts({
      fonts: {
        sans: 'Inter:300,400,500,600,700',
        mono: 'JetBrains Mono:400,500',
      },
    }),
  ],
  theme: {
    colors: {
      primary: {
        50: '#f0f4ff',
        100: '#dbe4ff',
        200: '#bac8ff',
        300: '#91a7ff',
        400: '#748ffc',
        500: '#5c7cfa',
        600: '#4c6ef5',
        700: '#4263eb',
        800: '#3b5bdb',
        900: '#364fc7',
      },
      neutral: {
        50: '#fafafa',
        100: '#f5f5f5',
        200: '#e5e5e5',
        300: '#d4d4d4',
        400: '#a3a3a3',
        500: '#737373',
        600: '#525252',
        700: '#404040',
        800: '#262626',
        900: '#171717',
        950: '#0a0a0a',
      },
      success: '#22c55e',
      warning: '#f59e0b',
      error: '#ef4444',
      info: '#06b6d4',
    },
    borderRadius: {
      xl: '0.75rem',
      '2xl': '1rem',
      '3xl': '1.5rem',
    },
    boxShadow: {
      'glow': '0 0 20px rgba(139, 92, 246, 0.25)',
      'glow-lg': '0 0 40px rgba(139, 92, 246, 0.35)',
      'elevated': '0 4px 24px rgba(0, 0, 0, 0.3)',
      'elevated-lg': '0 8px 40px rgba(0, 0, 0, 0.4)',
      'card': '0 1px 2px rgba(0,0,0,0.3)',
      'card-hover': '0 0 20px rgba(139,92,246,0.25)',
    },
  },
  shortcuts: {
    // Remove all hardcoded shortcuts — app.css defines btn-primary, btn-ghost, input-field, badge
  },
});

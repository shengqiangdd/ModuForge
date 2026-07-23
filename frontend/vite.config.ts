import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import UnoCSS from 'unocss/vite';
import path from 'path';

export default defineConfig({
  plugins: [UnoCSS(), svelte()],
  resolve: {
    alias: {
      '$lib': path.resolve('./src/lib'),
      '$app': path.resolve('./src/app'),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5174,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
  build: {
    // 优化构建性能 - 使用 rolldown 内置压缩（Vite 8 默认）
    target: 'esnext',
    cssCodeSplit: true,
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          // CodeMirror 单独分块（体积最大）
          if (id.includes('codemirror') || id.includes('@codemirror')) {
            if (id.includes('/lang-')) return 'codemirror-lang';
            return 'codemirror';
          }
          // Svelte 核心单独分块
          if (id.includes('svelte') || id.includes('@sveltejs/')) return 'svelte';
          // i18n 单独分块（频繁改动）
          if (id.includes('/i18n/')) return 'i18n';
        },
      },
    },
  },
});

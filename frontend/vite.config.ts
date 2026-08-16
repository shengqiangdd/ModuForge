/// <reference types="vitest" />
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
    // CodeMirror/Svelte 是大而稳定的库，已按库拆分为独立 chunk；
    // 阈值上调到 700kB 避免对合理分块产生噪音 warning
    chunkSizeWarningLimit: 700,
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
  test: {
    // 使用 jsdom 作为 DOM 环境（Svelte 5 组件测试需要）
    environment: 'jsdom',
    // 全局 setup 文件
    setupFiles: ['./src/test-setup.ts'],
    // 包含 Svelte 组件测试
    include: ['src/**/*.{test,spec}.{ts,js,svelte}'],
    // 排除 E2E 测试
    exclude: ['e2e/**', 'node_modules/**'],
    // 为 Svelte 组件启用 CSS 处理
    css: false,
  },
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import CacheMonitor from './CacheMonitor.svelte';

describe('CacheMonitor', () => {
  beforeEach(() => {
    vi.mock('$app/environment', () => ({
      browser: true,
      dev: false,
      building: false,
      version: '0.0.0'
    }));
  });

  it('renders cache monitor title', () => {
    render(CacheMonitor);
    expect(screen.getByText('🗄️ 缓存监控')).toBeTruthy();
  });

  it('shows refresh button', () => {
    render(CacheMonitor);
    expect(screen.getByText('刷新')).toBeTruthy();
  });

  it('shows clear cache button', () => {
    render(CacheMonitor);
    expect(screen.getByText('清除缓存')).toBeTruthy();
  });
});

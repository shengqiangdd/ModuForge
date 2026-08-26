import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import MonitorPanel from './MonitorPanel.svelte';

describe('MonitorPanel', () => {
  beforeEach(() => {
    vi.mock('$app/environment', () => ({
      browser: true,
      dev: false,
      building: false,
      version: '0.0.0'
    }));
  });

  it('renders monitor panel title', () => {
    render(MonitorPanel);
    expect(screen.getByText('📡 监控告警')).toBeTruthy();
  });

  it('shows alerts tab', () => {
    render(MonitorPanel);
    expect(screen.getByText(/告警/)).toBeTruthy();
  });

  it('shows logs tab', () => {
    render(MonitorPanel);
    expect(screen.getByText('日志统计')).toBeTruthy();
  });
});

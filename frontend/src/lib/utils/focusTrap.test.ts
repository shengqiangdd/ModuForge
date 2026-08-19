import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { focusTrap } from './focusTrap';

function createModal(html: string): { node: HTMLElement; buttons: HTMLElement[]; outside: HTMLElement } {
  document.body.innerHTML = `<button id="outside">outside</button>` + html;
  const outside = document.getElementById('outside') as HTMLElement;
  const node = document.body.querySelector('div') as HTMLElement;
  const buttons = Array.from(node.querySelectorAll('button'));
  return { node, buttons, outside };
}

describe('focusTrap', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  afterEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
  });

  it('挂载后聚焦第一个可聚焦元素', () => {
    const { node } = createModal('<div><button>1</button><button>2</button></div>');
    const rAF = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      cb(0);
      return 0;
    });
    focusTrap(node);
    expect(document.activeElement?.textContent).toBe('1');
    rAF.mockRestore();
  });

  it('Tab 在最后一个元素时循环到第一个', () => {
    const { node, buttons } = createModal('<div><button>1</button><button>2</button></div>');
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 0);
    const trap = focusTrap(node);
    buttons[1].focus();
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true });
    const prevent = vi.spyOn(event, 'preventDefault');
    node.dispatchEvent(event);
    expect(prevent).toHaveBeenCalled();
    expect(document.activeElement?.textContent).toBe('1');
    trap.destroy();
  });

  it('Shift+Tab 在第一个元素时循环到最后一个', () => {
    const { node, buttons } = createModal('<div><button>1</button><button>2</button></div>');
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 0);
    const trap = focusTrap(node);
    buttons[0].focus();
    const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true });
    const prevent = vi.spyOn(event, 'preventDefault');
    node.dispatchEvent(event);
    expect(prevent).toHaveBeenCalled();
    expect(document.activeElement?.textContent).toBe('2');
    trap.destroy();
  });

  it('无可聚焦元素时 Tab 不报错', () => {
    const { node } = createModal('<div><p>text only</p></div>');
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 0);
    const trap = focusTrap(node);
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true });
    expect(() => node.dispatchEvent(event)).not.toThrow();
    trap.destroy();
  });

  it('非 Tab 键不拦截', () => {
    const { node } = createModal('<div><button>1</button><button>2</button></div>');
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 0);
    const trap = focusTrap(node);
    const event = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true });
    const prevent = vi.spyOn(event, 'preventDefault');
    node.dispatchEvent(event);
    expect(prevent).not.toHaveBeenCalled();
    trap.destroy();
  });

  it('destroy 恢复之前聚焦元素并解绑监听', () => {
    const { node, outside } = createModal('<div><button>1</button></div>');
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 0);
    outside.focus();
    const trap = focusTrap(node);
    trap.destroy();
    expect(document.activeElement).toBe(outside);
    // 销毁后再派发 Tab 不应触发 preventDefault
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true });
    const prevent = vi.spyOn(event, 'preventDefault');
    node.dispatchEvent(event);
    expect(prevent).not.toHaveBeenCalled();
  });

  it('跳过 disabled 元素', () => {
    const { node } = createModal('<div><button disabled>disabled</button><button>active</button></div>');
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      cb(0);
      return 0;
    });
    const trap = focusTrap(node);
    expect(document.activeElement?.textContent).toBe('active');
    trap.destroy();
  });
});

export type ToastType = 'success' | 'error' | 'info' | 'warning';

export interface Toast {
  id: string;
  type: ToastType;
  message: string;
  duration: number;
}

const MAX_TOASTS = 5;

let _toasts: Toast[] = [];
let _listeners: Array<() => void> = [];
const _timers: Map<string, ReturnType<typeof setTimeout>> = new Map();

export function getToasts(): Toast[] {
  return _toasts;
}

export function subscribe(fn: () => void): () => void {
  // Prevent duplicate subscriptions from the same callback reference
  if (!_listeners.includes(fn)) {
    _listeners.push(fn);
  }
  return () => { _listeners = _listeners.filter(l => l !== fn); };
}

function notify() {
  for (let i = 0; i < _listeners.length; i++) _listeners[i]();
}

export function toast(message: string, type: ToastType = 'info', duration = 3000) {
  // Deduplicate: skip if the most recent toast has the same message and type
  const last = _toasts[_toasts.length - 1];
  if (last && last.message === message && last.type === type) return;

  const id = Math.random().toString(36).slice(2);
  // Enforce max toast limit — drop oldest
  if (_toasts.length >= MAX_TOASTS) {
    const dropped = _toasts.slice(0, _toasts.length - MAX_TOASTS + 1);
    dropped.forEach(t => { const tm = _timers.get(t.id); if (tm) { clearTimeout(tm); _timers.delete(t.id); } });
    _toasts = _toasts.slice(_toasts.length - MAX_TOASTS + 1);
  }
  _toasts = [..._toasts, { id, type, message, duration }];
  notify();
  if (duration > 0) {
    const timer = setTimeout(() => dismiss(id), duration);
    _timers.set(id, timer);
  }
}

export function dismiss(id: string) {
  const idx = _toasts.findIndex(t => t.id === id);
  if (idx === -1) return;
  const tm = _timers.get(id);
  if (tm) { clearTimeout(tm); _timers.delete(id); }
  _toasts = [..._toasts.slice(0, idx), ..._toasts.slice(idx + 1)];
  notify();
}

/** Dismiss all toasts — call on component unmount for cleanup */
export function dismissAll() {
  _timers.forEach((tm) => clearTimeout(tm));
  _timers.clear();
  if (_toasts.length > 0) {
    _toasts = [];
    notify();
  }
}

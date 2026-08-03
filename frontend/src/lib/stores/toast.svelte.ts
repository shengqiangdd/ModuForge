type ToastType = 'success' | 'error' | 'info' | 'warning';

interface Toast {
  id: string;
  type: ToastType;
  message: string;
  duration: number;
}

const MAX_TOASTS = 5;

let _toasts: Toast[] = [];
let _listeners: Array<() => void> = [];

export function getToasts(): Toast[] {
  return _toasts;
}

export function subscribe(fn: () => void): () => void {
  _listeners.push(fn);
  return () => { _listeners = _listeners.filter(l => l !== fn); };
}

function notify() {
  for (let i = 0; i < _listeners.length; i++) _listeners[i]();
}

export function toast(message: string, type: ToastType = 'info', duration = 3000) {
  const id = Math.random().toString(36).slice(2);
  // Enforce max toast limit — drop oldest
  if (_toasts.length >= MAX_TOASTS) {
    _toasts = _toasts.slice(_toasts.length - MAX_TOASTS + 1);
  }
  _toasts = [..._toasts, { id, type, message, duration }];
  notify();
  if (duration > 0) {
    setTimeout(() => dismiss(id), duration);
  }
}

export function dismiss(id: string) {
  const idx = _toasts.findIndex(t => t.id === id);
  if (idx === -1) return;
  // Splice avoids full array reconstruction
  _toasts = [..._toasts.slice(0, idx), ..._toasts.slice(idx + 1)];
  notify();
}

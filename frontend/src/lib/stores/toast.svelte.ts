type ToastType = 'success' | 'error' | 'info' | 'warning';

interface Toast {
  id: string;
  type: ToastType;
  message: string;
  duration: number;
}

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
  _listeners.forEach(fn => fn());
}

export function toast(message: string, type: ToastType = 'info', duration = 3000) {
  const id = Math.random().toString(36).slice(2);
  _toasts = [..._toasts, { id, type, message, duration }];
  notify();
  if (duration > 0) {
    setTimeout(() => dismiss(id), duration);
  }
}

export function dismiss(id: string) {
  _toasts = _toasts.filter(t => t.id !== id);
  notify();
}

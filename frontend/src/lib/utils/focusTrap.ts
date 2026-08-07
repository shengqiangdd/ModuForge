const FOCUSABLE_SELECTOR = 'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function focusTrap(node: HTMLElement) {
  const previouslyFocused = document.activeElement as HTMLElement | null;

  const focusableElements = () => node.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR);
  const first = () => focusableElements()[0];
  const last = () => focusableElements()[focusableElements().length - 1];

  function handleKeydown(e: KeyboardEvent) {
    if (e.key !== 'Tab') return;
    const els = focusableElements();
    if (els.length === 0) return;

    const current = document.activeElement;
    const idx = Array.from(els).indexOf(current as HTMLElement);

    if (e.shiftKey) {
      if (idx <= 0) {
        e.preventDefault();
        last().focus();
      }
    } else {
      if (idx === els.length - 1 || idx === -1) {
        e.preventDefault();
        first().focus();
      }
    }
  }

  node.addEventListener('keydown', handleKeydown);

  // Focus the first focusable element when the modal opens
  requestAnimationFrame(() => {
    first()?.focus();
  });

  return {
    destroy() {
      node.removeEventListener('keydown', handleKeydown);
      previouslyFocused?.focus();
    }
  };
}
/**
 * Generate ARIA label from element context
 */
export function generateAriaLabel(element: string, context: string): string {
	const labels: Record<string, string> = {
		button: `${context} button`,
		input: `${context} input field`,
		link: `${context} link`,
		icon: `${context} icon`
	};
	return labels[element] || context;
}

/**
 * Focus trap manager
 */
export function manageFocus(container: HTMLElement): { destroy: () => void } {
	const focusableSelectors = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Tab') return;

		const focusable = container.querySelectorAll(focusableSelectors);
		const first = focusable[0] as HTMLElement;
		const last = focusable[focusable.length - 1] as HTMLElement;

		if (event.shiftKey) {
			if (document.activeElement === first) {
				event.preventDefault();
				last.focus();
			}
		} else {
			if (document.activeElement === last) {
				event.preventDefault();
				first.focus();
			}
		}
	}

	container.addEventListener('keydown', handleKeydown);

	return {
		destroy() {
			container.removeEventListener('keydown', handleKeydown);
		}
	};
}

/**
 * Announce message to screen readers
 */
export function announceToScreenReader(message: string): void {
	const el = document.createElement('div');
	el.setAttribute('role', 'status');
	el.setAttribute('aria-live', 'polite');
	el.setAttribute('aria-atomic', 'true');
	el.style.cssText = 'position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0)';
	el.textContent = message;

	document.body.appendChild(el);
	setTimeout(() => el.remove(), 1000);
}

/**
 * Default keyboard shortcuts
 */
export function getKeyboardShortcuts(): Record<string, string> {
	return {
		'ctrl+enter': 'generate',
		'ctrl+s': 'save',
		'ctrl+/': 'toggle-help',
		'escape': 'close'
	};
}

/**
 * Handle keyboard navigation
 */
export function handleKeyboardNavigation(
	event: KeyboardEvent,
	actions: Record<string, () => void>
): void {
	const key = [];
	if (event.ctrlKey || event.metaKey) key.push('ctrl');
	if (event.shiftKey) key.push('shift');
	if (event.altKey) key.push('alt');
	key.push(event.key.toLowerCase());

	const combo = key.join('+');
	if (actions[combo]) {
		event.preventDefault();
		actions[combo]();
	}
}

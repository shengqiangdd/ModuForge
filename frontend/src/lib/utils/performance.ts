/**
 * Throttle function execution
 */
export function throttle<T extends (...args: unknown[]) => unknown>(
	fn: T,
	delay: number
): (...args: Parameters<T>) => void {
	let lastCall = 0;
	let timeoutId: ReturnType<typeof setTimeout> | null = null;

	return (...args: Parameters<T>) => {
		const now = Date.now();
		const remaining = delay - (now - lastCall);

		if (remaining <= 0) {
			if (timeoutId) {
				clearTimeout(timeoutId);
				timeoutId = null;
			}
			lastCall = now;
			fn(...args);
		} else if (!timeoutId) {
			timeoutId = setTimeout(() => {
				lastCall = Date.now();
				timeoutId = null;
				fn(...args);
			}, remaining);
		}
	};
}

/**
 * Debounce function execution
 */
export function debounce<T extends (...args: unknown[]) => unknown>(
	fn: T,
	delay: number
): (...args: Parameters<T>) => void {
	let timeoutId: ReturnType<typeof setTimeout> | null = null;

	return (...args: Parameters<T>) => {
		if (timeoutId) {
			clearTimeout(timeoutId);
		}
		timeoutId = setTimeout(() => {
			timeoutId = null;
			fn(...args);
		}, delay);
	};
}

export interface VirtualListResult {
	startIndex: number;
	endIndex: number;
	visibleItems: unknown[];
	offsetY: number;
	totalHeight: number;
}

/**
 * Calculate virtual list visible range
 */
export function virtualList<T>(
	items: T[],
	containerHeight: number,
	itemHeight: number,
	scrollTop: number = 0
): VirtualListResult {
	const totalHeight = items.length * itemHeight;
	const startIndex = Math.floor(scrollTop / itemHeight);
	const visibleCount = Math.ceil(containerHeight / itemHeight);
	const endIndex = Math.min(startIndex + visibleCount + 1, items.length);

	return {
		startIndex: Math.max(0, startIndex),
		endIndex,
		visibleItems: items.slice(Math.max(0, startIndex), endIndex),
		offsetY: Math.max(0, startIndex * itemHeight),
		totalHeight
	};
}

/**
 * Lazy load with condition
 */
export async function lazyLoad<T>(
	condition: boolean,
	loader: () => Promise<T>
): Promise<T | null> {
	if (!condition) return null;
	return loader();
}

/**
 * Batch updates into chunks
 */
export function batchUpdate<T>(
	items: T[],
	batchSize: number,
	callback: (batch: T[]) => void
): void {
	for (let i = 0; i < items.length; i += batchSize) {
		callback(items.slice(i, i + batchSize));
	}
}

/**
 * Measure execution time
 */
export function measureTime<T>(label: string, fn: () => T): T {
	console.time(label);
	const result = fn();
	console.timeEnd(label);
	return result;
}

/**
 * Request idle callback polyfill
 */
export function requestIdleCallbackPolyfill(
	callback: (deadline: { timeRemaining: () => number }) => void
): void {
	if (typeof requestIdleCallback !== 'undefined') {
		requestIdleCallback(callback);
	} else {
		setTimeout(() => callback({ timeRemaining: () => 0 }), 1);
	}
}

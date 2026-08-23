import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { throttle, debounce, virtualList, batchUpdate } from './performance';

describe('performance', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	describe('throttle', () => {
		it('should throttle function calls', () => {
			const fn = vi.fn();
			const throttled = throttle(fn, 100);

			throttled();
			throttled();
			throttled();

			expect(fn).toHaveBeenCalledTimes(1);

			vi.advanceTimersByTime(100);
			throttled();

			expect(fn).toHaveBeenCalledTimes(2);
		});
	});

	describe('debounce', () => {
		it('should debounce function calls', () => {
			const fn = vi.fn();
			const debounced = debounce(fn, 100);

			debounced();
			debounced();
			debounced();

			expect(fn).not.toHaveBeenCalled();

			vi.advanceTimersByTime(100);
			expect(fn).toHaveBeenCalledTimes(1);
		});
	});

	describe('virtualList', () => {
		it('should calculate visible range', () => {
			const items = Array.from({ length: 100 }, (_, i) => i);
			const result = virtualList(items, 300, 30, 0);

			expect(result.startIndex).toBe(0);
			expect(result.endIndex).toBeGreaterThan(0);
			expect(result.visibleItems.length).toBeGreaterThan(0);
			expect(result.totalHeight).toBe(3000);
		});

		it('should handle scroll position', () => {
			const items = Array.from({ length: 100 }, (_, i) => i);
			const result = virtualList(items, 300, 30, 150);

			expect(result.startIndex).toBe(5);
			expect(result.offsetY).toBe(150);
		});

		it('should handle empty items', () => {
			const result = virtualList([], 300, 30, 0);
			expect(result.visibleItems.length).toBe(0);
			expect(result.totalHeight).toBe(0);
		});
	});

	describe('batchUpdate', () => {
		it('should batch items', () => {
			const items = [1, 2, 3, 4, 5];
			const batches: number[][] = [];

			batchUpdate(items, 2, (batch) => batches.push(batch));

			expect(batches.length).toBe(3);
			expect(batches[0]).toEqual([1, 2]);
			expect(batches[1]).toEqual([3, 4]);
			expect(batches[2]).toEqual([5]);
		});

		it('should handle empty items', () => {
			const batches: number[][] = [];
			batchUpdate([], 2, (batch) => batches.push(batch));
			expect(batches.length).toBe(0);
		});
	});
});

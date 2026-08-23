import { describe, it, expect, vi, beforeEach } from 'vitest';
import { isOnline, isStandalone, onOnlineChange } from './pwa';

describe('pwa', () => {
	describe('isOnline', () => {
		it('should return boolean', () => {
			expect(typeof isOnline()).toBe('boolean');
		});
	});

	describe('isStandalone', () => {
		it('should return boolean', () => {
			expect(typeof isStandalone()).toBe('boolean');
		});
	});

	describe('onOnlineChange', () => {
		it('should return unsubscribe function', () => {
			const unsub = onOnlineChange(() => {});
			expect(typeof unsub).toBe('function');
			unsub();
		});
	});
});

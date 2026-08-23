import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Breakpoint, getViewportWidth, getViewportHeight, getViewportSize, isMobile, isTablet, isDesktop } from './responsive';

describe('responsive', () => {
	it('should have breakpoint enum', () => {
		expect(Breakpoint.MOBILE).toBe(375);
		expect(Breakpoint.TABLET).toBe(768);
		expect(Breakpoint.DESKTOP).toBe(1024);
		expect(Breakpoint.LARGE).toBe(1440);
	});

	describe('getViewportWidth', () => {
		it('should return viewport width', () => {
			const width = getViewportWidth();
			expect(typeof width).toBe('number');
			expect(width).toBeGreaterThan(0);
		});
	});

	describe('getViewportHeight', () => {
		it('should return viewport height', () => {
			const height = getViewportHeight();
			expect(typeof height).toBe('number');
			expect(height).toBeGreaterThan(0);
		});
	});

	describe('getViewportSize', () => {
		it('should return size object', () => {
			const size = getViewportSize();
			expect(size.width).toBeDefined();
			expect(size.height).toBeDefined();
		});
	});

	describe('isMobile', () => {
		it('should return boolean', () => {
			const result = isMobile();
			expect(typeof result).toBe('boolean');
		});
	});

	describe('isTablet', () => {
		it('should return boolean', () => {
			const result = isTablet();
			expect(typeof result).toBe('boolean');
		});
	});

	describe('isDesktop', () => {
		it('should return boolean', () => {
			const result = isDesktop();
			expect(typeof result).toBe('boolean');
		});
	});

	it('viewport checks should be mutually exclusive', () => {
		const m = isMobile();
		const t = isTablet();
		const d = isDesktop();

		// Only one should be true
		const trueCount = [m, t, d].filter(Boolean).length;
		expect(trueCount).toBe(1);
	});
});

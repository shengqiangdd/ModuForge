import { describe, it, expect } from 'vitest';
import { generateAriaLabel, getKeyboardShortcuts, handleKeyboardNavigation } from './accessibility';

describe('accessibility', () => {
	describe('generateAriaLabel', () => {
		it('should generate button label', () => {
			const label = generateAriaLabel('button', 'Submit');
			expect(label).toBe('Submit button');
		});

		it('should generate input label', () => {
			const label = generateAriaLabel('input', 'Email');
			expect(label).toBe('Email input field');
		});

		it('should handle unknown element', () => {
			const label = generateAriaLabel('custom', 'Action');
			expect(label).toBe('Action');
		});
	});

	describe('getKeyboardShortcuts', () => {
		it('should return shortcuts', () => {
			const shortcuts = getKeyboardShortcuts();
			expect(shortcuts['ctrl+enter']).toBe('generate');
			expect(shortcuts['escape']).toBe('close');
		});
	});

	describe('handleKeyboardNavigation', () => {
		it('should trigger action for matching combo', () => {
			const actions = { 'ctrl+enter': vi.fn() };
			const event = new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true });

			handleKeyboardNavigation(event, actions);

			expect(actions['ctrl+enter']).toHaveBeenCalled();
		});

		it('should not trigger for non-matching combo', () => {
			const actions = { 'ctrl+s': vi.fn() };
			const event = new KeyboardEvent('keydown', { key: 'a', ctrlKey: true });

			handleKeyboardNavigation(event, actions);

			expect(actions['ctrl+s']).not.toHaveBeenCalled();
		});
	});
});

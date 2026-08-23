import { describe, it, expect } from 'vitest';
import { snippets, searchSnippets, getSnippetById, getSnippetsByLanguage } from './snippets';

describe('snippets', () => {
	it('should have snippets defined', () => {
		expect(snippets).toBeDefined();
		expect(snippets.length).toBeGreaterThan(0);
	});

	it('should have all required fields', () => {
		for (const snippet of snippets) {
			expect(snippet.id).toBeDefined();
			expect(snippet.name).toBeDefined();
			expect(snippet.language).toBeDefined();
			expect(snippet.trigger).toBeDefined();
			expect(snippet.code).toBeDefined();
		}
	});

	it('should search snippets by query', () => {
		const results = searchSnippets('daemon');
		expect(results.length).toBeGreaterThan(0);
		expect(results.some((s) => s.name.toLowerCase().includes('daemon'))).toBe(true);
	});

	it('should search snippets by language', () => {
		const results = searchSnippets('', 'go');
		expect(results.length).toBeGreaterThan(0);
		for (const s of results) {
			expect(s.language).toBe('go');
		}
	});

	it('should search by trigger', () => {
		const results = searchSnippets('module-prop');
		expect(results.length).toBe(1);
		expect(results[0].trigger).toBe('module-prop');
	});

	it('should return all snippets for empty query', () => {
		const results = searchSnippets('');
		expect(results.length).toBe(snippets.length);
	});

	it('should get snippet by ID', () => {
		const snippet = getSnippetById('shell/module-prop');
		expect(snippet).toBeDefined();
		expect(snippet?.name).toBe('module.prop');
	});

	it('should return undefined for unknown ID', () => {
		const snippet = getSnippetById('nonexistent');
		expect(snippet).toBeUndefined();
	});

	it('should get snippets by language', () => {
		const results = getSnippetsByLanguage('shell');
		expect(results.length).toBeGreaterThan(0);
		for (const s of results) {
			expect(s.language).toBe('shell');
		}
	});

	it('should have shell snippets', () => {
		const shellSnippets = snippets.filter((s) => s.language === 'shell');
		expect(shellSnippets.length).toBeGreaterThanOrEqual(4);
	});

	it('should have go snippets', () => {
		const goSnippets = snippets.filter((s) => s.language === 'go');
		expect(goSnippets.length).toBeGreaterThanOrEqual(2);
	});
});

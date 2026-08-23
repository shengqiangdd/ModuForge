import { describe, it, expect } from 'vitest';
import { searchInFile, replaceInFile } from './search';

describe('searchInFile', () => {
	it('should find matches', () => {
		const content = 'hello world\nhello again';
		const result = searchInFile(content, { query: 'hello' });
		expect(result.totalCount).toBe(2);
		expect(result.matches.length).toBe(2);
	});

	it('should return line and column', () => {
		const content = 'hello world';
		const result = searchInFile(content, { query: 'world' });
		expect(result.matches[0].line).toBe(1);
		expect(result.matches[0].column).toBe(7);
	});

	it('should handle case insensitive search', () => {
		const content = 'Hello HELLO hello';
		const result = searchInFile(content, { query: 'hello', caseSensitive: false });
		expect(result.totalCount).toBe(3);
	});

	it('should handle case sensitive search', () => {
		const content = 'Hello HELLO hello';
		const result = searchInFile(content, { query: 'hello', caseSensitive: true });
		expect(result.totalCount).toBe(1);
	});

	it('should handle whole word search', () => {
		const content = 'hello world hello-world';
		const result = searchInFile(content, { query: 'hello', wholeWord: true });
		expect(result.totalCount).toBe(1);
	});

	it('should handle regex search', () => {
		const content = 'abc 123 def 456';
		const result = searchInFile(content, { query: '\\d+', useRegex: true });
		expect(result.totalCount).toBe(2);
	});

	it('should return empty for no matches', () => {
		const content = 'hello world';
		const result = searchInFile(content, { query: 'xyz' });
		expect(result.totalCount).toBe(0);
		expect(result.matches.length).toBe(0);
	});

	it('should return empty for empty query', () => {
		const content = 'hello world';
		const result = searchInFile(content, { query: '' });
		expect(result.totalCount).toBe(0);
	});

	it('should handle invalid regex', () => {
		const content = 'hello';
		const result = searchInFile(content, { query: '[invalid', useRegex: true });
		expect(result.totalCount).toBe(0);
	});
});

describe('replaceInFile', () => {
	it('should replace all matches', () => {
		const content = 'hello world hello';
		const result = replaceInFile(content, { query: 'hello', replaceWith: 'hi' });
		expect(result.content).toBe('hi world hi');
		expect(result.replaceCount).toBe(2);
	});

	it('should replace with case insensitive', () => {
		const content = 'Hello HELLO hello';
		const result = replaceInFile(content, { query: 'hello', replaceWith: 'hi', caseSensitive: false });
		expect(result.content).toBe('hi hi hi');
		expect(result.replaceCount).toBe(3);
	});

	it('should replace with regex', () => {
		const content = 'abc 123 def 456';
		const result = replaceInFile(content, { query: '\\d+', replaceWith: 'NUM', useRegex: true });
		expect(result.content).toBe('abc NUM def NUM');
		expect(result.replaceCount).toBe(2);
	});

	it('should replace with whole word', () => {
		const content = 'hello hello-world';
		const result = replaceInFile(content, { query: 'hello', replaceWith: 'hi', wholeWord: true });
		expect(result.content).toBe('hi hello-world');
		expect(result.replaceCount).toBe(1);
	});

	it('should return original for no matches', () => {
		const content = 'hello world';
		const result = replaceInFile(content, { query: 'xyz', replaceWith: 'abc' });
		expect(result.content).toBe(content);
		expect(result.replaceCount).toBe(0);
	});

	it('should return original for empty query', () => {
		const content = 'hello world';
		const result = replaceInFile(content, { query: '', replaceWith: 'abc' });
		expect(result.content).toBe(content);
		expect(result.replaceCount).toBe(0);
	});

	it('should handle invalid regex', () => {
		const content = 'hello';
		const result = replaceInFile(content, { query: '[invalid', replaceWith: 'x', useRegex: true });
		expect(result.content).toBe(content);
		expect(result.replaceCount).toBe(0);
	});
});

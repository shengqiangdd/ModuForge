import { describe, it, expect } from 'vitest';
import { parseAnsiLog, parseStructuredLog, filterByLevel, searchInLogs, generateSummary } from './logParser';

describe('logParser', () => {
	describe('parseAnsiLog', () => {
		it('should parse ANSI color codes', () => {
			const input = '\x1b[31mError\x1b[0m';
			const result = parseAnsiLog(input);
			expect(result).toContain('<span style="color:#f44336">');
			expect(result).toContain('</span>');
		});

		it('should handle no ANSI codes', () => {
			const input = 'plain text';
			const result = parseAnsiLog(input);
			expect(result).toBe('plain text');
		});
	});

	describe('parseStructuredLog', () => {
		it('should parse log lines', () => {
			const input = 'INFO: Starting build\nERROR: Build failed';
			const result = parseStructuredLog(input);
			expect(result.length).toBe(2);
			expect(result[0].level).toBe('info');
			expect(result[1].level).toBe('error');
		});

		it('should extract timestamps', () => {
			const input = '12:30:45 Building module';
			const result = parseStructuredLog(input);
			expect(result[0].timestamp).toBe('12:30:45');
		});

		it('should extract source', () => {
			const input = '[builder] Compiling files';
			const result = parseStructuredLog(input);
			expect(result[0].source).toBe('builder');
		});

		it('should assign line numbers', () => {
			const input = 'line1\nline2\nline3';
			const result = parseStructuredLog(input);
			expect(result[0].line).toBe(1);
			expect(result[1].line).toBe(2);
			expect(result[2].line).toBe(3);
		});

		it('should skip empty lines', () => {
			const input = 'line1\n\n\nline2';
			const result = parseStructuredLog(input);
			expect(result.length).toBe(2);
		});
	});

	describe('filterByLevel', () => {
		it('should filter by level', () => {
			const lines = parseStructuredLog('INFO: test\nERROR: fail\nWARN: warning');
			const errors = filterByLevel(lines, 'error');
			expect(errors.length).toBe(1);
			expect(errors[0].level).toBe('error');
		});
	});

	describe('searchInLogs', () => {
		it('should search by query', () => {
			const lines = parseStructuredLog('Building module\nCompiling files\nBuild complete');
			const results = searchInLogs(lines, 'build');
			expect(results.length).toBe(2);
		});

		it('should return all for empty query', () => {
			const lines = parseStructuredLog('line1\nline2');
			const results = searchInLogs(lines, '');
			expect(results.length).toBe(2);
		});
	});

	describe('generateSummary', () => {
		it('should generate summary', () => {
			const lines = parseStructuredLog('INFO: ok\nERROR: fail\nWARN: warning\nSUCCESS: done');
			const summary = generateSummary(lines);
			expect(summary.total).toBe(4);
			expect(summary.errors).toBe(1);
			expect(summary.warnings).toBe(1);
			expect(summary.success).toBe(1);
		});

		it('should handle empty lines', () => {
			const summary = generateSummary([]);
			expect(summary.total).toBe(0);
			expect(summary.errors).toBe(0);
		});
	});
});

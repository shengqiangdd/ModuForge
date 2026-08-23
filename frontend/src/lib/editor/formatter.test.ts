import { describe, it, expect } from 'vitest';
import { formatCode, formatShell, formatGo, formatJSON, autoFormat } from './formatter';

describe('formatter', () => {
	it('should remove trailing whitespace', () => {
		const input = 'hello   \nworld   ';
		const result = formatCode(input, 'text');
		expect(result).toBe('hello\nworld');
	});

	it('should normalize tabs to spaces', () => {
		const input = '\tconst x = 1;';
		const result = formatCode(input, 'text');
		expect(result).toBe('    const x = 1;');
	});

	it('should collapse multiple blank lines', () => {
		const input = 'line1\n\n\n\nline2';
		const result = formatCode(input, 'text');
		expect(result).toBe('line1\n\nline2');
	});

	it('should remove leading blank lines', () => {
		const input = '\n\n\nhello';
		const result = formatCode(input, 'text');
		expect(result).toBe('hello');
	});

	it('should remove trailing blank lines', () => {
		const input = 'hello\n\n\n';
		const result = formatCode(input, 'text');
		expect(result).toBe('hello');
	});

	it('should ensure final newline', () => {
		const input = 'hello';
		const result = formatCode(input, 'text');
		expect(result.endsWith('\n')).toBe(true);
	});

	it('should handle empty input', () => {
		const result = formatCode('', 'text');
		expect(result).toBe('');
	});

	it('should format shell script', () => {
		const input = '#!/system/bin/sh\necho "hello"';
		const result = formatShell(input);
		expect(result).toContain('#!/system/bin/sh');
	});
});

describe('formatGo', () => {
	it('should handle basic Go code', () => {
		const input = 'package main\n\nfunc main() {\nfmt.Println("hello")\n}';
		const result = formatGo(input);
		expect(result).toContain('package main');
	});

	it('should handle empty input', () => {
		const result = formatGo('');
		expect(result).toBe('');
	});
});

describe('formatJSON', () => {
	it('should format valid JSON', () => {
		const input = '{"a":1,"b":2}';
		const result = formatJSON(input);
		expect(result).toContain('"a": 1');
		expect(result).toContain('"b": 2');
	});

	it('should return original for invalid JSON', () => {
		const input = '{invalid json}';
		const result = formatJSON(input);
		expect(result).toBe(input);
	});
});

describe('autoFormat', () => {
	it('should auto-format shell', () => {
		const input = 'echo "hello"';
		const result = autoFormat(input, 'shell');
		expect(typeof result).toBe('string');
	});

	it('should auto-format go', () => {
		const input = 'package main';
		const result = autoFormat(input, 'go');
		expect(typeof result).toBe('string');
	});

	it('should auto-format json', () => {
		const input = '{"key":"value"}';
		const result = autoFormat(input, 'json');
		expect(result).toContain('"key": "value"');
	});

	it('should fallback to basic format', () => {
		const input = 'hello';
		const result = autoFormat(input, 'unknown');
		expect(result).toBe('hello\n');
	});
});

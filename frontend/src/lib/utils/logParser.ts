export type LogLevel = 'debug' | 'info' | 'warn' | 'error' | 'success';

export interface ParsedLine {
	line: number;
	level: LogLevel;
	message: string;
	timestamp?: string;
	source?: string;
}

const LEVEL_PATTERNS: [RegExp, LogLevel][] = [
	[/\berror\b/i, 'error'],
	[/\bfail(?:ed)?\b/i, 'error'],
	[/\bwarn(?:ing)?\b/i, 'warn'],
	[/\bsuccess\b/i, 'success'],
	[/\bok\b/i, 'success'],
	[/\bdone\b/i, 'success'],
	[/\bdebug\b/i, 'debug'],
	[/\binfo\b/i, 'info']
];

/**
 * Parse ANSI escape codes to HTML spans
 */
export function parseAnsiLog(raw: string): string {
	const ansiMap: [RegExp, string][] = [
		[/\x1b\[31m/g, '<span style="color:#f44336">'],
		[/\x1b\[32m/g, '<span style="color:#4caf50">'],
		[/\x1b\[33m/g, '<span style="color:#ff9800">'],
		[/\x1b\[34m/g, '<span style="color:#2196f3">'],
		[/\x1b\[35m/g, '<span style="color:#9c27b0">'],
		[/\x1b\[36m/g, '<span style="color:#00bcd4">'],
		[/\x1b\[0m/g, '</span>'],
		[/\x1b\[\d+m/g, '']
	];

	let result = raw;
	for (const [pattern, replacement] of ansiMap) {
		result = result.replace(pattern, replacement);
	}

	return result;
}

/**
 * Parse structured log lines
 */
export function parseStructuredLog(raw: string): ParsedLine[] {
	const lines = raw.split('\n');
	return lines
		.filter((l) => l.trim().length > 0)
		.map((line, i) => parseLogLine(line, i + 1));
}

function parseLogLine(line: string, lineNum: number): ParsedLine {
	let level: LogLevel = 'info';
	for (const [pattern, lv] of LEVEL_PATTERNS) {
		if (pattern.test(line)) {
			level = lv;
			break;
		}
	}

	// Try to extract timestamp (HH:MM:SS or ISO)
	let timestamp: string | undefined;
	const tsMatch = line.match(/(\d{2}:\d{2}:\d{2})/);
	if (tsMatch) {
		timestamp = tsMatch[1];
	}

	// Try to extract source [source]
	let source: string | undefined;
	const srcMatch = line.match(/\[([^\]]+)\]/);
	if (srcMatch) {
		source = srcMatch[1];
	}

	return {
		line: lineNum,
		level,
		message: line,
		timestamp,
		source
	};
}

/**
 * Filter lines by level
 */
export function filterByLevel(lines: ParsedLine[], level: LogLevel): ParsedLine[] {
	return lines.filter((l) => l.level === level);
}

/**
 * Search in log lines
 */
export function searchInLogs(lines: ParsedLine[], query: string): ParsedLine[] {
	if (!query) return lines;
	const lower = query.toLowerCase();
	return lines.filter((l) => l.message.toLowerCase().includes(lower));
}

/**
 * Generate log summary
 */
export function generateSummary(lines: ParsedLine[]): {
	total: number;
	errors: number;
	warnings: number;
	success: number;
} {
	let errors = 0;
	let warnings = 0;
	let success = 0;

	for (const line of lines) {
		switch (line.level) {
			case 'error':
				errors++;
				break;
			case 'warn':
				warnings++;
				break;
			case 'success':
				success++;
				break;
		}
	}

	return {
		total: lines.length,
		errors,
		warnings,
		success
	};
}

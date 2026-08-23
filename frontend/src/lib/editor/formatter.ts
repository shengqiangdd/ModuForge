/**
 * Format code with basic rules:
 * - Normalize indentation (tabs → spaces)
 * - Remove trailing whitespace
 * - Collapse multiple blank lines
 * - Ensure final newline
 */
export function formatCode(code: string, language: string): string {
	let lines = code.split('\n');

	// Process each line
	lines = lines.map((line) => {
		// Remove trailing whitespace
		line = line.replace(/\s+$/, '');

		// Normalize tabs to 4 spaces
		line = line.replace(/\t/g, '    ');

		return line;
	});

	// Collapse multiple blank lines into one
	const collapsed: string[] = [];
	let prevBlank = false;

	for (const line of lines) {
		const isBlank = line.trim() === '';

		if (isBlank && prevBlank) {
			continue;
		}

		collapsed.push(line);
		prevBlank = isBlank;
	}

	// Remove leading blank lines
	while (collapsed.length > 0 && collapsed[0].trim() === '') {
		collapsed.shift();
	}

	// Remove trailing blank lines
	while (collapsed.length > 0 && collapsed[collapsed.length - 1].trim() === '') {
		collapsed.pop();
	}

	let result = collapsed.join('\n');

	// Ensure final newline
	if (result.length > 0 && !result.endsWith('\n')) {
		result += '\n';
	}

	return result;
}

/**
 * Format Shell script
 */
export function formatShell(code: string): string {
	const lines = code.split('\n');
	const formatted: string[] = [];

	for (let i = 0; i < lines.length; i++) {
		let line = lines[i].replace(/\s+$/, '');

		// Ensure shebang is at top
		if (i === 0 && !line.startsWith('#!') && line.trim() !== '') {
			formatted.unshift('#!/system/bin/sh');
			formatted.push('');
		}

		formatted.push(line);
	}

	return formatCode(formatted.join('\n'), 'shell');
}

/**
 * Format Go code (basic)
 */
export function formatGo(code: string): string {
	const lines = code.split('\n');
	const formatted: string[] = [];
	let indent = 0;

	for (const line of lines) {
		let trimmed = line.replace(/\s+$/, '');

		// Decrease indent for closing braces
		if (trimmed.startsWith('}') || trimmed.startsWith(')') || trimmed.startsWith(']')) {
			indent = Math.max(0, indent - 1);
		}

		// Apply indentation
		if (trimmed.length > 0) {
			trimmed = '    '.repeat(indent) + trimmed;
		}

		formatted.push(trimmed);

		// Increase indent for opening braces
		if (trimmed.endsWith('{') || trimmed.endsWith('(') || trimmed.endsWith('[')) {
			indent++;
		}
	}

	return formatCode(formatted.join('\n'), 'go');
}

/**
 * Format JSON (basic pretty-print)
 */
export function formatJSON(code: string): string {
	try {
		const parsed = JSON.parse(code);
		return JSON.stringify(parsed, null, 2) + '\n';
	} catch {
		// If parsing fails, return original
		return code;
	}
}

/**
 * Auto-detect and format code
 */
export function autoFormat(code: string, language: string): string {
	switch (language) {
		case 'shell':
			return formatShell(code);
		case 'go':
			return formatGo(code);
		case 'json':
			return formatJSON(code);
		default:
			return formatCode(code, language);
	}
}

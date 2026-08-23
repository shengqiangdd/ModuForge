export interface SearchOptions {
	query: string;
	caseSensitive?: boolean;
	wholeWord?: boolean;
	useRegex?: boolean;
	replaceWith?: string;
}

export interface SearchMatch {
	line: number;
	column: number;
	length: number;
	text: string;
}

export interface SearchResult {
	matches: SearchMatch[];
	totalCount: number;
}

export interface ReplaceResult {
	content: string;
	replaceCount: number;
}

/**
 * Search for matches in file content
 */
export function searchInFile(content: string, options: SearchOptions): SearchResult {
	const { query, caseSensitive = false, wholeWord = false, useRegex = false } = options;

	if (!query) {
		return { matches: [], totalCount: 0 };
	}

	const lines = content.split('\n');
	const matches: SearchMatch[] = [];

	let regex: RegExp;
	try {
		if (useRegex) {
			const flags = caseSensitive ? 'g' : 'gi';
			regex = new RegExp(query, flags);
		} else {
			let pattern = escapeRegExp(query);
			if (wholeWord) {
				pattern = `\\b${pattern}\\b`;
			}
			const flags = caseSensitive ? 'g' : 'gi';
			regex = new RegExp(pattern, flags);
		}
	} catch {
		return { matches: [], totalCount: 0 };
	}

	for (let lineNum = 0; lineNum < lines.length; lineNum++) {
		const line = lines[lineNum];
		let match: RegExpExecArray | null;

		regex.lastIndex = 0;
		while ((match = regex.exec(line)) !== null) {
			matches.push({
				line: lineNum + 1,
				column: match.index + 1,
				length: match[0].length,
				text: match[0]
			});

			// Prevent infinite loop on zero-length matches
			if (match[0].length === 0) {
				regex.lastIndex++;
			}
		}
	}

	return {
		matches,
		totalCount: matches.length
	};
}

/**
 * Replace all matches in file content
 */
export function replaceInFile(content: string, options: SearchOptions): ReplaceResult {
	const { query, replaceWith = '', caseSensitive = false, wholeWord = false, useRegex = false } = options;

	if (!query) {
		return { content, replaceCount: 0 };
	}

	let regex: RegExp;
	try {
		if (useRegex) {
			const flags = caseSensitive ? 'g' : 'gi';
			regex = new RegExp(query, flags);
		} else {
			let pattern = escapeRegExp(query);
			if (wholeWord) {
				pattern = `\\b${pattern}\\b`;
			}
			const flags = caseSensitive ? 'g' : 'gi';
			regex = new RegExp(pattern, flags);
		}
	} catch {
		return { content, replaceCount: 0 };
	}

	const before = content;
	const newContent = content.replace(regex, replaceWith);
	const replaceCount = (before.match(regex) || []).length;

	return {
		content: newContent,
		replaceCount
	};
}

/**
 * Escape special regex characters
 */
function escapeRegExp(str: string): string {
	return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

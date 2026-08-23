export interface LanguageConfig {
	id: string;
	name: string;
	extension: string;
	mimeType: string;
	cmLanguage: string;
}

export const languages: LanguageConfig[] = [
	{
		id: 'shell',
		name: 'Shell',
		extension: '.sh',
		mimeType: 'text/x-shellscript',
		cmLanguage: '@codemirror/lang-shell'
	},
	{
		id: 'javascript',
		name: 'JavaScript',
		extension: '.js',
		mimeType: 'text/javascript',
		cmLanguage: '@codemirror/lang-javascript'
	},
	{
		id: 'typescript',
		name: 'TypeScript',
		extension: '.ts',
		mimeType: 'text/typescript',
		cmLanguage: '@codemirror/lang-javascript'
	},
	{
		id: 'json',
		name: 'JSON',
		extension: '.json',
		mimeType: 'application/json',
		cmLanguage: '@codemirror/lang-json'
	},
	{
		id: 'properties',
		name: 'Module Properties',
		extension: '.prop',
		mimeType: 'text/plain',
		cmLanguage: '@codemirror/legacy-modes/mode/properties'
	},
	{
		id: 'markdown',
		name: 'Markdown',
		extension: '.md',
		mimeType: 'text/markdown',
		cmLanguage: '@codemirror/lang-markdown'
	}
];

/**
 * Get language config by file name
 */
export function getLanguageByFile(fileName: string): LanguageConfig | undefined {
	const ext = '.' + fileName.split('.').pop();
	return languages.find((l) => l.extension === ext);
}

/**
 * Get CodeMirror language package import path
 */
export function getLanguageCodeMirror(name: string): string | undefined {
	const lang = languages.find((l) => l.id === name || l.name === name);
	return lang?.cmLanguage;
}

/**
 * Get all supported extensions
 */
export function getSupportedExtensions(): string[] {
	return languages.map((l) => l.extension);
}

/**
 * Check if a file is supported
 */
export function isSupportedFile(fileName: string): boolean {
	return getLanguageByFile(fileName) !== undefined;
}

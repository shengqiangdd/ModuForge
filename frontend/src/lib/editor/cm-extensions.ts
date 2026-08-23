import { EditorView } from '@codemirror/view';
import { EditorState } from '@codemirror/state';
import { javascript } from '@codemirror/lang-javascript';
import { json } from '@codemirror/lang-json';
import { markdown } from '@codemirror/lang-markdown';
import { basicSetup } from 'codemirror';
import { search, searchKeymap } from '@codemirror/search';
import { keymap } from '@codemirror/view';
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
import { syntaxHighlighting, defaultHighlightStyle, bracketMatching, foldGutter, indentOnInput } from '@codemirror/language';
import { linter, type Diagnostic } from '@codemirror/lint';

/**
 * Dark theme matching ModuForge color scheme
 */
export function createEditorTheme() {
	return EditorView.theme({
		'&': {
			backgroundColor: '#0d1117',
			color: '#c9d1d9'
		},
		'.cm-content': {
			caretColor: '#58a6ff',
			fontFamily: "'Monaco', 'Menlo', 'Consolas', monospace",
			fontSize: '13px'
		},
		'.cm-cursor': {
			borderLeftColor: '#58a6ff'
		},
		'.cm-activeLine': {
			backgroundColor: '#161b22'
		},
		'.cm-selectionBackground': {
			backgroundColor: '#1f3a5f !important'
		},
		'.cm-gutters': {
			backgroundColor: '#0d1117',
			color: '#484f58',
			borderRight: '1px solid #21262d'
		},
		'.cm-activeLineGutter': {
			backgroundColor: '#161b22'
		},
		'.cm-lineNumbers .cm-gutterElement': {
			padding: '0 8px 0 12px'
		},
		'.cm-foldPlaceholder': {
			backgroundColor: '#21262d',
			color: '#8b949e',
			border: '1px solid #30363d'
		},
		'.cm-matchingBracket': {
			backgroundColor: '#264f78',
			color: '#fff'
		}
	}, { dark: true });
}

/**
 * Get language extension by filename
 */
export function getLanguageExtension(filename: string) {
	const ext = filename.split('.').pop()?.toLowerCase();
	switch (ext) {
		case 'js':
		case 'ts':
		case 'jsx':
		case 'tsx':
			return javascript();
		case 'json':
			return json();
		case 'md':
			return markdown();
		case 'sh':
			return javascript(); // Shell uses JS highlighting as fallback
		default:
			return javascript();
	}
}

/**
 * Error linter extension
 */
function errorLinter(errors: Array<{ line: number; column: number; message: string }>) {
	return linter((view) => {
		const diagnostics: Diagnostic[] = [];
		const doc = view.state.doc;

		for (const err of errors) {
			const line = doc.line(Math.min(err.line, doc.lines));
			diagnostics.push({
				from: line.from + Math.min(err.column - 1, line.length),
				to: line.to,
				severity: 'error',
				message: err.message
			});
		}

		return diagnostics;
	});
}

/**
 * Create complete editor extensions
 */
export function createEditorExtensions(
	filename: string,
	options: {
		errors?: Array<{ line: number; column: number; message: string }>;
		readOnly?: boolean;
	} = {}
) {
	const extensions = [
		basicSetup,
		createEditorTheme(),
		getLanguageExtension(filename),
		history(),
		search()
	];

	if (options.errors && options.errors.length > 0) {
		extensions.push(errorLinter(options.errors));
	}

	if (options.readOnly) {
		extensions.push(EditorState.readOnly.of(true));
		extensions.push(EditorView.editable.of(false));
	}

	return extensions;
}

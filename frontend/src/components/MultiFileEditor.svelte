<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { EditorView } from '@codemirror/view';
	import { EditorState } from '@codemirror/state';
	import { createEditorExtensions } from '$lib/editor/cm-extensions';

	interface FileTab {
		name: string;
		content: string;
		language?: string;
	}

	interface FileError {
		file: string;
		line: number;
		column: number;
		message: string;
	}

	let {
		files = [],
		activeFile = '',
		readOnly = false,
		errors = [],
		onFileChange = () => {},
		onFileSelect = () => {},
		onFileClose = () => {},
		onFormat = () => {}
	}: {
		files: FileTab[];
		activeFile: string;
		readOnly?: boolean;
		errors?: FileError[];
		onFileChange?: (file: string, content: string) => void;
		onFileSelect?: (file: string) => void;
		onFileClose?: (file: string) => void;
		onFormat?: (file: string) => void;
	} = $props();

	let editorContainer: HTMLDivElement | undefined = $state(undefined);
	let editorView: EditorView | undefined = $state(undefined);
	let cursorLine = $state(1);
	let cursorCol = $state(1);

	const currentFile = $derived(files.find((f) => f.name === activeFile));
	const fileErrors = $derived(errors.filter((e) => e.file === activeFile));

	function getLanguage(name: string): string {
		const ext = name.split('.').pop()?.toLowerCase() || '';
		const map: Record<string, string> = {
			js: 'JavaScript', ts: 'TypeScript', json: 'JSON',
			md: 'Markdown', sh: 'Shell', go: 'Go', prop: 'Properties'
		};
		return map[ext] || 'Plain Text';
	}

	function formatSize(content: string): string {
		const bytes = new TextEncoder().encode(content).length;
		if (bytes < 1024) return `${bytes} B`;
		return `${(bytes / 1024).toFixed(1)} KB`;
	}

	function closeTab(e: MouseEvent, name: string) {
		e.stopPropagation();
		onFileClose(name);
	}

	function initEditor() {
		if (!editorContainer || !currentFile) return;

		if (editorView) {
			editorView.destroy();
		}

		const extensions = createEditorExtensions(currentFile.name, {
			errors: fileErrors,
			readOnly
		});

		extensions.push(
			EditorView.updateListener.of((update) => {
				if (update.docChanged) {
					const content = update.state.doc.toString();
					onFileChange(activeFile, content);
				}
				const pos = update.state.selection.main.head;
				const line = update.state.doc.lineAt(pos);
				cursorLine = line.number;
				cursorCol = pos - line.from + 1;
			})
		);

		const state = EditorState.create({
			doc: currentFile.content,
			extensions
		});

		editorView = new EditorView({
			state,
			parent: editorContainer
		});
	}

	$effect(() => {
		if (activeFile && editorContainer) {
			initEditor();
		}
	});

	onMount(() => {
		if (activeFile) initEditor();
	});

	onDestroy(() => {
		editorView?.destroy();
	});
</script>

<div class="editor-wrapper">
	<!-- Tab bar -->
	<div class="tab-bar">
		<div class="tabs-scroll">
			{#each files as file}
				<button
					class="tab"
					class:active={file.name === activeFile}
					onclick={() => onFileSelect(file.name)}
				>
					<span class="tab-name">{file.name}</span>
					{#if !readOnly}
						<button class="tab-close" onclick={(e) => closeTab(e, file.name)}>×</button>
					{/if}
				</button>
			{/each}
		</div>
		<div class="tab-actions">
			{#if !readOnly}
				<button class="format-btn" onclick={() => onFormat(activeFile)} title="Format">⚙</button>
			{/if}
		</div>
	</div>

	<!-- Editor -->
	<div class="editor-area" bind:this={editorContainer}></div>

	<!-- Status bar -->
	<div class="status-bar">
		<span>{getLanguage(activeFile)}</span>
		<span>Ln {cursorLine}, Col {cursorCol}</span>
		{#if currentFile}
			<span>{formatSize(currentFile.content)}</span>
		{/if}
		{#if fileErrors.length > 0}
			<span class="error-count">{fileErrors.length} error(s)</span>
		{/if}
	</div>
</div>

<style>
	.editor-wrapper {
		display: flex;
		flex-direction: column;
		height: 100%;
		border: 1px solid #21262d;
		border-radius: 6px;
		overflow: hidden;
	}

	.tab-bar {
		display: flex;
		background: #0d1117;
		border-bottom: 1px solid #21262d;
	}

	.tabs-scroll {
		display: flex;
		overflow-x: auto;
		flex: 1;
	}

	.tabs-scroll::-webkit-scrollbar {
		height: 2px;
	}

	.tab {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.5rem 0.75rem;
		border: none;
		background: transparent;
		color: #8b949e;
		cursor: pointer;
		font-size: 0.8rem;
		white-space: nowrap;
		border-right: 1px solid #21262d;
	}

	.tab.active {
		background: #161b22;
		color: #c9d1d9;
		border-bottom: 2px solid #58a6ff;
	}

	.tab-close {
		border: none;
		background: transparent;
		color: #8b949e;
		cursor: pointer;
		font-size: 1rem;
		padding: 0;
		line-height: 1;
	}

	.tab-close:hover {
		color: #f85149;
	}

	.tab-actions {
		display: flex;
		align-items: center;
		padding-right: 0.5rem;
	}

	.format-btn {
		border: none;
		background: transparent;
		color: #8b949e;
		cursor: pointer;
		font-size: 1rem;
		padding: 0.25rem;
	}

	.format-btn:hover {
		color: #58a6ff;
	}

	.editor-area {
		flex: 1;
		overflow: hidden;
	}

	.status-bar {
		display: flex;
		gap: 1rem;
		padding: 0.25rem 0.75rem;
		background: #161b22;
		border-top: 1px solid #21262d;
		font-size: 0.7rem;
		color: #8b949e;
	}

	.error-count {
		color: #f85149;
	}
</style>

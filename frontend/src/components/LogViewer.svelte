<script lang="ts">
	import { parseStructuredLog, filterByLevel, searchInLogs, generateSummary, parseAnsiLog } from '$lib/utils/logParser';
	import type { LogLevel, ParsedLine } from '$lib/utils/logParser';

	let { logs = '', autoScroll = true }: { logs?: string; autoScroll?: boolean } = $props();

	let filterLevel = $state<LogLevel | 'all'>('all');
	let searchQuery = $state('');
	let containerEl: HTMLDivElement | undefined = $state(undefined);

	const allLines = $derived(parseStructuredLog(logs));
	const filteredLines = $derived.by(() => {
		let result = allLines;
		if (filterLevel !== 'all') {
			result = filterByLevel(result, filterLevel);
		}
		if (searchQuery) {
			result = searchInLogs(result, searchQuery);
		}
		return result;
	});
	const summary = $derived(generateSummary(allLines));

	function getLevelColor(level: LogLevel): string {
		switch (level) {
			case 'error': return '#f44336';
			case 'warn': return '#ff9800';
			case 'success': return '#4caf50';
			case 'debug': return '#9e9e9e';
			default: return '#2196f3';
		}
	}

	function highlightErrors(line: string): string {
		return line.replace(/\b(error|failed?|fatal)\b/gi, '<mark>$1</mark>');
	}

	$effect(() => {
		if (containerEl && autoScroll) {
			containerEl.scrollTop = containerEl.scrollHeight;
		}
	});
</script>

<div class="log-viewer">
	<div class="toolbar">
		<div class="filters">
			<button class:active={filterLevel === 'all'} onclick={() => filterLevel = 'all'}>All</button>
			<button class:active={filterLevel === 'error'} onclick={() => filterLevel = 'error'}>Errors</button>
			<button class:active={filterLevel === 'warn'} onclick={() => filterLevel = 'warn'}>Warnings</button>
			<button class:active={filterLevel === 'info'} onclick={() => filterLevel = 'info'}>Info</button>
		</div>
		<input type="text" placeholder="Search logs..." bind:value={searchQuery} class="search-input" />
	</div>

	<div class="log-content" bind:this={containerEl}>
		{#if filteredLines.length === 0}
			<div class="empty">No log entries</div>
		{/if}
		{#each filteredLines as entry}
			<div class="log-line" class:error={entry.level === 'error'} class:warn={entry.level === 'warn'}>
				<span class="line-num">{entry.line}</span>
				<span class="level-dot" style="background: {getLevelColor(entry.level)}"></span>
				<span class="message">{@html highlightErrors(parseAnsiLog(entry.message))}</span>
			</div>
		{/each}
	</div>

	<div class="summary">
		<span class="stat">{summary.total} lines</span>
		{#if summary.errors > 0}
			<span class="stat error">{summary.errors} errors</span>
		{/if}
		{#if summary.warnings > 0}
			<span class="stat warn">{summary.warnings} warnings</span>
		{/if}
		{#if summary.success > 0}
			<span class="stat success">{summary.success} success</span>
		{/if}
	</div>
</div>

<style>
	.log-viewer {
		border: 1px solid #333;
		border-radius: 8px;
		background: #1a1a2e;
		color: #eee;
		font-family: 'Monaco', 'Menlo', monospace;
		font-size: 0.8rem;
		display: flex;
		flex-direction: column;
		height: 400px;
	}

	.toolbar {
		display: flex;
		gap: 0.5rem;
		padding: 0.5rem;
		border-bottom: 1px solid #333;
		flex-wrap: wrap;
	}

	.filters {
		display: flex;
		gap: 0.25rem;
	}

	.filters button {
		padding: 0.25rem 0.5rem;
		border: 1px solid #444;
		border-radius: 4px;
		background: transparent;
		color: #aaa;
		cursor: pointer;
		font-size: 0.75rem;
	}

	.filters button.active {
		background: #333;
		color: #fff;
		border-color: #666;
	}

	.search-input {
		flex: 1;
		min-width: 150px;
		padding: 0.25rem 0.5rem;
		border: 1px solid #444;
		border-radius: 4px;
		background: #0d1117;
		color: #eee;
		font-size: 0.75rem;
	}

	.log-content {
		flex: 1;
		overflow-y: auto;
		padding: 0.5rem;
	}

	.log-line {
		display: flex;
		gap: 0.5rem;
		padding: 0.125rem 0;
		align-items: flex-start;
	}

	.log-line.error {
		background: rgba(244, 67, 54, 0.1);
	}

	.log-line.warn {
		background: rgba(255, 152, 0, 0.1);
	}

	.line-num {
		color: #555;
		min-width: 2rem;
		text-align: right;
		user-select: none;
	}

	.level-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
		margin-top: 4px;
	}

	.message {
		flex: 1;
		word-break: break-all;
		white-space: pre-wrap;
	}

	.summary {
		display: flex;
		gap: 1rem;
		padding: 0.5rem;
		border-top: 1px solid #333;
		font-size: 0.7rem;
	}

	.stat { color: #888; }
	.stat.error { color: #f44336; }
	.stat.warn { color: #ff9800; }
	.stat.success { color: #4caf50; }

	.empty {
		text-align: center;
		color: #666;
		padding: 2rem;
	}
</style>

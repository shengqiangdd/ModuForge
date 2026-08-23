<script lang="ts">
	interface LogEntry {
		timestamp: string;
		message: string;
		level: 'info' | 'thinking' | 'action' | 'error';
	}

	let { logs = [], collapsed = false }: {
		logs: LogEntry[];
		collapsed?: boolean;
	} = $props();

	let isCollapsed = $state(collapsed);
	let logsContainer: HTMLDivElement | undefined = $state(undefined);

	const MAX_LOGS = 100;

	const displayLogs = $derived(logs.slice(-MAX_LOGS));

	function toggleCollapse() {
		isCollapsed = !isCollapsed;
	}

	function formatTime(ts: string): string {
		try {
			const d = new Date(ts);
			return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
		} catch {
			return ts;
		}
	}

	function getLevelIcon(level: string): string {
		switch (level) {
			case 'error': return '✗';
			case 'action': return '→';
			case 'thinking': return '💭';
			default: return 'ℹ';
		}
	}

	function getLevelColor(level: string): string {
		switch (level) {
			case 'error': return '#f44336';
			case 'action': return '#2196f3';
			case 'thinking': return '#9c27b0';
			default: return '#666';
		}
	}

	$effect(() => {
		if (logsContainer && !isCollapsed) {
			logsContainer.scrollTop = logsContainer.scrollHeight;
		}
	});
</script>

<div class="ai-thinking-panel" class:collapsed={isCollapsed}>
	<button class="header" onclick={toggleCollapse}>
		<span class="title">AI Thinking Process</span>
		<span class="toggle">{isCollapsed ? '▸' : '▾'}</span>
	</button>

	{#if !isCollapsed}
		<div class="logs" bind:this={logsContainer}>
			{#each displayLogs as log}
				<div class="log-entry {log.level}">
					<span class="time">{formatTime(log.timestamp)}</span>
					<span class="icon" style="color: {getLevelColor(log.level)}">
						{getLevelIcon(log.level)}
					</span>
					<span class="message">{log.message}</span>
				</div>
			{/each}

			{#if displayLogs.length === 0}
				<div class="empty">Waiting for AI response...</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.ai-thinking-panel {
		border: 1px solid #333;
		border-radius: 8px;
		background: #1a1a2e;
		color: #eee;
		font-family: 'Monaco', 'Menlo', monospace;
		font-size: 0.8rem;
		overflow: hidden;
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.75rem 1rem;
		background: #16213e;
		border: none;
		color: #eee;
		cursor: pointer;
		width: 100%;
		text-align: left;
	}

	.header:hover {
		background: #1a2744;
	}

	.title {
		font-weight: 600;
		font-size: 0.875rem;
	}

	.toggle {
		font-size: 1rem;
		opacity: 0.7;
	}

	.logs {
		max-height: 400px;
		overflow-y: auto;
		padding: 0.5rem;
	}

	.log-entry {
		display: flex;
		gap: 0.5rem;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		align-items: flex-start;
	}

	.log-entry:hover {
		background: rgba(255, 255, 255, 0.05);
	}

	.time {
		color: #666;
		flex-shrink: 0;
		font-size: 0.7rem;
	}

	.icon {
		flex-shrink: 0;
		width: 1rem;
		text-align: center;
	}

	.message {
		flex: 1;
		word-break: break-word;
		white-space: pre-wrap;
	}

	.log-entry.error .message {
		color: #f44336;
	}

	.log-entry.thinking .message {
		color: #bb86fc;
	}

	.log-entry.action .message {
		color: #82b1ff;
	}

	.empty {
		text-align: center;
		color: #666;
		padding: 2rem;
	}

	/* Scrollbar styling */
	.logs::-webkit-scrollbar {
		width: 6px;
	}

	.logs::-webkit-scrollbar-track {
		background: transparent;
	}

	.logs::-webkit-scrollbar-thumb {
		background: #444;
		border-radius: 3px;
	}
</style>

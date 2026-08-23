<script lang="ts">
	interface HistoryEntry {
		timestamp: string;
		author: string;
		version: number;
		changes: string;
	}

	let {
		history = [],
		onRestore = () => {}
	}: {
		history: HistoryEntry[];
		onRestore?: (version: number) => void;
	} = $props();

	let confirmVersion = $state<number | null>(null);

	function formatTime(ts: string): string {
		try {
			const d = new Date(ts);
			return d.toLocaleString('en-US', {
				month: 'short', day: 'numeric',
				hour: '2-digit', minute: '2-digit'
			});
		} catch {
			return ts;
		}
	}

	function requestRestore(version: number) {
		confirmVersion = version;
	}

	function confirmRestore() {
		if (confirmVersion !== null) {
			onRestore(confirmVersion);
			confirmVersion = null;
		}
	}

	function cancelRestore() {
		confirmVersion = null;
	}
</script>

<div class="file-history">
	<h3>File History</h3>

	{#if history.length === 0}
		<div class="empty">No history available</div>
	{/if}

	<div class="timeline">
		{#each history as entry, i}
			<div class="entry" class:latest={i === 0}>
				<div class="dot"></div>
				{#if i < history.length - 1}
					<div class="line"></div>
				{/if}
				<div class="content">
					<div class="header">
						<span class="version">v{entry.version}</span>
						<span class="time">{formatTime(entry.timestamp)}</span>
					</div>
					<div class="author">{entry.author}</div>
					<div class="changes">{entry.changes}</div>
					{#if confirmVersion === entry.version}
						<div class="confirm-dialog">
							<span>Restore this version?</span>
							<button class="confirm-btn" onclick={confirmRestore}>Yes</button>
							<button class="cancel-btn" onclick={cancelRestore}>No</button>
						</div>
					{:else}
						<button class="restore-btn" onclick={() => requestRestore(entry.version)}>
							Restore
						</button>
					{/if}
				</div>
			</div>
		{/each}
	</div>
</div>

<style>
	.file-history {
		padding: 1rem;
		max-width: 300px;
		background: #161b22;
		border-radius: 8px;
		color: #c9d1d9;
	}

	h3 {
		margin: 0 0 1rem;
		font-size: 0.9rem;
	}

	.empty {
		text-align: center;
		color: #8b949e;
		padding: 2rem 0;
		font-size: 0.85rem;
	}

	.timeline {
		position: relative;
		padding-left: 1.5rem;
	}

	.entry {
		position: relative;
		padding-bottom: 1.5rem;
	}

	.dot {
		position: absolute;
		left: -1.5rem;
		top: 0.25rem;
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: #30363d;
		border: 2px solid #58a6ff;
		z-index: 1;
	}

	.entry.latest .dot {
		background: #58a6ff;
	}

	.line {
		position: absolute;
		left: calc(-1.5rem + 4px);
		top: 12px;
		width: 2px;
		height: calc(100% - 12px);
		background: #21262d;
	}

	.content {
		background: #0d1117;
		padding: 0.75rem;
		border-radius: 6px;
		border: 1px solid #21262d;
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.25rem;
	}

	.version {
		font-weight: 600;
		font-size: 0.85rem;
		color: #58a6ff;
	}

	.time {
		font-size: 0.7rem;
		color: #8b949e;
	}

	.author {
		font-size: 0.75rem;
		color: #8b949e;
		margin-bottom: 0.25rem;
	}

	.changes {
		font-size: 0.8rem;
		color: #c9d1d9;
		margin-bottom: 0.5rem;
	}

	.restore-btn {
		padding: 0.25rem 0.5rem;
		border: 1px solid #30363d;
		border-radius: 4px;
		background: transparent;
		color: #8b949e;
		cursor: pointer;
		font-size: 0.75rem;
	}

	.restore-btn:hover {
		background: #21262d;
		color: #c9d1d9;
	}

	.confirm-dialog {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.75rem;
	}

	.confirm-btn, .cancel-btn {
		padding: 0.25rem 0.5rem;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-size: 0.75rem;
	}

	.confirm-btn {
		background: #238636;
		color: white;
	}

	.cancel-btn {
		background: #30363d;
		color: #8b949e;
	}
</style>

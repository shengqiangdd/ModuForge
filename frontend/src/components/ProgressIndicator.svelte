<script lang="ts">
	interface Step {
		name: string;
		status: 'pending' | 'running' | 'completed' | 'failed';
		duration?: number;
	}

	let { steps = [], currentStep = 0, estimatedTotal = 0 }: {
		steps: Step[];
		currentStep: number;
		estimatedTotal?: number;
	} = $props();

	function getStatusIcon(status: string): string {
		switch (status) {
			case 'completed': return '✓';
			case 'failed': return '✗';
			case 'running': return '●';
			default: return '○';
		}
	}

	function formatDuration(ms?: number): string {
		if (!ms) return '';
		if (ms < 1000) return `${ms}ms`;
		return `${(ms / 1000).toFixed(1)}s`;
	}
</script>

<div class="progress-indicator" class:mobile={typeof window !== 'undefined' && window.innerWidth < 768}>
	<div class="steps">
		{#each steps as step, i}
			<div class="step" class:active={i === currentStep} class:completed={step.status === 'completed'} class:failed={step.status === 'failed'}>
				<div class="icon {step.status}">
					{getStatusIcon(step.status)}
				</div>
				<div class="info">
					<span class="name">{step.name}</span>
					{#if step.duration}
						<span class="duration">{formatDuration(step.duration)}</span>
					{/if}
				</div>
				{#if i < steps.length - 1}
					<div class="connector" class:completed={step.status === 'completed'}></div>
				{/if}
			</div>
		{/each}
	</div>

	{#if estimatedTotal > 0}
		<div class="total-estimate">
			Estimated total: {formatDuration(estimatedTotal)}
		</div>
	{/if}
</div>

<style>
	.progress-indicator {
		padding: 1rem;
		font-family: system-ui, -apple-system, sans-serif;
	}

	.steps {
		display: flex;
		align-items: flex-start;
		gap: 0;
	}

	.step {
		display: flex;
		flex-direction: column;
		align-items: center;
		position: relative;
		flex: 1;
		min-width: 0;
	}

	.icon {
		width: 32px;
		height: 32px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 14px;
		font-weight: 600;
		background: #e0e0e0;
		color: #666;
		transition: all 0.3s ease;
		z-index: 1;
	}

	.icon.completed {
		background: #4caf50;
		color: white;
	}

	.icon.failed {
		background: #f44336;
		color: white;
	}

	.icon.running {
		background: #2196f3;
		color: white;
		animation: pulse 1.5s infinite;
	}

	@keyframes pulse {
		0%, 100% { transform: scale(1); }
		50% { transform: scale(1.1); }
	}

	.info {
		margin-top: 0.5rem;
		text-align: center;
	}

	.name {
		display: block;
		font-size: 0.75rem;
		color: #333;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 80px;
	}

	.duration {
		display: block;
		font-size: 0.625rem;
		color: #888;
		margin-top: 0.25rem;
	}

	.connector {
		position: absolute;
		top: 16px;
		left: calc(50% + 16px);
		width: calc(100% - 32px);
		height: 2px;
		background: #e0e0e0;
	}

	.connector.completed {
		background: #4caf50;
	}

	.total-estimate {
		margin-top: 1rem;
		text-align: center;
		font-size: 0.75rem;
		color: #666;
	}

	/* Mobile: vertical layout */
	@media (max-width: 767px) {
		.steps {
			flex-direction: column;
			gap: 0.5rem;
		}

		.step {
			flex-direction: row;
			gap: 0.75rem;
		}

		.connector {
			display: none;
		}

		.name {
			max-width: none;
		}
	}
</style>

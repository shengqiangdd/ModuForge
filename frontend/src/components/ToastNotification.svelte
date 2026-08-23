<script lang="ts">
	interface Action {
		label: string;
		onClick: () => void;
	}

	let {
		type = 'info',
		message = '',
		action = undefined,
		duration = 3000,
		onDismiss = () => {}
	}: {
		type?: 'success' | 'error' | 'warning' | 'info';
		message?: string;
		action?: Action;
		duration?: number;
		onDismiss?: () => void;
	} = $props();

	let visible = $state(true);
	let exiting = $state(false);

	function getIcon(): string {
		switch (type) {
			case 'success': return '✓';
			case 'error': return '✗';
			case 'warning': return '⚠';
			default: return 'ℹ';
		}
	}

	function dismiss() {
		exiting = true;
		setTimeout(() => {
			visible = false;
			onDismiss();
		}, 300);
	}

	$effect(() => {
		if (duration > 0) {
			const timer = setTimeout(dismiss, duration);
			return () => clearTimeout(timer);
		}
	});
</script>

{#if visible}
	<div class="toast {type}" class:exiting={exiting} role="alert">
		<span class="icon">{getIcon()}</span>
		<span class="message">{message}</span>
		{#if action}
			<button class="action-btn" onclick={action.onClick}>{action.label}</button>
		{/if}
		<button class="close-btn" onclick={dismiss} aria-label="Close">×</button>
	</div>
{/if}

<style>
	.toast {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.875rem 1rem;
		border-radius: 8px;
		background: #333;
		color: white;
		font-size: 0.875rem;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
		animation: slideIn 0.3s ease;
		max-width: 400px;
		min-width: 280px;
	}

	.toast.exiting {
		animation: slideOut 0.3s ease forwards;
	}

	.toast.success {
		background: #2e7d32;
		border-left: 4px solid #4caf50;
	}

	.toast.error {
		background: #c62828;
		border-left: 4px solid #f44336;
	}

	.toast.warning {
		background: #f57f17;
		border-left: 4px solid #ffeb3b;
	}

	.toast.info {
		background: #1565c0;
		border-left: 4px solid #42a5f5;
	}

	.icon {
		font-size: 1.125rem;
		flex-shrink: 0;
	}

	.message {
		flex: 1;
	}

	.action-btn {
		background: rgba(255, 255, 255, 0.2);
		border: none;
		color: white;
		padding: 0.375rem 0.75rem;
		border-radius: 4px;
		cursor: pointer;
		font-size: 0.8125rem;
		white-space: nowrap;
	}

	.action-btn:hover {
		background: rgba(255, 255, 255, 0.3);
	}

	.close-btn {
		background: none;
		border: none;
		color: white;
		font-size: 1.25rem;
		cursor: pointer;
		opacity: 0.7;
		padding: 0;
		line-height: 1;
	}

	.close-btn:hover {
		opacity: 1;
	}

	@keyframes slideIn {
		from {
			transform: translateX(100%);
			opacity: 0;
		}
		to {
			transform: translateX(0);
			opacity: 1;
		}
	}

	@keyframes slideOut {
		from {
			transform: translateX(0);
			opacity: 1;
		}
		to {
			transform: translateX(100%);
			opacity: 0;
		}
	}
</style>

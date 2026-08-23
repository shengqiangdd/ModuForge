<script lang="ts">
	import { onMount } from 'svelte';

	const STORAGE_KEY = 'moduforge_onboarding_complete';

	let currentStep = $state(0);
	let visible = $state(false);

	const steps = [
		{
			title: 'Enter Your Requirement',
			description: 'Describe what module you want to create. Be as specific as possible.',
			highlight: '#requirement-input'
		},
		{
			title: 'Generate Module',
			description: 'Click Generate to let AI create your Magisk module. Watch the progress.',
			highlight: '#generate-btn'
		},
		{
			title: 'View Logs & Results',
			description: 'Check the build logs and download your module when ready.',
			highlight: '#log-viewer'
		}
	];

	onMount(() => {
		const completed = localStorage.getItem(STORAGE_KEY);
		if (!completed) {
			visible = true;
		}
	});

	function next() {
		if (currentStep < steps.length - 1) {
			currentStep++;
		} else {
			complete();
		}
	}

	function skip() {
		complete();
	}

	function complete() {
		localStorage.setItem(STORAGE_KEY, 'true');
		visible = false;
	}
</script>

{#if visible}
	<div class="onboarding-overlay">
		<div class="wizard">
			<div class="header">
				<h2>Welcome to ModuForge</h2>
				<span class="step-indicator">Step {currentStep + 1} of {steps.length}</span>
			</div>

			<div class="content">
				<div class="step-number">{currentStep + 1}</div>
				<h3>{steps[currentStep].title}</h3>
				<p>{steps[currentStep].description}</p>
			</div>

			<div class="progress">
				{#each steps as _, i}
					<div class="dot" class:active={i === currentStep} class:done={i < currentStep}></div>
				{/each}
			</div>

			<div class="actions">
				<button class="skip-btn" onclick={skip}>Skip</button>
				<button class="next-btn" onclick={next}>
					{currentStep === steps.length - 1 ? 'Get Started' : 'Next'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.onboarding-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
	}

	.wizard {
		background: white;
		border-radius: 12px;
		padding: 2rem;
		max-width: 400px;
		width: 90%;
		box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	h2 {
		font-size: 1.1rem;
		margin: 0;
	}

	.step-indicator {
		font-size: 0.75rem;
		color: #888;
	}

	.content {
		text-align: center;
		padding: 1rem 0;
	}

	.step-number {
		width: 48px;
		height: 48px;
		border-radius: 50%;
		background: #2196f3;
		color: white;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 1.5rem;
		font-weight: bold;
		margin: 0 auto 1rem;
	}

	h3 {
		margin: 0 0 0.5rem;
		font-size: 1rem;
	}

	p {
		margin: 0;
		color: #666;
		font-size: 0.9rem;
	}

	.progress {
		display: flex;
		justify-content: center;
		gap: 0.5rem;
		margin: 1.5rem 0;
	}

	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: #ddd;
		transition: all 0.3s;
	}

	.dot.active {
		background: #2196f3;
		transform: scale(1.3);
	}

	.dot.done {
		background: #4caf50;
	}

	.actions {
		display: flex;
		justify-content: space-between;
		gap: 1rem;
	}

	.skip-btn {
		padding: 0.5rem 1rem;
		border: none;
		background: transparent;
		color: #888;
		cursor: pointer;
		font-size: 0.9rem;
	}

	.next-btn {
		padding: 0.5rem 1.5rem;
		border: none;
		background: #2196f3;
		color: white;
		border-radius: 6px;
		cursor: pointer;
		font-size: 0.9rem;
	}

	.next-btn:hover {
		background: #1976d2;
	}
</style>

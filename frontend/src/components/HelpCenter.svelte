<script lang="ts">
	let faqOpen = $state<number | null>(null);

	const faqs = [
		{ q: 'How to create a module?', a: 'Enter your requirement in the prompt field and click Generate. The AI will create a complete Magisk module structure.' },
		{ q: 'What languages are supported?', a: 'Go, Shell (Bash), C, and JSON configuration files are fully supported.' },
		{ q: 'How to fix build errors?', a: 'The system includes auto-fix capabilities. If a build fails, it will attempt to fix errors automatically up to 3 times.' },
		{ q: 'Can I use custom templates?', a: 'Yes! You can save your own prompts as templates and reuse them from the template library.' },
		{ q: 'How to preview module before install?', a: 'After generation, click "Preview" to see the module structure, or "Download" to get the zip file.' }
	];

	const examples = [
		'Detect if device is rooted',
		'Hide app from root detection',
		'Change build.prop properties',
		'Optimize CPU performance',
		'Monitor battery usage',
		'Block system ads'
	];

	const shortcuts = [
		{ key: 'Ctrl+Enter', action: 'Generate module' },
		{ key: 'Ctrl+S', action: 'Save current work' },
		{ key: 'Ctrl+/', action: 'Toggle help' },
		{ key: 'Escape', action: 'Close panels' }
	];

	function toggleFaq(index: number) {
		faqOpen = faqOpen === index ? null : index;
	}
</script>

<div class="help-center">
	<section class="faq">
		<h3>Frequently Asked Questions</h3>
		{#each faqs as faq, i}
			<div class="faq-item" class:open={faqOpen === i}>
				<button class="faq-question" onclick={() => toggleFaq(i)}>
					<span>{faq.q}</span>
					<span class="arrow">{faqOpen === i ? '▾' : '▸'}</span>
				</button>
				{#if faqOpen === i}
					<div class="faq-answer">{faq.a}</div>
				{/if}
			</div>
		{/each}
	</section>

	<section class="examples">
		<h3>Example Requirements</h3>
		<div class="example-list">
			{#each examples as example}
				<button class="example-btn">{example}</button>
			{/each}
		</div>
	</section>

	<section class="shortcuts">
		<h3>Keyboard Shortcuts</h3>
		<div class="shortcut-list">
			{#each shortcuts as s}
				<div class="shortcut-item">
					<kbd>{s.key}</kbd>
					<span>{s.action}</span>
				</div>
			{/each}
		</div>
	</section>
</div>

<style>
	.help-center {
		padding: 1rem;
		max-width: 600px;
	}

	section {
		margin-bottom: 1.5rem;
	}

	h3 {
		font-size: 0.9rem;
		margin-bottom: 0.75rem;
		color: #333;
	}

	.faq-item {
		border: 1px solid #ddd;
		border-radius: 6px;
		margin-bottom: 0.5rem;
		overflow: hidden;
	}

	.faq-question {
		width: 100%;
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.75rem;
		background: #f5f5f5;
		border: none;
		cursor: pointer;
		text-align: left;
		font-size: 0.85rem;
	}

	.faq-answer {
		padding: 0.75rem;
		font-size: 0.8rem;
		color: #555;
		border-top: 1px solid #ddd;
		background: white;
	}

	.example-list {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.example-btn {
		padding: 0.5rem 0.75rem;
		border: 1px solid #ddd;
		border-radius: 20px;
		background: white;
		cursor: pointer;
		font-size: 0.8rem;
		transition: all 0.2s;
	}

	.example-btn:hover {
		background: #e3f2fd;
		border-color: #2196f3;
	}

	.shortcut-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.shortcut-item {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		font-size: 0.8rem;
	}

	kbd {
		padding: 0.25rem 0.5rem;
		border: 1px solid #ccc;
		border-radius: 4px;
		background: #f5f5f5;
		font-family: monospace;
		font-size: 0.75rem;
	}

	@media (max-width: 768px) {
		.help-center {
			padding: 0.75rem;
		}

		.example-list {
			flex-direction: column;
		}
	}
</style>

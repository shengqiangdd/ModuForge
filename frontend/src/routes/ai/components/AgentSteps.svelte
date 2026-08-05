<script lang="ts">
  import ToolResult from './ToolResult.svelte';

  type AgentStep = {
    type: 'think' | 'skill_call' | 'skill_result' | 'answer' | 'reasoning';
    skill?: string;
    input?: any;
    content?: string;
    round?: number;
  };

  let {
    steps = [],
    collapsed = true,
    expandedSteps = new Set<number>(),
    maxRoundIndex = 0,
    selectedRound = 0,
    agentMode = 'act' as 'plan' | 'act',
    onToggleCollapse,
    onPrevRound,
    onNextRound,
    onSetAgentMode,
    onToggleStep,
  }: {
    steps: AgentStep[];
    collapsed?: boolean;
    expandedSteps?: Set<number>;
    maxRoundIndex?: number;
    selectedRound?: number;
    agentMode?: 'plan' | 'act';
    onToggleCollapse?: () => void;
    onPrevRound?: () => void;
    onNextRound?: () => void;
    onSetAgentMode?: (mode: 'plan' | 'act') => void;
    onToggleStep?: (index: number) => void;
  } = $props();

  let filteredSteps = $derived(
    steps.filter(s => !(s.type === 'think' && s.content && s.content.length < 30))
  );

  // 按当前轮次过滤步骤，显示当前轮次的步数
  let currentRoundSteps = $derived(
    steps.filter(s => s.round === selectedRound)
  );

  // Check if there's an active thinking step (last think step without a following skill_call or answer)
  let hasActiveThinking = $derived(
    steps.length > 0 &&
    steps[steps.length - 1].type === 'think' &&
    !steps.some((s, i) => i === steps.length - 1 && s.type === 'answer')
  );

  function toggleStep(index: number) {
    onToggleStep?.(index);
  }

  /** Build a short parameter summary for tool calls */
  function buildParamSummary(input: any): string {
    if (!input) return '';
    const parts: string[] = [];
    if (input.path) {
      const filename = input.path.split('/').pop() || input.path;
      parts.push(filename);
    }
    if (input.pattern) parts.push(`"${input.pattern}"`);
    if (input.command) parts.push(input.command.slice(0, 40));
    if (input.old_text) parts.push(`"${input.old_text.slice(0, 20)}..."`);
    if (input.content && !input.path) parts.push(`${input.content.length} chars`);
    if (input.files && Array.isArray(input.files)) parts.push(`${input.files.length} files`);
    if (input.project_id) parts.push(input.project_id.slice(0, 8));
    return parts.join(' · ') || '';
  }

  /** Build tool icon based on skill name */
  function toolIcon(skill: string): string {
    if (skill === 'read_file') return '📖';
    if (skill === 'write_file' || skill === 'write_file_batch') return '✏️';
    if (skill === 'edit_file') return '🔍';
    if (skill === 'bash') return '💻';
    if (skill === 'build_module') return '🔨';
    if (skill === 'test_module') return '🧪';
    if (skill === 'grep_search') return '🔎';
    if (skill === 'glob_search') return '📁';
    if (skill === 'list_dir') return '📂';
    return '🔧';
  }
</script>

{#if steps.length > 0}
  <div class="agent-timeline flex-shrink-0 px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
    <div class="flex items-center gap-2 mb-1.5">
      <button class="flex items-center gap-1.5 flex-1 text-left" onclick={onToggleCollapse}>
        <span class="text-[14px]">🤖</span>
        <span class="text-xs font-semibold text-[var(--color-text)]">Agent 步骤</span>
        <span class="text-[10px] text-[var(--color-text-muted)]">{currentRoundSteps.length} 步</span>
        {#if maxRoundIndex > 0}
          <span class="text-[10px] px-1.5 py-0.5 rounded" style="background: var(--color-surface); border: 1px solid var(--color-border); color: var(--color-text-muted);">
            轮次 {selectedRound + 1}/{maxRoundIndex + 1}
          </span>
        {/if}
        <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)] transition-transform {collapsed ? '' : 'rotate-180'}">expand_less</span>
      </button>
      {#if maxRoundIndex > 0}
        <div class="flex items-center gap-0.5 text-[10px]">
          <button class="p-0.5 rounded hover:bg-[var(--color-bg-hover)] disabled:opacity-30" disabled={selectedRound <= 0} onclick={onPrevRound} title="上一轮">
            <span class="material-symbols-outlined text-[14px]">chevron_left</span>
          </button>
          <button class="p-0.5 rounded hover:bg-[var(--color-bg-hover)] disabled:opacity-30" disabled={selectedRound >= maxRoundIndex} onclick={onNextRound} title="下一轮">
            <span class="material-symbols-outlined text-[14px]">chevron_right</span>
          </button>
        </div>
      {/if}
      <div class="flex items-center gap-1 text-[10px]">
        <button class="px-1.5 py-0.5 rounded text-[10px] transition-colors {agentMode === 'plan' ? 'bg-[var(--color-primary)] text-white' : 'text-[var(--color-text-muted)] hover:bg-[var(--color-bg-hover)]'}" onclick={() => onSetAgentMode?.('plan')} title="Plan 模式：只读探索，不修改文件">📋 Plan</button>
        <button class="px-1.5 py-0.5 rounded text-[10px] transition-colors {agentMode === 'act' ? 'bg-[var(--color-primary)] text-white' : 'text-[var(--color-text-muted)] hover:bg-[var(--color-bg-hover)]'}" onclick={() => onSetAgentMode?.('act')} title="Act 模式：完整执行，包含文件写入">⚡ Act</button>
      </div>
    </div>
    {#if !collapsed}
      <div class="mt-1.5 space-y-1 agent-steps-scroll">
        {#each filteredSteps as step, i}
          <div class="step-card" style="animation: fadeInUp 0.2s ease-out both; animation-delay: {i * 0.03}s">
            {#if step.type === 'think'}
              <div class="flex gap-2 items-start">
                <div class="step-icon think">
                  {#if hasActiveThinking && i === filteredSteps.length - 1}
                    <span class="animate-pulse">🧠</span>
                  {:else}
                    🧠
                  {/if}
                </div>
                <div class="step-content flex-1 min-w-0">
                  <div class="step-label think">
                    思考
                    {#if hasActiveThinking && i === filteredSteps.length - 1}
                      <span class="thinking-dots">
                        <span class="dot">.</span><span class="dot">.</span><span class="dot">.</span>
                      </span>
                    {/if}
                  </div>
                  <div class="text-[11px] mt-0.5 step-result-content {expandedSteps.has(i) ? 'expanded' : ''}" style="color: var(--color-text-secondary);">{step.content}</div>
                  {#if step.content && (step.content.split('\n').length > 3 || step.content.length > 200)}
                    <button class="text-[9px] mt-0.5 hover:underline" style="color: #3b82f6;" onclick={() => toggleStep(i)}>
                      {expandedSteps.has(i) ? '收起' : '展开'}
                    </button>
                  {/if}
                </div>
              </div>
            {:else if step.type === 'skill_call'}
              <div class="flex gap-2 items-start">
                <div class="step-icon skill">{toolIcon(step.skill || '')}</div>
                <div class="step-content flex-1 min-w-0">
                  <div class="step-label skill">
                    <span class="font-mono text-[10px]">{step.skill}</span>
                    {#if step.input}
                      {@const summary = buildParamSummary(step.input)}
                      {#if summary}
                        <span class="text-[10px] text-[var(--color-text-muted)] font-normal ml-1">{summary}</span>
                      {/if}
                    {/if}
                  </div>
                  {#if step.input}
                    {@const inputStr = JSON.stringify(step.input, null, 2)}
                    <pre class="step-detail text-[10px] mt-0.5 font-mono step-result-content {expandedSteps.has(i) ? 'expanded' : ''}" style="color: var(--color-text-muted);">{inputStr}</pre>
                    {#if inputStr.length > 150}
                      <button class="text-[9px] mt-0.5 hover:underline" style="color: #d97706;" onclick={() => toggleStep(i)}>
                        {expandedSteps.has(i) ? '收起' : '展开参数'}
                      </button>
                    {/if}
                  {/if}
                </div>
              </div>
            {:else if step.type === 'skill_result'}
              <div class="flex gap-2 items-start">
                <div class="step-icon result">📋</div>
                <div class="step-content flex-1 min-w-0">
                  <div class="step-label result">结果: {step.skill}</div>
                  <ToolResult
                    skillName={step.skill || ''}
                    content={step.content || ''}
                    expanded={expandedSteps.has(i)}
                    onToggle={() => toggleStep(i)}
                  />
                </div>
              </div>
            {:else if step.type === 'answer'}
              <div class="flex gap-2 items-start">
                <div class="step-icon answer">✅</div>
                <div class="step-content flex-1 min-w-0">
                  <div class="step-label answer">完成</div>
                  <div class="text-xs mt-0.5 step-result-content {expandedSteps.has(i) ? 'expanded' : ''}" style="color: var(--color-text);">{step.content}</div>
                  {#if step.content && (step.content.split('\n').length > 5 || step.content.length > 300)}
                    <button class="text-[9px] mt-0.5 hover:underline" style="color: var(--color-primary);" onclick={() => toggleStep(i)}>
                      {expandedSteps.has(i) ? '收起' : '展开全部'}
                    </button>
                  {/if}
                </div>
              </div>
            {:else if step.type === 'reasoning'}
              <div class="flex gap-2 items-start">
                <div class="step-icon think">🧠</div>
                <div class="step-content flex-1 min-w-0">
                  <div class="step-label think">推理过程</div>
                  <div class="text-[11px] mt-0.5 step-result-content {expandedSteps.has(i) ? 'expanded' : ''}" style="color: var(--color-text-secondary);">{step.content}</div>
                  {#if step.content && (step.content.split('\n').length > 3 || step.content.length > 200)}
                    <button class="text-[9px] mt-0.5 hover:underline" style="color: #3b82f6;" onclick={() => toggleStep(i)}>
                      {expandedSteps.has(i) ? '收起' : '展开'}
                    </button>
                  {/if}
                </div>
              </div>
            {/if}
          </div>
        {/each}
        {#if hasActiveThinking}
          <div class="thinking-indicator flex items-center gap-2 px-2 py-1 rounded-lg bg-primary-500/5">
            <div class="thinking-spinner"></div>
            <span class="text-[11px] text-primary-500">思考中...</span>
          </div>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .thinking-dots {
    display: inline-flex;
    gap: 1px;
    margin-left: 2px;
  }
  .thinking-dots .dot {
    animation: blink 1.4s infinite both;
    font-weight: bold;
  }
  .thinking-dots .dot:nth-child(2) { animation-delay: 0.2s; }
  .thinking-dots .dot:nth-child(3) { animation-delay: 0.4s; }

  @keyframes blink {
    0%, 80%, 100% { opacity: 0; }
    40% { opacity: 1; }
  }

  .thinking-spinner {
    width: 12px;
    height: 12px;
    border: 2px solid var(--color-primary);
    border-top-color: transparent;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>

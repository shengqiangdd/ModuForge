<script lang="ts">
  import ToolResult from './ToolResult.svelte';

  type AgentStepInput = {
    path?: string;
    pattern?: string;
    command?: string;
    old_text?: string;
    content?: string;
    files?: string[];
    project_id?: string;
    [key: string]: unknown;
  };

  type AgentStep = {
    type: 'think' | 'skill_call' | 'skill_result' | 'answer' | 'reasoning';
    skill?: string;
    input?: AgentStepInput;
    content?: string;
    round?: number;
  };

  let {
    steps = [],
    expandedSteps = new Set<number>(),
    onToggleStep,
    streaming = false,
  }: {
    steps: AgentStep[];
    expandedSteps?: Set<number>;
    onToggleStep?: (index: number) => void;
    streaming?: boolean; // true while the assistant is still producing steps
  } = $props();

  let collapsed = $state(true);

  // Count visible steps (skip short think steps)
  let visibleCount = $derived(
    steps.filter(s => !(s.type === 'think' && s.content && s.content.length < 30)).length
  );

  // Live "currently running" tool: the last step is an unclosed skill_call
  // (no following skill_result for it) while the round is still streaming.
  let activeTool = $derived.by(() => {
    if (!streaming || steps.length === 0) return null;
    const last = steps[steps.length - 1];
    if (last.type === 'skill_call' && last.skill) return last.skill;
    // skill_call followed immediately by a thin skill_result has no live state;
    // but if the tail is a skill_result we are waiting for the next call.
    return null;
  });

  function toggleStep(index: number) {
    onToggleStep?.(index);
  }

  function buildParamSummary(input: AgentStepInput): string {
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
  <div class="mt-2 rounded-xl overflow-hidden border border-[var(--color-border)]">
    <!-- Collapse toggle header -->
    <button
      class="flex items-center gap-1.5 w-full px-3 py-1.5 text-xs font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors"
      onclick={() => collapsed = !collapsed}
    >
      <span class="material-symbols-outlined text-[14px]">robot_2</span>
      Agent 步骤
      <span class="text-[10px] text-[var(--color-text-muted)]">{visibleCount} 步</span>
      {#if activeTool}
        <span class="flex items-center gap-1 px-1.5 py-0.5 rounded-full text-[10px] font-medium" style="background: rgba(255,193,7,0.15); color: #fbbf24;">
          <span class="live-dot"></span>
          <span class="font-mono truncate max-w-[120px]">{activeTool}</span>
        </span>
      {/if}
      <span class="material-symbols-outlined text-[14px] ml-auto transition-transform {collapsed ? '' : 'rotate-180'}">expand_more</span>
    </button>

    {#if !collapsed}
      <div class="px-2 py-1.5 space-y-1" style="background: var(--color-surface); border-top: 1px solid var(--color-border);">
        {#each steps as step, i}
          {#if !(step.type === 'think' && step.content && step.content.length < 30)}
            <div class="step-card" style="animation: fadeInUp 0.2s ease-out both; animation-delay: {i * 0.02}s">
              {#if step.type === 'think'}
                <div class="flex gap-1.5 items-start">
                  <span class="flex-shrink-0 text-[12px]">🧠</span>
                  <div class="flex-1 min-w-0">
                    <div class="text-[10px] font-medium text-[var(--color-text-secondary)]">思考</div>
                    <div class="text-[11px] mt-0.5 step-result-content {expandedSteps.has(i) ? 'expanded' : ''}" style="color: var(--color-text-muted);">{step.content}</div>
                    {#if step.content && (step.content.split('\n').length > 3 || step.content.length > 200)}
                      <button class="text-[9px] mt-0.5 hover:underline" style="color: var(--color-primary);" onclick={() => toggleStep(i)}>
                        {expandedSteps.has(i) ? '收起' : '展开'}
                      </button>
                    {/if}
                  </div>
                </div>
              {:else if step.type === 'skill_call'}
                <div class="flex gap-1.5 items-start {activeTool === step.skill && i === steps.length - 1 ? 'step-live' : ''}">
                  <span class="flex-shrink-0 text-[12px]">{toolIcon(step.skill || '')}</span>
                  <div class="flex-1 min-w-0">
                    <div class="text-[10px] font-medium">
                      <span class="font-mono">{step.skill}</span>
                      {#if activeTool === step.skill && i === steps.length - 1}
                        <span class="ml-1 align-middle inline-block w-2 h-2 rounded-full" style="background:#fbbf24; animation: livePulse 1s ease-in-out infinite;"></span>
                      {/if}
                      {#if step.input}
                        {@const summary = buildParamSummary(step.input)}
                        {#if summary}
                          <span class="text-[10px] text-[var(--color-text-muted)] font-normal ml-1">{summary}</span>
                        {/if}
                      {/if}
                    </div>
                    {#if step.input}
                      {@const inputStr = JSON.stringify(step.input, null, 2)}
                      <pre class="text-[10px] mt-0.5 font-mono step-result-content {expandedSteps.has(i) ? 'expanded' : ''}" style="color: var(--color-text-muted);">{inputStr}</pre>
                      {#if inputStr.length > 150}
                        <button class="text-[9px] mt-0.5 hover:underline" style="color: #d97706;" onclick={() => toggleStep(i)}>
                          {expandedSteps.has(i) ? '收起' : '展开参数'}
                        </button>
                      {/if}
                    {/if}
                  </div>
                </div>
              {:else if step.type === 'skill_result'}
                <div class="flex gap-1.5 items-start">
                  <span class="flex-shrink-0 text-[12px]">📋</span>
                  <div class="flex-1 min-w-0">
                    <div class="text-[10px] font-medium text-[var(--color-text-secondary)]">结果: {step.skill}</div>
                    <ToolResult
                      skillName={step.skill || ''}
                      content={step.content || ''}
                      expanded={expandedSteps.has(i)}
                      onToggle={() => toggleStep(i)}
                    />
                  </div>
                </div>
              {:else if step.type === 'answer'}
                <div class="flex gap-1.5 items-start">
                  <span class="flex-shrink-0 text-[12px]">✅</span>
                  <div class="flex-1 min-w-0">
                    <div class="text-[10px] font-medium text-[var(--color-text-secondary)]">完成</div>
                    <div class="text-[11px] mt-0.5 step-result-content {expandedSteps.has(i) ? 'expanded' : ''}" style="color: var(--color-text);">{step.content}</div>
                    {#if step.content && (step.content.split('\n').length > 5 || step.content.length > 300)}
                      <button class="text-[9px] mt-0.5 hover:underline" style="color: var(--color-primary);" onclick={() => toggleStep(i)}>
                        {expandedSteps.has(i) ? '收起' : '展开全部'}
                      </button>
                    {/if}
                  </div>
                </div>
              {:else if step.type === 'reasoning'}
                <div class="flex gap-1.5 items-start">
                  <span class="flex-shrink-0 text-[12px]">🧠</span>
                  <div class="flex-1 min-w-0">
                    <div class="text-[10px] font-medium text-[var(--color-text-secondary)]">推理过程</div>
                    <div class="text-[11px] mt-0.5 step-result-content {expandedSteps.has(i) ? 'expanded' : ''}" style="color: var(--color-text-muted);">{step.content}</div>
                    {#if step.content && (step.content.split('\n').length > 3 || step.content.length > 200)}
                      <button class="text-[9px] mt-0.5 hover:underline" style="color: var(--color-primary);" onclick={() => toggleStep(i)}>
                        {expandedSteps.has(i) ? '收起' : '展开'}
                      </button>
                    {/if}
                  </div>
                </div>
              {/if}
            </div>
          {/if}
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  .step-card {
    padding: 4px 6px;
    border-radius: 6px;
    transition: background 0.15s;
  }
  .step-card:hover {
    background: var(--color-bg-hover, rgba(0,0,0,0.03));
  }
  .step-live {
    background: rgba(255,193,7,0.08);
    border: 1px solid rgba(255,193,7,0.25);
    border-radius: 6px;
    padding: 4px 6px;
  }
  .live-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #fbbf24;
    animation: livePulse 1s ease-in-out infinite;
  }
  @keyframes livePulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.4; transform: scale(0.7); }
  }
  .step-result-content {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    word-break: break-all;
  }
  .step-result-content.expanded {
    -webkit-line-clamp: unset;
    overflow: visible;
  }

  @keyframes fadeInUp {
    from { opacity: 0; transform: translateY(4px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
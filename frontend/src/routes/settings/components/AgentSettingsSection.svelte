<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let agentMaxIterations = $state(50);
  let agentMaxResultLen = $state(32768);
  let savingAgentConfig = $state(false);

  async function loadConfig() {
    const token = getToken();
    try {
      const r = await fetch('/api/v1/settings/agent', { headers: { Authorization: `Bearer ${token}` } });
      if (r.ok) {
        const data = await r.json();
        agentMaxIterations = parseInt(data.max_iterations) || 50;
        agentMaxResultLen = parseInt(data.max_result_len) || 32768;
      }
    } catch (e) { console.error('Failed to load agent config:', e); }
  }

  async function saveAgentConfig() {
    savingAgentConfig = true;
    const token = getToken();
    try {
      const r = await fetch('/api/v1/settings/agent', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          max_iterations: agentMaxIterations,
          max_result_len: agentMaxResultLen,
        }),
      });
      if (r.ok) {
        toast('Agent 配置已保存', 'success');
      } else {
        const data = await r.json();
        toast(data.error || '保存失败', 'error');
      }
    } catch {
      toast('保存失败', 'error');
    }
    savingAgentConfig = false;
  }

  onMount(() => {
    loadConfig();
  });
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: linear-gradient(135deg, rgba(249,115,22,0.15), rgba(239,68,68,0.15))">
      <span class="material-symbols-outlined text-[18px]" style="color: #f97316">smart_toy</span>
    </div>
    <div>
      <h2 class="text-base font-semibold text-[var(--color-text)]">Agent 配置</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">调整 Agent 行为参数</p>
    </div>
  </div>
  <div class="space-y-4">
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <label for="agent-max-iterations" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">最大迭代次数</label>
        <input id="agent-max-iterations" type="number" class="input-field" min="1" max="100" bind:value={agentMaxIterations} />
        <p class="text-xs mt-1" style="color: var(--color-text-muted)">Agent 单次任务最多执行步骤数（1-100）</p>
      </div>
      <div>
        <label for="agent-max-result-len" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">技能结果最大长度</label>
        <input id="agent-max-result-len" type="number" class="input-field" min="500" max="100000" step="1000" bind:value={agentMaxResultLen} />
        <p class="text-xs mt-1" style="color: var(--color-text-muted)">单次技能返回内容最大字符数（500-100000）</p>
      </div>
    </div>
    <button type="button" class="auth-submit px-6 py-2.5 rounded-xl font-semibold text-sm text-white disabled:opacity-50" onclick={saveAgentConfig} disabled={savingAgentConfig}>
      {savingAgentConfig ? '保存中...' : '保存 Agent 配置'}
    </button>
  </div>
</section>

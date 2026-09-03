<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  let agentMaxIterations = $state(50);
  let agentMaxResultLen = $state(32768);
  let savingAgentConfig = $state(false);

  // 工具执行策略相关
  let globalToolPolicy = $state('confirm');
  let toolPolicies = $state<any[]>([]);
  let savingToolPolicy = $state(false);
  let showAddPolicy = $state(false);
  let newPolicyServer = $state('');
  let newPolicyTool = $state('');
  let newPolicyMode = $state('confirm');

  const policyModes = [
    { value: 'confirm', label: '需要确认', description: '执行前需要用户确认', icon: 'help', color: '#f97316' },
    { value: 'allow', label: '自动允许', description: '自动执行，无需确认', icon: 'check_circle', color: '#22c55e' },
    { value: 'deny', label: '拒绝', description: '完全禁止执行此工具', icon: 'block', color: '#ef4444' },
  ];

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
    
    // 加载工具策略
    await loadToolPolicies();
  }

  async function loadToolPolicies() {
    const token = getToken();
    try {
      // 加载全局策略
      const globalR = await fetch('/api/v1/tool-policies/global', { headers: { Authorization: `Bearer ${token}` } });
      if (globalR.ok) {
        const data = await globalR.json();
        globalToolPolicy = data.mode || 'confirm';
      }
      
      // 加载所有工具策略
      const policiesR = await fetch('/api/v1/tool-policies', { headers: { Authorization: `Bearer ${token}` } });
      if (policiesR.ok) {
        toolPolicies = await policiesR.json();
      }
    } catch (e) { console.error('Failed to load tool policies:', e); }
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

  async function saveGlobalToolPolicy() {
    savingToolPolicy = true;
    const token = getToken();
    try {
      const r = await fetch('/api/v1/tool-policies/global', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ mode: globalToolPolicy }),
      });
      if (r.ok) {
        toast('全局工具策略已保存', 'success');
      } else {
        toast('保存失败', 'error');
      }
    } catch {
      toast('保存失败', 'error');
    }
    savingToolPolicy = false;
  }

  async function addToolPolicy() {
    if (!newPolicyServer || !newPolicyTool) {
      toast('请填写 Server 和 Tool 名称', 'error');
      return;
    }
    const token = getToken();
    try {
      const r = await fetch('/api/v1/tool-policies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          server: newPolicyServer,
          tool: newPolicyTool,
          mode: newPolicyMode,
        }),
      });
      if (r.ok) {
        toast('策略已添加', 'success');
        showAddPolicy = false;
        newPolicyServer = '';
        newPolicyTool = '';
        newPolicyMode = 'confirm';
        await loadToolPolicies();
      } else {
        const data = await r.json();
        toast(data.error || '添加失败', 'error');
      }
    } catch {
      toast('添加失败', 'error');
    }
  }

  async function deleteToolPolicy(server: string, tool: string) {
    const token = getToken();
    try {
      const r = await fetch(`/api/v1/tool-policies/${encodeURIComponent(server)}/${encodeURIComponent(tool)}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (r.ok) {
        toast('策略已删除', 'success');
        await loadToolPolicies();
      }
    } catch {
      toast('删除失败', 'error');
    }
  }

  onMount(() => {
    loadConfig();
  });
</script>

<!-- Agent 基础配置 -->
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

<!-- 工具执行策略 -->
<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: linear-gradient(135deg, rgba(34,197,94,0.15), rgba(16,185,129,0.15))">
      <span class="material-symbols-outlined text-[18px]" style="color: #22c55e">build</span>
    </div>
    <div>
      <h2 class="text-base font-semibold text-[var(--color-text)]">工具执行策略</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">配置 AI 工具的执行权限</p>
    </div>
  </div>

  <!-- 全局策略 -->
  <div class="mb-6">
    <h3 class="text-sm font-semibold mb-3" style="color: var(--color-text)">默认策略</h3>
    <p class="text-xs mb-3" style="color: var(--color-text-muted)">未配置的工具将使用此默认策略</p>
    <div class="grid grid-cols-3 gap-3">
      {#each policyModes as mode}
        <button
          type="button"
          class="p-4 rounded-xl border-2 text-left transition-all {globalToolPolicy === mode.value ? 'ring-2' : ''}"
          style="border-color: {globalToolPolicy === mode.value ? mode.color : 'var(--color-border)'}; background: {globalToolPolicy === mode.value ? mode.color + '15' : 'transparent'}"
          onclick={() => globalToolPolicy = mode.value}
        >
          <span class="material-symbols-outlined text-[20px]" style="color: {mode.color}">{mode.icon}</span>
          <div class="font-semibold text-sm mt-1" style="color: var(--color-text)">{mode.label}</div>
          <div class="text-xs mt-0.5" style="color: var(--color-text-muted)">{mode.description}</div>
        </button>
      {/each}
    </div>
    <button type="button" class="auth-submit px-6 py-2.5 rounded-xl font-semibold text-sm text-white disabled:opacity-50 mt-4" onclick={saveGlobalToolPolicy} disabled={savingToolPolicy}>
      {savingToolPolicy ? '保存中...' : '保存默认策略'}
    </button>
  </div>

  <!-- 工具策略列表 -->
  <div>
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-sm font-semibold" style="color: var(--color-text)">工具策略列表</h3>
      <button type="button" class="text-sm font-medium flex items-center gap-1" style="color: var(--color-primary)" onclick={() => showAddPolicy = !showAddPolicy}>
        <span class="material-symbols-outlined text-[16px]">{showAddPolicy ? 'close' : 'add'}</span>
        {showAddPolicy ? '取消' : '添加策略'}
      </button>
    </div>

    {#if showAddPolicy}
      <div class="p-4 rounded-xl mb-4" style="background: var(--color-bg-elevated, rgba(127,127,127,0.07))">
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div>
            <label class="block text-xs font-medium mb-1" style="color: var(--color-text-secondary)">Server 名称</label>
            <input type="text" class="input-field text-sm" placeholder="如: github, filesystem" bind:value={newPolicyServer} />
          </div>
          <div>
            <label class="block text-xs font-medium mb-1" style="color: var(--color-text-secondary)">Tool 名称</label>
            <input type="text" class="input-field text-sm" placeholder="如: push_files" bind:value={newPolicyTool} />
          </div>
          <div>
            <label class="block text-xs font-medium mb-1" style="color: var(--color-text-secondary)">执行策略</label>
            <select class="input-field text-sm" bind:value={newPolicyMode}>
              {#each policyModes as mode}
                <option value={mode.value}>{mode.label}</option>
              {/each}
            </select>
          </div>
        </div>
        <button type="button" class="auth-submit px-4 py-2 rounded-lg font-medium text-sm text-white mt-3" onclick={addToolPolicy}>
          添加
        </button>
      </div>
    {/if}

    {#if toolPolicies.length === 0}
      <p class="text-sm py-4 text-center" style="color: var(--color-text-muted)">暂无自定义工具策略，将使用默认策略</p>
    {:else}
      <div class="space-y-2">
        {#each toolPolicies as policy}
          <div class="flex items-center justify-between p-3 rounded-xl" style="background: var(--color-bg-elevated, rgba(127,127,127,0.07))">
            <div class="flex items-center gap-3">
              <span class="material-symbols-outlined text-[18px]" style="color: {policyModes.find(m => m.value === policy.mode)?.color || '#888'}">
                {policyModes.find(m => m.value === policy.mode)?.icon || 'help'}
              </span>
              <div>
                <span class="font-mono font-semibold text-sm" style="color: var(--color-text)">{policy.tool}</span>
                <span class="text-xs ml-2" style="color: var(--color-text-muted)">@ {policy.server}</span>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-xs px-2 py-1 rounded" style="background: {policyModes.find(m => m.value === policy.mode)?.color || '#888'}20; color: {policyModes.find(m => m.value === policy.mode)?.color || '#888'}">
                {policyModes.find(m => m.value === policy.mode)?.label || policy.mode}
              </span>
              <button type="button" class="btn-ghost p-1" onclick={() => deleteToolPolicy(policy.server, policy.tool)} title="删除">
                <span class="material-symbols-outlined text-[16px]" style="color: var(--color-error)">delete</span>
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</section>

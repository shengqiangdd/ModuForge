<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { getToken } from '$lib/api/client';

  interface SkillInfo {
    id: number;
    name: string;
    description: string;
    prompt: string;
    input_schema: string;
    is_public: boolean;
    is_builtin?: boolean;
    icon?: string;
    category?: string;
  }

  interface EvolutionData {
    versions: { version: number; timestamp: string; changes: string }[];
    score: number;
    metrics: Record<string, number>;
    stats?: Record<string, unknown>;
    success_rate?: string;
    avg_duration?: string;
  }

  let customSkills: SkillInfo[] = $state([]);
  let builtinSkills: SkillInfo[] = $state([]);
  let allSkills = $derived([...builtinSkills, ...customSkills]);
  let showAllSkills = $state(false);
  let expandedSkillId = $state<number | null>(null);
  let showSkillModal = $state(false);
  let editingSkill: SkillInfo | null = $state(null);
  let skillForm = $state({ name: '', description: '', prompt: '', input_schema: '{}', is_public: false });
  let loadingSkills = $state(false);
  let testingSkillId = $state<number | null>(null);
  let testSkillId = $state<number | null>(null);
  let testInput = $state('');
  let testResult = $state('');
  let showTestInput = $state(false);

  let skillEvolutionData = $state<Record<number, EvolutionData>>({});
  let loadingEvolution = $state<Set<number>>(new Set());
  let showEvolution = $state<number | null>(null);

  async function loadCustomSkills() {
    loadingSkills = true;
    try {
      const resBuiltin = await fetch('/api/v1/agent/skills', {
        headers: { 'Authorization': `Bearer ${getToken()}` },
      });
      if (resBuiltin.ok) {
        const data = await resBuiltin.json();
        builtinSkills = (data.skills || []).map((s: any) => ({ ...s, id: `builtin_${s.name}`, is_builtin: true }));
      }
      const res = await fetch('/api/v1/agent/custom-skills', {
        headers: { 'Authorization': `Bearer ${getToken()}` },
      });
      if (res.ok) {
        const data = await res.json();
        customSkills = data.skills || [];
      }
    } catch {}
    loadingSkills = false;
  }

  function openNewSkill() {
    editingSkill = null;
    skillForm = { name: '', description: '', prompt: '', input_schema: '{}', is_public: false };
    testInput = '';
    testResult = '';
    showSkillModal = true;
  }

  function openEditSkill(s: any) {
    editingSkill = s;
    skillForm = { name: s.name, description: s.description, prompt: s.prompt, input_schema: s.input_schema || '{}', is_public: s.is_public };
    testInput = '';
    testResult = '';
    showSkillModal = true;
  }

  async function saveSkill() {
    const token = getToken();
    const isEdit = !!editingSkill;
    const url = isEdit ? `/api/v1/agent/custom-skills/${editingSkill!.id}` : '/api/v1/agent/custom-skills';
    const method = isEdit ? 'PUT' : 'POST';
    try {
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify(skillForm),
      });
      if (res.ok) {
        toast(isEdit ? '技能已更新' : '技能已创建', 'success');
        showSkillModal = false;
        await loadCustomSkills();
      } else {
        toast((await res.json()).error || '保存失败', 'error');
      }
    } catch { toast('保存失败', 'error'); }
  }

  async function deleteSkill(id: number) {
    if (!confirm('确定删除此技能？')) return;
    try {
      const res = await fetch(`/api/v1/agent/custom-skills/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      if (res.ok) {
        customSkills = customSkills.filter((s: any) => s.id !== id);
        toast('已删除', 'success');
      } else {
        toast((await res.json()).error || '删除失败', 'error');
      }
    } catch { toast('删除失败', 'error'); }
  }

  async function testSkill(id: number) {
    if (!testInput) return;
    testingSkillId = id;
    testResult = '';
    try {
      const res = await fetch(`/api/v1/agent/custom-skills/${id}/execute`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({ input: testInput }),
      });
      if (res.ok) {
        testResult = await res.text();
      } else {
        testResult = (await res.json()).error || '测试失败';
      }
    } catch (e: any) { testResult = e.message || '测试失败'; }
    testingSkillId = null;
  }

  async function loadSkillEvolution(skillId: number) {
    if (loadingEvolution.has(skillId)) return;
    loadingEvolution = new Set(loadingEvolution).add(skillId);
    try {
      const [evoRes, optRes] = await Promise.all([
        fetch(`/api/v1/agent/custom-skills/${skillId}/evolution`, {
          headers: { Authorization: `Bearer ${getToken()}` },
        }),
        fetch(`/api/v1/agent/custom-skills/${skillId}/optimize`, {
          headers: { Authorization: `Bearer ${getToken()}` },
        }),
      ]);
      const evo = evoRes.ok ? await evoRes.json() : {};
      const opt = optRes.ok ? await optRes.json() : {};
      skillEvolutionData = { ...skillEvolutionData, [skillId]: { ...evo, ...opt } };
    } catch {}
    loadingEvolution = new Set([...loadingEvolution].filter(x => x !== skillId));
  }

  onMount(() => {
    loadCustomSkills();
  });
</script>

<section class="card p-6">
  <div class="flex items-center gap-3 mb-5">
    <div class="w-9 h-9 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-primary)">smart_toy</span>
    </div>
    <div class="flex-1">
      <h2 class="text-base font-semibold text-[var(--color-text)]">AI 技能</h2>
      <p class="text-xs" style="color: var(--color-text-muted)">共 {allSkills.length} 个技能</p>
    </div>
    <button class="btn-primary text-sm" onclick={openNewSkill}>
      <span class="material-symbols-outlined text-[16px]">add</span>
      创建
    </button>
  </div>
  {#if loadingSkills}
    <div class="space-y-2">
      {#each Array(3) as _}
        <div class="skeleton h-12 w-full rounded-xl"></div>
      {/each}
    </div>
  {:else if allSkills.length === 0}
    <p class="text-sm text-center py-6" style="color: var(--color-text-muted)">暂无技能</p>
  {:else}
    <div class="space-y-1.5">
      {#each (showAllSkills ? allSkills : allSkills.slice(0, 6)) as skill}
        <div class="rounded-xl overflow-hidden" style="border: 1px solid var(--color-border);">
          <div
            role="button"
            tabindex="0"
            class="w-full flex items-center gap-2.5 p-2.5 text-left hover:bg-[var(--color-surface-secondary)] transition-colors cursor-pointer"
            onclick={() => expandedSkillId = expandedSkillId === skill.id ? null : skill.id}
            onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); expandedSkillId = expandedSkillId === skill.id ? null : skill.id; } }}
          >
            <span class="material-symbols-outlined text-[16px] text-[var(--color-text-muted)]">
              {expandedSkillId === skill.id ? 'expand_more' : 'chevron_right'}
            </span>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-[var(--color-text)]">{skill.name}</span>
                {#if skill.is_builtin}
                  <span class="text-[10px] px-1.5 py-0.5 rounded" style="background: var(--color-primary-light); color: var(--color-primary)">内置</span>
                {:else if skill.is_public}
                  <span class="text-[10px] px-1.5 py-0.5 rounded" style="background: var(--color-success-light); color: var(--color-success)">公开</span>
                {/if}
              </div>
              <p class="text-xs truncate max-w-md" style="color: var(--color-text-muted)">{skill.description}</p>
            </div>
            {#if !skill.is_builtin}
              <div class="flex items-center gap-1">
                <button class="text-[10px] px-2 py-1 rounded hover:bg-[var(--color-surface)]" style="color: var(--color-text-muted)" onclick={(e) => { e.stopPropagation(); openEditSkill(skill); }}>编辑</button>
                <button class="text-[10px] px-2 py-1 rounded hover:bg-[var(--color-error-light)]" style="color: var(--color-error)" onclick={(e) => { e.stopPropagation(); deleteSkill(skill.id); }}>删除</button>
              </div>
            {/if}
          </div>
          {#if expandedSkillId === skill.id}
            <div class="px-3 pb-3 pt-1" style="border-top: 1px solid var(--color-border)">
              {#if skill.is_builtin}
                <div class="text-[10px] font-medium mb-1" style="color: var(--color-text-muted)">功能说明</div>
                <div class="text-xs p-2.5 rounded-lg whitespace-pre-wrap" style="background: var(--color-bg); color: var(--color-text-secondary); line-height: 1.6">{skill.description}</div>
              {:else}
                <div class="text-[10px] font-medium mb-1" style="color: var(--color-text-muted)">PROMPT</div>
                <pre class="text-xs p-2.5 rounded-lg overflow-x-auto max-h-48 overflow-y-auto whitespace-pre-wrap" style="background: var(--color-bg); color: var(--color-text-secondary); font-family: 'JetBrains Mono', monospace; font-size: 11px; line-height: 1.5">{skill.prompt || '无提示词'}</pre>
                {#if skill.input_schema && skill.input_schema !== '{}'}
                  <div class="text-[10px] font-medium mt-2 mb-1" style="color: var(--color-text-muted)">INPUT SCHEMA</div>
                  <pre class="text-xs p-2 rounded-lg overflow-x-auto" style="background: var(--color-bg); color: var(--color-text-secondary); font-family: 'JetBrains Mono', monospace; font-size: 10px">{typeof skill.input_schema === 'string' ? skill.input_schema : JSON.stringify(skill.input_schema, null, 2)}</pre>
                {/if}
                <div class="flex items-center gap-2 mt-2">
                  <button class="btn-ghost text-[11px] px-2.5 py-1" onclick={() => { loadSkillEvolution(skill.id); showEvolution = showEvolution === skill.id ? null : skill.id; }}>
                    📊 进化数据
                  </button>
                  <button class="btn-ghost text-[11px] px-2.5 py-1" onclick={() => { testSkillId = skill.id; testResult = ''; showTestInput = true; }}>
                    🧪 测试
                  </button>
                </div>
                {#if showEvolution === skill.id && skillEvolutionData[skill.id]}
                  {@const evo = skillEvolutionData[skill.id]}
                  {@const stats = evo.stats || {}}
                  <div class="grid grid-cols-1 sm:grid-cols-3 gap-2 mt-2">
                    <div class="text-center p-1.5 rounded-lg" style="background: var(--color-bg)">
                      <div class="text-sm font-bold" style="color: var(--color-text)">{stats.total_runs || 0}</div>
                      <div class="text-[9px]" style="color: var(--color-text-muted)">运行次数</div>
                    </div>
                    <div class="text-center p-1.5 rounded-lg" style="background: var(--color-bg)">
                      <div class="text-sm font-bold" style="color: {evo.success_rate && parseFloat(evo.success_rate) >= 80 ? 'var(--color-success)' : 'var(--color-warning)'}">
                        {evo.success_rate || '0%'}
                      </div>
                      <div class="text-[9px]" style="color: var(--color-text-muted)">成功率</div>
                    </div>
                    <div class="text-center p-1.5 rounded-lg" style="background: var(--color-bg)">
                      <div class="text-sm font-bold" style="color: var(--color-text)">{evo.avg_duration || '0ms'}</div>
                      <div class="text-[9px]" style="color: var(--color-text-muted)">平均耗时</div>
                    </div>
                  </div>
                {/if}
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
    {#if allSkills.length > 6}
      <button
        type="button"
        class="w-full mt-3 py-2 text-xs font-medium rounded-lg transition-colors"
        style="color: var(--color-primary); background: var(--color-primary-light)"
        onclick={() => showAllSkills = !showAllSkills}
      >
        {showAllSkills ? '收起' : `显示全部 (${allSkills.length})`}
      </button>
    {/if}
  {/if}
</section>

<!-- Custom Skill Modal -->
{#if showSkillModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showSkillModal = false; }} onkeydown={(e) => { if (e.key === 'Escape') showSkillModal = false; }}>
    <div class="card p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto" role="dialog" aria-modal="true" tabindex="-1">
      <div class="flex items-center gap-3 mb-5">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center" style="background: var(--color-primary-light)">
          <span class="material-symbols-outlined text-[16px]" style="color: var(--color-primary)">smart_toy</span>
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--color-text)]">{editingSkill ? '编辑' : '创建'}自定义技能</h3>
          <p class="text-xs text-[var(--color-text-muted)]">定义 AI 技能模板</p>
        </div>
      </div>
      <div class="space-y-4">
        <div>
          <label for="skill-name" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">名称</label>
          <input id="skill-name" type="text" class="input-field" bind:value={skillForm.name} placeholder="技能名称" />
        </div>
        <div>
          <label for="skill-description" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">描述</label>
          <input id="skill-description" type="text" class="input-field" bind:value={skillForm.description} placeholder="简要描述技能功能" />
        </div>
        <div>
          <label for="skill-prompt" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">提示词模板</label>
          <textarea id="skill-prompt" class="input-field resize-none" rows="5" bind:value={skillForm.prompt} placeholder="使用 {'{input}'} 表示用户输入位置"></textarea>
        </div>
        <div>
          <label for="skill-schema" class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">输入 Schema (JSON)</label>
            <textarea id="skill-schema" class="input-field resize-none font-mono text-xs" rows="3" bind:value={skillForm.input_schema} placeholder={`{"type": "object", "properties": {"input": {"type": "string"}}}`}></textarea>
        </div>
        <div class="flex items-center gap-2">
          <input type="checkbox" id="skill-public" bind:checked={skillForm.is_public} />
          <label for="skill-public" class="text-sm text-[var(--color-text)]">公开（其他用户可查看和使用）</label>
        </div>
        {#if editingSkill}
          <div class="border-t pt-4" style="border-color: var(--color-border);">
            <span class="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">测试运行</span>
            <div class="flex gap-2 mb-2">
              <input type="text" class="input-field flex-1" bind:value={testInput} placeholder="输入测试内容" />
              <button class="btn-primary text-sm px-3 py-1.5" onclick={() => editingSkill && testSkill(editingSkill.id)} disabled={!editingSkill || testingSkillId === editingSkill?.id || !testInput}>
                {testingSkillId === editingSkill?.id ? '运行中...' : '运行'}
              </button>
            </div>
            {#if testResult}
              <pre class="p-3 rounded-xl text-xs font-mono max-h-40 overflow-y-auto" style="background: var(--color-bg); color: var(--color-text); border: 1px solid var(--color-border);">{testResult}</pre>
            {/if}
          </div>
        {/if}
      </div>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button class="btn-ghost text-sm" onclick={() => showSkillModal = false}>取消</button>
        <button class="btn-primary text-sm" onclick={saveSkill} disabled={!skillForm.name || !skillForm.prompt}>
          保存
        </button>
      </div>
    </div>
  </div>
{/if}

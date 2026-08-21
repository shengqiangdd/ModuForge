<script lang="ts">
  import { onMount } from 'svelte';

  interface Tag {
    id: number;
    name: string;
    color: string;
    project_count?: number;
  }

  interface Project {
    id: number;
    name: string;
    slug: string;
    description?: string;
    created_at: string;
  }

  let tags = $state<Tag[]>([]);
  let loading = $state(true);
  let errorMsg = $state('');
  let successMsg = $state('');
  let showForm = $state(false);
  let editingTag = $state<Tag | null>(null);
  let formName = $state('');
  let formColor = $state('#3b82f6');
  let saving = $state(false);
  let selectedTag = $state<Tag | null>(null);
  let tagProjects = $state<Project[]>([]);
  let loadingProjects = $state(false);
  let filterTag = $state('');

  const presetColors = ['#3b82f6', '#ef4444', '#f97316', '#eab308', '#22c55e', '#8b5cf6', '#ec4899', '#06b6d4'];

  function getToken() { return localStorage.getItem('moduforge_token') || ''; }

  function msg(err: string, ok?: string) {
    errorMsg = err; successMsg = ok || '';
    setTimeout(() => { errorMsg = ''; successMsg = ''; }, 4000);
  }

  async function loadTags() {
    loading = true;
    try {
      const r = await fetch('/api/v1/tags', { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { const d = await r.json(); tags = d.tags || d || []; }
    } catch {}
    loading = false;
  }

  async function saveTag() {
    if (!formName.trim()) { msg('标签名不能为空'); return; }
    saving = true;
    try {
      const url = editingTag ? `/api/v1/tags/${editingTag.id}` : '/api/v1/tags';
      const method = editingTag ? 'PUT' : 'POST';
      const r = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({ name: formName.trim(), color: formColor }),
      });
      if (r.ok) {
        msg('', editingTag ? '标签已更新' : '标签已创建');
        showForm = false;
        editingTag = null;
        formName = '';
        formColor = '#3b82f6';
        loadTags();
      } else {
        const d = await r.json().catch(() => ({}));
        msg(d.error || '操作失败');
      }
    } catch (e: any) { msg(e?.message || '操作失败'); }
    saving = false;
  }

  async function deleteTag(tag: Tag) {
    if (!confirm(`确定删除标签 "${tag.name}"？`)) return;
    try {
      const r = await fetch(`/api/v1/tags/${tag.id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) {
        msg('', '标签已删除');
        if (selectedTag?.id === tag.id) { selectedTag = null; tagProjects = []; }
        loadTags();
      }
    } catch (e: any) { msg(e?.message || '删除失败'); }
  }

  async function viewProjects(tag: Tag) {
    selectedTag = tag;
    loadingProjects = true;
    try {
      const r = await fetch(`/api/v1/tags/${tag.id}/projects`, { headers: { Authorization: `Bearer ${getToken()}` } });
      if (r.ok) { const d = await r.json(); tagProjects = d.projects || d || []; }
    } catch {}
    loadingProjects = false;
  }

  function startEdit(tag: Tag) {
    editingTag = tag;
    formName = tag.name;
    formColor = tag.color || '#3b82f6';
    showForm = true;
  }

  function openCreate() {
    editingTag = null;
    formName = '';
    formColor = '#3b82f6';
    showForm = true;
  }

  const filteredTags = $derived(
    tags.filter(t => !filterTag || t.name.toLowerCase().includes(filterTag.toLowerCase()))
  );

  onMount(loadTags);
</script>

<div class="w-full p-6 max-w-5xl mx-auto space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold text-[var(--color-text)]">标签管理</h1>
      <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">项目标签的增删改查</p>
    </div>
    <div class="flex gap-2">
      <button class="px-3 py-1.5 rounded-lg text-sm flex items-center gap-1.5" style="background: var(--color-surface); color: var(--color-text-secondary); border: 1px solid var(--color-border)" onclick={loadTags}>
        <span class="material-symbols-outlined text-[16px]">refresh</span>
        刷新
      </button>
      <button class="px-3 py-1.5 rounded-lg text-sm font-medium" style="background: var(--color-primary); color: white" onclick={openCreate}>
        <span class="material-symbols-outlined text-[16px] align-middle">add</span>
        新建标签
      </button>
    </div>
  </div>

  {#if errorMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-error-light); color: var(--color-error)">{errorMsg}</div>
  {/if}
  {#if successMsg}
    <div class="px-4 py-3 rounded-xl text-sm" style="background: var(--color-success-light); color: var(--color-success)">{successMsg}</div>
  {/if}

  <!-- Form -->
  {#if showForm}
    <div class="card p-5 space-y-3">
      <h3 class="text-sm font-semibold text-[var(--color-text)]">{editingTag ? '编辑标签' : '新建标签'}</h3>
      <input class="input-field w-full" placeholder="标签名称" bind:value={formName} />
      <div class="flex gap-2 flex-wrap">
        {#each presetColors as c}
          <button
            class="w-6 h-6 rounded-full border-2 transition-transform"
            style="background: {c}; border-color: {formColor === c ? 'var(--color-text)' : 'transparent'}; transform: scale({formColor === c ? 1.2 : 1})"
            onclick={() => formColor = c}
          ></button>
        {/each}
        <input type="color" class="w-6 h-6 cursor-pointer" bind:value={formColor} />
      </div>
      <div class="flex gap-2 justify-end">
        <button class="px-3 py-1.5 rounded-lg text-sm" style="background: var(--color-surface); color: var(--color-text-secondary)" onclick={() => { showForm = false; editingTag = null; }}>取消</button>
        <button class="px-3 py-1.5 rounded-lg text-sm font-medium" style="background: var(--color-primary); color: white" disabled={saving} onclick={saveTag}>
          {saving ? '保存中...' : '保存'}
        </button>
      </div>
    </div>
  {/if}

  <!-- Search -->
  <input class="input-field w-full" placeholder="搜索标签..." bind:value={filterTag} />

  <!-- Tags Grid -->
  {#if loading}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">加载中...</div>
  {:else if filteredTags.length === 0}
    <div class="text-center py-8 text-sm" style="color: var(--color-text-muted)">暂无标签</div>
  {:else}
    <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
      {#each filteredTags as tag (tag.id)}
        <div
          class="card p-4 cursor-pointer hover:shadow-md transition-shadow {selectedTag?.id === tag.id ? 'ring-2' : ''}"
          style={selectedTag?.id === tag.id ? 'ring-color: var(--color-primary)' : ''}
          onclick={() => viewProjects(tag)}
          role="button"
          tabindex="0"
          onkeydown={(e) => { if (e.key === 'Enter') viewProjects(tag); }}
        >
          <div class="flex items-center gap-2 mb-2">
            <div class="w-3 h-3 rounded-full flex-shrink-0" style="background: {tag.color || '#3b82f6'}"></div>
            <span class="text-sm font-medium text-[var(--color-text)] truncate">{tag.name}</span>
          </div>
          {#if tag.project_count !== undefined}
            <p class="text-xs text-[var(--color-text-muted)]">{tag.project_count} 个项目</p>
          {/if}
          <div class="flex gap-1 mt-2">
            <button class="text-xs px-2 py-1 rounded hover:bg-[var(--color-surface)]" style="color: var(--color-text-muted)" onclick={(e) => { e.stopPropagation(); startEdit(tag); }}>编辑</button>
            <button class="text-xs px-2 py-1 rounded hover:bg-[var(--color-error-light)]" style="color: var(--color-error)" onclick={(e) => { e.stopPropagation(); deleteTag(tag); }}>删除</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <!-- Tag Projects -->
  {#if selectedTag}
    <div class="card p-5">
      <div class="flex items-center gap-2 mb-3">
        <div class="w-3 h-3 rounded-full" style="background: {selectedTag.color}"></div>
        <h3 class="text-sm font-semibold text-[var(--color-text)]">{selectedTag.name} 关联项目</h3>
      </div>
      {#if loadingProjects}
        <p class="text-xs text-[var(--color-text-muted)]">加载中...</p>
      {:else if tagProjects.length === 0}
        <p class="text-xs text-[var(--color-text-muted)]">暂无关联项目</p>
      {:else}
        <div class="space-y-2">
          {#each tagProjects as p (p.id)}
            <div class="flex items-center justify-between py-2 border-b last:border-0" style="border-color: var(--color-border)">
              <div>
                <span class="text-sm font-medium text-[var(--color-text)]">{p.name}</span>
                <span class="text-xs text-[var(--color-text-muted)] ml-2">{p.slug}</span>
              </div>
              <span class="text-xs text-[var(--color-text-muted)]">{new Date(p.created_at).toLocaleDateString()}</span>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .input-field {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 6px 12px;
    color: var(--color-text);
    font-size: 14px;
    outline: none;
  }
  .input-field:focus { border-color: var(--color-primary); }
</style>

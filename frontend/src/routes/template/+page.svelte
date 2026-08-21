<script lang="ts">
  import { onMount } from 'svelte';

  interface Template {
    id: number;
    name: string;
    description: string;
    content: string;
    use_count: number;
    created_at: string;
  }

  let templates = $state<Template[]>([]);
  let loading = $state(true);
  let showModal = $state(false);
  let editing = $state<Template | null>(null);
  let formName = $state('');
  let formDesc = $state('');
  let formContent = $state('');
  let saving = $state(false);
  let searchQuery = $state('');

  let filtered = $derived(
    searchQuery
      ? templates.filter(t => t.name.toLowerCase().includes(searchQuery.toLowerCase()))
      : templates
  );

  async function loadTemplates() {
    loading = true;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/templates', {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) templates = await res.json();
    } catch (e) { console.error(e); }
    loading = false;
  }

  function openCreate() {
    editing = null;
    formName = '';
    formDesc = '';
    formContent = '';
    showModal = true;
  }

  function openEdit(t: Template) {
    editing = t;
    formName = t.name;
    formDesc = t.description;
    formContent = t.content;
    showModal = true;
  }

  async function save() {
    if (!formName.trim()) return;
    saving = true;
    const token = localStorage.getItem('token');
    const body = { name: formName, description: formDesc, content: formContent };
    try {
      if (editing) {
        await fetch(`/api/v1/templates/${editing.id}`, {
          method: 'PUT',
          headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
          body: JSON.stringify(body)
        });
      } else {
        await fetch('/api/v1/templates', {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
          body: JSON.stringify(body)
        });
      }
      showModal = false;
      await loadTemplates();
    } catch (e) { console.error(e); }
    saving = false;
  }

  async function deleteTemplate(id: number) {
    if (!confirm('确定删除此模板？')) return;
    const token = localStorage.getItem('token');
    await fetch(`/api/v1/templates/${id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` }
    });
    templates = templates.filter(t => t.id !== id);
  }

  async function useTemplate(id: number) {
    const token = localStorage.getItem('token');
    const res = await fetch(`/api/v1/templates/${id}/use`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` }
    });
    if (res.ok) {
      const data = await res.json();
      alert(`项目已创建：${data.name || '新项目'}`);
    }
  }

  onMount(loadTemplates);
</script>

<div class="page">
  <div class="header">
    <h1>模板管理</h1>
    <button class="btn-primary" onclick={openCreate}>+ 新建模板</button>
  </div>

  <input
    type="text"
    bind:value={searchQuery}
    placeholder="搜索模板..."
    class="search-input"
  />

  {#if loading}
    <div class="empty">加载中...</div>
  {:else if filtered.length === 0}
    <div class="empty">
      <span class="empty-icon">📋</span>
      <p>暂无模板</p>
    </div>
  {:else}
    <div class="grid">
      {#each filtered as t}
        <div class="card">
          <div class="card-header">
            <h3>{t.name}</h3>
            <span class="uses">使用 {t.use_count} 次</span>
          </div>
          <p class="desc">{t.description || '暂无描述'}</p>
          <div class="card-actions">
            <button class="btn-sm primary" onclick={() => useTemplate(t.id)}>使用</button>
            <button class="btn-sm" onclick={() => openEdit(t)}>编辑</button>
            <button class="btn-sm danger" onclick={() => deleteTemplate(t.id)}>删除</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  {#if showModal}
    <div class="overlay" onclick={() => showModal = false}>
      <div class="modal" onclick={(e) => e.stopPropagation()}>
        <h2>{editing ? '编辑模板' : '新建模板'}</h2>
        <label>
          名称
          <input type="text" bind:value={formName} placeholder="模板名称" />
        </label>
        <label>
          描述
          <input type="text" bind:value={formDesc} placeholder="模板描述" />
        </label>
        <label>
          内容
          <textarea bind:value={formContent} rows="8" placeholder="模板内容（JSON 或文本）"></textarea>
        </label>
        <div class="modal-actions">
          <button class="btn-secondary" onclick={() => showModal = false}>取消</button>
          <button class="btn-primary" onclick={save} disabled={saving}>
            {saving ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .page { padding: 1.5rem; max-width: 1000px; margin: 0 auto; }
  .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
  .header h1 { font-size: 1.5rem; color: var(--text-primary); }
  .btn-primary { padding: 0.5rem 1.2rem; border: none; border-radius: 8px; background: var(--primary); color: white; cursor: pointer; font-weight: 600; }
  .search-input { width: 100%; padding: 0.6rem 1rem; border: 1px solid var(--border); border-radius: 8px; background: var(--bg-card); color: var(--text-primary); margin-bottom: 1.5rem; outline: none; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem; }
  .card { border: 1px solid var(--border); border-radius: 10px; padding: 1rem; background: var(--bg-card); }
  .card-header { display: flex; justify-content: space-between; align-items: center; }
  .card-header h3 { color: var(--text-primary); margin: 0; }
  .uses { font-size: 0.8rem; color: var(--text-tertiary); }
  .desc { color: var(--text-secondary); font-size: 0.9rem; margin: 0.5rem 0; }
  .card-actions { display: flex; gap: 0.5rem; margin-top: 0.75rem; }
  .btn-sm { padding: 0.3rem 0.7rem; border: 1px solid var(--border); border-radius: 6px; background: transparent; cursor: pointer; font-size: 0.8rem; color: var(--text-secondary); }
  .btn-sm.primary { background: var(--primary); color: white; border-color: var(--primary); }
  .btn-sm.danger { color: #ef4444; border-color: #ef4444; }
  .overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .modal { background: var(--bg-card); border-radius: 12px; padding: 1.5rem; width: 90%; max-width: 500px; }
  .modal h2 { margin: 0 0 1rem; color: var(--text-primary); }
  .modal label { display: block; margin-bottom: 1rem; color: var(--text-secondary); font-size: 0.85rem; }
  .modal input, .modal textarea { width: 100%; margin-top: 0.3rem; padding: 0.5rem; border: 1px solid var(--border); border-radius: 6px; background: var(--bg); color: var(--text-primary); font-size: 0.9rem; }
  .modal-actions { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 1rem; }
  .btn-secondary { padding: 0.5rem 1rem; border: 1px solid var(--border); border-radius: 8px; background: transparent; cursor: pointer; color: var(--text-secondary); }
  .empty { text-align: center; padding: 3rem; color: var(--text-secondary); }
  .empty-icon { font-size: 3rem; display: block; margin-bottom: 1rem; }
</style>

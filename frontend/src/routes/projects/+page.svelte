<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '../../lib/api/client';

  let projects = $state<any[]>([]);
  let loading = $state(true);

  onMount(async () => {
    try { projects = await client.get('/projects'); } catch (e) { console.error(e); } finally { loading = false; }
  });
</script>

<div class="p-6 max-w-7xl mx-auto">
  <div class="flex justify-between items-center mb-8">
    <div>
      <h1 class="text-2xl font-bold text-[var(--color-text)]">我的项目</h1>
      <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">管理和构建你的 Magisk 模块</p>
    </div>
    <button class="btn-primary flex items-center gap-2" onclick={() => window.location.href = '/projects/new'}>
      <span class="material-symbols-outlined text-[18px]">add</span>
      新建项目
    </button>
  </div>

  {#if loading}
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each Array(3) as _}
        <div class="card p-5"><div class="skeleton h-5 w-32 mb-3"></div><div class="skeleton h-3 w-full mb-2"></div><div class="skeleton h-3 w-2/3"></div></div>
      {/each}
    </div>
  {:else if projects.length === 0}
    <div class="text-center py-16">
      <span class="material-symbols-outlined text-5xl mb-3 block" style="color: var(--color-text-muted)">inventory_2</span>
      <p class="text-[var(--color-text-secondary)]">还没有项目，创建一个开始吧！</p>
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each projects as project}
        <div class="card-hover p-5 group">
          <div class="flex items-center gap-3 mb-3">
            <div class="w-10 h-10 rounded-xl flex items-center justify-center group-hover:scale-105 transition-transform" style="background: var(--color-primary-light)">
              <span class="material-symbols-outlined text-xl" style="color: var(--color-primary)">code_blocks</span>
            </div>
            <div>
              <h3 class="font-semibold text-[var(--color-text)]">{project.name}</h3>
              <span class="text-[10px] font-medium px-2 py-0.5 rounded-full" style="background: var(--color-primary-light); color: var(--color-primary)">{project.module_type}</span>
            </div>
          </div>
          <p class="text-sm text-[var(--color-text-secondary)] line-clamp-2 mb-4">{project.description || '无描述'}</p>
          <div class="flex gap-2">
            <a href="/projects/{project.id}" class="flex-1 py-2 rounded-xl text-sm font-medium text-center no-underline bg-primary-600 text-white hover:bg-primary-700 transition-colors">编辑</a>
            <a href="/projects/{project.id}/build" class="flex-1 py-2 rounded-xl text-sm font-medium text-center no-underline border border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors">构建</a>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

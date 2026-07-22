<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '../../lib/api/client';

  let projects = $state<any[]>([]);
  let loading = $state(true);

  onMount(async () => {
    try {
      projects = await client.get('/projects');
    } catch(e) {
      console.error(e);
    } finally {
      loading = false;
    }
  });
</script>

<div class="p-6">
  <div class="flex justify-between items-center mb-6">
    <h1 class="text-headline-large text-on-surface">我的项目</h1>
    <md-filled-tonal-button onclick={() => window.location.href='/projects/new'}>
      <md-icon slot="icon">add</md-icon>
      新建项目
    </md-filled-tonal-button>
  </div>

  {#if loading}
    <div class="flex justify-center p-12"><md-circular-progress indeterminate /></div>
  {:else if projects.length === 0}
    <div class="text-center p-12 text-on-surface-variant">
      <md-icon class="text-6xl mb-4">inventory_2</md-icon>
      <p class="text-body-large">还没有项目，创建一个开始吧！</p>
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each projects as project}
        <md-card>
          <div class="p-4">
            <div class="flex items-center gap-2 mb-2">
              <md-icon>code_blocks</md-icon>
              <span class="text-title-medium">{project.name}</span>
            </div>
            <span class="text-label-small text-tertiary px-2 py-0.5 rounded bg-tertiary-container">
              {project.module_type}
            </span>
            <p class="text-body-small text-on-surface-variant mt-2">{project.description || '无描述'}</p>
            <div class="flex gap-2 mt-4">
              <md-filled-tonal-button size="small" href="/projects/{project.id}">
                编辑
              </md-filled-tonal-button>
              <md-outlined-button size="small" href="/projects/{project.id}/build">
                构建
              </md-outlined-button>
            </div>
          </div>
        </md-card>
      {/each}
    </div>
  {/if}
</div>

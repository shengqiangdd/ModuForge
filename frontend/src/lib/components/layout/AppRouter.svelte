<script lang="ts">
  import type { Component } from 'svelte';
  import { t } from '$lib/i18n';
  import PageSkeleton from '$lib/components/ui/PageSkeleton.svelte';

  type Route = 'auth' | 'projects' | 'editor' | 'builds' | 'tests' | 'settings' | 'market' | 'market-publish' | 'dashboard' | 'ai' | 'mcp' | 'devices' | 'crash' | 'glossary' | 'notification' | 'activity' | 'search' | 'template' | 'module-version';

  interface Project { id: string; name: string; module_type: string; description: string; created_at: string; updated_at: string; }

  interface Props {
    current: Route;
    projectId: string;
    routeComponent: Component | null;
    routeKey: string;
    routeLoading: boolean;
    projects: Project[];
    filteredProjects: Project[];
    projectSearchRaw: string;
    showCreateModal: boolean;
    newProjectName: string;
    newProjectDesc: string;
    creatingProject: boolean;
    mobileMenuOpen: boolean;
    onNavigate: (route: Route, id?: string) => void;
    onSearchInput: (val: string) => void;
    onSetShowCreateModal: (v: boolean) => void;
    onSetNewProjectName: (v: string) => void;
    onSetNewProjectDesc: (v: string) => void;
    onCreateProject: () => void;
    onConfirmDelete: (id: string) => void;
    onSetMobileMenuOpen: (v: boolean) => void;
  }

  let {
    current,
    projectId,
    routeComponent,
    routeKey,
    routeLoading,
    projects,
    filteredProjects,
    projectSearchRaw,
    showCreateModal,
    newProjectName,
    newProjectDesc,
    creatingProject,
    mobileMenuOpen,
    onNavigate,
    onSearchInput,
    onSetShowCreateModal,
    onSetNewProjectName,
    onSetNewProjectDesc,
    onCreateProject,
    onConfirmDelete,
    onSetMobileMenuOpen,
  }: Props = $props();

  const bottomNavItems = $derived([
    { id: 'projects', icon: 'folder', label: '项目' },
    { id: 'ai', icon: 'psychology', label: 'AI' },
    { id: 'devices', icon: 'devices', label: '设备' },
    { id: 'market', icon: 'storefront', label: '市场' },
    { id: 'settings', icon: 'settings', label: '设置' },
  ]);
</script>

<!-- ═══ Main Content ═══ -->
<main class="flex-1 flex flex-col min-h-0 {current === 'ai' ? '' : 'pb-16 md:pb-0'}" style="background: var(--color-bg);">
  <!-- Page Content -->
  {#if current === 'projects'}
    <div class="flex-1 overflow-y-auto page-enter min-w-0" style="overscroll-behavior: contain; -webkit-overflow-scrolling: touch;">
      <div class="w-full p-4 md:p-6 max-w-7xl mx-auto">
        <div class="flex items-center gap-3 mb-4">
          <div class="flex-1">
            <h2 class="text-xl md:text-2xl font-bold" style="color: var(--color-text)">{$t('nav.projects')}</h2>
            <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">{$t('project.select_or_create')}</p>
          </div>
          <button class="btn-primary flex items-center gap-2" onclick={() => onSetShowCreateModal(true)}>
            <span class="material-symbols-outlined text-[18px]">add</span>
            新建
          </button>
        </div>
        <div class="relative mb-4">
          <span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-neutral-400 text-[18px]">search</span>
          <input type="text" placeholder="搜索项目..." class="input-field" style="padding-left: 36px;" value={projectSearchRaw} oninput={(e) => onSearchInput((e.target as HTMLInputElement).value)} />
        </div>
        {#if filteredProjects.length === 0 && projectSearchRaw}
          <div class="text-center py-16">
            <span class="material-symbols-outlined text-5xl mb-3 block" style="color: var(--color-text-muted)">search_off</span>
            <p class="text-[var(--color-text-secondary)]">没有找到匹配 "{projectSearchRaw}" 的项目</p>
          </div>
        {:else}
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            <!-- New Project Card -->
            <button
              class="border-2 border-dashed rounded-2xl p-8 text-center transition-all duration-200 cursor-pointer group min-h-[200px] flex flex-col items-center justify-center"
              style="border-color: var(--color-border); color: var(--color-text-secondary)"
              onclick={() => onSetShowCreateModal(true)}
            >
              <div class="w-12 h-12 rounded-2xl flex items-center justify-center mx-auto mb-3 group-hover:scale-110 transition-transform" style="background: var(--gradient-brand-subtle)">
                <span class="material-symbols-outlined text-2xl" style="color: var(--color-primary)">add</span>
              </div>
              <p class="font-semibold" style="color: var(--color-text)">{$t('project.new')}</p>
              <p class="text-xs mt-1" style="color: var(--color-text-muted)">Create a new universal module</p>
            </button>
            <!-- Existing Projects -->
            {#each filteredProjects as project, i (project.id)}
              <div
                role="button"
                tabindex="0"
                class="project-card rounded-2xl border p-5 cursor-pointer group relative overflow-hidden"
                style="background: var(--color-bg-elevated); border-color: var(--color-border)"
                onclick={() => onNavigate('editor', project.id)}
                onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onNavigate('editor', project.id); } }}
              >
                <!-- Hover gradient -->
                <div class="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-300" style="background: linear-gradient(135deg, color-mix(in srgb, var(--color-primary) 5%, transparent) 0%, color-mix(in srgb, var(--color-info) 3%, transparent) 100%)"></div>
                
                <div class="relative z-10">
                  <div class="flex items-start justify-between mb-3">
                    <div class="w-11 h-11 rounded-xl flex items-center justify-center group-hover:scale-110 transition-transform duration-300" style="background: var(--gradient-brand-subtle)">
                      <span class="material-symbols-outlined" style="color: var(--color-primary)">folder</span>
                    </div>
                    <button class="p-2 rounded-lg opacity-0 group-hover:opacity-100 transition-all duration-200 min-w-[44px] min-h-[44px] flex items-center justify-center hover:bg-[var(--color-error-light)]"
                            style="color: var(--color-text-muted)"
                            onclick={(e) => { e.stopPropagation(); onConfirmDelete(project.id); }}
                            aria-label="删除项目 {project.name}">
                      <span class="material-symbols-outlined text-[18px]" style="color: var(--color-error)">delete</span>
                    </button>
                  </div>
                  <h3 class="font-semibold mb-1 group-hover:text-[var(--color-primary)] transition-colors" style="color: var(--color-text)">{project.name}</h3>
                  <p class="text-xs leading-relaxed" style="color: var(--color-text-muted); max-height: 10rem; overflow-y: auto; scrollbar-width: thin; white-space: pre-wrap">{project.description || 'No description'}</p>
                  <div class="flex items-center gap-2 mt-3">
                    <span class="badge" style="background: var(--color-primary-light); color: var(--color-primary)">{project.module_type ? project.module_type.charAt(0).toUpperCase() + project.module_type.slice(1) : 'Universal'}</span>
                    <span class="text-[11px]" style="color: var(--color-text-muted)">{new Date(project.updated_at).toLocaleDateString()}</span>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>

    <!-- Create Project Modal -->
    {#if showCreateModal}
      <div class="fixed inset-0 z-50 flex items-center justify-center p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onSetShowCreateModal(false); }} onkeydown={(e) => { if (e.key === 'Escape') onSetShowCreateModal(false); }}>
        <div class="w-full max-w-md rounded-2xl shadow-xl p-6 border animate-[scaleIn_0.2s_ease-out]"
             style="background: var(--color-bg-elevated); border-color: var(--color-border)"
             role="dialog" aria-modal="true" aria-labelledby="new-project-title" tabindex="-1">
          <h3 id="new-project-title" class="text-lg font-bold mb-4" style="color: var(--color-text)">{$t('project.new')}</h3>
          <div class="space-y-4">
            <div>
              <label for="new-project-name" class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">项目名称</label>
              <input id="new-project-name" type="text" value={newProjectName} oninput={(e) => onSetNewProjectName((e.target as HTMLInputElement).value)} placeholder="My Awesome Module"
                     class="input-field" />
            </div>
            <div class="flex items-center gap-1.5 px-2 py-1 rounded-md w-fit" style="background: var(--color-primary-light); color: var(--color-primary)">
              <span class="material-symbols-outlined text-[12px]">hub</span>
              <span class="text-[10px] font-medium">Universal · Magisk + KSU + APatch</span>
            </div>
            <div>
              <label for="new-project-desc" class="block text-sm font-medium mb-1.5" style="color: var(--color-text-secondary)">描述</label>
              <textarea id="new-project-desc" value={newProjectDesc} oninput={(e) => onSetNewProjectDesc((e.target as HTMLTextAreaElement).value)} placeholder="Optional description..." rows="3"
                        class="input-field resize-none"></textarea>
            </div>
          </div>
          <div class="flex justify-end gap-3 mt-6">
            <button class="btn-ghost" onclick={() => onSetShowCreateModal(false)}>取消</button>
            <button class="btn-primary disabled:opacity-50"
                    onclick={onCreateProject} disabled={creatingProject || !newProjectName.trim()}>
              {creatingProject ? '创建中...' : '创建'}
            </button>
          </div>
        </div>
      </div>
    {/if}
  {:else}
    <!-- Lazy-loaded route component -->
    <div class="flex-1 {current === 'ai' ? '' : 'overflow-y-auto'} flex flex-col min-h-0 min-w-0 {current === 'ai' ? '' : 'page-enter'}" style="overscroll-behavior: contain; -webkit-overflow-scrolling: touch;">
      {#if routeLoading}
        <PageSkeleton variant={current === 'editor' ? 'editor' : current === 'market' ? 'market' : current === 'market-publish' ? 'market' : 'list'} />
      {:else if routeComponent}
        {#key routeKey}
          {@const RouteComp = routeComponent}
          <RouteComp
            {projectId}
            onNavigate={(route: string, id?: string) => onNavigate(route as Route, id)}
          />
        {/key}
      {/if}
    </div>
  {/if}
</main>

<!-- ═══ Mobile Bottom Nav (hidden on AI page) ═══ -->
{#if current !== 'ai'}
<nav class="md:hidden fixed bottom-0 left-0 right-0 h-16 glass flex items-center justify-around px-2 z-40" style="border-top: 1px solid var(--color-border)">
  {#each bottomNavItems as item}
    {@const isActive = current === item.id}
    <button
      class="flex flex-col items-center gap-0.5 py-1.5 px-3 rounded-xl transition-all duration-150 min-w-[60px] min-h-[44px]"
      style={isActive ? 'color: var(--color-primary)' : 'color: var(--color-text-muted)'}
      onclick={() => onNavigate(item.id as Route, item.id === 'editor' ? projectId : undefined)}
    >
      <span class="material-symbols-outlined text-[22px]">{item.icon}</span>
      <span class="text-[10px] font-medium leading-tight">{item.label}</span>
    </button>
  {/each}
  <button
    class="flex flex-col items-center gap-0.5 py-1.5 px-3 rounded-xl transition-all duration-150 min-w-[60px] min-h-[44px]"
    style={mobileMenuOpen ? 'color: var(--color-primary)' : 'color: var(--color-text-muted)'}
    onclick={() => onSetMobileMenuOpen(!mobileMenuOpen)}
  >
    <span class="material-symbols-outlined text-[22px]">{mobileMenuOpen ? 'close' : 'more_horiz'}</span>
    <span class="text-[10px] font-medium leading-tight">更多</span>
  </button>
</nav>
{/if}

{#if mobileMenuOpen && current !== 'ai'}
  <div class="fixed inset-0 z-50 md:hidden" role="presentation" onclick={() => onSetMobileMenuOpen(false)} onkeydown={(e) => { if (e.key === 'Escape') onSetMobileMenuOpen(false); }}>
    <div class="absolute bottom-16 left-2 right-2 rounded-2xl border shadow-2xl p-4 grid grid-cols-3 gap-3"
         style="background: var(--color-bg-elevated); border-color: var(--color-border)"
         role="presentation"
         onclick={(e) => e.stopPropagation()}>
      {#each [
        { id: 'dashboard', icon: 'monitoring', label: '仪表盘' },
        { id: 'glossary', icon: 'menu_book', label: '术语表' },
        { id: 'crash', icon: 'bug_report', label: '崩溃分析' },
        { id: 'market-publish', icon: 'publish', label: '发布模块' },
      ] as item}
        <button class="flex flex-col items-center gap-1.5 p-3 rounded-xl hover:bg-[var(--color-surface)] transition-colors"
                style="color: var(--color-text-secondary)"
                onclick={() => { onNavigate(item.id as Route); onSetMobileMenuOpen(false); }}>
          <span class="material-symbols-outlined text-[24px]">{item.icon}</span>
          <span class="text-[11px] font-medium">{item.label}</span>
        </button>
      {/each}
    </div>
  </div>
{/if}

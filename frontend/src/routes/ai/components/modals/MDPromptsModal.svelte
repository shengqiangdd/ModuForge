<script lang="ts">
  import { toast } from '$lib/stores/toast.svelte';
  
  let { open, onClose }: { open: boolean; onClose: () => void } = $props();
  
  let prompts = $state<Array<{name: string; content: string; size: number; is_md: boolean}>>([]);
  let selectedPrompt = $state<string>('');
  let editContent = $state<string>('');
  let loading = $state(false);
  let saving = $state(false);
  
  // Load prompts when modal opens
  $effect(() => {
    if (open) {
      loadPrompts();
    }
  });
  
  async function loadPrompts() {
    loading = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch('/api/v1/md-prompts', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        prompts = data.prompts || [];
        if (prompts.length > 0 && !selectedPrompt) {
          selectPrompt(prompts[0].name);
        }
      } else {
        toast('加载提示词列表失败', 'error');
      }
    } catch (e) {
      toast(`加载失败: ${e instanceof Error ? e.message : e}`, 'error');
    } finally {
      loading = false;
    }
  }
  
  async function selectPrompt(name: string) {
    selectedPrompt = name;
    loading = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/md-prompts/${name}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        editContent = data.content || '';
      } else {
        toast('加载提示词内容失败', 'error');
      }
    } catch (e) {
      toast(`加载失败: ${e instanceof Error ? e.message : e}`, 'error');
    } finally {
      loading = false;
    }
  }
  
  async function savePrompt() {
    if (!selectedPrompt || !editContent) {
      toast('请选择提示词并输入内容', 'error');
      return;
    }
    
    saving = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/md-prompts/${selectedPrompt}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ content: editContent })
      });
      
      if (res.ok) {
        toast('提示词保存成功', 'success');
        // Reload the prompt list to reflect changes
        await loadPrompts();
      } else {
        const err = await res.json().catch(() => ({}));
        toast(`保存失败: ${err.error || '未知错误'}`, 'error');
      }
    } catch (e) {
      toast(`保存失败: ${e instanceof Error ? e.message : e}`, 'error');
    } finally {
      saving = false;
    }
  }
  
  async function resetPrompt() {
    if (!selectedPrompt) {
      toast('请选择要重置的提示词', 'error');
      return;
    }
    
    if (!confirm(`确定要重置 ${selectedPrompt} 到默认内容吗？`)) {
      return;
    }
    
    loading = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/md-prompts/${selectedPrompt}/reset`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      
      if (res.ok) {
        const data = await res.json();
        editContent = data.content || '';
        toast('提示词已重置', 'success');
      } else {
        toast('重置失败', 'error');
      }
    } catch (e) {
      toast(`重置失败: ${e instanceof Error ? e.message : e}`, 'error');
    } finally {
      loading = false;
    }
  }
  
  async function reloadPrompts() {
    loading = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch('/api/v1/md-prompts/reload', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      
      if (res.ok) {
        toast('提示词已重新加载', 'success');
        await loadPrompts();
      } else {
        toast('重新加载失败', 'error');
      }
    } catch (e) {
      toast(`重新加载失败: ${e instanceof Error ? e.message : e}`, 'error');
    } finally {
      loading = false;
    }
  }
  
  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-2 sm:p-4">
    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-6xl max-h-[95vh] sm:max-h-[90vh] flex flex-col">
      <!-- Header -->
      <div class="flex items-center justify-between px-4 sm:px-6 py-3 sm:py-4 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
        <div class="min-w-0">
          <h2 class="text-lg sm:text-xl font-semibold text-gray-900 dark:text-white truncate">
            MD 提示词编辑器
          </h2>
          <p class="text-xs sm:text-sm text-gray-500 dark:text-gray-400 mt-0.5 hidden sm:block">
            编辑 Agent 的系统提示词（Markdown 格式）
          </p>
        </div>
        <div class="flex items-center gap-2 flex-shrink-0 ml-2">
          <button
            onclick={reloadPrompts}
            disabled={loading}
            class="px-2 sm:px-3 py-1.5 text-xs sm:text-sm bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-lg transition-colors disabled:opacity-50"
          >
            <span class="hidden sm:inline">🔄 重新加载</span>
            <span class="sm:hidden">🔄</span>
          </button>
          <button
            onclick={onClose}
            class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
          >
            ✕
          </button>
        </div>
      </div>
      
      <!-- Content -->
      <div class="flex flex-1 overflow-hidden min-h-0">
        <!-- Sidebar - Prompt List (hidden on mobile, show as dropdown) -->
        <div class="hidden sm:block w-56 md:w-64 border-r border-gray-200 dark:border-gray-700 overflow-y-auto flex-shrink-0">
          <div class="p-3 md:p-4">
            <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
              提示词文件
            </h3>
            {#if loading && prompts.length === 0}
              <div class="text-center py-4 text-gray-500">加载中...</div>
            {:else if prompts.length === 0}
              <div class="text-center py-4 text-gray-500">暂无提示词</div>
            {:else}
              <div class="space-y-1">
                {#each prompts as prompt}
                  <button
                    onclick={() => selectPrompt(prompt.name)}
                    class="w-full text-left px-3 py-2 rounded-lg transition-colors {selectedPrompt === prompt.name 
                      ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300' 
                      : 'hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300'}"
                  >
                    <div class="font-medium text-sm truncate">{prompt.name}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      {formatSize(prompt.size)}
                    </div>
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        </div>
        
        <!-- Editor -->
        <div class="flex-1 flex flex-col overflow-hidden min-w-0">
          <!-- Mobile: prompt selector -->
          {#if prompts.length > 0}
            <div class="sm:hidden px-3 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/50">
              <select
                class="w-full px-3 py-2 rounded-lg text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 border border-gray-300 dark:border-gray-600"
                value={selectedPrompt}
                onchange={(e) => selectPrompt((e.target as HTMLSelectElement).value)}
              >
                {#each prompts as prompt}
                  <option value={prompt.name}>{prompt.name} ({formatSize(prompt.size)})</option>
                {/each}
              </select>
            </div>
          {/if}

          {#if selectedPrompt}
            <div class="flex items-center justify-between px-3 sm:px-4 py-2 sm:py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/50 flex-shrink-0">
              <div class="font-medium text-gray-900 dark:text-white text-sm sm:text-base truncate min-w-0">
                {selectedPrompt}
              </div>
              <div class="flex items-center gap-1.5 sm:gap-2 flex-shrink-0 ml-2">
                <button
                  onclick={resetPrompt}
                  disabled={loading}
                  class="px-2 sm:px-3 py-1.5 text-xs sm:text-sm bg-yellow-100 dark:bg-yellow-900/30 hover:bg-yellow-200 dark:hover:bg-yellow-800/50 text-yellow-700 dark:text-yellow-300 rounded-lg transition-colors disabled:opacity-50"
                >
                  <span class="hidden sm:inline">↩️ 重置为默认</span>
                  <span class="sm:hidden">↩️</span>
                </button>
                <button
                  onclick={savePrompt}
                  disabled={saving || loading}
                  class="px-3 sm:px-4 py-1.5 text-xs sm:text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50"
                >
                  {saving ? '...' : '💾 保存'}
                </button>
              </div>
            </div>
            
            <div class="flex-1 overflow-hidden min-h-0">
              <textarea
                bind:value={editContent}
                disabled={loading}
                class="w-full h-full p-3 sm:p-4 font-mono text-xs sm:text-sm bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 border-none resize-none focus:outline-none disabled:opacity-50"
                placeholder="输入 Markdown 格式的提示词..."
                spellcheck="false"
              ></textarea>
            </div>
          {:else}
            <div class="flex-1 flex items-center justify-center text-gray-500 text-sm p-4">
              选择一个提示词文件进行编辑
            </div>
          {/if}
        </div>
      </div>
      
      <!-- Footer -->
      <div class="px-4 sm:px-6 py-2 sm:py-3 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/50 flex-shrink-0">
        <div class="flex items-center justify-between text-xs sm:text-sm text-gray-500 dark:text-gray-400">
          <div class="hidden sm:block">
            💡 提示词使用 Markdown 格式，支持标题、列表、代码块等
          </div>
          <div>
            共 {prompts.length} 个文件
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

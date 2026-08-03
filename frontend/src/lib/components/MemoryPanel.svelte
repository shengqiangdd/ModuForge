<script lang="ts">
  interface KnowledgeEntry {
    id: number
    category: string
    key: string
    value: string
    updated_at: string
  }

  interface SessionSummary {
    id: number
    session_id: string
    summary: string
    key_decisions: string[]
    files_changed: string[]
    created_at: string
  }

  let { projectId = '' }: { projectId?: string } = $props()

  let expanded = $state(true)
  let knowledge = $state<KnowledgeEntry[]>([])
  let summaries = $state<SessionSummary[]>([])
  let activeTab = $state<'knowledge' | 'history'>('knowledge')
  let newCategory = $state('architecture')
  let newKey = $state('')
  let newValue = $state('')
  let loading = $state(false)

  const CATEGORY_LABELS: Record<string, string> = {
    architecture: '🏗️ 架构决策',
    decision: '🔧 技术决策',
    issue: '⚠️ 已知问题',
    file_purpose: '📁 文件用途',
    tech_stack: '🛠️ 技术栈',
    requirement: '📋 需求记录',
  }

  $effect(() => {
    if (projectId) {
      loadKnowledge()
      loadSummaries()
    }
  })

  async function loadKnowledge() {
    if (!projectId) return
    try {
      const res = await fetch(`/api/ai/memory/project/${projectId}`)
      if (res.ok) {
        const data = await res.json()
        knowledge = data || []
      }
    } catch (e) {
      console.error('Failed to load knowledge:', e)
    }
  }

  async function loadSummaries() {
    if (!projectId) return
    try {
      const res = await fetch(`/api/ai/memory/summaries/${projectId}`)
      if (res.ok) {
        const data = await res.json()
        summaries = data || []
      }
    } catch (e) {
      console.error('Failed to load summaries:', e)
    }
  }

  async function addKnowledge() {
    if (!projectId || !newKey || !newValue) return
    loading = true
    try {
      const res = await fetch(`/api/ai/memory/project/${projectId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ category: newCategory, key: newKey, value: newValue })
      })
      if (res.ok) {
        newKey = ''
        newValue = ''
        await loadKnowledge()
      }
    } catch (e) {
      console.error('Failed to add knowledge:', e)
    } finally {
      loading = false
    }
  }

  async function deleteKnowledge(category: string, key: string) {
    if (!projectId) return
    try {
      await fetch(`/api/ai/memory/project/${projectId}/${category}/${key}`, { method: 'DELETE' })
      await loadKnowledge()
    } catch (e) {
      console.error('Failed to delete knowledge:', e)
    }
  }

  let grouped = $derived(
    knowledge.reduce((acc: Record<string, KnowledgeEntry[]>, entry) => {
      if (!acc[entry.category]) acc[entry.category] = []
      acc[entry.category].push(entry)
      return acc
    }, {} as Record<string, KnowledgeEntry[]>)
  )
</script>

{#if projectId}
  <div class="bg-white border border-gray-200 rounded-lg">
    <!-- Header -->
    <button
      onclick={() => expanded = !expanded}
      class="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
    >
      <span class="flex items-center gap-2">
        🧠 项目记忆
        <span class="text-xs text-gray-400">
          {knowledge.length} 条知识 · {summaries.length} 条摘要
        </span>
      </span>
      <span class="text-gray-400">{expanded ? '−' : '+'}</span>
    </button>

    {#if expanded}
      <div class="border-t border-gray-100">
        <!-- Tabs -->
        <div class="flex border-b border-gray-100">
          <button
            onclick={() => activeTab = 'knowledge'}
            class="flex-1 py-2 text-sm {activeTab === 'knowledge' ? 'text-blue-600 border-b-2 border-blue-600' : 'text-gray-500'}"
          >
            知识库
          </button>
          <button
            onclick={() => activeTab = 'history'}
            class="flex-1 py-2 text-sm {activeTab === 'history' ? 'text-blue-600 border-b-2 border-blue-600' : 'text-gray-500'}"
          >
            会话历史
          </button>
        </div>

        <div class="p-3 max-h-96 overflow-y-auto">
          {#if activeTab === 'knowledge'}
            <!-- Add new knowledge -->
            <div class="mb-3 p-2 bg-gray-50 rounded-lg">
              <select
                bind:value={newCategory}
                class="w-full mb-2 px-2 py-1.5 text-xs border border-gray-200 rounded"
              >
                {#each Object.entries(CATEGORY_LABELS) as [k, v]}
                  <option value={k}>{v}</option>
                {/each}
              </select>
              <input
                type="text"
                placeholder="名称/标签"
                bind:value={newKey}
                class="w-full mb-2 px-2 py-1.5 text-xs border border-gray-200 rounded"
              />
              <textarea
                placeholder="内容"
                bind:value={newValue}
                rows={2}
                class="w-full mb-2 px-2 py-1.5 text-xs border border-gray-200 rounded resize-none"
              ></textarea>
              <button
                onclick={addKnowledge}
                disabled={loading || !newKey || !newValue}
                class="w-full py-1.5 text-xs bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50"
              >
                {loading ? '保存中...' : '添加'}
              </button>
            </div>

            <!-- Knowledge list -->
            {#if Object.keys(grouped).length === 0}
              <p class="text-xs text-gray-400 text-center py-4">暂无项目知识</p>
            {:else}
              {#each Object.entries(grouped) as [cat, entries]}
                <div class="mb-3">
                  <div class="text-xs font-medium text-gray-500 mb-1">
                    {CATEGORY_LABELS[cat] || cat}
                  </div>
                  {#each entries as entry (entry.id)}
                    <div class="group flex items-start gap-2 p-2 bg-gray-50 rounded mb-1">
                      <div class="flex-1 min-w-0">
                        <div class="text-xs font-medium text-gray-700">{entry.key}</div>
                        <div class="text-xs text-gray-500 whitespace-pre-wrap">{entry.value}</div>
                      </div>
                      <button
                        onclick={() => deleteKnowledge(entry.category, entry.key)}
                        class="opacity-0 group-hover:opacity-100 text-red-400 hover:text-red-600 text-xs"
                      >
                        ✕
                      </button>
                    </div>
                  {/each}
                </div>
              {/each}
            {/if}
          {:else}
            <!-- Session history -->
            {#if summaries.length === 0}
              <p class="text-xs text-gray-400 text-center py-4">暂无会话摘要</p>
            {:else}
              {#each summaries as s (s.id)}
                <div class="mb-3 p-2 bg-gray-50 rounded-lg">
                  <div class="text-xs text-gray-400 mb-1">
                    {new Date(s.created_at).toLocaleDateString('zh-CN')}
                  </div>
                  <div class="text-xs text-gray-700 whitespace-pre-wrap">{s.summary}</div>
                  {#if s.key_decisions && s.key_decisions.length > 0}
                    <div class="mt-1.5 text-xs text-gray-500">
                      <span class="font-medium">决策：</span>
                      {s.key_decisions.join('；')}
                    </div>
                  {/if}
                  {#if s.files_changed && s.files_changed.length > 0}
                    <div class="mt-1 text-xs text-gray-400">
                      <span class="font-medium">文件：</span>
                      {s.files_changed.join(', ')}
                    </div>
                  {/if}
                </div>
              {/each}
            {/if}
          {/if}
        </div>
      </div>
    {/if}
  </div>
{/if}

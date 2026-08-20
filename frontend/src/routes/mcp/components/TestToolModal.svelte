<script lang="ts">
  import { client } from '$lib/api/client';

  interface Props {
    server: string;
    tool: string;
    onClose: () => void;
  }

  let { server, tool, onClose }: Props = $props();

  let testArgs = $state('{}');
  let testResult = $state('');
  let testing = $state(false);

  async function runTest() {
    if (!server || !tool) return;
    testing = true;
    testResult = '';
    let parsed: Record<string, unknown> = {};
    try {
      parsed = JSON.parse(testArgs || '{}');
    } catch {
      testResult = '❌ 参数不是合法 JSON';
      testing = false;
      return;
    }
    try {
      const data = await client.post<{ result: string }>('/agent/mcp/test', {
        server,
        tool,
        arguments: parsed,
      });
      testResult = data.result || '(空结果)';
    } catch (e: any) {
      testResult = '❌ ' + (e.message || '调用失败');
    } finally {
      testing = false;
    }
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }
</script>

<div
  class="fixed inset-0 z-50 flex items-center justify-center p-4"
  style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)"
  role="presentation"
  onclick={handleBackdropClick}
>
  <div
    class="w-full max-w-lg rounded-2xl shadow-xl p-6 border animate-[scaleIn_0.2s_ease-out]"
    style="background: var(--color-bg-elevated); border-color: var(--color-border)"
    role="dialog"
    aria-modal="true"
  >
    <div class="flex items-center gap-2 mb-4">
      <span class="material-symbols-outlined" style="color: var(--color-primary)">terminal</span>
      <h3 class="text-lg font-bold flex-1" style="color: var(--color-text)">测试工具调用</h3>
      <button class="p-2 rounded-lg hover:bg-[var(--color-surface)]" style="color: var(--color-text-muted)" onclick={onClose}>✕</button>
    </div>

    <div class="space-y-3 mb-4">
      <div>
        <label class="block text-xs font-medium mb-1" style="color: var(--color-text-muted)">服务器 / 工具</label>
        <div class="flex items-center gap-2 font-mono text-sm" style="color: var(--color-text)">
          <span class="badge" style="background: var(--color-primary-light); color: var(--color-primary)">{server}</span>
          <span style="color: var(--color-text-muted)">→</span>
          <span class="badge" style="background: var(--color-success-light); color: var(--color-success)">{tool}</span>
        </div>
      </div>
      <div>
        <label for="mcp-test-args" class="block text-xs font-medium mb-1" style="color: var(--color-text-muted)">参数 (JSON)</label>
        <textarea
          id="mcp-test-args"
          bind:value={testArgs}
          rows="4"
          class="input-field font-mono text-xs resize-none"
          placeholder={'{"owner":"octocat","repo":"hello-world"}'}
        ></textarea>
      </div>
      {#if testResult}
        <div>
          <label class="block text-xs font-medium mb-1" style="color: var(--color-text-muted)">结果</label>
          <pre
            class="rounded-xl p-3 overflow-x-auto text-xs font-mono whitespace-pre-wrap"
            style="background: var(--color-surface); color: var(--color-text); max-height: 240px; overflow-y: auto"
          >{testResult}</pre>
        </div>
      {/if}
    </div>

    <div class="flex justify-end gap-3">
      <button class="btn-ghost" onclick={onClose}>关闭</button>
      <button class="btn-primary disabled:opacity-50" onclick={runTest} disabled={testing}>
        {testing ? '调用中...' : '调用'}
      </button>
    </div>
  </div>
</div>

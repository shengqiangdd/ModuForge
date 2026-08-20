<script lang="ts">
  import { apiPost } from '../device-api';

  let { serial, onMsg }: { serial: string; onMsg: (err: string, ok?: string) => void } = $props();

  let shellCmd = $state('');
  let shellOutput = $state('');

  async function runShell() {
    if (!shellCmd.trim()) return;
    const d = await apiPost('/api/v1/adb/shell', { serial, command: shellCmd.trim() });
    shellOutput = d.error || d.output || '';
  }
</script>

<div class="info-card overflow-hidden">
  <div class="p-4 border-b" style="border-color: var(--color-border)">
    <div class="flex gap-2">
      <input type="text" class="input-field flex-1 font-mono text-sm" placeholder="输入 Shell 命令..." bind:value={shellCmd}
        onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') runShell(); }} />
      <button class="btn-primary text-sm" onclick={runShell}>执行</button>
    </div>
  </div>
  <pre class="p-4 text-xs overflow-auto max-h-[500px] font-mono" style="background: var(--color-surface); color: var(--color-text); white-space: pre-wrap; word-break: break-all">{shellOutput || '$ '}</pre>
</div>

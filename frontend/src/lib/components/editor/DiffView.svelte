<script lang="ts">
  import CodeEditor from '$lib/components/CodeEditor.svelte';

  let {
    files = [],
    selectedDiffFile = null,
    onSelectDiffFile,
    onCloseDiffView,
    detectLanguage = (_: string) => 'javascript',
  }: {
    files?: { path: string; current: string; incoming: string }[];
    selectedDiffFile?: string | null;
    onSelectDiffFile?: (path: string) => void;
    onCloseDiffView?: () => void;
    detectLanguage?: (path: string) => string;
  } = $props();

  function getCurrentContent(): string {
    if (!selectedDiffFile) return '';
    const df = files.find(f => f.path === selectedDiffFile);
    return df?.current || '';
  }

  function getIncomingContent(): string {
    if (!selectedDiffFile) return '';
    const df = files.find(f => f.path === selectedDiffFile);
    return df?.incoming || '';
  }
</script>

{#if selectedDiffFile}
  <div class="flex-1 overflow-hidden">
    <CodeEditor
      value={getCurrentContent()}
      language={detectLanguage(selectedDiffFile)}
      onChange={() => {}}
      diffMode={true}
      diffContent={getIncomingContent()}
      diffLabel={selectedDiffFile}
    />
  </div>
{/if}

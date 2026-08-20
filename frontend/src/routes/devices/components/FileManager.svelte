<script lang="ts">
  import { apiGet, apiPost } from '../device-api';
  import type { FileInfo } from '../device-types';

  let {
    serial,
    onMsg,
    onConfirm
  }: {
    serial: string;
    onMsg: (err: string, ok?: string) => void;
    onConfirm: (title: string, message: string, variant: 'primary' | 'danger', cb: () => void) => void;
  } = $props();

  let files = $state<FileInfo[]>([]);
  let currentPath = $state('/sdcard/');
  let fileSearchQuery = $state('');
  let filteredFiles = $derived(!fileSearchQuery ? files : files.filter(f => f.name.toLowerCase().includes(fileSearchQuery.toLowerCase())));
  let newFolderName = $state('');
  let renameTarget = $state('');
  let renamePath = $state('');
  let uploading = $state(false);
  let uploadTarget = $state('');
  let fileInput = $state<HTMLInputElement | null>(null);
  let dragOver = $state(false);
  let previewContent = $state('');
  let previewPath = $state('');

  function formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  async function loadFiles(path?: string) {
    if (!serial) return;
    if (path !== undefined) currentPath = path;
    const d = await apiGet(`/api/v1/adb/files?serial=${serial}&path=${encodeURIComponent(currentPath)}`);
    files = d.files || [];
    currentPath = d.path || currentPath;
  }

  function navigateTo(path: string) {
    loadFiles(path);
  }

  function goUp() {
    if (currentPath === '/') return;
    const parts = currentPath.split('/').filter(Boolean);
    parts.pop();
    const parent = parts.length === 0 ? '/' : '/' + parts.join('/') + '/';
    loadFiles(parent);
  }

  async function deleteFile(path: string) {
    await apiPost('/api/v1/adb/delete', { serial, remote_path: path });
    loadFiles();
  }

  async function downloadFile(path: string) {
    const token = localStorage.getItem('moduforge_token') || '';
    const res = await fetch(`/api/v1/adb/file/download?serial=${encodeURIComponent(serial)}&path=${encodeURIComponent(path)}`, {
      headers: { 'Authorization': `Bearer ${token}` },
    });
    if (!res.ok) { onMsg('下载失败'); return; }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = path.split('/').filter(Boolean).pop() || 'file';
    a.click();
    URL.revokeObjectURL(url);
    onMsg('', '下载完成');
  }

  async function mkdir() {
    if (!newFolderName.trim()) { onMsg('请输入文件夹名'); return; }
    const d = await apiPost('/api/v1/adb/mkdir', { serial, remote_path: currentPath + newFolderName.trim() });
    if (d.error) { onMsg(d.error); return; }
    newFolderName = '';
    onMsg('', '文件夹已创建');
    loadFiles();
  }

  async function renameFile(oldPath: string) {
    const newName = renameTarget.trim();
    if (!newName) { onMsg('请输入新文件名'); return; }
    const parts = oldPath.split('/');
    parts[parts.length - 1] = newName;
    const newPath = parts.join('/');
    const d = await apiPost('/api/v1/adb/rename', { serial, old_path: oldPath, new_path: newPath });
    if (d.error) { onMsg(d.error); return; }
    renameTarget = '';
    renamePath = '';
    onMsg('', '重命名成功');
    loadFiles();
  }

  async function previewFile(path: string) {
    const token = localStorage.getItem('moduforge_token') || '';
    const res = await fetch('/api/v1/adb/file/read', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ serial, remote_path: path }),
    });
    const d = await res.json();
    if (d.error) { onMsg(d.error); return; }
    previewContent = d.content || '(空文件)';
    previewPath = path;
  }

  function closePreview() {
    previewPath = '';
    previewContent = '';
  }

  async function uploadFile() {
    if (!fileInput?.files?.length) { onMsg('请选择文件'); return; }
    const targetPath = uploadTarget.trim() || currentPath;
    uploading = true;
    const token = localStorage.getItem('moduforge_token') || '';
    const form = new FormData();
    form.append('serial', serial);
    form.append('remote_path', targetPath.endsWith('/') ? targetPath + fileInput.files[0].name : targetPath);
    form.append('file', fileInput.files[0]);
    const res = await fetch('/api/v1/adb/file/upload', {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}` },
      body: form,
    });
    const d = await res.json();
    uploading = false;
    if (d.error) { onMsg(d.error); return; }
    fileInput.value = '';
    uploadTarget = '';
    onMsg('', '上传成功');
    loadFiles();
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    dragOver = true;
  }

  function handleDragLeave() {
    dragOver = false;
  }

  async function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
    const dropFiles = e.dataTransfer?.files;
    if (!dropFiles || dropFiles.length === 0) return;
    const token = localStorage.getItem('moduforge_token') || '';
    for (let i = 0; i < dropFiles.length; i++) {
      const form = new FormData();
      form.append('serial', serial);
      form.append('remote_path', currentPath + dropFiles[i].name);
      form.append('file', dropFiles[i]);
      await fetch('/api/v1/adb/file/upload', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
        body: form,
      });
    }
    onMsg('', `已上传 ${dropFiles.length} 个文件`);
    loadFiles();
  }

  $effect(() => {
    if (serial) loadFiles();
  });
</script>

<div class="info-card overflow-hidden">
  <div class="p-4 border-b flex flex-wrap items-center justify-between gap-2" style="border-color: var(--color-border)">
    <div class="flex items-center gap-2 text-sm min-w-0">
      <button class="btn-ghost text-xs px-2" onclick={goUp} disabled={currentPath === '/'}>⬆</button>
      <span style="color: var(--color-text-secondary)">路径:</span>
      <code class="text-xs px-2 py-1 rounded truncate max-w-[40vw] sm:max-w-none" style="background: var(--color-surface)">{currentPath}</code>
    </div>
    <button class="btn-ghost text-xs" onclick={() => loadFiles()}>刷新</button>
  </div>
  <!-- New Folder + Upload + Search Area -->
  <div class="p-3 border-b space-y-2" style="border-color: var(--color-border); background: var(--color-surface)">
    <div class="flex items-center gap-2">
      <input type="text" class="input-field text-xs flex-1" placeholder="搜索文件..." bind:value={fileSearchQuery}
        onkeydown={(e: KeyboardEvent) => { if (e.key === 'Escape') fileSearchQuery = ''; }} />
      {#if fileSearchQuery}
        <span class="text-xs" style="color: var(--color-text-muted)">{filteredFiles.length}/{files.length}</span>
      {/if}
    </div>
    <div class="flex items-center gap-2">
      <input type="text" class="input-field text-xs flex-1" placeholder="新建文件夹名" bind:value={newFolderName}
        onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') mkdir(); }} />
      <button class="btn-ghost text-xs" onclick={mkdir}>新建文件夹</button>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <input type="file" bind:this={fileInput} class="text-xs max-w-full" style="color: var(--color-text)" />
      <input type="text" class="input-field text-xs flex-1 min-w-[120px]" placeholder="目标路径（留空使用当前目录）" bind:value={uploadTarget} />
      <button class="btn-primary text-xs" disabled={uploading} onclick={uploadFile}>
        {uploading ? '上传中...' : '上传'}
      </button>
    </div>
  </div>
  <!-- Drag & Drop Zone -->
  <div
    class="relative"
    role="presentation"
    ondragover={handleDragOver}
    ondragleave={handleDragLeave}
    ondrop={handleDrop}
  >
    {#if dragOver}
      <div class="absolute inset-0 z-10 flex items-center justify-center rounded-lg" style="background: color-mix(in srgb, var(--color-primary) 10%, transparent); border: 2px dashed var(--color-primary)">
        <span class="text-sm font-medium" style="color: var(--color-primary)">释放文件以上传</span>
      </div>
    {/if}
  <div class="max-h-[500px] overflow-y-auto">
    {#each filteredFiles as file (file.path)}
      <div class="flex flex-wrap items-center px-4 py-2 text-sm gap-y-1.5" style="border-bottom: 1px solid var(--color-border)">
        <span class="material-symbols-outlined text-[18px] mr-2" style="color: {file.is_dir ? 'var(--color-primary)' : 'var(--color-text-muted)'}">
          {file.is_dir ? 'folder' : 'description'}
        </span>
        {#if renamePath === file.path}
          <input type="text" class="input-field text-xs flex-1 min-w-[120px]" bind:value={renameTarget}
            onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') renameFile(file.path); if (e.key === 'Escape') { renamePath = ''; renameTarget = ''; } }}
            onblur={() => { if (renameTarget !== file.name) renameFile(file.path); else { renamePath = ''; renameTarget = ''; } }} />
        {:else}
          <button
            class="flex-1 min-w-0 text-left truncate"
            style="color: {file.is_dir ? 'var(--color-primary)' : 'var(--color-text)'}"
            onclick={() => file.is_dir ? navigateTo(file.path + '/') : null}
          >
            {file.name}
          </button>
        {/if}
        <span class="text-xs ml-4" style="color: var(--color-text-muted)">{file.is_dir ? '' : formatSize(file.size)}</span>
        <span class="text-xs ml-4 hidden sm:inline" style="color: var(--color-text-muted)">{file.mode}</span>
        <div class="flex items-center gap-2 ml-auto">
          <button class="text-xs" style="color: var(--color-text-secondary)" onclick={() => { renamePath = file.path; renameTarget = file.name; }}>重命名</button>
          {#if !file.is_dir}
            <button class="text-xs" style="color: var(--color-primary)" onclick={() => downloadFile(file.path)}>下载</button>
            <button class="text-xs" style="color: var(--color-text-secondary)" onclick={() => previewFile(file.path)}>预览</button>
          {/if}
          <button class="text-xs" style="color: var(--color-error)" onclick={() => onConfirm('删除文件', `确定要删除 ${file.name} 吗？此操作不可撤销。`, 'danger', () => deleteFile(file.path))}>删除</button>
        </div>
      </div>
    {/each}
    {#if files.length === 0}
      <div class="text-center py-12" style="color: var(--color-text-muted)">空目录</div>
    {/if}
  </div>
  </div>
  <!-- Preview Modal -->
  {#if previewPath}
    <div class="border-t" style="border-color: var(--color-border)">
      <div class="flex items-center justify-between px-4 py-2 bg-surface" style="border-bottom: 1px solid var(--color-border)">
        <span class="text-xs font-semibold" style="color: var(--color-text)">预览: {previewPath}</span>
        <button class="text-xs" style="color: var(--color-text-secondary)" onclick={closePreview}>关闭</button>
      </div>
      <pre class="p-4 text-xs overflow-auto max-h-[300px] font-mono" style="background: var(--color-surface); color: var(--color-text); white-space: pre-wrap; word-break: break-all">{previewContent}</pre>
    </div>
  {/if}
</div>

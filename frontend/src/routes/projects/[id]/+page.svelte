<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '../../../lib/api/client';
  import CodeEditor from '../../../lib/components/CodeEditor.svelte';
  import PreviewPanel from '../../../lib/components/PreviewPanel.svelte';
  import TeamManager from '../../../lib/components/TeamManager.svelte';
  import MemoryPanel from '../../../lib/components/MemoryPanel.svelte';

  const id = window.location.pathname.split('/').filter(Boolean).pop() || '';

  let files = $state<{path: string; content?: string}[]>([]);
  let selectedFile = $state<string | null>(null);
  let editorContent = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let project = $state<any>(null);
  let activeTab = $state<'editor' | 'preview' | 'versions' | 'dependencies' | 'security' | 'git'>('editor');

  // Repo tracking
  let repoUrl = $state('');
  let repoInfo = $state<any>(null);
  let repoFiles = $state<any[]>([]);
  let showRepoDialog = $state(false);
  let repoLoading = $state(false);

  // Templates
  let templates = $state<any[]>([]);
  let showTemplatePanel = $state(false);
  let templateSearch = $state('');

  // Translation
  let translateLang = $state('en');
  let originalDesc = $state('');
  let translatedDesc = $state('');
  let translating = $state(false);

  // AI Chat
  let chatOpen = $state(false);
  let chatMessages = $state<Array<{role: string, content: string}>>([]);
  let chatInput = $state('');
  let chatStreaming = $state(false);

  // Build log
  let showBuildLog = $state(false);
  let buildLogs = $state<Array<{timestamp: string, level: string, message: string}>>([]);
  let buildId = $state('');

  // Git version management
  let showGitPanel = $state(false);
  let gitCommits = $state<Array<{hash: string, message: string, author: string, timestamp: string}>>([]);
  let gitHeadHash = $state('');
  let gitMessage = $state('');
  let gitLoading = $state(false);

  // Git branches
  let gitBranches = $state<Array<{name: string, is_current: boolean, hash: string}>>([]);
  let gitCurrentBranch = $state('main');
  let newBranchName = $state('');
  let gitRemote = $state('origin');
  let gitBranchLoading = $state(false);
  let gitPushLoading = $state(false);
  let gitPullLoading = $state(false);
  let gitPushOutput = $state('');
  let gitPullOutput = $state('');

  // ADB device panel
  let showADBPanels = $state(false);
  let adbDevices = $state<Array<{serial: string, model: string, state: string}>>([]);
  let adbAvailable = $state<boolean | null>(null);
  let adbChecking = $state(false);
  let adbLoadingDevices = $state(false);
  let adbOutput = $state('');

  // Screenshot panel
  let showScreenshotPanel = $state(false);
  let screenshotDevice = $state('');
  let screenshotLoading = $state(false);
  let screenshotImages = $state<Array<{filename: string, path: string}>>([]);
  let screenshotStreaming = $state(false);

  // Module signature
  let signatureInfo = $state<{hash: string, size: number, signed_at: string, algorithm: string} | null>(null);
  let signing = $state(false);
  let verifying = $state(false);
  let verifyResult = $state<{valid: boolean, message?: string, signed?: boolean, error?: string} | null>(null);

  // Security: Vulnerability scanning
  let vulnScanning = $state(false);
  let vulnResults = $state<any>(null);
  let vulnHistory = $state<any[]>([]);

  // Security: Permission audit
  let permAuditing = $state(false);
  let permResults = $state<any>(null);
  let permHistory = $state<any[]>([]);

  // Security: Signature from new endpoint
  let sigInfo = $state<any>(null);

  // Validation
  let validationResults = $state<Array<{file: string, valid: boolean, errors: string[], warnings: string[]}>>([]);
  let validating = $state(false);
  let showValidation = $state(false);

  // Mirror (MJPEG screen casting)
  let showMirrorPanel = $state(false);
  let mirroring = $state(false);
  let mirrorFPS = $state(3);
  let mirrorDevice = $state('');
  let mirrorURL = $state('');
  let mirrorAspect = $state<'contain' | 'cover' | 'stretch'>('contain');

  // Update check
  let showUpdatePanel = $state(false);
  let updateChecking = $state(false);
  let updateResult = $state<any>(null);
  let updateModuleVersion = $state('');
  let updateModuleRepo = $state('');

  // Benchmark
  let showBenchmarkPanel = $state(false);
  let benchmarkRunning = $state(false);
  let benchmarkResult = $state<any>(null);
  let benchmarkDevice = $state('');
  let benchmarkHistory = $state<any[]>([]);

  // ZIP export
  let exporting = $state(false);

  // Collaboration
  let showCollabPanel = $state(false);
  let collaborators = $state<Array<{id: string, user_id: string, username: string, role: string, invited_at: string}>>([]);
  let collabComments = $state<Array<{id: string, user_id: string, username: string, file_path: string, line_number: number, content: string, resolved: boolean, created_at: string}>>([]);
  let collabSessions = $state<Array<{id: string, user_id: string, username: string, file_path: string, cursor_line: number, cursor_col: number, color: string}>>([]);
  let collabUsername = $state('');
  let collabInviteUser = $state('');
  let collabInviteRole = $state('editor');
  let commentFilePath = $state('');
  let commentLineNumber = $state(0);
  let commentContent = $state('');
  let collabWsConnected = $state(false);
  const COLLAB_COLORS = ['#e53935','#1e88e5','#43a047','#fb8c00','#8e24aa','#00acc1','#6d4c41','#546e7a'];
  let myCollabColor = $state(COLLAB_COLORS[Math.floor(Math.random() * COLLAB_COLORS.length)]);

  // Team management
  let showTeamPanel = $state(false);

  // Plugin system
  let showPluginPanel = $state(false);
  let pluginList = $state<Array<{id: string, name: string, slug: string, description: string, author: string, version: string, enabled: boolean}>>([]);
  let activities = $state<any[]>([]);

  // Feature 1: Version Management
  let projectVersions = $state<Array<{id: number, project_id: string, version: string, changelog: string, file_count: number, total_size: number, file_hash: string, created_at: string}>>([]);
  let versionsLoading = $state(false);
  let showCreateVersionDialog = $state(false);
  let newVersionNumber = $state('');
  let newVersionChangelog = $state('');
  let creatingVersion = $state(false);
  let versionDiffFrom = $state('');
  let versionDiffTo = $state('');
  let versionDiffResult = $state<any>(null);
  let diffingVersions = $state(false);

  // Feature 3: Dependency Resolution
  let depAnalysis = $state<any>(null);
  let depTree = $state<any>(null);
  let depsLoading = $state(false);
  let resolvingDeps = $state(false);

  async function loadActivities() {
    try {
      const res = await fetch(`/api/v1/projects/${id}/activities?limit=10`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        const data = await res.json();
        activities = data.activities || [];
      }
    } catch {}
  }

  // Feature 1: Version Management functions
  async function loadVersions() {
    versionsLoading = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${id}/versions`, {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) {
        const data = await res.json();
        projectVersions = data.versions || [];
      }
    } catch {}
    versionsLoading = false;
  }

  async function createVersion() {
    if (!newVersionNumber.trim()) return;
    creatingVersion = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${id}/versions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ version: newVersionNumber, changelog: newVersionChangelog }),
      });
      if (res.ok) {
        showCreateVersionDialog = false;
        newVersionNumber = '';
        newVersionChangelog = '';
        await loadVersions();
      } else {
        const err = await res.json();
        alert(err.error || '创建版本失败');
      }
    } catch {}
    creatingVersion = false;
  }

  async function rollbackVersion(version: string) {
    if (!confirm(`确定要回滚到版本 ${version} 吗？当前未保存的更改将丢失。`)) return;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${id}/versions/${version}/rollback`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) {
        const data = await res.json();
        alert(`回滚成功，已恢复 ${data.files_restored} 个文件`);
        // Reload files
        window.location.reload();
      } else {
        const err = await res.json();
        alert(err.error || '回滚失败');
      }
    } catch {}
  }

  async function diffVersions() {
    if (!versionDiffFrom || !versionDiffTo) return;
    diffingVersions = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${id}/versions/diff?from=${versionDiffFrom}&to=${versionDiffTo}`, {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) {
        versionDiffResult = await res.json();
      } else {
        const err = await res.json();
        alert(err.error || '对比失败');
      }
    } catch {}
    diffingVersions = false;
  }

  // Feature 3: Dependency Resolution functions
  async function analyzeDependencies() {
    depsLoading = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${id}/analyze-deps`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({}),
      });
      if (res.ok) {
        depAnalysis = await res.json();
      }
    } catch {}
    depsLoading = false;
  }

  async function loadDependencyTree() {
    depsLoading = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${id}/dependencies`, {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (res.ok) {
        depTree = await res.json();
      }
    } catch {}
    depsLoading = false;
  }

  async function resolveDependencies() {
    resolvingDeps = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch(`/api/v1/projects/${id}/resolve-deps`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ auto_install: false }),
      });
      if (res.ok) {
        depAnalysis = await res.json();
      }
    } catch {}
    resolvingDeps = false;
  }
  let pluginInstallName = $state('');
  let pluginInstallSlug = $state('');
  let pluginInstallDesc = $state('');
  let pluginInstallAuthor = $state('');
  let pluginInstallVersion = $state('1.0.0');
  let pluginHookName = $state('');
  let pluginHookType = $state('pre_save');
  let pluginHookEntry = $state('');
  let selectedPluginId = $state('');

  async function validateCurrentFile() {
    if (!selectedFile) return;
    validating = true;
    showValidation = true;
    try {
      const res = await fetch('/api/v1/validate/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filename: selectedFile, content: editorContent })
      });
      if (res.ok) {
        const data = await res.json();
        validationResults = [data];
      }
    } catch {
      validationResults = [{ file: selectedFile || '', valid: false, errors: ['Validation service unavailable'], warnings: [] }];
    }
    validating = false;
  }

  async function validateAllFiles() {
    validating = true;
    showValidation = true;
    const filesMap: Record<string, string> = {};
    for (const f of files) {
      if (f.content) {
        filesMap[f.path] = f.content;
      } else if (f.path === selectedFile) {
        filesMap[f.path] = editorContent;
      }
    }
    try {
      const res = await fetch('/api/v1/validate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files: filesMap })
      });
      if (res.ok) {
        const data = await res.json();
        validationResults = data.results || [];
      }
    } catch {
      validationResults = [{ file: 'all', valid: false, errors: ['Validation service unavailable'], warnings: [] }];
    }
    validating = false;
  }

  // Module ZIP import
  interface ModuleInfo {
    id: string; name: string; version: string; versionCode: string;
    author: string; description: string;
    ksu_supported: boolean; apatch_supported: boolean;
  }
  interface ZipFileInfo {
    path: string; size: number; is_dir: boolean;
  }
  interface ZipFileContent {
    path: string; content: string; is_dir: boolean;
  }
  let showZipImportDialog = $state(false);
  let zipImportLoading = $state(false);
  let parsedModule = $state<ModuleInfo | null>(null);
  let parsedFiles = $state<ZipFileInfo[]>([]);
  let zipImporting = $state(false);
  let zipImportError = $state('');
  let zipFileRef: HTMLInputElement | undefined = $state();
  let zipImportFile: File | null = $state(null);

  function handleZipImportFile() {
    const file = zipFileRef?.files?.[0];
    if (!file) return;
    zipImportFile = file;
    zipImportLoading = true;
    zipImportError = '';
    parsedModule = null;
    parsedFiles = [];
    const formData = new FormData();
    formData.append('file', file);
    fetch('/api/v1/module/parse-zip', {
      method: 'POST',
      body: formData,
    }).then(async res => {
      if (res.ok) {
        const data = await res.json();
        parsedModule = data.module;
        parsedFiles = data.files || [];
        showZipImportDialog = true;
      } else {
        const err = await res.json().catch(() => ({ error: '解析失败' }));
        zipImportError = err.error || '解析 ZIP 文件失败';
      }
    }).catch(() => {
      zipImportError = '无法连接到服务器';
    }).finally(() => {
      zipImportLoading = false;
    });
  }

  async function confirmZipImport() {
    if (!parsedModule || !parsedModule.id) {
      zipImportError = '无效的模块文件（缺少 module.prop）';
      return;
    }
    zipImporting = true;
    zipImportError = '';
    const file = zipImportFile;
    if (!file) {
      zipImportError = '文件已丢失，请重新选择';
      zipImporting = false;
      return;
    }
    const formData = new FormData();
    formData.append('file', file);
    try {
      const res = await fetch('/api/v1/module/import-zip', {
        method: 'POST',
        body: formData,
      });
      if (res.ok) {
        const data = await res.json();
        const fileContents: ZipFileContent[] = data.files || [];
        let imported = 0; let failed = 0;
        for (const fc of fileContents) {
          if (fc.is_dir) continue;
          try {
            const saveRes = await fetch(`/api/v1/projects/${id}/files/${encodeURIComponent(fc.path)}`, {
              method: 'PUT',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ content: fc.content }),
            });
            if (saveRes.ok) imported++; else failed++;
          } catch { failed++; }
        }
        alert(`导入完成：${imported} 个文件成功，${failed} 个失败`);
        showZipImportDialog = false;
        const refreshed = await client.get<{path: string}[]>(`/projects/${id}/files`);
        files = refreshed.map(f => ({ path: f.path }));
        if (refreshed.length > 0) selectFile(refreshed[0].path);
      } else {
        const err = await res.json().catch(() => ({ error: '导入失败' }));
        zipImportError = err.error || '导入失败';
      }
    } catch {
      zipImportError = '无法连接到服务器';
    }
    zipImporting = false;
  }

  async function exportZip() {
    exporting = true;
    const zipFiles = [];
    for (const f of files) {
      let content = f.content || '';
      if (f.path === selectedFile) {
        content = editorContent;
      }
      if (!content && f.path !== selectedFile) {
        try {
          const fileData = await client.get<{path: string; content: string}>(`/projects/${id}/files/${encodeURIComponent(f.path)}`);
          content = fileData.content;
        } catch { /* skip */ }
      }
      zipFiles.push({ path: f.path, content });
    }

    try {
      const res = await fetch('/api/v1/build/zip', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_id: id, files: zipFiles })
      });

      if (res.ok) {
        const blob = await res.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `moduforge_module_${id}.zip`;
        a.click();
        window.URL.revokeObjectURL(url);
      } else {
        alert('Failed to build ZIP');
      }
    } catch {
      alert('Export service unavailable');
    }
    exporting = false;
  }

  function generatePreviewContent(): string {
    const htmlFile = files.find(f => f.path.endsWith('.html'));
    if (htmlFile) {
      return `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body>
  ${editorContent}
</body>
</html>`;
    }
    return editorContent;
  }

  function detectLanguage(path: string): string {
    const ext = path.split('.').pop()?.toLowerCase() || '';
    const langMap: Record<string, string> = {
      'js': 'javascript',
      'jsx': 'javascript',
      'ts': 'javascript',
      'tsx': 'javascript',
      'py': 'python',
      'html': 'html',
      'htm': 'html',
      'css': 'css',
      'scss': 'css',
      'less': 'css',
      'json': 'json',
      'xml': 'xml',
      'yaml': 'json',
      'yml': 'json',
      'toml': 'json',
      'sh': 'shell',
      'bash': 'shell',
      'go': 'javascript',
      'vue': 'javascript',
      'svelte': 'javascript',
    };
    return langMap[ext] || 'javascript';
  }

  function getFileIcon(path: string): string {
    const ext = path.split('.').pop()?.toLowerCase() || '';
    const iconMap: Record<string, string> = {
      'png': '🖼️',
      'jpg': '🖼️',
      'jpeg': '🖼️',
      'ico': '🖼️',
      'gif': '🖼️',
      'svg': '🖼️',
      'txt': '📝',
      'md': '📝',
      'json': '⚙️',
      'yaml': '⚙️',
      'yml': '⚙️',
      'toml': '⚙️',
      'xml': '⚙️',
      'js': '🧩',
      'ts': '🧩',
      'jsx': '🧩',
      'tsx': '🧩',
      'svelte': '🧩',
      'vue': '🧩',
      'css': '🎨',
      'scss': '🎨',
      'less': '🎨',
      'go': '🐹',
      'py': '🐍',
      'sh': '🐚',
      'bash': '🐚',
    };
    return iconMap[ext] || '📄';
  }

  // Mock data for demo
  const mockRepoInfo = {
    owner: 'Magisk-Modules-Repo',
    name: 'systemless-hosts',
    stars: 1250,
    topics: ['magisk', 'hosts', 'adblock'],
    license: 'GPL-3.0',
    fetched_at: new Date().toISOString()
  };

  const mockRepoFiles = [
    { name: 'module.prop', type: 'file', path: 'module.prop' },
    { name: 'system', type: 'dir', path: 'system' },
    { name: 'customize.sh', type: 'file', path: 'customize.sh' },
    { name: 'META-INF', type: 'dir', path: 'META-INF' }
  ];

  async function fetchRepoInfo() {
    repoLoading = true;
    try {
      // Try real API first, fallback to mock
      const response = await fetch('/api/v1/repo/fetch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: repoUrl })
      });
      if (response.ok) {
        repoInfo = await response.json();
      } else {
        repoInfo = mockRepoInfo;
      }
    } catch {
      repoInfo = mockRepoInfo;
    }

    try {
      const filesResponse = await fetch('/api/v1/repo/files', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: repoUrl, path: '' })
      });
      if (filesResponse.ok) {
        repoFiles = await filesResponse.json();
      } else {
        repoFiles = mockRepoFiles;
      }
    } catch {
      repoFiles = mockRepoFiles;
    }
    repoLoading = false;
  }

  async function loadTemplates() {
    try {
      const response = await fetch('/api/v1/templates/list');
      if (response.ok) {
        const data = await response.json();
        templates = Array.isArray(data) ? data : [];
      }
    } catch {
      // Use default templates
      templates = [
        { name: 'system.prop', description: '通过 system.prop 修改系统属性的 Magisk/KSU 模块', category: 'system' },
        { name: 'boot_animation', description: '自定义开机动画的 Magisk 模块', category: 'ui' },
        { name: 'audio_tweaks', description: '音频参数优化的 Magisk/KSU 模块', category: 'module' }
      ];
    }
  }

  function applyTemplate(tmpl: any) {
    editorContent = tmpl.files?.map((f: any) => `# ${f.path}\n${f.content}`).join('\n\n') || `# ${tmpl.name}\n# ${tmpl.description}`;
    showTemplatePanel = false;
  }

  async function translateDescription() {
    if (!originalDesc) return;
    translating = true;
    try {
      const response = await fetch('/api/v1/translate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: originalDesc, target_lang: translateLang })
      });
      if (response.ok) {
        const data = await response.json();
        translatedDesc = data.translated_text;
      } else {
        translatedDesc = `[${translateLang}] ${originalDesc}`;
      }
    } catch {
      translatedDesc = `[${translateLang}] ${originalDesc}`;
    }
    translating = false;
  }

  async function sendChatMessage() {
    if (!chatInput.trim() || chatStreaming) return;
    const msg = chatInput;
    chatMessages = [...chatMessages, { role: 'user', content: msg }];
    chatInput = '';
    chatStreaming = true;

    // Add placeholder for AI response
    chatMessages = [...chatMessages, { role: 'assistant', content: '' }];

    try {
      const response = await fetch('/api/v1/ai/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prompt: msg }),
      });

      if (!response.ok) {
        chatMessages[chatMessages.length - 1].content = 'AI 服务暂时不可用，请稍后再试。';
        chatMessages = chatMessages;
        chatStreaming = false;
        return;
      }

      const reader = response.body?.getReader();
      if (!reader) throw new Error('No reader');

      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6));
              if (data.type === 'delta') {
                chatMessages[chatMessages.length - 1].content += data.content;
                chatMessages = chatMessages;
              }
              if (data.type === 'done') {
                chatStreaming = false;
                chatMessages = chatMessages;
              }
            } catch {}
          }
        }
      }
    } catch {
      chatMessages[chatMessages.length - 1].content = '连接错误，请稍后再试。';
      chatMessages = chatMessages;
    }
    chatStreaming = false;
  }

  function handleChatKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendChatMessage();
    }
  }

  function loadBuildLogs() {
    if (!buildId) return;
    buildLogs = [];
    const eventSource = new EventSource(`/api/v1/build/log?build_id=${buildId}`);
    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        buildLogs = [...buildLogs, data];
      } catch {}
    };
    eventSource.onerror = () => {
      eventSource.close();
    };
  }

  async function loadGitCommits() {
    gitLoading = true;
    try {
      const res = await fetch(`/api/v1/git/commits?project_id=${id}&limit=20`);
      if (res.ok) {
        gitCommits = await res.json();
      }
      const headRes = await fetch(`/api/v1/git/head?project_id=${id}`);
      if (headRes.ok) {
        const head = await headRes.json();
        gitHeadHash = head.hash;
      }
    } catch {}
    gitLoading = false;
  }

  async function saveGitCommit() {
    if (!gitMessage.trim()) return;
    gitLoading = true;
    try {
      const res = await fetch('/api/v1/git/commit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_id: id, message: gitMessage })
      });
      if (res.ok) {
        gitMessage = '';
        await loadGitCommits();
      }
    } catch {}
    gitLoading = false;
  }

  async function gitCheckout(hash: string) {
    try {
      const res = await fetch('/api/v1/git/checkout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_id: id, hash })
      });
      if (res.ok) {
        await loadGitCommits();
      }
    } catch {}
  }

  async function loadGitBranches() {
    gitBranchLoading = true;
    try {
      const res = await fetch(`/api/v1/git/branches?project_id=${id}`);
      if (res.ok) {
        const data = await res.json();
        gitBranches = data.branches || [];
        const current = gitBranches.find((b: any) => b.is_current);
        if (current) gitCurrentBranch = current.name;
      }
    } catch {}
    gitBranchLoading = false;
  }

  async function loadGitCurrentBranch() {
    try {
      const res = await fetch(`/api/v1/git/branch?project_id=${id}`);
      if (res.ok) {
        const data = await res.json();
        gitCurrentBranch = data.branch || 'main';
      }
    } catch {}
  }

  async function createGitBranch() {
    if (!newBranchName.trim()) return;
    gitBranchLoading = true;
    try {
      const res = await fetch('/api/v1/git/branch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_id: id, branch_name: newBranchName })
      });
      if (res.ok) {
        newBranchName = '';
        await loadGitBranches();
      }
    } catch {}
    gitBranchLoading = false;
  }

  async function switchGitBranch(branchName: string) {
    gitBranchLoading = true;
    try {
      const res = await fetch('/api/v1/git/checkout-branch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_id: id, branch_name: branchName })
      });
      if (res.ok) {
        await loadGitBranches();
        await loadGitCommits();
      }
    } catch {}
    gitBranchLoading = false;
  }

  async function pushGit() {
    gitPushLoading = true;
    gitPushOutput = '推送中...';
    try {
      const res = await fetch('/api/v1/git/push', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_id: id, remote: gitRemote })
      });
      const data = await res.json();
      if (res.ok) {
        gitPushOutput = data.output || '推送成功';
      } else {
        gitPushOutput = data.error || '推送失败';
      }
    } catch (e: any) {
      gitPushOutput = e.message || '推送失败';
    }
    gitPushLoading = false;
  }

  async function pullGit() {
    gitPullLoading = true;
    gitPullOutput = '拉取中...';
    try {
      const res = await fetch('/api/v1/git/pull', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_id: id, remote: gitRemote })
      });
      const data = await res.json();
      if (res.ok) {
        gitPullOutput = data.output || '拉取成功';
        await loadGitCommits();
      } else {
        gitPullOutput = data.error || '拉取失败';
      }
    } catch (e: any) {
      gitPullOutput = e.message || '拉取失败';
    }
    gitPullLoading = false;
  }

  async function checkADB() {
    adbChecking = true;
    try {
      const res = await fetch('/api/v1/adb/check');
      if (res.ok) {
        const data = await res.json();
        adbAvailable = data.available;
        if (data.available) {
          await loadADBDevices();
        }
      }
    } catch {
      adbAvailable = false;
    }
    adbChecking = false;
  }

  async function loadADBDevices() {
    adbLoadingDevices = true;
    try {
      const res = await fetch('/api/v1/adb/devices');
      if (res.ok) {
        adbDevices = await res.json();
      }
    } catch {}
    adbLoadingDevices = false;
  }

  async function installModule(serial: string) {
    adbOutput = 'Installing module...';
    try {
      const res = await fetch('/api/v1/adb/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ serial })
      });
      if (res.ok) {
        const data = await res.json();
        adbOutput = data.output || 'Done';
      } else {
        const data = await res.json();
        adbOutput = data.error || 'Install failed';
      }
    } catch {
      adbOutput = 'Service unavailable';
    }
  }

  async function rebootDevice(serial: string) {
    try {
      await fetch('/api/v1/adb/reboot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ serial })
      });
      adbOutput = 'Rebooting...';
    } catch {}
  }

  async function takeScreenshot() {
    if (!screenshotDevice) return;
    screenshotLoading = true;
    try {
      const token = localStorage.getItem('moduforge_token');
      const res = await fetch(`/api/v1/adb/screenshot?serial=${encodeURIComponent(screenshotDevice)}`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {}
      });
      if (res.ok) {
        const data = await res.json();
        screenshotImages = [{ filename: data.filename, path: data.path }, ...screenshotImages];
      } else {
        const data = await res.json();
        adbOutput = data.error || 'Screenshot failed';
      }
    } catch {
      adbOutput = 'Screenshot service unavailable';
    }
    screenshotLoading = false;
  }

  async function streamScreenshots() {
    if (!screenshotDevice || screenshotStreaming) return;
    screenshotStreaming = true;
    try {
      const token = localStorage.getItem('moduforge_token');
      const res = await fetch(`/api/v1/adb/screenshot/stream?serial=${encodeURIComponent(screenshotDevice)}`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {}
      });
      if (!res.ok || !res.body) {
        screenshotStreaming = false;
        return;
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6));
              if (data.filename) {
                screenshotImages = [{ filename: data.filename, path: `data/screenshots/${data.filename}` }, ...screenshotImages];
              }
              if (data.done) screenshotStreaming = false;
              if (data.error) { adbOutput = data.error; screenshotStreaming = false; }
            } catch {}
          }
        }
      }
    } catch {
      adbOutput = 'Stream unavailable';
    }
    screenshotStreaming = false;
  }

  async function signModule() {
    signing = true;
    verifyResult = null;
    try {
      const res = await fetch(`/api/v1/projects/${id}/sign`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}`,
        },
      });
      if (res.ok) {
        sigInfo = await res.json();
        signatureInfo = { hash: sigInfo.file_hash, size: 0, signed_at: sigInfo.signed_at, algorithm: sigInfo.algorithm };
      } else {
        const data = await res.json();
        alert(data.error || '签名失败');
      }
    } catch {
      alert('签名服务不可用');
    }
    signing = false;
  }

  async function verifyModule() {
    verifying = true;
    try {
      const res = await fetch(`/api/v1/projects/${id}/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}`,
        },
      });
      if (res.ok) {
        verifyResult = await res.json();
      }
    } catch {
      verifyResult = { valid: false };
    }
    verifying = false;
  }

  async function loadSignature() {
    try {
      const res = await fetch(`/api/v1/projects/${id}/signature`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) sigInfo = await res.json();
    } catch {}
  }

  // Vulnerability scanning
  async function scanVulnerabilities() {
    vulnScanning = true;
    try {
      const res = await fetch(`/api/v1/projects/${id}/scan-vulns`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}`,
        },
      });
      if (res.ok) vulnResults = await res.json();
    } catch {}
    vulnScanning = false;
  }

  async function loadVulnHistory() {
    try {
      const res = await fetch(`/api/v1/projects/${id}/vulnerabilities`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        const data = await res.json();
        vulnHistory = data.scans || [];
      }
    } catch {}
  }

  // Permission audit
  async function auditPermissions() {
    permAuditing = true;
    try {
      const res = await fetch(`/api/v1/projects/${id}/audit`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}`,
        },
      });
      if (res.ok) permResults = await res.json();
    } catch {}
    permAuditing = false;
  }

  async function loadPermHistory() {
    try {
      const res = await fetch(`/api/v1/projects/${id}/permissions`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` },
      });
      if (res.ok) {
        const data = await res.json();
        permHistory = data.audits || [];
      }
    } catch {}
  }

  // Mirror functions
  function startMirror() {
    if (!mirrorDevice) return;
    mirrorURL = `/api/v1/adb/mirror?serial=${mirrorDevice}&fps=${mirrorFPS}`;
    mirroring = true;
  }

  function stopMirror() {
    mirroring = false;
    mirrorURL = '';
  }

  function captureMirrorFrame() {
    const img = document.querySelector('.mirror-container img') as HTMLImageElement;
    if (!img) return;
    const canvas = document.createElement('canvas');
    canvas.width = img.naturalWidth || img.width;
    canvas.height = img.naturalHeight || img.height;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.drawImage(img, 0, 0);
    canvas.toBlob((blob) => {
      if (!blob) return;
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `mirror_capture_${Date.now()}.png`;
      a.click();
      URL.revokeObjectURL(url);
    });
  }

  // Update check functions
  async function checkModuleUpdate() {
    updateChecking = true;
    updateResult = null;
    try {
      const res = await fetch('/api/v1/update/check', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          module_id: id,
          current_version: updateModuleVersion || 'v1.0',
          repo_url: updateModuleRepo
        })
      });
      if (res.ok) {
        updateResult = await res.json();
      } else {
        updateResult = { has_update: false, error: 'Check failed' };
      }
    } catch {
      updateResult = { has_update: false, error: 'Service unavailable' };
    }
    updateChecking = false;
  }

  // Benchmark functions
  async function runBenchmark() {
    if (!benchmarkDevice) return;
    benchmarkRunning = true;
    benchmarkResult = null;
    try {
      const res = await fetch('/api/v1/benchmark/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ module_id: id, serial: benchmarkDevice })
      });
      if (res.ok) {
        benchmarkResult = await res.json();
      } else {
        const data = await res.json();
        adbOutput = data.error || 'Benchmark failed';
      }
    } catch {
      adbOutput = 'Benchmark service unavailable';
    }
    benchmarkRunning = false;
  }

  async function loadBenchmarkHistory() {
    try {
      const res = await fetch(`/api/v1/benchmark/history?module_id=${id}&limit=10`);
      if (res.ok) {
        const data = await res.json();
        benchmarkHistory = data.results || [];
      }
    } catch {}
  }

  // ===== Collaboration functions =====
  async function loadCollaborators() {
    try {
      const res = await client.get<{collaborators: any[]}>(`/projects/${id}/collaborators`);
      collaborators = res.collaborators || [];
    } catch { /* ignore */ }
  }

  async function inviteCollaborator() {
    if (!collabInviteUser.trim()) return;
    try {
      await client.post(`/projects/${id}/collaborators`, { user_id: collabInviteUser, role: collabInviteRole });
      collabInviteUser = '';
      await loadCollaborators();
    } catch { /* ignore */ }
  }

  async function removeCollaborator(userId: string) {
    try {
      await client.del(`/projects/${id}/collaborators/${userId}`);
      await loadCollaborators();
    } catch { /* ignore */ }
  }

  async function loadCollabComments() {
    try {
      const res = await client.get<{comments: any[]}>(`/projects/${id}/comments`);
      collabComments = res.comments || [];
    } catch { /* ignore */ }
  }

  async function addCollabComment() {
    if (!commentContent.trim()) return;
    try {
      await client.post(`/projects/${id}/comments`, {
        file_path: commentFilePath || selectedFile || '',
        line_number: commentLineNumber,
        content: commentContent,
        user_id: '',
        username: collabUsername || 'Anonymous'
      });
      commentContent = '';
      await loadCollabComments();
    } catch { /* ignore */ }
  }

  async function resolveComment(commentId: string) {
    try {
      await client.post(`/comments/${commentId}/resolve`);
      await loadCollabComments();
    } catch { /* ignore */ }
  }

  async function loadEditSessions() {
    try {
      const res = await client.get<{sessions: any[]}>(`/projects/${id}/edit-sessions`);
      collabSessions = res.sessions || [];
    } catch { /* ignore */ }
  }

  function sendCollabCursor(line: number, col: number) {
    if (!collabWsConnected) return;
    try {
      const ws = (window as any).__moduforge_ws;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          type: 'collab_cursor_update',
          payload: {
            project_id: id,
            user_id: '',
            username: collabUsername || 'Anonymous',
            file: selectedFile || '',
            line, col,
            color: myCollabColor
          }
        }));
      }
    } catch { /* ignore */ }
  }

  function openCollabPanel() {
    showCollabPanel = true;
    loadCollaborators();
    loadCollabComments();
    loadEditSessions();
    connectCollabWs();
  }

  function connectCollabWs() {
    if ((window as any).__moduforge_ws) return;
    const token = localStorage.getItem('moduforge_token');
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws?uid=${Date.now()}&project_id=${id}${token ? '&token=' + token : ''}`;
    const ws = new WebSocket(wsUrl);
    (window as any).__moduforge_ws = ws;

    ws.onopen = () => { collabWsConnected = true; };
    ws.onclose = () => { collabWsConnected = false; (window as any).__moduforge_ws = null; };
    ws.onmessage = (event: MessageEvent) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'collab_cursor_update' && msg.payload?.file === selectedFile) {
          collabSessions = collabSessions.map(s =>
            s.user_id === msg.payload.user_id ? { ...s, cursor_line: msg.payload.line, cursor_col: msg.payload.col, file_path: msg.payload.file } : s
          );
        }
        if (msg.type === 'collab_join') {
          loadEditSessions();
        }
        if (msg.type === 'collab_leave') {
          collabSessions = (collabSessions || []).filter(s => s.user_id !== msg.payload?.user_id);
        }
      } catch { /* ignore */ }
    };
  }

  // ===== Plugin functions =====
  async function loadPlugins() {
    try {
      const res = await client.get<{plugins: any[]}>('/plugins');
      pluginList = res.plugins || [];
    } catch { /* ignore */ }
  }

  async function installPlugin() {
    if (!pluginInstallName.trim() || !pluginInstallSlug.trim()) return;
    try {
      await client.post('/plugins/install', {
        name: pluginInstallName,
        slug: pluginInstallSlug,
        description: pluginInstallDesc,
        author: pluginInstallAuthor,
        version: pluginInstallVersion,
      });
      pluginInstallName = '';
      pluginInstallSlug = '';
      pluginInstallDesc = '';
      pluginInstallAuthor = '';
      await loadPlugins();
    } catch { /* ignore */ }
  }

  async function togglePlugin(pluginId: string, enabled: boolean) {
    try {
      if (enabled) {
        await client.post(`/plugins/${pluginId}/enable`);
      } else {
        await client.post(`/plugins/${pluginId}/disable`);
      }
      await loadPlugins();
    } catch { /* ignore */ }
  }

  async function uninstallPlugin(pluginId: string) {
    try {
      await client.del(`/plugins/${pluginId}`);
      await loadPlugins();
    } catch { /* ignore */ }
  }

  async function registerHook() {
    if (!selectedPluginId || !pluginHookName.trim() || !pluginHookEntry.trim()) return;
    try {
      await client.post(`/plugins/${selectedPluginId}/hooks`, {
        hook_name: pluginHookName,
        hook_type: pluginHookType,
        entry_point: pluginHookEntry,
      });
      pluginHookName = '';
      pluginHookEntry = '';
    } catch { /* ignore */ }
  }

  onMount(async () => {
    if (!id) { loading = false; return; }
    try {
      project = await client.get(`/projects/${id}`);
      const fileData = await client.get<{path: string}[]>(`/projects/${id}/files`);
      files = (fileData || []).map(f => ({ path: f.path }));
    } catch(e) {
      console.error(e);
    } finally {
      loading = false;
    }
    loadActivities();
  });

  async function selectFile(path: string) {
    selectedFile = path;
    translatedDesc = '';
    originalDesc = '';
    try {
      const file = await client.get<{path: string; content: string}>(`/projects/${id}/files/${encodeURIComponent(path)}`);
      editorContent = file.content;
      // Extract description from module.prop
      if (path.endsWith('.prop')) {
        const match = file.content.match(/^description=(.+)$/m);
        if (match) originalDesc = match[1];
      }
    } catch(e) {
      console.error(e);
      editorContent = '';
    }
  }

  async function saveFile() {
    if (!selectedFile) return;
    saving = true;
    try {
      await client.put(`/projects/${id}/files/${encodeURIComponent(selectedFile)}`, { content: editorContent });
    } catch(e) {
      console.error(e);
    } finally {
      saving = false;
    }
  }
</script>

<div class="flex h-screen">
  <!-- 文件树 -->
  <aside class="w-64 bg-[var(--color-bg-elevated)] border-r border-[var(--color-border)] flex flex-col">
    <div class="p-4 border-b border-[var(--color-border)]">
      <h2 class="text-base font-semibold text-[var(--color-text)] truncate">{project?.name || '加载中...'}</h2>
    </div>

    <div class="flex-1 overflow-auto p-2">
      {#if loading}
        <div class="flex justify-center p-4"><div class="animate-spin h-5 w-5 border-2 border-primary-500 border-t-transparent rounded-full"></div></div>
      {:else if files.length === 0}
        <p class="text-xs text-[var(--color-text-secondary)] p-4">暂无文件</p>
      {:else}
        <div class="space-y-1">
          {#each files as file}
            <div class="flex items-center gap-2 p-2 rounded-xl hover:bg-[var(--color-surface)] cursor-pointer transition-colors cursor-pointer" style={selectedFile === file.path ? 'background: color-mix(in srgb, var(--color-primary) 20%, transparent)' : ''} onclick={() => selectFile(file.path)}
            >
              <span class="text-lg mr-2">{getFileIcon(file.path)}</span>
              <span>{file.path}</span>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <MemoryPanel projectId={id} />

    <div class="p-3 border-t border-[var(--color-border)] space-y-2">
      <button class="btn-primary w-full" onclick={() => { showRepoDialog = true; }}>
        <span class="material-symbols-outlined" slot="start">link</span>
        导入 GitHub 仓库
      </button>
      <button class="btn-primary w-full" onclick={() => { showTemplatePanel = true; loadTemplates(); }}>
        <span class="material-symbols-outlined" slot="start">description</span>
        模板推荐
      </button>
      <button class="btn-primary w-full" onclick={() => showBuildLog = true}>
        <span class="material-symbols-outlined" slot="start">terminal</span>
        构建日志
      </button>
      <button class="btn-primary w-full" href="/projects/{id}/build">
        <span class="material-symbols-outlined" slot="start">build</span>
        构建模块
      </button>
      <button class="btn-primary w-full" onclick={() => zipFileRef?.click()} disabled={zipImportLoading}>
        <span class="material-symbols-outlined" slot="start">file_upload</span>
        {zipImportLoading ? '解析中...' : '导入模块 ZIP'}
      </button>
      <input type="file" accept=".zip" class="hidden" bind:this={zipFileRef} onchange={handleZipImportFile} />
      <button class="btn-primary w-full" onclick={exportZip} disabled={exporting}>
        <span class="material-symbols-outlined" slot="start">archive</span>
        {exporting ? '打包中...' : '导出模块 ZIP'}
      </button>
      <button class="btn-primary w-full" onclick={signModule} disabled={signing}>
        <span class="material-symbols-outlined" slot="start">verified</span>
        {signing ? '签名中...' : '签名模块'}
      </button>
      <button class="btn-primary w-full" onclick={() => { showGitPanel = !showGitPanel; if (showGitPanel) loadGitCommits(); }}>
        <span class="material-symbols-outlined" slot="start">history</span>
        版本历史 ⏱
      </button>
      <button class="btn-primary w-full" onclick={() => { showADBPanels = !showADBPanels; if (showADBPanels && adbAvailable === null) checkADB(); }}>
        <span class="material-symbols-outlined" slot="start">phone_android</span>
        设备
      </button>
      <button class="btn-primary w-full" onclick={() => { showScreenshotPanel = !showScreenshotPanel; if (showScreenshotPanel && adbDevices.length === 0) checkADB(); }}>
        <span class="material-symbols-outlined" slot="start">photo_camera</span>
        真机截图
      </button>
      <button class="btn-primary w-full" onclick={() => { showMirrorPanel = !showMirrorPanel; if (showMirrorPanel && adbDevices.length === 0) checkADB(); }}>
        <span class="material-symbols-outlined" slot="start">screen_share</span>
        真机投屏
      </button>
      <button class="btn-primary w-full" onclick={() => showUpdatePanel = !showUpdatePanel}>
        <span class="material-symbols-outlined" slot="start">system_update</span>
        检查更新
      </button>
      <button class="btn-primary w-full" onclick={() => { showBenchmarkPanel = !showBenchmarkPanel; if (showBenchmarkPanel && adbDevices.length === 0) { checkADB(); loadBenchmarkHistory(); } }}>
        <span class="material-symbols-outlined" slot="start">speed</span>
        性能测试
      </button>
      <button class="btn-primary w-full" onclick={() => showTeamPanel = !showTeamPanel}>
        <span class="material-symbols-outlined" slot="start">groups</span>
        团队
      </button>
      <button class="btn-primary w-full" onclick={openCollabPanel}>
        <span class="material-symbols-outlined" slot="start">group</span>
        协作 👥
      </button>
      <button class="btn-primary w-full" onclick={() => showPluginPanel = !showPluginPanel}>
        <span class="material-symbols-outlined" slot="start">extension</span>
        插件
      </button>
    </div>
  </aside>

  <!-- 编辑器 -->
  <main class="flex-1 flex flex-col">
    {#if selectedFile}
      <!-- 标签页 -->
      <div class="flex items-center justify-between px-4 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg)]">
        <div class="flex items-center gap-4">
          <button
            class="px-3 py-1 text-sm transition-colors"
            class:text-primary={activeTab === 'editor'}
            class:text-[var(--color-text-secondary)]={activeTab !== 'editor'}
            class:border-b-2={activeTab === 'editor'}
            class:border-primary={activeTab === 'editor'}
            onclick={() => activeTab = 'editor'}
          >
            编辑
          </button>
          <button
            class="px-3 py-1 text-sm transition-colors"
            class:text-primary={activeTab === 'preview'}
            class:text-[var(--color-text-secondary)]={activeTab !== 'preview'}
            class:border-b-2={activeTab === 'preview'}
            class:border-primary={activeTab === 'preview'}
            onclick={() => activeTab = 'preview'}
          >
            预览
          </button>
          <button
            class="px-3 py-1 text-sm transition-colors"
            class:text-primary={activeTab === 'versions'}
            class:text-[var(--color-text-secondary)]={activeTab !== 'versions'}
            class:border-b-2={activeTab === 'versions'}
            class:border-primary={activeTab === 'versions'}
            onclick={() => { activeTab = 'versions'; loadVersions(); }}
          >
            版本
          </button>
          <button
            class="px-3 py-1 text-sm transition-colors"
            class:text-primary={activeTab === 'dependencies'}
            class:text-[var(--color-text-secondary)]={activeTab !== 'dependencies'}
            class:border-b-2={activeTab === 'dependencies'}
            class:border-primary={activeTab === 'dependencies'}
            onclick={() => { activeTab = 'dependencies'; analyzeDependencies(); loadDependencyTree(); }}
          >
            依赖
          </button>
          <button
            class="px-3 py-1 text-sm transition-colors"
            class:text-primary={activeTab === 'security'}
            class:text-[var(--color-text-secondary)]={activeTab !== 'security'}
            class:border-b-2={activeTab === 'security'}
            class:border-primary={activeTab === 'security'}
            onclick={() => { activeTab = 'security'; loadSignature(); loadVulnHistory(); loadPermHistory(); }}
          >
            🔒 安全
          </button>
          <button
            class="px-3 py-1 text-sm transition-colors"
            class:text-primary={activeTab === 'git'}
            class:text-[var(--color-text-secondary)]={activeTab !== 'git'}
            class:border-b-2={activeTab === 'git'}
            class:border-primary={activeTab === 'git'}
            onclick={() => { activeTab = 'git'; loadGitBranches(); loadGitCommits(); }}
          >
            Git
          </button>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-sm text-[var(--color-text)] font-mono">{selectedFile}</span>

          {#if selectedFile?.endsWith('.prop')}
            <select
              class="px-2 py-1 border border-[var(--color-border)] rounded bg-[var(--color-bg)] text-[var(--color-text)] text-sm"
              bind:value={translateLang}
            >
              <option value="en">English</option>
              <option value="zh">中文</option>
              <option value="ja">日本語</option>
              <option value="ko">한국어</option>
            </select>
            <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={translateDescription} disabled={translating}>
              <span class="material-symbols-outlined" slot="start">translate</span>
              {translating ? '翻译中...' : '翻译'}
            </button>
          {/if}

          <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={saveFile} disabled={saving}>
            {saving ? '保存中...' : '保存'}
          </button>
          <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={validateCurrentFile} disabled={validating}>
            <span class="material-symbols-outlined" slot="start">check_circle</span>
            {validating ? '校验中...' : '校验'}
          </button>
        </div>
      </div>

      <!-- 内容区域 -->
      <div class="flex-1 overflow-hidden relative">
        {#if activeTab === 'editor'}
          <!-- Remote cursors overlay -->
          {#if collabSessions.length > 0}
            <div class="absolute top-1 right-2 z-10 flex gap-1">
              {#each collabSessions.filter(s => s.file_path === selectedFile) as s}
                <div class="px-2 py-0.5 rounded-full text-xs text-white font-medium" style="background-color: {s.color}">
                  {s.username} 行{s.cursor_line}
                </div>
              {/each}
            </div>
          {/if}
          <CodeEditor
            value={editorContent}
            language={selectedFile ? detectLanguage(selectedFile) : 'javascript'}
            onChange={(val) => { editorContent = val; sendCollabCursor(0, 0); }}
          />
        {:else if activeTab === 'preview'}
          <PreviewPanel htmlContent={generatePreviewContent()} />
        {:else if activeTab === 'versions'}
          <!-- Version Management Panel -->
          <div class="h-full overflow-y-auto p-4 space-y-4">
            <div class="flex items-center justify-between">
              <h3 class="text-lg font-semibold">版本管理</h3>
              <button class="btn-primary text-sm" onclick={() => showCreateVersionDialog = true}>
                <span class="material-symbols-outlined text-sm" slot="start">add</span>
                创建版本
              </button>
            </div>

            {#if versionsLoading}
              <div class="text-center py-8 text-[var(--color-text-secondary)]">加载中...</div>
            {:else if projectVersions.length === 0}
              <div class="text-center py-12 text-[var(--color-text-secondary)]">
                <span class="material-symbols-outlined text-4xl mb-2 block">history</span>
                暂无版本记录
              </div>
            {:else}
              <div class="space-y-2">
                {#each projectVersions as v}
                  <div class="p-3 border border-[var(--color-border)] rounded-lg bg-[var(--color-surface)] hover:border-primary/50 transition-colors">
                    <div class="flex items-center justify-between">
                      <div>
                        <span class="font-mono font-semibold text-primary">{v.version}</span>
                        <span class="text-xs text-[var(--color-text-secondary)] ml-2">
                          {new Date(v.created_at).toLocaleString('zh-CN')}
                        </span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class="text-xs text-[var(--color-text-secondary)]">
                          {v.file_count} 文件 · {v.total_size > 1024 ? (v.total_size / 1024).toFixed(1) + ' KB' : v.total_size + ' B'}
                        </span>
                        <button class="btn-ghost text-xs px-2 py-1" onclick={() => rollbackVersion(v.version)}>
                          回滚
                        </button>
                      </div>
                    </div>
                    {#if v.changelog}
                      <p class="text-xs text-[var(--color-text-secondary)] mt-1">{v.changelog}</p>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}

            <!-- Version Diff Section -->
            {#if projectVersions.length >= 2}
              <div class="border-t border-[var(--color-border)] pt-4 mt-4">
                <h4 class="font-medium mb-3">版本对比</h4>
                <div class="flex items-center gap-2">
                  <select class="input-field text-sm" bind:value={versionDiffFrom}>
                    <option value="">选择基准版本</option>
                    {#each projectVersions as v}
                      <option value={v.version}>{v.version}</option>
                    {/each}
                  </select>
                  <span class="text-[var(--color-text-secondary)]">→</span>
                  <select class="input-field text-sm" bind:value={versionDiffTo}>
                    <option value="">选择目标版本</option>
                    {#each projectVersions as v}
                      <option value={v.version}>{v.version}</option>
                    {/each}
                  </select>
                  <button class="btn-ghost text-sm" onclick={diffVersions} disabled={diffingVersions || !versionDiffFrom || !versionDiffTo}>
                    {diffingVersions ? '对比中...' : '对比'}
                  </button>
                </div>

                {#if versionDiffResult}
                  <div class="mt-3 space-y-1">
                    <p class="text-xs text-[var(--color-text-secondary)]">
                      {versionDiffResult.version_a} → {versionDiffResult.version_b} ({versionDiffResult.total} 个文件)
                    </p>
                    {#each versionDiffResult.diffs || [] as diff}
                      <div class="flex items-center gap-2 text-xs py-1">
                        <span class="material-symbols-outlined text-sm
                          {diff.status === 'added' ? 'text-green-500' : diff.status === 'removed' ? 'text-red-500' : diff.status === 'modified' ? 'text-yellow-500' : 'text-[var(--color-text-secondary)]'}">
                          {diff.status === 'added' ? 'add_circle' : diff.status === 'removed' ? 'remove_circle' : diff.status === 'modified' ? 'edit' : 'check_circle'}
                        </span>
                        <span class="font-mono">{diff.path}</span>
                        <span class="text-[var(--color-text-secondary)]">{diff.status}</span>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {:else if activeTab === 'dependencies'}
          <!-- Dependency Resolution Panel -->
          <div class="h-full overflow-y-auto p-4 space-y-4">
            <div class="flex items-center justify-between">
              <h3 class="text-lg font-semibold">依赖分析</h3>
              <div class="flex gap-2">
                <button class="btn-ghost text-sm" onclick={analyzeDependencies} disabled={depsLoading}>
                  <span class="material-symbols-outlined text-sm" slot="start">search</span>
                  {depsLoading ? '分析中...' : '分析依赖'}
                </button>
                <button class="btn-ghost text-sm" onclick={resolveDependencies} disabled={resolvingDeps}>
                  <span class="material-symbols-outlined text-sm" slot="start">auto_fix_high</span>
                  {resolvingDeps ? '解析中...' : '自动解析'}
                </button>
              </div>
            </div>

            {#if depAnalysis}
              <!-- Analysis Results -->
              <div class="grid grid-cols-3 gap-3">
                <div class="p-3 border border-[var(--color-border)] rounded-lg text-center">
                  <div class="text-2xl font-bold text-primary">{depAnalysis.dependencies?.length || 0}</div>
                  <div class="text-xs text-[var(--color-text-secondary)]">依赖总数</div>
                </div>
                <div class="p-3 border border-[var(--color-border)] rounded-lg text-center">
                  <div class="text-2xl font-bold text-green-500">{(depAnalysis.dependencies?.length || 0) - (depAnalysis.missing?.length || 0)}</div>
                  <div class="text-xs text-[var(--color-text-secondary)]">已解析</div>
                </div>
                <div class="p-3 border border-[var(--color-border)] rounded-lg text-center">
                  <div class="text-2xl font-bold text-red-500">{depAnalysis.missing?.length || 0}</div>
                  <div class="text-xs text-[var(--color-text-secondary)]">缺失</div>
                </div>
              </div>

              {#if depAnalysis.warnings?.length > 0}
                <div class="p-3 bg-yellow-500/10 border border-yellow-500/30 rounded-lg">
                  <div class="flex items-center gap-2 text-yellow-600 text-sm font-medium mb-1">
                    <span class="material-symbols-outlined text-sm">warning</span>
                    警告
                  </div>
                  {#each depAnalysis.warnings as warning}
                    <p class="text-xs text-[var(--color-text-secondary)]">{warning}</p>
                  {/each}
                </div>
              {/if}

              {#if depAnalysis.missing?.length > 0}
                <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
                  <div class="px-3 py-2 bg-red-500/10 border-b border-[var(--color-border)] font-medium text-sm">
                    缺失的依赖
                  </div>
                  {#each depAnalysis.missing as dep}
                    <div class="px-3 py-2 border-b border-[var(--color-border)] last:border-b-0 flex items-center justify-between">
                      <div>
                        <span class="font-mono text-sm">{dep.name}</span>
                        <span class="text-xs text-[var(--color-text-secondary)] ml-2">来源: {dep.source}</span>
                        <span class="text-xs text-[var(--color-text-secondary)] ml-2">引用: {dep.reference_path}</span>
                      </div>
                      <button class="btn-ghost text-xs">安装</button>
                    </div>
                  {/each}
                </div>
              {/if}

              {#if depAnalysis.dependencies?.length > 0}
                <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
                  <div class="px-3 py-2 bg-primary/10 border-b border-[var(--color-border)] font-medium text-sm">
                    所有依赖
                  </div>
                  {#each depAnalysis.dependencies as dep}
                    <div class="px-3 py-2 border-b border-[var(--color-border)] last:border-b-0 flex items-center justify-between">
                      <div class="flex items-center gap-2">
                        <span class="material-symbols-outlined text-sm
                          {dep.id && !dep.id.startsWith('lib') ? 'text-green-500' : 'text-[var(--color-text-secondary)]'}">
                          {dep.id && !dep.id.startsWith('lib') ? 'check_circle' : 'help_outline'}
                        </span>
                        <span class="font-mono text-sm">{dep.name}</span>
                        {#if dep.version}
                          <span class="text-xs text-[var(--color-text-secondary)]">v{dep.version}</span>
                        {/if}
                        <span class="text-xs text-[var(--color-text-secondary)]">{dep.source}</span>
                      </div>
                      <span class="text-xs {dep.required ? 'text-red-500' : 'text-[var(--color-text-secondary)]'}">
                        {dep.required ? '必需' : '可选'}
                      </span>
                    </div>
                  {/each}
                </div>
              {/if}
            {/if}

            <!-- Dependency Tree -->
            {#if depTree}
              <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
                <div class="px-3 py-2 bg-[var(--color-surface)] border-b border-[var(--color-border)] font-medium text-sm">
                  依赖树
                </div>
                <div class="p-3">
                  <div class="flex items-center gap-2">
                    <span class="material-symbols-outlined text-sm text-primary">folder</span>
                    <span class="font-mono text-sm font-semibold">{depTree.name}</span>
                    <span class="text-xs text-[var(--color-text-secondary)]">v{depTree.version}</span>
                    <span class="px-1.5 py-0.5 text-xs rounded bg-green-500/20 text-green-600">{depTree.status}</span>
                  </div>
                  {#if depTree.children?.length > 0}
                    <div class="ml-6 mt-2 space-y-1 border-l border-[var(--color-border)] pl-3">
                      {#each depTree.children as child}
                        <div class="flex items-center gap-2">
                          <span class="material-symbols-outlined text-sm {child.status === 'resolved' ? 'text-green-500' : 'text-red-500'}">
                            {child.status === 'resolved' ? 'check_circle' : 'cancel'}
                          </span>
                          <span class="font-mono text-sm">{child.name}</span>
                          {#if child.version}
                            <span class="text-xs text-[var(--color-text-secondary)]">v{child.version}</span>
                          {/if}
                          <span class="px-1.5 py-0.5 text-xs rounded
                            {child.status === 'resolved' ? 'bg-green-500/20 text-green-600' : 'bg-red-500/20 text-red-600'}">
                            {child.status}
                          </span>
                        </div>
                      {/each}
                    </div>
                  {/if}
                </div>
              </div>
            {/if}

            {#if !depAnalysis && !depsLoading}
              <div class="text-center py-12 text-[var(--color-text-secondary)]">
                <span class="material-symbols-outlined text-4xl mb-2 block">account_tree</span>
                点击"分析依赖"开始扫描项目依赖
              </div>
            {/if}
          </div>
        {/if}
      </div>

      {#if translatedDesc && selectedFile?.endsWith('.prop')}
        <div class="p-3 border-t border-[var(--color-border)] bg-[var(--color-surface)]">
          <div class="flex items-center gap-2 mb-2">
            <span class="material-symbols-outlined text-sm">translate</span>
            <span class="text-xs font-medium">翻译结果</span>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div class="p-3 bg-[var(--color-bg)] rounded-lg border border-[var(--color-border)]">
              <p class="text-[11px] font-medium text-[var(--color-text-secondary)] mb-1">原文</p>
              <p class="text-xs">{originalDesc}</p>
            </div>
            <div class="p-3 bg-[var(--color-bg)] rounded-lg border border-[var(--color-border)]">
              <p class="text-[11px] font-medium text-[var(--color-text-secondary)] mb-1">译文 ({translateLang})</p>
              <p class="text-xs">{translatedDesc}</p>
      </div>
    </div>
  </div>
{/if}

{#if showCreateVersionDialog}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onclick={() => showCreateVersionDialog = false}>
    <div class="bg-[var(--color-bg)] rounded-xl p-6 w-full max-w-md border border-[var(--color-border)] shadow-xl" onclick={(e) => e.stopPropagation()}>
      <h3 class="text-lg font-semibold mb-4 flex items-center gap-2">
        <span class="material-symbols-outlined text-primary">add_circle</span>
        创建新版本
      </h3>
      <div class="space-y-3">
        <div>
          <label class="block text-sm font-medium mb-1">版本号</label>
          <input type="text" placeholder="e.g. 1.0.0" class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={newVersionNumber} />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1">更新日志</label>
          <textarea placeholder="描述此版本的更改..." class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] h-24 resize-none" bind:value={newVersionChangelog}></textarea>
        </div>
      </div>
      <div class="flex justify-end gap-2 mt-4">
        <button class="btn-ghost" onclick={() => showCreateVersionDialog = false}>取消</button>
        <button class="btn-primary" onclick={createVersion} disabled={creatingVersion || !newVersionNumber.trim()}>
          {creatingVersion ? '创建中...' : '创建版本'}
        </button>
      </div>
    </div>
  </div>
{/if}

      {#if showValidation && validationResults.length > 0}
        <div class="border-t border-[var(--color-border)] bg-[var(--color-surface)]">
          <div class="flex items-center justify-between px-4 py-2">
            <div class="flex items-center gap-2">
              <span class="material-symbols-outlined text-sm">check_circle</span>
              <span class="text-xs font-medium">语法校验结果</span>
            </div>
            <div class="flex items-center gap-2">
              <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={validateAllFiles} disabled={validating}>
                校验全部文件
              </button>
              <button onclick={() => showValidation = false}>
                <span class="material-symbols-outlined text-sm">close</span>
              </button>
            </div>
          </div>
          <div class="px-4 pb-3 space-y-2 max-h-48 overflow-auto">
            {#each validationResults as vr}
              <div class="p-2 rounded-lg border" style={vr.valid ? 'border-color: var(--color-success); background: var(--color-success-light)' : 'border-color: var(--color-error); background: var(--color-error-light)'}>
                <div class="flex items-center gap-2 mb-1">
                  <md-icon class="text-sm {vr.valid ? 'text-green-600' : 'text-red-600'}">
                    {vr.valid ? 'check_circle' : 'error'}
                  </md-icon>
                  <span class="text-xs font-medium">{vr.file}</span>
                  {#if vr.valid}
                    <span class="text-xs text-green-600">通过</span>
                  {/if}
                </div>
                {#each vr.errors as err}
                  <p class="text-xs text-red-600 ml-6">{err}</p>
                {/each}
                {#each vr.warnings as warn}
                  <p class="text-xs text-amber-600 ml-6">{warn}</p>
                {/each}
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {:else}
      <div class="flex-1 flex items-center justify-center text-[var(--color-text-secondary)]">
        <div class="text-center">
          <span class="material-symbols-outlined text-5xl mb-2">edit_document</span>
          <p class="text-base">选择一个文件开始编辑</p>
        </div>
      </div>
    {/if}
  </main>
</div>

<!-- Repo Dialog -->
{#if showRepoDialog}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-auto">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">导入 GitHub 仓库</h2>
        <button onclick={() => showRepoDialog = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="flex gap-2 mb-4">
        <input
          type="text"
          placeholder="https://github.com/user/repo"
          class="flex-1 px-4 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]"
          bind:value={repoUrl}
        />
        <button class="btn-primary" onclick={fetchRepoInfo} disabled={repoLoading}>
          {repoLoading ? '获取中...' : '获取'}
        </button>
      </div>

      {#if repoInfo}
        <div class="border border-[var(--color-border)] rounded-xl p-4 mb-4">
          <div class="flex items-center gap-2 mb-2">
            <span class="material-symbols-outlined">folder</span>
            <span class="text-base font-semibold">{repoInfo.owner}/{repoInfo.name}</span>
          </div>
          <div class="flex gap-4 text-xs text-[var(--color-text-secondary)]">
            <span>⭐ {repoInfo.stars}</span>
            <span>📄 {repoInfo.license || 'N/A'}</span>
          </div>
          {#if repoInfo.topics?.length}
            <div class="flex flex-wrap gap-2 mt-2">
              {#each repoInfo.topics as topic}
                <span class="px-2 py-1 bg-primary-600-50 text-primary-700 rounded-full text-xs">{topic}</span>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      {#if repoFiles.length}
        <div class="border border-[var(--color-border)] rounded-xl p-4">
          <h3 class="text-sm font-semibold mb-2">仓库文件</h3>
          <div class="space-y-1">
            {#each repoFiles as file}
              <div class="flex items-center gap-2 p-2 rounded-xl hover:bg-[var(--color-surface)] cursor-pointer transition-colors">
                <span class="material-symbols-outlined" slot="start">{file.type === 'dir' ? 'folder' : 'description'}</span>
                <span>{file.name}</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Template Panel -->
{#if showTemplatePanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-auto">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">模板推荐</h2>
        <button onclick={() => showTemplatePanel = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <input
        type="text"
        placeholder="输入描述搜索模板..."
        class="w-full px-4 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] mb-4"
        bind:value={templateSearch}
      />

      <div class="grid grid-cols-1 gap-3">
        {#each (templates || []).filter(t => !templateSearch || t.description?.toLowerCase().includes(templateSearch.toLowerCase())) as tmpl}
          <button
            class="p-4 border border-[var(--color-border)] rounded-xl text-left hover:bg-[var(--color-surface)] transition-colors"
            onclick={() => applyTemplate(tmpl)}
          >
            <div class="flex items-center gap-2 mb-1">
              <span class="material-symbols-outlined">{tmpl.category === 'system' ? 'settings' : tmpl.category === 'ui' ? 'palette' : 'extension'}</span>
              <span class="text-sm font-semibold">{tmpl.name}</span>
            </div>
            <p class="text-xs text-[var(--color-text-secondary)]">{tmpl.description}</p>
          </button>
        {/each}
      </div>
    </div>
  </div>
{/if}

<!-- AI Chat Bubble -->
<button
  class="fixed bottom-6 right-6 w-14 h-14 bg-primary-600 text-white rounded-full shadow-lg flex items-center justify-center hover:bg-primary-600/90 transition-colors z-40"
  onclick={() => chatOpen = !chatOpen}
>
  <span class="text-2xl">{chatOpen ? '✕' : '💬'}</span>
</button>

{#if chatOpen}
  <div class="fixed bottom-24 right-6 w-96 h-[500px] bg-[var(--color-bg)] rounded-2xl shadow-2xl flex flex-col z-50 border border-[var(--color-border)]">
    <div class="p-4 border-b border-[var(--color-border)] flex items-center justify-between">
      <div class="flex items-center gap-2">
        <span class="text-lg">🤖</span>
        <span class="text-base font-semibold">AI 助手</span>
      </div>
      <button onclick={() => chatOpen = false}>
        <span class="material-symbols-outlined">close</span>
      </button>
    </div>

    <div class="flex-1 overflow-auto p-4 space-y-4">
      {#if chatMessages.length === 0}
        <div class="text-center text-[var(--color-text-secondary)] py-8">
          <p class="text-base mb-2">👋 你好！</p>
          <p class="text-xs">我是 Magisk/KSU 模块开发助手，有什么可以帮你的？</p>
        </div>
      {/if}
      {#each chatMessages as msg}
        <div class="flex {msg.role === 'user' ? 'justify-end' : 'justify-start'}">
          <div class="max-w-[80%] {msg.role === 'user' ? 'bg-primary-600 text-white' : 'bg-[var(--color-surface)] text-[var(--color-text)]'} rounded-2xl px-4 py-2">
            <p class="text-xs whitespace-pre-wrap">{msg.content}{chatStreaming && msg.role === 'assistant' && msg === chatMessages[chatMessages.length - 1] ? '▊' : ''}</p>
          </div>
        </div>
      {/each}
    </div>

    <div class="p-4 border-t border-[var(--color-border)]">
      <div class="flex gap-2">
        <input
          type="text"
          placeholder="输入消息..."
          class="flex-1 px-4 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]"
          bind:value={chatInput}
          onkeydown={handleChatKeydown}
          disabled={chatStreaming}
        />
        <button class="btn-primary" onclick={sendChatMessage} disabled={chatStreaming || !chatInput.trim()}>
          <span class="material-symbols-outlined" slot="start">send</span>
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Build Log Panel -->
{#if showBuildLog}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">构建日志</h2>
        <button onclick={() => showBuildLog = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="flex gap-2 mb-4">
        <input
          type="text"
          placeholder="输入构建 ID..."
          class="flex-1 px-4 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]"
          bind:value={buildId}
        />
        <button class="btn-primary" onclick={loadBuildLogs}>
          <span class="material-symbols-outlined" slot="start">refresh</span>
          加载日志
        </button>
      </div>

      <div class="flex-1 overflow-auto bg-[var(--color-surface)] rounded-xl p-4 font-mono text-sm">
        {#if buildLogs.length === 0}
          <p class="text-[var(--color-text-secondary)]">暂无日志</p>
        {:else}
          {#each buildLogs as log}
            <div class="mb-1 {
              log.level === 'ERROR' ? 'text-error' :
              log.level === 'WARN' ? 'text-yellow' :
              log.level === 'SUCCESS' ? 'text-green' :
              'text-[var(--color-text)]'
            }">
              <span class="text-[var(--color-text-secondary)]">[{log.timestamp}]</span>
              <span class="font-bold">[{log.level}]</span>
              {log.message}
            </div>
          {/each}
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- Git Version History Panel -->
{#if showGitPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">版本历史</h2>
        <button onclick={() => showGitPanel = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="flex-1 overflow-auto mb-4">
        {#if gitLoading}
          <div class="flex justify-center p-4"><div class="animate-spin h-5 w-5 border-2 border-primary-500 border-t-transparent rounded-full"></div></div>
        {:else if gitCommits.length === 0}
          <p class="text-[var(--color-text-secondary)] text-center py-8">暂无提交历史</p>
        {:else}
          <div class="space-y-2">
            {#each gitCommits as commit}
              <div class="p-3 rounded-xl border {commit.hash === gitHeadHash ? 'border-primary bg-primary-600-50' : 'border-[var(--color-border)]'}">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="font-mono text-sm font-bold">{commit.hash.substring(0, 8)}</span>
                    <span class="text-xs">{commit.message}</span>
                    {#if commit.hash === gitHeadHash}
                      <span class="px-2 py-0.5 bg-primary-600 text-white rounded-full text-xs">HEAD</span>
                    {/if}
                  </div>
                  <div class="flex items-center gap-2">
                    <span class="text-xs text-[var(--color-text-secondary)]">{commit.author}</span>
                    <span class="text-xs text-[var(--color-text-secondary)]">{new Date(commit.timestamp).toLocaleString('zh-CN')}</span>
                    {#if commit.hash !== gitHeadHash}
                      <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={() => gitCheckout(commit.hash)}>恢复</button>
                    {/if}
                  </div>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <div class="border-t border-[var(--color-border)] pt-4">
        <div class="flex gap-2">
          <input
            type="text"
            placeholder="输入版本描述..."
            class="flex-1 px-4 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]"
            bind:value={gitMessage}
            onkeydown={(e) => { if (e.key === 'Enter') saveGitCommit(); }}
          />
          <button class="btn-primary" onclick={saveGitCommit} disabled={gitLoading || !gitMessage.trim()}>
            <span class="material-symbols-outlined" slot="start">save</span>
            保存版本
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- ADB Device Panel -->
{#if showADBPanels}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">ADB 设备管理</h2>
        <button onclick={() => showADBPanels = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="flex gap-2 mb-4">
        <button class="btn-primary" onclick={checkADB} disabled={adbChecking}>
          <span class="material-symbols-outlined" slot="start">search</span>
          {adbChecking ? '检测中...' : '检测 ADB'}
        </button>
        {#if adbAvailable === false}
          <span class="text-error flex items-center gap-1">
            <span class="material-symbols-outlined text-sm">error</span>
            ADB 未检测到
          </span>
        {/if}
      </div>

      {#if adbAvailable === true}
        <div class="flex-1 overflow-auto mb-4">
          {#if adbLoadingDevices}
            <div class="flex justify-center p-4"><div class="animate-spin h-5 w-5 border-2 border-primary-500 border-t-transparent rounded-full"></div></div>
          {:else if adbDevices.length === 0}
            <p class="text-[var(--color-text-secondary)] text-center py-8">未发现设备</p>
          {:else}
            <div class="space-y-2">
              {#each adbDevices as dev}
                <div class="p-3 rounded-xl border border-[var(--color-border)]">
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                      <span class="font-mono text-sm font-bold">{dev.serial}</span>
                      <span class="text-xs">{dev.model || 'Unknown'}</span>
                      <span class="px-2 py-0.5 rounded-full text-xs" style={dev.state === 'device' ? 'background: var(--color-success-light); color: var(--color-success)' : dev.state === 'offline' ? 'background: var(--color-error-light); color: var(--color-error)' : 'background: var(--color-warning-light); color: var(--color-warning)'}>{dev.state}</span>
                    </div>
                    <div class="flex gap-2">
                      {#if dev.state === 'device'}
                        <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={() => installModule(dev.serial)}>
                          <span class="material-symbols-outlined" slot="start">download</span>
                          安装模块
                        </button>
{/if}

<!-- ZIP Import Dialog -->
{#if showZipImportDialog}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" onclick={() => showZipImportDialog = false}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-lg border border-[var(--color-border)]" onclick={(e) => e.stopPropagation()}>
      <div class="px-6 py-4 border-b border-[var(--color-border)]">
        <h3 class="text-lg font-semibold text-[var(--color-text)]">确认导入模块</h3>
      </div>
      <div class="px-6 py-4 space-y-4">
        {#if zipImportError}
          <div class="p-3 rounded-lg text-xs" style="background: color-mix(in srgb, var(--color-error, #ef4444) 8%, var(--color-bg)); border: 1px solid color-mix(in srgb, var(--color-error, #ef4444) 20%, transparent);">
            <p class="text-[var(--color-error)]">{zipImportError}</p>
          </div>
        {/if}
        {#if parsedModule}
          <div class="rounded-xl p-4 space-y-2" style="background: var(--color-surface); border: 1px solid var(--color-border);">
            <div class="flex items-center gap-2 pb-2 border-b border-[var(--color-border)]">
              <span class="material-symbols-outlined text-primary-500">badge</span>
              <h4 class="font-semibold text-[var(--color-text)]">{parsedModule.name || '未知模块'}</h4>
            </div>
            <div class="grid grid-cols-2 gap-2 text-xs">
              {#if parsedModule.id}<div><span class="text-[var(--color-text-muted)]">ID：</span><span class="text-[var(--color-text)]">{parsedModule.id}</span></div>{/if}
              {#if parsedModule.version}<div><span class="text-[var(--color-text-muted)]">版本：</span><span class="text-[var(--color-text)]">{parsedModule.version}</span></div>{/if}
              {#if parsedModule.versionCode}<div><span class="text-[var(--color-text-muted)]">版本号：</span><span class="text-[var(--color-text)]">{parsedModule.versionCode}</span></div>{/if}
              {#if parsedModule.author}<div><span class="text-[var(--color-text-muted)]">作者：</span><span class="text-[var(--color-text)]">{parsedModule.author}</span></div>{/if}
            </div>
            {#if parsedModule.description}
              <p class="text-xs text-[var(--color-text-secondary)] pt-1 border-t border-[var(--color-border)]">{parsedModule.description}</p>
            {/if}
            {#if parsedModule.ksu_supported || parsedModule.apatch_supported}
              <div class="flex gap-2 pt-1">
                {#if parsedModule.ksu_supported}
                  <span class="inline-flex items-center gap-0.5 px-2 py-0.5 rounded text-[10px] font-medium" style="background: color-mix(in srgb, #22c55e 12%, transparent); color: #22c55e;">
                    <span class="material-symbols-outlined text-[10px]">check_circle</span> KSU
                  </span>
                {/if}
                {#if parsedModule.apatch_supported}
                  <span class="inline-flex items-center gap-0.5 px-2 py-0.5 rounded text-[10px] font-medium" style="background: color-mix(in srgb, #8b5cf6 12%, transparent); color: #8b5cf6;">
                    <span class="material-symbols-outlined text-[10px]">check_circle</span> APatch
                  </span>
                {/if}
              </div>
            {/if}
          </div>
        {:else}
          <div class="flex items-center justify-center py-6">
            <span class="material-symbols-outlined animate-spin text-primary-500">progress_activity</span>
            <span class="ml-2 text-sm text-[var(--color-text-secondary)]">解析模块信息...</span>
          </div>
        {/if}
        <div class="text-xs text-[var(--color-text-muted)]">
          <span class="material-symbols-outlined text-[12px] align-text-bottom">info</span>
          将导入 {(parsedFiles || []).filter(f => !f.is_dir).length} 个文件到当前项目
        </div>
      </div>
      <div class="flex justify-end gap-2 px-6 py-4 border-t border-[var(--color-border)]">
        <button class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={() => showZipImportDialog = false}>取消</button>
        <button class="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={confirmZipImport} disabled={zipImporting || !parsedModule?.id}>
          {#if zipImporting}
            <span class="material-symbols-outlined text-[14px] animate-spin">progress_activity</span>
            导入中...
          {:else}
            <span class="material-symbols-outlined text-[14px]">download</span>
            确认导入
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

                      <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={() => rebootDevice(dev.serial)}>
                        <span class="material-symbols-outlined" slot="start">refresh</span>
                        重启
                      </button>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      {#if adbOutput}
        <div class="border-t border-[var(--color-border)] pt-4">
          <div class="bg-[var(--color-surface)] rounded-xl p-4 font-mono text-sm max-h-32 overflow-auto">
            {adbOutput}
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Screenshot Panel -->
{#if showScreenshotPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">真机截图</h2>
        <button onclick={() => showScreenshotPanel = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="flex gap-2 mb-4">
        <select
          class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]"
          bind:value={screenshotDevice}
        >
          <option value="">选择设备</option>
          {#each adbDevices as dev}
            <option value={dev.serial}>{dev.serial} ({dev.model || 'Unknown'})</option>
          {/each}
        </select>
        <button class="btn-primary" onclick={takeScreenshot} disabled={screenshotLoading || !screenshotDevice}>
          <span class="material-symbols-outlined" slot="start">photo_camera</span>
          {screenshotLoading ? '截图中...' : '截图'}
        </button>
        <button class="btn-ghost border border-[var(--color-border)]" onclick={streamScreenshots} disabled={screenshotStreaming || !screenshotDevice}>
          <span class="material-symbols-outlined" slot="start">burst_mode</span>
          {screenshotStreaming ? '连续截图中...' : '连续截图'}
        </button>
      </div>

      <div class="flex-1 overflow-auto">
        {#if screenshotImages.length === 0}
          <div class="text-center text-[var(--color-text-secondary)] py-12">
            <span class="material-symbols-outlined text-5xl mb-2">photo_camera</span>
            <p>选择设备后点击截图</p>
          </div>
        {:else}
          <div class="grid grid-cols-2 gap-3">
            {#each screenshotImages as img}
              <div class="border border-[var(--color-border)] rounded-xl overflow-hidden">
                <div class="bg-[var(--color-surface)] p-2 text-xs font-mono truncate">{img.filename}</div>
                <img src="/api/v1/adb/screenshot/file?path={encodeURIComponent(img.path)}" alt={img.filename} class="w-full" />
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- Signature Panel -->
{#if signatureInfo}
  <div class="fixed bottom-24 right-6 w-80 bg-[var(--color-bg)] rounded-2xl shadow-2xl z-50 border border-[var(--color-border)] p-4">
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-2">
        <span class="material-symbols-outlined text-primary">verified</span>
        <span class="text-base font-semibold">模块签名</span>
      </div>
      <button onclick={() => { signatureInfo = null; verifyResult = null; }}>
        <span class="material-symbols-outlined text-sm">close</span>
      </button>
    </div>

    <div class="space-y-2 text-xs">
      <div class="flex justify-between">
        <span class="text-[var(--color-text-secondary)]">算法</span>
        <span class="font-mono">{signatureInfo.algorithm}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-[var(--color-text-secondary)]">大小</span>
        <span class="font-mono">{(signatureInfo.size / 1024).toFixed(1)} KB</span>
      </div>
      <div class="flex justify-between">
        <span class="text-[var(--color-text-secondary)]">签名时间</span>
        <span class="font-mono">{new Date(signatureInfo.signed_at).toLocaleString('zh-CN')}</span>
      </div>
      <div>
        <span class="text-[var(--color-text-secondary)]">SHA256</span>
        <p class="font-mono text-xs break-all mt-1 bg-[var(--color-surface)] p-2 rounded">{signatureInfo.hash}</p>
      </div>
    </div>

    <div class="flex gap-2 mt-3">
      <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={verifyModule} disabled={verifying}>
        <span class="material-symbols-outlined" slot="start">check_circle</span>
        {verifying ? '验证中...' : '验证'}
      </button>
    </div>

    {#if verifyResult}
      <div class="mt-2 p-2 rounded-lg border" style={verifyResult.valid ? 'border-color: var(--color-success); background: var(--color-success-light)' : 'border-color: var(--color-error); background: var(--color-error-light)'}>
        <div class="flex items-center gap-2">
          <md-icon class="text-sm {verifyResult.valid ? 'text-green-600' : 'text-red-600'}">
            {verifyResult.valid ? 'check_circle' : 'error'}
          </md-icon>
          <span class="text-xs">{verifyResult.valid ? '校验通过' : '校验失败'}</span>
        </div>
      </div>
            {/if}
          </div>
        {:else if activeTab === 'security'}
          <!-- Security Panel -->
          <div class="h-full overflow-y-auto p-4 space-y-6">
            <h3 class="text-lg font-semibold">🔒 安全检查</h3>

            <!-- Signature Section -->
            <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
              <div class="px-4 py-3 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center justify-between">
                <h4 class="font-medium text-sm">模块签名</h4>
                <div class="flex gap-2">
                  <button class="btn-ghost text-xs" onclick={signModule} disabled={signing}>
                    <span class="material-symbols-outlined text-sm" slot="start">edit_note</span>
                    {signing ? '签名中...' : '签名模块'}
                  </button>
                  <button class="btn-ghost text-xs" onclick={verifyModule} disabled={verifying || !sigInfo}>
                    <span class="material-symbols-outlined text-sm" slot="start">verified</span>
                    {verifying ? '验证中...' : '验证签名'}
                  </button>
                </div>
              </div>
              <div class="p-4">
                {#if sigInfo && sigInfo.signed}
                  <div class="space-y-2">
                    <div class="flex items-center gap-2 text-green-600">
                      <span class="material-symbols-outlined text-sm">check_circle</span>
                      <span class="text-sm font-medium">已签名</span>
                    </div>
                    <div class="grid grid-cols-2 gap-2 text-xs">
                      <div><span class="text-[var(--color-text-secondary)]">算法:</span> {sigInfo.algorithm}</div>
                      <div><span class="text-[var(--color-text-secondary)]">签名时间:</span> {new Date(sigInfo.signed_at).toLocaleString('zh-CN')}</div>
                      <div class="col-span-2"><span class="text-[var(--color-text-secondary)]">文件哈希:</span> <code class="font-mono text-[10px] break-all">{sigInfo.file_hash}</code></div>
                      {#if sigInfo.fingerprint}
                        <div class="col-span-2"><span class="text-[var(--color-text-secondary)]">公钥指纹:</span> <code class="font-mono text-[10px] break-all">{sigInfo.fingerprint}</code></div>
                      {/if}
                    </div>
                    {#if verifyResult}
                      <div class="mt-2 p-2 rounded text-sm {verifyResult.valid ? 'bg-green-500/10 text-green-600' : 'bg-red-500/10 text-red-600'}">
                        {verifyResult.valid ? '✅ 签名验证通过' : '❌ 签名验证失败'}
                        {#if verifyResult.message}
                          <span class="text-xs block">{verifyResult.message}</span>
                        {:else if verifyResult.error}
                          <span class="text-xs block">{verifyResult.error}</span>
                        {/if}
                      </div>
                    {/if}
                  </div>
                {:else}
                  <div class="text-center py-4 text-[var(--color-text-secondary)]">
                    <span class="material-symbols-outlined text-3xl mb-2 block">lock_open</span>
                    <p class="text-sm">模块未签名</p>
                    <p class="text-xs mt-1">签名可确保模块代码的完整性和来源可信</p>
                  </div>
                {/if}
              </div>
            </div>

            <!-- Vulnerability Scanning Section -->
            <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
              <div class="px-4 py-3 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center justify-between">
                <h4 class="font-medium text-sm">漏洞扫描</h4>
                <button class="btn-ghost text-xs" onclick={scanVulnerabilities} disabled={vulnScanning}>
                  <span class="material-symbols-outlined text-sm" slot="start">bug_report</span>
                  {vulnScanning ? '扫描中...' : '开始扫描'}
                </button>
              </div>
              <div class="p-4">
                {#if vulnResults}
                  <div class="space-y-3">
                    <!-- Stats -->
                    <div class="grid grid-cols-4 gap-2 text-center">
                      <div class="p-2 rounded bg-red-500/10">
                        <div class="text-lg font-bold text-red-500">{vulnResults.critical_count}</div>
                        <div class="text-[10px] text-[var(--color-text-secondary)]">高危</div>
                      </div>
                      <div class="p-2 rounded bg-orange-500/10">
                        <div class="text-lg font-bold text-orange-500">{vulnResults.high_count}</div>
                        <div class="text-[10px] text-[var(--color-text-secondary)]">中高</div>
                      </div>
                      <div class="p-2 rounded bg-yellow-500/10">
                        <div class="text-lg font-bold text-yellow-500">{vulnResults.medium_count}</div>
                        <div class="text-[10px] text-[var(--color-text-secondary)]">中危</div>
                      </div>
                      <div class="p-2 rounded bg-blue-500/10">
                        <div class="text-lg font-bold text-blue-500">{vulnResults.low_count}</div>
                        <div class="text-[10px] text-[var(--color-text-secondary)]">低危</div>
                      </div>
                    </div>
                    <div class="text-center text-sm">
                      风险评分: <span class="font-bold {vulnResults.risk_score >= 80 ? 'text-green-500' : vulnResults.risk_score >= 60 ? 'text-yellow-500' : 'text-red-500'}">{vulnResults.risk_score}/100</span>
                    </div>

                    <!-- Issues -->
                    {#if vulnResults.issues?.length > 0}
                      <div class="space-y-1 max-h-64 overflow-y-auto">
                        {#each vulnResults.issues as issue}
                          <div class="p-2 rounded text-xs border border-[var(--color-border)]">
                            <div class="flex items-center gap-2 mb-1">
                              <span class="px-1.5 py-0.5 rounded text-[10px] font-medium
                                {issue.severity === 'critical' ? 'bg-red-500/20 text-red-600' :
                                  issue.severity === 'high' ? 'bg-orange-500/20 text-orange-600' :
                                  issue.severity === 'medium' ? 'bg-yellow-500/20 text-yellow-600' :
                                  'bg-blue-500/20 text-blue-600'}">
                                {issue.severity}
                              </span>
                              <span class="text-[var(--color-text-secondary)]">{issue.file}:{issue.line}</span>
                            </div>
                            <p class="text-[var(--color-text)]">{issue.message}</p>
                            {#if issue.fix}
                              <p class="text-[var(--color-text-secondary)] mt-1">💡 {issue.fix}</p>
                            {/if}
                          </div>
                        {/each}
                      </div>
                    {:else}
                      <p class="text-center text-sm text-green-500">✅ 未发现安全漏洞</p>
                    {/if}
                  </div>
                {:else}
                  <div class="text-center py-6 text-[var(--color-text-secondary)]">
                    <span class="material-symbols-outlined text-3xl mb-2 block">security</span>
                    <p class="text-sm">点击"开始扫描"检测代码中的安全漏洞</p>
                    <p class="text-xs mt-1">检查危险函数、硬编码路径、不安全权限等</p>
                  </div>
                {/if}
              </div>
            </div>

            <!-- Permission Audit Section -->
            <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
              <div class="px-4 py-3 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center justify-between">
                <h4 class="font-medium text-sm">权限审计</h4>
                <button class="btn-ghost text-xs" onclick={auditPermissions} disabled={permAuditing}>
                  <span class="material-symbols-outlined text-sm" slot="start">admin_panel_settings</span>
                  {permAuditing ? '审计中...' : '开始审计'}
                </button>
              </div>
              <div class="p-4">
                {#if permResults}
                  <div class="space-y-3">
                    <!-- Stats -->
                    <div class="flex items-center justify-between">
                      <div class="flex gap-3 text-sm">
                        <span>总计: <strong>{permResults.total_permissions}</strong></span>
                        <span class="text-red-500">危险: <strong>{permResults.dangerous_count}</strong></span>
                        <span class="text-[var(--color-text-secondary)]">普通: <strong>{permResults.normal_count}</strong></span>
                      </div>
                      <div class="text-sm">
                        风险评分: <span class="font-bold {permResults.risk_score >= 80 ? 'text-green-500' : permResults.risk_score >= 60 ? 'text-yellow-500' : 'text-red-500'}">{permResults.risk_score}/100</span>
                        <span class="ml-2 px-2 py-0.5 rounded text-xs
                          {permResults.risk_level === 'safe' ? 'bg-green-500/20 text-green-600' :
                            permResults.risk_level === 'low' ? 'bg-blue-500/20 text-blue-600' :
                            permResults.risk_level === 'medium' ? 'bg-yellow-500/20 text-yellow-600' :
                            permResults.risk_level === 'high' ? 'bg-orange-500/20 text-orange-600' :
                            'bg-red-500/20 text-red-600'}">
                          {permResults.risk_level}
                        </span>
                      </div>
                    </div>

                    <!-- Warnings -->
                    {#if permResults.warnings?.length > 0}
                      <div class="p-2 bg-yellow-500/10 border border-yellow-500/30 rounded text-xs">
                        {#each permResults.warnings as warning}
                          <p>⚠️ {warning}</p>
                        {/each}
                      </div>
                    {/if}

                    <!-- Dangerous permissions -->
                    {#if permResults.dangerous_permissions?.length > 0}
                      <div>
                        <h5 class="text-xs font-medium text-red-500 mb-1">⚠️ 危险权限</h5>
                        <div class="space-y-1 max-h-48 overflow-y-auto">
                          {#each permResults.dangerous_permissions as perm}
                            <div class="p-2 rounded text-xs border border-red-500/20 bg-red-500/5">
                              <div class="flex items-center gap-2">
                                <span class="font-mono">{perm.name}</span>
                                {#if perm.is_common}
                                  <span class="px-1 py-0.5 rounded text-[10px] bg-blue-500/20 text-blue-600">常见</span>
                                {/if}
                              </div>
                              <p class="text-[var(--color-text-secondary)] mt-0.5">{perm.description} — {perm.risk}</p>
                            </div>
                          {/each}
                        </div>
                      </div>
                    {/if}

                    <!-- All permissions -->
                    {#if permResults.permissions?.length > 0}
                      <details class="group">
                        <summary class="text-xs font-medium cursor-pointer text-[var(--color-text-secondary)] hover:text-[var(--color-text)]">
                          查看所有权限 ({permResults.permissions.length})
                        </summary>
                        <div class="mt-1 space-y-1 max-h-48 overflow-y-auto">
                          {#each permResults.permissions as perm}
                            <div class="p-1.5 rounded text-[11px] border border-[var(--color-border)] flex items-center justify-between">
                              <span class="font-mono">{perm.name}</span>
                              <span class="px-1 py-0.5 rounded text-[10px]
                                {perm.level === 'dangerous' ? 'bg-red-500/20 text-red-600' :
                                  perm.level === 'normal' ? 'bg-green-500/20 text-green-600' :
                                  'bg-gray-500/20 text-gray-600'}">
                                {perm.level}
                              </span>
                            </div>
                          {/each}
                        </div>
                      </details>
                    {/if}
                  </div>
                {:else}
                  <div class="text-center py-6 text-[var(--color-text-secondary)]">
                    <span class="material-symbols-outlined text-3xl mb-2 block">admin_panel_settings</span>
                    <p class="text-sm">点击"开始审计"分析模块请求的权限</p>
                    <p class="text-xs mt-1">检查危险权限、权限合理性、风险评分</p>
                  </div>
                {/if}
              </div>
            </div>

            <!-- Audit History -->
            {#if vulnHistory.length > 0 || permHistory.length > 0}
              <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
                <div class="px-4 py-3 bg-[var(--color-surface)] border-b border-[var(--color-border)]">
                  <h4 class="font-medium text-sm">审计历史</h4>
                </div>
                <div class="p-4 space-y-2 max-h-48 overflow-y-auto">
                  {#each vulnHistory.slice(0, 5) as scan}
                    <div class="text-xs p-2 rounded bg-[var(--color-bg)] border border-[var(--color-border)]">
                      <span class="text-[var(--color-text-secondary)]">{new Date(scan.scanned_at).toLocaleString('zh-CN')}</span>
                      <span class="ml-2">漏洞扫描: {scan.total_issues} 个问题</span>
                    </div>
                  {/each}
                  {#each permHistory.slice(0, 5) as audit}
                    <div class="text-xs p-2 rounded bg-[var(--color-bg)] border border-[var(--color-border)]">
                      <span class="text-[var(--color-text-secondary)]">{new Date(audit.audited_at).toLocaleString('zh-CN')}</span>
                      <span class="ml-2">权限审计: {audit.total_permissions} 个权限</span>
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
          </div>
        {:else if activeTab === 'git'}
          <!-- Git Panel -->
          <div class="h-full overflow-y-auto p-4 space-y-6">
            <h3 class="text-lg font-semibold">Git 版本控制</h3>

            <!-- Current Branch -->
            <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
              <div class="px-4 py-3 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span class="material-symbols-outlined text-sm">branch</span>
                  <span class="font-medium text-sm">当前分支</span>
                  <span class="text-xs px-2 py-0.5 rounded-full bg-primary-100 text-primary-700 font-mono">{gitCurrentBranch}</span>
                </div>
                <button class="btn-ghost text-xs" onclick={loadGitBranches} disabled={gitBranchLoading}>
                  <span class="material-symbols-outlined text-sm" slot="start">refresh</span>
                  {gitBranchLoading ? '刷新中...' : '刷新'}
                </button>
              </div>
              <div class="p-4">
                <!-- Branch List -->
                <div class="space-y-2">
                  {#each gitBranches as branch}
                    <div class="flex items-center justify-between p-2 rounded-lg border {branch.is_current ? 'border-primary-500 bg-primary-500/10' : 'border-[var(--color-border)] hover:bg-[var(--color-surface)]'}">
                      <div class="flex items-center gap-2">
                        <span class="material-symbols-outlined text-sm {branch.is_current ? 'text-primary-500' : 'text-[var(--color-text-muted)]'}">
                          {branch.is_current ? 'radio_button_checked' : 'radio_button_unchecked'}
                        </span>
                        <span class="text-sm font-mono">{branch.name}</span>
                        {#if branch.is_current}
                          <span class="text-[10px] px-1.5 py-0.5 rounded bg-primary-500 text-white">当前</span>
                        {/if}
                      </div>
                      {#if !branch.is_current}
                        <button class="btn-ghost text-xs" onclick={() => switchGitBranch(branch.name)} disabled={gitBranchLoading}>
                          切换
                        </button>
                      {/if}
                    </div>
                  {/each}
                </div>

                <!-- Create Branch -->
                <div class="mt-3 flex gap-2">
                  <input
                    type="text"
                    class="flex-1 px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] text-sm"
                    placeholder="新分支名称"
                    bind:value={newBranchName}
                  />
                  <button class="btn-primary text-sm" onclick={createGitBranch} disabled={gitBranchLoading || !newBranchName.trim()}>
                    <span class="material-symbols-outlined text-sm" slot="start">add</span>
                    创建
                  </button>
                </div>
              </div>
            </div>

            <!-- Commit Changes -->
            <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
              <div class="px-4 py-3 bg-[var(--color-surface)] border-b border-[var(--color-border)]">
                <h4 class="font-medium text-sm">提交更改</h4>
              </div>
              <div class="p-4 space-y-3">
                <textarea
                  class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] text-sm resize-none"
                  rows="2"
                  placeholder="提交信息..."
                  bind:value={gitMessage}
                ></textarea>
                <div class="flex gap-2">
                  <button class="btn-primary text-sm flex-1" onclick={saveGitCommit} disabled={gitLoading || !gitMessage.trim()}>
                    <span class="material-symbols-outlined text-sm" slot="start">commit</span>
                    {gitLoading ? '提交中...' : '提交'}
                  </button>
                </div>
              </div>
            </div>

            <!-- Push / Pull -->
            <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
              <div class="px-4 py-3 bg-[var(--color-surface)] border-b border-[var(--color-border)]">
                <h4 class="font-medium text-sm">远程操作</h4>
              </div>
              <div class="p-4 space-y-3">
                <div class="flex flex-col gap-1">
                  <label class="text-[11px] font-medium text-[var(--color-text-secondary)]">远程仓库</label>
                  <input
                    type="text"
                    class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] text-sm"
                    placeholder="origin"
                    bind:value={gitRemote}
                  />
                </div>
                <div class="flex gap-2">
                  <button class="btn-ghost border border-[var(--color-border)] text-sm flex-1" onclick={pushGit} disabled={gitPushLoading}>
                    <span class="material-symbols-outlined text-sm" slot="start">upload</span>
                    {gitPushLoading ? '推送中...' : '推送'}
                  </button>
                  <button class="btn-ghost border border-[var(--color-border)] text-sm flex-1" onclick={pullGit} disabled={gitPullLoading}>
                    <span class="material-symbols-outlined text-sm" slot="start">download</span>
                    {gitPullLoading ? '拉取中...' : '拉取'}
                  </button>
                </div>
                {#if gitPushOutput}
                  <div class="text-xs p-2 rounded bg-[var(--color-surface)] border border-[var(--color-border)] font-mono whitespace-pre-wrap max-h-24 overflow-y-auto">{gitPushOutput}</div>
                {/if}
                {#if gitPullOutput}
                  <div class="text-xs p-2 rounded bg-[var(--color-surface)] border border-[var(--color-border)] font-mono whitespace-pre-wrap max-h-24 overflow-y-auto">{gitPullOutput}</div>
                {/if}
              </div>
            </div>

            <!-- Commit History -->
            <div class="border border-[var(--color-border)] rounded-lg overflow-hidden">
              <div class="px-4 py-3 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center justify-between">
                <h4 class="font-medium text-sm">提交历史</h4>
                <button class="btn-ghost text-xs" onclick={loadGitCommits} disabled={gitLoading}>
                  <span class="material-symbols-outlined text-sm" slot="start">refresh</span>
                  {gitLoading ? '加载中...' : '刷新'}
                </button>
              </div>
              <div class="p-4 space-y-2 max-h-96 overflow-y-auto">
                {#if gitCommits.length === 0}
                  <p class="text-sm text-[var(--color-text-secondary)] text-center py-4">暂无提交记录</p>
                {:else}
                  {#each gitCommits as commit}
                    <div class="flex items-start gap-3 p-2 rounded-lg border border-[var(--color-border)] hover:bg-[var(--color-surface)] cursor-pointer"
                         onclick={() => gitCheckout(commit.hash)}>
                      <span class="material-symbols-outlined text-sm text-[var(--color-text-muted)] mt-0.5">commit</span>
                      <div class="flex-1 min-w-0">
                        <p class="text-sm text-[var(--color-text)] truncate">{commit.message}</p>
                        <div class="flex items-center gap-2 text-[10px] text-[var(--color-text-secondary)] mt-0.5">
                          <span class="font-mono">{commit.hash.slice(0, 8)}</span>
                          <span>{commit.author}</span>
                          <span>{new Date(commit.timestamp).toLocaleString('zh-CN')}</span>
                        </div>
                      </div>
                      {#if commit.hash === gitHeadHash}
                        <span class="text-[10px] px-1.5 py-0.5 rounded bg-green-100 text-green-700">HEAD</span>
                      {/if}
                    </div>
                  {/each}
                {/if}
              </div>
            </div>
          </div>
        {/if}

<!-- Mirror Panel -->
{#if showMirrorPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-4xl w-full mx-4 max-h-[90vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">真机投屏</h2>
        <button onclick={() => { stopMirror(); showMirrorPanel = false; }}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="flex gap-3 mb-4 flex-wrap items-end">
        <div class="flex flex-col gap-1">
          <label class="text-[11px] font-medium text-[var(--color-text-secondary)]">设备</label>
          <select
            class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] min-w-[200px]"
            bind:value={mirrorDevice}
            disabled={mirroring}
          >
            <option value="">选择设备</option>
            {#each adbDevices as dev}
              <option value={dev.serial}>{dev.serial} ({dev.model || 'Unknown'})</option>
            {/each}
          </select>
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-[11px] font-medium text-[var(--color-text-secondary)]">帧率</label>
          <select
            class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]"
            bind:value={mirrorFPS}
            disabled={mirroring}
          >
            <option value={1}>1 FPS</option>
            <option value={2}>2 FPS</option>
            <option value={3}>3 FPS</option>
            <option value={5}>5 FPS</option>
            <option value={10}>10 FPS</option>
          </select>
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-[11px] font-medium text-[var(--color-text-secondary)]">画面比例</label>
          <select
            class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]"
            bind:value={mirrorAspect}
          >
            <option value="contain">适应</option>
            <option value="cover">填充</option>
            <option value="stretch">拉伸</option>
          </select>
        </div>
        {#if !mirroring}
          <button class="btn-primary" onclick={startMirror} disabled={!mirrorDevice}>
            <span class="material-symbols-outlined" slot="start">play_arrow</span>
            开始投屏
          </button>
        {:else}
          <button class="btn-primary" onclick={stopMirror}>
            <span class="material-symbols-outlined" slot="start">stop</span>
            停止投屏
          </button>
          <button class="btn-ghost border border-[var(--color-border)]" onclick={captureMirrorFrame}>
            <span class="material-symbols-outlined" slot="start">photo_camera</span>
            截图
          </button>
        {/if}
      </div>

      <div class="flex-1 overflow-auto bg-black rounded-xl flex items-center justify-center min-h-[300px]">
        {#if mirroring && mirrorURL}
          <div class="mirror-container w-full h-full flex items-center justify-center">
            <img
              src={mirrorURL}
              alt="Device Screen"
              class="max-w-full max-h-[60vh]"
              style="object-fit: {mirrorAspect}; image-rendering: auto;"
            />
          </div>
        {:else}
          <div class="text-center text-[var(--color-text-secondary)] py-12">
            <span class="material-symbols-outlined text-5xl mb-2">screen_share</span>
            <p>选择设备后点击开始投屏</p>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- Update Check Panel -->
{#if showUpdatePanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-2xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">检查更新</h2>
        <button onclick={() => showUpdatePanel = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="space-y-3 mb-4">
        <div class="flex flex-col gap-1">
          <label class="text-[11px] font-medium text-[var(--color-text-secondary)]">当前版本</label>
          <input
            type="text"
            placeholder="v1.0"
            class="px-4 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]"
            bind:value={updateModuleVersion}
          />
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-[11px] font-medium text-[var(--color-text-secondary)]">GitHub 仓库 URL</label>
          <input
            type="text"
            placeholder="https://github.com/user/repo"
            class="px-4 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]"
            bind:value={updateModuleRepo}
          />
        </div>
        <button class="btn-primary" onclick={checkModuleUpdate} disabled={updateChecking || !updateModuleRepo}>
          <span class="material-symbols-outlined" slot="start">system_update</span>
          {updateChecking ? '检查中...' : '检查更新'}
        </button>
      </div>

      {#if updateResult}
        <div class="border border-[var(--color-border)] rounded-xl p-4">
          {#if updateResult.has_update}
            <div class="flex items-center gap-2 mb-3">
              <span class="material-symbols-outlined text-green-600">arrow_upward</span>
              <span class="text-base font-semibold text-green-600">有新版本可用</span>
            </div>
            <div class="space-y-2 text-xs">
              <div class="flex justify-between">
                <span class="text-[var(--color-text-secondary)]">当前版本</span>
                <span class="font-mono">{updateResult.current_version}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-[var(--color-text-secondary)]">最新版本</span>
                <span class="font-mono font-bold">{updateResult.latest_version}</span>
              </div>
              {#if updateResult.download_url}
                <div class="mt-3">
                  <a href={updateResult.download_url} target="_blank" rel="noopener">
                    <button class="btn-ghost border border-[var(--color-border)]">
                      <span class="material-symbols-outlined" slot="start">download</span>
                      下载最新版本
                    </button>
                  </a>
                </div>
              {/if}
              {#if updateResult.release_note}
                <div class="mt-3 p-3 bg-[var(--color-surface)] rounded-lg">
                  <p class="text-[11px] font-medium text-[var(--color-text-secondary)] mb-1">Release Notes</p>
                  <p class="text-xs whitespace-pre-wrap">{updateResult.release_note}</p>
                </div>
              {/if}
            </div>
          {:else}
            <div class="flex items-center gap-2">
              <span class="material-symbols-outlined text-green-600">check_circle</span>
              <span class="text-base font-semibold text-green-600">已是最新版本</span>
            </div>
            {#if updateResult.error}
              <p class="text-xs text-[var(--color-text-secondary)] mt-2">{updateResult.error}</p>
            {/if}
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Benchmark Panel -->
{#if showBenchmarkPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[85vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">性能基准测试</h2>
        <button onclick={() => showBenchmarkPanel = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="flex gap-3 mb-4 items-end">
        <div class="flex flex-col gap-1">
          <label class="text-[11px] font-medium text-[var(--color-text-secondary)]">选择设备</label>
          <select
            class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] min-w-[200px]"
            bind:value={benchmarkDevice}
          >
            <option value="">选择设备</option>
            {#each adbDevices as dev}
              <option value={dev.serial}>{dev.serial} ({dev.model || 'Unknown'})</option>
            {/each}
          </select>
        </div>
        <button class="btn-primary" onclick={runBenchmark} disabled={benchmarkRunning || !benchmarkDevice}>
          <span class="material-symbols-outlined" slot="start">speed</span>
          {benchmarkRunning ? '测试中...' : '开始测试'}
        </button>
        <button class="btn-ghost border border-[var(--color-border)]" onclick={loadBenchmarkHistory}>
          <span class="material-symbols-outlined" slot="start">history</span>
          历史记录
        </button>
      </div>

      <div class="flex-1 overflow-auto space-y-4">
        {#if benchmarkResult}
          <div class="border border-[var(--color-border)] rounded-xl p-4">
            <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
              <span class="material-symbols-outlined text-sm">speed</span>
              测试结果
            </h3>
            <div class="grid grid-cols-2 gap-3">
              {#each Object.entries(benchmarkResult.before || {}) as [key, value]}
                <div class="p-3 bg-[var(--color-surface)] rounded-lg">
                  <p class="text-[11px] font-medium text-[var(--color-text-secondary)] mb-1">{key}</p>
                  <p class="text-xs font-mono">{String(value).substring(0, 120)}</p>
                </div>
              {/each}
            </div>
            {#if benchmarkResult.diff?.note}
              <div class="mt-3 p-3 bg-primary-600-50 rounded-lg">
                <p class="text-xs text-primary-700">{benchmarkResult.diff.note}</p>
              </div>
            {/if}
          </div>
        {/if}

        {#if benchmarkHistory.length > 0}
          <div class="border border-[var(--color-border)] rounded-xl p-4">
            <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
              <span class="material-symbols-outlined text-sm">history</span>
              历史记录
            </h3>
            <div class="space-y-2">
              {#each benchmarkHistory as bench}
                <div class="p-3 bg-[var(--color-surface)] rounded-lg">
                  <div class="flex items-center justify-between mb-1">
                    <span class="text-xs font-mono">{bench.id}</span>
                    <span class="text-xs text-[var(--color-text-secondary)]">{new Date(bench.created_at).toLocaleString('zh-CN')}</span>
                  </div>
                  <div class="flex gap-4 text-xs text-[var(--color-text-secondary)]">
                    <span>设备: {bench.device_serial}</span>
                    <span>模块: {bench.module_id}</span>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        {#if !benchmarkResult && benchmarkHistory.length === 0}
          <div class="text-center text-[var(--color-text-secondary)] py-12">
            <span class="material-symbols-outlined text-5xl mb-2">speed</span>
            <p>选择设备后点击开始测试</p>
            <p class="text-xs mt-1">测试将采集 CPU、内存、存储等设备性能数据</p>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- Team Panel -->
{#if showTeamPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-2xl w-full mx-4 max-h-[85vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">团队管理</h2>
        <button onclick={() => showTeamPanel = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>
      <div class="flex-1 overflow-auto">
        <TeamManager {projectId} />
      </div>
    </div>
  </div>
{/if}

<!-- Collaboration Panel -->
{#if showCollabPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[85vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">团队协作</h2>
        <div class="flex items-center gap-3">
          <span class="text-xs {collabWsConnected ? 'text-green-600' : 'text-red-600'}">
            {collabWsConnected ? '● 已连接' : '○ 未连接'}
          </span>
          <button onclick={() => showCollabPanel = false}>
            <span class="material-symbols-outlined">close</span>
          </button>
        </div>
      </div>

      <!-- Username -->
      <div class="flex gap-2 mb-4">
        <input
          type="text"
          placeholder="你的用户名..."
          class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] flex-1"
          bind:value={collabUsername}
        />
      </div>

      <div class="flex-1 overflow-auto space-y-4">
        <!-- Collaborators -->
        <div class="border border-[var(--color-border)] rounded-xl p-4">
          <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
            <span class="material-symbols-outlined text-sm">group</span>
            协作者
          </h3>
          {#if collaborators.length > 0}
            <div class="space-y-2 mb-3">
              {#each collaborators as c}
                <div class="flex items-center justify-between p-2 bg-[var(--color-surface)] rounded-lg">
                  <div class="flex items-center gap-2">
                    <div class="w-8 h-8 rounded-full flex items-center justify-center text-white text-sm font-bold" style="background-color: {COLLAB_COLORS[collaborators.indexOf(c) % COLLAB_COLORS.length]}">
                      {(c.username || c.user_id)[0]?.toUpperCase() || '?'}
                    </div>
                    <div>
                      <p class="text-xs font-medium">{c.username || c.user_id}</p>
                      <p class="text-xs text-[var(--color-text-secondary)]">{c.role}</p>
                    </div>
                  </div>
                  <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={() => removeCollaborator(c.user_id)}>
                    <span class="material-symbols-outlined text-sm">close</span>
                  </button>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-xs text-[var(--color-text-secondary)] mb-3">暂无协作者</p>
          {/if}
          <div class="flex gap-2">
            <input
              type="text"
              placeholder="用户名"
              class="px-3 py-1 border border-[var(--color-border)] rounded bg-[var(--color-bg)] text-[var(--color-text)] text-sm flex-1"
              bind:value={collabInviteUser}
            />
            <select class="px-2 py-1 border border-[var(--color-border)] rounded bg-[var(--color-bg)] text-[var(--color-text)] text-sm" bind:value={collabInviteRole}>
              <option value="editor">编辑者</option>
              <option value="viewer">查看者</option>
              <option value="admin">管理员</option>
            </select>
            <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={inviteCollaborator}>邀请</button>
          </div>
        </div>

        <!-- Active editors -->
        <div class="border border-[var(--color-border)] rounded-xl p-4">
          <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
            <span class="material-symbols-outlined text-sm">edit</span>
            活跃编辑者
          </h3>
          {#if collabSessions.length > 0}
            <div class="space-y-2">
              {#each collabSessions as s}
                <div class="flex items-center gap-3 p-2 bg-[var(--color-surface)] rounded-lg">
                  <div class="w-3 h-3 rounded-full" style="background-color: {s.color}"></div>
                  <div>
                    <p class="text-xs font-medium">{s.username || s.user_id}</p>
                    <p class="text-xs text-[var(--color-text-secondary)]">编辑 {s.file_path} · 行 {s.cursor_line}, 列 {s.cursor_col}</p>
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-xs text-[var(--color-text-secondary)]">无其他编辑者在线</p>
          {/if}
        </div>

        <!-- Comments -->
        <div class="border border-[var(--color-border)] rounded-xl p-4">
          <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
            <span class="material-symbols-outlined text-sm">comment</span>
            评论
          </h3>
          {#if collabComments.length > 0}
            <div class="space-y-2 mb-3 max-h-48 overflow-auto">
              {#each collabComments as c}
                <div class="p-2 rounded-lg border" style={c.resolved ? 'border-color: var(--color-success); background: var(--color-success-light)' : 'border-color: var(--color-border); background: var(--color-surface)'}>
                  <div class="flex items-center justify-between mb-1">
                    <div class="flex items-center gap-2">
                      <span class="text-xs font-medium">{c.username}</span>
                      <span class="text-xs text-[var(--color-text-secondary)]">{c.file_path}:{c.line_number}</span>
                    </div>
                    {#if !c.resolved}
                      <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={() => resolveComment(c.id)}>解决</button>
                    {:else}
                      <span class="text-xs text-green-600">已解决</span>
                    {/if}
                  </div>
                  <p class="text-xs">{c.content}</p>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-xs text-[var(--color-text-secondary)] mb-3">暂无评论</p>
          {/if}
          <div class="space-y-2">
            <div class="flex gap-2">
              <input type="text" placeholder="文件路径" class="px-2 py-1 border border-[var(--color-border)] rounded bg-[var(--color-bg)] text-[var(--color-text)] text-sm flex-1" bind:value={commentFilePath} />
              <input type="number" placeholder="行号" class="px-2 py-1 border border-[var(--color-border)] rounded bg-[var(--color-bg)] text-[var(--color-text)] text-sm w-20" bind:value={commentLineNumber} />
            </div>
            <div class="flex gap-2">
              <input type="text" placeholder="评论内容..." class="px-3 py-1 border border-[var(--color-border)] rounded bg-[var(--color-bg)] text-[var(--color-text)] text-sm flex-1" bind:value={commentContent} />
              <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={addCollabComment}>发送</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- Activity Feed -->
<div class="border border-[var(--color-border)] rounded-xl p-4 mb-4">
  <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
    <span class="material-symbols-outlined text-sm">timeline</span>
    最近活动
  </h3>
  {#if activities.length > 0}
    <div class="space-y-2 max-h-48 overflow-auto">
      {#each activities as act}
        <div class="flex items-start gap-2 text-xs">
          <span class="material-symbols-outlined text-[14px] mt-0.5 flex-shrink-0"
                style="color: {act.activity_type === 'build_completed' ? 'var(--color-success)' : act.activity_type === 'build_failed' ? 'var(--color-error)' : 'var(--color-primary)'}">
            {act.activity_type === 'build_completed' ? 'check_circle' : act.activity_type === 'build_failed' ? 'error' : act.activity_type === 'file_edited' ? 'edit_note' : 'circle'}
          </span>
          <div>
            <p style="color: var(--color-text)">{act.description}</p>
            <p class="text-[10px] mt-0.5" style="color: var(--color-text-muted)">{new Date(act.created_at).toLocaleString()}</p>
          </div>
        </div>
      {/each}
    </div>
  {:else}
    <p class="text-xs text-[var(--color-text-secondary)]">暂无活动</p>
  {/if}
</div>

<!-- Plugin Panel -->
{#if showPluginPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-[var(--color-bg)] rounded-2xl p-6 max-w-2xl w-full mx-4 max-h-[85vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">插件系统</h2>
        <button onclick={() => showPluginPanel = false}>
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>

      <div class="flex-1 overflow-auto space-y-4">
        <!-- Install Plugin -->
        <div class="border border-[var(--color-border)] rounded-xl p-4">
          <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
            <span class="material-symbols-outlined text-sm">add</span>
            安装插件
          </h3>
          <div class="grid grid-cols-2 gap-2 mb-2">
            <input type="text" placeholder="插件名称" class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={pluginInstallName} />
            <input type="text" placeholder="slug (唯一标识)" class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={pluginInstallSlug} />
            <input type="text" placeholder="描述" class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={pluginInstallDesc} />
            <input type="text" placeholder="作者" class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={pluginInstallAuthor} />
          </div>
          <button class="btn-primary" onclick={installPlugin} disabled={!pluginInstallName.trim() || !pluginInstallSlug.trim()}>
            <span class="material-symbols-outlined" slot="start">download</span>
            安装
          </button>
        </div>

        <!-- Plugin List -->
        <div class="border border-[var(--color-border)] rounded-xl p-4">
          <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
            <span class="material-symbols-outlined text-sm">extension</span>
            已安装插件
            <button class="btn-ghost border border-[var(--color-border)] ml-auto text-xs px-3 py-1" onclick={loadPlugins}>刷新</button>
          </h3>
          {#if pluginList.length > 0}
            <div class="space-y-2">
              {#each pluginList as p}
                <div class="p-3 bg-[var(--color-surface)] rounded-lg">
                  <div class="flex items-center justify-between mb-2">
                    <div>
                      <div class="flex items-center gap-2">
                        <span class="text-xs font-medium">{p.name}</span>
                        <span class="text-xs text-[var(--color-text-secondary)]">v{p.version}</span>
                        <span class="px-2 py-0.5 rounded-full text-xs" style={p.enabled ? 'background: var(--color-success-light); color: var(--color-success)' : 'background: var(--color-surface); color: var(--color-text-muted)'}>
                          {p.enabled ? '已启用' : '已禁用'}
                        </span>
                      </div>
                      <p class="text-xs text-[var(--color-text-secondary)] mt-1">{p.description} · {p.author}</p>
                    </div>
                    <div class="flex gap-1">
                      <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={() => { selectedPluginId = p.id; }}>
                        钩子
                      </button>
                      <button class="btn-ghost border border-[var(--color-border)] text-xs px-3 py-1" onclick={() => togglePlugin(p.id, !p.enabled)}>
                        {p.enabled ? '禁用' : '启用'}
                      </button>
                      <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={() => uninstallPlugin(p.id)}>
                        <span class="material-symbols-outlined text-sm">delete</span>
                      </button>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-xs text-[var(--color-text-secondary)]">暂无已安装插件</p>
          {/if}
        </div>

        <!-- Register Hook -->
        {#if selectedPluginId}
          <div class="border border-[var(--color-border)] rounded-xl p-4">
            <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
              <span class="material-symbols-outlined text-sm">webhook</span>
              注册钩子
            </h3>
            <div class="grid grid-cols-2 gap-2 mb-2">
              <input type="text" placeholder="钩子名称 (e.g. pre_save)" class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={pluginHookName} />
              <select class="px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={pluginHookType}>
                <option value="pre_save">pre_save</option>
                <option value="post_save">post_save</option>
                <option value="pre_build">pre_build</option>
                <option value="post_build">post_build</option>
                <option value="on_comment">on_comment</option>
              </select>
            </div>
            <input type="text" placeholder="入口 (e.g. my-plugin/handler.js)" class="w-full px-3 py-2 border border-[var(--color-border)] rounded-lg bg-[var(--color-bg)] text-[var(--color-text)] mb-2" bind:value={pluginHookEntry} />
            <button class="btn-ghost border border-[var(--color-border)]" onclick={registerHook} disabled={!pluginHookName.trim() || !pluginHookEntry.trim()}>
              <span class="material-symbols-outlined" slot="start">add</span>
              注册钩子
            </button>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

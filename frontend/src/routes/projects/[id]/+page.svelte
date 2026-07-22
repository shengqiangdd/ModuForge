<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '../../../lib/api/client';
  import CodeEditor from '../../../lib/components/CodeEditor.svelte';
  import PreviewPanel from '../../../lib/components/PreviewPanel.svelte';

  const id = window.location.pathname.split('/').filter(Boolean).pop() || '';

  let files = $state<{path: string; content?: string}[]>([]);
  let selectedFile = $state<string | null>(null);
  let editorContent = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let project = $state<any>(null);
  let activeTab = $state<'editor' | 'preview'>('editor');

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
  let verifyResult = $state<{valid: boolean} | null>(null);

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

  // Plugin system
  let showPluginPanel = $state(false);
  let pluginList = $state<Array<{id: string, name: string, slug: string, description: string, author: string, version: string, enabled: boolean}>>([]);
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
        templates = await response.json();
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
      const res = await fetch('/api/v1/sign', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ zip_path: `data/storage/downloads/moduforge_module_${id}.zip` })
      });
      if (res.ok) {
        signatureInfo = await res.json();
      } else {
        const data = await res.json();
        adbOutput = data.error || 'Sign failed';
      }
    } catch {
      adbOutput = 'Sign service unavailable';
    }
    signing = false;
  }

  async function verifyModule() {
    if (!signatureInfo) return;
    verifying = true;
    try {
      const res = await fetch('/api/v1/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          zip_path: `data/storage/downloads/moduforge_module_${id}.zip`,
          expected_hash: signatureInfo.hash
        })
      });
      if (res.ok) {
        verifyResult = await res.json();
      }
    } catch {
      verifyResult = { valid: false };
    }
    verifying = false;
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
          collabSessions = collabSessions.filter(s => s.user_id !== msg.payload?.user_id);
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
    try {
      project = await client.get(`/projects/${id}`);
      const fileData = await client.get<{path: string}[]>(`/projects/${id}/files`);
      files = fileData.map(f => ({ path: f.path }));
    } catch(e) {
      console.error(e);
    } finally {
      loading = false;
    }
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
  <aside class="w-64 bg-surface-container-high border-r border-outline-variant flex flex-col">
    <div class="p-4 border-b border-outline-variant">
      <h2 class="text-title-medium text-on-surface truncate">{project?.name || '加载中...'}</h2>
    </div>

    <div class="flex-1 overflow-auto p-2">
      {#if loading}
        <div class="flex justify-center p-4"><md-circular-progress indeterminate /></div>
      {:else if files.length === 0}
        <p class="text-body-small text-on-surface-variant p-4">暂无文件</p>
      {:else}
        <md-list>
          {#each files as file}
            <md-list-item
              class="cursor-pointer"
              class:bg-primary-container={selectedFile === file.path}
              onclick={() => selectFile(file.path)}
            >
              <span slot="start" class="text-lg mr-2">{getFileIcon(file.path)}</span>
              <span>{file.path}</span>
            </md-list-item>
          {/each}
        </md-list>
      {/if}
    </div>

    <div class="p-3 border-t border-outline-variant space-y-2">
      <md-filled-button class="w-full" onclick={() => { showRepoDialog = true; }}>
        <md-icon slot="start">link</md-icon>
        导入 GitHub 仓库
      </md-filled-button>
      <md-filled-button class="w-full" onclick={() => { showTemplatePanel = true; loadTemplates(); }}>
        <md-icon slot="start">description</md-icon>
        模板推荐
      </md-filled-button>
      <md-filled-button class="w-full" onclick={() => showBuildLog = true}>
        <md-icon slot="start">terminal</md-icon>
        构建日志
      </md-filled-button>
      <md-filled-button class="w-full" href="/projects/{id}/build">
        <md-icon slot="start">build</md-icon>
        构建模块
      </md-filled-button>
      <md-filled-button class="w-full" onclick={exportZip} disabled={exporting}>
        <md-icon slot="start">archive</md-icon>
        {exporting ? '打包中...' : '导出模块 ZIP'}
      </md-filled-button>
      <md-filled-button class="w-full" onclick={signModule} disabled={signing}>
        <md-icon slot="start">verified</md-icon>
        {signing ? '签名中...' : '签名模块'}
      </md-filled-button>
      <md-filled-button class="w-full" onclick={() => { showGitPanel = !showGitPanel; if (showGitPanel) loadGitCommits(); }}>
        <md-icon slot="start">history</md-icon>
        版本历史 ⏱
      </md-filled-button>
      <md-filled-button class="w-full" onclick={() => { showADBPanels = !showADBPanels; if (showADBPanels && adbAvailable === null) checkADB(); }}>
        <md-icon slot="start">phone_android</md-icon>
        设备
      </md-filled-button>
      <md-filled-button class="w-full" onclick={() => { showScreenshotPanel = !showScreenshotPanel; if (showScreenshotPanel && adbDevices.length === 0) checkADB(); }}>
        <md-icon slot="start">photo_camera</md-icon>
        真机截图
      </md-filled-button>
      <md-filled-button class="w-full" onclick={() => { showMirrorPanel = !showMirrorPanel; if (showMirrorPanel && adbDevices.length === 0) checkADB(); }}>
        <md-icon slot="start">screen_share</md-icon>
        真机投屏
      </md-filled-button>
      <md-filled-button class="w-full" onclick={() => showUpdatePanel = !showUpdatePanel}>
        <md-icon slot="start">system_update</md-icon>
        检查更新
      </md-filled-button>
      <md-filled-button class="w-full" onclick={() => { showBenchmarkPanel = !showBenchmarkPanel; if (showBenchmarkPanel && adbDevices.length === 0) { checkADB(); loadBenchmarkHistory(); } }}>
        <md-icon slot="start">speed</md-icon>
        性能测试
      </md-filled-button>
      <md-filled-button class="w-full" onclick={openCollabPanel}>
        <md-icon slot="start">group</md-icon>
        协作 👥
      </md-filled-button>
      <md-filled-button class="w-full" onclick={() => showPluginPanel = !showPluginPanel}>
        <md-icon slot="start">extension</md-icon>
        插件
      </md-filled-button>
    </div>
  </aside>

  <!-- 编辑器 -->
  <main class="flex-1 flex flex-col">
    {#if selectedFile}
      <!-- 标签页 -->
      <div class="flex items-center justify-between px-4 py-2 border-b border-outline-variant bg-surface">
        <div class="flex items-center gap-4">
          <button
            class="px-3 py-1 text-body-medium transition-colors"
            class:text-primary={activeTab === 'editor'}
            class:text-on-surface-variant={activeTab !== 'editor'}
            class:border-b-2={activeTab === 'editor'}
            class:border-primary={activeTab === 'editor'}
            onclick={() => activeTab = 'editor'}
          >
            编辑
          </button>
          <button
            class="px-3 py-1 text-body-medium transition-colors"
            class:text-primary={activeTab === 'preview'}
            class:text-on-surface-variant={activeTab !== 'preview'}
            class:border-b-2={activeTab === 'preview'}
            class:border-primary={activeTab === 'preview'}
            onclick={() => activeTab = 'preview'}
          >
            预览
          </button>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-body-medium text-on-surface font-mono">{selectedFile}</span>

          {#if selectedFile?.endsWith('.prop')}
            <select
              class="px-2 py-1 border border-outline rounded bg-surface text-on-surface text-sm"
              bind:value={translateLang}
            >
              <option value="en">English</option>
              <option value="zh">中文</option>
              <option value="ja">日本語</option>
              <option value="ko">한국어</option>
            </select>
            <md-filled-tonal-button size="small" onclick={translateDescription} disabled={translating}>
              <md-icon slot="start">translate</md-icon>
              {translating ? '翻译中...' : '翻译'}
            </md-filled-tonal-button>
          {/if}

          <md-filled-tonal-button size="small" onclick={saveFile} disabled={saving}>
            {saving ? '保存中...' : '保存'}
          </md-filled-tonal-button>
          <md-filled-tonal-button size="small" onclick={validateCurrentFile} disabled={validating}>
            <md-icon slot="start">check_circle</md-icon>
            {validating ? '校验中...' : '校验'}
          </md-filled-tonal-button>
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
        {:else}
          <PreviewPanel htmlContent={generatePreviewContent()} />
        {/if}
      </div>

      {#if translatedDesc && selectedFile?.endsWith('.prop')}
        <div class="p-3 border-t border-outline-variant bg-surface-container">
          <div class="flex items-center gap-2 mb-2">
            <md-icon class="text-sm">translate</md-icon>
            <span class="text-label-medium">翻译结果</span>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div class="p-3 bg-surface rounded-lg border border-outline">
              <p class="text-label-small text-on-surface-variant mb-1">原文</p>
              <p class="text-body-small">{originalDesc}</p>
            </div>
            <div class="p-3 bg-surface rounded-lg border border-outline">
              <p class="text-label-small text-on-surface-variant mb-1">译文 ({translateLang})</p>
              <p class="text-body-small">{translatedDesc}</p>
            </div>
          </div>
        </div>
      {/if}

      {#if showValidation && validationResults.length > 0}
        <div class="border-t border-outline-variant bg-surface-container">
          <div class="flex items-center justify-between px-4 py-2">
            <div class="flex items-center gap-2">
              <md-icon class="text-sm">check_circle</md-icon>
              <span class="text-label-medium">语法校验结果</span>
            </div>
            <div class="flex items-center gap-2">
              <md-filled-tonal-button size="small" onclick={validateAllFiles} disabled={validating}>
                校验全部文件
              </md-filled-tonal-button>
              <button onclick={() => showValidation = false}>
                <md-icon class="text-sm">close</md-icon>
              </button>
            </div>
          </div>
          <div class="px-4 pb-3 space-y-2 max-h-48 overflow-auto">
            {#each validationResults as vr}
              <div class="p-2 rounded-lg border {vr.valid ? 'border-green-400 bg-green-50' : 'border-red-400 bg-red-50'}">
                <div class="flex items-center gap-2 mb-1">
                  <md-icon class="text-sm {vr.valid ? 'text-green-600' : 'text-red-600'}">
                    {vr.valid ? 'check_circle' : 'error'}
                  </md-icon>
                  <span class="text-body-small font-medium">{vr.file}</span>
                  {#if vr.valid}
                    <span class="text-body-small text-green-600">通过</span>
                  {/if}
                </div>
                {#each vr.errors as err}
                  <p class="text-body-small text-red-600 ml-6">{err}</p>
                {/each}
                {#each vr.warnings as warn}
                  <p class="text-body-small text-amber-600 ml-6">{warn}</p>
                {/each}
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {:else}
      <div class="flex-1 flex items-center justify-center text-on-surface-variant">
        <div class="text-center">
          <md-icon class="text-5xl mb-2">edit_document</md-icon>
          <p class="text-body-large">选择一个文件开始编辑</p>
        </div>
      </div>
    {/if}
  </main>
</div>

<!-- Repo Dialog -->
{#if showRepoDialog}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-surface rounded-2xl p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-auto">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">导入 GitHub 仓库</h2>
        <button onclick={() => showRepoDialog = false}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <div class="flex gap-2 mb-4">
        <input
          type="text"
          placeholder="https://github.com/user/repo"
          class="flex-1 px-4 py-2 border border-outline rounded-lg bg-surface text-on-surface"
          bind:value={repoUrl}
        />
        <md-filled-button onclick={fetchRepoInfo} disabled={repoLoading}>
          {repoLoading ? '获取中...' : '获取'}
        </md-filled-button>
      </div>

      {#if repoInfo}
        <div class="border border-outline rounded-xl p-4 mb-4">
          <div class="flex items-center gap-2 mb-2">
            <md-icon>folder</md-icon>
            <span class="text-title-medium">{repoInfo.owner}/{repoInfo.name}</span>
          </div>
          <div class="flex gap-4 text-body-small text-on-surface-variant">
            <span>⭐ {repoInfo.stars}</span>
            <span>📄 {repoInfo.license || 'N/A'}</span>
          </div>
          {#if repoInfo.topics?.length}
            <div class="flex flex-wrap gap-2 mt-2">
              {#each repoInfo.topics as topic}
                <span class="px-2 py-1 bg-primary-container text-on-primary-container rounded-full text-xs">{topic}</span>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      {#if repoFiles.length}
        <div class="border border-outline rounded-xl p-4">
          <h3 class="text-title-small mb-2">仓库文件</h3>
          <md-list>
            {#each repoFiles as file}
              <md-list-item>
                <md-icon slot="start">{file.type === 'dir' ? 'folder' : 'description'}</md-icon>
                <span>{file.name}</span>
              </md-list-item>
            {/each}
          </md-list>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Template Panel -->
{#if showTemplatePanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-surface rounded-2xl p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-auto">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">模板推荐</h2>
        <button onclick={() => showTemplatePanel = false}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <input
        type="text"
        placeholder="输入描述搜索模板..."
        class="w-full px-4 py-2 border border-outline rounded-lg bg-surface text-on-surface mb-4"
        bind:value={templateSearch}
      />

      <div class="grid grid-cols-1 gap-3">
        {#each templates.filter(t => !templateSearch || t.description?.toLowerCase().includes(templateSearch.toLowerCase())) as tmpl}
          <button
            class="p-4 border border-outline rounded-xl text-left hover:bg-surface-container transition-colors"
            onclick={() => applyTemplate(tmpl)}
          >
            <div class="flex items-center gap-2 mb-1">
              <md-icon>{tmpl.category === 'system' ? 'settings' : tmpl.category === 'ui' ? 'palette' : 'extension'}</md-icon>
              <span class="text-title-small">{tmpl.name}</span>
            </div>
            <p class="text-body-small text-on-surface-variant">{tmpl.description}</p>
          </button>
        {/each}
      </div>
    </div>
  </div>
{/if}

<!-- AI Chat Bubble -->
<button
  class="fixed bottom-6 right-6 w-14 h-14 bg-primary text-on-primary rounded-full shadow-lg flex items-center justify-center hover:bg-primary/90 transition-colors z-40"
  onclick={() => chatOpen = !chatOpen}
>
  <span class="text-2xl">{chatOpen ? '✕' : '💬'}</span>
</button>

{#if chatOpen}
  <div class="fixed bottom-24 right-6 w-96 h-[500px] bg-surface rounded-2xl shadow-2xl flex flex-col z-50 border border-outline">
    <div class="p-4 border-b border-outline-variant flex items-center justify-between">
      <div class="flex items-center gap-2">
        <span class="text-lg">🤖</span>
        <span class="text-title-medium">AI 助手</span>
      </div>
      <button onclick={() => chatOpen = false}>
        <md-icon>close</md-icon>
      </button>
    </div>

    <div class="flex-1 overflow-auto p-4 space-y-4">
      {#if chatMessages.length === 0}
        <div class="text-center text-on-surface-variant py-8">
          <p class="text-body-large mb-2">👋 你好！</p>
          <p class="text-body-small">我是 Magisk/KSU 模块开发助手，有什么可以帮你的？</p>
        </div>
      {/if}
      {#each chatMessages as msg}
        <div class="flex {msg.role === 'user' ? 'justify-end' : 'justify-start'}">
          <div class="max-w-[80%] {msg.role === 'user' ? 'bg-primary text-on-primary' : 'bg-surface-container text-on-surface'} rounded-2xl px-4 py-2">
            <p class="text-body-small whitespace-pre-wrap">{msg.content}{chatStreaming && msg.role === 'assistant' && msg === chatMessages[chatMessages.length - 1] ? '▊' : ''}</p>
          </div>
        </div>
      {/each}
    </div>

    <div class="p-4 border-t border-outline-variant">
      <div class="flex gap-2">
        <input
          type="text"
          placeholder="输入消息..."
          class="flex-1 px-4 py-2 border border-outline rounded-lg bg-surface text-on-surface"
          bind:value={chatInput}
          onkeydown={handleChatKeydown}
          disabled={chatStreaming}
        />
        <md-filled-button onclick={sendChatMessage} disabled={chatStreaming || !chatInput.trim()}>
          <md-icon slot="start">send</md-icon>
        </md-filled-button>
      </div>
    </div>
  </div>
{/if}

<!-- Build Log Panel -->
{#if showBuildLog}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-surface rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">构建日志</h2>
        <button onclick={() => showBuildLog = false}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <div class="flex gap-2 mb-4">
        <input
          type="text"
          placeholder="输入构建 ID..."
          class="flex-1 px-4 py-2 border border-outline rounded-lg bg-surface text-on-surface"
          bind:value={buildId}
        />
        <md-filled-button onclick={loadBuildLogs}>
          <md-icon slot="start">refresh</md-icon>
          加载日志
        </md-filled-button>
      </div>

      <div class="flex-1 overflow-auto bg-surface-container rounded-xl p-4 font-mono text-sm">
        {#if buildLogs.length === 0}
          <p class="text-on-surface-variant">暂无日志</p>
        {:else}
          {#each buildLogs as log}
            <div class="mb-1 {
              log.level === 'ERROR' ? 'text-error' :
              log.level === 'WARN' ? 'text-yellow' :
              log.level === 'SUCCESS' ? 'text-green' :
              'text-on-surface'
            }">
              <span class="text-on-surface-variant">[{log.timestamp}]</span>
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
    <div class="bg-surface rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">版本历史</h2>
        <button onclick={() => showGitPanel = false}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <div class="flex-1 overflow-auto mb-4">
        {#if gitLoading}
          <div class="flex justify-center p-4"><md-circular-progress indeterminate /></div>
        {:else if gitCommits.length === 0}
          <p class="text-on-surface-variant text-center py-8">暂无提交历史</p>
        {:else}
          <div class="space-y-2">
            {#each gitCommits as commit}
              <div class="p-3 rounded-xl border {commit.hash === gitHeadHash ? 'border-primary bg-primary-container' : 'border-outline'}">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="font-mono text-sm font-bold">{commit.hash.substring(0, 8)}</span>
                    <span class="text-body-small">{commit.message}</span>
                    {#if commit.hash === gitHeadHash}
                      <span class="px-2 py-0.5 bg-primary text-on-primary rounded-full text-xs">HEAD</span>
                    {/if}
                  </div>
                  <div class="flex items-center gap-2">
                    <span class="text-body-small text-on-surface-variant">{commit.author}</span>
                    <span class="text-body-small text-on-surface-variant">{new Date(commit.timestamp).toLocaleString('zh-CN')}</span>
                    {#if commit.hash !== gitHeadHash}
                      <md-filled-tonal-button size="small" onclick={() => gitCheckout(commit.hash)}>恢复</md-filled-tonal-button>
                    {/if}
                  </div>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <div class="border-t border-outline-variant pt-4">
        <div class="flex gap-2">
          <input
            type="text"
            placeholder="输入版本描述..."
            class="flex-1 px-4 py-2 border border-outline rounded-lg bg-surface text-on-surface"
            bind:value={gitMessage}
            onkeydown={(e) => { if (e.key === 'Enter') saveGitCommit(); }}
          />
          <md-filled-button onclick={saveGitCommit} disabled={gitLoading || !gitMessage.trim()}>
            <md-icon slot="start">save</md-icon>
            保存版本
          </md-filled-button>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- ADB Device Panel -->
{#if showADBPanels}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-surface rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">ADB 设备管理</h2>
        <button onclick={() => showADBPanels = false}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <div class="flex gap-2 mb-4">
        <md-filled-button onclick={checkADB} disabled={adbChecking}>
          <md-icon slot="start">search</md-icon>
          {adbChecking ? '检测中...' : '检测 ADB'}
        </md-filled-button>
        {#if adbAvailable === false}
          <span class="text-error flex items-center gap-1">
            <md-icon class="text-sm">error</md-icon>
            ADB 未检测到
          </span>
        {/if}
      </div>

      {#if adbAvailable === true}
        <div class="flex-1 overflow-auto mb-4">
          {#if adbLoadingDevices}
            <div class="flex justify-center p-4"><md-circular-progress indeterminate /></div>
          {:else if adbDevices.length === 0}
            <p class="text-on-surface-variant text-center py-8">未发现设备</p>
          {:else}
            <div class="space-y-2">
              {#each adbDevices as dev}
                <div class="p-3 rounded-xl border border-outline">
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                      <span class="font-mono text-sm font-bold">{dev.serial}</span>
                      <span class="text-body-small">{dev.model || 'Unknown'}</span>
                      <span class="px-2 py-0.5 rounded-full text-xs {
                        dev.state === 'device' ? 'bg-green-100 text-green-800' :
                        dev.state === 'offline' ? 'bg-red-100 text-red-800' :
                        'bg-yellow-100 text-yellow-800'
                      }">{dev.state}</span>
                    </div>
                    <div class="flex gap-2">
                      {#if dev.state === 'device'}
                        <md-filled-tonal-button size="small" onclick={() => installModule(dev.serial)}>
                          <md-icon slot="start">download</md-icon>
                          安装模块
                        </md-filled-tonal-button>
                      {/if}
                      <md-filled-tonal-button size="small" onclick={() => rebootDevice(dev.serial)}>
                        <md-icon slot="start">refresh</md-icon>
                        重启
                      </md-filled-tonal-button>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      {#if adbOutput}
        <div class="border-t border-outline-variant pt-4">
          <div class="bg-surface-container rounded-xl p-4 font-mono text-sm max-h-32 overflow-auto">
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
    <div class="bg-surface rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">真机截图</h2>
        <button onclick={() => showScreenshotPanel = false}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <div class="flex gap-2 mb-4">
        <select
          class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface"
          bind:value={screenshotDevice}
        >
          <option value="">选择设备</option>
          {#each adbDevices as dev}
            <option value={dev.serial}>{dev.serial} ({dev.model || 'Unknown'})</option>
          {/each}
        </select>
        <md-filled-button onclick={takeScreenshot} disabled={screenshotLoading || !screenshotDevice}>
          <md-icon slot="start">photo_camera</md-icon>
          {screenshotLoading ? '截图中...' : '截图'}
        </md-filled-button>
        <md-filled-tonal-button onclick={streamScreenshots} disabled={screenshotStreaming || !screenshotDevice}>
          <md-icon slot="start">burst_mode</md-icon>
          {screenshotStreaming ? '连续截图中...' : '连续截图'}
        </md-filled-tonal-button>
      </div>

      <div class="flex-1 overflow-auto">
        {#if screenshotImages.length === 0}
          <div class="text-center text-on-surface-variant py-12">
            <md-icon class="text-5xl mb-2">photo_camera</md-icon>
            <p>选择设备后点击截图</p>
          </div>
        {:else}
          <div class="grid grid-cols-2 gap-3">
            {#each screenshotImages as img}
              <div class="border border-outline rounded-xl overflow-hidden">
                <div class="bg-surface-container p-2 text-body-small font-mono truncate">{img.filename}</div>
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
  <div class="fixed bottom-24 right-6 w-80 bg-surface rounded-2xl shadow-2xl z-50 border border-outline p-4">
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-2">
        <md-icon class="text-primary">verified</md-icon>
        <span class="text-title-medium">模块签名</span>
      </div>
      <button onclick={() => { signatureInfo = null; verifyResult = null; }}>
        <md-icon class="text-sm">close</md-icon>
      </button>
    </div>

    <div class="space-y-2 text-body-small">
      <div class="flex justify-between">
        <span class="text-on-surface-variant">算法</span>
        <span class="font-mono">{signatureInfo.algorithm}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-on-surface-variant">大小</span>
        <span class="font-mono">{(signatureInfo.size / 1024).toFixed(1)} KB</span>
      </div>
      <div class="flex justify-between">
        <span class="text-on-surface-variant">签名时间</span>
        <span class="font-mono">{new Date(signatureInfo.signed_at).toLocaleString('zh-CN')}</span>
      </div>
      <div>
        <span class="text-on-surface-variant">SHA256</span>
        <p class="font-mono text-xs break-all mt-1 bg-surface-container p-2 rounded">{signatureInfo.hash}</p>
      </div>
    </div>

    <div class="flex gap-2 mt-3">
      <md-filled-tonal-button size="small" onclick={verifyModule} disabled={verifying}>
        <md-icon slot="start">check_circle</md-icon>
        {verifying ? '验证中...' : '验证'}
      </md-filled-tonal-button>
    </div>

    {#if verifyResult}
      <div class="mt-2 p-2 rounded-lg {verifyResult.valid ? 'bg-green-50 border border-green-400' : 'bg-red-50 border border-red-400'}">
        <div class="flex items-center gap-2">
          <md-icon class="text-sm {verifyResult.valid ? 'text-green-600' : 'text-red-600'}">
            {verifyResult.valid ? 'check_circle' : 'error'}
          </md-icon>
          <span class="text-body-small">{verifyResult.valid ? '校验通过' : '校验失败'}</span>
        </div>
      </div>
    {/if}
  </div>
{/if}

<!-- Mirror Panel -->
{#if showMirrorPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-surface rounded-2xl p-6 max-w-4xl w-full mx-4 max-h-[90vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">真机投屏</h2>
        <button onclick={() => { stopMirror(); showMirrorPanel = false; }}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <div class="flex gap-3 mb-4 flex-wrap items-end">
        <div class="flex flex-col gap-1">
          <label class="text-label-small text-on-surface-variant">设备</label>
          <select
            class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface min-w-[200px]"
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
          <label class="text-label-small text-on-surface-variant">帧率</label>
          <select
            class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface"
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
          <label class="text-label-small text-on-surface-variant">画面比例</label>
          <select
            class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface"
            bind:value={mirrorAspect}
          >
            <option value="contain">适应</option>
            <option value="cover">填充</option>
            <option value="stretch">拉伸</option>
          </select>
        </div>
        {#if !mirroring}
          <md-filled-button onclick={startMirror} disabled={!mirrorDevice}>
            <md-icon slot="start">play_arrow</md-icon>
            开始投屏
          </md-filled-button>
        {:else}
          <md-filled-button onclick={stopMirror}>
            <md-icon slot="start">stop</md-icon>
            停止投屏
          </md-filled-button>
          <md-filled-tonal-button onclick={captureMirrorFrame}>
            <md-icon slot="start">photo_camera</md-icon>
            截图
          </md-filled-tonal-button>
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
          <div class="text-center text-on-surface-variant py-12">
            <md-icon class="text-5xl mb-2">screen_share</md-icon>
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
    <div class="bg-surface rounded-2xl p-6 max-w-2xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">检查更新</h2>
        <button onclick={() => showUpdatePanel = false}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <div class="space-y-3 mb-4">
        <div class="flex flex-col gap-1">
          <label class="text-label-small text-on-surface-variant">当前版本</label>
          <input
            type="text"
            placeholder="v1.0"
            class="px-4 py-2 border border-outline rounded-lg bg-surface text-on-surface"
            bind:value={updateModuleVersion}
          />
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-label-small text-on-surface-variant">GitHub 仓库 URL</label>
          <input
            type="text"
            placeholder="https://github.com/user/repo"
            class="px-4 py-2 border border-outline rounded-lg bg-surface text-on-surface"
            bind:value={updateModuleRepo}
          />
        </div>
        <md-filled-button onclick={checkModuleUpdate} disabled={updateChecking || !updateModuleRepo}>
          <md-icon slot="start">system_update</md-icon>
          {updateChecking ? '检查中...' : '检查更新'}
        </md-filled-button>
      </div>

      {#if updateResult}
        <div class="border border-outline rounded-xl p-4">
          {#if updateResult.has_update}
            <div class="flex items-center gap-2 mb-3">
              <md-icon class="text-green-600">arrow_upward</md-icon>
              <span class="text-title-medium text-green-600">有新版本可用</span>
            </div>
            <div class="space-y-2 text-body-small">
              <div class="flex justify-between">
                <span class="text-on-surface-variant">当前版本</span>
                <span class="font-mono">{updateResult.current_version}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-on-surface-variant">最新版本</span>
                <span class="font-mono font-bold">{updateResult.latest_version}</span>
              </div>
              {#if updateResult.download_url}
                <div class="mt-3">
                  <a href={updateResult.download_url} target="_blank" rel="noopener">
                    <md-filled-tonal-button>
                      <md-icon slot="start">download</md-icon>
                      下载最新版本
                    </md-filled-tonal-button>
                  </a>
                </div>
              {/if}
              {#if updateResult.release_note}
                <div class="mt-3 p-3 bg-surface-container rounded-lg">
                  <p class="text-label-small text-on-surface-variant mb-1">Release Notes</p>
                  <p class="text-body-small whitespace-pre-wrap">{updateResult.release_note}</p>
                </div>
              {/if}
            </div>
          {:else}
            <div class="flex items-center gap-2">
              <md-icon class="text-green-600">check_circle</md-icon>
              <span class="text-title-medium text-green-600">已是最新版本</span>
            </div>
            {#if updateResult.error}
              <p class="text-body-small text-on-surface-variant mt-2">{updateResult.error}</p>
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
    <div class="bg-surface rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[85vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">性能基准测试</h2>
        <button onclick={() => showBenchmarkPanel = false}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <div class="flex gap-3 mb-4 items-end">
        <div class="flex flex-col gap-1">
          <label class="text-label-small text-on-surface-variant">选择设备</label>
          <select
            class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface min-w-[200px]"
            bind:value={benchmarkDevice}
          >
            <option value="">选择设备</option>
            {#each adbDevices as dev}
              <option value={dev.serial}>{dev.serial} ({dev.model || 'Unknown'})</option>
            {/each}
          </select>
        </div>
        <md-filled-button onclick={runBenchmark} disabled={benchmarkRunning || !benchmarkDevice}>
          <md-icon slot="start">speed</md-icon>
          {benchmarkRunning ? '测试中...' : '开始测试'}
        </md-filled-button>
        <md-filled-tonal-button onclick={loadBenchmarkHistory}>
          <md-icon slot="start">history</md-icon>
          历史记录
        </md-filled-tonal-button>
      </div>

      <div class="flex-1 overflow-auto space-y-4">
        {#if benchmarkResult}
          <div class="border border-outline rounded-xl p-4">
            <h3 class="text-title-small mb-3 flex items-center gap-2">
              <md-icon class="text-sm">speed</md-icon>
              测试结果
            </h3>
            <div class="grid grid-cols-2 gap-3">
              {#each Object.entries(benchmarkResult.before || {}) as [key, value]}
                <div class="p-3 bg-surface-container rounded-lg">
                  <p class="text-label-small text-on-surface-variant mb-1">{key}</p>
                  <p class="text-body-small font-mono">{String(value).substring(0, 120)}</p>
                </div>
              {/each}
            </div>
            {#if benchmarkResult.diff?.note}
              <div class="mt-3 p-3 bg-primary-container rounded-lg">
                <p class="text-body-small text-on-primary-container">{benchmarkResult.diff.note}</p>
              </div>
            {/if}
          </div>
        {/if}

        {#if benchmarkHistory.length > 0}
          <div class="border border-outline rounded-xl p-4">
            <h3 class="text-title-small mb-3 flex items-center gap-2">
              <md-icon class="text-sm">history</md-icon>
              历史记录
            </h3>
            <div class="space-y-2">
              {#each benchmarkHistory as bench}
                <div class="p-3 bg-surface-container rounded-lg">
                  <div class="flex items-center justify-between mb-1">
                    <span class="text-body-small font-mono">{bench.id}</span>
                    <span class="text-body-small text-on-surface-variant">{new Date(bench.created_at).toLocaleString('zh-CN')}</span>
                  </div>
                  <div class="flex gap-4 text-body-small text-on-surface-variant">
                    <span>设备: {bench.device_serial}</span>
                    <span>模块: {bench.module_id}</span>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        {#if !benchmarkResult && benchmarkHistory.length === 0}
          <div class="text-center text-on-surface-variant py-12">
            <md-icon class="text-5xl mb-2">speed</md-icon>
            <p>选择设备后点击开始测试</p>
            <p class="text-body-small mt-1">测试将采集 CPU、内存、存储等设备性能数据</p>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- Collaboration Panel -->
{#if showCollabPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-surface rounded-2xl p-6 max-w-3xl w-full mx-4 max-h-[85vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">团队协作</h2>
        <div class="flex items-center gap-3">
          <span class="text-body-small {collabWsConnected ? 'text-green-600' : 'text-red-600'}">
            {collabWsConnected ? '● 已连接' : '○ 未连接'}
          </span>
          <button onclick={() => showCollabPanel = false}>
            <md-icon>close</md-icon>
          </button>
        </div>
      </div>

      <!-- Username -->
      <div class="flex gap-2 mb-4">
        <input
          type="text"
          placeholder="你的用户名..."
          class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface flex-1"
          bind:value={collabUsername}
        />
      </div>

      <div class="flex-1 overflow-auto space-y-4">
        <!-- Collaborators -->
        <div class="border border-outline rounded-xl p-4">
          <h3 class="text-title-small mb-3 flex items-center gap-2">
            <md-icon class="text-sm">group</md-icon>
            协作者
          </h3>
          {#if collaborators.length > 0}
            <div class="space-y-2 mb-3">
              {#each collaborators as c}
                <div class="flex items-center justify-between p-2 bg-surface-container rounded-lg">
                  <div class="flex items-center gap-2">
                    <div class="w-8 h-8 rounded-full flex items-center justify-center text-white text-sm font-bold" style="background-color: {COLLAB_COLORS[collaborators.indexOf(c) % COLLAB_COLORS.length]}">
                      {(c.username || c.user_id)[0]?.toUpperCase() || '?'}
                    </div>
                    <div>
                      <p class="text-body-small font-medium">{c.username || c.user_id}</p>
                      <p class="text-xs text-on-surface-variant">{c.role}</p>
                    </div>
                  </div>
                  <md-icon-button onclick={() => removeCollaborator(c.user_id)}>
                    <md-icon class="text-sm">close</md-icon>
                  </md-icon-button>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-body-small text-on-surface-variant mb-3">暂无协作者</p>
          {/if}
          <div class="flex gap-2">
            <input
              type="text"
              placeholder="用户名"
              class="px-3 py-1 border border-outline rounded bg-surface text-on-surface text-sm flex-1"
              bind:value={collabInviteUser}
            />
            <select class="px-2 py-1 border border-outline rounded bg-surface text-on-surface text-sm" bind:value={collabInviteRole}>
              <option value="editor">编辑者</option>
              <option value="viewer">查看者</option>
              <option value="admin">管理员</option>
            </select>
            <md-filled-tonal-button size="small" onclick={inviteCollaborator}>邀请</md-filled-tonal-button>
          </div>
        </div>

        <!-- Active editors -->
        <div class="border border-outline rounded-xl p-4">
          <h3 class="text-title-small mb-3 flex items-center gap-2">
            <md-icon class="text-sm">edit</md-icon>
            活跃编辑者
          </h3>
          {#if collabSessions.length > 0}
            <div class="space-y-2">
              {#each collabSessions as s}
                <div class="flex items-center gap-3 p-2 bg-surface-container rounded-lg">
                  <div class="w-3 h-3 rounded-full" style="background-color: {s.color}"></div>
                  <div>
                    <p class="text-body-small font-medium">{s.username || s.user_id}</p>
                    <p class="text-xs text-on-surface-variant">编辑 {s.file_path} · 行 {s.cursor_line}, 列 {s.cursor_col}</p>
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-body-small text-on-surface-variant">无其他编辑者在线</p>
          {/if}
        </div>

        <!-- Comments -->
        <div class="border border-outline rounded-xl p-4">
          <h3 class="text-title-small mb-3 flex items-center gap-2">
            <md-icon class="text-sm">comment</md-icon>
            评论
          </h3>
          {#if collabComments.length > 0}
            <div class="space-y-2 mb-3 max-h-48 overflow-auto">
              {#each collabComments as c}
                <div class="p-2 rounded-lg {c.resolved ? 'bg-green-50 border border-green-300' : 'bg-surface-container border border-outline'}">
                  <div class="flex items-center justify-between mb-1">
                    <div class="flex items-center gap-2">
                      <span class="text-body-small font-medium">{c.username}</span>
                      <span class="text-xs text-on-surface-variant">{c.file_path}:{c.line_number}</span>
                    </div>
                    {#if !c.resolved}
                      <md-filled-tonal-button size="small" onclick={() => resolveComment(c.id)}>解决</md-filled-tonal-button>
                    {:else}
                      <span class="text-xs text-green-600">已解决</span>
                    {/if}
                  </div>
                  <p class="text-body-small">{c.content}</p>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-body-small text-on-surface-variant mb-3">暂无评论</p>
          {/if}
          <div class="space-y-2">
            <div class="flex gap-2">
              <input type="text" placeholder="文件路径" class="px-2 py-1 border border-outline rounded bg-surface text-on-surface text-sm flex-1" bind:value={commentFilePath} />
              <input type="number" placeholder="行号" class="px-2 py-1 border border-outline rounded bg-surface text-on-surface text-sm w-20" bind:value={commentLineNumber} />
            </div>
            <div class="flex gap-2">
              <input type="text" placeholder="评论内容..." class="px-3 py-1 border border-outline rounded bg-surface text-on-surface text-sm flex-1" bind:value={commentContent} />
              <md-filled-tonal-button size="small" onclick={addCollabComment}>发送</md-filled-tonal-button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- Plugin Panel -->
{#if showPluginPanel}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div class="bg-surface rounded-2xl p-6 max-w-2xl w-full mx-4 max-h-[85vh] flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-headline-small">插件系统</h2>
        <button onclick={() => showPluginPanel = false}>
          <md-icon>close</md-icon>
        </button>
      </div>

      <div class="flex-1 overflow-auto space-y-4">
        <!-- Install Plugin -->
        <div class="border border-outline rounded-xl p-4">
          <h3 class="text-title-small mb-3 flex items-center gap-2">
            <md-icon class="text-sm">add</md-icon>
            安装插件
          </h3>
          <div class="grid grid-cols-2 gap-2 mb-2">
            <input type="text" placeholder="插件名称" class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface" bind:value={pluginInstallName} />
            <input type="text" placeholder="slug (唯一标识)" class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface" bind:value={pluginInstallSlug} />
            <input type="text" placeholder="描述" class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface" bind:value={pluginInstallDesc} />
            <input type="text" placeholder="作者" class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface" bind:value={pluginInstallAuthor} />
          </div>
          <md-filled-button onclick={installPlugin} disabled={!pluginInstallName.trim() || !pluginInstallSlug.trim()}>
            <md-icon slot="start">download</md-icon>
            安装
          </md-filled-button>
        </div>

        <!-- Plugin List -->
        <div class="border border-outline rounded-xl p-4">
          <h3 class="text-title-small mb-3 flex items-center gap-2">
            <md-icon class="text-sm">extension</md-icon>
            已安装插件
            <md-filled-tonal-button size="small" onclick={loadPlugins} class="ml-auto">刷新</md-filled-tonal-button>
          </h3>
          {#if pluginList.length > 0}
            <div class="space-y-2">
              {#each pluginList as p}
                <div class="p-3 bg-surface-container rounded-lg">
                  <div class="flex items-center justify-between mb-2">
                    <div>
                      <div class="flex items-center gap-2">
                        <span class="text-body-small font-medium">{p.name}</span>
                        <span class="text-xs text-on-surface-variant">v{p.version}</span>
                        <span class="px-2 py-0.5 rounded-full text-xs {p.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'}">
                          {p.enabled ? '已启用' : '已禁用'}
                        </span>
                      </div>
                      <p class="text-xs text-on-surface-variant mt-1">{p.description} · {p.author}</p>
                    </div>
                    <div class="flex gap-1">
                      <md-filled-tonal-button size="small" onclick={() => { selectedPluginId = p.id; }}>
                        钩子
                      </md-filled-tonal-button>
                      <md-filled-tonal-button size="small" onclick={() => togglePlugin(p.id, !p.enabled)}>
                        {p.enabled ? '禁用' : '启用'}
                      </md-filled-tonal-button>
                      <md-icon-button onclick={() => uninstallPlugin(p.id)}>
                        <md-icon class="text-sm">delete</md-icon>
                      </md-icon-button>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-body-small text-on-surface-variant">暂无已安装插件</p>
          {/if}
        </div>

        <!-- Register Hook -->
        {#if selectedPluginId}
          <div class="border border-outline rounded-xl p-4">
            <h3 class="text-title-small mb-3 flex items-center gap-2">
              <md-icon class="text-sm">webhook</md-icon>
              注册钩子
            </h3>
            <div class="grid grid-cols-2 gap-2 mb-2">
              <input type="text" placeholder="钩子名称 (e.g. pre_save)" class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface" bind:value={pluginHookName} />
              <select class="px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface" bind:value={pluginHookType}>
                <option value="pre_save">pre_save</option>
                <option value="post_save">post_save</option>
                <option value="pre_build">pre_build</option>
                <option value="post_build">post_build</option>
                <option value="on_comment">on_comment</option>
              </select>
            </div>
            <input type="text" placeholder="入口 (e.g. my-plugin/handler.js)" class="w-full px-3 py-2 border border-outline rounded-lg bg-surface text-on-surface mb-2" bind:value={pluginHookEntry} />
            <md-filled-tonal-button onclick={registerHook} disabled={!pluginHookName.trim() || !pluginHookEntry.trim()}>
              <md-icon slot="start">add</md-icon>
              注册钩子
            </md-filled-tonal-button>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

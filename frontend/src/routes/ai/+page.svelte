<script lang="ts">
import { onMount, onDestroy } from 'svelte';
import { toast } from '$lib/stores/toast.svelte';
import { client, authFetch } from '$lib/api/client';

  type Mode = 'generate' | 'chat' | 'repair';

  interface Model {
    id: string; name: string; provider: string; max_tokens: number;
    supports_stream: boolean; price_input_per_m: number; price_output_per_m: number;
  }
  interface Provider {
    name: string; id: string; endpoint: string; models: Model[];
    requires_key: boolean; is_free: boolean; tier: string;
  }
  interface AIPrompt { id: string; mode: string; content: string; updated_at: string; }

  let providers = $state<Provider[]>([]);
  let selectedProviderID = $state('');
  let selectedModelID = $state('');
  let configLoaded = $state(false);
  let refreshing = $state(false);

  let availableModels = $derived((providers || []).find(x => x.id === selectedProviderID)?.models || []);
  let freeModels = $derived((availableModels || []).filter(m => m.price_input_per_m === 0 && m.price_output_per_m === 0));
  let paidModels = $derived((availableModels || []).filter(m => m.price_input_per_m > 0 || m.price_output_per_m > 0));
  let selectedModel = $derived(availableModels.find(m => m.id === selectedModelID) || null);

  let mode = $state<Mode>('generate');
  let input = $state('');
  let messages = $state<{role: string; content: string}[]>([]);
  let streaming = $state(false);
  let buildLog = $state('');
  let streamCtrl: any = null;
  let chatEnd: HTMLDivElement | undefined = $state();
  let reasoningContent = $state('');
  let showReasoning = $state(false);

  // Prompt settings
  let showPromptSettings = $state(false);
  let promptTab = $state<Mode>('generate');
  let prompts = $state<AIPrompt[]>([]);
  let promptDraft = $state('');
  let promptSaving = $state(false);
  let promptLoading = $state(false);

  // Progress indicator
  let progressSteps = $state<string[]>([]);
  let currentStepIndex = $state(-1);
  let progressStepDetails = $state<{step: string; message: string; time: number}[]>([]);
  const progressLabels: Record<string, string> = {
    start: '正在连接AI...',
    analyze: '正在分析需求...',
    structure: '正在生成模块结构...',
    script: '正在编写安装脚本...',
    system: '正在配置系统文件...',
    optimize: '正在优化代码...',
    done: '生成完成！',
  };

  // Generation history (localStorage)
  interface GenHistoryItem {
    id: string;
    title: string;
    timestamp: number;
    model: string;
    mode: string;
    messageCount: number;
    preview: string;
  }
  let genHistory = $state<GenHistoryItem[]>([]);
  let showGenHistory = $state(false);

  // WebUI preview
  let hasWebUIFiles = $state(false);
  let webUIPreviewHTML = $state('');
  let previewWebUIMode = $state(false);

  // Recommended files type for file structure display
  interface RecommendedFile {
    path: string;
    required: boolean;
    description: string;
  }

  // Preview modal
  interface PreviewFile {
    path: string;
    content: string;
  }
  let showPreviewModal = $state(false);
  let previewFiles = $state<PreviewFile[]>([]);
  let previewSelectedFile = $state<string | null>(null);

  // Security scan
  interface SecurityIssue {
    severity: string;
    file: string;
    line: number;
    rule: string;
    message: string;
  }
  interface SecurityScanResult {
    safe: boolean;
    issues: SecurityIssue[];
    score: number;
    summary: string;
  }

  let scanResult = $state<SecurityScanResult | null>(null);
  let scanning = $state(false);
  let showSecurityWarning = $state(false);
  let pendingImportFiles = $state<{path: string; content: string}[]>([]);

  // Import to project
  let showImportDialog = $state(false);
  let importFiles = $state<{path: string; content: string}[]>([]);
  let importProjects = $state<{id: string; name: string}[]>([]);
  let selectedImportProject = $state('');
  let importing = $state(false);

  function cleanRecommendedContent(content: string): string {
    return content.replace(/\{"recommended_files":\s*\[[\s\S]*?\]\}/, '').trim();
  }

  function extractRecommendedFiles(content: string): RecommendedFile[] | null {
    try {
      const jsonMatch = content.match(/\{"recommended_files":\s*\[[\s\S]*?\]\}/);
      if (jsonMatch) {
        const parsed = JSON.parse(jsonMatch[0]);
        if (parsed.recommended_files && Array.isArray(parsed.recommended_files)) {
          for (const f of parsed.recommended_files) {
            if (typeof f.path !== 'string') return null;
          }
          return parsed.recommended_files;
        }
      }
      const fullParse = JSON.parse(content);
      if (fullParse.recommended_files && Array.isArray(fullParse.recommended_files)) {
        for (const f of fullParse.recommended_files) {
          if (typeof f.path !== 'string') return null;
        }
        return fullParse.recommended_files;
      }
    } catch {}
    return null;
  }

  function openPreview(files: {path: string; content: string}[]) {
    previewFiles = files;
    previewSelectedFile = files.length > 0 ? files[0].path : null;
    showPreviewModal = true;
  }

  function getPreviewContent(): string {
    if (!previewSelectedFile) return '';
    const f = previewFiles.find(x => x.path === previewSelectedFile);
    return f?.content || '';
  }

  function getFileLanguage(path: string): string {
    if (path.endsWith('.sh') || path.endsWith('.bash')) return 'shell';
    if (path.endsWith('.py')) return 'python';
    if (path.endsWith('.json')) return 'json';
    if (path.endsWith('.html') || path.endsWith('.htm')) return 'html';
    if (path.endsWith('.css')) return 'css';
    if (path.endsWith('.xml') || path.endsWith('.conf')) return 'xml';
    if (path.endsWith('.js') || path.endsWith('.mjs')) return 'javascript';
    if (path.endsWith('.prop') || path.endsWith('.properties')) return 'shell';
    if (path.endsWith('.md')) return 'markdown';
    return 'shell';
  }

  function getFileIcon(path: string): string {
    if (path === 'module.prop') return 'badge';
    if (path.endsWith('.sh') || path.endsWith('.bash')) return 'code';
    if (path.endsWith('.json')) return 'data_object';
    if (path.endsWith('.html')) return 'web';
    if (path.endsWith('.css')) return 'palette';
    if (path.endsWith('.xml') || path.endsWith('.conf')) return 'settings';
    if (path.endsWith('.js')) return 'javascript';
    if (path.endsWith('.md')) return 'description';
    return 'insert_drive_file';
  }

  // Error detail for failed generations
  interface ErrorDetail {
    message: string;
    suggestion: string;
  }

  function parseErrorDetail(content: string): ErrorDetail | null {
    try {
      const parsed = JSON.parse(content);
      if (parsed.error && parsed.error_detail) {
        return { message: parsed.error, suggestion: parsed.suggestion || '' };
      }
      if (parsed.error) {
        return { message: parsed.error, suggestion: '' };
      }
    } catch {}
    if (content.includes('AI service unavailable') || content.includes('LLM not configured')) {
      return {
        message: 'AI 服务不可用或未配置 API 密钥',
        suggestion: '请在设置中配置 LLM API 密钥，或检查网络连接。'
      };
    }
    return null;
  }

  function extractFiles(content: string): { path: string; content: string }[] | null {
    function tryParse(text: string): { path: string; content: string }[] | null {
      try {
        const parsed = JSON.parse(text);
        if (parsed.files && Array.isArray(parsed.files)) {
          for (const f of parsed.files) {
            if (typeof f.path !== 'string' || typeof f.content !== 'string') return null;
          }
          return parsed.files;
        }
      } catch {}
      return null;
    }
    const result = tryParse(content);
    if (result) return result;
    const blockMatch = content.match(/```(?:json)?\s*\n?([\s\S]*?)```/);
    if (blockMatch) {
      const result = tryParse(blockMatch[1].trim());
      if (result) return result;
    }
    return null;
  }

  function parseMessageContent(content: string): Array<{type: 'text'; content: string} | {type: 'code'; language: string; content: string}> {
    const segments: Array<{type: 'text'; content: string} | {type: 'code'; language: string; content: string}> = [];
    const parts = content.split(/(```[\s\S]*?```)/g);
    for (const part of parts) {
      if (!part) continue;
      const codeMatch = part.match(/^```(\w*)\n?([\s\S]*?)```$/);
      if (codeMatch) {
        segments.push({ type: 'code', language: codeMatch[1] || 'shell', content: codeMatch[2] });
      } else if (part.trim()) {
        segments.push({ type: 'text', content: part });
      }
    }
    return segments;
  }

  function checkWebUIFiles(files: {path: string; content: string}[]): boolean {
    for (const f of files) {
      if (f.path.startsWith('webroot/') && (f.path.endsWith('.html') || f.path.endsWith('.htm'))) {
        return true;
      }
    }
    return false;
  }

  function getWebUIPreviewHTML(files: {path: string; content: string}[]): string {
    const htmlFile = files.find(f => f.path.startsWith('webroot/') && (f.path.endsWith('.html') || f.path.endsWith('.htm')));
    if (!htmlFile) return '';
    let html = htmlFile.content;
    const cssFiles = files.filter(f => f.path.startsWith('webroot/') && f.path.endsWith('.css'));
    const jsFiles = files.filter(f => f.path.startsWith('webroot/') && (f.path.endsWith('.js') || f.path.endsWith('.mjs')));
    const scriptClose = '<' + '/script>';
    for (const css of cssFiles) {
      const filename = css.path.split('/').pop() || '';
      if (!html.includes('<link') || !html.includes(filename)) {
        html = html.replace('</head>', '<style>' + css.content + '</style></head>');
      }
    }
    for (const js of jsFiles) {
      const filename = js.path.split('/').pop() || '';
      if (!html.includes('<script') || !html.includes(filename)) {
        html = html.replace('</body>', '<script>' + js.content + scriptClose + '</body>');
      }
    }
    return html;
  }

  function loadGenHistory() {
    try {
      const stored = localStorage.getItem('moduforge_ai_history');
      if (stored) genHistory = JSON.parse(stored);
    } catch { genHistory = []; }
  }

  function saveGenHistory() {
    try {
      localStorage.setItem('moduforge_ai_history', JSON.stringify(genHistory.slice(0, 50)));
    } catch {}
  }

  function addGenHistory(title: string, mode: string, msgs: {role: string; content: string}[]) {
    const item: GenHistoryItem = {
      id: Date.now().toString(36),
      title: title.slice(0, 60),
      timestamp: Date.now(),
      model: selectedModel?.name || selectedModelID || 'unknown',
      mode,
      messageCount: msgs.length,
      preview: msgs.filter(m => m.role === 'assistant').slice(-1)[0]?.content.slice(0, 100) || '',
    };
    genHistory = [item, ...genHistory].slice(0, 50);
    saveGenHistory();
  }

  function restoreGenHistory(item: GenHistoryItem) {
    messages = [];
    showGenHistory = false;
  }

  function updateProgressFromContent(content: string) {
    const lower = content.toLowerCase();
    if (lower.includes('module.prop') || lower.includes('analyze') || lower.includes('需求')) {
      currentStepIndex = Math.max(currentStepIndex, 0);
    }
    if (lower.includes('customize.sh') || lower.includes('install') || lower.includes('结构') || lower.includes('script')) {
      currentStepIndex = Math.max(currentStepIndex, 1);
    }
    if (lower.includes('#!/') || lower.includes('set -e') || lower.includes('编写') || lower.includes('代码')) {
      currentStepIndex = Math.max(currentStepIndex, 2);
    }
    if (lower.includes('optimize') || lower.includes('performance') || lower.includes('优化')) {
      currentStepIndex = Math.max(currentStepIndex, 3);
    }
  }

  async function loadImportProjects() {
    try {
      const projects = await client.get<{id: string; name: string}[]>('/projects');
      importProjects = projects;
      if (projects.length > 0) selectedImportProject = projects[0].id;
    } catch (e: any) {
      toast(e.message || '加载项目列表失败', 'error');
    }
  }

  function openImportDialog(index: number) {
    const msg = messages[index];
    if (!msg) return;
    const files = extractFiles(msg.content);
    if (!files) return;
    importFiles = files;
    scanResult = null;
    loadImportProjects();
    showImportDialog = true;
  }

  async function scanAndImport() {
    if (!selectedImportProject || importFiles.length === 0) return;
    scanning = true;
    try {
      const res = await authFetch('/api/v1/security/scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files: Object.fromEntries(importFiles.map(f => [f.path, f.content])) }),
      });
      scanResult = await res.json();
    } catch {
      scanResult = null;
    }
    scanning = false;

    if (scanResult && !scanResult.safe) {
      const criticalIssues = scanResult.issues.filter(i => i.severity === 'critical');
      if (criticalIssues.length > 0) {
        showSecurityWarning = true;
        pendingImportFiles = importFiles;
        return;
      }
    }

    proceedImport();
  }

  function proceedImport() {
    showSecurityWarning = false;
    doImport();
  }

  function continueImportAfterWarning() {
    showSecurityWarning = false;
    doImport();
  }

  async function doImport() {
    if (!selectedImportProject || importFiles.length === 0) return;
    importing = true;
    let success = 0;
    let fail = 0;
    for (const f of importFiles) {
      try {
        await client.put(`/projects/${selectedImportProject}/files/${encodeURIComponent(f.path)}`, { content: f.content });
        success++;
      } catch {
        fail++;
      }
    }
    importing = false;
    if (fail === 0) {
      toast(`成功导入 ${success} 个文件到项目`, 'success');
      showImportDialog = false;
    } else {
      toast(`导入完成：${success} 成功，${fail} 失败`, success > 0 ? 'warning' : 'error');
    }
  }

  const modes = [
    { value: 'generate' as const, label: '生成模块', icon: 'auto_fix_high', desc: '描述需求，AI 生成通用模块代码' },
    { value: 'chat' as const, label: 'AI 对话', icon: 'chat', desc: '与 AI 对话获取帮助' },
    { value: 'repair' as const, label: '修复构建', icon: 'build_circle', desc: '粘贴日志分析问题' },
  ];

  const defaultPrompts: Record<Mode, string> = {
    generate: '',
    chat: '',
    repair: '',
  };

  $effect(() => {
    if (chatEnd) chatEnd.scrollIntoView({ behavior: 'smooth' });
  });

  // promptDraft is set in openPromptSettings after loadPrompts completes

  function onTimeout() {
    streaming = false;
    currentStepIndex = -1;
    messages = [...messages, { role: 'assistant', content: '⏱️ **请求超时**（超过 60 秒）\n\n可能原因：\n1. LLM 服务响应太慢（试试换一个更快的模型）\n2. 网络连接不稳定\n3. API 端点配置错误\n\n建议：切换到免费模型（如 Qwen-turbo）重试，或在设置中检查 LLM 配置。' }];
    toast('AI 请求超时', 'error');
  }

  function onStreamError(e: Event) {
    const detail = (e as CustomEvent).detail || '未知错误';
    streaming = false;
    currentStepIndex = -1;
    messages = [...messages, { role: 'assistant', content: `❌ **AI 错误**\n\n${detail}\n\n请检查：\n1. LLM API Key 是否正确配置（设置 → LLM 配置）\n2. 网络连接是否正常\n3. API 额度是否充足` }];
    toast(detail, 'error');
  }

  onMount(async () => {
    window.addEventListener('ai-stream', onStreamData);
    window.addEventListener('ai-stream-done', onStreamDone);
    window.addEventListener('ai-stream-timeout', onTimeout);
    window.addEventListener('ai-stream-error', onStreamError);
    await loadProviders();
  });
  onDestroy(() => {
    window.removeEventListener('ai-stream', onStreamData);
    window.removeEventListener('ai-stream-done', onStreamDone);
    window.removeEventListener('ai-stream-timeout', onTimeout);
    window.removeEventListener('ai-stream-error', onStreamError);
  });

  async function loadProviders() {
    try {
      const res = await fetch('/api/v1/llm/providers');
      const data = await res.json();
      providers = data.providers || [];
      try {
        const cfgRes = await fetch('/api/v1/llm/config', { headers: { 'Authorization': `Bearer ${localStorage.getItem('moduforge_token') || ''}` } });
        if (cfgRes.ok) {
          const cfg = await cfgRes.json();
          // Validate saved config against current provider list
          if (cfg.provider && providers.some(p => p.id === cfg.provider)) {
            selectedProviderID = cfg.provider;
            if (cfg.model_id) {
              const provider = providers.find(p => p.id === cfg.provider);
              if (provider?.models.some(m => m.id === cfg.model_id)) {
                selectedModelID = cfg.model_id;
              } else if (provider && provider.models.length > 0) {
                selectedModelID = provider.models[0].id;
              }
            }
          }
        }
      } catch {}
      if (!selectedProviderID && providers.length > 0) { selectedProviderID = providers[0].id; if (providers[0].models.length > 0) selectedModelID = providers[0].models[0].id; }
      configLoaded = true;
    } catch { configLoaded = true; }
  }

  function onProviderChange() { if (availableModels.length > 0) selectedModelID = availableModels[0].id; }

  async function saveConfig() {
    const token = localStorage.getItem('moduforge_token');
    if (!token || !selectedProviderID || !selectedModelID) return;
    try {
      const res = await fetch('/api/v1/llm/config', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, body: JSON.stringify({ provider: selectedProviderID, model_id: selectedModelID }) });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
        console.error('saveConfig failed:', err);
        toast(err.error || '保存配置失败', 'error');
      }
    } catch (e) { console.error('saveConfig error:', e); }
  }

  async function refreshModels() {
    refreshing = true;
    try { await fetch('/api/v1/llm/refresh'); await loadProviders(); } catch {}
    refreshing = false;
  }

  // Prompt management
  async function loadPrompts(): Promise<AIPrompt[]> {
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      const res = await fetch('/api/v1/ai/prompts', { headers: { 'Authorization': `Bearer ${token}` } });
      if (res.ok) {
        const data = await res.json();
        prompts = data.prompts || [];
        return prompts;
      } else {
        console.warn('loadPrompts failed:', res.status, res.statusText);
      }
    } catch (e) {
      console.error('loadPrompts error:', e);
      toast('加载提示词失败，请刷新页面重试', 'error');
    }
    return [];
  }

  async function savePrompt() {
    promptSaving = true;
    promptLoading = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      await fetch('/api/v1/ai/prompts', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ mode: promptTab, content: promptDraft }),
      });
      await loadPrompts();
      toast('提示词保存成功', 'success');
    } catch { toast('保存提示词失败', 'error'); }
    promptLoading = false;
    promptSaving = false;
  }

  async function resetPrompt() {
    promptLoading = true;
    try {
      const token = localStorage.getItem('moduforge_token') || '';
      await fetch(`/api/v1/ai/prompts/${promptTab}/reset`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      const updated = await loadPrompts();
      const p = updated.find(x => x.mode === promptTab);
      promptDraft = p?.content || '';
      toast('已恢复默认提示词', 'success');
    } catch { toast('恢复默认提示词失败', 'error'); }
    promptLoading = false;
  }

  async function switchPromptTab(newMode: Mode) {
    promptTab = newMode;
    promptLoading = true;
    const updated = await loadPrompts();
    const p = updated.find(x => x.mode === newMode);
    promptDraft = p?.content || '';
    promptLoading = false;
  }

  async function openPromptSettings() {
    promptLoading = true;
    const updated = await loadPrompts();
    const p = updated.find(x => x.mode === promptTab);
    promptDraft = p?.content || '';
    showPromptSettings = true;
    promptLoading = false;
  }

  function onStreamData(e: Event) {
    const detail = (e as CustomEvent).detail as string;
    for (const line of detail.split('\n')) {
      if (!line.startsWith('data: ')) continue;
      const data = line.slice(6);
      if (data === '[DONE]') { streaming = false; return; }
      try {
        const parsed = JSON.parse(data);

        // 处理步骤事件
        if (parsed.type === 'step') {
          const stepIndex = ['start', 'structure', 'script', 'system', 'optimize', 'done'].indexOf(parsed.step);
          if (stepIndex >= 0) {
            currentStepIndex = stepIndex;
            // 添加步骤详情到步骤历史（去重）
            if (!progressStepDetails.find(d => d.step === parsed.step)) {
              progressStepDetails = [...progressStepDetails, { step: parsed.step, message: parsed.message, time: Date.now() }];
            }
          }
          return;
        }

        // 处理思考过程事件
        if (parsed.type === 'reasoning') {
          reasoningContent += parsed.content;
          showReasoning = true;
          return;
        }

        // 处理错误事件
        if (parsed.type === 'error' || parsed.error) {
          const errMsg = parsed.error || '未知错误';
          streaming = false;
          currentStepIndex = -1;
          messages = [...messages, { role: 'assistant', content: `❌ **AI 错误**\n\n${errMsg}\n\n请检查：\n1. LLM API Key 是否正确配置（设置 → LLM 配置）\n2. 网络连接是否正常\n3. API 额度是否充足` }];
          toast(errMsg, 'error');
          return;
        }

        // 处理普通内容（原有逻辑）
        let content = '';
        let reasoning = '';
        if (parsed.content) {
          content = parsed.content;
        } else if (parsed.choices && parsed.choices[0]) {
          const delta = parsed.choices[0].delta;
          if (delta) {
            if (delta.content) content = delta.content;
            if (delta.reasoning_content) reasoning = delta.reasoning_content;
          }
        }
        if (reasoning) {
          reasoningContent += reasoning;
          showReasoning = true;
        }
        if (content) {
          if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') {
            messages[messages.length - 1].content += content;
            messages = messages;
          } else { messages = [...messages, { role: 'assistant', content: content }]; }
          updateProgressFromContent(content);
        }
      } catch {
        if (messages.length > 0 && messages[messages.length - 1].role === 'assistant') {
          messages[messages.length - 1].content += data; messages = messages;
        } else { messages = [...messages, { role: 'assistant', content: data }]; }
        updateProgressFromContent(data);
      }
    }
  }

  function onStreamDone() {
    streaming = false;
    currentStepIndex = 4;
    reasoningContent = '';
    const lastMsg = messages.filter(m => m.role === 'assistant').slice(-1)[0];
    if (lastMsg) {
      addGenHistory(lastMsg.content.slice(0, 60), mode, messages);
    } else {
      // 流结束但没有任何 assistant 消息（LLM 返回空）
      messages = [...messages, { role: 'assistant', content: '⚠️ AI 未返回任何内容。请检查：\n1. API Key 是否有效\n2. 模型是否支持当前请求\n3. 服务是否正常运行' }];
    }
  }

  async function send() {
    const text = input.trim();
    if (!text || streaming) return;
    if (!selectedProviderID || !selectedModelID) {
      toast('请先选择 AI 模型', 'error');
      return;
    }
    input = '';
    messages = [...messages, { role: 'user', content: text }];
    streaming = true;
    // Setup progress steps
    progressSteps = Object.values(progressLabels);
    currentStepIndex = 0;
    progressStepDetails = [];
    await saveConfig();
    let body: unknown; let path: string;
    if (mode === 'generate') { path = '/ai/generate'; body = { description: text }; }
    else if (mode === 'repair') { path = '/ai/repair'; body = { build_log: buildLog || text }; }
    else { path = '/ai/chat'; body = { message: text }; }
    streamCtrl = (await import('../../lib/api/client')).streamRequest(path, body);
  }

  function stopStream() {
    streamCtrl?.close();
    streaming = false;
    currentStepIndex = -1;
    progressSteps = [];
    progressStepDetails = [];
    reasoningContent = '';
    showReasoning = false;
  }
</script>

<div class="flex flex-col h-full ai-page">
  <!-- Top Bar: Model Selection -->
  {#if configLoaded && providers.length > 0}
    <div class="px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] top-bar-select">
      <div class="flex items-center gap-2 mb-2">
        <select class="flex-1 px-3 py-2 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] cursor-pointer" bind:value={selectedProviderID} onchange={onProviderChange}>
          {#each providers as p}
            <option value={p.id}>{p.name} {p.is_free ? '🆓' : p.tier === 'subscription' ? '💳' : ''}</option>
          {/each}
        </select>
        <button class="flex items-center gap-1 px-2.5 py-2 rounded-lg text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={refreshModels} disabled={refreshing}>
          <span class="material-symbols-outlined text-[14px] {refreshing ? 'animate-spin' : ''}">refresh</span>
        </button>
      </div>
      <div class="flex items-center gap-2">
        <select class="flex-1 px-3 py-2 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] cursor-pointer" bind:value={selectedModelID}>
          {#if freeModels.length > 0}<optgroup label="🆓 免费">{#each freeModels as m}<option value={m.id}>{m.name}</option>{/each}</optgroup>{/if}
          {#if paidModels.length > 0}<optgroup label="💰 付费">{#each paidModels as m}<option value={m.id}>{m.name}</option>{/each}</optgroup>{/if}
        </select>
        {#if selectedModel}
          <span class="text-xs text-[var(--color-text-muted)] whitespace-nowrap">
            {selectedModel.price_input_per_m === 0 ? '✅ 免费' : `$${selectedModel.price_input_per_m}/M`}
          </span>
        {/if}
      </div>
    </div>
  {/if}

<style>
  @media (max-width: 768px) {
    /* 消息气泡全宽 */
    .ai-page :global(.msg-bubble) {
      max-width: 100% !important;
    }
    /* 输入区域 */
    .ai-page :global(.input-row) {
      flex-direction: column;
      gap: 8px;
    }
    .ai-page :global(.input-row textarea) {
      width: 100%;
      min-height: 60px;
    }
    .ai-page :global(.input-row button) {
      width: 100%;
      justify-content: center;
      min-height: 44px;
    }
    /* 顶部选择框 */
    .ai-page :global(.top-bar-select) {
      width: 100%;
      min-height: 40px;
    }
    .ai-page :global(.top-bar-select select) {
      width: 100%;
      min-height: 40px;
    }
    /* 模式标签页 */
    .ai-page :global(.mode-tabs) {
      flex-wrap: nowrap;
      gap: 4px;
      padding: 8px 8px;
      overflow-x: auto;
    }
    .ai-page :global(.mode-tabs button) {
      flex: 1 0 auto;
      min-width: 0;
      min-height: 40px;
      white-space: nowrap;
      font-size: 11px;
      padding: 4px 8px;
    }
    /* 输入区域底部留空 */
    .ai-page :global(.ai-input-area) {
      padding: 12px;
      padding-bottom: 80px;
    }
    /* 提示词弹窗全屏 */
    .ai-page :global(.prompt-modal-overlay) {
      align-items: stretch !important;
      padding: 0 !important;
    }
    .ai-page :global(.prompt-modal-overlay > div) {
      max-width: 100% !important;
      max-height: 100% !important;
      border-radius: 0 !important;
      width: 100%;
      height: 100%;
    }
    .ai-page :global(.prompt-modal-overlay textarea) {
      height: 60vh !important;
    }

    /* 消息列表 */
    .ai-page :global(.messages-area) {
      padding: 12px;
      padding-bottom: 80px;
    }
  }

  .ai-page {
    position: relative;
  }
</style>

  <!-- Mode Tabs + Prompt Settings -->
  <div class="px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-surface)] mode-tabs">
    <div class="flex items-center gap-1.5 mb-2">
      {#each modes as m}
        <button
          class="flex-1 flex items-center justify-center gap-1 px-2 py-2 rounded-xl text-xs font-medium transition-all duration-150 min-h-[36px]
            {mode === m.value ? 'bg-primary-600 text-white shadow-sm' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
          onclick={() => mode = m.value}
        >
          <span class="material-symbols-outlined text-[14px]">{m.icon}</span>
          {m.label}
        </button>
      {/each}
    </div>
    <div class="flex justify-end gap-1">
      <button
        class="flex items-center gap-1 px-2 py-1 rounded-lg text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all"
        onclick={() => { loadGenHistory(); showGenHistory = !showGenHistory; }}
        title="生成历史"
      >
        <span class="material-symbols-outlined text-[14px]">history</span>
        历史
      </button>
      <button
        class="flex items-center gap-1 px-2 py-1 rounded-lg text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-all"
        onclick={openPromptSettings}
        title="提示词设置"
      >
        <span class="material-symbols-outlined text-[14px]">tune</span>
        提示词
      </button>
    </div>
  </div>

  <!-- Progress Indicator -->
  {#if streaming && currentStepIndex >= 0}
    <div class="px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
      <div class="flex items-center gap-2 mb-2">
        <span class="material-symbols-outlined text-[16px] text-primary-500 animate-spin">progress_activity</span>
        <span class="text-sm font-medium text-[var(--color-text)]">
          {progressStepDetails.length > 0 ? progressStepDetails[progressStepDetails.length - 1].message : '正在准备...'}
        </span>
      </div>
      <div class="space-y-1.5">
        {#each ['start', 'structure', 'script', 'system', 'optimize', 'done'] as step, si}
          {@const detail = progressStepDetails.find(d => d.step === step)}
          {@const isDone = detail !== undefined}
          {@const isCurrent = si === currentStepIndex && !isDone}
          <div class="flex items-center gap-2 px-2 py-1 rounded-lg {isDone ? 'bg-primary-500/10' : isCurrent ? 'bg-primary-500/5' : ''}">
            {#if isDone}
              <span class="material-symbols-outlined text-[14px] text-primary-500">check_circle</span>
            {:else if isCurrent}
              <span class="material-symbols-outlined text-[14px] text-primary-500 animate-pulse">radio_button_checked</span>
            {:else}
              <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]">radio_button_unchecked</span>
            {/if}
            <span class="text-xs {isDone ? 'text-primary-600' : isCurrent ? 'text-[var(--color-text)]' : 'text-[var(--color-text-muted)]'}">
              {progressLabels[step] || step}
            </span>
            {#if isDone && detail}
              <span class="text-[10px] text-[var(--color-text-muted)] ml-auto">
                {new Date(detail.time).toLocaleTimeString('zh-CN', {hour: '2-digit', minute: '2-digit', second: '2-digit'})}
              </span>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Messages -->
  <div class="flex-1 overflow-y-auto p-4 space-y-4 messages-area">
    {#if messages.length === 0}
      <div class="flex items-center justify-center h-full">
        <div class="text-center">
          <div class="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4" style="background: var(--gradient-brand-subtle)">
            <span class="material-symbols-outlined text-3xl" style="color: var(--color-primary)">psychology</span>
          </div>
          <p class="text-lg font-semibold text-[var(--color-text)]">{modes.find(m => m.value === mode)?.desc}</p>
          <p class="text-sm text-[var(--color-text-muted)] mt-1">
            {#if mode === 'generate'}生成兼容 Magisk / KernelSU / APatch 的通用模块{:else if mode === 'repair'}粘贴构建日志，AI 分析问题并给出修复建议{:else}随时提问关于模块开发的问题{/if}
          </p>
        </div>
      </div>
    {:else}
      {#each messages as msg, i}
        <div class="flex {msg.role === 'user' ? 'justify-end' : 'justify-start'}">
          <div class="max-w-2xl px-4 py-3 rounded-2xl text-sm leading-relaxed whitespace-pre-wrap msg-bubble
            {msg.role === 'user'
              ? 'bg-primary-600 text-white rounded-br-md'
              : 'bg-[var(--color-surface)] text-[var(--color-text)] border border-[var(--color-border)] rounded-bl-md'}">
            {#if msg.role === 'assistant'}
              <div class="flex items-center gap-1.5 mb-1.5">
                <span class="material-symbols-outlined text-primary-500 text-[14px]">auto_awesome</span>
                <span class="text-xs font-medium text-primary-600">AI</span>
              </div>
              {#if showReasoning && reasoningContent}
                <div class="mb-2 rounded-xl overflow-hidden border border-[var(--color-border)]">
                  <button
                    class="flex items-center gap-1.5 w-full px-3 py-1.5 text-xs font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors"
                    onclick={() => showReasoning = !showReasoning}
                  >
                    <span class="material-symbols-outlined text-[14px]">psychology</span>
                    思考过程
                    <span class="material-symbols-outlined text-[14px] ml-auto transition-transform" class:rotate-180={showReasoning}>expand_more</span>
                  </button>
                  {#if showReasoning}
                    <div class="px-3 py-2 text-xs leading-relaxed whitespace-pre-wrap" style="color: var(--color-text-muted); background: var(--color-surface); border-top: 1px solid var(--color-border);">
                      {reasoningContent}
                    </div>
                  {/if}
                </div>
              {/if}
              {@const _msgSegments = parseMessageContent(cleanRecommendedContent(msg.content))}
              {@const _recFiles = extractRecommendedFiles(msg.content)}
              {#if _recFiles}
                <div class="mt-2 mb-2">
                  <p class="text-xs font-medium text-[var(--color-text-secondary)] mb-1.5">推荐文件清单：</p>
                  <div class="space-y-1">
                    {#each _recFiles as rf}
                      <div class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-xs" style="background: var(--color-surface);">
                        <span class="material-symbols-outlined text-[14px] {rf.required ? 'text-primary-500' : 'text-[var(--color-text-muted)]'}">
                          {rf.required ? 'required' : 'check_circle'}
                        </span>
                        <code class="font-mono font-medium text-[var(--color-text)]">{rf.path}</code>
                        {#if rf.description}
                          <span class="text-[var(--color-text-muted)] ml-1">— {rf.description}</span>
                        {/if}
                        {#if rf.required}
                          <span class="ml-auto px-1.5 py-0.5 rounded text-[10px] font-medium" style="background: var(--color-primary-light); color: var(--color-primary)">必需</span>
                        {:else}
                          <span class="ml-auto px-1.5 py-0.5 rounded text-[10px] font-medium" style="background: var(--color-surface); color: var(--color-text-muted)">可选</span>
                        {/if}
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}
              {@const _errDetail = parseErrorDetail(msg.content)}
              {#if _errDetail}
                <div class="p-3 rounded-lg text-xs" style="background: color-mix(in srgb, var(--color-error, #ef4444) 8%, var(--color-bg)); border: 1px solid color-mix(in srgb, var(--color-error, #ef4444) 20%, transparent);">
                  <div class="flex items-center gap-1.5 mb-1">
                    <span class="material-symbols-outlined text-[14px]" style="color: var(--color-error, #ef4444)">error_outline</span>
                    <span class="font-medium" style="color: var(--color-error, #ef4444)">生成失败</span>
                  </div>
                  <p class="text-[var(--color-text)]">{_errDetail.message}</p>
                  {#if _errDetail.suggestion}
                    <p class="mt-1 text-[var(--color-text-secondary)]">{_errDetail.suggestion}</p>
                  {/if}
                </div>
              {:else}
                <div class="space-y-2">
                  {#each _msgSegments as seg, si}
                    {#if seg.type === 'code'}
                      <div class="rounded-xl overflow-hidden border border-[var(--color-border)]" style="background: #1e1e2e;">
                        {#if seg.language}
                          <div class="flex items-center gap-1.5 px-3 py-1.5 text-[10px] font-medium" style="background: #181825; color: #a6adc8; border-bottom: 1px solid #313244;">
                            <span class="material-symbols-outlined text-[12px]">code</span>
                            {seg.language}
                          </div>
                        {/if}
                        <pre class="p-3 text-xs font-mono leading-relaxed overflow-x-auto" style="color: #cdd6f4; tab-size: 2;"><code>{seg.content}</code></pre>
                      </div>
                    {:else}
                      <div class="prose prose-sm max-w-none text-[var(--color-text)]">
                        {#each seg.content.split(/\n(?=\d+\.\s|步骤\s*\d+|Step\s*\d+)/) as stepText, stepIdx}
                          {#if stepText.trim()}
                            <div class="flex items-start gap-2 py-0.5">
                              {#if /^\d+\.\s/.test(stepText.trim()) || /^步骤\s*\d+/i.test(stepText.trim()) || /^Step\s*\d+/i.test(stepText.trim())}
                                <span class="flex-shrink-0 w-5 h-5 rounded-full bg-primary-600/20 text-primary-600 flex items-center justify-center text-[10px] font-bold mt-0.5">{stepIdx + 1}</span>
                              {/if}
                              <span class="leading-relaxed whitespace-pre-wrap">{stepText}</span>
                            </div>
                          {/if}
                        {/each}
                      </div>
                    {/if}
                  {/each}
                </div>
                {@const _files = extractFiles(msg.content)}
                {#if _files}
                  {@const _hasWebUI = checkWebUIFiles(_files)}
                  <div class="mt-2 flex flex-wrap gap-2">
                    <button
                      class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
                      style="background: var(--color-surface); color: var(--color-text); border: 1px solid var(--color-border);"
                      onclick={() => openPreview(_files)}
                    >
                      <span class="material-symbols-outlined text-[14px]">folder_open</span>
                      一键预览
                    </button>
                    {#if _hasWebUI}
                      <button
                        class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
                        style="background: var(--color-surface); color: var(--color-primary); border: 1px solid var(--color-primary);"
                        onclick={() => { hasWebUIFiles = true; webUIPreviewHTML = getWebUIPreviewHTML(_files); previewWebUIMode = true; openPreview(_files); }}
                      >
                        <span class="material-symbols-outlined text-[14px]">web</span>
                        预览 WebUI
                      </button>
                    {/if}
                    <button
                      class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
                      style="background: var(--color-primary-light); color: var(--color-primary)"
                      onclick={() => openImportDialog(i)}
                    >
                      <span class="material-symbols-outlined text-[14px]">download</span>
                      导入到项目
                    </button>
                  </div>
                {/if}
              {/if}
            {:else}
              {msg.content}
            {/if}
        </div>
      </div>
      {/each}
    {/if}
    <div bind:this={chatEnd}></div>
  </div>

  <!-- Input -->
  <div class="border-t border-[var(--color-border)] p-3 bg-[var(--color-bg-elevated)] ai-input-area">
    {#if mode === 'generate'}
      <div class="flex items-center gap-2 mb-2">
        <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium" style="background: var(--color-primary-light); color: var(--color-primary)">
          <span class="material-symbols-outlined text-[12px]">hub</span>
          Universal · Magisk + KSU + APatch
        </span>
      </div>
    {/if}
    {#if mode === 'repair'}
      <textarea class="input-field text-xs font-mono resize-none mb-2" rows="2" placeholder="粘贴构建日志（可选）" bind:value={buildLog}></textarea>
    {/if}
    <div class="flex items-end gap-2 input-row">
      <textarea
        class="flex-1 input-field resize-none"
        rows="3"
        style="min-height: 80px;"
        placeholder={mode === 'generate' ? '描述你的通用模块功能...' : mode === 'repair' ? '描述问题...' : '输入消息...'}
        bind:value={input}
        onkeydown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }}
      ></textarea>
      <div class="flex gap-2">
        {#if streaming}
          <button class="p-3 rounded-xl transition-colors" onclick={stopStream} style="background: var(--color-error-light); color: var(--color-error)">
            <span class="material-symbols-outlined text-[20px]">stop_circle</span>
          </button>
        {:else}
          <button class="p-3 rounded-xl bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={send} disabled={!input.trim()}>
            <span class="material-symbols-outlined text-[20px]">send</span>
          </button>
        {/if}
      </div>
    </div>
  </div>
</div>

<!-- Generation History Panel -->
{#if showGenHistory}
  <div class="fixed right-0 top-0 h-full z-40 w-80 shadow-2xl border-l border-[var(--color-border)] bg-[var(--color-bg)] flex flex-col" style="margin-top: 0;">
    <div class="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
      <div class="flex items-center gap-2">
        <span class="material-symbols-outlined text-[16px] text-primary-500">history</span>
        <span class="text-sm font-semibold text-[var(--color-text)]">生成历史</span>
      </div>
      <button class="p-1 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={() => showGenHistory = false}>
        <span class="material-symbols-outlined text-[18px]">close</span>
      </button>
    </div>
    <div class="flex-1 overflow-y-auto p-3">
      {#if genHistory.length === 0}
        <div class="flex flex-col items-center justify-center h-full text-sm text-[var(--color-text-muted)] gap-2">
          <span class="material-symbols-outlined text-[32px]">history</span>
          <span>暂无生成记录</span>
        </div>
      {:else}
        <div class="space-y-2">
          {#each genHistory as item}
            <button
              class="w-full text-left px-3 py-2.5 rounded-xl transition-colors hover:bg-[var(--color-surface)] border border-[var(--color-border)]"
              onclick={() => restoreGenHistory(item)}
            >
              <div class="flex items-center justify-between mb-1">
                <span class="text-xs font-medium text-[var(--color-text)] truncate max-w-[180px]">{item.title || '未命名'}</span>
                <span class="text-[10px] text-[var(--color-text-muted)]">{new Date(item.timestamp).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
              </div>
              <div class="flex items-center gap-2 text-[10px] text-[var(--color-text-muted)]">
                <span class="px-1.5 py-0.5 rounded" style="background: var(--color-primary-light); color: var(--color-primary)">{item.mode}</span>
                <span>{item.model}</span>
                <span>{item.messageCount} 条消息</span>
              </div>
              {#if item.preview}
                <p class="text-xs text-[var(--color-text-secondary)] mt-1 line-clamp-2">{item.preview}</p>
              {/if}
            </button>
          {/each}
        </div>
      {/if}
    </div>
    <div class="px-3 py-2 border-t border-[var(--color-border)]">
      <button
        class="w-full px-3 py-1.5 rounded-lg text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors"
        onclick={() => { genHistory = []; localStorage.removeItem('moduforge_ai_history'); }}
      >
        清空历史
      </button>
    </div>
  </div>
{/if}

<!-- Import to Project Modal -->
{#if showImportDialog}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" onclick={() => showImportDialog = false}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-md border border-[var(--color-border)] max-h-[85vh] flex flex-col" onclick={(e) => e.stopPropagation()}>
      <div class="px-6 py-4 border-b border-[var(--color-border)]">
        <h3 class="text-lg font-semibold text-[var(--color-text)]">导入到项目</h3>
      </div>
      <div class="px-6 py-4 space-y-4 overflow-y-auto flex-1">
        <p class="text-sm text-[var(--color-text-secondary)]">选择目标项目，导入 {importFiles.length} 个文件</p>
        <div>
          <label class="block text-sm font-medium mb-1.5 text-[var(--color-text-secondary)]">目标项目</label>
          <select class="w-full px-3 py-2 rounded-xl text-sm border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)]" bind:value={selectedImportProject}>
            {#each importProjects as p}
              <option value={p.id}>{p.name}</option>
            {/each}
          </select>
        </div>

        {#if scanning}
          <div class="flex items-center gap-2 py-3">
            <span class="material-symbols-outlined text-[18px] animate-spin text-primary-500">progress_activity</span>
            <span class="text-sm text-[var(--color-text-secondary)]">安全扫描中...</span>
          </div>
        {:else if scanResult}
          <div class="rounded-xl p-3 border" style="border-color: {scanResult.safe ? 'var(--color-success, #22c55e)' : 'var(--color-error, #ef4444)'}; background: color-mix(in srgb, {scanResult.safe ? '#22c55e' : '#ef4444'} 8%, var(--color-bg))">
            <div class="flex items-center gap-2 mb-1">
              <span class="material-symbols-outlined text-[18px]" style="color: {scanResult.safe ? '#22c55e' : '#ef4444'}">{scanResult.safe ? 'verified' : 'warning'}</span>
              <span class="text-sm font-medium" style="color: {scanResult.safe ? '#22c55e' : '#ef4444'}">安全评分：{scanResult.score}/100</span>
            </div>
            <p class="text-xs text-[var(--color-text-secondary)]">{scanResult.summary}</p>
            {#if scanResult.issues.length > 0}
              <div class="mt-2 space-y-1 max-h-32 overflow-y-auto">
                {#each scanResult.issues as issue}
                  <div class="flex items-start gap-1.5 text-xs px-2 py-1 rounded" style="background: color-mix(in srgb, var(--color-surface) 50%, transparent)">
                    <span class="material-symbols-outlined text-[12px] mt-0.5 flex-shrink-0" style="color: {issue.severity === 'critical' ? '#ef4444' : issue.severity === 'warning' ? '#f59e0b' : '#6b7280'}">
                      {issue.severity === 'critical' ? 'error' : issue.severity === 'warning' ? 'warning' : 'info'}
                    </span>
                    <span style="color: var(--color-text-secondary)"><strong>{issue.rule}</strong>: {issue.message}</span>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {/if}

        <div class="text-xs text-[var(--color-text-muted)]">
          <span class="flex items-center gap-1">
            <span class="material-symbols-outlined text-[12px]">security</span>
            导入前将自动进行安全扫描
          </span>
        </div>
      </div>
      <div class="flex justify-end gap-2 px-6 py-4 border-t border-[var(--color-border)]">
        <button class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={() => showImportDialog = false}>取消</button>
        <button class="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50" onclick={scanAndImport} disabled={importing || scanning || !selectedImportProject}>
          {#if importing}
            <span class="material-symbols-outlined text-[14px] animate-spin">progress_activity</span>
          {:else if scanning}
            <span class="material-symbols-outlined text-[14px] animate-spin">progress_activity</span>
          {:else}
            <span class="material-symbols-outlined text-[14px]">security</span>
          {/if}
          {importing ? '导入中...' : scanning ? '扫描中...' : '安全导入'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Security Warning Modal -->
{#if showSecurityWarning}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onclick={() => showSecurityWarning = false}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-md border border-[var(--color-border)]" onclick={(e) => e.stopPropagation()}>
      <div class="px-6 py-4 border-b border-[var(--color-border)] flex items-center gap-2">
        <span class="material-symbols-outlined text-error-500">warning</span>
        <h3 class="text-lg font-semibold text-[var(--color-text)]">安全警告</h3>
      </div>
      <div class="px-6 py-4 space-y-3">
        <p class="text-sm text-[var(--color-text-secondary)]">发现严重安全问题，导入可能存在风险：</p>
        {#if scanResult}
          {#each (scanResult?.issues || []).filter(i => i.severity === 'critical') as issue}
            <div class="flex items-start gap-2 p-2 rounded-lg text-xs" style="background: color-mix(in srgb, #ef4444 10%, var(--color-bg))">
              <span class="material-symbols-outlined text-[14px] text-error-500 flex-shrink-0">error</span>
              <div>
                <p class="font-medium" style="color: var(--color-error)">{issue.rule}</p>
                <p class="text-[var(--color-text-secondary)]">{issue.file}:{issue.line} — {issue.message}</p>
              </div>
            </div>
          {/each}
        {/if}
      </div>
      <div class="flex justify-end gap-2 px-6 py-4 border-t border-[var(--color-border)]">
        <button class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors" onclick={() => { showSecurityWarning = false; showImportDialog = true; }}>取消导入</button>
        <button class="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium text-white transition-colors" style="background: var(--color-error, #ef4444)" onclick={continueImportAfterWarning}>
          <span class="material-symbols-outlined text-[14px]">download</span>
          忽略风险并导入
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Preview Modal -->
{#if showPreviewModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" onclick={() => { showPreviewModal = false; previewWebUIMode = false; }}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-4xl max-h-[85vh] flex flex-col border border-[var(--color-border)]" onclick={(e) => e.stopPropagation()}>
      <!-- Header -->
      <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)]">
        <div class="flex items-center gap-2">
          {#if hasWebUIFiles && previewWebUIMode}
            <span class="material-symbols-outlined text-primary-600">web</span>
            <h2 class="text-lg font-semibold text-[var(--color-text)]">WebUI 预览</h2>
          {:else}
            <span class="material-symbols-outlined text-primary-600">folder_open</span>
            <h2 class="text-lg font-semibold text-[var(--color-text)]">模块文件预览</h2>
          {/if}
        </div>
        <div class="flex items-center gap-2">
          {#if hasWebUIFiles}
            <button
              class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors {previewWebUIMode ? 'bg-primary-600 text-white' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
              onclick={() => previewWebUIMode = !previewWebUIMode}
            >
              <span class="material-symbols-outlined text-[14px]">{previewWebUIMode ? 'folder_open' : 'web'}</span>
              {previewWebUIMode ? '文件视图' : 'WebUI 预览'}
            </button>
          {/if}
          <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={() => { showPreviewModal = false; previewWebUIMode = false; }}>
            <span class="material-symbols-outlined text-[20px]">close</span>
          </button>
        </div>
      </div>
      <!-- Content -->
      {#if hasWebUIFiles && previewWebUIMode && webUIPreviewHTML}
        <div class="flex flex-1 overflow-hidden">
          <div class="flex-1 flex flex-col">
            <iframe
              sandbox="allow-scripts allow-same-origin"
              srcdoc={webUIPreviewHTML}
              class="w-full h-full border-0"
              title="WebUI Preview"
            ></iframe>
          </div>
          <div class="w-64 border-l border-[var(--color-border)] overflow-y-auto p-3 flex-shrink-0">
            <p class="text-xs font-medium text-[var(--color-text-secondary)] mb-2">WebUI 文件</p>
            <div class="space-y-0.5">
              {#each (previewFiles || []).filter(f => f.path.startsWith('webroot/')) as pf}
                <button
                  class="flex items-center gap-2 w-full px-2.5 py-1.5 rounded-lg text-xs text-left transition-colors
                    {previewSelectedFile === pf.path ? 'bg-primary-600 text-white' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
                  onclick={() => { previewSelectedFile = pf.path; previewWebUIMode = false; }}
                >
                  <span class="material-symbols-outlined text-[14px]">{getFileIcon(pf.path)}</span>
                  <span class="font-mono truncate">{pf.path.split('/').pop()}</span>
                </button>
              {/each}
            </div>
          </div>
        </div>
      {:else}
        <div class="flex flex-1 overflow-hidden">
          <!-- File Tree -->
          <div class="w-64 border-r border-[var(--color-border)] overflow-y-auto p-3 flex-shrink-0">
            <div class="space-y-0.5">
              {#each previewFiles as pf}
                <button
                  class="flex items-center gap-2 w-full px-2.5 py-1.5 rounded-lg text-xs text-left transition-colors
                    {previewSelectedFile === pf.path ? 'bg-primary-600 text-white' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
                  onclick={() => previewSelectedFile = pf.path}
                >
                  <span class="material-symbols-outlined text-[14px]">{getFileIcon(pf.path)}</span>
                  <span class="font-mono truncate">{pf.path.split('/').pop()}</span>
                </button>
              {/each}
            </div>
          </div>
          <!-- File Content -->
          <div class="flex-1 flex flex-col overflow-hidden">
            {#if previewSelectedFile}
              <div class="px-4 py-2 border-b border-[var(--color-border)] bg-[var(--color-surface)] flex items-center gap-2">
                <span class="material-symbols-outlined text-[14px] text-[var(--color-text-muted)]">{getFileIcon(previewSelectedFile)}</span>
                <code class="text-xs font-mono text-[var(--color-text-secondary)]">{previewSelectedFile}</code>
                <span class="ml-auto text-[10px] px-1.5 py-0.5 rounded font-mono" style="background: var(--color-primary-light); color: var(--color-primary)">
                  {getFileLanguage(previewSelectedFile)}
                </span>
              </div>
              <div class="flex-1 overflow-auto p-4">
                <pre class="text-xs font-mono leading-relaxed whitespace-pre-wrap" style="color: var(--color-text); tab-size: 2;"><code>{getPreviewContent()}</code></pre>
              </div>
            {:else}
              <div class="flex items-center justify-center h-full text-sm text-[var(--color-text-muted)]">
                选择一个文件查看内容
              </div>
            {/if}
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Prompt Settings Modal -->
{#if showPromptSettings}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm prompt-modal-overlay" onclick={() => showPromptSettings = false}>
    <div class="bg-[var(--color-bg)] rounded-2xl shadow-2xl w-full max-w-3xl max-h-[80vh] flex flex-col border border-[var(--color-border)]" onclick={(e) => e.stopPropagation()}>
      <!-- Header -->
      <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)]">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-primary-600">tune</span>
          <h2 class="text-lg font-semibold text-[var(--color-text)]">AI 提示词设置</h2>
        </div>
        <button class="p-1.5 rounded-lg hover:bg-[var(--color-surface)] transition-colors" onclick={() => showPromptSettings = false}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <!-- Tabs -->
      <div class="flex items-center gap-1 px-6 pt-4">
        {#each modes as m}
          <button
            class="flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium transition-all
              {promptTab === m.value ? 'text-[var(--color-primary)]' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)]'}"
            style={promptTab === m.value ? 'background: var(--color-primary-light)' : ''}
            onclick={() => switchPromptTab(m.value)}
          >
            <span class="material-symbols-outlined text-[14px]">{m.icon}</span>
            {m.label}
          </button>
        {/each}
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto px-6 py-4">
        <p class="text-xs text-[var(--color-text-muted)] mb-3">
          自定义此模式下 AI 的系统提示词。留空则使用内置默认提示词。
        </p>
        {#key promptTab}
        {#if promptLoading}
          <div class="flex items-center justify-center py-8">
            <span class="material-symbols-outlined animate-spin text-primary-500">progress_activity</span>
          </div>
        {:else}
          <textarea
            class="w-full h-64 px-4 py-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-text)] font-mono text-xs leading-relaxed resize-y focus:outline-none focus:ring-2 focus:ring-primary-500/30 focus:border-primary-500"
            bind:value={promptDraft}
            placeholder="输入自定义提示词..."
          ></textarea>
        {/if}
        {/key}
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-between px-6 py-4 border-t border-[var(--color-border)]">
        <button
          class="flex items-center gap-1.5 px-3 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors"
          onclick={resetPrompt}
        >
          <span class="material-symbols-outlined text-[14px]">restart_alt</span>
          恢复默认
        </button>
        <div class="flex items-center gap-2">
          <button
            class="px-4 py-2 rounded-xl text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-surface)] transition-colors"
            onclick={() => showPromptSettings = false}
          >取消</button>
          <button
            class="flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors disabled:opacity-50"
            onclick={savePrompt}
            disabled={promptSaving}
          >
            {#if promptSaving}
              <span class="material-symbols-outlined text-[14px] animate-spin">progress_activity</span>
            {:else}
              <span class="material-symbols-outlined text-[14px]">save</span>
            {/if}
            保存
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

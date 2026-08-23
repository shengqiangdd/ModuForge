// ═══════════════════════════════════════════════════════════════
// Pure utility functions extracted from +page.svelte
// ═══════════════════════════════════════════════════════════════

export function generateUUID(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID();
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
}

// ─── File utilities ───

export function getFileLanguage(path: string): string {
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

export function getFileIcon(path: string): string {
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

// ─── Content parsing ───

export interface RecommendedFile {
  path: string;
  required: boolean;
  description: string;
}

export function cleanRecommendedContent(content: string): string {
  let cleaned = content.replace(/\{"recommended_files":\s*\[[\s\S]*?\]\}/, '').trim();
  cleaned = cleaned.replace(/^function call:\s*\w+\([^)]*\)\s*$/gm, '');
  cleaned = cleaned.replace(/^Function Call:\s*\w+\([^)]*\)\s*$/gm, '');
  cleaned = cleaned.replace(/^Tool '\w+' not found\. Available tools:.*$/gm, '');
  cleaned = cleaned.replace(/^Executing tool '\w+'\.\.\.$/gm, '');
  cleaned = cleaned.replace(/^(\w+_?\w*)\s*$/gm, (match, p1) => {
    const knownTools = ['read_file', 'write_file', 'validate', 'lint_code', 'review_code', 'detect', 'check_compat', 'profile_code', 'gen_docs', 'web_search', 'think', 'gather_requirements', 'match_template', 'test_module', 'regression_check', 'create_dir', 'generate_code', 'code_pipeline', 'build_module', 'memory_manager', 'self_evolve', 'pattern_learn', 'agent_preset'];
    return knownTools.includes(p1) ? '' : match;
  });
  cleaned = cleaned.replace(/^Successfully wrote to '.*'$/gm, '');
  cleaned = cleaned.replace(/function call:\s*\w+\([\s\S]*?\)/gm, '');
  cleaned = cleaned.replace(/\n{3,}/g, '\n\n');
  return cleaned.trim();
}

export function extractRecommendedFiles(content: string): RecommendedFile[] | null {
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
  } catch { /* JSON.parse 失败时继续下方字符串匹配 */ }
  return null;
}

export function extractFiles(content: string): { path: string; content: string }[] | null {
  function tryParse(text: string): { path: string; content: string }[] | null {
    try {
      const parsed = JSON.parse(text);
      if (parsed.files && Array.isArray(parsed.files)) {
        for (const f of parsed.files) {
          if (typeof f.path !== 'string' || typeof f.content !== 'string') return null;
        }
        return parsed.files;
      }
    } catch { /* JSON.parse 失败时继续下方字符串匹配 */ }
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

export function parseMessageContent(content: string): Array<{type: 'text'; content: string} | {type: 'code'; language: string; content: string}> {
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

export function checkWebUIFiles(files: {path: string; content: string}[]): boolean {
  for (const f of files) {
    if (f.path.startsWith('webroot/') && (f.path.endsWith('.html') || f.path.endsWith('.htm'))) {
      return true;
    }
  }
  return false;
}

export function getWebUIPreviewHTML(files: {path: string; content: string}[]): string {
  const htmlFile = files.find(f => f.path.startsWith('webroot/') && (f.path.endsWith('.html') || f.path.endsWith('.htm')));
  if (!htmlFile) return '';
  let html = htmlFile.content;
  const cssFiles = files.filter(f => f.path.startsWith('webroot/') && f.path.endsWith('.css'));
  const jsFiles = files.filter(f => f.path.startsWith('webroot/') && (f.path.endsWith('.js') || f.path.endsWith('.mjs')));
  const scriptClose = '<' + '/script>';
  // Inject CSP meta tag into head to restrict scripts, styles, and connections in iframe preview
  // connect-src 'none' blocks all network requests (fetch, XHR, WebSocket) from iframe
  const cspMeta = '<meta http-equiv="Content-Security-Policy" content="default-src \'self\' \'unsafe-inline\'; script-src \'self\' \'unsafe-inline\'; style-src \'self\' \'unsafe-inline\'; img-src \'self\' data:; connect-src \'none\';">';
  html = html.replace(/<head(\s[^>]*)?>/i, '$&' + cspMeta);
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

// ─── Error parsing ───

export interface ErrorDetail {
  message: string;
  suggestion: string;
}

export function parseErrorDetail(content: string): ErrorDetail | null {
  try {
    const parsed = JSON.parse(content);
    if (parsed.error && parsed.error_detail) {
      return { message: parsed.error, suggestion: parsed.suggestion || '' };
    }
    if (parsed.error) {
      return { message: parsed.error, suggestion: '' };
    }
  } catch { /* JSON.parse 失败时继续下方字符串匹配 */ }
  if (content.includes('AI service unavailable') || content.includes('LLM not configured')) {
    return {
      message: 'AI 服务不可用或未配置 API 密钥',
      suggestion: '请在设置中配置 LLM API 密钥，或检查网络连接。'
    };
  }
  return null;
}

export function isGarbledText(text: string): boolean {
  if (!text || text.length < 10) return false;
  // Check for high ratio of non-ASCII characters that look like mojibake
  let garbledCount = 0;
  for (let i = 0; i < text.length; i++) {
    const code = text.charCodeAt(i);
    // Common mojibake patterns: bytes interpreted as Latin-1
    if (code >= 0xC0 && code <= 0xFF && i + 1 < text.length) {
      const next = text.charCodeAt(i + 1);
      if (next >= 0x80 && next <= 0xBF) garbledCount += 2;
    }
  }
  return garbledCount > text.length * 0.3;
}

// ─── Clipboard ───

export async function safeCopyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch { /* JSON.parse 失败时继续下方字符串匹配 */ }
  return fallbackCopy(text);
}

function fallbackCopy(text: string): boolean {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.position = 'fixed';
  ta.style.left = '-9999px';
  document.body.appendChild(ta);
  ta.select();
  const ok = document.execCommand('copy');
  document.body.removeChild(ta);
  return ok;
}

export function copyToClipboard(text: string) {
  safeCopyText(text);
}

// ─── Gather spec extraction ───

export function extractGatherSpec(content: string): Record<string, unknown> | null {
  try {
    const match = content.match(/```json\s*\n?([\s\S]*?)```/);
    if (match) return JSON.parse(match[1]);
    return JSON.parse(content);
  } catch { return null; }
}

// ─── MCP security helpers ───

export function redactArgValues(v: unknown, depth = 0): unknown {
  if (depth > 4) return v;
  if (Array.isArray(v)) return v.map(x => redactArgValues(x, depth + 1));
  if (v && typeof v === 'object') {
    const out: Record<string, unknown> = {};
    for (const [k, val] of Object.entries(v as Record<string, unknown>)) {
      if (/token|secret|password|passwd|api[_-]?key|authorization|auth|credential|bearer|cookie/i.test(k)) {
        out[k] = typeof val === 'string' && val.length > 8 ? `${val.slice(0, 4)}***${val.slice(-2)}` : '***';
      } else {
        out[k] = redactArgValues(val, depth + 1);
      }
    }
    return out;
  }
  return v;
}

import { parseMessageContent, extractFiles, extractRecommendedFiles, parseErrorDetail, checkWebUIFiles, safeCopyText } from './utils';
import type { Message } from './types';

// ─── Markdown rendering (moved from +page.svelte) ───
export function renderMarkdown(text: string): string {
  const codeBlocks: string[] = [];
  let html = text.replace(/```(\w*)\n?([\s\S]*?)```/g, (_m, lang, code) => {
    codeBlocks.push(`<div class="code-block-wrapper relative group"><div class="flex items-center gap-1.5 px-3 py-1.5 text-[10px] font-medium" style="background: #181825; color: #a6adc8; border-bottom: 1px solid #313244;"><span class="material-symbols-outlined text-[12px]">code</span>${lang || 'code'}<div class="ml-auto"><button class="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] transition-colors hover:bg-white/10" style="color: #a6adc8;" onclick="copyCode(this)"><span class="material-symbols-outlined text-[12px]">content_copy</span></button></div></div><pre class="code-block"><code class="language-${lang}">${code}</code></pre></div>`);
    return `\x00CB${codeBlocks.length - 1}\x00`;
  });

  html = html.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  html = html.replace(/`([^`]+)`/g, '<code class="inline-code">$1</code>');
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/(?<!\*)\*(?!\*)(.+?)(?<!\*)\*(?!\*)/g, '<em>$1</em>');
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer" class="ai-link">$1</a>');

  const lines = html.split('\n');
  const blocks: string[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];
    const h3m = line.match(/^###\s+(.+)$/);
    if (h3m) { blocks.push(`<h3 class="text-base font-semibold mt-3 mb-1 text-[var(--color-text)]">${h3m[1]}</h3>`); i++; continue; }
    const h2m = line.match(/^##\s+(.+)$/);
    if (h2m) { blocks.push(`<h2 class="text-lg font-bold mt-4 mb-2 text-[var(--color-text)]">${h2m[1]}</h2>`); i++; continue; }
    const h1m = line.match(/^#\s+(.+)$/);
    if (h1m) { blocks.push(`<h1 class="text-xl font-bold mt-4 mb-2 text-[var(--color-text)]">${h1m[1]}</h1>`); i++; continue; }
    if (/^---+$/.test(line.trim())) { blocks.push('<hr class="my-2 border-[var(--color-border)]">'); i++; continue; }

    if (/^\d+\.\s+/.test(line.trim())) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s+/.test(lines[i].trim())) {
        const m = lines[i].trim().match(/^\d+\.\s+(.+)$/);
        if (m) items.push(`<li class="leading-relaxed">${m[1]}</li>`);
        i++;
      }
      blocks.push(`<ol class="list-decimal pl-5 my-1.5 space-y-0.5">${items.join('')}</ol>`);
      continue;
    }

    if (/^[•\-\*]\s+/.test(line.trim())) {
      const items: string[] = [];
      while (i < lines.length && /^[•\-\*]\s+/.test(lines[i].trim())) {
        const m = lines[i].trim().match(/^[•\-\*]\s+(.+)$/);
        if (m) items.push(`<li class="leading-relaxed">${m[1]}</li>`);
        i++;
      }
      blocks.push(`<ul class="list-disc pl-5 my-1.5 space-y-0.5">${items.join('')}</ul>`);
      continue;
    }

    if (line.trim() === '') { i++; continue; }
    if (/\x00CB\d+\x00/.test(line)) { blocks.push(line); i++; continue; }

    const paraLines: string[] = [];
    while (i < lines.length && lines[i].trim() !== '' && !/^#{1,3}\s/.test(lines[i]) && !/^\d+\.\s+/.test(lines[i].trim()) && !/^[•\-\*]\s+/.test(lines[i].trim()) && !/^---+$/.test(lines[i].trim()) && !/\x00CB\d+\x00/.test(lines[i])) {
      paraLines.push(lines[i]);
      i++;
    }
    if (paraLines.length > 0) {
      blocks.push(`<p class="my-1">${paraLines.join('<br>')}</p>`);
    }
  }

  html = blocks.join('\n');
  html = html.replace(/\x00CB(\d+)\x00/g, (_m, idx) => codeBlocks[parseInt(idx)]);
  return html;
}

// ─── Memoization caches ───
const markdownCache = new Map<string, string>();
const segmentCache = new Map<string, Array<{ type: 'text'; content: string } | { type: 'code'; language: string; content: string }>>();
const filesCache = new Map<string, { path: string; content: string }[] | null>();
const recFilesCache = new Map<string, { path: string; required: boolean; description: string }[] | null>();
const errDetailCache = new Map<string, { message: string; suggestion: string } | null>();
const webUICache = new Map<string, boolean>();

export function memoRenderMarkdown(text: string): string {
  if (markdownCache.has(text)) return markdownCache.get(text)!;
  const html = renderMarkdown(text);
  if (markdownCache.size > 200) markdownCache.clear();
  markdownCache.set(text, html);
  return html;
}

export function memoParseSegments(text: string) {
  if (segmentCache.has(text)) return segmentCache.get(text)!;
  const segs = parseMessageContent(text);
  if (segmentCache.size > 200) segmentCache.clear();
  segmentCache.set(text, segs);
  return segs;
}

export function memoExtractFiles(content: string) {
  if (filesCache.has(content)) return filesCache.get(content)!;
  const files = extractFiles(content);
  if (filesCache.size > 100) filesCache.clear();
  filesCache.set(content, files);
  return files;
}

export function memoExtractRecFiles(content: string) {
  if (recFilesCache.has(content)) return recFilesCache.get(content)!;
  const files = extractRecommendedFiles(content);
  if (recFilesCache.size > 100) recFilesCache.clear();
  recFilesCache.set(content, files);
  return files;
}

export function memoParseErrorDetail(content: string) {
  if (errDetailCache.has(content)) return errDetailCache.get(content)!;
  const detail = parseErrorDetail(content);
  if (errDetailCache.size > 100) errDetailCache.clear();
  errDetailCache.set(content, detail);
  return detail;
}

export function memoCheckWebUI(files: { path: string; content: string }[]): boolean {
  const key = files.map(f => f.path).join('|');
  if (webUICache.has(key)) return webUICache.get(key)!;
  const result = checkWebUIFiles(files);
  if (webUICache.size > 50) webUICache.clear();
  webUICache.set(key, result);
  return result;
}

// ─── Markdown Worker ───
let markdownWorker: Worker | null = null;
const markdownWorkerCallbacks = new Map<number, (html: string) => void>();
let markdownWorkerId = 0;

export function initMarkdownWorker(): void {
  try {
    markdownWorker = new Worker(new URL('$lib/workers/markdown.worker.ts', import.meta.url), { type: 'module' });
    markdownWorker.onmessage = (e) => {
      const { id, html } = e.data;
      const callback = markdownWorkerCallbacks.get(id);
      if (callback) {
        callback(html);
        markdownWorkerCallbacks.delete(id);
      }
    };
  } catch (err) {
    console.warn('Failed to initialize markdown worker, falling back to main thread:', err);
    markdownWorker = null;
  }
}

export function terminateMarkdownWorker(): void {
  if (markdownWorker) {
    markdownWorker.terminate();
    markdownWorker = null;
    markdownWorkerCallbacks.clear();
  }
}

export function preRenderMarkdown(text: string): void {
  if (markdownCache.has(text) || !markdownWorker) return;
  const id = ++markdownWorkerId;
  markdownWorkerCallbacks.set(id, (html) => {
    if (markdownCache.size > 200) markdownCache.clear();
    markdownCache.set(text, html);
  });
  markdownWorker.postMessage({ id, text });
}

export function preRenderVisibleMessages(visibleMessages: Message[]): void {
  if (!markdownWorker) return;
  for (const msg of visibleMessages) {
    if (msg.role === 'assistant' && msg.content) {
      preRenderMarkdown(msg.content);
    }
  }
}

// ─── Global copyCode setup ───
export function setupCopyCode(): void {
  if (typeof window !== 'undefined' && !(window as any).copyCode) {
    (window as any).copyCode = function (btn: HTMLElement) {
      const pre = btn.closest('.code-block-wrapper')?.querySelector('pre code');
      if (!pre) return;
      const text = pre.textContent || '';
      safeCopyText(text).then(ok => {
        if (ok) {
          btn.innerHTML = '<span class="material-symbols-outlined text-[12px]">check</span>';
          setTimeout(() => { btn.innerHTML = '<span class="material-symbols-outlined text-[12px]">content_copy</span>'; }, 2000);
        }
      });
    };
  }
}

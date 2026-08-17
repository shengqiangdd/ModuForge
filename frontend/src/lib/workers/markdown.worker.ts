// Markdown rendering Web Worker
// Offloads expensive markdown parsing from the main thread

interface RenderRequest {
  id: number;
  text: string;
}

interface RenderResponse {
  id: number;
  html: string;
}

// Simple markdown parser (same logic as main thread, but in worker)
function renderMarkdown(text: string): string {
  // 1. Extract code blocks first to protect them
  const codeBlocks: string[] = [];
  // Quotes are escaped too to prevent attribute injection (e.g. a language
  // name or link URL containing `"` breaking out of an attribute).
  const escapeHtml = (s: string) =>
    s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  let html = text.replace(/```(\w*)\n?([\s\S]*?)```/g, (_m, lang, code) => {
    codeBlocks.push(`<div class="code-block-wrapper relative group"><div class="flex items-center gap-1.5 px-3 py-1.5 text-[10px] font-medium" style="background: #181825; color: #a6adc8; border-bottom: 1px solid #313244;"><span class="material-symbols-outlined text-[12px]">code</span>${escapeHtml(lang || 'code')}<div class="ml-auto"><button class="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] transition-colors hover:bg-white/10" style="color: #a6adc8;" onclick="copyCode(this)"><span class="material-symbols-outlined text-[12px]">content_copy</span></button></div></div><pre class="code-block"><code class="language-${escapeHtml(lang)}">${escapeHtml(code)}</code></pre></div>`);
    return `\x00CB${codeBlocks.length - 1}\x00`;
  });

  // 2. Escape HTML
  html = html.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

  // 3. Inline code
  html = html.replace(/`([^`]+)`/g, '<code class="inline-code">$1</code>');

  // 4. Bold & Italic
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/(?<!\*)\*(?!\*)(.+?)(?<!\*)\*(?!\*)/g, '<em>$1</em>');

  // 5. Links (with scheme validation to prevent javascript: XSS)
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_m, text, url) => {
    if (/^(javascript|data|vbscript):/i.test(url.trim())) {
      return text; // strip dangerous links, show text only
    }
    // Quote-escape the URL so it cannot break out of the href attribute
    // (attribute injection, e.g. [x](" onmouseover="alert(1))).
    const safeUrl = escapeHtml(url);
    return `<a href="${safeUrl}" target="_blank" rel="noopener noreferrer" class="ai-link">${text}</a>`;
  });

  // 6. Process line by line
  const lines = html.split('\n');
  const blocks: string[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Headings
    const h3m = line.match(/^###\s+(.+)$/);
    if (h3m) { blocks.push(`<h3 class="text-base font-semibold mt-3 mb-1 text-[var(--color-text)]">${h3m[1]}</h3>`); i++; continue; }
    const h2m = line.match(/^##\s+(.+)$/);
    if (h2m) { blocks.push(`<h2 class="text-lg font-bold mt-4 mb-2 text-[var(--color-text)]">${h2m[1]}</h2>`); i++; continue; }
    const h1m = line.match(/^#\s+(.+)$/);
    if (h1m) { blocks.push(`<h1 class="text-xl font-bold mt-4 mb-2 text-[var(--color-text)]">${h1m[1]}</h1>`); i++; continue; }

    // Horizontal rule
    if (/^---+$/.test(line.trim())) { blocks.push('<hr class="my-2 border-[var(--color-border)]">'); i++; continue; }

    // Numbered list
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

    // Bullet list
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

    // Empty line
    if (line.trim() === '') { i++; continue; }

    // Code block placeholder
    if (/\x00CB\d+\x00/.test(line)) { blocks.push(line); i++; continue; }

    // Regular text
    const paraLines: string[] = [];
    while (i < lines.length && lines[i].trim() !== '' && !/^#{1,3}\s/.test(lines[i]) && !/^\d+\.\s+/.test(lines[i].trim()) && !/^[•\-\*]\s+/.test(lines[i].trim()) && !/^---+$/.test(lines[i].trim()) && !/\x00CB\d+\x00/.test(lines[i])) {
      paraLines.push(lines[i]);
      i++;
    }
    if (paraLines.length > 0) {
      blocks.push(`<p class="my-1.5 leading-relaxed">${paraLines.join('<br>')}</p>`);
    }
  }

  // Join blocks and restore code blocks
  let result = blocks.join('\n');
  result = result.replace(/\x00CB(\d+)\x00/g, (_m, idx) => codeBlocks[parseInt(idx)] || '');

  return result;
}

// Cache for rendered results
const cache = new Map<string, string>();
const MAX_CACHE_SIZE = 100;

// Handle messages from main thread
self.onmessage = (e: MessageEvent<RenderRequest>) => {
  const { id, text } = e.data;

  // Check cache
  const cacheKey = text.length > 200 ? text.substring(0, 200) : text;
  if (cache.has(cacheKey)) {
    const response: RenderResponse = { id, html: cache.get(cacheKey)! };
    self.postMessage(response);
    return;
  }

  // Render
  const html = renderMarkdown(text);

  // Cache result
  if (cache.size >= MAX_CACHE_SIZE) {
    const firstKey = cache.keys().next().value;
    if (firstKey) cache.delete(firstKey);
  }
  cache.set(cacheKey, html);

  const response: RenderResponse = { id, html };
  self.postMessage(response);
};

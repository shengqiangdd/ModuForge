export function renderMarkdown(text: string) {
  // HTML-escape the input first to prevent XSS
  const escape = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  const escaped = escape(text);
  return escaped
    .replace(/### (.+)/g, '<strong class="text-sm block mt-2 mb-1">$1</strong>')
    .replace(/## (.+)/g, '<strong class="text-base block mt-3 mb-1">$1</strong>')
    .replace(/# (.+)/g, '<strong class="text-lg block mt-3 mb-1">$1</strong>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" target="_blank" class="text-primary-600 underline">$1</a>')
    .replace(/\n/g, '<br>');
}
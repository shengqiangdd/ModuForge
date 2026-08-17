export function renderMarkdown(text: string) {
  // HTML-escape the input first to prevent XSS.
  // Quotes are escaped too: an unescaped " in a URL would otherwise allow
  // attribute injection like  [x](" onmouseover="alert(1))  → href=""
  // onmouseover="alert(1)" ... which turns AI/market content into XSS.
  const escape = (s: string) =>
    s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  const escaped = escape(text);
  return escaped
    .replace(/### (.+)/g, '<strong class="text-sm block mt-2 mb-1">$1</strong>')
    .replace(/## (.+)/g, '<strong class="text-base block mt-3 mb-1">$1</strong>')
    .replace(/# (.+)/g, '<strong class="text-lg block mt-3 mb-1">$1</strong>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\[(.+?)\]\((.+?)\)/g, (m, text, url) => {
      // Allow only safe URL schemes: http(s), mailto, anchors and relative
      // paths. Strip javascript:, data:, vbscript: and anything else that
      // could execute script on click.
      if (!/^(https?:|mailto:|#|\/|\.\/|\.\.\/)/i.test(url)) {
        return text;
      }
      return (
        '<a href="' +
        url +
        '" target="_blank" rel="noopener noreferrer" class="text-primary-600 underline">' +
        text +
        '</a>'
      );
    })
    .replace(/\n/g, '<br>');
}
